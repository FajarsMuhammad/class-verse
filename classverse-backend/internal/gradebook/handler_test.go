package gradebook_test

import (
	"crypto/ed25519"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/auth"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/gradebook"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/org"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/dbtest"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

func TestHTTP(t *testing.T) {
	p := dbtest.New(t)
	q := query.New(p)
	ctx := t.Context()
	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	user := func(name string) uuid.UUID {
		u, err := q.CreateTestUser(ctx, query.CreateTestUserParams{Email: uuid.NewString() + "@test.invalid", DisplayName: name})
		if err != nil {
			t.Fatal(err)
		}
		return u.ID
	}
	admin, ani, outsider := user("Guru"), user("=Ani"), user("Luar")
	school, err := q.CreateOrg(ctx, query.CreateOrgParams{Name: "S", Slug: "gb-http", CreatedBy: admin, CreatedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, `INSERT INTO memberships(org_id,user_id,role) VALUES($1,$2,'admin'),($1,$3,'participant')`, school.ID, admin, ani); err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	auths := org.NewService(org.NewStore(p), time.Now, log)
	tokens := auth.NewTokens(ed25519.NewKeyFromSeed(make([]byte, 32)), time.Now)
	r := chi.NewRouter()
	r.Use(auth.RequireUser(tokens, log))
	gradebook.NewHandler(gradebook.NewService(p, auths, buildings{}), log, time.Now).Routes(r)
	call := func(u uuid.UUID, path string) (int, string, string) {
		tok, err := tokens.Issue(u)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("%s missing no-store", path)
		}
		return w.Code, w.Body.String(), w.Header().Get("Content-Type")
	}
	base := "/orgs/" + school.ID.String() + "/gradebook"
	for _, tc := range []struct {
		user uuid.UUID
		path string
		code int
	}{
		{ani, base + "/me", 200},
		{ani, base + "/students/" + ani.String(), 200},
		{ani, base + "/students/" + admin.String(), 403},
		{ani, base, 403},
		{ani, base + "/csv", 403},
		{outsider, base + "/me", 404},
		{admin, base + "/students/" + outsider.String(), 404},
		{admin, base + "/students/nope", 404},
		{admin, base + "?year_id=bad", 400},
		{admin, base + "?limit=51", 400},
		{admin, base + "?class_id=" + uuid.NewString(), 400},
		{admin, base + "?module_id=" + uuid.NewString(), 404},
		{admin, base + "?building_key=ruang-guru", 404},
		{admin, base + "?limit=20", 200},
	} {
		if code, body, _ := call(tc.user, tc.path); code != tc.code {
			t.Errorf("%s = %d %s", tc.path, code, body)
		}
	}
	code, body, kind := call(admin, base+"/csv")
	if code != 200 || !strings.HasPrefix(kind, "text/csv") || !strings.Contains(body, "'=Ani") || strings.Contains(body, "@test.invalid") {
		t.Fatalf("csv=%d %s %q", code, kind, body)
	}
}
