package module

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/org"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

// Counts are the authoring tab counts of the module list.
type Counts = query.CountModulesRow

// Counts returns school-wide authoring counts for the list's non-status filters (teacher/admin).
func (s *Service) Counts(ctx context.Context, userID, orgID uuid.UUID, f Filter) (Counts, error) {
	if _, err := s.auth.Authorize(ctx, userID, orgID, org.RoleTeacher); err != nil {
		return Counts{}, err
	}
	params := query.CountModulesParams{OrgID: orgID, Search: f.Search}
	params.BuildingKey.String, params.BuildingKey.Valid = f.BuildingKey, f.BuildingKey != ""
	params.Level.String, params.Level.Valid = f.Level, f.Level != ""
	c, err := query.New(s.pool).CountModules(ctx, params)
	if err != nil {
		return Counts{}, fmt.Errorf("count modules: %w", err)
	}
	return c, nil
}
