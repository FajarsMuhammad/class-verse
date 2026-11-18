package quiz

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

func (s *Service) bumpDraft(ctx context.Context, q *query.Queries, m query.Quiz, expected int64) error {
	if expected != 0 && expected != m.DraftVersion {
		return ErrConflict
	}
	if _, err := q.BumpQuizDraft(ctx, query.BumpQuizDraftParams{OrgID: m.OrgID, ID: m.ID, DraftVersion: m.DraftVersion}); errors.Is(err, pgx.ErrNoRows) {
		return ErrConflict
	} else if err != nil {
		return fmt.Errorf("update draft version: %w", err)
	}
	return nil
}

func (s *Service) publishLocked(ctx context.Context, q *query.Queries, orgID, id uuid.UUID) (query.Quiz, error) {
	cur, err := s.lock(ctx, q, orgID, id)
	if err != nil {
		return query.Quiz{}, err
	}
	if _, err := q.QuizRevisionForDraft(ctx, query.QuizRevisionForDraftParams{OrgID: orgID, QuizID: id, DraftVersion: cur.DraftVersion}); err == nil {
		return cur, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return cur, fmt.Errorf("find published draft: %w", err)
	}
	questions, err := q.ListQuizQuestions(ctx, id)
	if err != nil {
		return query.Quiz{}, fmt.Errorf("publish questions: %w", err)
	}
	if len(questions) == 0 {
		return query.Quiz{}, ErrNoQuestions
	}
	r, err := q.InsertQuizRevision(ctx, query.InsertQuizRevisionParams{OrgID: orgID, ID: id, PublishedAt: pgTime(s.now())})
	if errors.Is(err, pgx.ErrNoRows) {
		return s.lock(ctx, q, orgID, id)
	}
	if err != nil {
		return query.Quiz{}, fmt.Errorf("insert quiz revision: %w", err)
	}
	out, err := q.PublishQuizPointer(ctx, query.PublishQuizPointerParams{OrgID: orgID, ID: id, PublishedRevisionID: uuid.NullUUID{UUID: r.ID, Valid: true}, PublishedAt: r.PublishedAt})
	if err != nil {
		return out, fmt.Errorf("publish quiz pointer: %w", err)
	}
	return out, nil
}

// Publish snapshots exactly the expected working version. Retrying that version returns same revision.
func (s *Service) Publish(ctx context.Context, userID, orgID, id uuid.UUID, expected int64) (query.Quiz, error) {
	if err := s.staff(ctx, userID, orgID); err != nil {
		return query.Quiz{}, err
	}
	var out query.Quiz
	err := s.inTx(ctx, func(q *query.Queries) error {
		cur, err := s.lock(ctx, q, orgID, id)
		if err != nil {
			return err
		}
		if expected != cur.DraftVersion {
			return ErrConflict
		}
		out, err = s.publishLocked(ctx, q, orgID, id)
		return err
	})
	return out, err
}

// History lists immutable revision metadata for authorized authors.
func (s *Service) History(ctx context.Context, userID, orgID, id uuid.UUID) ([]query.QuizRevisionHistoryRow, error) {
	if err := s.staff(ctx, userID, orgID); err != nil {
		return nil, err
	}
	rows, err := query.New(s.pool).QuizRevisionHistory(ctx, query.QuizRevisionHistoryParams{OrgID: orgID, QuizID: id})
	if err != nil {
		return nil, fmt.Errorf("quiz history: %w", err)
	}
	return rows, nil
}
