// Package dbtest creates throwaway PostgreSQL databases for integration tests.
package dbtest

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // database/sql driver for goose
	"github.com/pressly/goose/v3"
)

// New creates an empty database, applies all migrations, and drops it when
// the test ends. It skips the test when TEST_DATABASE_URL is not set; the
// URL must point at a server where the user may create databases.
func New(t *testing.T) *pgxpool.Pool {
	t.Helper()
	admin := os.Getenv("TEST_DATABASE_URL")
	if admin == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping PostgreSQL integration test")
	}
	ctx := context.Background()

	suffix := make([]byte, 6)
	_, _ = rand.Read(suffix)
	name := "kv_test_" + hex.EncodeToString(suffix)

	conn, err := pgx.Connect(ctx, admin)
	if err != nil {
		t.Fatalf("connect admin database: %v", err)
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatalf("create database: %v", err)
	}
	t.Cleanup(func() {
		c, err := pgx.Connect(context.Background(), admin)
		if err != nil {
			t.Logf("drop database: %v", err)
			return
		}
		defer c.Close(context.Background())
		if _, err := c.Exec(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)"); err != nil {
			t.Logf("drop database: %v", err)
		}
	})

	dbURL, err := withDatabase(admin, name)
	if err != nil {
		t.Fatal(err)
	}
	if err := migrate(ctx, dbURL); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func withDatabase(raw, name string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse TEST_DATABASE_URL: %w", err)
	}
	u.Path = "/" + name
	return u.String(), nil
}

func migrate(ctx context.Context, dbURL string) error {
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer db.Close()
	p, err := goose.NewProvider(goose.DialectPostgres, db, os.DirFS(migrationsDir()))
	if err != nil {
		return fmt.Errorf("goose provider: %w", err)
	}
	if _, err := p.Up(ctx); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}

// migrationsDir finds db/migrations relative to this source file, so tests
// work from any package directory.
func migrationsDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "db", "migrations")
}
