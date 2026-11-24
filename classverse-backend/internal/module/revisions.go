package module

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

func (s *Service) bumpDraft(ctx context.Context, q *query.Queries, m query.Module, expected int64) error {
	if expected != 0 && expected != m.DraftVersion {
		return ErrConflict
	}
	if _, err := q.BumpModuleDraft(ctx, query.BumpModuleDraftParams{OrgID: m.OrgID, ID: m.ID, DraftVersion: m.DraftVersion}); errors.Is(err, pgx.ErrNoRows) {
		return ErrConflict
	} else if err != nil {
		return fmt.Errorf("update draft version: %w", err)
	}
	return nil
}

func (s *Service) publishLocked(ctx context.Context, q *query.Queries, orgID, id uuid.UUID) (query.Module, error) {
	cur, err := s.lock(ctx, q, orgID, id)
	if err != nil {
		return query.Module{}, err
	}
	if _, err := q.ModuleRevisionForDraft(ctx, query.ModuleRevisionForDraftParams{OrgID: orgID, ModuleID: id, DraftVersion: cur.DraftVersion}); err == nil {
		return cur, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return cur, fmt.Errorf("find published draft: %w", err)
	}
	if _, err := q.LockPublishedStepQuizzes(ctx, query.LockPublishedStepQuizzesParams{OrgID: orgID, ModuleID: id}); err != nil {
		return cur, fmt.Errorf("lock quiz bindings: %w", err)
	}
	steps, err := q.ListModuleSteps(ctx, query.ListModuleStepsParams{OrgID: orgID, ModuleIds: []uuid.UUID{id}})
	if err != nil {
		return query.Module{}, fmt.Errorf("publish steps: %w", err)
	}
	if len(steps) == 0 {
		return query.Module{}, &ValidationError{Fields: map[string]string{"steps": "Tambahkan minimal satu langkah"}}
	}
	// Lock file rows before snapshot creation; deletion holds these same locks.
	if _, err := q.LockDraftFileReferences(ctx, query.LockDraftFileReferencesParams{OrgID: orgID, ModuleID: id}); err != nil {
		return query.Module{}, fmt.Errorf("lock published files: %w", err)
	}
	for _, st := range steps {
		if st.ModuleStep.QuizID.Valid {
			id := st.ModuleStep.QuizID.UUID
			if err := checkQuiz(ctx, q, orgID, &id); err != nil {
				return query.Module{}, err
			}
		}
		if st.ModuleStep.FileID.Valid {
			id := st.ModuleStep.FileID.UUID
			if err := checkFile(ctx, q, orgID, &id); err != nil {
				return query.Module{}, err
			}
		}
	}
	r, err := q.InsertModuleRevision(ctx, query.InsertModuleRevisionParams{OrgID: orgID, ID: id, PublishedAt: pgTime(s.now())})
	if errors.Is(err, pgx.ErrNoRows) {
		return s.lock(ctx, q, orgID, id)
	}
	if err != nil {
		return query.Module{}, fmt.Errorf("insert module revision: %w", err)
	}
	if _, err := q.SnapshotModuleSteps(ctx, query.SnapshotModuleStepsParams{OrgID: orgID, ModuleID: id, RevisionID: r.ID}); err != nil {
		return query.Module{}, fmt.Errorf("snapshot module steps: %w", err)
	}
	m, err := q.PublishModulePointer(ctx, query.PublishModulePointerParams{OrgID: orgID, ID: id, PublishedRevisionID: uuid.NullUUID{UUID: r.ID, Valid: true}, PublishedAt: r.PublishedAt})
	if err != nil {
		return m, fmt.Errorf("publish module pointer: %w", err)
	}
	return m, nil
}

// Publish snapshots exactly the expected working version. Retrying that version returns same revision.
func (s *Service) Publish(ctx context.Context, userID, orgID, id uuid.UUID, expected int64) (query.Module, error) {
	if err := s.staff(ctx, userID, orgID); err != nil {
		return query.Module{}, err
	}
	var out query.Module
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
func (s *Service) History(ctx context.Context, userID, orgID, id uuid.UUID) ([]query.ModuleRevisionHistoryRow, error) {
	if err := s.staff(ctx, userID, orgID); err != nil {
		return nil, err
	}
	rows, err := query.New(s.pool).ModuleRevisionHistory(ctx, query.ModuleRevisionHistoryParams{OrgID: orgID, ModuleID: id})
	if err != nil {
		return nil, fmt.Errorf("module history: %w", err)
	}
	return rows, nil
}
