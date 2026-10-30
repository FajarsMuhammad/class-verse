package invite

import (
	"crypto/ed25519"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/auth"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/org"
)

func TestHandlerFlow(t *testing.T) {
	f := setup(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	tokens := auth.NewTokens(ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize)), f.clock.Now)

	router := chi.NewRouter()
	router.Route("/api", func(r chi.Router) {
		r.Use(auth.RequireUser(tokens, log))
		org.NewHandler(f.orgSvc, log).Routes(r)
		NewHandler(f.svc, "http://localhost:5173", f.clock.Now, log).Routes(r)
	})

	call := func(user uuid.UUID, method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		var rd io.Reader = http.NoBody
		if body != "" {
			rd = strings.NewReader(body)
		}
		req := httptest.NewRequest(method, path, rd)
		tok, err := tokens.Issue(user)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+tok)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	code := func(rec *httptest.ResponseRecorder) string {
		var b struct {
			Error struct{ Code string } `json:"error"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &b)
		return b.Error.Code
	}
	orgPath := "/api/orgs/" + f.orgID.String()

	if rec := call(f.outsider, http.MethodGet, orgPath, ""); rec.Code != http.StatusNotFound || code(rec) != "not_found" {
		t.Errorf("outsider get org = %d %s", rec.Code, rec.Body)
	}
	if rec := call(f.member, http.MethodGet, orgPath, ""); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"role":"participant"`) {
		t.Errorf("member get org = %d %s", rec.Code, rec.Body)
	}
	if rec := call(f.member, http.MethodGet, "/api/orgs/not-a-uuid", ""); rec.Code != http.StatusNotFound {
		t.Errorf("malformed org id = %d", rec.Code)
	}
	if rec := call(f.member, http.MethodPost, orgPath+"/invites", `{"role":"participant"}`); rec.Code != http.StatusForbidden {
		t.Errorf("participant create = %d %s", rec.Code, rec.Body)
	}
	if rec := call(f.outsider, http.MethodPost, orgPath+"/invites", `{"role":"participant"}`); rec.Code != http.StatusNotFound {
		t.Errorf("outsider create = %d %s", rec.Code, rec.Body)
	}

	rec := call(f.admin, http.MethodPost, orgPath+"/invites", `{"role":"teacher","expires_in_days":3,"max_uses":5}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("admin create = %d %s", rec.Code, rec.Body)
	}
	var inv struct {
		ID   string `json:"id"`
		Code string `json:"code"`
		Link string `json:"link"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &inv)
	if inv.Link != "http://localhost:5173/gabung/"+inv.Code {
		t.Errorf("link = %q", inv.Link)
	}

	if rec := call(f.joiner, http.MethodGet, "/api/invites/"+inv.Code, ""); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"org_name":"SMA Satu"`) {
		t.Errorf("preview = %d %s", rec.Code, rec.Body)
	}
	if rec := call(f.joiner, http.MethodPost, "/api/invites/"+inv.Code+"/accept", ""); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"role":"teacher"`) {
		t.Errorf("accept = %d %s", rec.Code, rec.Body)
	}
	if rec := call(f.joiner, http.MethodPost, "/api/invites/"+inv.Code+"/accept", ""); rec.Code != http.StatusConflict {
		t.Errorf("accept again = %d %s", rec.Code, rec.Body)
	}

	if rec := call(f.admin, http.MethodGet, orgPath+"/invites?limit=1", ""); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"next_cursor":null`) {
		t.Errorf("list = %d %s", rec.Code, rec.Body)
	}
	if rec := call(f.admin, http.MethodDelete, orgPath+"/invites/"+inv.ID, ""); rec.Code != http.StatusNoContent {
		t.Errorf("revoke = %d %s", rec.Code, rec.Body)
	}
	if rec := call(f.outsider, http.MethodGet, "/api/invites/"+inv.Code, ""); rec.Code != http.StatusGone || code(rec) != "invite_expired" {
		t.Errorf("revoked preview = %d %s", rec.Code, rec.Body)
	}

	// Guessing codes: the 10th request in a minute is still allowed, the 11th is not.
	var last *httptest.ResponseRecorder
	for range 11 {
		last = call(f.outsider, http.MethodGet, "/api/invites/ZZZZ-ZZZZ", "")
	}
	if last.Code != http.StatusTooManyRequests || last.Header().Get("Retry-After") == "" {
		t.Errorf("rate limit = %d %v", last.Code, last.Header())
	}
}

func TestMeListsOrgs(t *testing.T) {
	f := setup(t)
	orgs, err := f.orgSvc.ListOrgSummaries(t.Context(), f.teacher)
	if err != nil || len(orgs) != 1 || orgs[0].Slug != "sma-satu" || orgs[0].Role != "teacher" {
		t.Fatalf("ListOrgSummaries = %+v, %v", orgs, err)
	}
	none, err := f.orgSvc.ListOrgSummaries(t.Context(), f.outsider)
	if err != nil || none == nil || len(none) != 0 {
		t.Errorf("outsider = %#v, %v (must be empty, not nil, for JSON [])", none, err)
	}
}
