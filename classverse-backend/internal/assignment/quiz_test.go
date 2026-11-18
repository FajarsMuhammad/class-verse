package assignment_test

import (
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/assignment"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/org"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/quiz"
)

type buildings struct{}

func (buildings) Building(string) (bool, bool) { return false, true }
func TestAssignmentQuizCannotReuseCatalogPass(t *testing.T) {
	f := setup(t)
	ctx := t.Context()
	qid := uuid.New()
	if _, err := f.pool.Exec(ctx, `INSERT INTO quizzes(id,org_id,building_key,level,title,pass_score,created_by,created_at,updated_at,published_at) VALUES($1,$2,'sains','SMA','Kuis',60,$3,now(),now(),now())`, qid, f.org, f.admin); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `INSERT INTO quiz_questions(id,org_id,quiz_id,position,prompt,options,answer,explanation,created_at,updated_at) VALUES($1,$2,$3,0,'Pilih A',ARRAY['A','B','C','D'],0,'Jawaban A',now(),now())`, uuid.New(), f.org, qid); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `UPDATE module_steps SET kind='kuis',body='',quiz_id=$1 WHERE id=$2`, qid, f.second); err != nil {
		t.Fatal(err)
	}
	auth := org.NewService(org.NewStore(f.pool), time.Now, slog.New(slog.NewTextHandler(io.Discard, nil)))
	qs := quiz.NewService(f.pool, auth, buildings{}, time.Now)
	if _, err := qs.Publish(ctx, f.admin, f.org, qid, 1); err != nil {
		t.Fatal(err)
	}
	f.publish(t)
	a, err := f.svc.Issue(ctx, f.admin, f.org, assignment.Input{ID: uuid.New(), ClassID: f.class, ModuleID: f.module})
	if err != nil {
		t.Fatal(err)
	}
	steps, _ := f.svc.Steps(ctx, f.student, f.org, a.ID)
	if _, err := qs.StartAssignment(ctx, f.student, f.org, qid, steps[1].ID); !errors.Is(err, quiz.ErrNotFound) {
		t.Fatalf("locked quiz: %v", err)
	}
	catalog, err := qs.Start(ctx, f.student, f.org, qid)
	if err != nil {
		t.Fatal(err)
	}
	if err := qs.Answer(ctx, f.student, f.org, qid, catalog.ID, 0, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := qs.Finish(ctx, f.student, f.org, qid, catalog.ID); err != nil {
		t.Fatal(err)
	}
	steps, _ = f.svc.Steps(ctx, f.student, f.org, a.ID)
	if steps[1].Done {
		t.Fatal("catalog pass bypassed task sequence")
	}
	if err := f.svc.Complete(ctx, f.student, f.org, a.ID, steps[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Complete(ctx, f.student, f.org, a.ID, steps[1].ID); !errors.Is(err, assignment.ErrQuizStep) {
		t.Fatalf("manual quiz complete=%v", err)
	}
	attempt, err := qs.StartAssignment(ctx, f.student, f.org, qid, steps[1].ID)
	if err != nil {
		t.Fatal(err)
	}
	if attempt.ID == catalog.ID || attempt.Questions[0].Answer != -1 {
		t.Fatal("context/key isolation failed")
	}
	if err := qs.Answer(ctx, f.student, f.org, qid, attempt.ID, 0, 0); err != nil {
		t.Fatal(err)
	}
	result, err := qs.Finish(ctx, f.student, f.org, qid, attempt.ID)
	if err != nil || result.Score != 100 {
		t.Fatalf("result=%+v %v", result, err)
	}
	steps, _ = f.svc.Steps(ctx, f.student, f.org, a.ID)
	if !steps[1].Done {
		t.Fatal("context pass did not complete step")
	}
	if _, err := qs.Finish(ctx, f.student, f.org, qid, attempt.ID); err != nil {
		t.Fatal(err)
	}
}
