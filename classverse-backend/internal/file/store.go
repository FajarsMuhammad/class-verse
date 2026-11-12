package file

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

// Store runs short metadata transactions; never hold one during object transfer.
type Store struct{ pool *pgxpool.Pool }

// NewStore wraps a database pool.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func (s *Store) inTx(ctx context.Context, fn func(*query.Queries) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin file transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(query.New(tx)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit file transaction: %w", err)
	}
	return nil
}

func fileError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
