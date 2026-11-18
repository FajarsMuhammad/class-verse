package quiz

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/auth"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/org"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/httpx"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/ratelimit"
)

// Handler serves quiz endpoints; mount inside RequireUser.
type Handler struct {
	svc    *Service
	log    *slog.Logger
	reads  *ratelimit.Limiter
	writes *ratelimit.Limiter
}

// NewHandler wires the HTTP layer.
func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log, reads: ratelimit.New(600, time.Minute, svc.now), writes: ratelimit.New(240, time.Minute, svc.now)}
}

// Routes mounts quiz endpoints.
func (h *Handler) Routes(r chi.Router) {
	r.Get("/orgs/{org_id}/quizzes", h.handle(h.list))
	r.Post("/orgs/{org_id}/quizzes", h.handle(h.create))
	r.Post("/orgs/{org_id}/quizzes/{quiz_id}/publish", h.handle(h.publish))
	r.Get("/orgs/{org_id}/quizzes/{quiz_id}/revisions", h.handle(h.history))
	r.Put("/orgs/{org_id}/quizzes/{quiz_id}", h.handle(h.update))
	r.Delete("/orgs/{org_id}/quizzes/{quiz_id}", h.handle(h.archive))
	r.Post("/orgs/{org_id}/quizzes/{quiz_id}/questions", h.handle(h.addQuestion))
	r.Put("/orgs/{org_id}/quizzes/{quiz_id}/questions/{question_id}", h.handle(h.updateQuestion))
	r.Delete("/orgs/{org_id}/quizzes/{quiz_id}/questions/{question_id}", h.handle(h.deleteQuestion))
	r.Put("/orgs/{org_id}/quizzes/{quiz_id}/order", h.handle(h.reorder))
	r.Post("/orgs/{org_id}/quizzes/{quiz_id}/attempts", h.handle(h.start))
	r.Put("/orgs/{org_id}/quizzes/{quiz_id}/attempts/{attempt_id}/answers/{index}", h.handle(h.answer))
	r.Post("/orgs/{org_id}/quizzes/{quiz_id}/attempts/{attempt_id}/finish", h.handle(h.finish))
}

