package module_test

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/auth"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/module"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/org"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/dbtest"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

// TestProgress covers free-order completion, idempotency, isolation, archive, and "Lanjutkan belajar".
func TestProgress(t *testing.T) {
	pool := dbtest.New(t)
	ctx := t.Context()
	now := time.Date(2026, 11, 13, 8, 0, 0, 0, time.UTC)
	var ticks atomic.Int64
	clock := func() time.Time { return now.Add(time.Duration(ticks.Add(1)) * time.Second) }
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	q := query.New(pool)
	user := func(email string) uuid.UUID {
		u, err := q.CreateTestUser(ctx, query.CreateTestUserParams{Email: email, DisplayName: email, BirthDate: pgtype.Date{Time: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true}})
		if err != nil {
			t.Fatal(err)
		}
		return u.ID
	}
	admin, student, peer, outsider := user("admin@prog.test"), user("s@prog.test"), user("p@prog.test"), user("o@prog.test")
	orgs := org.NewService(org.NewStore(pool), time.Now, log)
	school, err := orgs.Create(ctx, admin, org.CreateInput{Name: "Prog School", Slug: "prog-school"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := orgs.Create(ctx, outsider, org.CreateInput{Name: "Other", Slug: "prog-other"})
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range []uuid.UUID{student, peer} {
		if _, err := pool.Exec(ctx, "INSERT INTO memberships (org_id, user_id, role, joined_at) VALUES ($1, $2, 'participant', now())", school.ID, u); err != nil {
			t.Fatal(err)
		}
	}
	svc := module.NewService(pool, orgs, buildings{"sains": false, "angka": false}, clock)
	path := func(title string, building string, published bool, n int) (uuid.UUID, []uuid.UUID) {
		t.Helper()
		m, err := svc.Create(ctx, admin, school.ID, module.Input{BuildingKey: building, Level: "SD", Title: title})
		if err != nil {
			t.Fatal(err)
		}
		steps := make([]uuid.UUID, n)
		for i := range n {
			st, err := svc.AddStep(ctx, admin, school.ID, m.ID, module.StepInput{Kind: "materi", Title: fmt.Sprintf("%s %d", title, i+1), Body: "isi", Minutes: 5})
			if err != nil {
				t.Fatal(err)
			}
			steps[i] = st.ID
		}
		if published {
			if _, err := svc.Publish(ctx, admin, school.ID, m.ID, int64(n+1)); err != nil {
				t.Fatal(err)
			}
		}
		return m.ID, steps
	}
	solar, ss := path("Tata Surya", "sains", true, 3)
	math, ms := path("Pecahan", "angka", true, 2)
	draft, ds := path("Draf", "sains", false, 1)

	if r, err := svc.ResumeFor(ctx, student, school.ID); err != nil || r != nil {
		t.Fatalf("resume before any progress: %+v %v", r, err)
	}

	// Free order: the last step can be completed first; repeats and concurrent repeats are idempotent.
	var wg sync.WaitGroup
	for range 5 {
		wg.Go(func() {
			if err := svc.SetDone(ctx, student, school.ID, solar, ss[2], true); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	var rows int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM module_progress WHERE user_id = $1", student).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("idempotent rows = %d %v", rows, err)
	}
	var first time.Time
	if err := pool.QueryRow(ctx, "SELECT completed_at FROM module_progress WHERE org_id = $1 AND user_id = $2 AND step_id = $3", school.ID, student, ss[2]).Scan(&first); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetDone(ctx, student, school.ID, solar, ss[2], true); err != nil {
		t.Fatal(err)
	}
	var repeated time.Time
	if err := pool.QueryRow(ctx, "SELECT completed_at FROM module_progress WHERE org_id = $1 AND user_id = $2 AND step_id = $3", school.ID, student, ss[2]).Scan(&repeated); err != nil || !first.Equal(repeated) {
		t.Fatalf("repeat changed completion time: %v %v %v", first, repeated, err)
	}
	r, err := svc.ResumeFor(ctx, student, school.ID)
	if err != nil || r == nil || r.ModuleID != solar || r.Done != 1 || r.Total != 3 || r.NextTitle != "Tata Surya 1" {
		t.Fatalf("resume after first step: %+v %v", r, err)
	}

	// Most recent activity wins; a finished module is not offered.
	if err := svc.SetDone(ctx, student, school.ID, math, ms[0], true); err != nil {
		t.Fatal(err)
	}
	if r, _ := svc.ResumeFor(ctx, student, school.ID); r == nil || r.ModuleID != math {
		t.Fatalf("resume most recent: %+v", r)
	}
	if err := svc.SetDone(ctx, student, school.ID, math, ms[1], true); err != nil {
		t.Fatal(err)
	}
	if r, _ := svc.ResumeFor(ctx, student, school.ID); r == nil || r.ModuleID != solar {
		t.Fatalf("finished module still offered: %+v", r)
	}

	// Undo, per-user isolation, and catalog flags.
	if err := svc.SetDone(ctx, student, school.ID, solar, ss[2], false); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetDone(ctx, student, school.ID, solar, ss[0], true); err != nil {
		t.Fatal(err)
	}
	flags := func(u uuid.UUID) []bool {
		t.Helper()
		mods, err := svc.List(ctx, u, school.ID, module.Filter{BuildingKey: "sains"})
		if err != nil || len(mods) != 1 {
			t.Fatalf("list: %+v %v", mods, err)
		}
		out := []bool{}
		for _, s := range mods[0].Steps {
			out = append(out, s.Done)
		}
		return out
	}
	if got := fmt.Sprint(flags(student)); got != "[true false false]" {
		t.Fatalf("student flags %s", got)
	}
	if got := fmt.Sprint(flags(peer)); got != "[false false false]" {
		t.Fatalf("peer flags %s", got)
	}

	// Drafts, archived steps, other modules, and other schools are not found.
	if err := svc.SetDone(ctx, student, school.ID, draft, ds[0], true); !errors.Is(err, module.ErrNotFound) {
		t.Fatalf("draft step: %v", err)
	}
	if err := svc.SetDone(ctx, admin, school.ID, draft, ds[0], true); !errors.Is(err, module.ErrNotFound) {
		t.Fatalf("staff tries a draft: %v", err)
	}
	if err := svc.SetDone(ctx, student, school.ID, math, ss[1], true); !errors.Is(err, module.ErrNotFound) {
		t.Fatalf("step of another module: %v", err)
	}
	if err := svc.SetDone(ctx, outsider, school.ID, solar, ss[1], true); !errors.Is(err, org.ErrNotMember) {
		t.Fatalf("outsider: %v", err)
	}
	if err := svc.SetDone(ctx, outsider, other.ID, solar, ss[1], true); !errors.Is(err, module.ErrNotFound) {
		t.Fatalf("cross-org: %v", err)
	}

	// Archiving draft steps cannot remove steps from ongoing pinned revision.
	if err := svc.ArchiveStep(ctx, admin, school.ID, solar, ss[1]); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetDone(ctx, student, school.ID, solar, ss[1], true); err != nil {
		t.Fatalf("archived step: %v", err)
	}
	if r, _ := svc.ResumeFor(ctx, student, school.ID); r == nil || r.Total != 3 || r.Done != 2 || r.NextTitle != "Tata Surya 3" {
		t.Fatalf("resume retains published step: %+v", r)
	}
	if err := svc.SetDone(ctx, student, school.ID, solar, ss[2], true); err != nil {
		t.Fatal(err)
	}
	if r, _ := svc.ResumeFor(ctx, student, school.ID); r != nil {
		t.Fatalf("everything finished, resume = %+v", r)
	}

	// HTTP.
	router := chi.NewRouter()
	tokens := auth.NewTokens(ed25519.NewKeyFromSeed(make([]byte, 32)), time.Now)
	router.Use(auth.RequireUser(tokens, log))
	module.NewHandler(module.NewService(pool, orgs, buildings{"sains": false, "angka": false}, func() time.Time { return now }), log).Routes(router)
	call := func(u uuid.UUID, method, p string) *httptest.ResponseRecorder {
		t.Helper()
		token, err := tokens.Issue(u)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(method, p, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}
	base := fmt.Sprintf("/orgs/%s/modules", school.ID)
	done := fmt.Sprintf("%s/%s/steps/%s/done", base, math, ms[0])
	if w := call(peer, "PUT", done); w.Code != 204 {
		t.Fatalf("http done: %d %s", w.Code, w.Body.String())
	}
	var body struct{ Resume *module.ResumeResponse }
	w := call(peer, "GET", base+"/resume")
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &body) != nil || body.Resume == nil || body.Resume.BuildingKey != "angka" || body.Resume.Done != 1 {
		t.Fatalf("http resume: %d %s", w.Code, w.Body.String())
	}
	if w := call(peer, "DELETE", done); w.Code != 204 {
		t.Fatalf("http undo: %d", w.Code)
	}
	w = call(peer, "GET", base+"/resume")
	if json.Unmarshal(w.Body.Bytes(), &body) != nil || body.Resume != nil {
		t.Fatalf("http resume after undo: %s", w.Body.String())
	}
	if w := call(peer, "PUT", fmt.Sprintf("%s/%s/steps/%s/done", base, draft, ds[0])); w.Code != 404 {
		t.Fatalf("http draft done: %d", w.Code)
	}
	if w := call(outsider, "GET", base+"/resume"); w.Code != 404 {
		t.Fatalf("http outsider resume: %d", w.Code)
	}
	if err := svc.SetDone(ctx, peer, school.ID, math, ms[0], true); err != nil {
		t.Fatal(err)
	}
	if err := svc.Archive(ctx, admin, school.ID, math); err != nil {
		t.Fatal(err)
	}
	if r, err := svc.ResumeFor(ctx, peer, school.ID); err != nil || r != nil {
		t.Fatalf("archived module offered: %+v %v", r, err)
	}
	if err := svc.SetDone(ctx, peer, school.ID, math, ms[0], false); !errors.Is(err, module.ErrNotFound) {
		t.Fatalf("archived module writable: %v", err)
	}
	for range 120 {
		call(peer, "PUT", done)
	}
	if w := call(peer, "PUT", done); w.Code != 429 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("http write rate limit: %d %s", w.Code, w.Body.String())
	}
}
