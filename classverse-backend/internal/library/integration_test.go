package library_test

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/db/seed"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/auth"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/file"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/library"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/org"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/dbtest"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

// TestCatalog covers roles, drafts, search, paging, file rules and the seed, against real Postgres and storage.
func TestCatalog(t *testing.T) {
	if os.Getenv("TEST_STORAGE_ENDPOINT") == "" {
		t.Skip("TEST_STORAGE_* not configured")
	}
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
	admin, student, outsider := user("admin@lib.test"), user("student@lib.test"), user("out@lib.test")
	orgs := org.NewService(org.NewStore(pool), clock, log)
	school, err := orgs.Create(ctx, admin, org.CreateInput{Name: "Lib School", Slug: "lib-school"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := orgs.Create(ctx, outsider, org.CreateInput{Name: "Other", Slug: "other-school"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO memberships (org_id, user_id, role, joined_at) VALUES ($1, $2, 'participant', now())", school.ID, student); err != nil {
		t.Fatal(err)
	}
	storage, err := file.NewStorage(os.Getenv("TEST_STORAGE_ENDPOINT"), os.Getenv("TEST_STORAGE_BUCKET"), os.Getenv("TEST_STORAGE_ACCESS_KEY"), os.Getenv("TEST_STORAGE_SECRET_KEY"), false)
	if err != nil {
		t.Fatal(err)
	}
	files := file.NewService(file.NewStore(pool), clock)
	uploads := file.NewUploader(files, storage)
	svc := library.NewService(pool, orgs, clock)

	// Seed twice: same book and file, quota charged once.
	sample, err := svc.SeedSample(ctx, uploads, school.ID, seed.TataSurya)
	if err != nil {
		t.Fatal(err)
	}
	again, err := svc.SeedSample(ctx, uploads, school.ID, seed.TataSurya)
	if err != nil || again.ID != sample.ID || !sample.Sample || !sample.PublishedAt.Valid {
		t.Fatalf("seed: %+v %+v %v", sample, again, err)
	}
	usage, err := files.Usage(ctx, admin, school.ID)
	if err != nil || usage.UsedBytes != int64(len(seed.TataSurya)) {
		t.Fatalf("seed quota: %+v %v", usage, err)
	}
	t.Cleanup(func() {
		_ = svc.Archive(ctx, admin, school.ID, sample.ID)
		_ = uploads.Delete(ctx, admin, school.ID, sample.FileID.UUID)
	})

	physical := library.Input{Title: "Atlas 50% Dunia", Author: "Rina", Shelf: "referensi", Level: "SMP", Format: "buku", Copies: 2, Cover: "sejarah"}
	if _, err := svc.Create(ctx, student, school.ID, physical); !errors.Is(err, org.ErrForbidden) {
		t.Fatalf("participant create: %v", err)
	}
	if _, err := svc.Create(ctx, outsider, school.ID, physical); !errors.Is(err, org.ErrNotMember) {
		t.Fatalf("outsider create: %v", err)
	}
	draft, err := svc.Create(ctx, admin, school.ID, physical)
	if err != nil || draft.PublishedAt.Valid {
		t.Fatalf("draft: %+v %v", draft, err)
	}
	// Author is optional (migration 00022).
	anonymous := physical
	anonymous.Author = ""
	if b, err := svc.Create(ctx, admin, school.ID, anonymous); err != nil || b.Author != "" {
		t.Fatalf("anonymous author: %+v %v", b, err)
	}

	// A file from another school or of the wrong kind is rejected.
	foreign := sample.FileID.UUID
	cross := library.Input{Title: "X", Author: "Y", Shelf: "sains", Level: "SD", Format: "ebook", FileID: &foreign, Cover: "ipa"}
	if _, err := svc.Create(ctx, outsider, other.ID, cross); !isField(err, "file_id") {
		t.Fatalf("cross-org file: %v", err)
	}
	cross.Format = "audio"
	if _, err := svc.Create(ctx, admin, school.ID, cross); !isField(err, "file_id") {
		t.Fatalf("pdf as audio: %v", err)
	}

	list := func(u uuid.UUID, f library.Filter) []string {
		t.Helper()
		page, err := svc.List(ctx, u, school.ID, f)
		if err != nil {
			t.Fatal(err)
		}
		titles := []string{}
		for _, b := range page.Items {
			titles = append(titles, b.Title)
		}
		return titles
	}
	if got := list(student, library.Filter{Drafts: true}); fmt.Sprint(got) != "[Contoh: Petualangan di Tata Surya]" {
		t.Fatalf("participant sees drafts: %v", got)
	}
	if got := list(admin, library.Filter{Drafts: true}); len(got) != 2 {
		t.Fatalf("staff drafts: %v", got)
	}
	// Literal % must not match everything.
	if got := list(admin, library.Filter{Drafts: true, Query: "50%"}); fmt.Sprint(got) != "[Atlas 50% Dunia]" {
		t.Fatalf("escaped search: %v", got)
	}
	// "_" would match any single character if not escaped.
	if got := list(admin, library.Filter{Drafts: true, Query: "_tlas"}); len(got) != 0 {
		t.Fatalf("wildcard search: %v", got)
	}
	if got := list(admin, library.Filter{Drafts: true, Shelf: "sains"}); len(got) != 1 {
		t.Fatalf("shelf filter: %v", got)
	}
	if _, err := svc.List(ctx, admin, school.ID, library.Filter{Shelf: "musik"}); !isField(err, "shelf") {
		t.Fatalf("bad shelf: %v", err)
	}
	if _, err := svc.List(ctx, outsider, school.ID, library.Filter{}); !errors.Is(err, org.ErrNotMember) {
		t.Fatalf("outsider list: %v", err)
	}
	if got := list(admin, library.Filter{Drafts: true, Status: "draft"}); len(got) != 1 {
		t.Fatalf("draft filter: %v", got)
	}
	if got := list(student, library.Filter{Drafts: true, Status: "draft"}); len(got) != 0 {
		t.Fatalf("participant draft filter leaks: %v", got)
	}
	if got := list(admin, library.Filter{Drafts: true, Format: "ebook"}); fmt.Sprint(got) != "[Contoh: Petualangan di Tata Surya]" {
		t.Fatalf("format filter: %v", got)
	}
	if _, err := svc.List(ctx, admin, school.ID, library.Filter{Level: "TK"}); !isField(err, "level") {
		t.Fatalf("bad level: %v", err)
	}
	if c, err := svc.Counts(ctx, admin, school.ID, library.Filter{}); err != nil || c.AllCount != 2 || c.PublishedCount != 1 || c.DraftCount != 1 {
		t.Fatalf("counts: %+v %v", c, err)
	}
	if _, err := svc.Counts(ctx, student, school.ID, library.Filter{}); !errors.Is(err, org.ErrForbidden) {
		t.Fatalf("participant counts: %v", err)
	}

	// Paging is stable by title.
	for i := range 3 {
		in := physical
		in.Title, in.Published = fmt.Sprintf("Buku %d", i), true
		if _, err := svc.Create(ctx, admin, school.ID, in); err != nil {
			t.Fatal(err)
		}
	}
	page, err := svc.List(ctx, student, school.ID, library.Filter{Limit: 2})
	if err != nil || len(page.Items) != 2 || page.NextCursor == "" {
		t.Fatalf("page 1: %+v %v", page, err)
	}
	page2, err := svc.List(ctx, student, school.ID, library.Filter{Limit: 2, Cursor: page.NextCursor})
	if err != nil || len(page2.Items) != 2 || page2.NextCursor != "" || page2.Items[0].Title != "Buku 2" {
		t.Fatalf("page 2: %+v %v", page2, err)
	}

	// Participants read a file only through a published book.
	draftEbook := library.Input{Title: "Draf PDF", Author: "Rina", Shelf: "sains", Level: "SD", Format: "ebook", FileID: &foreign, Cover: "ipa"}
	hidden, err := svc.Create(ctx, admin, school.ID, draftEbook)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Archive(ctx, admin, school.ID, sample.ID); err != nil {
		t.Fatal(err)
	}
	r := chi.NewRouter()
	tokens := auth.NewTokens(ed25519.NewKeyFromSeed(make([]byte, 32)), clock)
	r.Use(auth.RequireUser(tokens, log))
	file.NewHandler(uploads, log, clock).Routes(r)
	library.NewHandler(svc, log).Routes(r)
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
	content := fmt.Sprintf("/orgs/%s/files/%s/content", school.ID, foreign)
	if w := call(student, "GET", content, ""); w.Code != 404 {
		t.Fatalf("participant read draft-only file: %d", w.Code)
	}
	if w := call(admin, "GET", content, ""); w.Code != 200 {
		t.Fatalf("staff read: %d", w.Code)
	}
	if w := call(student, "GET", fmt.Sprintf("/orgs/%s/files", school.ID), ""); w.Code != 403 {
		t.Fatalf("participant file manager: %d", w.Code)
	}
	// Publish through the API, then the participant can read.
	body := fmt.Sprintf(`{"title":"Draf PDF","author":"Rina","shelf":"sains","level":"SD","format":"ebook","file_id":"%s","cover":"ipa","published":true}`, foreign)
	books := fmt.Sprintf("/orgs/%s/library/books", school.ID)
	if w := call(student, "PUT", books+"/"+hidden.ID.String(), body); w.Code != 403 {
		t.Fatalf("participant update: %d", w.Code)
	}
	if w := call(admin, "PUT", books+"/"+hidden.ID.String(), body); w.Code != 200 {
		t.Fatalf("publish: %d %s", w.Code, w.Body.String())
	}
	if w := call(student, "GET", content, ""); w.Code != 200 {
		t.Fatalf("participant read published: %d", w.Code)
	}
	// Position shows up in the catalog for the reader.
	if w := call(student, "PUT", fmt.Sprintf("/orgs/%s/files/%s/position", school.ID, foreign), `{"position":2}`); w.Code != 204 {
		t.Fatalf("position: %d", w.Code)
	}
	var got struct {
		Items []library.BookResponse `json:"items"`
	}
	w := call(student, "GET", books+"?q=draf", "")
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil || len(got.Items) != 1 || got.Items[0].Position != 2 || got.Items[0].FileID == nil {
		t.Fatalf("catalog position: %s %v", w.Body.String(), err)
	}
	// A file in use cannot be deleted until its book is archived.
	if w := call(admin, "DELETE", fmt.Sprintf("/orgs/%s/files/%s", school.ID, foreign), ""); w.Code != 409 {
		t.Fatalf("delete in-use file: %d", w.Code)
	}
	if w := call(admin, "DELETE", books+"/"+hidden.ID.String(), ""); w.Code != 204 {
		t.Fatalf("archive: %d", w.Code)
	}
	if w := call(admin, "DELETE", books+"/"+hidden.ID.String(), ""); w.Code != 404 {
		t.Fatalf("archive twice: %d", w.Code)
	}
	if w := call(student, "GET", content, ""); w.Code != 404 {
		t.Fatalf("archived still readable: %d", w.Code)
	}
	if w := call(admin, "GET", books+"?limit=0", ""); w.Code != 400 {
		t.Fatalf("bad limit: %d", w.Code)
	}
	if w := call(admin, "GET", books+"?cursor=@@", ""); w.Code != 400 {
		t.Fatalf("bad cursor: %d", w.Code)
	}
}

func isField(err error, field string) bool {
	var v *library.ValidationError
	return errors.As(err, &v) && v.Fields[field] != ""
}
