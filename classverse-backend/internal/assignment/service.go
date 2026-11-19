// Package assignment issues frozen class tasks with independent, ordered progress.
package assignment

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/org"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

// Domain errors; HTTP maps ownership failures to not found.
var (
	ErrNotFound  = errors.New("assignment or content not found")
	ErrLocked    = errors.New("previous assignment step is incomplete")
	ErrCancelled = errors.New("assignment cancelled")
	ErrConflict  = errors.New("request id already used for different assignment")
	ErrEmpty     = errors.New("assignment needs steps and recipients")
	ErrQuizStep  = errors.New("assignment quiz step requires a passing contextual attempt")
	ErrInvalid   = errors.New("assignment id, class and module are required")
)

// Input uses a client-generated ID to make issuance retry-safe.
type Input struct {
	ID, ClassID, ModuleID uuid.UUID
	DueAt                 *time.Time
}

// Service owns issuance and monotonic assignment progress, not catalog progress.
type Service struct {
	pool *pgxpool.Pool
	auth *org.Service
	now  func() time.Time
}

// NewService wires the existing school authorizer and clock.
func NewService(pool *pgxpool.Pool, auth *org.Service, now func() time.Time) *Service {
	return &Service{pool: pool, auth: auth, now: now}
}
func pgTime(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t.UTC(), Valid: true} }
func (s *Service) inTx(ctx context.Context, fn func(*query.Queries) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin assignment: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(query.New(tx)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit assignment: %w", err)
	}
	return nil
}
func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
func (s *Service) existing(ctx context.Context, q *query.Queries, user, orgID uuid.UUID, in Input) (query.Assignment, error) {
	a, err := q.GetAssignment(ctx, query.GetAssignmentParams{OrgID: orgID, ID: in.ID})
	if err != nil {
		return a, err
	}
	sameDue := in.DueAt == nil && !a.DueAt.Valid || in.DueAt != nil && a.DueAt.Valid && in.DueAt.UTC().Truncate(time.Microsecond).Equal(a.DueAt.Time)
	if a.CreatedBy != user || a.ClassID != in.ClassID || a.ModuleID != in.ModuleID || !sameDue {
		return a, ErrConflict
	}
	return a, nil
}

// Issue freezes one consistent class roster and the current published module steps.
func (s *Service) Issue(ctx context.Context, user, orgID uuid.UUID, in Input) (query.Assignment, error) {
	if _, err := s.auth.Authorize(ctx, user, orgID, org.RoleTeacher); err != nil {
		return query.Assignment{}, err
	}
	if in.ID == uuid.Nil || in.ClassID == uuid.Nil || in.ModuleID == uuid.Nil {
		return query.Assignment{}, ErrInvalid
	}
	var a query.Assignment
	err := s.inTx(ctx, func(q *query.Queries) error {
		// Reuse request-scoped advisory lock: no rows exist to lock on the first retry race.
		if err := q.LockAssignmentRequest(ctx, query.LockAssignmentRequestParams{OrgID: orgID, RequestID: in.ID}); err != nil {
			return err
		}
		var err error
		a, err = s.existing(ctx, q, user, orgID, in)
		if err == nil || errors.Is(err, ErrConflict) {
			return err
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		c, err := q.GetAssignmentClass(ctx, query.GetAssignmentClassParams{OrgID: orgID, ID: in.ClassID})
		if err != nil {
			return notFound(err)
		}
		if err := q.LockAssignmentRoster(ctx, query.LockAssignmentRosterParams{OrgID: orgID, YearID: c.AcademicYearID}); err != nil {
			return err
		}
		if _, err := q.LockClass(ctx, query.LockClassParams{OrgID: orgID, ID: c.ID}); err != nil {
			return notFound(err)
		}
		if _, err := q.LockModule(ctx, query.LockModuleParams{OrgID: orgID, ID: in.ModuleID}); err != nil {
			return notFound(err)
		}
		if _, err := q.LockAssignmentFileReferences(ctx, query.LockAssignmentFileReferencesParams{OrgID: orgID, ModuleID: in.ModuleID}); err != nil {
			return fmt.Errorf("lock snapshot files: %w", err)
		}
		due := pgtype.Timestamptz{}
		if in.DueAt != nil {
			due = pgTime(*in.DueAt)
		}
		a, err = q.CreateAssignment(ctx, query.CreateAssignmentParams{ID: in.ID, OrgID: orgID, ClassID: in.ClassID, ModuleID: in.ModuleID, CreatedBy: user, CreatedAt: pgTime(s.now()), DueAt: due})
		if err != nil {
			return notFound(err)
		}
		n, err := q.SnapshotAssignmentSteps(ctx, query.SnapshotAssignmentStepsParams{AssignmentID: a.ID, OrgID: orgID, ModuleID: in.ModuleID})
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrEmpty
		}
		n, err = q.SnapshotAssignmentRecipients(ctx, query.SnapshotAssignmentRecipientsParams{AssignmentID: a.ID, OrgID: orgID, ClassID: in.ClassID})
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrEmpty
		}
		return nil
	})
	return a, err
}

// List returns own tasks for students, all school tasks for staff.
func (s *Service) List(ctx context.Context, user, orgID uuid.UUID, page ...int32) ([]query.ListAssignmentsRow, error) {
	return s.ListFiltered(ctx, user, orgID, "", uuid.NullUUID{}, page...)
}

