package quiz_test

import (
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/module"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/org"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/dbtest"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/quiz"
)

type buildings map[string]bool

func (b buildings) Building(key string) (bool, bool) {
	staff, ok := b[key]
	return staff, ok
}

type fixture struct {
	pool                        *pgxpool.Pool
	orgs                        *org.Service
	svc                         *quiz.Service
	school, other               uuid.UUID
	admin, student, peer, stray uuid.UUID
}

func setup(t *testing.T) fixture {
	t.Helper()
	pool := dbtest.New(t)
	ctx := t.Context()
	clock := func() time.Time { return time.Now().UTC() }
	q := query.New(pool)
	user := func(email string) uuid.UUID {
		u, err := q.CreateTestUser(ctx, query.CreateTestUserParams{Email: email, DisplayName: email, BirthDate: pgtype.Date{Time: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true}})
		if err != nil {
			t.Fatal(err)
		}
		return u.ID
	}
	f := fixture{admin: user("admin@quiz.test"), student: user("student@quiz.test"), peer: user("peer@quiz.test"), stray: user("stray@quiz.test")}
	orgs := org.NewService(org.NewStore(pool), clock, slog.New(slog.NewTextHandler(io.Discard, nil)))
	school, err := orgs.Create(ctx, f.admin, org.CreateInput{Name: "Quiz School", Slug: "quiz-school"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := orgs.Create(ctx, f.stray, org.CreateInput{Name: "Other", Slug: "quiz-other"})
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range []uuid.UUID{f.student, f.peer} {
		if _, err := pool.Exec(ctx, "INSERT INTO memberships (org_id, user_id, role, joined_at) VALUES ($1, $2, 'participant', now())", school.ID, u); err != nil {
			t.Fatal(err)
		}
	}
	f.pool, f.orgs = pool, orgs
	f.svc, f.school, f.other = quiz.NewService(pool, orgs, buildings{"sains": false, "ruang-guru": true}, clock), school.ID, other.ID
	return f
}

func question(prompt string, answer int) quiz.QuestionInput {
	return quiz.QuestionInput{Prompt: prompt, Options: []string{"A", "B", "C", "D"}, Answer: answer, Explanation: "Karena " + prompt}
}

// published creates a published quiz with the given answers (one question each).
func (f fixture) published(t *testing.T, answers ...int) uuid.UUID {
	t.Helper()
	ctx := t.Context()
	in := quiz.Input{BuildingKey: "sains", Level: "SD", Title: "Kuis Planet"}
	z, err := f.svc.Create(ctx, f.admin, f.school, in)
	if err != nil {
		t.Fatal(err)
	}
	for i, a := range answers {
		if _, err := f.svc.AddQuestion(ctx, f.admin, f.school, z.ID, question("Soal "+string(rune('1'+i)), a)); err != nil {
			t.Fatal(err)
		}
	}
	in.Published = true
	if _, err := f.svc.Update(ctx, f.admin, f.school, z.ID, in); err != nil {
		t.Fatal(err)
	}
	return z.ID
}

func TestAuthoringRolesAndVisibility(t *testing.T) {
	f := setup(t)
	ctx := t.Context()
	in := quiz.Input{BuildingKey: "sains", Level: "SD", Title: "Kuis Planet"}
	if _, err := f.svc.Create(ctx, f.student, f.school, in); !errors.Is(err, org.ErrForbidden) {
		t.Fatalf("participant create: %v", err)
	}
	if _, err := f.svc.Create(ctx, f.stray, f.school, in); !errors.Is(err, org.ErrNotMember) {
		t.Fatalf("outsider create: %v", err)
	}
	if _, err := f.svc.Create(ctx, f.admin, f.school, quiz.Input{BuildingKey: "sains", Level: "SD", Title: "X", Published: true}); !errors.Is(err, quiz.ErrNoQuestions) {
		t.Fatalf("publish empty on create: %v", err)
	}
	z, err := f.svc.Create(ctx, f.admin, f.school, in)
	if err != nil || z.PassScore != 60 || z.PublishedAt.Valid {
		t.Fatalf("create=%+v err=%v", z, err)
	}
	in.Published = true
	if _, err := f.svc.Update(ctx, f.admin, f.school, z.ID, in); !errors.Is(err, quiz.ErrNoQuestions) {
		t.Fatalf("publish without questions: %v", err)
	}
	q1, err := f.svc.AddQuestion(ctx, f.admin, f.school, z.ID, question("Satu", 2))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.AddQuestion(ctx, f.student, f.school, z.ID, question("X", 0)); !errors.Is(err, org.ErrForbidden) {
		t.Fatalf("participant add question: %v", err)
	}
	// Draft: invisible to participants, and they cannot start it.
	if list, _ := f.svc.List(ctx, f.student, f.school, quiz.Filter{Drafts: true}); len(list) != 0 {
		t.Fatalf("draft leaked: %+v", list)
	}
	if _, err := f.svc.Start(ctx, f.student, f.school, z.ID); !errors.Is(err, quiz.ErrNotFound) {
		t.Fatalf("participant start draft: %v", err)
	}
	if _, err := f.svc.Update(ctx, f.admin, f.school, z.ID, in); err != nil {
		t.Fatal(err)
	}
	// The last question of a published quiz cannot be removed.
	if err := f.svc.DeleteQuestion(ctx, f.admin, f.school, z.ID, q1.ID); !errors.Is(err, quiz.ErrNoQuestions) {
		t.Fatalf("delete last published question: %v", err)
	}
	staff, err := f.svc.List(ctx, f.admin, f.school, quiz.Filter{Drafts: true})
	if err != nil || len(staff) != 1 || len(staff[0].Questions) != 1 || staff[0].Questions[0].Answer != 2 {
		t.Fatalf("staff list=%+v err=%v", staff, err)
	}
	part, err := f.svc.List(ctx, f.student, f.school, quiz.Filter{BuildingKey: "sains", Level: "SD", Drafts: true})
	if err != nil || len(part) != 1 || part[0].Questions != nil || part[0].QuestionCount != 1 || part[0].BestScore != -1 {
		t.Fatalf("participant list must not include questions/answers: %+v err=%v", part, err)
	}
	if list, err := f.svc.List(ctx, f.stray, f.school, quiz.Filter{}); !errors.Is(err, org.ErrNotMember) || list != nil {
		t.Fatalf("outsider list: %v", err)
	}
	if err := f.svc.Archive(ctx, f.admin, f.school, z.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Start(ctx, f.student, f.school, z.ID); !errors.Is(err, quiz.ErrNotFound) {
		t.Fatalf("start archived: %v", err)
	}
}

func TestAttemptHidesKeyGradesAndSnapshots(t *testing.T) {
	f := setup(t)
	ctx := t.Context()
	id := f.published(t, 1, 3, 0)
	a, err := f.svc.Start(ctx, f.student, f.school, id)
	if err != nil || len(a.Questions) != 3 || a.Finished {
		t.Fatalf("start=%+v err=%v", a, err)
	}
	for _, q := range a.Questions {
		if q.Answer != -1 || q.Explanation != "" {
			t.Fatalf("key leaked before finish: %+v", q)
		}
	}
	// Starting again resumes the same open attempt.
	again, err := f.svc.Start(ctx, f.student, f.school, id)
	if err != nil || again.ID != a.ID {
		t.Fatalf("resume=%v err=%v", again.ID, err)
	}
	// The teacher edits the quiz after the attempt started: the attempt keeps its snapshot.
	staff, _ := f.svc.List(ctx, f.admin, f.school, quiz.Filter{Drafts: true})
	first := staff[0].Questions[0]
	if _, err := f.svc.UpdateQuestion(ctx, f.admin, f.school, id, first.ID, question("Diganti", 2)); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ i, choice int }{{0, 0}, {0, 1}, {0, 1}, {1, 3}} { // change + repeat are fine
		if err := f.svc.Answer(ctx, f.student, f.school, id, a.ID, c.i, c.choice); err != nil {
			t.Fatal(err)
		}
	}
	for _, bad := range []struct{ i, choice int }{{3, 0}, {-1, 0}, {0, 4}, {0, -1}} {
		var v *quiz.ValidationError
		if err := f.svc.Answer(ctx, f.student, f.school, id, a.ID, bad.i, bad.choice); !errors.As(err, &v) {
			t.Fatalf("bad answer %+v: %v", bad, err)
		}
	}
	if err := f.svc.Answer(ctx, f.peer, f.school, id, a.ID, 2, 0); !errors.Is(err, quiz.ErrNotFound) {
		t.Fatalf("peer answering another attempt: %v", err)
	}
	if _, err := f.svc.Finish(ctx, f.peer, f.school, id, a.ID); !errors.Is(err, quiz.ErrNotFound) {
		t.Fatalf("peer finishing another attempt: %v", err)
	}
	done, err := f.svc.Finish(ctx, f.student, f.school, id, a.ID)
	// Snapshot answer for question 1 is still 1 (not the edited 2); question 3 unanswered counts wrong.
	if err != nil || !done.Finished || done.Correct != 2 || done.Score != 66 || !done.Passed() {
		t.Fatalf("finish=%+v err=%v", done, err)
	}
	if done.Questions[0].Answer != 1 || done.Questions[0].Prompt != "Soal 1" || done.Questions[0].Explanation == "" {
		t.Fatalf("snapshot/key after finish: %+v", done.Questions[0])
	}
	if again, err := f.svc.Finish(ctx, f.student, f.school, id, a.ID); err != nil || again.Score != 66 {
		t.Fatalf("finish twice=%+v err=%v", again, err)
	}
	if err := f.svc.Answer(ctx, f.student, f.school, id, a.ID, 2, 0); !errors.Is(err, quiz.ErrFinished) {
		t.Fatalf("answer after finish: %v", err)
	}
	// Unlimited retries; the list reports the best score and the finished count.
	retry, err := f.svc.Start(ctx, f.student, f.school, id)
	if err != nil || retry.ID == a.ID || retry.Questions[0].Prompt != "Soal 1" {
		t.Fatalf("retry=%+v err=%v", retry, err)
	}
	if low, err := f.svc.Finish(ctx, f.student, f.school, id, retry.ID); err != nil || low.Score != 0 || low.Passed() {
		t.Fatalf("empty retry=%+v err=%v", low, err)
	}
	list, err := f.svc.List(ctx, f.student, f.school, quiz.Filter{})
	if err != nil || list[0].BestScore != 66 || list[0].Attempts != 2 {
		t.Fatalf("best=%+v err=%v", list, err)
	}
	if peer, _ := f.svc.List(ctx, f.peer, f.school, quiz.Filter{}); peer[0].BestScore != -1 || peer[0].Attempts != 0 {
		t.Fatalf("peer sees other's history: %+v", peer)
	}
	// Another school's member cannot reach this quiz.
	if _, err := f.svc.Start(ctx, f.stray, f.other, id); !errors.Is(err, quiz.ErrNotFound) {
		t.Fatalf("cross-org start: %v", err)
	}
	// Staff results use the Rapor's highest score; participants and other schools are refused.
	results, err := f.svc.Results(ctx, f.admin, f.school, id, 20, 0)
	if err != nil || len(results) != 1 || results[0].UserID != f.student || results[0].BestScore != 66 || results[0].Attempts != 2 || results[0].TotalCount != 1 {
		t.Fatalf("results=%+v err=%v", results, err)
	}
	if _, err := f.svc.Results(ctx, f.student, f.school, id, 20, 0); !errors.Is(err, org.ErrForbidden) {
		t.Fatalf("participant results: %v", err)
	}
	if _, err := f.svc.Results(ctx, f.stray, f.other, id, 20, 0); !errors.Is(err, quiz.ErrNotFound) {
		t.Fatalf("cross-org results: %v", err)
	}
	counts, err := f.svc.Counts(ctx, f.admin, f.school, quiz.Filter{})
	if err != nil || counts.AllCount != 1 || counts.PublishedCount != 1 || counts.EmptyCount != 0 {
		t.Fatalf("counts=%+v err=%v", counts, err)
	}
}

// Passing completes the quiz step of a published module; failing or a manual mark does not.
func TestQuizStepCompletesOnPass(t *testing.T) {
	f := setup(t)
	ctx := t.Context()
	id := f.published(t, 0, 0)
	mods := module.NewService(f.pool, f.orgs, buildings{"sains": false}, time.Now)
	m, err := mods.Create(ctx, f.admin, f.school, module.Input{BuildingKey: "sains", Level: "SD", Title: "Tata Surya"})
	if err != nil {
		t.Fatal(err)
	}
	var v *module.ValidationError
	draft, _ := f.svc.Create(ctx, f.admin, f.school, quiz.Input{BuildingKey: "sains", Level: "SD", Title: "Draf"})
	if _, err := mods.AddStep(ctx, f.admin, f.school, m.ID, module.StepInput{Kind: "kuis", Title: "Kuis", Minutes: 5, QuizID: &draft.ID}); !errors.As(err, &v) || v.Fields["quiz_id"] == "" {
		t.Fatalf("draft quiz linked: %v", err)
	}
	step, err := mods.AddStep(ctx, f.admin, f.school, m.ID, module.StepInput{Kind: "kuis", Title: "Kuis kecil", Minutes: 5, QuizID: &id})
	if err != nil || !step.QuizID.Valid {
		t.Fatalf("quiz step=%+v err=%v", step, err)
	}
	if _, err := mods.Publish(ctx, f.admin, f.school, m.ID, 2); err != nil {
		t.Fatal(err)
	}
	if err := mods.SetDone(ctx, f.student, f.school, m.ID, step.ID, true); !errors.Is(err, module.ErrQuizStep) {
		t.Fatalf("manual completion of quiz step: %v", err)
	}
	done := func(user uuid.UUID) bool {
		list, err := mods.List(ctx, user, f.school, module.Filter{})
		if err != nil {
			t.Fatal(err)
		}
		return list[0].Steps[0].Done
	}
	fail, _ := f.svc.Start(ctx, f.student, f.school, id)
	_ = f.svc.Answer(ctx, f.student, f.school, id, fail.ID, 0, 0) // 50 < 60
	if _, err := f.svc.Finish(ctx, f.student, f.school, id, fail.ID); err != nil || done(f.student) {
		t.Fatalf("failed attempt completed the step: %v", err)
	}
	pass, _ := f.svc.Start(ctx, f.student, f.school, id)
	_ = f.svc.Answer(ctx, f.student, f.school, id, pass.ID, 0, 0)
	_ = f.svc.Answer(ctx, f.student, f.school, id, pass.ID, 1, 0)
	if r, err := f.svc.Finish(ctx, f.student, f.school, id, pass.ID); err != nil || !r.Passed() || !done(f.student) {
		t.Fatalf("pass did not complete the step: %+v %v", r, err)
	}
	if done(f.peer) {
		t.Fatal("another member's step completed")
	}
}

// Concurrent starts and finishes must not create two open attempts or grade twice.
func TestAttemptConcurrency(t *testing.T) {
	f := setup(t)
	ctx := t.Context()
	id := f.published(t, 0, 0)
	const n = 8
	ids := make([]uuid.UUID, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			a, err := f.svc.Start(ctx, f.student, f.school, id)
			if err != nil {
				t.Error(err)
				return
			}
			ids[i] = a.ID
		})
	}
	wg.Wait()
	for _, x := range ids {
		if x != ids[0] {
			t.Fatalf("parallel starts created several attempts: %v", ids)
		}
	}
	_ = f.svc.Answer(ctx, f.student, f.school, id, ids[0], 0, 0)
	scores := make([]int16, n)
	for i := range n {
		wg.Go(func() {
			a, err := f.svc.Finish(ctx, f.student, f.school, id, ids[0])
			if err != nil {
				t.Error(err)
				return
			}
			scores[i] = a.Score
		})
	}
	wg.Wait()
	for _, s := range scores {
		if s != 50 {
			t.Fatalf("finish not idempotent: %v", scores)
		}
	}
}
