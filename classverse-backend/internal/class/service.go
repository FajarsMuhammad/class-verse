// Package class manages classes (rombel) per school and academic year (C1).
//
// Approved rules: at most one active class per student per school and academic year; teachers and admins
// manage classes and placements, only admins choose the active year; history is kept and nobody is promoted
// or placed automatically. Classes guide reports and labels; they do not restrict the open module catalog.
package class

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/org"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

// Domain errors.
var (
	ErrNotFound      = errors.New("year or class not found")
	ErrNotStudent    = errors.New("only participants of the school can be placed")
	ErrClassNotEmpty = errors.New("class still has students")
)

// ValidationError lists invalid input fields.
type ValidationError struct{ Fields map[string]string }

func (e *ValidationError) Error() string { return "invalid class fields" }

// Authorizer checks a user's role in an organization (implemented by org.Service).
type Authorizer interface {
	Authorize(ctx context.Context, userID, orgID uuid.UUID, minRole org.Role) (org.Role, error)
}

// Year is an academic year of a school.
type Year struct {
	ID     uuid.UUID
	Label  string
	Active bool
}

// Class is an active class with its number of placed students.
type Class struct {
	ID       uuid.UUID
	YearID   uuid.UUID
	Name     string
	Students int32
}

// RosterEntry is a participant of the school and their class in the selected year (nil: not placed).
type RosterEntry struct {
	UserID      uuid.UUID
	DisplayName string
	AvatarKey   string
	ClassID     *uuid.UUID
}

// Placement is one row of a student's class history.
type Placement struct {
	ClassName string
	YearLabel string
	StartedAt time.Time
	EndedAt   *time.Time // nil: still in this class
}

// Service implements class rules.
type Service struct {
	pool *pgxpool.Pool
	auth Authorizer
	now  func() time.Time
}

// NewService wires the class service.
func NewService(pool *pgxpool.Pool, auth Authorizer, now func() time.Time) *Service {
	return &Service{pool: pool, auth: auth, now: now}
}

var yearLabel = regexp.MustCompile(`^(\d{4})/(\d{4})$`)

func validLabel(s string) (string, bool) {
	s = strings.TrimSpace(s)
	m := yearLabel.FindStringSubmatch(s)
	if m == nil {
		return s, false
	}
	a, _ := strconv.Atoi(m[1])
	b, _ := strconv.Atoi(m[2])
	return s, b == a+1
}

func validName(s string) (string, bool) {
	s = strings.Join(strings.Fields(strings.ToValidUTF8(s, "")), " ")
	n := utf8.RuneCountInString(s)
	return s, n >= 1 && n <= 40 && !strings.ContainsFunc(s, unicode.IsControl)
}

func pgTime(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t.UTC(), Valid: true} }

// uniqueViolation reports a unique constraint violation (SQLSTATE 23505).
func uniqueViolation(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505"
}

func (s *Service) inTx(ctx context.Context, fn func(*query.Queries) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin class transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(query.New(tx)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit class transaction: %w", err)
	}
	return nil
}

