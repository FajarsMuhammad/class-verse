package room

import (
	"context"
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
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/auth"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/org"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/dbtest"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

// busyRooms reports a busy room the way the hub adapter does.
type busyRooms struct {
	busy    bool
	toggled []bool
}

func (busyRooms) Occupancy(uuid.UUID) (int, bool) { return 0, false }

func (b *busyRooms) SetAllChat(_ uuid.UUID, on bool) { b.toggled = append(b.toggled, on) }

func (b busyRooms) WhileEmpty(_ context.Context, _ uuid.UUID, fn func() error) error {
	if b.busy {
		return ErrBusy
	}
	return fn()
}

func TestHandler(t *testing.T) {
	pool := dbtest.New(t)
	ctx := t.Context()
	now := time.Date(2026, 3, 1, 8, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	q := query.New(pool)
	newUser := func(email string) uuid.UUID {
		t.Helper()
		u, err := q.CreateTestUser(ctx, query.CreateTestUserParams{Email: email, DisplayName: "User",
			BirthDate: pgtype.Date{Time: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true}})
		if err != nil {
			t.Fatal(err)
		}
		return u.ID
	}
	teacher, participant, outsider := newUser("t@example.test"), newUser("p@example.test"), newUser("o@example.test")
	orgSvc := org.NewService(org.NewStore(pool), clock, log)
	school, err := orgSvc.Create(ctx, teacher, org.CreateInput{Name: "Handler School", Slug: "handler-school"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.CreateMembership(ctx, query.CreateMembershipParams{OrgID: school.ID, UserID: participant, Role: query.OrgRoleParticipant, JoinedAt: pgtype.Timestamptz{Time: now, Valid: true}}); err != nil {
		t.Fatal(err)
	}
	live := &busyRooms{}
	tokens := auth.NewTokens(ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize)), clock)
	router := chi.NewRouter()
	router.Route("/api", func(r chi.Router) {
		r.Use(auth.RequireUser(tokens, log))
		NewHandler(NewService(NewStore(pool), orgSvc, testBuildings{}, live, clock, log), log).Routes(r)
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
	errCode := func(b map[string]any) string {
		e, _ := b["error"].(map[string]any)
		s, _ := e["code"].(string)
		return s
	}
	orgRooms := "/api/orgs/" + school.ID.String() + "/rooms"

	status, body := call(participant, http.MethodPost, orgRooms, `{"name":"Biologi","map_key":"classroom","building_key":"kelas-a"}`)
	if status != http.StatusForbidden {
		t.Fatalf("participant create = %d %v", status, body)
	}
	status, body = call(teacher, http.MethodPost, orgRooms, `{"name":"Biologi","map_key":"classroom","building_key":"kelas-a","capacity":500}`)
	if status != http.StatusBadRequest {
		t.Fatalf("unknown field capacity accepted = %d %v", status, body)
	}
	status, body = call(teacher, http.MethodPost, orgRooms, `{"name":"Biologi","map_key":"classroom","building_key":"kelas-a"}`)
	if status != http.StatusCreated || body["capacity"] != float64(40) {
		t.Fatalf("create = %d %v", status, body)
	}
	roomPath := "/api/rooms/" + body["id"].(string)
	staff, staffBody := call(teacher, http.MethodPost, orgRooms, `{"name":"Rapat guru","map_key":"classroom","building_key":"ruang-guru"}`)
	if staff != http.StatusCreated {
		t.Fatalf("staff create = %d %v", staff, staffBody)
	}

	tests := []struct {
		name   string
		user   uuid.UUID
		method string
		path   string
		body   string
		status int
		code   string
	}{
		{"outsider list", outsider, http.MethodGet, orgRooms, "", http.StatusNotFound, "not_found"},
		{"outsider get", outsider, http.MethodGet, roomPath, "", http.StatusNotFound, "not_found"},
		{"participant hidden staff room", participant, http.MethodGet, "/api/rooms/" + staffBody["id"].(string), "", http.StatusNotFound, "not_found"},
		{"participant patch", participant, http.MethodPatch, roomPath, `{"name":"Kimia"}`, http.StatusForbidden, "forbidden"},
		{"map immutable", teacher, http.MethodPatch, roomPath, `{"map_key":"garden"}`, http.StatusBadRequest, "validation_failed"},
		{"bad limit", teacher, http.MethodGet, orgRooms + "?limit=0", "", http.StatusBadRequest, "validation_failed"},
		{"bad cursor", teacher, http.MethodGet, orgRooms + "?cursor=bm90LWEtY3Vyc29y", "", http.StatusBadRequest, "validation_failed"},
		{"open now", teacher, http.MethodPatch, roomPath, `{"scheduled_at":null}`, http.StatusOK, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, body := call(tt.user, tt.method, tt.path, tt.body)
			if status != tt.status || errCode(body) != tt.code {
				t.Fatalf("status=%d code=%q body=%v", status, errCode(body), body)
			}
		})
	}

	if status, _ := call(participant, http.MethodPatch, roomPath, `{"all_chat_enabled":false}`); status != http.StatusForbidden {
		t.Fatalf("participant toggle = %d", status)
	}
	if status, _ := call(teacher, http.MethodPatch, roomPath, `{"all_chat_enabled":"no"}`); status != http.StatusBadRequest {
		t.Fatalf("bad toggle = %d", status)
	}
	status, body = call(teacher, http.MethodPatch, roomPath, `{"all_chat_enabled":false}`)
	if status != http.StatusOK || body["all_chat_enabled"] != false || body["name"] != "Biologi" {
		t.Fatalf("toggle = %d %v", status, body)
	}
	if len(live.toggled) != 1 || live.toggled[0] {
		t.Fatalf("hub not told: %v", live.toggled)
	}
	if _, got := call(teacher, http.MethodGet, roomPath, ""); got["all_chat_enabled"] != false {
		t.Fatalf("toggle not persisted: %v", got)
	}

	_, list := call(participant, http.MethodGet, orgRooms, "")
	if items := list["items"].([]any); len(items) != 1 {
		t.Fatalf("participant sees %d rooms, want 1 (staff hidden)", len(items))
	}

	live.busy = true
	if status, body := call(teacher, http.MethodDelete, roomPath, ""); status != http.StatusConflict || errCode(body) != "room_busy" {
		t.Fatalf("busy archive = %d %v", status, body)
	}
	live.busy = false
	if status, _ := call(teacher, http.MethodDelete, roomPath, ""); status != http.StatusNoContent {
		t.Fatalf("archive = %d", status)
	}
	if status, _ := call(teacher, http.MethodGet, roomPath, ""); status != http.StatusNotFound {
		t.Fatalf("archived get = %d", status)
	}
}
