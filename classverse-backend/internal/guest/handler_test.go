package guest

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
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

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/auth"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/org"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/api"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/dbtest"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

func TestGuestHTTPLifecycle(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.New(t)
	q := query.New(pool)
	now := time.Now().UTC()
	clock := func() time.Time { return now }
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	host, err := q.CreateTestUser(ctx, query.CreateTestUserParams{Email: "http-host@example.test", DisplayName: "Host"})
	if err != nil {
		t.Fatal(err)
	}
	school, err := q.CreateOrg(ctx, query.CreateOrgParams{Name: "School", Slug: "http-guest", CreatedBy: host.ID, CreatedAt: ts(now)})
	if err != nil {
		t.Fatal(err)
	}
	_, err = q.CreateMembership(ctx, query.CreateMembershipParams{OrgID: school.ID, UserID: host.ID, Role: query.OrgRoleTeacher, JoinedAt: ts(now)})
	if err != nil {
		t.Fatal(err)
	}
	room, err := q.CreateRoom(ctx, query.CreateRoomParams{OrgID: school.ID, Name: "Meeting", MapKey: "classroom", CreatedBy: host.ID, Now: ts(now)})
	if err != nil {
		t.Fatal(err)
	}
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tokens := auth.NewTokens(key, clock)
	svc := NewService(pool, org.NewService(org.NewStore(pool), clock, log), tokens, clock, log)
	handler := NewHandler(svc, "https://school.example", clock, log)
	router := chi.NewRouter()
	handler.PublicRoutes(router)
	router.Group(func(r chi.Router) { r.Use(auth.RequireGuest(tokens, log)); handler.GuestRoutes(r) })
	router.Group(func(r chi.Router) { r.Use(auth.RequireUser(tokens, log)); handler.HostRoutes(r) })
	hostToken, err := tokens.Issue(host.ID)
	if err != nil {
		t.Fatal(err)
	}
	base := "/rooms/" + room.ID.String()
	call := func(method, path, body, token string, status int) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.RemoteAddr = "127.0.0.1:1234"
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != status {
			t.Fatalf("%s %s: got %d want %d: %s", method, path, w.Code, status, w.Body.String())
		}
		return w
	}
	decode := func(w *httptest.ResponseRecorder, dst any) {
		t.Helper()
		if err := json.Unmarshal(w.Body.Bytes(), dst); err != nil {
			t.Fatal(err)
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("missing no-store")
		}
	}
	call("GET", base+"/guest-links", "", "", http.StatusUnauthorized)
	call("POST", base+"/guest-links", `{"expires_in_hours":25,"max_guests":41}`, hostToken, 400)
	call("POST", base+"/guest-links", `{"max_guests":0}`, hostToken, 400)
	call("POST", base+"/guest-links", `{"unknown":true}`, hostToken, 400)
	call("POST", "/rooms/not-uuid/guest-links", `{}`, hostToken, 404)
	var link api.GuestLink
	decode(call("POST", base+"/guest-links", `{}`, hostToken, 201), &link)
	var links struct {
		Items []api.GuestLink `json:"items"`
	}
	decode(call("GET", base+"/guest-links", "", hostToken, 200), &links)
	if len(links.Items) != 1 || links.Items[0].Id != link.Id {
		t.Fatal("link missing")
	}
	var preview api.GuestLinkPreview
	decode(call("GET", "/guest/links/"+link.Code, "", "", 200), &preview)
	if preview.RoomName != "Meeting" {
		t.Fatal(preview.RoomName)
	}
	call("GET", "/guest/links/bad", "", "", 404)
	path := "/guest/links/" + link.Code + "/requests"
	call("POST", path, `{"display_name":"x","request_key":"`+uuid.NewString()+`"}`, "", 400)
	call("POST", path, `{"display_name":"Guest","request_key":"00000000-0000-0000-0000-000000000000"}`, "", 400)
	requestKey := uuid.NewString()
	body := fmt.Sprintf(`{"display_name":"Guest One","request_key":%q}`, requestKey)
	var session api.GuestSession
	decode(call("POST", path, body, "", 201), &session)
	decode(call("POST", path, body, "", 201), &session)
	if session.Status != api.GuestSessionStatusPending {
		t.Fatal(session.Status)
	}
	call("GET", base+"/guest-links", "", session.Token, 401)
	call("GET", "/guest/me", "", hostToken, 401)
	decode(call("GET", "/guest/me", "", session.Token, 200), &session)
	var requests struct {
		Items []api.GuestRequest `json:"items"`
	}
	decode(call("GET", base+"/guest-requests", "", hostToken, 200), &requests)
	if len(requests.Items) != 1 {
		t.Fatal(len(requests.Items))
	}
	decisionPath := base + "/guest-requests/" + requests.Items[0].Id.String()
	call("POST", decisionPath, `{"decision":"bad"}`, hostToken, 400)
	var request api.GuestRequest
	decode(call("POST", decisionPath, `{"decision":"approve"}`, hostToken, 200), &request)
	call("POST", decisionPath, `{"decision":"deny"}`, hostToken, 404)
	decode(call("GET", "/guest/me", "", session.Token, 200), &session)
	if session.Status != api.GuestSessionStatusApproved {
		t.Fatal(session.Status)
	}
	call("DELETE", base+"/guest-links/"+uuid.NewString(), "", hostToken, 404)
	call("DELETE", base+"/guest-links/"+link.Id.String(), "", hostToken, 204)
	call("DELETE", base+"/guest-links/"+link.Id.String(), "", hostToken, 204)
	call("GET", "/guest/me", "", session.Token, 410)
	call("GET", "/guest/links/"+link.Code, "", "", 410)
	call("POST", path, body, "", 410)
	// Public preview budget is per client, including invalid codes.
	for range 30 {
		router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/guest/links/bad", nil))
	}
	rate := httptest.NewRecorder()
	router.ServeHTTP(rate, httptest.NewRequest("GET", "/guest/links/bad", nil))
	if rate.Code != 429 || rate.Header().Get("Retry-After") == "" {
		t.Fatal("missing rate limit")
	}
}