// ListFiltered applies search before selecting a page, preserving recipient authorization.
func (s *Service) ListFiltered(ctx context.Context, user, orgID uuid.UUID, search string, classID uuid.NullUUID, page ...int32) ([]query.ListAssignmentsRow, error) {
	pageLimit := pgtype.Int4{}
	pageOffset := int32(0)
	if len(page) == 2 {
		pageLimit = pgtype.Int4{Int32: page[0], Valid: true}
		pageOffset = page[1]
	}

	role, err := s.auth.Authorize(ctx, user, orgID, org.RoleParticipant)
	if err != nil {
		return nil, err
	}
	return query.New(s.pool).ListAssignments(ctx, query.ListAssignmentsParams{Search: search, ClassID: classID, PageLimit: pageLimit, PageOffset: pageOffset, OrgID: orgID, UserID: user, Staff: role.AtLeast(org.RoleTeacher)})
}
func (s *Service) accessible(ctx context.Context, q *query.Queries, user, orgID, id uuid.UUID, role org.Role) (query.Assignment, error) {
	a, err := q.GetAssignment(ctx, query.GetAssignmentParams{OrgID: orgID, ID: id})
	if err != nil {
		return a, notFound(err)
	}
	if !role.AtLeast(org.RoleTeacher) {
		_, err = q.OwnAssignmentRecipient(ctx, query.OwnAssignmentRecipientParams{OrgID: orgID, AssignmentID: id, UserID: user})
	}
	return a, notFound(err)
}

// Steps returns metadata for locked steps, but never their content or referenced IDs.
func (s *Service) Steps(ctx context.Context, user, orgID, id uuid.UUID) ([]query.AssignmentStepsRow, error) {
	role, err := s.auth.Authorize(ctx, user, orgID, org.RoleParticipant)
	if err != nil {
		return nil, err
	}
	q := query.New(s.pool)
	if _, err := s.accessible(ctx, q, user, orgID, id, role); err != nil {
		return nil, err
	}
	rows, err := q.AssignmentSteps(ctx, query.AssignmentStepsParams{OrgID: orgID, AssignmentID: id, UserID: user})
	if err != nil {
		return nil, err
	}
	for i, r := range rows {
		if !r.Unlocked && !role.AtLeast(org.RoleTeacher) {
			rows[i].Body = ""
			rows[i].RichContent = nil
			rows[i].QuizRevisionID = uuid.NullUUID{}
			rows[i].FileID = uuid.NullUUID{}
			rows[i].VideoID = pgtype.Text{}
			rows[i].QuizID = uuid.NullUUID{}
		}
	}
	return rows, nil
}

// Complete marks a self-reported non-quiz step complete, strictly in order; repeating is harmless.
func (s *Service) Complete(ctx context.Context, user, orgID, id, stepID uuid.UUID) error {
	if _, err := s.auth.Authorize(ctx, user, orgID, org.RoleParticipant); err != nil {
		return err
	}
	return s.inTx(ctx, func(q *query.Queries) error {
		a, err := q.LockAssignment(ctx, query.LockAssignmentParams{OrgID: orgID, ID: id})
		if err != nil {
			return notFound(err)
		}
		if _, err := q.OwnAssignmentRecipient(ctx, query.OwnAssignmentRecipientParams{OrgID: orgID, AssignmentID: id, UserID: user}); err != nil {
			return notFound(err)
		}
		if a.CancelledAt.Valid {
			return ErrCancelled
		}
		rows, err := q.AssignmentSteps(ctx, query.AssignmentStepsParams{OrgID: orgID, AssignmentID: id, UserID: user})
		if err != nil {
			return err
		}
		for _, r := range rows {
			if r.ID == stepID {
				if !r.Unlocked {
					return ErrLocked
				}
				if r.Kind == "kuis" {
					return ErrQuizStep
				}
				return q.CompleteAssignmentStep(ctx, query.CompleteAssignmentStepParams{OrgID: orgID, AssignmentID: id, UserID: user, StepID: stepID, CompletedAt: pgTime(s.now())})
			}
		}
		return ErrNotFound
	})
}

// Cancel stops new work without deleting history.
func (s *Service) Cancel(ctx context.Context, user, orgID, id uuid.UUID) error {
	if _, err := s.auth.Authorize(ctx, user, orgID, org.RoleTeacher); err != nil {
		return err
	}
	return s.inTx(ctx, func(q *query.Queries) error {
		if _, err := q.LockAssignment(ctx, query.LockAssignmentParams{OrgID: orgID, ID: id}); err != nil {
			return notFound(err)
		}
		return q.CancelAssignment(ctx, query.CancelAssignmentParams{OrgID: orgID, ID: id, CancelledAt: pgTime(s.now())})
	})
}

// Roster exposes frozen recipient progress to staff only.
func (s *Service) Roster(ctx context.Context, user, orgID, id uuid.UUID) ([]query.AssignmentRosterRow, error) {
	if _, err := s.auth.Authorize(ctx, user, orgID, org.RoleTeacher); err != nil {
		return nil, err
	}
	q := query.New(s.pool)
	if _, err := q.GetAssignment(ctx, query.GetAssignmentParams{OrgID: orgID, ID: id}); err != nil {
		return nil, notFound(err)
	}
	return q.AssignmentRoster(ctx, query.AssignmentRosterParams{OrgID: orgID, AssignmentID: id})
}
