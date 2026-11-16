package quiz_test

import (
	"crypto/ed25519"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/auth"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/quiz"
)

// TestHTTP checks the wire contract: no answer key before finish, role errors, and status codes.
func TestHTTP(t *testing.T) {
	f := setup(t)
	id := f.published(t, 2)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	tokens := auth.NewTokens(ed25519.NewKeyFromSeed(make([]byte, 32)), time.Now)
	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireUser(tokens, log))
		r.Route("/api", quiz.NewHandler(f.svc, log).Routes)
	})
	call := func(user uuid.UUID, method, path, body string) (int, string) {
		tok, err := tokens.Issue(user)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+tok)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code, w.Body.String()
	}
	base := "/api/orgs/" + f.school.String() + "/quizzes"
	quizPath := base + "/" + id.String()

	if code, _ := call(f.student, http.MethodPost, base, `{"building_key":"sains","level":"SD","title":"X"}`); code != http.StatusForbidden {
		t.Fatalf("participant create=%d", code)
	}
	if code, body := call(f.student, http.MethodGet, base+"?drafts=true", ""); code != http.StatusOK || strings.Contains(body, `"answer"`) || strings.Contains(body, "Karena") {
		t.Fatalf("participant list leaked questions: %d %s", code, body)
	}
	code, body := call(f.student, http.MethodPost, quizPath+"/attempts", "")
	var a struct {
		ID        uuid.UUID `json:"id"`
		Questions []struct {
			Answer      *int    `json:"answer"`
			Explanation *string `json:"explanation"`
		} `json:"questions"`
		Score  *int  `json:"score"`
		Passed *bool `json:"passed"`
	}
	if code != http.StatusOK || json.Unmarshal([]byte(body), &a) != nil || a.Questions[0].Answer != nil || a.Questions[0].Explanation != nil || strings.Contains(body, "Karena") {
		t.Fatalf("start leaked key: %d %s", code, body)
	}
	attempt := quizPath + "/attempts/" + a.ID.String()
	for _, bad := range []struct{ path, body string }{{"/answers/x", `{"choice":0}`}, {"/answers/0", `{}`}, {"/answers/0", `{"choice":7}`}, {"/answers/9", `{"choice":0}`}} {
		if code, body := call(f.student, http.MethodPut, attempt+bad.path, bad.body); code != http.StatusBadRequest {
			t.Fatalf("bad answer %s %s = %d %s", bad.path, bad.body, code, body)
		}
	}
	if code, _ := call(f.peer, http.MethodPut, attempt+"/answers/0", `{"choice":2}`); code != http.StatusNotFound {
		t.Fatalf("peer answer=%d", code)
	}
	if code, _ := call(f.student, http.MethodPut, attempt+"/answers/0", `{"choice":2}`); code != http.StatusNoContent {
		t.Fatalf("answer=%d", code)
	}
	code, body = call(f.student, http.MethodPost, attempt+"/finish", "")
	if code != http.StatusOK || json.Unmarshal([]byte(body), &a) != nil || *a.Score != 100 || !*a.Passed || *a.Questions[0].Answer != 2 || *a.Questions[0].Explanation == "" {
		t.Fatalf("finish=%d %s", code, body)
	}
	if code, body := call(f.student, http.MethodPut, attempt+"/answers/0", `{"choice":1}`); code != http.StatusConflict || !strings.Contains(body, "attempt_finished") {
		t.Fatalf("answer after finish=%d %s", code, body)
	}
	if code, body := call(f.student, http.MethodGet, base, ""); code != http.StatusOK || !strings.Contains(body, `"best_score":100`) {
		t.Fatalf("best score=%d %s", code, body)
	}
	if code, body := call(f.admin, http.MethodGet, base+"?drafts=true", ""); code != http.StatusOK || !strings.Contains(body, `"answer":2`) {
		t.Fatalf("staff authoring view=%d %s", code, body)
	}
	if code, _ := call(f.stray, http.MethodGet, base, ""); code != http.StatusNotFound {
		t.Fatalf("outsider list=%d", code)
	}
	if code, _ := call(f.student, http.MethodPost, base+"/not-a-uuid/attempts", ""); code != http.StatusNotFound {
		t.Fatalf("malformed id=%d", code)
	}

	// Full authoring flow over HTTP.
	decodeID := func(body string) string {
		var v struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal([]byte(body), &v); err != nil || v.ID == "" {
			t.Fatalf("decode id from %s", body)
		}
		return v.ID
	}
	code, body = call(f.admin, http.MethodPost, base, `{"building_key":"sains","level":"SMP","title":"Kuis Bumi","description":"","pass_score":0,"published":false}`)
	if code != http.StatusCreated || !strings.Contains(body, `"pass_score":60`) || !strings.Contains(body, `"best_score":null`) {
		t.Fatalf("create=%d %s", code, body)
	}
	newQuiz := base + "/" + decodeID(body)
	if code, body := call(f.admin, http.MethodPost, base, `{"building_key":"sains","level":"SD","title":"X","pass_score":101,"published":false}`); code != http.StatusBadRequest || !strings.Contains(body, "pass_score") {
		t.Fatalf("bad pass score=%d %s", code, body)
	}
	if code, body := call(f.admin, http.MethodPut, newQuiz, `{"building_key":"sains","level":"SMP","title":"Kuis Bumi","description":"","published":true}`); code != http.StatusBadRequest || !strings.Contains(body, "minimal 1 soal") {
		t.Fatalf("publish empty=%d %s", code, body)
	}
	var qids []string
	for _, prompt := range []string{"Satu", "Dua"} {
		code, body := call(f.admin, http.MethodPost, newQuiz+"/questions", `{"prompt":"`+prompt+`","options":["a","b","c","d"],"answer":3}`)
		if code != http.StatusCreated {
			t.Fatalf("add question=%d %s", code, body)
		}
		qids = append(qids, decodeID(body))
	}
	if code, body := call(f.admin, http.MethodPost, newQuiz+"/questions", `{"prompt":"X","options":["a","b","c"],"answer":0}`); code != http.StatusBadRequest || !strings.Contains(body, "options") {
		t.Fatalf("three options=%d %s", code, body)
	}
	if code, body := call(f.admin, http.MethodPut, newQuiz+"/questions/"+qids[0], `{"prompt":"Satu baru","options":["a","b","c","d"],"answer":1,"explanation":"Karena"}`); code != http.StatusOK || !strings.Contains(body, `"answer":1`) {
		t.Fatalf("update question=%d %s", code, body)
	}
	if code, _ := call(f.admin, http.MethodPut, newQuiz+"/questions/"+uuid.NewString(), `{"prompt":"X","options":["a","b","c","d"],"answer":1}`); code != http.StatusNotFound {
		t.Fatalf("update missing question=%d", code)
	}
	if code, body := call(f.admin, http.MethodPut, newQuiz+"/order", `{"question_ids":["`+qids[1]+`"]}`); code != http.StatusBadRequest || !strings.Contains(body, "question_ids") {
		t.Fatalf("partial order=%d %s", code, body)
	}
	if code, _ := call(f.admin, http.MethodPut, newQuiz+"/order", `{"question_ids":["`+qids[1]+`","`+qids[0]+`"]}`); code != http.StatusNoContent {
		t.Fatalf("reorder=%d", code)
	}
	if code, body := call(f.admin, http.MethodGet, base+"?drafts=true&level=SMP", ""); code != http.StatusOK || strings.Index(body, "Dua") > strings.Index(body, "Satu baru") {
		t.Fatalf("order not applied: %s", body)
	}
	if code, _ := call(f.student, http.MethodPut, newQuiz+"/order", `{"question_ids":[]}`); code != http.StatusForbidden {
		t.Fatalf("participant reorder=%d", code)
	}
	if code, _ := call(f.admin, http.MethodPut, newQuiz, `{"building_key":"sains","level":"SMP","title":"Kuis Bumi","description":"","published":true}`); code != http.StatusOK {
		t.Fatalf("publish=%d", code)
	}
	if code, _ := call(f.admin, http.MethodDelete, newQuiz+"/questions/"+qids[1], ""); code != http.StatusNoContent {
		t.Fatalf("delete question=%d", code)
	}
	if code, _ := call(f.admin, http.MethodDelete, newQuiz+"/questions/"+qids[1], ""); code != http.StatusNotFound {
		t.Fatalf("delete twice=%d", code)
	}
	if code, body := call(f.admin, http.MethodDelete, newQuiz+"/questions/"+qids[0], ""); code != http.StatusBadRequest || !strings.Contains(body, "minimal 1 soal") {
		t.Fatalf("delete last published=%d %s", code, body)
	}
	if code, body := call(f.admin, http.MethodGet, base+"?level=Kelas9", ""); code != http.StatusBadRequest || !strings.Contains(body, "level") {
		t.Fatalf("bad level=%d %s", code, body)
	}
	if code, _ := call(f.admin, http.MethodDelete, newQuiz, ""); code != http.StatusNoContent {
		t.Fatalf("archive=%d", code)
	}
	if code, _ := call(f.admin, http.MethodDelete, newQuiz, ""); code != http.StatusNotFound {
		t.Fatalf("archive twice=%d", code)
	}
}