func (h *Handler) handle(fn func(http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return httpx.Handle(h.log, func(w http.ResponseWriter, r *http.Request) error {
		w.Header().Set("Cache-Control", "no-store")
		user, ok := auth.UserID(r.Context())
		if !ok {
			return &httpx.Error{Status: 401, Code: "unauthenticated", Message: "Silakan masuk"}
		}
		limit := h.writes
		if r.Method == http.MethodGet {
			limit = h.reads
		}
		if err := httpx.RateLimit(w, limit, user.String()); err != nil {
			return err
		}
		err := fn(w, r)
		var verr *ValidationError
		switch {
		case err == nil:
			return nil
		case errors.As(err, &verr):
			return httpx.ValidationFailed(verr.Fields)
		case errors.Is(err, ErrConflict):
			return &httpx.Error{Status: 409, Code: "draft_conflict", Message: "Draf berubah di tab lain. Muat versi server atau salin perubahan lokal."}
		case errors.Is(err, ErrNotFound):
			return &httpx.Error{Status: http.StatusNotFound, Code: "not_found", Message: "Kuis atau soal tidak ditemukan"}
		case errors.Is(err, ErrQuestionLimit):
			return httpx.ValidationFailed(map[string]string{"questions": fmt.Sprintf("Maksimal %d soal per kuis", maxQuestions)})
		case errors.Is(err, ErrNoQuestions):
			return httpx.ValidationFailed(map[string]string{"questions": "Kuis terbit perlu minimal 1 soal"})
		case errors.Is(err, ErrBadOrdering):
			return httpx.ValidationFailed(map[string]string{"question_ids": "Urutan harus memuat semua soal tepat sekali"})
		case errors.Is(err, ErrFinished):
			return &httpx.Error{Status: http.StatusConflict, Code: "attempt_finished", Message: "Kuis ini sudah selesai dikerjakan"}
		}
		var apiErr *httpx.Error
		if mapped := org.ToAPIError(err); errors.As(mapped, &apiErr) {
			return mapped
		}
		return fmt.Errorf("quiz: %w", err)
	})
}

// QuestionResponse is a question with its answer; only staff authoring views return it.
type QuestionResponse struct {
	ID          uuid.UUID `json:"id"`
	Prompt      string    `json:"prompt"`
	Options     []string  `json:"options"`
	Answer      int16     `json:"answer"`
	Explanation string    `json:"explanation"`
}

// Response is the public view of a quiz. Questions is present only for staff with drafts=true.
type Response struct {
	DraftVersion        int64              `json:"draft_version"`
	PublishedRevisionID *uuid.UUID         `json:"published_revision_id"`
	ID                  uuid.UUID          `json:"id"`
	BuildingKey         string             `json:"building_key"`
	Level               string             `json:"level"`
	Title               string             `json:"title"`
	Description         string             `json:"description"`
	PassScore           int16              `json:"pass_score"`
	Published           bool               `json:"published"`
	UpdatedAt           time.Time          `json:"updated_at"`
	QuestionCount       int32              `json:"question_count"`
	BestScore           *int16             `json:"best_score"`
	Attempts            int32              `json:"attempts"`
	Questions           []QuestionResponse `json:"questions,omitempty"`
}

func toQuestion(q query.QuizQuestion) QuestionResponse {
	return QuestionResponse{ID: q.ID, Prompt: q.Prompt, Options: q.Options, Answer: q.Answer, Explanation: q.Explanation}
}

func toQuiz(z Quiz) Response {
	out := Response{
		DraftVersion: z.DraftVersion, PublishedRevisionID: func() *uuid.UUID {
			if z.PublishedRevisionID.Valid {
				return &z.PublishedRevisionID.UUID
			}
			return nil
		}(), ID: z.ID, BuildingKey: z.BuildingKey, Level: z.Level, Title: z.Title, Description: z.Description,
		PassScore: z.PassScore, Published: z.PublishedAt.Valid, UpdatedAt: z.UpdatedAt.Time,
		QuestionCount: z.QuestionCount, Attempts: z.Attempts,
	}
	if z.BestScore >= 0 {
		out.BestScore = &z.BestScore
	}
	for _, q := range z.Questions {
		out.Questions = append(out.Questions, toQuestion(q))
	}
	return out
}

// AttemptQuestion is a snapshot question. Answer and Explanation are null until the attempt is finished.
type AttemptQuestion struct {
	Prompt      string   `json:"prompt"`
	Options     []string `json:"options"`
	Answer      *int     `json:"answer"`
	Explanation *string  `json:"explanation"`
}

// AttemptResponse is one try with the caller's choices (-1: not answered yet).
type AttemptResponse struct {
	ID        uuid.UUID         `json:"id"`
	QuizID    uuid.UUID         `json:"quiz_id"`
	Questions []AttemptQuestion `json:"questions"`
	Answers   []int16           `json:"answers"`
	PassScore int16             `json:"pass_score"`
	Finished  bool              `json:"finished"`
	Correct   *int16            `json:"correct"`
	Score     *int16            `json:"score"`
	Passed    *bool             `json:"passed"`
}

func toAttemptResponse(a Attempt) AttemptResponse {
	out := AttemptResponse{ID: a.ID, QuizID: a.QuizID, Answers: a.Answers, PassScore: a.PassScore, Finished: a.Finished,
		Questions: make([]AttemptQuestion, len(a.Questions))}
	for i, q := range a.Questions {
		out.Questions[i] = AttemptQuestion{Prompt: q.Prompt, Options: q.Options}
		if a.Finished {
			out.Questions[i].Answer, out.Questions[i].Explanation = &q.Answer, &q.Explanation
		}
	}
	if a.Finished {
		passed := a.Passed()
		out.Correct, out.Score, out.Passed = &a.Correct, &a.Score, &passed
	}
	return out
}

type quizRequest struct {
	ExpectedVersion int64  `json:"expected_version"`
	BuildingKey     string `json:"building_key"`
	Level           string `json:"level"`
	Title           string `json:"title"`
	Description     string `json:"description"`
	PassScore       int    `json:"pass_score"`
	Published       bool   `json:"published"`
}

type questionRequest struct {
	ExpectedVersion int64    `json:"expected_version"`
	Prompt          string   `json:"prompt"`
	Options         []string `json:"options"`
	Answer          int      `json:"answer"`
	Explanation     string   `json:"explanation"`
}

// ids reads path UUIDs in order; malformed IDs are treated as not found (org.PathUUID).
func ids(r *http.Request, names ...string) (uuid.UUID, []uuid.UUID, error) {
	user, _ := auth.UserID(r.Context())
	out := make([]uuid.UUID, 0, len(names))
	for _, n := range names {
		id, err := org.PathUUID(r, n)
		if err != nil {
			return user, nil, err
		}
		out = append(out, id)
	}
	return user, out, nil
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) error {
	user, p, err := ids(r, "org_id")
	if err != nil {
		return err
	}
	q := r.URL.Query()
	quizzes, err := h.svc.List(r.Context(), user, p[0], Filter{BuildingKey: q.Get("building_key"), Level: q.Get("level"), Drafts: q.Get("drafts") == "true"})
	if err != nil {
		return err
	}
	items := make([]Response, 0, len(quizzes))
	for _, z := range quizzes {
		items = append(items, toQuiz(z))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
	return nil
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) error {
	user, p, err := ids(r, "org_id")
	if err != nil {
		return err
	}
	var req quizRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	z, err := h.svc.Create(r.Context(), user, p[0], Input(req))
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusCreated, toQuiz(Quiz{Quiz: z, BestScore: -1}))
	return nil
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) error {
	user, p, err := ids(r, "org_id", "quiz_id")
	if err != nil {
		return err
	}
	var req quizRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	if req.ExpectedVersion < 1 {
		return httpx.ValidationFailed(map[string]string{"expected_version": "Versi draf wajib"})
	}
	z, err := h.svc.Update(r.Context(), user, p[0], p[1], Input(req))
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, toQuiz(Quiz{Quiz: z, BestScore: -1}))
	return nil
}

