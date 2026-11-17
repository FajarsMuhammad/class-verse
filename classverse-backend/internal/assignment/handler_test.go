package assignment_test

import (
	"crypto/ed25519"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/assignment"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/auth"
)

func TestHTTP(t *testing.T) {
	f := setup(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	tokens := auth.NewTokens(ed25519.NewKeyFromSeed(make([]byte, 32)), time.Now)
	r := chi.NewRouter()
	r.Use(auth.RequireUser(tokens, log))
	assignment.NewHandler(f.svc, log).Routes(r)
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
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("missing no-store")
		}
		return w.Code, w.Body.String()
	}
	id := uuid.New()
	base := "/orgs/" + f.org.String() + "/assignments"
	payload := `{"id":"` + id.String() + `","class_id":"` + f.class.String() + `","module_id":"` + f.module.String() + `"}`
	for _, u := range []uuid.UUID{f.admin, f.admin} {
		if code, body := call(u, "POST", base, payload); code != 200 {
			t.Fatalf("issue=%d %s", code, body)
		}
	}
	if code, _ := call(f.student, "POST", base, payload); code != 403 {
		t.Fatalf("student issue=%d", code)
	}
	if code, _ := call(f.other, "GET", base, ""); code != 404 {
		t.Fatalf("outsider=%d", code)
	}
	if code, body := call(f.admin, "POST", base, `{}`); code != 400 {
		t.Fatalf("bad input=%d %s", code, body)
	}
	if code, _ := call(f.admin, "POST", base, `{"id":"bad"}`); code != 400 {
		t.Fatalf("invalid json=%d", code)
	}
	if code, body := call(f.student, "GET", base, ""); code != 200 || !strings.Contains(body, "Modul awal") {
		t.Fatalf("list=%d %s", code, body)
	}
	path := base + "/" + id.String()
	code, body := call(f.student, "GET", path+"/steps", "")
	if code != 200 {
		t.Fatal(body)
	}
	var response struct {
		Items []assignment.StepResponse `json:"items"`
	}
	if err := json.Unmarshal([]byte(body), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Items) != 2 || response.Items[1].Body != "" || response.Items[1].Unlocked {
		t.Fatalf("locked content leaked: %s", body)
	}
	if code, _ := call(f.student, "PUT", path+"/steps/"+response.Items[1].ID.String()+"/done", ""); code != 409 {
		t.Fatalf("locked=%d", code)
	}
	if code, _ := call(f.student, "PUT", path+"/steps/"+response.Items[0].ID.String()+"/done", ""); code != 204 {
		t.Fatalf("complete=%d", code)
	}
	if code, body := call(f.admin, "GET", path+"/roster", ""); code != 200 || !strings.Contains(body, `"done":1`) {
		t.Fatalf("roster=%d %s", code, body)
	}
	if code, _ := call(f.student, "GET", path+"/roster", ""); code != 403 {
		t.Fatalf("private roster=%d", code)
	}
	if code, _ := call(f.student, "DELETE", path, ""); code != 403 {
		t.Fatalf("student cancel=%d", code)
	}
	if code, _ := call(f.admin, "DELETE", path, ""); code != 204 {
		t.Fatalf("cancel=%d", code)
	}
	if code, _ := call(f.student, "PUT", path+"/steps/"+response.Items[1].ID.String()+"/done", ""); code != 409 {
		t.Fatalf("cancelled=%d", code)
	}
	if code, _ := call(f.admin, "GET", base+"/nope/steps", ""); code != 404 {
		t.Fatalf("malformed=%d", code)
	}
	if code, _ := call(f.admin, "GET", base+"/"+uuid.NewString()+"/steps", ""); code != 404 {
		t.Fatalf("missing=%d", code)
	}
}
