package dbtest_test

import (
	"database/sql"
	"os"
	"testing"

	"github.com/pressly/goose/v3"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/dbtest"
)

func TestModuleProgressMigration(t *testing.T) {
	pool := dbtest.New(t)
	ctx := t.Context()
	db, err := sql.Open("pgx", pool.Config().ConnString())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, db, os.DirFS("../../../../db/migrations"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.DownTo(ctx, 10); err != nil {
		t.Fatal(err)
	}
	var absent bool
	if err := pool.QueryRow(ctx, "SELECT to_regclass('module_progress') IS NULL AND to_regclass('module_steps') IS NOT NULL").Scan(&absent); err != nil || !absent {
		t.Fatalf("down: progress absent, steps retained=%v err=%v", absent, err)
	}
	if _, err := provider.UpTo(ctx, 11); err != nil {
		t.Fatal(err)
	}
	var present bool
	if err := pool.QueryRow(ctx, "SELECT to_regclass('module_progress') IS NOT NULL").Scan(&present); err != nil || !present {
		t.Fatalf("up: present=%v err=%v", present, err)
	}
}