func (h *Handler) archive(w http.ResponseWriter, r *http.Request) error {
	user, p, err := ids(r, "org_id", "quiz_id")
	if err != nil {
		return err
	}
	if err := h.svc.Archive(r.Context(), user, p[0], p[1]); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *Handler) addQuestion(w http.ResponseWriter, r *http.Request) error {
	user, p, err := ids(r, "org_id", "quiz_id")
	if err != nil {
		return err
	}
	var req questionRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	if req.ExpectedVersion < 1 {
		return httpx.ValidationFailed(map[string]string{"expected_version": "Versi draf wajib"})
	}
	q, err := h.svc.AddQuestion(r.Context(), user, p[0], p[1], QuestionInput(req))
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusCreated, toQuestion(q))
	return nil
}

func (h *Handler) updateQuestion(w http.ResponseWriter, r *http.Request) error {
	user, p, err := ids(r, "org_id", "quiz_id", "question_id")
	if err != nil {
		return err
	}
	var req questionRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	if req.ExpectedVersion < 1 {
		return httpx.ValidationFailed(map[string]string{"expected_version": "Versi draf wajib"})
	}
	q, err := h.svc.UpdateQuestion(r.Context(), user, p[0], p[1], p[2], QuestionInput(req))
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, toQuestion(q))
	return nil
}

