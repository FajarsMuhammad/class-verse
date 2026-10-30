// Package auth handles registration, email and Google login, access and
// refresh tokens, and the user profile.
package auth

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

// Store gives the service access to users, OTPs, and refresh tokens.
type Store struct {
	pool *pgxpool.Pool
	*query.Queries
}

// NewStore wraps a pgx pool.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool, Queries: query.New(pool)}
}

// InTx runs fn inside one transaction. fn's error rolls the transaction back.
func (s *Store) InTx(ctx context.Context, fn func(q *query.Queries) error) error {
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		return fn(s.WithTx(tx))
	})
	if err != nil {
		return fmt.Errorf("transaction: %w", err)
	}
	return nil
}