// year loads a year of this school; another school's year is not found.
func year(ctx context.Context, q *query.Queries, orgID, id uuid.UUID) (query.AcademicYear, error) {
	y, err := q.GetAcademicYear(ctx, query.GetAcademicYearParams{OrgID: orgID, ID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return y, ErrNotFound
	}
	if err != nil {
		return y, fmt.Errorf("get academic year: %w", err)
	}
	return y, nil
}

// Years lists the school's academic years, newest first (teacher/admin).
func (s *Service) Years(ctx context.Context, userID, orgID uuid.UUID) ([]Year, error) {
	if _, err := s.auth.Authorize(ctx, userID, orgID, org.RoleTeacher); err != nil {
		return nil, err
	}
	rows, err := query.New(s.pool).ListAcademicYears(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list academic years: %w", err)
	}
	out := make([]Year, len(rows))
	for i, y := range rows {
		out[i] = Year{ID: y.ID, Label: y.Label, Active: y.Active}
	}
	return out, nil
}

// CreateYear adds an academic year like "2026/2027" (admin). The school's first year becomes active.
func (s *Service) CreateYear(ctx context.Context, userID, orgID uuid.UUID, label string) (Year, error) {
	if _, err := s.auth.Authorize(ctx, userID, orgID, org.RoleAdmin); err != nil {
		return Year{}, err
	}
	label, ok := validLabel(label)
	if !ok {
		return Year{}, &ValidationError{Fields: map[string]string{"label": "Tulis tahun ajaran seperti 2026/2027"}}
	}
	var y query.AcademicYear
	err := s.inTx(ctx, func(q *query.Queries) error {
		if err := q.LockOrgYears(ctx, orgID); err != nil {
			return fmt.Errorf("lock academic years: %w", err)
		}
		n, err := q.CountAcademicYears(ctx, orgID)
		if err != nil {
			return fmt.Errorf("count academic years: %w", err)
		}
		y, err = q.CreateAcademicYear(ctx, query.CreateAcademicYearParams{ID: uuid.New(), OrgID: orgID, Label: label, Active: n == 0, CreatedAt: pgTime(s.now())})
		if uniqueViolation(err) {
			return &ValidationError{Fields: map[string]string{"label": "Tahun ajaran ini sudah ada"}}
		}
		if err != nil {
			return fmt.Errorf("create academic year: %w", err)
		}
		return nil
	})
	return Year{ID: y.ID, Label: y.Label, Active: y.Active}, err
}

// Activate makes a year the school's active one (admin). Older years stay available.
func (s *Service) Activate(ctx context.Context, userID, orgID, yearID uuid.UUID) error {
	if _, err := s.auth.Authorize(ctx, userID, orgID, org.RoleAdmin); err != nil {
		return err
	}
	return s.inTx(ctx, func(q *query.Queries) error {
		if err := q.LockOrgYears(ctx, orgID); err != nil {
			return fmt.Errorf("lock academic years: %w", err)
		}
		if _, err := year(ctx, q, orgID, yearID); err != nil {
			return err
		}
		if err := q.ClearActiveYear(ctx, orgID); err != nil {
			return fmt.Errorf("clear active year: %w", err)
		}
		if _, err := q.SetActiveYear(ctx, query.SetActiveYearParams{OrgID: orgID, ID: yearID}); err != nil {
			return fmt.Errorf("set active year: %w", err)
		}
		return nil
	})
}

// Classes lists the active classes of a year with their student counts (teacher/admin).
func (s *Service) Classes(ctx context.Context, userID, orgID, yearID uuid.UUID) ([]Class, error) {
	if _, err := s.auth.Authorize(ctx, userID, orgID, org.RoleTeacher); err != nil {
		return nil, err
	}
	q := query.New(s.pool)
	if _, err := year(ctx, q, orgID, yearID); err != nil {
		return nil, err
	}
	rows, err := q.ListClasses(ctx, query.ListClassesParams{OrgID: orgID, AcademicYearID: yearID})
	if err != nil {
		return nil, fmt.Errorf("list classes: %w", err)
	}
	out := make([]Class, len(rows))
	for i, c := range rows {
		out[i] = Class{ID: c.ID, YearID: c.AcademicYearID, Name: c.Name, Students: c.Students}
	}
	return out, nil
}

func nameTaken(err error) error {
	if uniqueViolation(err) {
		return &ValidationError{Fields: map[string]string{"name": "Nama kelas sudah dipakai di tahun ajaran ini"}}
	}
	return nil
}

// CreateClass adds a class to a year of this school (teacher/admin).
func (s *Service) CreateClass(ctx context.Context, userID, orgID, yearID uuid.UUID, name string) (Class, error) {
	if _, err := s.auth.Authorize(ctx, userID, orgID, org.RoleTeacher); err != nil {
		return Class{}, err
	}
	name, ok := validName(name)
	if !ok {
		return Class{}, &ValidationError{Fields: map[string]string{"name": "Nama kelas 1–40 karakter"}}
	}
	q := query.New(s.pool)
	if _, err := year(ctx, q, orgID, yearID); err != nil {
		return Class{}, err
	}
	c, err := q.CreateClass(ctx, query.CreateClassParams{ID: uuid.New(), OrgID: orgID, AcademicYearID: yearID, Name: name, CreatedBy: userID, CreatedAt: pgTime(s.now())})
	if taken := nameTaken(err); taken != nil {
		return Class{}, taken
	}
	if err != nil {
		return Class{}, fmt.Errorf("create class: %w", err)
	}
	return Class{ID: c.ID, YearID: c.AcademicYearID, Name: c.Name}, nil
}

// RenameClass changes a class name (teacher/admin).
func (s *Service) RenameClass(ctx context.Context, userID, orgID, classID uuid.UUID, name string) (Class, error) {
	if _, err := s.auth.Authorize(ctx, userID, orgID, org.RoleTeacher); err != nil {
		return Class{}, err
	}
	name, ok := validName(name)
	if !ok {
		return Class{}, &ValidationError{Fields: map[string]string{"name": "Nama kelas 1–40 karakter"}}
	}
	c, err := query.New(s.pool).RenameClass(ctx, query.RenameClassParams{OrgID: orgID, ID: classID, Name: name})
	if errors.Is(err, pgx.ErrNoRows) {
		return Class{}, ErrNotFound
	}
	if taken := nameTaken(err); taken != nil {
		return Class{}, taken
	}
	if err != nil {
		return Class{}, fmt.Errorf("rename class: %w", err)
	}
	return Class{ID: c.ID, YearID: c.AcademicYearID, Name: c.Name, Students: c.Students}, nil
}

// ArchiveClass hides an empty class (teacher/admin). History rows keep pointing at it.
func (s *Service) ArchiveClass(ctx context.Context, userID, orgID, classID uuid.UUID) error {
	if _, err := s.auth.Authorize(ctx, userID, orgID, org.RoleTeacher); err != nil {
		return err
	}
	return s.inTx(ctx, func(q *query.Queries) error {
		// Placements lock the class too, so nobody joins between the count and the archive.
		if _, err := q.LockClass(ctx, query.LockClassParams{OrgID: orgID, ID: classID}); errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return fmt.Errorf("lock class: %w", err)
		}
		n, err := q.CountActiveEnrollments(ctx, classID)
		if err != nil {
			return fmt.Errorf("count students: %w", err)
		}
		if n > 0 {
			return ErrClassNotEmpty
		}
		if err := q.ArchiveClass(ctx, query.ArchiveClassParams{OrgID: orgID, ID: classID, ArchivedAt: pgTime(s.now())}); err != nil {
			return fmt.Errorf("archive class: %w", err)
		}
		return nil
	})
}

