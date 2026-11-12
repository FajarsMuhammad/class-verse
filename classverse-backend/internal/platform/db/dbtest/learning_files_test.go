package dbtest_test

import (
	"database/sql"
	"os"
	"testing"

	"github.com/pressly/goose/v3"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/dbtest"
)

func TestLearningFilesMigration(t *testing.T) {
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
	if _, err := provider.DownTo(ctx, 5); err != nil {
		t.Fatal(err)
	}
	var absent bool
	if err := pool.QueryRow(ctx, "SELECT to_regclass('files') IS NULL AND to_regclass('file_storage') IS NULL").Scan(&absent); err != nil || !absent {
		t.Fatalf("down: absent=%v err=%v", absent, err)
	}
	if _, err := provider.UpTo(ctx, 6); err != nil {
		t.Fatal(err)
	}
	var present bool
	if err := pool.QueryRow(ctx, "SELECT to_regclass('files') IS NOT NULL AND to_regclass('file_storage') IS NOT NULL").Scan(&present); err != nil || !present {
		t.Fatalf("up: present=%v err=%v", present, err)
	}
}
