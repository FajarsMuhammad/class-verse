package quiz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/org"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

// unanswered marks a question with no choice yet; it counts as wrong when the attempt is finished.
const unanswered = -1

// Snapshot is a question as it was when the attempt started. Later edits never change a result.
type Snapshot struct {
	ID          uuid.UUID `json:"id"`
	Prompt      string    `json:"prompt"`
	Options     []string  `json:"options"`
	Answer      int       `json:"answer"`
	Explanation string    `json:"explanation"`
}

// Attempt is one try. Answer and Explanation of each question are only filled once Finished.
type Attempt struct {
	ID        uuid.UUID
	QuizID    uuid.UUID
	Questions []Snapshot
	Answers   []int16
	PassScore int16
	Finished  bool
	Correct   int16
	Score     int16
}

// Passed reports whether a finished attempt reached the pass score.
func (a Attempt) Passed() bool { return a.Finished && a.Score >= a.PassScore }

// grade counts correct answers; the score is floored so 59.9 never becomes a pass.
func grade(qs []Snapshot, answers []int16) (correct, score int) {
	for i, q := range qs {
		if i < len(answers) && int(answers[i]) == q.Answer {
			correct++
		}
	}
	if len(qs) == 0 {
		return 0, 0
	}
	return correct, correct * 100 / len(qs)
}

func toAttempt(row query.QuizAttempt) (Attempt, error) {
	var qs []Snapshot
	if err := json.Unmarshal(row.Questions, &qs); err != nil {
		return Attempt{}, fmt.Errorf("decode attempt questions: %w", err)
	}
	a := Attempt{
		ID: row.ID, QuizID: row.QuizID, Questions: qs, Answers: row.Answers, PassScore: row.PassScore,
		Finished: row.FinishedAt.Valid, Correct: row.Correct.Int16, Score: row.Score.Int16,
	}
	if !a.Finished {
		// The key stays on the server until the whole quiz is finished (approved rule 3).
		for i := range a.Questions {
			a.Questions[i].Answer, a.Questions[i].Explanation = unanswered, ""
		}
	}
	return a, nil
}

// Start returns the caller's unfinished attempt for a published quiz, or starts a new one with a snapshot
// of the current questions. Staff may also try drafts to check their own quiz.
func (s *Service) Start(ctx context.Context, userID, orgID, quizID uuid.UUID) (Attempt, error) {
	return s.start(ctx, userID, orgID, quizID, uuid.Nil, uuid.Nil, uuid.Nil)
}

// StartAssignment starts a fresh task-context attempt, never reusing catalog work.
func (s *Service) StartAssignment(ctx context.Context, userID, orgID, quizID, stepID uuid.UUID) (Attempt, error) {
	if stepID == uuid.Nil {
		return Attempt{}, ErrNotFound
	}
	return s.start(ctx, userID, orgID, quizID, stepID, uuid.Nil, uuid.Nil)
}

