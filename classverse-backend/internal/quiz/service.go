// Package quiz manages self-paced quizzes (Q1): teachers author four-option questions per campus building
// and level; members take them and the server grades.
//
// Rules approved 2026-11-13: pass score default 60 per quiz; unlimited attempts (report uses the best);
// answers and explanations are revealed only after the whole quiz is finished.
package quiz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

// Levels mirrors the CHECK constraint in 00012_quizzes.sql (same values as modules).
var Levels = []string{"SD", "SMP", "SMA", "Umum"}

const (
	maxQuestions     = 50
	optionCount      = 4
	defaultPassScore = 60
)

// Domain errors.
var (
	ErrConflict      = errors.New("draft version conflict")
	ErrNotFound      = errors.New("quiz not found")
	ErrQuestionLimit = errors.New("too many questions")
	ErrNoQuestions   = errors.New("quiz has no questions")
	ErrBadOrdering   = errors.New("order must list every question once")
	ErrFinished      = errors.New("attempt already finished")
)

// ValidationError lists invalid input fields.
type ValidationError struct{ Fields map[string]string }

func (e *ValidationError) Error() string { return "invalid quiz fields" }

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

// Input is the editable part of a quiz. PassScore 0 means the default (60).
type Input struct {
	ExpectedVersion int64
	BuildingKey     string
	Level           string
	Title           string
	Description     string
	PassScore       int
	Published       bool
}

// QuestionInput is one question: exactly four options, Answer is the index of the correct one.
type QuestionInput struct {
	ExpectedVersion int64
	Prompt          string
	Options         []string
	Answer          int
	Explanation     string
}

// Quiz is a quiz with the caller's history and, for staff, its questions with answers.
type Quiz struct {
	TotalCount int64
	query.Quiz
	QuestionCount int32
	BestScore     int16 // -1: no finished attempt yet
	Attempts      int32
	ModuleTitles  []string             // staff authoring only: active modules with a kuis step on it
	Questions     []query.QuizQuestion // staff only; nil for participants
}

// Filter narrows a quiz listing.
type Filter struct {
	BuildingKey string
	Level       string
	Drafts      bool
	Offset      int32
	Limit       int32
	Search      string
	Status      string
}

// Service implements quiz rules.
type Service struct {
	pool      *pgxpool.Pool
	auth      Authorizer
	buildings Buildings
	now       func() time.Time
}

// NewService wires the quiz service.
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
	// Staff-only buildings (Ruang Guru) have no quizzes for participants.
	if staff, ok := s.buildings.Building(in.BuildingKey); !ok || staff {
		v.add("building_key", "Pilih gedung")
	}
	if in.PassScore < 1 || in.PassScore > 100 {
		v.add("pass_score", "Nilai lulus 1–100")
	}
	return in, v.err()
}

func validateQuestion(in QuestionInput) (QuestionInput, error) {
	var v ValidationError
	var ok bool
	if in.Prompt, ok = text(in.Prompt, 1, 500, true); !ok {
		v.add("prompt", "Pertanyaan 1–500 karakter")
	}
	if in.Explanation, ok = text(in.Explanation, 0, 500, true); !ok {
		v.add("explanation", "Penjelasan maksimal 500 karakter")
	}
	if len(in.Options) != optionCount && len(in.Options) != 2 {
		v.add("options", "Isi tepat 4 pilihan atau 2 pilihan benar/salah")
	} else {
		seen := map[string]bool{}
		opts := make([]string, len(in.Options))
		for i, o := range in.Options {
			o, ok = text(o, 1, 200, false)
			key := strings.ToLower(o)
			if !ok || seen[key] {
				v.add("options", "Setiap pilihan 1–200 karakter dan tidak boleh sama")
				break
			}
			seen[key], opts[i] = true, o
		}
		in.Options = opts
	}
	if in.Answer < 0 || in.Answer >= len(in.Options) {
		v.add("answer", "Pilih jawaban benar")
	}
	return in, v.err()
}

