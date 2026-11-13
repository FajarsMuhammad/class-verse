// Package module manages learning paths (M1): a module per campus building and level, made of
// ordered steps (short text, an F1 file read in-app, or a YouTube video).
//
// Revision rule (approved 2026-11-13): published modules are edited in place; steps are never
// deleted, only archived, so M2 progress keeps pointing at real rows.
package module

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/org"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

// Allowed values mirror the CHECK constraints in 00010_modules.sql.
var (
	Levels = []string{"SD", "SMP", "SMA", "Umum"}
	Kinds  = []string{"materi", "berkas", "video"}
)

const maxSteps = 50

var videoID = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

// Domain errors.
var (
	ErrNotFound    = errors.New("module not found")
	ErrStepLimit   = errors.New("too many steps")
	ErrBadOrdering = errors.New("order must list every active step once")
)

// ValidationError lists invalid input fields.
type ValidationError struct{ Fields map[string]string }

func (e *ValidationError) Error() string { return "invalid module fields" }

func (e *ValidationError) add(field, msg string) {
	if e.Fields == nil {
		e.Fields = map[string]string{}
	}
	e.Fields[field] = msg
}

func (e *ValidationError) err() error {
	if e.Fields == nil {
		return nil
	}
	return e
}

// Authorizer checks a user's role in an organization (implemented by org.Service).
type Authorizer interface {
	Authorize(ctx context.Context, userID, orgID uuid.UUID, minRole org.Role) (org.Role, error)
}

// Buildings reports whether a campus building exists (implemented in cmd/server from the campus map).
type Buildings interface {
	Building(key string) (staffOnly, ok bool)
}

// Input is the editable part of a module.
type Input struct {
	BuildingKey string
	Level       string
	Title       string
	Description string
	Published   bool
}

// StepInput is the editable part of a step.
type StepInput struct {
	Kind    string
	Title   string
	Body    string
	FileID  *uuid.UUID
	VideoID string
	Minutes int
}

// Step is a step with its file kind (pdf/audio) and the caller's reader position for that file.
type Step struct {
	query.ModuleStep
	FileKind       string
	ReaderPosition int32
	Done           bool
}

// Module is a module with its active steps in order.
type Module struct {
	query.Module
	Steps []Step
}

// Filter narrows a module listing.
type Filter struct {
	BuildingKey string
	Level       string
	Drafts      bool
}

// Service implements module rules.
type Service struct {
	pool      *pgxpool.Pool
	auth      Authorizer
	buildings Buildings
	now       func() time.Time
}

// NewService wires the module service.
func NewService(pool *pgxpool.Pool, auth Authorizer, buildings Buildings, now func() time.Time) *Service {
	return &Service{pool: pool, auth: auth, buildings: buildings, now: now}
}

func text(s string, lo, hi int, multiline bool) (string, bool) {
	s = strings.TrimSpace(strings.ToValidUTF8(s, ""))
	n := utf8.RuneCountInString(s)
	bad := strings.ContainsFunc(s, func(r rune) bool {
		return unicode.IsControl(r) && (!multiline || (r != '\n' && r != '\t'))
	})
	return s, n >= lo && n <= hi && !bad
}

func (s *Service) validate(in Input) (Input, error) {
	var v ValidationError
	var ok bool
	if in.Title, ok = text(in.Title, 1, 120, false); !ok {
		v.add("title", "Judul 1–120 karakter")
	}
	if in.Description, ok = text(in.Description, 0, 500, true); !ok {
		v.add("description", "Deskripsi maksimal 500 karakter")
	}
	if !slices.Contains(Levels, in.Level) {
		v.add("level", "Pilih jenjang")
	}
	// Staff-only buildings (Ruang Guru) have no learning paths for participants.
	if staff, ok := s.buildings.Building(in.BuildingKey); !ok || staff {
		v.add("building_key", "Pilih gedung")
	}
	return in, v.err()
}