func (s *Service) start(ctx context.Context, userID, orgID, quizID, stepID, moduleID, moduleStepID uuid.UUID) (Attempt, error) {
	_, err := s.auth.Authorize(ctx, userID, orgID, org.RoleParticipant)
	if err != nil {
		return Attempt{}, err
	}
	var row query.QuizAttempt
	err = s.inTx(ctx, func(q *query.Queries) error {
		assignmentID := uuid.NullUUID{}
		var revisionID, moduleRevisionID uuid.NullUUID

		contextID := uuid.NullUUID{UUID: stepID, Valid: stepID != uuid.Nil}
		if contextID.Valid {
			step, err := q.AssignmentQuizContext(ctx, query.AssignmentQuizContextParams{OrgID: orgID, UserID: userID, StepID: stepID})
			if errors.Is(err, pgx.ErrNoRows) || err == nil && step.QuizID.UUID != quizID {
				return ErrNotFound
			}
			if err != nil {
				return fmt.Errorf("assignment quiz context: %w", err)
			}
			assignmentID = uuid.NullUUID{UUID: step.AssignmentID, Valid: true}
			revisionID = step.QuizRevisionID
			if _, err := q.LockAssignment(ctx, query.LockAssignmentParams{OrgID: orgID, ID: step.AssignmentID}); err != nil {
				return err
			}
			if _, err := q.AssignmentQuizContext(ctx, query.AssignmentQuizContextParams{OrgID: orgID, UserID: userID, StepID: stepID}); err != nil {
				return ErrNotFound
			}
		}
		// The quiz lock serializes concurrent starts, so the open-attempt index never races.
		z, err := q.LockAttemptQuiz(ctx, query.LockAttemptQuizParams{OrgID: orgID, ID: quizID})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("lock attempt quiz: %w", err)
		}
		if (!z.PublishedAt.Valid || z.ArchivedAt.Valid) && !contextID.Valid && moduleID == uuid.Nil {
			return ErrNotFound
		}
		if moduleID != uuid.Nil {
			if contextID.Valid {
				return ErrNotFound
			}
			if err := q.PinModuleRevision(ctx, query.PinModuleRevisionParams{OrgID: orgID, ModuleID: moduleID, UserID: userID}); err != nil {
				return fmt.Errorf("pin quiz module: %w", err)
			}
			rev, err := q.ModuleQuizRevision(ctx, query.ModuleQuizRevisionParams{OrgID: orgID, ModuleID: moduleID, StepID: moduleStepID, UserID: userID, QuizID: uuid.NullUUID{UUID: quizID, Valid: true}})
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			if err != nil {
				return fmt.Errorf("module quiz binding: %w", err)
			}
			revisionID = uuid.NullUUID{UUID: rev.ID, Valid: true}
			mr, err := q.ReadModuleRevision(ctx, query.ReadModuleRevisionParams{OrgID: orgID, ModuleID: moduleID, UserID: userID})
			if err != nil {
				return fmt.Errorf("module quiz revision: %w", err)
			}
			moduleRevisionID = uuid.NullUUID{UUID: mr.ID, Valid: true}
		}
		if !revisionID.Valid && !contextID.Valid {
			revisionID = z.PublishedRevisionID
		}
		row, err = q.OpenAttempt(ctx, query.OpenAttemptParams{QuizID: quizID, UserID: userID, AssignmentStepID: contextID, ModuleRevisionID: moduleRevisionID})
		if err == nil {
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("open attempt: %w", err)
		}
		var snap []Snapshot
		var passScore int16
		if revisionID.Valid {
			rev, err := q.GetQuizRevision(ctx, query.GetQuizRevisionParams{OrgID: orgID, QuizID: quizID, ID: revisionID.UUID})
			if err != nil {
				return fmt.Errorf("quiz revision: %w", err)
			}
			if err := json.Unmarshal(rev.Questions, &snap); err != nil {
				return fmt.Errorf("revision questions: %w", err)
			}
			passScore = rev.PassScore
		} else {
			return ErrNotFound
		}

		if len(snap) == 0 {
			return ErrNoQuestions
		}
		answers := make([]int16, len(snap))
		for i := range answers {
			answers[i] = unanswered
		}
		raw, err := json.Marshal(snap)
		if err != nil {
			return fmt.Errorf("encode snapshot: %w", err)
		}
		row, err = q.CreateAttempt(ctx, query.CreateAttemptParams{
			ID: uuid.New(), OrgID: orgID, QuizID: quizID, UserID: userID, Questions: raw, Answers: answers,
			QuizRevisionID: revisionID, ModuleID: uuid.NullUUID{UUID: moduleID, Valid: moduleID != uuid.Nil}, ModuleRevisionID: moduleRevisionID, PassScore: passScore, StartedAt: pgTime(s.now()), AssignmentID: assignmentID, AssignmentStepID: contextID,
		})
		if err != nil {
			return fmt.Errorf("create attempt: %w", err)
		}
		return nil
	})
	if err != nil {
		return Attempt{}, err
	}
	return toAttempt(row)
}

