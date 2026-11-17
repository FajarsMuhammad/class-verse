package module_test

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
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
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/file"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/module"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/org"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/dbtest"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

type buildings map[string]bool

func (b buildings) Building(key string) (bool, bool) {
	staff, ok := b[key]
	return staff, ok
}

// TestModules covers roles, drafts, step kinds, ordering, archive, and file access against real Postgres.
func TestModules(t *testing.T) {
	pool := dbtest.New(t)
	ctx := t.Context()
	clock := func() time.Time { return time.Now().UTC() }
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	q := query.New(pool)
	user := func(email string) uuid.UUID {
		u, err := q.CreateTestUser(ctx, query.CreateTestUserParams{Email: email, DisplayName: email, BirthDate: pgtype.Date{Time: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true}})
		if err != nil {
			t.Fatal(err)
		}
		return u.ID
	}
	admin, student, outsider := user("admin@mod.test"), user("student@mod.test"), user("out@mod.test")
	orgs := org.NewService(org.NewStore(pool), clock, log)
	school, err := orgs.Create(ctx, admin, org.CreateInput{Name: "Mod School", Slug: "mod-school"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := orgs.Create(ctx, outsider, org.CreateInput{Name: "Other", Slug: "mod-other"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO memberships (org_id, user_id, role, joined_at) VALUES ($1, $2, 'participant', now())", school.ID, student); err != nil {
		t.Fatal(err)
	}
	// A ready PDF per school, inserted directly: access rules need metadata only, not object storage.
	readyFile := func(orgID, uploader uuid.UUID) uuid.UUID {
		id := uuid.New()
		if _, err := pool.Exec(ctx, `INSERT INTO file_storage (org_id) VALUES ($1) ON CONFLICT DO NOTHING`, orgID); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO files (id, org_id, uploaded_by, kind, name, content_type, size_bytes, sha256, object_key, status, created_at, updated_at)
			VALUES ($1, $2, $3, 'pdf', 'bab1.pdf', 'application/pdf', 10, $4, $5, 'ready', now(), now())`, id, orgID, uploader, make([]byte, 32), id.String()); err != nil {
			t.Fatal(err)
		}
		return id
	}
	pdf, foreign := readyFile(school.ID, admin), readyFile(other.ID, outsider)

	svc := module.NewService(pool, orgs, buildings{"sains": false, "ruang-guru": true}, clock)
	in := module.Input{BuildingKey: "sains", Level: "SD", Title: "Tata Surya", Description: "Belajar planet"}
	if _, err := svc.Create(ctx, student, school.ID, in); !errors.Is(err, org.ErrForbidden) {
		t.Fatalf("participant create: %v", err)
	}
	if _, err := svc.Create(ctx, outsider, school.ID, in); !errors.Is(err, org.ErrNotMember) {
		t.Fatalf("outsider create: %v", err)
	}
	draft, err := svc.Create(ctx, admin, school.ID, in)
	if err != nil || draft.PublishedAt.Valid {
		t.Fatalf("draft: %+v %v", draft, err)
	}

	add := func(s module.StepInput) uuid.UUID {
		t.Helper()
		st, err := svc.AddStep(ctx, admin, school.ID, draft.ID, s)
		if err != nil {
			t.Fatal(err)
		}
		return st.ID
	}
	text := add(module.StepInput{Kind: "materi", Title: "Matahari", Body: "Matahari adalah bintang.", Minutes: 5})
	reading := add(module.StepInput{Kind: "berkas", Title: "Bacaan", FileID: &pdf, Minutes: 10})
	video := add(module.StepInput{Kind: "video", Title: "8 planet", VideoID: "https://youtu.be/dQw4w9WgXcQ", Minutes: 4})
	if _, err := svc.AddStep(ctx, admin, school.ID, draft.ID, module.StepInput{Kind: "berkas", Title: "Asing", FileID: &foreign, Minutes: 1}); !isField(err, "file_id") {
		t.Fatalf("cross-org file: %v", err)
	}
	if _, err := svc.AddStep(ctx, student, school.ID, draft.ID, module.StepInput{Kind: "materi", Title: "x", Body: "x", Minutes: 1}); !errors.Is(err, org.ErrForbidden) {
		t.Fatalf("participant step: %v", err)
	}

	files := file.NewService(file.NewStore(pool), clock)
	readable := func(u uuid.UUID) error { return files.SavePosition(ctx, u, school.ID, pdf, 1) }

	// Drafts and their files stay hidden from participants.
	list := func(u uuid.UUID, f module.Filter) []module.Module {
		t.Helper()
		mods, err := svc.List(ctx, u, school.ID, f)
		if err != nil {
			t.Fatal(err)
		}
		return mods
	}
	if got := list(student, module.Filter{Drafts: true}); len(got) != 0 {
		t.Fatalf("participant sees draft: %+v", got)
	}
	if err := readable(student); !errors.Is(err, file.ErrNotFound) {
		t.Fatalf("participant reads draft-only file: %v", err)
	}
	staffView := list(admin, module.Filter{Drafts: true})
	if len(staffView) != 1 || len(staffView[0].Steps) != 3 || staffView[0].Steps[2].VideoID.String != "dQw4w9WgXcQ" {
		t.Fatalf("staff view: %+v", staffView)
	}

	// Publish: participants see the path and can read its file.
	in.Published = true
	if _, err := svc.Update(ctx, admin, school.ID, draft.ID, in); err != nil {
		t.Fatal(err)
	}
	if got := list(student, module.Filter{BuildingKey: "sains", Level: "SD"}); len(got) != 1 || len(got[0].Steps) != 3 {
		t.Fatalf("published: %+v", got)
	}
	if got := list(student, module.Filter{BuildingKey: "sains", Level: "SMA"}); len(got) != 0 {
		t.Fatalf("level filter: %+v", got)
	}
	if err := readable(student); err != nil {
		t.Fatalf("participant reads published file: %v", err)
	}
	if st := list(student, module.Filter{})[0].Steps[1]; st.ReaderPosition != 1 || st.FileKind != "pdf" {
		t.Fatalf("reader position/kind: %+v", st)
	}

	// Reorder needs every active step exactly once.
	if err := svc.Reorder(ctx, admin, school.ID, draft.ID, []uuid.UUID{video, text}); !errors.Is(err, module.ErrBadOrdering) {
		t.Fatalf("partial reorder: %v", err)
	}
	if err := svc.Reorder(ctx, admin, school.ID, draft.ID, []uuid.UUID{video, text, text}); !errors.Is(err, module.ErrBadOrdering) {
		t.Fatalf("duplicate reorder: %v", err)
	}
	if err := svc.Reorder(ctx, admin, school.ID, draft.ID, []uuid.UUID{video, reading, text}); err != nil {
		t.Fatal(err)
	}
	order := func() string {
		var titles []string
		for _, s := range list(student, module.Filter{})[0].Steps {
			titles = append(titles, s.Title)
		}
		return strings.Join(titles, ",")
	}
	if got := order(); got != "8 planet,Bacaan,Matahari" {
		t.Fatalf("order: %s", got)
	}

	// The file is in use: it cannot be deleted while a non-archived step points at it.
	if err := file.NewUploader(files, nil).Delete(ctx, admin, school.ID, pdf); !errors.Is(err, file.ErrInUse) {
		t.Fatalf("delete in-use file: %v", err)
	}

	// Archiving a step hides it and its file; a new step goes after the remaining ones.
	if err := svc.ArchiveStep(ctx, admin, school.ID, draft.ID, reading); err != nil {
		t.Fatal(err)
	}
	if err := svc.ArchiveStep(ctx, admin, school.ID, draft.ID, reading); !errors.Is(err, module.ErrNotFound) {
		t.Fatalf("archive step twice: %v", err)
	}
	if err := readable(student); !errors.Is(err, file.ErrNotFound) {
		t.Fatalf("file of archived step still readable: %v", err)
	}
	add(module.StepInput{Kind: "materi", Title: "Bumi", Body: "Rumah kita.", Minutes: 5})
	if got := order(); got != "8 planet,Matahari,Bumi" {
		t.Fatalf("order after archive: %s", got)
	}
	// Updating an archived step or another school's module is not found.
	if _, err := svc.UpdateStep(ctx, admin, school.ID, draft.ID, reading, module.StepInput{Kind: "materi", Title: "x", Body: "x", Minutes: 1}); !errors.Is(err, module.ErrNotFound) {
		t.Fatalf("update archived step: %v", err)
	}
	if _, err := svc.Update(ctx, outsider, other.ID, draft.ID, in); !errors.Is(err, module.ErrNotFound) {
		t.Fatalf("cross-org update: %v", err)
	}

	// HTTP: shapes and status codes the frontend relies on.
	r := chi.NewRouter()
	tokens := auth.NewTokens(ed25519.NewKeyFromSeed(make([]byte, 32)), clock)
	r.Use(auth.RequireUser(tokens, log))
	module.NewHandler(svc, log).Routes(r)
	call := func(u uuid.UUID, method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		token, err := tokens.Issue(u)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	base := fmt.Sprintf("/orgs/%s/modules", school.ID)
	var got struct{ Items []module.Response }
	w := call(student, "GET", base+"?building_key=sains", "")
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &got) != nil || len(got.Items) != 1 || got.Items[0].Steps[0].VideoID == nil {
		t.Fatalf("http list: %d %s", w.Code, w.Body.String())
	}
	w = call(admin, "POST", base+"/"+draft.ID.String()+"/steps", `{"kind":"video","title":"V","video_id":"https://vimeo.com/1","minutes":3}`)
	if w.Code != 400 || !strings.Contains(w.Body.String(), "video_id") {
		t.Fatalf("http bad video: %d %s", w.Code, w.Body.String())
	}
	w = call(admin, "POST", base, `{"building_key":"ruang-guru","level":"SD","title":"X"}`)
	if w.Code != 400 || !strings.Contains(w.Body.String(), "building_key") {
		t.Fatalf("http staff building: %d %s", w.Code, w.Body.String())
	}
	if w := call(student, "DELETE", base+"/"+draft.ID.String(), ""); w.Code != 403 {
		t.Fatalf("http participant archive: %d", w.Code)
	}
	mod := base + "/" + draft.ID.String()
	w = call(admin, "PUT", mod, `{"building_key":"sains","level":"SMP","title":"Tata Surya 2","description":"","published":true}`)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"level":"SMP"`) {
		t.Fatalf("http update: %d %s", w.Code, w.Body.String())
	}
	var step module.StepResponse
	w = call(admin, "POST", mod+"/steps", `{"kind":"materi","title":"Bulan","body":"Satelit Bumi.","minutes":3}`)
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &step) != nil {
		t.Fatalf("http add step: %d %s", w.Code, w.Body.String())
	}
	w = call(admin, "PUT", mod+"/steps/"+step.ID.String(), `{"kind":"video","title":"Bulan","video_id":"dQw4w9WgXcQ","minutes":3}`)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"video_id":"dQw4w9WgXcQ"`) {
		t.Fatalf("http update step: %d %s", w.Code, w.Body.String())
	}
	if w := call(student, "PUT", mod+"/steps/"+step.ID.String(), `{"kind":"materi","title":"x","body":"x","minutes":1}`); w.Code != 403 {
		t.Fatalf("http participant update step: %d", w.Code)
	}
	if w := call(admin, "DELETE", mod+"/steps/"+step.ID.String(), ""); w.Code != 204 {
		t.Fatalf("http archive step: %d", w.Code)
	}
	if w := call(admin, "DELETE", mod+"/steps/"+step.ID.String(), ""); w.Code != 404 {
		t.Fatalf("http archive step twice: %d", w.Code)
	}
	if w := call(admin, "POST", mod+"/steps", `{`); w.Code != 400 {
		t.Fatalf("http bad json: %d", w.Code)
	}
	ordered := list(admin, module.Filter{Drafts: true})[0].Steps
	ids := fmt.Sprintf(`{"step_ids":["%s","%s","%s"]}`, ordered[2].ID, ordered[1].ID, ordered[0].ID)
	if w := call(admin, "PUT", mod+"/order", ids); w.Code != 204 {
		t.Fatalf("http reorder: %d %s", w.Code, w.Body.String())
	}
	if w := call(admin, "PUT", base+"/"+draft.ID.String()+"/order", `{"step_ids":[]}`); w.Code != 400 {
		t.Fatalf("http bad order: %d", w.Code)
	}
	if w := call(admin, "DELETE", base+"/"+draft.ID.String(), ""); w.Code != 204 {
		t.Fatalf("http archive: %d", w.Code)
	}
	if got := list(admin, module.Filter{Drafts: true}); len(got) != 0 {
		t.Fatalf("archived module listed: %+v", got)
	}
	if w := call(admin, "GET", base+"?level=TK", ""); w.Code != 400 {
		t.Fatalf("http bad level: %d", w.Code)
	}
}

func isField(err error, field string) bool {
	var v *module.ValidationError
	return errors.As(err, &v) && v.Fields[field] != ""
}
