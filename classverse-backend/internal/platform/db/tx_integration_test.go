package db_test

import (
	"context"
	"errors"
	"testing"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/dbtest"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

func TestInTxCommitsOrRollsBack(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	count := func() int {
		var n int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM users").Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	boom := errors.New("boom")
	err := db.InTx(ctx, pool, func(q *query.Queries) error {
		if _, err := q.CreateTestUser(ctx, query.CreateTestUserParams{Email: "a@x.test", DisplayName: "A"}); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) || count() != 0 {
		t.Fatalf("rollback: err=%v users=%d", err, count())
	}
	if err := db.InTx(ctx, pool, func(q *query.Queries) error {
		_, err := q.CreateTestUser(ctx, query.CreateTestUserParams{Email: "b@x.test", DisplayName: "B"})
		return err
	}); err != nil || count() != 1 {
		t.Fatalf("commit: err=%v users=%d", err, count())
	}
}