func (h *Handler) deleteQuestion(w http.ResponseWriter, r *http.Request) error {
	user, p, err := ids(r, "org_id", "quiz_id", "question_id")
	if err != nil {
		return err
	}
	version, err := strconv.ParseInt(r.URL.Query().Get("expected_version"), 10, 64)
	if err != nil || version < 1 {
		return httpx.ValidationFailed(map[string]string{"expected_version": "Versi draf wajib"})
	}
	if err := h.svc.DeleteQuestion(r.Context(), user, p[0], p[1], p[2], version); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *Handler) reorder(w http.ResponseWriter, r *http.Request) error {
	user, p, err := ids(r, "org_id", "quiz_id")
	if err != nil {
		return err
	}
	var req struct {
		QuestionIDs     []uuid.UUID `json:"question_ids"`
		ExpectedVersion int64       `json:"expected_version"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	if req.ExpectedVersion < 1 {
		return httpx.ValidationFailed(map[string]string{"expected_version": "Versi draf wajib"})
	}
	if err := h.svc.Reorder(r.Context(), user, p[0], p[1], req.QuestionIDs, req.ExpectedVersion); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *Handler) start(w http.ResponseWriter, r *http.Request) error {
	user, p, err := ids(r, "org_id", "quiz_id")
	if err != nil {
		return err
	}
	var a Attempt
	if raw := r.URL.Query().Get("assignment_step_id"); raw != "" {
		stepID, parseErr := uuid.Parse(raw)
		if parseErr != nil {
			return ErrNotFound
		}
		a, err = h.svc.StartAssignment(r.Context(), user, p[0], p[1], stepID)
	} else if raw := r.URL.Query().Get("module_id"); raw != "" {
		moduleID, e := uuid.Parse(raw)
		if e != nil {
			return ErrNotFound
		}
		stepID, e := uuid.Parse(r.URL.Query().Get("step_id"))
		if e != nil {
			return ErrNotFound
		}
		a, err = h.svc.StartModule(r.Context(), user, p[0], p[1], moduleID, stepID)
	} else {
		a, err = h.svc.Start(r.Context(), user, p[0], p[1])
	}
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, toAttemptResponse(a))
	return nil
}

func (h *Handler) answer(w http.ResponseWriter, r *http.Request) error {
	user, p, err := ids(r, "org_id", "quiz_id", "attempt_id")
	if err != nil {
		return err
	}
	index, err := strconv.Atoi(chi.URLParam(r, "index"))
	if err != nil {
		return &ValidationError{Fields: map[string]string{"index": "Soal tidak ada"}}
	}
	var req struct {
		Choice *int `json:"choice"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	if req.Choice == nil {
		return &ValidationError{Fields: map[string]string{"choice": "Pilih salah satu dari 4 jawaban"}}
	}
	if err := h.svc.Answer(r.Context(), user, p[0], p[1], p[2], index, *req.Choice); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *Handler) finish(w http.ResponseWriter, r *http.Request) error {
	user, p, err := ids(r, "org_id", "quiz_id", "attempt_id")
	if err != nil {
		return err
	}
	a, err := h.svc.Finish(r.Context(), user, p[0], p[1], p[2])
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, toAttemptResponse(a))
	return nil
}

func (h *Handler) publish(w http.ResponseWriter, r *http.Request) error {
	user, p, err := ids(r, "org_id", "quiz_id")
	if err != nil {
		return err
	}
	var req struct {
		ExpectedVersion int64 `json:"expected_version"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	m, err := h.svc.Publish(r.Context(), user, p[0], p[1], req.ExpectedVersion)
	if err != nil {
		return err
	}
	httpx.JSON(w, 200, toQuiz(Quiz{Quiz: m}))
	return nil
}
func (h *Handler) history(w http.ResponseWriter, r *http.Request) error {
	user, p, err := ids(r, "org_id", "quiz_id")
	if err != nil {
		return err
	}
	rows, err := h.svc.History(r.Context(), user, p[0], p[1])
	if err != nil {
		return err
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{"id": row.ID, "revision": row.Revision, "draft_version": row.DraftVersion, "published_at": row.PublishedAt.Time})
	}
	httpx.JSON(w, 200, map[string]any{"items": items})
	return nil
}