func validateStep(in StepInput) (StepInput, error) {
	var v ValidationError
	var ok bool
	if in.Title, ok = text(in.Title, 1, 120, false); !ok {
		v.add("title", "Judul 1–120 karakter")
	}
	if in.Minutes < 1 || in.Minutes > 240 {
		v.add("minutes", "Durasi 1–240 menit")
	}
	minBody := 0
	if in.Kind == "materi" {
		minBody = 1
	}
	if in.Body, ok = text(in.Body, minBody, 5000, true); !ok {
		v.add("body", "Isi materi 1–5000 karakter")
	}
	switch in.Kind {
	case "materi":
		in.FileID, in.VideoID = nil, ""
	case "berkas":
		if in.FileID == nil {
			v.add("file_id", "Pilih file")
		}
		in.VideoID = ""
	case "video":
		in.FileID = nil
		if in.VideoID = youTubeID(in.VideoID); in.VideoID == "" {
			v.add("video_id", "Tempel tautan YouTube yang valid")
		}
	default:
		v.add("kind", "Pilih jenis langkah")
	}
	return in, v.err()
}

// youTubeID accepts an 11-character video ID or a youtube.com / youtu.be link and returns the ID.
func youTubeID(raw string) string {
	raw = strings.TrimSpace(raw)
	if videoID.MatchString(raw) {
		return raw
	}
	for _, prefix := range []string{
		"https://www.youtube.com/watch?v=", "https://youtube.com/watch?v=", "https://m.youtube.com/watch?v=",
		"https://youtu.be/", "https://www.youtube.com/embed/", "https://www.youtube-nocookie.com/embed/",
		"https://www.youtube.com/shorts/",
	} {
		if rest, ok := strings.CutPrefix(raw, prefix); ok {
			id, _, _ := strings.Cut(rest, "&")
			id, _, _ = strings.Cut(id, "?")
			if videoID.MatchString(id) {
				return id
			}
		}
	}
	return ""
}

// checkFile requires a ready file of this school.
func checkFile(ctx context.Context, q *query.Queries, orgID uuid.UUID, id *uuid.UUID) error {
	if id == nil {
		return nil
	}
	if _, err := q.GetReadyFile(ctx, query.GetReadyFileParams{OrgID: orgID, ID: *id}); errors.Is(err, pgx.ErrNoRows) {
		return &ValidationError{Fields: map[string]string{"file_id": "File tidak ditemukan atau belum siap"}}
	} else if err != nil {
		return fmt.Errorf("get step file: %w", err)
	}
	return nil
}

func pgTime(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t.UTC(), Valid: true} }

func nullID(id *uuid.UUID) uuid.NullUUID {
	if id == nil {
		return uuid.NullUUID{}
	}
	return uuid.NullUUID{UUID: *id, Valid: true}
}

func nullText(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }

func (s *Service) inTx(ctx context.Context, fn func(*query.Queries) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin module transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(query.New(tx)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit module transaction: %w", err)
	}
	return nil
}

func (s *Service) staff(ctx context.Context, userID, orgID uuid.UUID) error {
	_, err := s.auth.Authorize(ctx, userID, orgID, org.RoleTeacher)
	return err
}

// Create adds a module (teacher/admin).
func (s *Service) Create(ctx context.Context, userID, orgID uuid.UUID, in Input) (query.Module, error) {
	if err := s.staff(ctx, userID, orgID); err != nil {
		return query.Module{}, err
	}
	in, err := s.validate(in)
	if err != nil {
		return query.Module{}, err
	}
	now := s.now()
	published := pgtype.Timestamptz{}
	if in.Published {
		published = pgTime(now)
	}
	m, err := query.New(s.pool).CreateModule(ctx, query.CreateModuleParams{
		ID: uuid.New(), OrgID: orgID, BuildingKey: in.BuildingKey, Level: in.Level, Title: in.Title,
		Description: in.Description, CreatedBy: userID, CreatedAt: pgTime(now), PublishedAt: published,
	})
	if err != nil {
		return query.Module{}, fmt.Errorf("create module: %w", err)
	}
	return m, nil
}

