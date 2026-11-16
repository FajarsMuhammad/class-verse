// Package gradebook reads academic progress and grades; report remains moderation-only.
package gradebook

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/org"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

// Domain errors for invalid or cross-school filters.
var (
	ErrNotFound = errors.New("report target or filter not found")
	ErrInvalid  = errors.New("invalid report filter")
)

// Buildings checks school-campus building keys.
type Buildings interface{ Building(string) (bool, bool) }

// Filter selects roster membership, not a date range or retroactive score ownership.
type Filter struct {
	YearID, ClassID, ModuleID, QuizID *uuid.UUID
	Building                          string
}

// Service reads existing learning records without changing them.
type Service struct {
	pool      *pgxpool.Pool
	auth      *org.Service
	buildings Buildings
}

// NewService wires school authorization and current campus buildings.
func NewService(p *pgxpool.Pool, a *org.Service, b Buildings) *Service {
	return &Service{pool: p, auth: a, buildings: b}
}
func nullable(id *uuid.UUID) uuid.NullUUID {
	if id == nil {
		return uuid.NullUUID{}
	}
	return uuid.NullUUID{UUID: *id, Valid: true}
}

// Module is catalog progress, not assigned work.
type Module struct {
	ID       uuid.UUID `json:"id"`
	Title    string    `json:"title"`
	Building string    `json:"building_key"`
	Level    string    `json:"level"`
	Archived bool      `json:"archived"`
	Done     int32     `json:"done"`
	Total    int32     `json:"total"`
}

// Quiz reports one best and last score per quiz. No answer keys are exposed.
type Quiz struct {
	ID       uuid.UUID `json:"id"`
	Title    string    `json:"title"`
	Building string    `json:"building_key"`
	Archived bool      `json:"archived"`
	Attempts int32     `json:"attempts"`
	Best     *int32    `json:"best"`
	Last     *int32    `json:"last"`
}

// Task keeps issuance class/year, independent of current enrollment.
type Task struct {
	ID        uuid.UUID  `json:"id"`
	Title     string     `json:"title"`
	ClassName string     `json:"class_name"`
	YearLabel string     `json:"year_label"`
	Building  string     `json:"building_key"`
	Cancelled bool       `json:"cancelled"`
	DueAt     *time.Time `json:"due_at"`
	Done      int32      `json:"done"`
	Total     int32      `json:"total"`
}

// Student is an individual academic report; missing grades stay null.
type Student struct {
	UserID      uuid.UUID `json:"user_id"`
	Name        string    `json:"name"`
	Modules     []Module  `json:"modules"`
	Quizzes     []Quiz    `json:"quizzes"`
	Assignments []Task    `json:"assignments"`
	AverageBest *float64  `json:"average_best"`
}

// Row summarizes one participant for staff and CSV.
type Row struct {
	UserID           uuid.UUID `json:"user_id"`
	Name             string    `json:"name"`
	ClassName        string    `json:"class_name"`
	ModulesDone      int32     `json:"modules_done"`
	ModulesTotal     int32     `json:"modules_total"`
	QuizzesAttempted int32     `json:"quizzes_attempted"`
	QuizzesTotal     int32     `json:"quizzes_total"`
	AverageBest      *float64  `json:"average_best"`
}

// Page is a stable name/id-ordered roster page.
type Page struct {
	Items []Row `json:"items"`
	Total int32 `json:"total"`
}