func pgTime(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t.UTC(), Valid: true} }

func (s *Service) inTx(ctx context.Context, fn func(*query.Queries) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin quiz transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(query.New(tx)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit quiz transaction: %w", err)
	}
	return nil
}

func (s *Service) staff(ctx context.Context, userID, orgID uuid.UUID) error {
	_, err := s.auth.Authorize(ctx, userID, orgID, org.RoleTeacher)
	return err
}

func (s *Service) lock(ctx context.Context, q *query.Queries, orgID, id uuid.UUID) (query.Quiz, error) {
	z, err := q.LockQuiz(ctx, query.LockQuizParams{OrgID: orgID, ID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return z, ErrNotFound
	}
	if err != nil {
		return z, fmt.Errorf("lock quiz: %w", err)
	}
	return z, nil
}

func withDefaults(in Input) Input {
	if in.PassScore == 0 {
		in.PassScore = defaultPassScore
	}
	return in
}

// Create adds a quiz (teacher/admin). A quiz needs questions before it can be published.
func (s *Service) Create(ctx context.Context, userID, orgID uuid.UUID, in Input) (query.Quiz, error) {
	if err := s.staff(ctx, userID, orgID); err != nil {
		return query.Quiz{}, err
	}
	in, err := s.validate(withDefaults(in))
	if err != nil {
		return query.Quiz{}, err
	}
	if in.Published {
		return query.Quiz{}, ErrNoQuestions
	}
	now := s.now()
	z, err := query.New(s.pool).CreateQuiz(ctx, query.CreateQuizParams{
		ID: uuid.New(), OrgID: orgID, BuildingKey: in.BuildingKey, Level: in.Level, Title: in.Title,
		Description: in.Description, PassScore: int16(in.PassScore), CreatedBy: userID, CreatedAt: pgTime(now),
	})
	if err != nil {
		return query.Quiz{}, fmt.Errorf("create quiz: %w", err)
	}
	return z, nil
}

// Update replaces a quiz's fields (teacher/admin). Publishing keeps the first publish time and
// requires at least one question. Finished attempts keep their own pass score.
func (s *Service) Update(ctx context.Context, userID, orgID, id uuid.UUID, in Input) (query.Quiz, error) {
	if err := s.staff(ctx, userID, orgID); err != nil {
		return query.Quiz{}, err
	}
	in, err := s.validate(withDefaults(in))
	if err != nil {
		return query.Quiz{}, err
	}
	var z query.Quiz
	err = s.inTx(ctx, func(q *query.Queries) error {
		cur, err := s.lock(ctx, q, orgID, id)
		if err != nil {
			return err
		}
		if err := s.bumpDraft(ctx, q, cur, in.ExpectedVersion); err != nil {
			return err
		}
		z, err = q.UpdateQuiz(ctx, query.UpdateQuizParams{
			OrgID: orgID, ID: id, BuildingKey: in.BuildingKey, Level: in.Level, Title: in.Title,
			Description: in.Description, PassScore: int16(in.PassScore), PublishedAt: cur.PublishedAt, UpdatedAt: pgTime(s.now()),
		})
		if err != nil {
			return fmt.Errorf("update quiz: %w", err)
		}
		if in.Published {
			z, err = s.publishLocked(ctx, q, orgID, id)
			return err
		}
		return nil
	})
	return z, err
}

// Archive hides a quiz from everyone (teacher/admin). Attempts stay for the report.
func (s *Service) Archive(ctx context.Context, userID, orgID, id uuid.UUID) error {
	if err := s.staff(ctx, userID, orgID); err != nil {
		return err
	}
	n, err := query.New(s.pool).ArchiveQuiz(ctx, query.ArchiveQuizParams{OrgID: orgID, ID: id, ArchivedAt: pgTime(s.now())})
	if err != nil {
		return fmt.Errorf("archive quiz: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// AddQuestion appends a question (teacher/admin).
func (s *Service) AddQuestion(ctx context.Context, userID, orgID, quizID uuid.UUID, in QuestionInput) (query.QuizQuestion, error) {
	if err := s.staff(ctx, userID, orgID); err != nil {
		return query.QuizQuestion{}, err
	}
	in, err := validateQuestion(in)
	if err != nil {
		return query.QuizQuestion{}, err
	}
	var out query.QuizQuestion
	err = s.inTx(ctx, func(q *query.Queries) error {
		// The quiz lock serializes appends, so positions stay unique.
		cur, err := s.lock(ctx, q, orgID, quizID)
		if err != nil {
			return err
		}
		if err := s.bumpDraft(ctx, q, cur, in.ExpectedVersion); err != nil {
			return err
		}
		pos, err := q.NextQuestionPosition(ctx, quizID)
		if err != nil {
			return fmt.Errorf("next question position: %w", err)
		}
		if pos >= maxQuestions {
			return ErrQuestionLimit
		}
		out, err = q.CreateQuestion(ctx, query.CreateQuestionParams{
			ID: uuid.New(), OrgID: orgID, QuizID: quizID, Position: pos, Prompt: in.Prompt, Options: in.Options,
			Answer: int16(in.Answer), Explanation: in.Explanation, CreatedAt: pgTime(s.now()),
		})
		if err != nil {
			return fmt.Errorf("create question: %w", err)
		}
		return nil
	})
	return out, err
}

// UpdateQuestion replaces a question (teacher/admin). Started attempts keep their snapshot.
func (s *Service) UpdateQuestion(ctx context.Context, userID, orgID, quizID, id uuid.UUID, in QuestionInput) (query.QuizQuestion, error) {
	if err := s.staff(ctx, userID, orgID); err != nil {
		return query.QuizQuestion{}, err
	}
	in, err := validateQuestion(in)
	if err != nil {
		return query.QuizQuestion{}, err
	}
	var out query.QuizQuestion
	err = s.inTx(ctx, func(q *query.Queries) error {
		cur, err := s.lock(ctx, q, orgID, quizID)
		if err != nil {
			return err
		}
		if err := s.bumpDraft(ctx, q, cur, in.ExpectedVersion); err != nil {
			return err
		}
		out, err = q.UpdateQuestion(ctx, query.UpdateQuestionParams{
			OrgID: orgID, QuizID: quizID, ID: id, Prompt: in.Prompt, Options: in.Options,
			Answer: int16(in.Answer), Explanation: in.Explanation, UpdatedAt: pgTime(s.now()),
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("update question: %w", err)
		}
		return nil
	})
	return out, err
}

// DeleteQuestion removes a question (teacher/admin). A published quiz keeps at least one question.
func (s *Service) DeleteQuestion(ctx context.Context, userID, orgID, quizID, id uuid.UUID, expected ...int64) error {
	if err := s.staff(ctx, userID, orgID); err != nil {
		return err
	}
	return s.inTx(ctx, func(q *query.Queries) error {
		z, err := s.lock(ctx, q, orgID, quizID)
		if err != nil {
			return err
		}
		version := int64(0)
		if len(expected) > 0 {
			version = expected[0]
		}
		if err := s.bumpDraft(ctx, q, z, version); err != nil {
			return err
		}
		n, err := q.DeleteQuestion(ctx, query.DeleteQuestionParams{OrgID: orgID, QuizID: quizID, ID: id})
		if err != nil {
			return fmt.Errorf("delete question: %w", err)
		}
		if n == 0 {
			return ErrNotFound
		}
		if z.PublishedAt.Valid {
			left, err := q.NextQuestionPosition(ctx, quizID)
			if err != nil {
				return fmt.Errorf("count questions: %w", err)
			}
			if left == 0 {
				return ErrNoQuestions
			}
		}
		return nil
	})
}

// Reorder sets the question order; ids must list every question exactly once (teacher/admin).
func (s *Service) Reorder(ctx context.Context, userID, orgID, quizID uuid.UUID, ids []uuid.UUID, expected ...int64) error {
	if err := s.staff(ctx, userID, orgID); err != nil {
		return err
	}
	return s.inTx(ctx, func(q *query.Queries) error {
		z, err := s.lock(ctx, q, orgID, quizID)
		if err != nil {
			return err
		}
		version := int64(0)
		if len(expected) > 0 {
			version = expected[0]
		}
		if err := s.bumpDraft(ctx, q, z, version); err != nil {
			return err
		}
		cur, err := q.QuestionIDs(ctx, query.QuestionIDsParams{OrgID: orgID, QuizID: quizID})
		if err != nil {
			return fmt.Errorf("list question ids: %w", err)
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
			if err := q.SetQuestionPosition(ctx, query.SetQuestionPositionParams{QuizID: quizID, ID: id, Position: int32(i)}); err != nil {
				return fmt.Errorf("set question position: %w", err)
			}
		}
		return nil
	})
}

// List returns quizzes with the caller's best score. Participants see only published quizzes and never
// the questions; staff with Drafts also get every question with its answer for authoring.
func (s *Service) List(ctx context.Context, userID, orgID uuid.UUID, f Filter) ([]Quiz, error) {
	role, err := s.auth.Authorize(ctx, userID, orgID, org.RoleParticipant)
	if err != nil {
		return nil, err
	}
	if f.Level != "" && !slices.Contains(Levels, f.Level) {
		return nil, &ValidationError{Fields: map[string]string{"level": "Jenjang tidak dikenal"}}
	}
	authoring := f.Drafts && role.AtLeast(org.RoleTeacher)
	q := query.New(s.pool)
	limit := f.Limit
	if limit == 0 {
		limit = 100
	}
	if limit < 1 || limit > 100 || f.Offset < 0 {
		return nil, &ValidationError{Fields: map[string]string{"pagination": "Pagination tidak valid"}}
	}
	params := query.ListQuizzesParams{PageLimit: limit, Search: f.Search, Status: f.Status, ListOffset: f.Offset, UserID: userID, OrgID: orgID, IncludeDrafts: authoring}
	if f.BuildingKey != "" {
		params.BuildingKey = pgtype.Text{String: f.BuildingKey, Valid: true}
	}
	if f.Level != "" {
		params.Level = pgtype.Text{String: f.Level, Valid: true}
	}
	rows, err := q.ListQuizzes(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list quizzes: %w", err)
	}
	out := make([]Quiz, len(rows))
	for i, r := range rows {
		out[i] = Quiz{Quiz: r.Quiz, TotalCount: r.TotalCount, QuestionCount: r.QuestionCount, BestScore: r.BestScore, Attempts: r.Attempts, ModuleTitles: r.ModuleTitles}
		if !authoring && r.Quiz.PublishedRevisionID.Valid {
			rev, err := q.PublishedQuizRevision(ctx, query.PublishedQuizRevisionParams{OrgID: orgID, QuizID: r.Quiz.ID})
			if err != nil {
				return nil, fmt.Errorf("published quiz: %w", err)
			}
			out[i].Title = rev.Title
			out[i].Description = rev.Description
			out[i].PassScore = rev.PassScore
			out[i].BuildingKey = rev.BuildingKey
			out[i].Level = rev.Level
			var questions []Snapshot
			if err := json.Unmarshal(rev.Questions, &questions); err != nil {
				return nil, fmt.Errorf("published questions: %w", err)
			}
			out[i].QuestionCount = int32(len(questions))
		}
		if authoring {
			if out[i].Questions, err = q.ListQuizQuestions(ctx, r.Quiz.ID); err != nil {
				return nil, fmt.Errorf("list questions: %w", err)
			}
		}
	}
	return out, nil
}
