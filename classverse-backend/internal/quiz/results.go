package quiz

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

// Counts are the authoring tab counts and the number of quizzes without questions.
type Counts = query.CountQuizzesRow

// Counts returns school-wide authoring counts for the list's non-status filters (teacher/admin).
func (s *Service) Counts(ctx context.Context, userID, orgID uuid.UUID, f Filter) (Counts, error) {
	if err := s.staff(ctx, userID, orgID); err != nil {
		return Counts{}, err
	}
	params := query.CountQuizzesParams{OrgID: orgID, Search: f.Search}
	params.BuildingKey.String, params.BuildingKey.Valid = f.BuildingKey, f.BuildingKey != ""
	params.Level.String, params.Level.Valid = f.Level, f.Level != ""
	c, err := query.New(s.pool).CountQuizzes(ctx, params)
	if err != nil {
		return Counts{}, fmt.Errorf("count quizzes: %w", err)
	}
	return c, nil
}

// Results returns one page of participants' best finished score per quiz (teacher/admin),
// the same highest-score rule as the Rapor.
func (s *Service) Results(ctx context.Context, userID, orgID, quizID uuid.UUID, limit, offset int32) ([]query.QuizResultsRow, error) {
	if err := s.staff(ctx, userID, orgID); err != nil {
		return nil, err
	}
	q := query.New(s.pool)
	ok, err := q.QuizExists(ctx, query.QuizExistsParams{OrgID: orgID, ID: quizID})
	if err != nil {
		return nil, fmt.Errorf("quiz exists: %w", err)
	}
	if !ok {
		return nil, ErrNotFound
	}
	rows, err := q.QuizResults(ctx, query.QuizResultsParams{OrgID: orgID, QuizID: quizID, PageLimit: limit, PageOffset: offset})
	if err != nil {
		return nil, fmt.Errorf("quiz results: %w", err)
	}
	return rows, nil
}
