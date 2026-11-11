package dbtest_test

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/pressly/goose/v3"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/dbtest"
)

func TestRoomLocationHistoryRename(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	var original uint32
	if err := pool.QueryRow(ctx, "SELECT 'room_location_history'::regclass::oid").Scan(&original); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("pgx", pool.Config().ConnString())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, db, os.DirFS("../../../../db/migrations"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Down(ctx); err != nil {
		t.Fatal(err)
	}
	var reverted uint32
	if err := pool.QueryRow(ctx, "SELECT 'mvp2_room_locations'::regclass::oid").Scan(&reverted); err != nil {
		t.Fatal(err)
	}
	if reverted != original {
		t.Fatal("rollback must rename the same table, not recreate it")
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	var restored uint32
	if err := pool.QueryRow(ctx, "SELECT 'room_location_history'::regclass::oid").Scan(&restored); err != nil {
		t.Fatal(err)
	}
	if restored != original {
		t.Fatal("rename must preserve table identity and data")
	}
}
