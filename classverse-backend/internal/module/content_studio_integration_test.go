package module_test

import (
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/file"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/module"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/org"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/dbtest"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/quiz"
)

func TestContentStudioRevisionPinsAndQuizBindings(t *testing.T) {
	p := dbtest.New(t)
	ctx := t.Context()
	admin, old, newStudent, school := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	for _, id := range []uuid.UUID{admin, old, newStudent} {
		if _, err := p.Exec(ctx, `INSERT INTO users(id,email,display_name) VALUES($1,$2,'Studio')`, id, id.String()+"@test.invalid"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := p.Exec(ctx, `INSERT INTO organizations(id,name,slug,created_by) VALUES($1,'Studio','studio-test',$2)`, school, admin); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, `INSERT INTO memberships(org_id,user_id,role) VALUES($1,$2,'admin'),($1,$3,'participant'),($1,$4,'participant')`, school, admin, old, newStudent); err != nil {
		t.Fatal(err)
	}
	auth := org.NewService(org.NewStore(p), time.Now, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ms := module.NewService(p, auth, buildings{"sains": false}, time.Now)
	qs := quiz.NewService(p, auth, buildings{"sains": false}, time.Now)
	z, err := qs.Create(ctx, admin, school, quiz.Input{BuildingKey: "sains", Level: "SMA", Title: "Quiz"})
	if err != nil {
		t.Fatal(err)
	}
	question, err := qs.AddQuestion(ctx, admin, school, z.ID, quiz.QuestionInput{Prompt: "Benar?", Options: []string{"Benar", "Salah"}, Answer: 0})
	if err != nil {
		t.Fatal(err)
	}
	z, err = qs.Publish(ctx, admin, school, z.ID, 2)
	if err != nil {
		t.Fatal(err)
	}
	m, err := ms.Create(ctx, admin, school, module.Input{BuildingKey: "sains", Level: "SMA", Title: "Original"})
	if err != nil {
		t.Fatal(err)
	}
	st, err := ms.AddStep(ctx, admin, school, m.ID, module.StepInput{Kind: "materi", Title: "Original step", Body: "Original", Minutes: 1})
	if err != nil {
		t.Fatal(err)
	}
	quizStep, err := ms.AddStep(ctx, admin, school, m.ID, module.StepInput{Kind: "kuis", Title: "Quiz step", QuizID: &z.ID, Minutes: 1})
	if err != nil {
		t.Fatal(err)
	}
	m, err = ms.Publish(ctx, admin, school, m.ID, 3)
	if err != nil {
		t.Fatal(err)
	}
	firstRevision := m.PublishedRevisionID
	again, err := ms.Publish(ctx, admin, school, m.ID, 3)
	if err != nil || again.PublishedRevisionID != firstRevision {
		t.Fatalf("publish retry: %v", err)
	}
	list, err := ms.List(ctx, old, school, module.Filter{})
	if err != nil || list[0].Title != "Original" {
		t.Fatalf("old read: %+v %v", list, err)
	}
	// Browsing is read-only: a learner who only viewed the list is not pinned to this revision.
	if _, err := ms.List(ctx, newStudent, school, module.Filter{}); err != nil {
		t.Fatal(err)
	}
	var browsePins int
	if err := p.QueryRow(ctx, `SELECT count(*) FROM module_revision_pins WHERE user_id=$1`, newStudent).Scan(&browsePins); err != nil || browsePins != 0 {
		t.Fatalf("list must not pin: %d %v", browsePins, err)
	}
	if err := ms.SetDone(ctx, old, school, m.ID, st.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := ms.UpdateStep(ctx, admin, school, m.ID, st.ID, module.StepInput{ExpectedVersion: 3, Kind: "materi", Title: "New step", Body: "New", Minutes: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := ms.UpdateStep(ctx, admin, school, m.ID, st.ID, module.StepInput{ExpectedVersion: 3, Kind: "materi", Title: "Stale", Body: "Stale", Minutes: 1}); !errors.Is(err, module.ErrConflict) {
		t.Fatalf("stale edit: %v", err)
	}
	// Shared quiz draft changes are NOT included in module publication.
	if _, err := qs.UpdateQuestion(ctx, admin, school, z.ID, question.ID, quiz.QuestionInput{ExpectedVersion: 2, Prompt: "Changed?", Options: []string{"Benar", "Salah"}, Answer: 1}); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 2 {
		wg.Go(func() {
			if _, err := ms.Publish(ctx, admin, school, m.ID, 4); err != nil {
				t.Errorf("concurrent publish: %v", err)
			}
		})
	}
	wg.Wait()
	list, err = ms.List(ctx, old, school, module.Filter{})
	if err != nil || list[0].Steps[0].Body != "Original" || !list[0].Steps[0].Done {
		t.Fatalf("old pin altered: %+v %v", list, err)
	}
	list, err = ms.List(ctx, newStudent, school, module.Filter{})
	if err != nil || list[0].Steps[0].Body != "New" || list[0].Steps[0].Done {
		t.Fatalf("new pin: %+v %v", list, err)
	}
	a, err := qs.StartModule(ctx, old, school, z.ID, m.ID, quizStep.ID)
	if err != nil || a.Questions[0].Prompt != "Benar?" || a.Questions[0].Answer != -1 {
		t.Fatalf("bound quiz: %+v %v", a, err)
	}
	if err := qs.Answer(ctx, old, school, z.ID, a.ID, 0, 2); err == nil {
		t.Fatal("two-option quiz accepted invalid choice")
	}
	if err := qs.Answer(ctx, old, school, z.ID, a.ID, 0, 0); err != nil {
		t.Fatal(err)
	}
	a, err = qs.Finish(ctx, old, school, z.ID, a.ID)
	if err != nil || a.Score != 100 || a.Questions[0].Answer != 0 {
		t.Fatalf("finish: %+v %v", a, err)
	}
	if _, err := p.Exec(ctx, `UPDATE module_revisions SET title='tamper' WHERE id=$1`, firstRevision.UUID); err == nil {
		t.Fatal("revision mutable")
	}
	var n int
	if err := p.QueryRow(ctx, `SELECT count(*) FROM module_revisions WHERE module_id=$1`, m.ID).Scan(&n); err != nil || n != 2 {
		t.Fatalf("revision count=%d %v", n, err)
	}
	if rows, err := ms.History(ctx, admin, school, m.ID); err != nil || len(rows) != 2 {
		t.Fatalf("module history: %v %v", rows, err)
	}
	if _, err := ms.History(ctx, old, school, m.ID); err == nil {
		t.Fatal("participant history allowed")
	}
	if rows, err := qs.History(ctx, admin, school, z.ID); err != nil || len(rows) != 1 {
		t.Fatalf("quiz history: %v %v", rows, err)
	}
	if _, err := qs.Publish(ctx, old, school, z.ID, 3); err == nil {
		t.Fatal("participant publish allowed")
	}
	if _, err := qs.Publish(ctx, admin, school, z.ID, 2); !errors.Is(err, quiz.ErrConflict) {
		t.Fatalf("stale quiz publish: %v", err)
	}
	z, err = qs.Publish(ctx, admin, school, z.ID, 3)
	if err != nil {
		t.Fatal(err)
	}
	againQuiz, err := qs.Publish(ctx, admin, school, z.ID, 3)
	if err != nil || againQuiz.PublishedRevisionID != z.PublishedRevisionID {
		t.Fatalf("quiz publish retry: %v", err)
	}
	if _, err := qs.History(ctx, old, school, z.ID); err == nil {
		t.Fatal("participant quiz history allowed")
	}
	// Draft metadata cannot move published catalog or a learner's pinned revision.
	m, err = ms.Update(ctx, admin, school, m.ID, module.Input{ExpectedVersion: 4, BuildingKey: "sains", Level: "SD", Title: "Draft metadata"})
	if err != nil {
		t.Fatal(err)
	}
	list, err = ms.List(ctx, old, school, module.Filter{Level: "SMA"})
	if err != nil || len(list) != 1 || list[0].Title != "Original" {
		t.Fatalf("draft catalog leak: %v %v", list, err)
	}
	if _, err := ms.Publish(ctx, admin, school, m.ID, 4); !errors.Is(err, module.ErrConflict) {
		t.Fatalf("stale module publish: %v", err)
	}
	if _, err := ms.Publish(ctx, old, school, m.ID, 5); err == nil {
		t.Fatal("participant module publish allowed")
	}
	// Draft-only files never grant learner access; old published files survive later revisions and archive.
	files := file.NewService(file.NewStore(p), time.Now)
	makeFile := func() uuid.UUID {
		id := uuid.New()
		if _, err := p.Exec(ctx, `INSERT INTO files(id,org_id,uploaded_by,kind,name,content_type,size_bytes,sha256,object_key,status,created_at,updated_at) VALUES($1,$2,$3,'pdf','Reference','application/pdf',10,$4,$5,'ready',now(),now())`, id, school, admin, make([]byte, 32), id.String()); err != nil {
			t.Fatal(err)
		}
		return id
	}
	if _, err := p.Exec(ctx, `INSERT INTO file_storage(org_id) VALUES($1)`, school); err != nil {
		t.Fatal(err)
	}
	firstFile, secondFile := makeFile(), makeFile()
	fm, err := ms.Create(ctx, admin, school, module.Input{BuildingKey: "sains", Level: "SMA", Title: "Files"})
	if err != nil {
		t.Fatal(err)
	}
	fs, err := ms.AddStep(ctx, admin, school, fm.ID, module.StepInput{Kind: "berkas", Title: "Reference", FileID: &firstFile, Minutes: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ms.Publish(ctx, admin, school, fm.ID, 2); err != nil {
		t.Fatal(err)
	}
	// Real activity pins the revision; browsing alone would follow later publications.
	if err := ms.SetDone(ctx, old, school, fm.ID, fs.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := ms.UpdateStep(ctx, admin, school, fm.ID, fs.ID, module.StepInput{ExpectedVersion: 2, Kind: "berkas", Title: "Replacement", FileID: &secondFile, Minutes: 1}); err != nil {
		t.Fatal(err)
	}
	if err := files.SavePosition(ctx, old, school, secondFile, 1); !errors.Is(err, file.ErrNotFound) {
		t.Fatalf("draft file leak: %v", err)
	}
	if _, err := ms.Publish(ctx, admin, school, fm.ID, 3); err != nil {
		t.Fatal(err)
	}
	if err := files.SavePosition(ctx, old, school, firstFile, 1); err != nil {
		t.Fatalf("historical file denied: %v", err)
	}
	if err := files.SavePosition(ctx, old, school, secondFile, 1); !errors.Is(err, file.ErrNotFound) {
		t.Fatalf("pin file isolation: %v", err)
	}
	if err := ms.Archive(ctx, admin, school, fm.ID); err != nil {
		t.Fatal(err)
	}
	if err := file.NewUploader(files, nil).Delete(ctx, admin, school, firstFile); !errors.Is(err, file.ErrInUse) {
		t.Fatalf("historical file retention: %v", err)
	}

	if err := qs.Archive(ctx, admin, school, z.ID); err != nil {
		t.Fatal(err)
	}
	// Module revision survives quiz archive; unrelated standalone discovery remains hidden.
	if _, err := qs.StartModule(ctx, newStudent, school, z.ID, m.ID, quizStep.ID); err != nil {
		t.Fatalf("historical quiz archive: %v", err)
	}
	if _, err := qs.Start(ctx, newStudent, school, z.ID); !errors.Is(err, quiz.ErrNotFound) {
		t.Fatalf("archived standalone: %v", err)
	}
	if _, err := ms.Publish(ctx, admin, school, m.ID, 5); err == nil {
		t.Fatal("new module publish accepted archived quiz")
	}

}
