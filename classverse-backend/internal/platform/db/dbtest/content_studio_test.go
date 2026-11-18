package dbtest_test

import (
	"database/sql"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/pressly/goose/v3"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/dbtest"
)

func TestContentStudioMigrationBackfillAndRollback(t *testing.T) {
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
	if _, err := provider.DownTo(ctx, 19); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.DownTo(ctx, 19); err != nil {
		t.Fatal(err)
	}
	admin, student, school, module, step, quiz, question, attempt := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO users(id,email,display_name) VALUES($1,$2,'Admin'),($3,$4,'Student')`, admin, admin.String()+"@test.invalid", student, student.String()+"@test.invalid")
	exec(`INSERT INTO organizations(id,name,slug,created_by) VALUES($1,'Legacy','legacy-cs',$2)`, school, admin)
	exec(`INSERT INTO memberships(org_id,user_id,role) VALUES($1,$2,'admin'),($1,$3,'participant')`, school, admin, student)
	exec(`INSERT INTO modules(id,org_id,building_key,level,title,created_by,created_at,updated_at,published_at) VALUES($1,$2,'sains','SMA','Legacy module',$3,now(),now(),now())`, module, school, admin)
	exec(`INSERT INTO module_steps(id,org_id,module_id,position,kind,title,body,minutes,created_at,updated_at) VALUES($1,$2,$3,0,'materi','Legacy step',E'Line one\n<script>literal</script>',5,now(),now())`, step, school, module)
	exec(`INSERT INTO module_progress(org_id,user_id,step_id,completed_at) VALUES($1,$2,$3,'2026-01-01T00:00:00Z')`, school, student, step)
	exec(`INSERT INTO quizzes(id,org_id,building_key,level,title,created_by,created_at,updated_at,published_at) VALUES($1,$2,'sains','SMA','Legacy quiz',$3,now(),now(),now())`, quiz, school, admin)
	exec(`INSERT INTO quiz_questions(id,org_id,quiz_id,position,prompt,options,answer,created_at,updated_at) VALUES($1,$2,$3,0,'Legacy?',ARRAY['A','B','C','D'],0,now(),now())`, question, school, quiz)
	exec(`INSERT INTO quiz_attempts(id,org_id,quiz_id,user_id,questions,answers,pass_score,started_at,finished_at,correct,score) VALUES($1,$2,$3,$4,'[{"prompt":"Old snapshot","options":["A","B","C","D"],"answer":0,"explanation":"Old"}]','{0}',60,now(),now(),1,100)`, attempt, school, quiz, student)
	if _, err := provider.UpTo(ctx, 20); err != nil {
		t.Fatal(err)
	}
	var before21 int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM module_revisions WHERE module_id=$1", module).Scan(&before21); err != nil || before21 != 1 {
		t.Fatalf("existing20 checkpoint: %d %v", before21, err)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	var text string
	if err := pool.QueryRow(ctx, `SELECT rich_content->'content'->0->'content'->0->>'text' FROM module_steps WHERE id=$1`, step).Scan(&text); err != nil || text != "Line one\n<script>literal</script>" {
		t.Fatalf("literal backfill: %q %v", text, err)
	}
	var pins, revisions, score int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM module_revision_pins WHERE user_id=$1`, student).Scan(&pins); err != nil || pins != 1 {
		t.Fatalf("pins=%d %v", pins, err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM module_revisions WHERE module_id=$1`, module).Scan(&revisions); err != nil || revisions != 1 {
		t.Fatalf("revisions=%d %v", revisions, err)
	}
	if err := pool.QueryRow(ctx, `SELECT score FROM quiz_attempts WHERE id=$1 AND questions->0->>'prompt'='Old snapshot'`, attempt).Scan(&score); err != nil || score != 100 {
		t.Fatalf("attempt changed=%d %v", score, err)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.DownTo(ctx, 19); err == nil {
		t.Fatal("unsafe rollback accepted versioned data")
	}
	var retained bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM module_revision_pins WHERE user_id=$1)`, student).Scan(&retained); err != nil || !retained {
		t.Fatalf("rollback lost pin: %v %v", retained, err)
	}
}

func TestContentStudioPreflightRepairsLegacyQuizReference(t *testing.T) {
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
	raw, err := os.ReadFile("../../../../db/upgrades/content_studio_preflight.sql")
	if err != nil {
		t.Fatal(err)
	}
	runPreflight := func() {
		t.Helper()
		c, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer c.Release()
		if _, err := c.Conn().PgConn().Exec(ctx, string(raw)).ReadAll(); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := provider.DownTo(ctx, 19); err != nil {
		t.Fatal(err)
	}
	admin, student, school, module, material, quizStep, quiz := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO users(id,email,display_name) VALUES($1,$2,'Admin'),($3,$4,'Student')`, admin, admin.String()+"@test.invalid", student, student.String()+"@test.invalid")
	exec(`INSERT INTO organizations(id,name,slug,created_by) VALUES($1,'Edge','preflight-edge',$2)`, school, admin)
	exec(`INSERT INTO memberships(org_id,user_id,role) VALUES($1,$2,'admin'),($1,$3,'participant')`, school, admin, student)
	exec(`INSERT INTO quizzes(id,org_id,building_key,level,title,created_by,created_at,updated_at) VALUES($1,$2,'sains','SMA','Unpublished',$3,now(),now())`, quiz, school, admin)
	exec(`INSERT INTO quiz_questions(id,org_id,quiz_id,position,prompt,options,answer,created_at,updated_at) VALUES($1,$2,$3,0,'Edge?',ARRAY['A','B','C','D'],0,now(),now())`, uuid.New(), school, quiz)
	exec(`INSERT INTO modules(id,org_id,building_key,level,title,created_by,created_at,updated_at) VALUES($1,$2,'sains','SMA','Unpublished legacy',$3,now(),now())`, module, school, admin)
	exec(`INSERT INTO module_steps(id,org_id,module_id,position,kind,title,body,minutes,created_at,updated_at) VALUES($1,$2,$3,0,'materi','Read','Body',1,now(),now())`, material, school, module)
	exec(`INSERT INTO module_steps(id,org_id,module_id,position,kind,title,body,quiz_id,minutes,created_at,updated_at) VALUES($1,$2,$3,1,'kuis','Quiz','',$4,1,now(),now())`, quizStep, school, module, quiz)
	exec(`INSERT INTO module_progress(org_id,user_id,step_id,completed_at) VALUES($1,$2,$3,now())`, school, student, material)
	if _, err := provider.UpTo(ctx, 20); err != nil {
		t.Fatal(err)
	}
	runPreflight()
	runPreflight()
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("21 after preflight: %v", err)
	}
	runPreflight()
	var published bool
	var revisions, bound int
	if err := pool.QueryRow(ctx, `SELECT published_at IS NOT NULL OR published_revision_id IS NOT NULL FROM quizzes WHERE id=$1`, quiz).Scan(&published); err != nil || published {
		t.Fatalf("draft quiz published: %v %v", published, err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM quiz_revisions WHERE quiz_id=$1`, quiz).Scan(&revisions); err != nil || revisions != 1 {
		t.Fatalf("idempotent revisions=%d %v", revisions, err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM module_revision_steps WHERE id=$1 AND quiz_revision_id IS NOT NULL`, quizStep).Scan(&bound); err != nil || bound != 1 {
		t.Fatalf("historical quiz binding=%d %v", bound, err)
	}
}
