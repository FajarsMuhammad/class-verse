package invite

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

// Store gives the service access to invites and memberships.
type Store struct {
	pool *pgxpool.Pool
	*query.Queries
}

// NewStore wraps a pgx pool.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool, Queries: query.New(pool)}
}

// InTx runs fn inside one transaction.
func (s *Store) InTx(ctx context.Context, fn func(q *query.Queries) error) error {
	if err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error { return fn(s.WithTx(tx)) }); err != nil {
		return fmt.Errorf("transaction: %w", err)
	}
	return nil
}
