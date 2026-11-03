package report

import (
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/auth"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/org"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/dbtest"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

func TestValidateReason(t *testing.T) {
	for _, tt := range []struct {
		name, in, want string
		invalid        bool
	}{
		{"trim and nfc", "  Cafe\u0301 kasar ", "Café kasar", false},
		{"empty", "   ", "", true},
		{"newline ok", "baris 1\nbaris 2", "baris 1\nbaris 2", false},
		{"control", "a\x00b", "", true},
		{"bidi override", "a\u202eb", "", true},
		{"limit", strings.Repeat("界", 1000), strings.Repeat("界", 1000), false},
		{"too long", strings.Repeat("界", 1001), "", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateReason(tt.in)
			if (err != nil) != tt.invalid || (!tt.invalid && got != tt.want) {
				t.Fatalf("got %q err %v", got, err)
			}
		})
	}
}

func TestReportsAndModerationLog(t *testing.T) {
	pool := dbtest.New(t)
	ctx := t.Context()
	now := time.Date(2026, 3, 1, 8, 0, 0, 0, time.UTC)
	ts := pgtype.Timestamptz{Time: now, Valid: true}
	clock := func() time.Time { return now }
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	q := query.New(pool)
	newUser := func(email string) uuid.UUID {
		t.Helper()
		u, err := q.CreateTestUser(ctx, query.CreateTestUserParams{Email: email, DisplayName: strings.Split(email, "@")[0],
			BirthDate: pgtype.Date{Time: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true}})
		if err != nil {
			t.Fatal(err)
		}
		return u.ID
	}
	admin, teacher, ani, budi, outsider := newUser("admin@r.test"), newUser("guru@r.test"), newUser("ani@r.test"),
		newUser("budi@r.test"), newUser("luar@r.test")
	orgSvc := org.NewService(org.NewStore(pool), clock, log)
	school, err := orgSvc.Create(ctx, admin, org.CreateInput{Name: "Report School", Slug: "report-school"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := orgSvc.Create(ctx, outsider, org.CreateInput{Name: "Other School", Slug: "other-school"})
	if err != nil {
		t.Fatal(err)
	}
	for id, role := range map[uuid.UUID]query.OrgRole{teacher: query.OrgRoleTeacher, ani: query.OrgRoleParticipant, budi: query.OrgRoleParticipant} {
		if _, err := q.CreateMembership(ctx, query.CreateMembershipParams{OrgID: school.ID, UserID: id, Role: role, JoinedAt: ts}); err != nil {
			t.Fatal(err)
		}
	}
	room := func(orgID, by uuid.UUID) uuid.UUID {
		r, err := q.CreateRoom(ctx, query.CreateRoomParams{OrgID: orgID, Name: "Kelas", MapKey: "classroom",
			BuildingKey: pgtype.Text{String: "kelas-a", Valid: true}, CreatedBy: by, Now: ts})
		if err != nil {
			t.Fatal(err)
		}
		return r.ID
	}
	classroom, otherRoom := room(school.ID, teacher), room(other.ID, outsider)
	// A stored message from Budi in the classroom, and one in the other school.
	message := func(roomID, sender uuid.UUID, body string) int64 {
		var sess uuid.UUID
		if err := pool.QueryRow(ctx, "INSERT INTO room_sessions (room_id, started_at) VALUES ($1,$2) ON CONFLICT DO NOTHING RETURNING id", roomID, now).Scan(&sess); err != nil {
			_ = pool.QueryRow(ctx, "SELECT id FROM room_sessions WHERE room_id=$1 AND ended_at IS NULL", roomID).Scan(&sess)
		}
		var id int64
		if err := pool.QueryRow(ctx, "INSERT INTO chat_messages (session_id, sender_id, scope, body, created_at) VALUES ($1,$2,'room',$3,$4) RETURNING id",
			sess, sender, body, now).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	budiMsg, aniMsg, foreignMsg := message(classroom, budi, "kata ****"), message(classroom, ani, "halo"), message(otherRoom, outsider, "luar")

	tokens := auth.NewTokens(ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize)), clock)
	router := chi.NewRouter()
	router.Route("/api", func(r chi.Router) {
		r.Use(auth.RequireUser(tokens, log))
		NewHandler(NewService(NewStore(pool), orgSvc, clock), log).Routes(r)
	})
	call := func(user uuid.UUID, method, path, body string) (int, map[string]any) {
		t.Helper()
		var rd io.Reader = http.NoBody
		if body != "" {
			rd = strings.NewReader(body)
		}
		req := httptest.NewRequest(method, path, rd)
		tok, err := tokens.Issue(user)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+tok)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		var out map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return rec.Code, out
	}

	// Message report: target and room come from the stored message, even if
	// the client sends another target.
	status, body := call(ani, http.MethodPost, "/api/reports",
		fmt.Sprintf(`{"target_user_id":%q,"message_id":%d,"reason":"  kasar  "}`, teacher, budiMsg))
	if status != http.StatusCreated {
		t.Fatalf("message report = %d %v", status, body)
	}
	if tgt := body["target"].(map[string]any)["id"]; tgt != budi.String() || body["message_text"] != "kata ****" ||
		body["room_id"] != classroom.String() || body["reason"] != "kasar" || body["status"] != "open" {
		t.Fatalf("report body = %v", body)
	}
	msgReportID := body["id"].(string)

	for _, tc := range []struct {
		name   string
		user   uuid.UUID
		body   string
		status int
	}{
		{"foreign message", ani, fmt.Sprintf(`{"target_user_id":%q,"message_id":%d,"reason":"x"}`, outsider, foreignMsg), http.StatusNotFound},
		{"own message", ani, fmt.Sprintf(`{"target_user_id":%q,"message_id":%d,"reason":"x"}`, ani, aniMsg), http.StatusNotFound},
		{"missing message", ani, fmt.Sprintf(`{"target_user_id":%q,"message_id":999999,"reason":"x"}`, budi), http.StatusNotFound},
		{"outsider reporter", outsider, fmt.Sprintf(`{"target_user_id":%q,"room_id":%q,"reason":"x"}`, budi, classroom), http.StatusNotFound},
		{"foreign target", ani, fmt.Sprintf(`{"target_user_id":%q,"room_id":%q,"reason":"x"}`, outsider, classroom), http.StatusNotFound},
		{"foreign room", ani, fmt.Sprintf(`{"target_user_id":%q,"room_id":%q,"reason":"x"}`, budi, otherRoom), http.StatusNotFound},
		{"self", ani, fmt.Sprintf(`{"target_user_id":%q,"room_id":%q,"reason":"x"}`, ani, classroom), http.StatusBadRequest},
		{"no room or message", ani, fmt.Sprintf(`{"target_user_id":%q,"reason":"x"}`, budi), http.StatusBadRequest},
		{"empty reason", ani, fmt.Sprintf(`{"target_user_id":%q,"room_id":%q,"reason":"  "}`, budi, classroom), http.StatusBadRequest},
		{"unknown field", ani, fmt.Sprintf(`{"target_user_id":%q,"room_id":%q,"reason":"x","text":"forged"}`, budi, classroom), http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if status, body := call(tc.user, http.MethodPost, "/api/reports", tc.body); status != tc.status {
				t.Fatalf("status = %d want %d (%v)", status, tc.status, body)
			}
		})
	}
	// User report (campus or unsaved chat): room + target.
	status, body = call(budi, http.MethodPost, "/api/reports", fmt.Sprintf(`{"target_user_id":%q,"room_id":%q,"reason":"mengganggu"}`, ani, classroom))
	if status != http.StatusCreated || body["message_id"] != nil {
		t.Fatalf("user report = %d %v", status, body)
	}

	reports := "/api/orgs/" + school.ID.String() + "/reports"
	if status, _ := call(ani, http.MethodGet, reports, ""); status != http.StatusForbidden {
		t.Fatalf("participant list = %d", status)
	}
	if status, _ := call(outsider, http.MethodGet, reports, ""); status != http.StatusNotFound {
		t.Fatalf("outsider list = %d", status)
	}
	status, body = call(teacher, http.MethodGet, reports+"?limit=1", "")
	if status != http.StatusOK || len(body["items"].([]any)) != 1 || body["next_cursor"] == nil {
		t.Fatalf("teacher list page 1 = %d %v", status, body)
	}
	status, body = call(teacher, http.MethodGet, reports+"?limit=1&cursor="+body["next_cursor"].(string), "")
	if status != http.StatusOK || len(body["items"].([]any)) != 1 || body["next_cursor"] != nil {
		t.Fatalf("teacher list page 2 = %d %v", status, body)
	}
	if status, _ := call(teacher, http.MethodGet, reports+"?status=bogus", ""); status != http.StatusBadRequest {
		t.Fatalf("bad status = %d", status)
	}

	review := "/api/reports/" + msgReportID
	if status, _ := call(ani, http.MethodPatch, review, `{"status":"reviewed"}`); status != http.StatusForbidden {
		t.Fatalf("participant review = %d", status)
	}
	if status, _ := call(outsider, http.MethodPatch, review, `{"status":"reviewed"}`); status != http.StatusNotFound {
		t.Fatalf("outsider review = %d", status)
	}
	if status, _ := call(teacher, http.MethodPatch, review, `{"status":"open"}`); status != http.StatusBadRequest {
		t.Fatalf("reopen = %d", status)
	}
	status, body = call(teacher, http.MethodPatch, review, `{"status":"reviewed"}`)
	if status != http.StatusOK || body["status"] != "reviewed" || body["reviewed_by"] != teacher.String() {
		t.Fatalf("review = %d %v", status, body)
	}
	status, body = call(teacher, http.MethodGet, reports+"?status=open", "")
	if status != http.StatusOK || len(body["items"].([]any)) != 1 {
		t.Fatalf("open filter = %d %v", status, body)
	}

	// Retention: purging the message keeps the report with a null message.
	if _, err := pool.Exec(ctx, "DELETE FROM chat_messages WHERE id=$1", budiMsg); err != nil {
		t.Fatal(err)
	}
	status, body = call(teacher, http.MethodGet, reports+"?status=reviewed", "")
	items := body["items"].([]any)
	if status != http.StatusOK || len(items) != 1 || items[0].(map[string]any)["message_text"] != nil {
		t.Fatalf("after purge = %d %v", status, body)
	}

	// Moderation log: admin only, organization scoped.
	if _, err := pool.Exec(ctx, "INSERT INTO moderation_actions (room_id, actor_id, target_id, action, created_at) VALUES ($1,$2,$3,'kick',$4), ($1,$2,NULL,'lock',$4), ($5,$6,NULL,'lock',$4)",
		classroom, teacher, budi, now, otherRoom, outsider); err != nil {
		t.Fatal(err)
	}
	logPath := "/api/orgs/" + school.ID.String() + "/moderation-log"
	if status, _ := call(teacher, http.MethodGet, logPath, ""); status != http.StatusForbidden {
		t.Fatalf("teacher log = %d", status)
	}
	status, body = call(admin, http.MethodGet, logPath, "")
	items = body["items"].([]any)
	if status != http.StatusOK || len(items) != 2 {
		t.Fatalf("admin log = %d %v", status, body)
	}
	for _, it := range items {
		m := it.(map[string]any)
		if m["room_id"] != classroom.String() {
			t.Fatalf("foreign action leaked: %v", m)
		}
		if m["action"] == "lock" && m["target"] != nil {
			t.Fatalf("lock has target: %v", m)
		}
	}
}