// Update replaces a module's fields (teacher/admin). Publishing keeps the first publish time.
func (s *Service) Update(ctx context.Context, userID, orgID, id uuid.UUID, in Input) (query.Module, error) {
	if err := s.staff(ctx, userID, orgID); err != nil {
		return query.Module{}, err
	}
	in, err := s.validate(in)
	if err != nil {
		return query.Module{}, err
	}
	var m query.Module
	err = s.inTx(ctx, func(q *query.Queries) error {
		cur, err := s.lock(ctx, q, orgID, id)
		if err != nil {
			return err
		}
		published := pgtype.Timestamptz{}
		if in.Published {
			published = cur.PublishedAt
			if !published.Valid {
				published = pgTime(s.now())
			}
		}
		m, err = q.UpdateModule(ctx, query.UpdateModuleParams{
			OrgID: orgID, ID: id, BuildingKey: in.BuildingKey, Level: in.Level, Title: in.Title,
			Description: in.Description, PublishedAt: published, UpdatedAt: pgTime(s.now()),
		})
		if err != nil {
			return fmt.Errorf("update module: %w", err)
		}
		return nil
	})
	return m, err
}

// Archive hides a module and its steps from everyone (teacher/admin).
func (s *Service) Archive(ctx context.Context, userID, orgID, id uuid.UUID) error {
	if err := s.staff(ctx, userID, orgID); err != nil {
		return err
	}
	n, err := query.New(s.pool).ArchiveModule(ctx, query.ArchiveModuleParams{OrgID: orgID, ID: id, ArchivedAt: pgTime(s.now())})
	if err != nil {
		return fmt.Errorf("archive module: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Service) lock(ctx context.Context, q *query.Queries, orgID, id uuid.UUID) (query.Module, error) {
	m, err := q.LockModule(ctx, query.LockModuleParams{OrgID: orgID, ID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return m, ErrNotFound
	}
	if err != nil {
		return m, fmt.Errorf("lock module: %w", err)
	}
	return m, nil
}

// AddStep appends a step to a module (teacher/admin).
func (s *Service) AddStep(ctx context.Context, userID, orgID, moduleID uuid.UUID, in StepInput) (query.ModuleStep, error) {
	if err := s.staff(ctx, userID, orgID); err != nil {
		return query.ModuleStep{}, err
	}
	in, err := validateStep(in)
	if err != nil {
		return query.ModuleStep{}, err
	}
	var step query.ModuleStep
	err = s.inTx(ctx, func(q *query.Queries) error {
		// The module lock serializes appends, so positions stay unique.
		if _, err := s.lock(ctx, q, orgID, moduleID); err != nil {
			return err
		}
		if err := checkFile(ctx, q, orgID, in.FileID); err != nil {
			return err
		}
		pos, err := q.NextStepPosition(ctx, moduleID)
		if err != nil {
			return fmt.Errorf("next step position: %w", err)
		}
		if pos >= maxSteps {
			return ErrStepLimit
		}
		step, err = q.CreateStep(ctx, query.CreateStepParams{
			ID: uuid.New(), OrgID: orgID, ModuleID: moduleID, Position: pos, Kind: in.Kind, Title: in.Title,
			Body: in.Body, FileID: nullID(in.FileID), VideoID: nullText(in.VideoID), Minutes: int32(in.Minutes),
			CreatedAt: pgTime(s.now()),
		})
		if err != nil {
			return fmt.Errorf("create step: %w", err)
		}
		return nil
	})
	return step, err
}

// UpdateStep replaces a step's content (teacher/admin).
func (s *Service) UpdateStep(ctx context.Context, userID, orgID, moduleID, stepID uuid.UUID, in StepInput) (query.ModuleStep, error) {
	if err := s.staff(ctx, userID, orgID); err != nil {
		return query.ModuleStep{}, err
	}
	in, err := validateStep(in)
	if err != nil {
		return query.ModuleStep{}, err
	}
	var step query.ModuleStep
	err = s.inTx(ctx, func(q *query.Queries) error {
		if _, err := s.lock(ctx, q, orgID, moduleID); err != nil {
			return err
		}
		if err := checkFile(ctx, q, orgID, in.FileID); err != nil {
			return err
		}
		step, err = q.UpdateStep(ctx, query.UpdateStepParams{
			OrgID: orgID, ModuleID: moduleID, ID: stepID, Kind: in.Kind, Title: in.Title, Body: in.Body,
			FileID: nullID(in.FileID), VideoID: nullText(in.VideoID), Minutes: int32(in.Minutes), UpdatedAt: pgTime(s.now()),
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("update step: %w", err)
		}
		return nil
	})
	return step, err
}

// ArchiveStep hides a step; the row stays for progress history (teacher/admin).
func (s *Service) ArchiveStep(ctx context.Context, userID, orgID, moduleID, stepID uuid.UUID) error {
	if err := s.staff(ctx, userID, orgID); err != nil {
		return err
	}
	return s.inTx(ctx, func(q *query.Queries) error {
		if _, err := s.lock(ctx, q, orgID, moduleID); err != nil {
			return err
		}
		n, err := q.ArchiveStep(ctx, query.ArchiveStepParams{OrgID: orgID, ModuleID: moduleID, ID: stepID, ArchivedAt: pgTime(s.now())})
		if err != nil {
			return fmt.Errorf("archive step: %w", err)
		}
		if n == 0 {
			return ErrNotFound
		}
		return nil
	})
}

// Reorder sets the step order; ids must list every active step exactly once (teacher/admin).
func (s *Service) Reorder(ctx context.Context, userID, orgID, moduleID uuid.UUID, ids []uuid.UUID) error {
	if err := s.staff(ctx, userID, orgID); err != nil {
		return err
	}
	return s.inTx(ctx, func(q *query.Queries) error {
		if _, err := s.lock(ctx, q, orgID, moduleID); err != nil {
			return err
		}
		cur, err := q.ActiveStepIDs(ctx, query.ActiveStepIDsParams{OrgID: orgID, ModuleID: moduleID})
		if err != nil {
			return fmt.Errorf("list step ids: %w", err)
		}
		sortIDs := func(a []uuid.UUID) []uuid.UUID {
			b := slices.Clone(a)
			slices.SortFunc(b, func(x, y uuid.UUID) int { return strings.Compare(x.String(), y.String()) })
			return b
		}
		if !slices.Equal(sortIDs(cur), sortIDs(ids)) {
			return ErrBadOrdering
		}
		for i, id := range ids {
			if err := q.SetStepPosition(ctx, query.SetStepPositionParams{ModuleID: moduleID, ID: id, Position: int32(i)}); err != nil {
				return fmt.Errorf("set step position: %w", err)
			}
		}
		return nil
	})
}

// List returns modules with their steps. Participants see only published modules; drafts are staff-only.
func (s *Service) List(ctx context.Context, userID, orgID uuid.UUID, f Filter) ([]Module, error) {
	role, err := s.auth.Authorize(ctx, userID, orgID, org.RoleParticipant)
	if err != nil {
		return nil, err
	}
	if f.Level != "" && !slices.Contains(Levels, f.Level) {
		return nil, &ValidationError{Fields: map[string]string{"level": "Jenjang tidak dikenal"}}
	}
	q := query.New(s.pool)
	params := query.ListModulesParams{OrgID: orgID, IncludeDrafts: f.Drafts && role.AtLeast(org.RoleTeacher)}
	if f.BuildingKey != "" {
		params.BuildingKey = pgtype.Text{String: f.BuildingKey, Valid: true}
	}
	if f.Level != "" {
		params.Level = pgtype.Text{String: f.Level, Valid: true}
	}
	rows, err := q.ListModules(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list modules: %w", err)
	}
	if len(rows) == 0 {
		return []Module{}, nil
	}
	ids := make([]uuid.UUID, len(rows))
	byID := make(map[uuid.UUID]int, len(rows))
	out := make([]Module, len(rows))
	for i, m := range rows {
		ids[i], byID[m.ID], out[i] = m.ID, i, Module{Module: m, Steps: []Step{}}
	}
	steps, err := q.ListModuleSteps(ctx, query.ListModuleStepsParams{UserID: userID, OrgID: orgID, ModuleIds: ids})
	if err != nil {
		return nil, fmt.Errorf("list steps: %w", err)
	}
	for _, st := range steps {
		i := byID[st.ModuleStep.ModuleID]
		out[i].Steps = append(out[i].Steps, Step{ModuleStep: st.ModuleStep, FileKind: st.FileKind, ReaderPosition: st.ReaderPosition, Done: st.Done})
	}
	return out, nil
}
