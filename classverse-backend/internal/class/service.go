// Package class manages classes (rombel) per school and academic year (C1).
//
// Approved rules (C1 + C2): at most one active class per student per school and academic year. Admins manage
// years, classes, class teachers and every placement; a wali kelas (homeroom) adds students without a class
// to their own class and removes them; guru mapel and other teachers read only. History is kept and nobody is
// promoted or placed automatically. Classes guide reports and labels; they do not restrict the module catalog.
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
	ErrNotStaff      = errors.New("only teachers and admins of the school can teach a class")
	ErrHasClass      = errors.New("student already has a class this year")
)

// MaxPage bounds roster pages and bulk moves.
const MaxPage = 50

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

// Class is an active class with its number of placed students and teachers.
type Class struct {
	ID              uuid.UUID
	YearID          uuid.UUID
	Name            string
	Students        int32
	SubjectTeachers int32
	Homeroom        *Person // nil: no wali kelas
	MyRole          string  // caller's role in the class: "homeroom", "subject" or ""
}

// Person is a member shown by name.
type Person struct {
	ID   uuid.UUID
	Name string
}

// Teacher is a teacher of a class: Role "homeroom" (wali kelas) or "subject" (guru mapel).
type Teacher struct {
	UserID  uuid.UUID
	Name    string
	Role    string
	Subject string
}

// StaffMember is a teacher/admin of the school.
type StaffMember struct {
	UserID uuid.UUID
	Name   string
	Role   string
}

// RosterFilter narrows a roster page. ClassID and Unplaced are exclusive.
type RosterFilter struct {
	ClassID  *uuid.UUID
	Unplaced bool
	Query    string
	Offset   int32
	Limit    int32
}

// RosterPage is one page of the roster with the filtered total.
type RosterPage struct {
	Items []RosterEntry
	Total int32
}

// MoveResult reports one student of a bulk move.
type MoveResult struct {
	UserID uuid.UUID
	Error  string // "" on success; not_found, not_student, has_class
}

// CopyResult reports a year copy.
type CopyResult struct {
	Created int
	Skipped []string // class names that already existed in the target year
}

// Promotion maps a class of one year to a class of another.
type Promotion struct {
	From uuid.UUID
	To   uuid.UUID
}

// PromotionResult reports a promotion run.
type PromotionResult struct {
	Moved   int
	Skipped []Person // students already placed in the target year
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
	return s.inRawTx(ctx, func(tx pgx.Tx) error { return fn(query.New(tx)) })
}

func (s *Service) inRawTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin class transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
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

func classRow(c query.ListClassesRow) Class {
	out := Class{ID: c.ID, YearID: c.AcademicYearID, Name: c.Name, Students: c.Students, SubjectTeachers: c.SubjectTeachers}
	if c.HomeroomID.Valid {
		out.Homeroom = &Person{ID: c.HomeroomID.UUID, Name: c.HomeroomName.String}
	}
	if c.MyRole.Valid {
		out.MyRole = string(c.MyRole.ClassTeacherRole)
	}
	return out
}

