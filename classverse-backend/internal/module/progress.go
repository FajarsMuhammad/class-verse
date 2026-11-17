package module

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/org"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

// Resume is the module to continue from the left panel ("Lanjutkan belajar").
type Resume struct {
	ModuleID    uuid.UUID
	BuildingKey string
	Level       string
	Title       string
	Done        int32
	Total       int32
	NextTitle   string
}

// SetDone marks a step done or not done for the caller. Steps are free to open in any order
// (approved 2026-11-13), so no ordering check. Participants only touch steps of published modules.
func (s *Service) SetDone(ctx context.Context, userID, orgID, moduleID, stepID uuid.UUID, done bool) error {
	role, err := s.auth.Authorize(ctx, userID, orgID, org.RoleParticipant)
	if err != nil {
		return err
	}
	return s.inTx(ctx, func(q *query.Queries) error {
		// Serialize with authoring so archive/unpublish cannot race the visibility check.
		if _, err := s.lock(ctx, q, orgID, moduleID); err != nil {
			return err
		}
		kind, err := q.ReadableStep(ctx, query.ReadableStepParams{OrgID: orgID, ModuleID: moduleID, ID: stepID, Staff: role.AtLeast(org.RoleTeacher)})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("get step: %w", err)
		}
		// A quiz step is completed only by passing the quiz (approved rule 4), never by hand.
		if kind == "kuis" {
			return ErrQuizStep
		}
		if done {
			err = q.CompleteStep(ctx, query.CompleteStepParams{OrgID: orgID, UserID: userID, StepID: stepID, CompletedAt: pgTime(s.now())})
		} else {
			err = q.UncompleteStep(ctx, query.UncompleteStepParams{OrgID: orgID, UserID: userID, StepID: stepID})
		}
		if err != nil {
			return fmt.Errorf("set step progress: %w", err)
		}
		return nil
	})
}

// ResumeFor returns the module the caller worked on last that still has unfinished steps, or nil.
func (s *Service) ResumeFor(ctx context.Context, userID, orgID uuid.UUID) (*Resume, error) {
	if _, err := s.auth.Authorize(ctx, userID, orgID, org.RoleParticipant); err != nil {
		return nil, err
	}
	r, err := query.New(s.pool).ResumeModule(ctx, query.ResumeModuleParams{OrgID: orgID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("resume module: %w", err)
	}
	return &Resume{ModuleID: r.ID, BuildingKey: r.BuildingKey, Level: r.Level, Title: r.Title, Done: r.Done, Total: r.Total, NextTitle: r.NextTitle}, nil
}
