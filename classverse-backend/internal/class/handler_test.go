package class_test

import (
	"crypto/ed25519"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/auth"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/class"
)

func TestHTTP(t *testing.T) {
	f := setup(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	tokens := auth.NewTokens(ed25519.NewKeyFromSeed(make([]byte, 32)), time.Now)
	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireUser(tokens, log))
		r.Route("/api", class.NewHandler(f.svc, log).Routes)
	})
	call := func(user uuid.UUID, method, path, body string) (int, string) {
		tok, err := tokens.Issue(user)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+tok)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code, w.Body.String()
	}
	id := func(body string) string {
		var v struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal([]byte(body), &v); err != nil || v.ID == "" {
			t.Fatalf("no id in %s", body)
		}
		return v.ID
	}
	base := "/api/orgs/" + f.school.String()

	if code, _ := call(f.teacher, http.MethodPost, base+"/academic-years", `{"label":"2026/2027"}`); code != http.StatusForbidden {
		t.Fatalf("teacher creates year=%d", code)
	}
	code, body := call(f.admin, http.MethodPost, base+"/academic-years", `{"label":"2026/2027"}`)
	if code != http.StatusCreated || !strings.Contains(body, `"active":true`) {
		t.Fatalf("create year=%d %s", code, body)
	}
	year := base + "/academic-years/" + id(body)
	if code, body := call(f.admin, http.MethodPost, base+"/academic-years", `{"label":"2026-27"}`); code != http.StatusBadRequest || !strings.Contains(body, "label") {
		t.Fatalf("bad label=%d %s", code, body)
	}
	if code, _ := call(f.ani, http.MethodGet, base+"/academic-years", ""); code != http.StatusForbidden {
		t.Fatalf("participant years=%d", code)
	}
	if code, _ := call(f.out, http.MethodGet, base+"/academic-years", ""); code != http.StatusNotFound {
		t.Fatalf("outsider years=%d", code)
	}
	code, body = call(f.admin, http.MethodPost, year+"/classes", `{"name":"  XI   IPA 2 "}`)
	if code != http.StatusCreated || !strings.Contains(body, `"name":"XI IPA 2"`) {
		t.Fatalf("create class=%d %s", code, body)
	}
	cls := id(body)
	if code, body := call(f.admin, http.MethodPost, year+"/classes", `{"name":"xi ipa 2"}`); code != http.StatusBadRequest || !strings.Contains(body, "sudah dipakai") {
		t.Fatalf("duplicate class=%d %s", code, body)
	}
	if code, _ := call(f.admin, http.MethodPut, year+"/roster/"+f.ani.String(), `{"class_id":"`+cls+`"}`); code != http.StatusNoContent {
		t.Fatalf("place=%d", code)
	}
	for _, invalid := range []string{`{}`, `{"class_id":"invalid"}`, `{"class_id":42}`} {
		if code, _ := call(f.admin, http.MethodPut, year+"/roster/"+f.ani.String(), invalid); code != http.StatusBadRequest {
			t.Fatalf("invalid placement body %s: %d", invalid, code)
		}
	}
	if code, _ := call(f.admin, http.MethodPut, year+"/roster/"+f.admin.String(), `{"class_id":"`+cls+`"}`); code != http.StatusNotFound {
		t.Fatalf("place teacher=%d", code)
	}
	if code, _ := call(f.ani, http.MethodPut, year+"/roster/"+f.ani.String(), `{"class_id":null}`); code != http.StatusForbidden {
		t.Fatalf("student edits roster=%d", code)
	}
	code, body = call(f.teacher, http.MethodGet, year+"/roster", "")
	if code != http.StatusOK || !strings.Contains(body, `"class_id":"`+cls+`"`) || !strings.Contains(body, `"class_id":null`) || strings.Contains(body, "teacher@class.test") {
		t.Fatalf("roster=%d %s", code, body)
	}
	if code, body := call(f.admin, http.MethodDelete, base+"/classes/"+cls, ""); code != http.StatusConflict || !strings.Contains(body, "class_not_empty") {
		t.Fatalf("archive non-empty=%d %s", code, body)
	}
	if code, body := call(f.teacher, http.MethodGet, base+"/members/"+f.ani.String()+"/placements", ""); code != http.StatusOK || !strings.Contains(body, `"year_label":"2026/2027"`) || !strings.Contains(body, `"ended_at":null`) {
		t.Fatalf("history=%d %s", code, body)
	}
	if code, _ := call(f.admin, http.MethodPut, year+"/roster/"+f.ani.String(), `{"class_id":null}`); code != http.StatusNoContent {
		t.Fatalf("remove=%d", code)
	}
	if code, body := call(f.admin, http.MethodPut, base+"/classes/"+cls, `{"name":"XI IPA Dua"}`); code != http.StatusOK || !strings.Contains(body, "XI IPA Dua") {
		t.Fatalf("rename=%d %s", code, body)
	}
	if code, _ := call(f.admin, http.MethodDelete, base+"/classes/"+cls, ""); code != http.StatusNoContent {
		t.Fatalf("archive=%d", code)
	}
	if code, _ := call(f.admin, http.MethodDelete, base+"/classes/"+cls, ""); code != http.StatusNotFound {
		t.Fatalf("archive twice=%d", code)
	}
	// Another school's admin cannot touch this year, even with a valid id.
	if code, _ := call(f.stray, http.MethodGet, "/api/orgs/"+f.other.String()+"/academic-years/"+strings.TrimPrefix(year, base+"/academic-years/")+"/classes", ""); code != http.StatusNotFound {
		t.Fatalf("cross-org year=%d", code)
	}
	if code, _ := call(f.admin, http.MethodPut, base+"/academic-years/nope/active", ""); code != http.StatusNotFound {
		t.Fatalf("malformed id=%d", code)
	}
	if code, _ := call(f.admin, http.MethodPut, year+"/active", ""); code != http.StatusNoContent {
		t.Fatalf("activate=%d", code)
	}
}
