package library

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/org"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

// Loan rules (L2, approved 2026-11-12). No queue and no fines.
const (
	MaxActiveLoans = 5
	LoanDuration   = 14 * 24 * time.Hour
)

// Loan errors.
var (
	ErrNotLoanable  = errors.New("only physical books can be borrowed")
	ErrNoCopies     = errors.New("no copies available")
	ErrLoanLimit    = errors.New("active loan limit reached")
	ErrCopiesInUse  = errors.New("copies below active loans")
	ErrLoanNotFound = errors.New("loan not found")
)

// Loan is one active or returned loan with the book's display fields.
type Loan struct {
	query.LibraryLoan
	Title  string
	Author string
	Cover  string
}

// Borrow lends one copy of a published physical book to the caller.
// A retry while the loan is active returns the same loan instead of a second one.
func (s *Service) Borrow(ctx context.Context, userID, orgID, bookID uuid.UUID) (query.LibraryLoan, error) {
	var loan query.LibraryLoan
	err := s.inTx(ctx, func(q *query.Queries) error {
		// Lock order is fixed (membership, then book) so concurrent borrows cannot deadlock.
		if _, err := q.LockMembership(ctx, query.LockMembershipParams{OrgID: orgID, UserID: userID}); errors.Is(err, pgx.ErrNoRows) {
			return org.ErrNotMember
		} else if err != nil {
			return fmt.Errorf("lock membership: %w", err)
		}
		book, err := q.LockBook(ctx, query.LockBookParams{OrgID: orgID, ID: bookID})
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && !book.PublishedAt.Valid) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("lock book: %w", err)
		}
		if book.Format != "buku" {
			return ErrNotLoanable
		}
		loan, err = q.ActiveLoan(ctx, query.ActiveLoanParams{BookID: bookID, UserID: userID})
		if err == nil {
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("get active loan: %w", err)
		}
		mine, err := q.CountMemberLoans(ctx, query.CountMemberLoansParams{OrgID: orgID, UserID: userID})
		if err != nil {
			return fmt.Errorf("count member loans: %w", err)
		}
		if mine >= MaxActiveLoans {
			return ErrLoanLimit
		}
		out, err := q.CountBookLoans(ctx, bookID)
		if err != nil {
			return fmt.Errorf("count book loans: %w", err)
		}
		if out >= book.Copies {
			return ErrNoCopies
		}
		now := s.now()
		loan, err = q.CreateLoan(ctx, query.CreateLoanParams{
			ID: uuid.New(), OrgID: orgID, BookID: bookID, UserID: userID,
			BorrowedAt: pgTime(now), DueAt: pgTime(now.Add(LoanDuration)),
		})
		if err != nil {
			return fmt.Errorf("create loan: %w", err)
		}
		return nil
	})
	return loan, err
}

// Return ends the caller's own loan. Returning twice is a no-op.
func (s *Service) Return(ctx context.Context, userID, orgID, loanID uuid.UUID) (query.LibraryLoan, error) {
	if _, err := s.auth.Authorize(ctx, userID, orgID, org.RoleParticipant); err != nil {
		return query.LibraryLoan{}, err
	}
	loan, err := query.New(s.pool).ReturnLoan(ctx, query.ReturnLoanParams{OrgID: orgID, ID: loanID, UserID: userID, Now: pgTime(s.now())})
	if errors.Is(err, pgx.ErrNoRows) {
		return query.LibraryLoan{}, ErrLoanNotFound
	}
	if err != nil {
		return query.LibraryLoan{}, fmt.Errorf("return loan: %w", err)
	}
	return loan, nil
}

// Loans lists the caller's active loans, soonest due first (at most MaxActiveLoans).
func (s *Service) Loans(ctx context.Context, userID, orgID uuid.UUID) ([]Loan, error) {
	if _, err := s.auth.Authorize(ctx, userID, orgID, org.RoleParticipant); err != nil {
		return nil, err
	}
	rows, err := query.New(s.pool).ListMemberLoans(ctx, query.ListMemberLoansParams{OrgID: orgID, UserID: userID})
	if err != nil {
		return nil, fmt.Errorf("list loans: %w", err)
	}
	out := make([]Loan, 0, len(rows))
	for _, r := range rows {
		out = append(out, Loan{LibraryLoan: r.LibraryLoan, Title: r.Title, Author: r.Author, Cover: r.Cover})
	}
	return out, nil
}

// checkCopies rejects lowering copies (or turning a physical book digital) below its active loans.
// The caller holds the book row lock, so no borrow can slip in between.
func checkCopies(ctx context.Context, q *query.Queries, bookID uuid.UUID, copies int) error {
	out, err := q.CountBookLoans(ctx, bookID)
	if err != nil {
		return fmt.Errorf("count book loans: %w", err)
	}
	if int32(copies) < out {
		return ErrCopiesInUse
	}
	return nil
}

func dueTime(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}
