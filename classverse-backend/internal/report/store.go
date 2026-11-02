package report

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

// Store gives the service access to reports and the moderation log.
type Store struct {
	*query.Queries
}

// NewStore wraps a pgx pool.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{Queries: query.New(pool)}
}
