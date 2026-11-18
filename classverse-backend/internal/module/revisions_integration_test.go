package module_test

import (
	"testing"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/dbtest"
)

func TestContentStudioSchema(t *testing.T) {
	pool := dbtest.New(t)
	ctx := t.Context()
	for _, table := range []string{"module_revisions", "module_revision_steps", "module_revision_pins", "quiz_revisions"} {
		var exists bool
		if err := pool.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", table).Scan(&exists); err != nil || !exists {
			t.Fatalf("missing %s: %v", table, err)
		}
	}
}