// lockOwn locks the caller's own attempt; anything else (other user, other school) is not found.
func (s *Service) lockOwn(ctx context.Context, q *query.Queries, userID, orgID, quizID, attemptID uuid.UUID) (query.QuizAttempt, error) {
	row, err := q.LockAttempt(ctx, query.LockAttemptParams{OrgID: orgID, QuizID: quizID, ID: attemptID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return row, ErrNotFound
	}
	if err != nil {
		return row, fmt.Errorf("lock attempt: %w", err)
	}
	return row, nil
}

// Answer records or changes one choice. Repeating the same answer is harmless; finished attempts are immutable.
func (s *Service) Answer(ctx context.Context, userID, orgID, quizID, attemptID uuid.UUID, index, choice int) error {
	if _, err := s.auth.Authorize(ctx, userID, orgID, org.RoleParticipant); err != nil {
		return err
	}
	return s.inTx(ctx, func(q *query.Queries) error {
		row, err := s.lockOwn(ctx, q, userID, orgID, quizID, attemptID)
		if err != nil {
			return err
		}
		if row.FinishedAt.Valid {
			return ErrFinished
		}
		if row.AssignmentID.Valid {
			if _, err := q.LockAssignment(ctx, query.LockAssignmentParams{OrgID: orgID, ID: row.AssignmentID.UUID}); err != nil {
				return err
			}
			if _, err := q.AssignmentQuizContext(ctx, query.AssignmentQuizContextParams{OrgID: orgID, UserID: userID, StepID: row.AssignmentStepID.UUID}); err != nil {
				return ErrNotFound
			}
		}
		if index < 0 || index >= len(row.Answers) {
			return &ValidationError{Fields: map[string]string{"index": "Soal tidak ada"}}
		}
		var snap []Snapshot
		if err := json.Unmarshal(row.Questions, &snap); err != nil {
			return fmt.Errorf("answer snapshot: %w", err)
		}
		if index >= len(snap) || choice < 0 || choice >= len(snap[index].Options) {
			return &ValidationError{Fields: map[string]string{"choice": "Pilih jawaban yang tersedia"}}
		}
		answers := append([]int16(nil), row.Answers...)
		answers[index] = int16(choice)
		if err := q.SetAnswers(ctx, query.SetAnswersParams{ID: attemptID, Answers: answers}); err != nil {
			return fmt.Errorf("save answer: %w", err)
		}
		return nil
	})
}

// Finish grades the attempt once. Calling it again returns the same stored result (idempotent).
func (s *Service) Finish(ctx context.Context, userID, orgID, quizID, attemptID uuid.UUID) (Attempt, error) {
	if _, err := s.auth.Authorize(ctx, userID, orgID, org.RoleParticipant); err != nil {
		return Attempt{}, err
	}
	var row query.QuizAttempt
	err := s.inTx(ctx, func(q *query.Queries) error {
		var err error
		if row, err = s.lockOwn(ctx, q, userID, orgID, quizID, attemptID); err != nil {
			return err
		}
		if row.FinishedAt.Valid {
			return nil
		}
		if row.AssignmentID.Valid {
			if _, err := q.LockAssignment(ctx, query.LockAssignmentParams{OrgID: orgID, ID: row.AssignmentID.UUID}); err != nil {
				return err
			}
			if _, err := q.AssignmentQuizContext(ctx, query.AssignmentQuizContextParams{OrgID: orgID, UserID: userID, StepID: row.AssignmentStepID.UUID}); err != nil {
				return ErrNotFound
			}
		}
		var qs []Snapshot
		if err := json.Unmarshal(row.Questions, &qs); err != nil {
			return fmt.Errorf("decode attempt questions: %w", err)
		}
		correct, score := grade(qs, row.Answers)
		row, err = q.FinishAttempt(ctx, query.FinishAttemptParams{
			ID: attemptID, FinishedAt: pgTime(s.now()),
			Correct: pgInt2(correct), Score: pgInt2(score),
		})
		if err != nil {
			return fmt.Errorf("finish attempt: %w", err)
		}
		// Same transaction: a pass and its module progress are saved together or not at all.
		if score >= int(row.PassScore) && row.AssignmentID.Valid {
			err = q.CompleteAssignmentStep(ctx, query.CompleteAssignmentStepParams{OrgID: orgID, AssignmentID: row.AssignmentID.UUID, UserID: userID, StepID: row.AssignmentStepID.UUID, CompletedAt: row.FinishedAt})
			if err != nil {
				return fmt.Errorf("complete assignment quiz: %w", err)
			}
		}
		if score >= int(row.PassScore) && !row.AssignmentID.Valid && !row.ModuleRevisionID.Valid && row.QuizRevisionID.Valid {
			if err := q.PinQuizModules(ctx, query.PinQuizModulesParams{OrgID: orgID, UserID: userID, QuizID: uuid.NullUUID{UUID: quizID, Valid: true}, QuizRevisionID: row.QuizRevisionID}); err != nil {
				return fmt.Errorf("pin quiz modules: %w", err)
			}
		}
		if score >= int(row.PassScore) && !row.AssignmentID.Valid {
			err = q.CompleteQuizSteps(ctx, query.CompleteQuizStepsParams{OrgID: orgID, UserID: userID, QuizID: uuid.NullUUID{UUID: quizID, Valid: true}, QuizRevisionID: row.QuizRevisionID, ModuleRevisionID: row.ModuleRevisionID, CompletedAt: row.FinishedAt})
			if err != nil {
				return fmt.Errorf("complete quiz steps: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return Attempt{}, err
	}
	return toAttempt(row)
}

func pgInt2(v int) pgtype.Int2 { return pgtype.Int2{Int16: int16(v), Valid: true} }

// StartModule resolves quiz revision from caller's module pin, never browser-supplied revision IDs.
func (s *Service) StartModule(ctx context.Context, userID, orgID, quizID, moduleID, stepID uuid.UUID) (Attempt, error) {
	if moduleID == uuid.Nil || stepID == uuid.Nil {
		return Attempt{}, ErrNotFound
	}
	return s.start(ctx, userID, orgID, quizID, uuid.Nil, moduleID, stepID)
}