// Roster lists every participant of the school with their class in the year (teacher/admin).
func (s *Service) Roster(ctx context.Context, userID, orgID, yearID uuid.UUID) ([]RosterEntry, error) {
	if _, err := s.auth.Authorize(ctx, userID, orgID, org.RoleTeacher); err != nil {
		return nil, err
	}
	q := query.New(s.pool)
	if _, err := year(ctx, q, orgID, yearID); err != nil {
		return nil, err
	}
	rows, err := q.ListRoster(ctx, query.ListRosterParams{OrgID: orgID, AcademicYearID: yearID})
	if err != nil {
		return nil, fmt.Errorf("list roster: %w", err)
	}
	out := make([]RosterEntry, len(rows))
	for i, r := range rows {
		out[i] = RosterEntry{UserID: r.UserID, DisplayName: r.DisplayName, AvatarKey: r.AvatarKey}
		if r.ClassID.Valid {
			id := r.ClassID.UUID
			out[i].ClassID = &id
		}
	}
	return out, nil
}

// Place puts a participant into a class of the year, moving them if they already have one; classID nil
// removes them from their class ("Belum ditempatkan"). One transaction; the old placement stays as history.
func (s *Service) Place(ctx context.Context, userID, orgID, yearID, studentID uuid.UUID, classID *uuid.UUID) error {
	if _, err := s.auth.Authorize(ctx, userID, orgID, org.RoleTeacher); err != nil {
		return err
	}
	return s.inTx(ctx, func(q *query.Queries) error {
		// Issuance freezes a consistent roster; all transfers of this school/year share its lock.
		if err := q.LockAssignmentRoster(ctx, query.LockAssignmentRosterParams{OrgID: orgID, YearID: yearID}); err != nil {
			return fmt.Errorf("lock class roster: %w", err)
		}
		// Membership row lock serializes placements of one student; the unique index is the final guard.
		role, err := q.GetMemberRoleForUpdate(ctx, query.GetMemberRoleForUpdateParams{OrgID: orgID, UserID: studentID})
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && role != query.OrgRoleParticipant) {
			return ErrNotStudent
		}
		if err != nil {
			return fmt.Errorf("get student role: %w", err)
		}
		if _, err := year(ctx, q, orgID, yearID); err != nil {
			return err
		}
		if classID != nil {
			c, err := q.LockClass(ctx, query.LockClassParams{OrgID: orgID, ID: *classID})
			if errors.Is(err, pgx.ErrNoRows) || (err == nil && c.AcademicYearID != yearID) {
				return ErrNotFound
			}
			if err != nil {
				return fmt.Errorf("lock class: %w", err)
			}
		}
		now := pgTime(s.now())
		cur, err := q.ActiveEnrollment(ctx, query.ActiveEnrollmentParams{OrgID: orgID, AcademicYearID: yearID, UserID: studentID})
		switch {
		case errors.Is(err, pgx.ErrNoRows):
		case err != nil:
			return fmt.Errorf("get active placement: %w", err)
		case classID != nil && cur.ClassID == *classID:
			return nil // already there
		default:
			if err := q.EndEnrollment(ctx, query.EndEnrollmentParams{ID: cur.ID, EndedAt: now}); err != nil {
				return fmt.Errorf("end placement: %w", err)
			}
		}
		if classID == nil {
			return nil
		}
		if _, err := q.CreateEnrollment(ctx, query.CreateEnrollmentParams{
			ID: uuid.New(), OrgID: orgID, AcademicYearID: yearID, ClassID: *classID, UserID: studentID, StartedAt: now, CreatedBy: userID,
		}); err != nil {
			return fmt.Errorf("create placement: %w", err)
		}
		return nil
	})
}

// History lists a student's placements in this school across years, newest first (teacher/admin).
func (s *Service) History(ctx context.Context, userID, orgID, studentID uuid.UUID) ([]Placement, error) {
	if _, err := s.auth.Authorize(ctx, userID, orgID, org.RoleTeacher); err != nil {
		return nil, err
	}
	rows, err := query.New(s.pool).EnrollmentHistory(ctx, query.EnrollmentHistoryParams{OrgID: orgID, UserID: studentID})
	if err != nil {
		return nil, fmt.Errorf("placement history: %w", err)
	}
	out := make([]Placement, len(rows))
	for i, r := range rows {
		out[i] = Placement{ClassName: r.ClassName, YearLabel: r.YearLabel, StartedAt: r.StartedAt.Time}
		if r.EndedAt.Valid {
			t := r.EndedAt.Time
			out[i].EndedAt = &t
		}
	}
	return out, nil
}