func (s *Service) validate(ctx context.Context, q *query.Queries, orgID uuid.UUID, f Filter) error {
	if f.ClassID != nil && f.YearID == nil {
		return ErrInvalid
	}
	if f.Building != "" {
		staff, ok := s.buildings.Building(f.Building)
		if !ok || staff {
			return ErrNotFound
		}
	}
	if f.YearID != nil {
		if _, err := q.GetAcademicYear(ctx, query.GetAcademicYearParams{OrgID: orgID, ID: *f.YearID}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
	}
	if f.ClassID != nil {
		valid, err := q.GradebookClassExists(ctx, query.GradebookClassExistsParams{OrgID: orgID, AcademicYearID: *f.YearID, ID: *f.ClassID})
		if err != nil {
			return err
		}
		if !valid {
			return ErrNotFound
		}
	}
	valid, err := q.GradebookContentFilter(ctx, query.GradebookContentFilterParams{OrgID: orgID, ModuleID: nullable(f.ModuleID), QuizID: nullable(f.QuizID)})
	if err != nil {
		return err
	}
	if !valid.Valid || !valid.Bool {
		return ErrNotFound
	}
	return nil
}

// Student returns self for a participant, or any school participant for staff.
func (s *Service) Student(ctx context.Context, user, orgID, target uuid.UUID, f Filter) (Student, error) {
	role, err := s.auth.Authorize(ctx, user, orgID, org.RoleParticipant)
	if err != nil {
		return Student{}, err
	}
	if target != user && !role.AtLeast(org.RoleTeacher) {
		return Student{}, org.ErrForbidden
	}
	q := query.New(s.pool)
	r := Student{UserID: target, Modules: []Module{}, Quizzes: []Quiz{}, Assignments: []Task{}}
	// Profile lookup must not expose another school's user, including staff-only school profiles.
	r.Name, err = q.GradebookStudentName(ctx, query.GradebookStudentNameParams{OrgID: orgID, TargetID: target, ActorID: user})
	if errors.Is(err, pgx.ErrNoRows) {
		return r, ErrNotFound
	}
	if err != nil {
		return r, err
	}
	if err := s.validate(ctx, q, orgID, f); err != nil {
		return r, err
	}
	modules, err := q.GradebookModules(ctx, query.GradebookModulesParams{OrgID: orgID, UserID: target, BuildingKey: f.Building, ModuleID: nullable(f.ModuleID)})
	if err != nil {
		return r, fmt.Errorf("report modules: %w", err)
	}
	for _, m := range modules {
		r.Modules = append(r.Modules, Module{ID: m.ID, Title: m.Title, Building: m.BuildingKey, Level: m.Level, Archived: m.Archived, Done: m.Done, Total: m.Total})
	}
	quizzes, err := q.GradebookQuizzes(ctx, query.GradebookQuizzesParams{OrgID: orgID, UserID: target, BuildingKey: f.Building, QuizID: nullable(f.QuizID)})
	if err != nil {
		return r, fmt.Errorf("report quizzes: %w", err)
	}
	sum, n := 0, 0
	for _, z := range quizzes {
		v := Quiz{ID: z.ID, Title: z.Title, Building: z.BuildingKey, Archived: z.Archived, Attempts: z.Attempts}
		if z.Best >= 0 {
			best, last := z.Best, z.Last
			v.Best = &best
			v.Last = &last
			sum += int(best)
			n++
		}
		r.Quizzes = append(r.Quizzes, v)
	}
	if n > 0 {
		avg := float64(sum) / float64(n)
		r.AverageBest = &avg
	}
	tasks, err := q.GradebookAssignments(ctx, query.GradebookAssignmentsParams{OrgID: orgID, UserID: target, BuildingKey: f.Building, ModuleID: nullable(f.ModuleID)})
	if err != nil {
		return r, fmt.Errorf("report tasks: %w", err)
	}
	for _, a := range tasks {
		v := Task{ID: a.ID, Title: a.Title, ClassName: a.ClassName, YearLabel: a.YearLabel, Building: a.BuildingKey, Cancelled: a.Cancelled, Done: a.Done, Total: a.Total}
		if a.DueAt.Valid {
			d := a.DueAt.Time
			v.DueAt = &d
		}
		r.Assignments = append(r.Assignments, v)
	}
	return r, nil
}

// Roster returns a bounded participant page; export explicitly raises the limit to its fixed ceiling.
func (s *Service) Roster(ctx context.Context, user, orgID uuid.UUID, f Filter, offset, limit int32) (Page, error) {
	if _, err := s.auth.Authorize(ctx, user, orgID, org.RoleTeacher); err != nil {
		return Page{}, err
	}
	if offset < 0 || limit < 1 || limit > 10001 {
		return Page{}, ErrInvalid
	}
	q := query.New(s.pool)
	if err := s.validate(ctx, q, orgID, f); err != nil {
		return Page{}, err
	}
	rows, err := q.GradebookRoster(ctx, query.GradebookRosterParams{OrgID: orgID, YearID: nullable(f.YearID), ClassID: nullable(f.ClassID), ModuleID: nullable(f.ModuleID), QuizID: nullable(f.QuizID), BuildingKey: f.Building, PageOffset: offset, PageLimit: limit})
	if err != nil {
		return Page{}, fmt.Errorf("report roster: %w", err)
	}
	total, err := q.GradebookStudentCount(ctx, query.GradebookStudentCountParams{OrgID: orgID, YearID: nullable(f.YearID), ClassID: nullable(f.ClassID)})
	if err != nil {
		return Page{}, err
	}
	page := Page{Items: make([]Row, len(rows)), Total: total}
	for i, r := range rows {
		page.Total = r.TotalStudents
		page.Items[i] = Row{UserID: r.ID, Name: r.DisplayName, ClassName: r.ClassName.String, ModulesDone: r.ModulesDone, ModulesTotal: r.ModulesTotal, QuizzesAttempted: r.QuizzesAttempted, QuizzesTotal: r.QuizzesTotal}
		if r.AverageBest >= 0 {
			v := r.AverageBest
			page.Items[i].AverageBest = &v
		}
	}
	return page, nil
}
