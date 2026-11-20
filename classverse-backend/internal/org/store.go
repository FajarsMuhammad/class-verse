// Package org manages organizations, memberships, and roles.
package org

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

// Store gives the service access to organizations and memberships.
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
	return db.InTx(ctx, s.pool, fn)
}
