package db

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

// InTx runs fn in one transaction: commit when fn returns nil, roll back otherwise.
// fn's error is returned unwrapped so callers can still match domain sentinels.
func InTx(ctx context.Context, pool *pgxpool.Pool, fn func(*query.Queries) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(query.New(tx)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// Time converts t to a non-null UTC timestamptz.
func Time(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t.UTC(), Valid: true} }

// Contains turns user text into a literal ILIKE "contains" pattern; empty text means no filter.
func Contains(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: "%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s) + "%", Valid: true}
}
