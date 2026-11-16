package dbtest_test

import (
	"database/sql"
	"os"
	"testing"

	"github.com/pressly/goose/v3"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/dbtest"
)

func TestAssignmentsMigration(t *testing.T) {
	pool := dbtest.New(t)
	db, err := sql.Open("pgx", pool.Config().ConnString())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, db, os.DirFS("../../../../db/migrations"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.DownTo(t.Context(), 14); err != nil {
		t.Fatal(err)
	}
	var absent bool
	if err := pool.QueryRow(t.Context(), `SELECT to_regclass('assignments') IS NULL AND to_regclass('classes') IS NOT NULL`).Scan(&absent); err != nil || !absent {
		t.Fatalf("down=%v %v", absent, err)
	}
	if _, err := provider.Up(t.Context()); err != nil {
		t.Fatal(err)
	}
}
