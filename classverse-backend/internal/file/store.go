package file

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

// Store runs short metadata transactions; never hold one during object transfer.
type Store struct{ pool *pgxpool.Pool }

// NewStore wraps a database pool.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func (s *Store) inTx(ctx context.Context, fn func(*query.Queries) error) error {
	return db.InTx(ctx, s.pool, fn)
}

func fileError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
