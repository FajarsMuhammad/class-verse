package library_test

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/auth"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/library"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/org"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/dbtest"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

// TestLoans covers stock, the per-member limit, retries, returns, and isolation against real Postgres.
func TestLoans(t *testing.T) {
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
	admin, outsider := user("admin@loan.test"), user("out@loan.test")
	students := make([]uuid.UUID, 8)
	for i := range students {
		students[i] = user(fmt.Sprintf("s%d@loan.test", i))
	}
	orgs := org.NewService(org.NewStore(pool), clock, log)
	school, err := orgs.Create(ctx, admin, org.CreateInput{Name: "Loan School", Slug: "loan-school"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := orgs.Create(ctx, outsider, org.CreateInput{Name: "Other", Slug: "loan-other"})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range students {
		if _, err := pool.Exec(ctx, "INSERT INTO memberships (org_id, user_id, role, joined_at) VALUES ($1, $2, 'participant', now())", school.ID, s); err != nil {
			t.Fatal(err)
		}
	}
	svc := library.NewService(pool, orgs, clock)
	book := func(title string, copies int, published bool) uuid.UUID {
		t.Helper()
		b, err := svc.Create(ctx, admin, school.ID, library.Input{Title: title, Author: "A", Shelf: "novel", Level: "SMA", Format: "buku", Copies: copies, Cover: "bahasa", Published: published})
		if err != nil {
			t.Fatal(err)
		}
		return b.ID
	}
	// s0 stays out of the races so later assertions know its exact loans.
	s0, racers := students[0], students[1:]

	// Last copy: many members race for one copy; exactly one wins.
	last := book("Satu Eksemplar", 1, true)
	var wg sync.WaitGroup
	results := make(chan error, len(racers))
	for _, s := range racers {
		wg.Go(func() {
			_, err := svc.Borrow(ctx, s, school.ID, last)
			results <- err
		})
	}
	wg.Wait()
	close(results)
	won := 0
	for err := range results {
		switch {
		case err == nil:
			won++
		case !errors.Is(err, library.ErrNoCopies):
			t.Fatalf("race error: %v", err)
		}
	}
	if won != 1 {
		t.Fatalf("last copy borrowed %d times", won)
	}

	// Retry returns the same loan; one member cannot hold two copies of one title.
	twin := book("Dua Eksemplar", 2, true)
	a, err := svc.Borrow(ctx, s0, school.ID, twin)
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.Borrow(ctx, s0, school.ID, twin)
	if err != nil || b.ID != a.ID {
		t.Fatalf("retry borrow: %v %v", b.ID, err)
	}
	if d := a.DueAt.Time.Sub(a.BorrowedAt.Time); d != library.LoanDuration {
		t.Fatalf("due after %v", d)
	}

	// Per-member limit holds when one member borrows different books concurrently.
	// A member outside the race above, so the count starts at zero.
	limiter := user("limit@loan.test")
	if _, err := pool.Exec(ctx, "INSERT INTO memberships (org_id, user_id, role, joined_at) VALUES ($1, $2, 'participant', now())", school.ID, limiter); err != nil {
		t.Fatal(err)
	}
	ids := make([]uuid.UUID, library.MaxActiveLoans+3)
	for i := range ids {
		ids[i] = book(fmt.Sprintf("Batas %d", i), 3, true)
	}
	results = make(chan error, len(ids))
	for _, id := range ids {
		wg.Go(func() {
			_, err := svc.Borrow(ctx, limiter, school.ID, id)
			results <- err
		})
	}
	wg.Wait()
	close(results)
	ok, limited := 0, 0
	for err := range results {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, library.ErrLoanLimit):
			limited++
		default:
			t.Fatalf("limit error: %v", err)
		}
	}
	mine, err := svc.Loans(ctx, limiter, school.ID)
	if err != nil || ok != library.MaxActiveLoans || limited != 3 || len(mine) != library.MaxActiveLoans {
		t.Fatalf("limit: ok=%d limited=%d loans=%d %v", ok, limited, len(mine), err)
	}

	// Return is idempotent, frees the copy, and only the borrower can return.
	if _, err := svc.Return(ctx, students[2], school.ID, a.ID); !errors.Is(err, library.ErrLoanNotFound) {
		t.Fatalf("return someone else's loan: %v", err)
	}
	r1, err := svc.Return(ctx, s0, school.ID, a.ID)
	if err != nil || !r1.ReturnedAt.Valid {
		t.Fatalf("return: %v", err)
	}
	r2, err := svc.Return(ctx, s0, school.ID, a.ID)
	if err != nil || !r2.ReturnedAt.Time.Equal(r1.ReturnedAt.Time) {
		t.Fatalf("return twice changed time: %v", err)
	}
	again, err := svc.Borrow(ctx, s0, school.ID, twin)
	if err != nil || again.ID == a.ID {
		t.Fatalf("borrow after return: %v", err)
	}

	// Catalog shows availability and the caller's loan.
	page, err := svc.List(ctx, s0, school.ID, library.Filter{Query: "Dua Eksemplar"})
	if err != nil || len(page.Items) != 1 || page.Items[0].OnLoan != 1 || page.Items[0].MyLoan == nil || page.Items[0].MyLoan.ID != again.ID {
		t.Fatalf("catalog loan state: %+v %v", page.Items, err)
	}

	// Rules: digital and draft books are not loanable; outsiders and other schools are rejected.
	draft := book("Draf", 3, false)
	if _, err := svc.Borrow(ctx, s0, school.ID, draft); !errors.Is(err, library.ErrNotFound) {
		t.Fatalf("draft borrow: %v", err)
	}
	if _, err := svc.Borrow(ctx, outsider, school.ID, twin); !errors.Is(err, org.ErrNotMember) {
		t.Fatalf("outsider borrow: %v", err)
	}
	if _, err := svc.Borrow(ctx, outsider, other.ID, twin); !errors.Is(err, library.ErrNotFound) {
		t.Fatalf("cross-org borrow: %v", err)
	}
	if _, err := svc.Return(ctx, outsider, other.ID, again.ID); !errors.Is(err, library.ErrLoanNotFound) {
		t.Fatalf("cross-org return: %v", err)
	}

	// Copies cannot drop below active loans, nor can the book turn digital while lent.
	in := library.Input{Title: "Dua Eksemplar", Author: "A", Shelf: "novel", Level: "SMA", Format: "buku", Copies: 0, Cover: "bahasa", Published: true}
	if _, err := svc.Update(ctx, admin, school.ID, twin, in); !errors.Is(err, library.ErrCopiesInUse) {
		t.Fatalf("copies below loans: %v", err)
	}
	in.Copies = 1
	if _, err := svc.Update(ctx, admin, school.ID, twin, in); err != nil {
		t.Fatalf("copies equal to loans: %v", err)
	}
	if _, err := svc.Borrow(ctx, students[3], school.ID, twin); !errors.Is(err, library.ErrNoCopies) {
		t.Fatalf("borrow beyond lowered copies: %v", err)
	}

	// Archived books stay in Pinjamanku so they can still be returned.
	if err := svc.Archive(ctx, admin, school.ID, twin); err != nil {
		t.Fatal(err)
	}
	loans, err := svc.Loans(ctx, s0, school.ID)
	if err != nil || len(loans) != 1 || loans[0].Title != "Dua Eksemplar" {
		t.Fatalf("loans after archive: %+v %v", loans, err)
	}
	if _, err := svc.Return(ctx, s0, school.ID, again.ID); err != nil {
		t.Fatalf("return archived: %v", err)
	}

	// HTTP: status codes and response shapes the frontend relies on.
	r := chi.NewRouter()
	tokens := auth.NewTokens(ed25519.NewKeyFromSeed(make([]byte, 32)), clock)
	r.Use(auth.RequireUser(tokens, log))
	library.NewHandler(svc, log).Routes(r)
	call := func(u uuid.UUID, method, path string) *httptest.ResponseRecorder {
		t.Helper()
		token, err := tokens.Issue(u)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(method, path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	errCode := func(w *httptest.ResponseRecorder) string {
		var body struct{ Error struct{ Code string } }
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		return body.Error.Code
	}
	base := fmt.Sprintf("/orgs/%s/library", school.ID)
	solo := book("HTTP", 1, true)
	w := call(s0, "POST", fmt.Sprintf("%s/books/%s/loans", base, solo))
	var lent library.MyLoan
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &lent) != nil || lent.ID == uuid.Nil {
		t.Fatalf("http borrow: %d %s", w.Code, w.Body.String())
	}
	if w := call(students[4], "POST", fmt.Sprintf("%s/books/%s/loans", base, solo)); w.Code != 409 || errCode(w) != "no_copies" {
		t.Fatalf("http no copies: %d %s", w.Code, w.Body.String())
	}
	if w := call(limiter, "POST", fmt.Sprintf("%s/books/%s/loans", base, book("Lagi", 1, true))); w.Code != 409 || errCode(w) != "loan_limit" {
		t.Fatalf("http limit: %d %s", w.Code, w.Body.String())
	}
	var list struct {
		Items     []library.LoanResponse `json:"items"`
		MaxActive int                    `json:"max_active"`
	}
	w = call(s0, "GET", base+"/loans")
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &list) != nil || len(list.Items) != 1 || list.MaxActive != library.MaxActiveLoans {
		t.Fatalf("http loans: %d %s", w.Code, w.Body.String())
	}
	var books struct{ Items []library.BookResponse }
	w = call(students[4], "GET", base+"/books?q=HTTP")
	if json.Unmarshal(w.Body.Bytes(), &books) != nil || len(books.Items) != 1 || books.Items[0].Available != 0 || books.Items[0].MyLoan != nil {
		t.Fatalf("http catalog for others: %s", w.Body.String())
	}
	if w := call(students[4], "POST", fmt.Sprintf("%s/loans/%s/return", base, lent.ID)); w.Code != 404 {
		t.Fatalf("http return other's loan: %d", w.Code)
	}
	for range 2 {
		if w := call(s0, "POST", fmt.Sprintf("%s/loans/%s/return", base, lent.ID)); w.Code != 204 {
			t.Fatalf("http return: %d %s", w.Code, w.Body.String())
		}
	}
	if w := call(outsider, "GET", base+"/loans"); w.Code != 404 { // non-members do not learn the org exists
		t.Fatalf("http outsider loans: %d", w.Code)
	}
	if w := call(s0, "POST", fmt.Sprintf("%s/books/%s/loans", base, "nope")); w.Code != 404 { // malformed IDs are treated as not found (org.PathUUID)
		t.Fatalf("http bad id: %d", w.Code)
	}
}
