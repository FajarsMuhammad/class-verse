package realtime

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// The first admission was valid, then revocation wins before join finishes.
type revokedGuestGate struct {
	adm   Admission
	calls atomic.Int32
}

func (g *revokedGuestGate) VerifyGuest(string) (uuid.UUID, error) { return g.adm.UserID, nil }
func (g *revokedGuestGate) AdmitGuest(context.Context, uuid.UUID, uuid.UUID) (Admission, error) {
	if g.calls.Add(1) == 1 {
		return g.adm, nil
	}
	return Admission{}, ErrNoAccess
}

func TestGuestFailedFinalAdmissionNeverWritesRoomData(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	hub := NewHub(map[string]*MapData{"campus": loadCampus(t)}, nil, time.Now, log)
	roomID, userID := uuid.New(), uuid.New()
	gate := &revokedGuestGate{adm: Admission{RoomID: roomID, OrgID: uuid.New(), MapKey: "campus", Capacity: 40, UserID: userID, Name: "Guest", Role: RoleGuest, GuestExpiresAt: time.Now().Add(time.Hour)}}
	gateway, err := NewGateway(hub, fakeTokens{}, fakeAdmitter{}, testOrigin, log)
	if err != nil {
		t.Fatal(err)
	}
	gateway.SetGuests(gate)
	router := chi.NewRouter()
	gateway.Routes(router)
	server := httptest.NewServer(router)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = hub.Shutdown(ctx)
		server.Close()
	})
	env := &wsEnv{srv: server}
	conn, err := env.dial(t, testOrigin, roomID)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.CloseNow() }()
	send(t, conn, TypeAuth, 1, AuthMsg{Token: "guest"})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, data, err := conn.Read(ctx)
	if err == nil {
		t.Fatalf("failed final admission leaked room payload: %s", data)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("socket was not closed")
	}
	if websocket.CloseStatus(err) != closeForbidden {
		t.Fatalf("close status: %v", err)
	}
}