// classOf loads an active class of this school; another school's or archived class is not found.
func classOf(ctx context.Context, q *query.Queries, orgID, id uuid.UUID) (query.Class, error) {
	c, err := q.GetClass(ctx, query.GetClassParams{OrgID: orgID, ID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return c, ErrNotFound
	}
	if err != nil {
		return c, fmt.Errorf("get class: %w", err)
	}
	return c, nil
}

// isHomeroom reports whether user is the active wali kelas of class.
func isHomeroom(ctx context.Context, q *query.Queries, classID, userID uuid.UUID) (bool, error) {
	role, err := q.ClassTeacherRole(ctx, query.ClassTeacherRoleParams{ClassID: classID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("class teacher role: %w", err)
	}
	return role == query.ClassTeacherRoleHomeroom, nil
}

// Years lists the school's academic years, newest first (teacher/admin).
func (s *Service) Years(ctx context.Context, userID, orgID uuid.UUID, page ...int32) ([]Year, error) {
	pageLimit := pgtype.Int4{}
	pageOffset := int32(0)
	if len(page) == 2 {
		pageLimit = pgtype.Int4{Int32: page[0], Valid: true}
		pageOffset = page[1]
	}

	if _, err := s.auth.Authorize(ctx, userID, orgID, org.RoleTeacher); err != nil {
		return nil, err
	}
	rows, err := query.New(s.pool).ListAcademicYears(ctx, query.ListAcademicYearsParams{OrgID: orgID, PageLimit: pageLimit, PageOffset: pageOffset})
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
func (s *Service) Classes(ctx context.Context, userID, orgID, yearID uuid.UUID, page ...int32) ([]Class, error) {
	pageLimit := pgtype.Int4{}
	pageOffset := int32(0)
	if len(page) == 2 {
		pageLimit = pgtype.Int4{Int32: page[0], Valid: true}
		pageOffset = page[1]
	}

	if _, err := s.auth.Authorize(ctx, userID, orgID, org.RoleTeacher); err != nil {
		return nil, err
	}
	q := query.New(s.pool)
	if _, err := year(ctx, q, orgID, yearID); err != nil {
		return nil, err
	}
	rows, err := q.ListClasses(ctx, query.ListClassesParams{PageLimit: pageLimit, PageOffset: pageOffset, UserID: userID, OrgID: orgID, AcademicYearID: yearID})
	if err != nil {
		return nil, fmt.Errorf("list classes: %w", err)
	}
	out := make([]Class, len(rows))
	for i, c := range rows {
		out[i] = classRow(c)
	}
	return out, nil
}

func nameTaken(err error) error {
	if uniqueViolation(err) {
		return &ValidationError{Fields: map[string]string{"name": "Nama kelas sudah dipakai di tahun ajaran ini"}}
	}
	return nil
}

// CreateClass adds a class to a year of this school (admin).
func (s *Service) CreateClass(ctx context.Context, userID, orgID, yearID uuid.UUID, name string) (Class, error) {
	if _, err := s.auth.Authorize(ctx, userID, orgID, org.RoleAdmin); err != nil {
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

// RenameClass changes a class name (admin).
func (s *Service) RenameClass(ctx context.Context, userID, orgID, classID uuid.UUID, name string) (Class, error) {
	if _, err := s.auth.Authorize(ctx, userID, orgID, org.RoleAdmin); err != nil {
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

// ArchiveClass hides an empty class and ends its teacher assignments (admin). History rows keep pointing at it.
func (s *Service) ArchiveClass(ctx context.Context, userID, orgID, classID uuid.UUID) error {
	if _, err := s.auth.Authorize(ctx, userID, orgID, org.RoleAdmin); err != nil {
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
		now := pgTime(s.now())
		if err := q.ArchiveClass(ctx, query.ArchiveClassParams{OrgID: orgID, ID: classID, ArchivedAt: now}); err != nil {
			return fmt.Errorf("archive class: %w", err)
		}
		if err := q.EndClassTeachers(ctx, query.EndClassTeachersParams{ClassID: classID, EndedAt: now}); err != nil {
			return fmt.Errorf("end class teachers: %w", err)
		}
		return nil
	})
}

// Roster returns one page of the school's participants with their class in the year (teacher/admin).
func (s *Service) Roster(ctx context.Context, userID, orgID, yearID uuid.UUID, f RosterFilter) (RosterPage, error) {
	if _, err := s.auth.Authorize(ctx, userID, orgID, org.RoleTeacher); err != nil {
		return RosterPage{}, err
	}
	if f.Limit < 1 || f.Limit > MaxPage || f.Offset < 0 || (f.ClassID != nil && f.Unplaced) || utf8.RuneCountInString(f.Query) > 80 {
		return RosterPage{}, &ValidationError{Fields: map[string]string{"filter": "Filter siswa tidak valid"}}
	}
	q := query.New(s.pool)
	if _, err := year(ctx, q, orgID, yearID); err != nil {
		return RosterPage{}, err
	}
	var classID uuid.NullUUID
	if f.ClassID != nil {
		classID = uuid.NullUUID{UUID: *f.ClassID, Valid: true}
	}
	needle := strings.TrimSpace(f.Query)
	total, err := q.CountRoster(ctx, query.CountRosterParams{AcademicYearID: yearID, OrgID: orgID, ClassID: classID, Unplaced: f.Unplaced, Q: needle})
	if err != nil {
		return RosterPage{}, fmt.Errorf("count roster: %w", err)
	}
	rows, err := q.ListRoster(ctx, query.ListRosterParams{AcademicYearID: yearID, OrgID: orgID, ClassID: classID, Unplaced: f.Unplaced, Q: needle, Off: f.Offset, Lim: f.Limit})
	if err != nil {
		return RosterPage{}, fmt.Errorf("list roster: %w", err)
	}
	out := make([]RosterEntry, len(rows))
	for i, r := range rows {
		out[i] = RosterEntry{UserID: r.UserID, DisplayName: r.DisplayName, AvatarKey: r.AvatarKey}
		if r.ClassID.Valid {
			id := r.ClassID.UUID
			out[i].ClassID = &id
		}
	}
	return RosterPage{Items: out, Total: total}, nil
}

// Place puts a participant into a class of the year, moving them if they already have one; classID nil
// removes them from their class. Admin may do any placement. A wali kelas may only add a student without a
// class to their class or remove a student of their class. One transaction; the old placement stays as history.
func (s *Service) Place(ctx context.Context, userID, orgID, yearID, studentID uuid.UUID, classID *uuid.UUID) error {
	role, err := s.auth.Authorize(ctx, userID, orgID, org.RoleTeacher)
	if err != nil {
		return err
	}
	return s.inTx(ctx, func(q *query.Queries) error {
		if err := lockRoster(ctx, q, orgID, yearID); err != nil {
			return err
		}
		return s.place(ctx, q, userID, role == org.RoleAdmin, orgID, yearID, studentID, classID)
	})
}

// Move places many students into one class (nil: remove from class) in one transaction, reporting each.
// Students who fail are skipped; the rest still move. Same rights as Place.
func (s *Service) Move(ctx context.Context, userID, orgID, yearID uuid.UUID, students []uuid.UUID, classID *uuid.UUID) ([]MoveResult, error) {
	role, err := s.auth.Authorize(ctx, userID, orgID, org.RoleTeacher)
	if err != nil {
		return nil, err
	}
	if len(students) == 0 || len(students) > MaxPage {
		return nil, &ValidationError{Fields: map[string]string{"user_ids": "Pilih 1–50 siswa"}}
	}
	out := make([]MoveResult, len(students))
	err = s.inRawTx(ctx, func(tx pgx.Tx) error {
		if err := lockRoster(ctx, query.New(tx), orgID, yearID); err != nil {
			return err
		}
		for i, id := range students {
			out[i].UserID = id
			// A savepoint keeps one failed student from aborting the whole transaction.
			err := savepoint(ctx, tx, func(q *query.Queries) error {
				return s.place(ctx, q, userID, role == org.RoleAdmin, orgID, yearID, id, classID)
			})
			switch {
			case err == nil:
			case errors.Is(err, ErrNotStudent):
				out[i].Error = "not_student"
			case errors.Is(err, ErrHasClass):
				out[i].Error = "has_class"
			default:
				return err // wrong class, missing right or DB failure: fail the whole request
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func lockRoster(ctx context.Context, q *query.Queries, orgID, yearID uuid.UUID) error {
	// Issuance freezes a consistent roster; all transfers of this school/year share its lock.
	if err := q.LockAssignmentRoster(ctx, query.LockAssignmentRosterParams{OrgID: orgID, YearID: yearID}); err != nil {
		return fmt.Errorf("lock class roster: %w", err)
	}
	if _, err := year(ctx, q, orgID, yearID); err != nil {
		return err
	}
	return nil
}

func savepoint(ctx context.Context, tx pgx.Tx, fn func(*query.Queries) error) error {
	sp, err := tx.Begin(ctx)
	if err != nil {
		return fmt.Errorf("savepoint: %w", err)
	}
	if err := fn(query.New(sp)); err != nil {
		_ = sp.Rollback(ctx)
		return err
	}
	if err := sp.Commit(ctx); err != nil {
		return fmt.Errorf("release savepoint: %w", err)
	}
	return nil
}

// place applies one placement inside a transaction holding the roster lock.
func (s *Service) place(ctx context.Context, q *query.Queries, userID uuid.UUID, admin bool, orgID, yearID, studentID uuid.UUID, classID *uuid.UUID) error {
	if classID != nil {
		c, err := q.LockClass(ctx, query.LockClassParams{OrgID: orgID, ID: *classID})
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && c.AcademicYearID != yearID) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("lock class: %w", err)
		}
	}
	// Membership row lock serializes placements of one student; the unique index is the final guard.
	role, err := q.GetMemberRoleForUpdate(ctx, query.GetMemberRoleForUpdateParams{OrgID: orgID, UserID: studentID})
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && role != query.OrgRoleParticipant) {
		return ErrNotStudent
	}
	if err != nil {
		return fmt.Errorf("get student role: %w", err)
	}
	now := pgTime(s.now())
	cur, err := q.ActiveEnrollment(ctx, query.ActiveEnrollmentParams{OrgID: orgID, AcademicYearID: yearID, UserID: studentID})
	hasCur := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("get active placement: %w", err)
	}
	if !admin {
		// Wali kelas: add a student without a class to their class, or remove a student from their class.
		target := cur.ClassID
		if classID != nil {
			target = *classID
		}
		if !hasCur && classID == nil {
			return nil
		}
		ok, err := isHomeroom(ctx, q, target, userID)
		if err != nil {
			return err
		}
		if !ok {
			return org.ErrForbidden
		}
		if classID != nil && hasCur && cur.ClassID != *classID {
			return ErrHasClass
		}
	}
	switch {
	case !hasCur:
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

// Staff lists the school's teachers and admins, candidates for class teachers (teacher/admin).
func (s *Service) Staff(ctx context.Context, userID, orgID uuid.UUID) ([]StaffMember, error) {
	if _, err := s.auth.Authorize(ctx, userID, orgID, org.RoleTeacher); err != nil {
		return nil, err
	}
	rows, err := query.New(s.pool).ListStaff(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list staff: %w", err)
	}
	out := make([]StaffMember, len(rows))
	for i, r := range rows {
		out[i] = StaffMember{UserID: r.UserID, Name: r.DisplayName, Role: string(r.Role)}
	}
	return out, nil
}

// Teachers lists the active wali kelas and guru mapel of a class (teacher/admin).
func (s *Service) Teachers(ctx context.Context, userID, orgID, classID uuid.UUID) ([]Teacher, error) {
	if _, err := s.auth.Authorize(ctx, userID, orgID, org.RoleTeacher); err != nil {
		return nil, err
	}
	q := query.New(s.pool)
	if _, err := classOf(ctx, q, orgID, classID); err != nil {
		return nil, err
	}
	rows, err := q.ListClassTeachers(ctx, query.ListClassTeachersParams{OrgID: orgID, ClassID: classID})
	if err != nil {
		return nil, fmt.Errorf("list class teachers: %w", err)
	}
	out := make([]Teacher, len(rows))
	for i, r := range rows {
		out[i] = Teacher{UserID: r.UserID, Name: r.DisplayName, Role: string(r.Role), Subject: r.Subject.String}
	}
	return out, nil
}

// SetTeacher makes a staff member the class's wali kelas (replacing the current one) or a guru mapel with an
// optional subject (admin). A person has one role per class; setting again replaces it. History is kept.
func (s *Service) SetTeacher(ctx context.Context, userID, orgID, classID, teacherID uuid.UUID, role, subject string) error {
	if _, err := s.auth.Authorize(ctx, userID, orgID, org.RoleAdmin); err != nil {
		return err
	}
	subject = strings.Join(strings.Fields(strings.ToValidUTF8(subject, "")), " ")
	var sub pgtype.Text
	switch {
	case role == string(query.ClassTeacherRoleHomeroom) && subject == "":
	case role == string(query.ClassTeacherRoleSubject):
		if n := utf8.RuneCountInString(subject); n > 40 || strings.ContainsFunc(subject, unicode.IsControl) {
			return &ValidationError{Fields: map[string]string{"subject": "Mata pelajaran maksimal 40 karakter"}}
		}
		sub = pgtype.Text{String: subject, Valid: subject != ""}
	default:
		return &ValidationError{Fields: map[string]string{"role": "Pilih wali kelas atau guru mapel"}}
	}
	return s.inTx(ctx, func(q *query.Queries) error {
		if _, err := q.LockClass(ctx, query.LockClassParams{OrgID: orgID, ID: classID}); errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return fmt.Errorf("lock class: %w", err)
		}
		r, err := q.GetMemberRoleForUpdate(ctx, query.GetMemberRoleForUpdateParams{OrgID: orgID, UserID: teacherID})
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && r == query.OrgRoleParticipant) {
			return ErrNotStaff
		}
		if err != nil {
			return fmt.Errorf("get teacher role: %w", err)
		}
		now := pgTime(s.now())
		if _, err := q.EndClassTeacher(ctx, query.EndClassTeacherParams{ClassID: classID, UserID: teacherID, EndedAt: now}); err != nil {
			return fmt.Errorf("end class teacher: %w", err)
		}
		if role == string(query.ClassTeacherRoleHomeroom) {
			if err := q.EndHomeroom(ctx, query.EndHomeroomParams{ClassID: classID, EndedAt: now}); err != nil {
				return fmt.Errorf("end homeroom: %w", err)
			}
		}
		if err := q.CreateClassTeacher(ctx, query.CreateClassTeacherParams{ID: uuid.New(), OrgID: orgID, ClassID: classID,
			UserID: teacherID, Role: query.ClassTeacherRole(role), Subject: sub, StartedAt: now, CreatedBy: userID}); err != nil {
			return fmt.Errorf("create class teacher: %w", err)
		}
		return nil
	})
}

// RemoveTeacher ends a person's role in a class (admin).
func (s *Service) RemoveTeacher(ctx context.Context, userID, orgID, classID, teacherID uuid.UUID) error {
	if _, err := s.auth.Authorize(ctx, userID, orgID, org.RoleAdmin); err != nil {
		return err
	}
	return s.inTx(ctx, func(q *query.Queries) error {
		if _, err := q.LockClass(ctx, query.LockClassParams{OrgID: orgID, ID: classID}); errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return fmt.Errorf("lock class: %w", err)
		}
		n, err := q.EndClassTeacher(ctx, query.EndClassTeacherParams{ClassID: classID, UserID: teacherID, EndedAt: pgTime(s.now())})
		if err != nil {
			return fmt.Errorf("end class teacher: %w", err)
		}
		if n == 0 {
			return ErrNotFound
		}
		return nil
	})
}

// CopyYear creates the active classes of fromYear in toYear (admin); names already used in toYear are skipped
// and reported. With teachers, still-staff wali kelas and guru mapel are copied too. Students are never copied.
func (s *Service) CopyYear(ctx context.Context, userID, orgID, toYear, fromYear uuid.UUID, teachers bool) (CopyResult, error) {
	if _, err := s.auth.Authorize(ctx, userID, orgID, org.RoleAdmin); err != nil {
		return CopyResult{}, err
	}
	if toYear == fromYear {
		return CopyResult{}, &ValidationError{Fields: map[string]string{"from_year_id": "Pilih tahun ajaran lain sebagai sumber"}}
	}
	res := CopyResult{Skipped: []string{}}
	err := s.inTx(ctx, func(q *query.Queries) error {
		if err := lockRoster(ctx, q, orgID, toYear); err != nil {
			return err
		}
		if _, err := year(ctx, q, orgID, fromYear); err != nil {
			return err
		}
		classes, err := q.ListYearClasses(ctx, query.ListYearClassesParams{OrgID: orgID, AcademicYearID: fromYear})
		if err != nil {
			return fmt.Errorf("list source classes: %w", err)
		}
		now := pgTime(s.now())
		for _, c := range classes {
			id, err := q.CreateClassIfFree(ctx, query.CreateClassIfFreeParams{ID: uuid.New(), OrgID: orgID, AcademicYearID: toYear, Name: c.Name, CreatedBy: userID, CreatedAt: now})
			if errors.Is(err, pgx.ErrNoRows) {
				res.Skipped = append(res.Skipped, c.Name)
				continue
			}
			if err != nil {
				return fmt.Errorf("copy class: %w", err)
			}
			res.Created++
			if teachers {
				if err := q.CopyClassTeachers(ctx, query.CopyClassTeachersParams{ToClassID: id, StartedAt: now, CreatedBy: userID, FromClassID: c.ID}); err != nil {
					return fmt.Errorf("copy class teachers: %w", err)
				}
			}
		}
		return nil
	})
	return res, err
}

// Promote places the active students of each source class (fromYear) into its mapped class of toYear (admin).
// Students who already have a class in toYear, or are no longer participants, are skipped and reported; no one
// is moved silently. Nothing happens without an explicit mapping (no automatic promotion).
func (s *Service) Promote(ctx context.Context, userID, orgID, toYear, fromYear uuid.UUID, maps []Promotion) (PromotionResult, error) {
	if _, err := s.auth.Authorize(ctx, userID, orgID, org.RoleAdmin); err != nil {
		return PromotionResult{}, err
	}
	if toYear == fromYear || len(maps) == 0 || len(maps) > 200 {
		return PromotionResult{}, &ValidationError{Fields: map[string]string{"mappings": "Pemetaan kelas tidak valid"}}
	}
	res := PromotionResult{Skipped: []Person{}}
	err := s.inRawTx(ctx, func(tx pgx.Tx) error {
		q := query.New(tx)
		if err := lockRoster(ctx, q, orgID, toYear); err != nil {
			return err
		}
		seen := map[uuid.UUID]bool{}
		for _, m := range maps {
			from, err := classOf(ctx, q, orgID, m.From)
			if err != nil {
				return err
			}
			to, err := classOf(ctx, q, orgID, m.To)
			if err != nil {
				return err
			}
			if from.AcademicYearID != fromYear || to.AcademicYearID != toYear || seen[m.From] {
				return &ValidationError{Fields: map[string]string{"mappings": "Pemetaan kelas tidak valid"}}
			}
			seen[m.From] = true
			students, err := q.ClassStudents(ctx, m.From)
			if err != nil {
				return fmt.Errorf("class students: %w", err)
			}
			for _, st := range students {
				_, err := q.ActiveEnrollment(ctx, query.ActiveEnrollmentParams{OrgID: orgID, AcademicYearID: toYear, UserID: st.UserID})
				if err == nil {
					res.Skipped = append(res.Skipped, Person{ID: st.UserID, Name: st.DisplayName})
					continue
				}
				if !errors.Is(err, pgx.ErrNoRows) {
					return fmt.Errorf("get target placement: %w", err)
				}
				target := m.To
				err = savepoint(ctx, tx, func(q *query.Queries) error {
					return s.place(ctx, q, userID, true, orgID, toYear, st.UserID, &target)
				})
				if errors.Is(err, ErrNotStudent) {
					res.Skipped = append(res.Skipped, Person{ID: st.UserID, Name: st.DisplayName})
					continue
				}
				if err != nil {
					return err
				}
				res.Moved++
			}
		}
		return nil
	})
	return res, err
}
