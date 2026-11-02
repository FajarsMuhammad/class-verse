package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type fakeTokens map[string]uuid.UUID

func (f fakeTokens) Verify(raw string) (uuid.UUID, error) {
	if id, ok := f[raw]; ok {
		return id, nil
	}
	return uuid.Nil, errors.New("bad token")
}

type fakeAdmitter struct {
	room    uuid.UUID
	members map[uuid.UUID]string
}

func (f fakeAdmitter) Admit(_ context.Context, roomID, userID uuid.UUID) (Admission, error) {
	name, ok := f.members[userID]
	if roomID != f.room || !ok {
		return Admission{}, ErrNoAccess
	}
	return Admission{RoomID: roomID, MapKey: "campus", Capacity: 200, UserID: userID, Name: name, Avatar: "default", Role: "participant"}, nil
}

type admissionMap map[uuid.UUID]Admission

func (a admissionMap) Admit(_ context.Context, roomID, userID uuid.UUID) (Admission, error) {
	adm, ok := a[roomID]
	if !ok {
		return Admission{}, ErrNoAccess
	}
	adm.UserID = userID
	return adm, nil
}

func TestGatewayTravelAdmission(t *testing.T) {
	e := newWSEnv(t)
	source := Admission{RoomID: e.room, OrgID: uuid.New(), MapKey: campusKey, Capacity: 200, UserID: e.ani, Role: "participant"}
	c := newConn(e.ani)
	from, err := e.hub.join(c, source)
	if err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Hour)
	for _, tt := range []struct {
		name     string
		orgID    uuid.UUID
		building string
		schedule *time.Time
		code     string
	}{
		{"other organization", uuid.New(), "kelas-a", nil, CodeNotFound},
		{"staff room", source.OrgID, staffBuildingKey, nil, CodeForbidden},
		{"scheduled", source.OrgID, "kelas-a", &future, CodeScheduled},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dest := Admission{RoomID: uuid.New(), OrgID: tt.orgID, MapKey: "classroom", BuildingKey: tt.building, ScheduledAt: tt.schedule, Role: "participant"}
			g := &Gateway{hub: e.hub, admit: admissionMap{dest.RoomID: dest}}
			_, err := g.travel(t.Context(), c, from, dest.RoomID)
			if got := travelError(err, 1).Code; got != tt.code {
				t.Fatalf("code=%s want=%s", got, tt.code)
			}
		})
	}
}

func TestGatewayEntryReconnect(t *testing.T) {
	e := newWSEnv(t)
	campus := Admission{RoomID: e.room, OrgID: uuid.New(), MapKey: campusKey, Role: "participant"}
	dest := Admission{RoomID: uuid.New(), OrgID: campus.OrgID, MapKey: "classroom", Role: "participant"}
	g := &Gateway{hub: e.hub, admit: admissionMap{campus.RoomID: campus, dest.RoomID: dest}}
	if _, err := g.entry(t.Context(), e.ani, dest); !errors.Is(err, errNotAtDoor) {
		t.Fatalf("cold entry=%v", err)
	}
	e.hub.setLocation(e.ani, dest.RoomID)
	got, err := g.entry(t.Context(), e.ani, campus)
	if err != nil || got.RoomID != dest.RoomID {
		t.Fatalf("resume=%+v err=%v", got, err)
	}
	delete(g.admit.(admissionMap), dest.RoomID)
	got, err = g.entry(t.Context(), e.ani, campus)
	if err != nil || got.RoomID != campus.RoomID {
		t.Fatalf("fallback=%+v err=%v", got, err)
	}
}

type wsEnv struct {
	srv    *httptest.Server
	hub    *Hub
	room   uuid.UUID
	ani    uuid.UUID
	budi   uuid.UUID
	tokens fakeTokens
}

const testOrigin = "http://localhost:5173"

func newWSEnv(t *testing.T) *wsEnv {
	t.Helper()
	md := loadCampus(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	hub := NewHub(map[string]*MapData{"campus": md}, nil, func() time.Time { return time.Now().UTC() }, log)
	e := &wsEnv{hub: hub, room: uuid.New(), ani: uuid.New(), budi: uuid.New()}
	e.tokens = fakeTokens{"tok-ani": e.ani, "tok-budi": e.budi, "tok-stranger": uuid.New()}
	gw, err := NewGateway(hub, e.tokens, fakeAdmitter{room: e.room, members: map[uuid.UUID]string{e.ani: "Ani", e.budi: "Budi"}}, testOrigin, log)
	if err != nil {
		t.Fatal(err)
	}
	r := chi.NewRouter()
	gw.Routes(r)
	e.srv = httptest.NewServer(r)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := hub.Shutdown(ctx); err != nil {
			t.Errorf("hub shutdown: %v", err)
		}
		e.srv.Close()
	})
	return e
}

func (e *wsEnv) dial(t *testing.T, origin string, room uuid.UUID) (*websocket.Conn, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	u := "ws" + strings.TrimPrefix(e.srv.URL, "http") + "/ws/rooms/" + room.String()
	h := http.Header{}
	if origin != "" {
		h.Set("Origin", origin)
	}
	c, _, err := websocket.Dial(ctx, u, &websocket.DialOptions{HTTPHeader: h})
	return c, err
}

func send(t *testing.T, c *websocket.Conn, typ string, seq uint64, d any) {
	t.Helper()
	b, _ := json.Marshal(d)
	env, _ := json.Marshal(Envelope{T: typ, Seq: seq, D: b})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := c.Write(ctx, websocket.MessageText, env); err != nil {
		t.Fatal(err)
	}
}

// next reads until a message of type want arrives (other types are skipped).
func next(t *testing.T, c *websocket.Conn, want string) (Envelope, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for {
		_, data, err := c.Read(ctx)
		if err != nil {
			return Envelope{}, err
		}
		var env Envelope
		if err := json.Unmarshal(data, &env); err != nil {
			t.Fatal(err)
		}
		if env.T == want {
			return env, nil
		}
	}
}

func (e *wsEnv) connect(t *testing.T, token string) *websocket.Conn {
	t.Helper()
	c, err := e.dial(t, testOrigin, e.room)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.CloseNow() })
	send(t, c, TypeAuth, 1, AuthMsg{Token: token})
	if _, err := next(t, c, TypeWelcome); err != nil {
		t.Fatalf("welcome: %v", err)
	}
	return c
}

func TestGatewayRejects(t *testing.T) {
	e := newWSEnv(t)

	if _, err := e.dial(t, "https://evil.example", e.room); err == nil {
		t.Error("foreign origin accepted")
	}
	if _, err := e.dial(t, "", e.room); err == nil {
		t.Error("missing origin accepted")
	}
	if _, err := e.dial(t, e.srv.URL, e.room); err == nil {
		t.Error("backend same-host origin bypassed configured frontend origin")
	}

	tests := []struct {
		name  string
		first func(c *websocket.Conn)
		room  uuid.UUID
		code  websocket.StatusCode
	}{
		{"wrong first message", func(c *websocket.Conn) { send(t, c, TypePing, 1, PingMsg{}) }, e.room, closeUnauthenticated},
		{"bad token", func(c *websocket.Conn) { send(t, c, TypeAuth, 1, AuthMsg{Token: "nope"}) }, e.room, closeUnauthenticated},
		{"not a member", func(c *websocket.Conn) { send(t, c, TypeAuth, 1, AuthMsg{Token: "tok-stranger"}) }, e.room, closeNotFound},
		{"unknown room", func(c *websocket.Conn) { send(t, c, TypeAuth, 1, AuthMsg{Token: "tok-ani"}) }, uuid.New(), closeNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := e.dial(t, testOrigin, tt.room)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = c.CloseNow() }()
			tt.first(c)
			_, err = next(t, c, "never")
			if got := websocket.CloseStatus(err); got != tt.code {
				t.Errorf("close = %d (%v), want %d", got, err, tt.code)
			}
		})
	}
}

func TestGatewayAuthTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("waits for the 5 s auth timeout")
	}
	e := newWSEnv(t)
	c, err := e.dial(t, testOrigin, e.room)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.CloseNow() }()
	ctx, cancel := context.WithTimeout(context.Background(), authTimeout+2*time.Second)
	defer cancel()
	_, _, err = c.Read(ctx)
	if got := websocket.CloseStatus(err); got != closeUnauthenticated {
		t.Errorf("close = %d (%v)", got, err)
	}
}

func TestGatewayTwoUsersSeeEachOther(t *testing.T) {
	e := newWSEnv(t)
	ani := e.connect(t, "tok-ani")
	budi := e.connect(t, "tok-budi")

	env, err := next(t, ani, TypeJoined)
	if err != nil {
		t.Fatal(err)
	}
	var p Participant
	_ = json.Unmarshal(env.D, &p)
	if p.ID != e.budi || p.Name != "Budi" {
		t.Fatalf("joined = %+v", p)
	}

	send(t, budi, TypeMove, 2, MoveMsg{X: p.X + 8, Y: p.Y, Dir: DirRight})
	for {
		env, err = next(t, ani, TypeSnapshot)
		if err != nil {
			t.Fatal(err)
		}
		var s SnapshotMsg
		_ = json.Unmarshal(env.D, &s)
		if len(s.P) == 1 && s.P[0].ID == e.budi && s.P[0].X == p.X+8 {
			break
		}
	}

	send(t, ani, TypePing, 3, PingMsg{TS: 42})
	env, err = next(t, ani, TypePong)
	var pong PongMsg
	if err != nil || json.Unmarshal(env.D, &pong) != nil || pong.TS != 42 {
		t.Errorf("pong = %+v, %v", pong, err)
	}
}

func TestGatewayProfilePublish(t *testing.T) {
	e := newWSEnv(t)
	a := e.connect(t, "tok-ani")
	b := e.connect(t, "tok-budi")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := e.hub.PublishProfile(ctx, e.ani, "Ani Baru", "cap-mangga"); err != nil {
		t.Fatal(err)
	}
	for _, c := range []*websocket.Conn{a, b} {
		env, err := next(t, c, TypeProfileUpdated)
		var p ProfileUpdatedMsg
		if err != nil || json.Unmarshal(env.D, &p) != nil || p.ID != e.ani || p.Avatar != "cap-mangga" {
			t.Fatal("profile event not delivered")
		}
	}
}

func TestGatewayReplacedConnection(t *testing.T) {
	e := newWSEnv(t)
	first := e.connect(t, "tok-ani")
	e.connect(t, "tok-ani")
	_, err := next(t, first, "never")
	if got := websocket.CloseStatus(err); got != closeReplaced {
		t.Errorf("first tab close = %d (%v), want %d", got, err, closeReplaced)
	}
}

func TestHubStopsIdleRoom(t *testing.T) {
	md := loadCampus(t)
	clk := &fakeClock{t: time.Now().UTC()}
	hub := NewHub(map[string]*MapData{"campus": md}, nil, clk.Now, slog.New(slog.NewTextHandler(io.Discard, nil)))
	defer func() { _ = hub.Shutdown(context.Background()) }()

	c := newConn(uuid.New())
	rm, err := hub.join(c, Admission{RoomID: uuid.New(), MapKey: "campus", Capacity: 10, UserID: c.userID})
	if err != nil {
		t.Fatal(err)
	}
	rm.post(leaveCmd{c: c})
	sync := make(chan struct{})
	rm.post(probeCmd{fn: func(*room) { close(sync) }}) // leave is handled before the clock moves
	<-sync
	clk.Advance(holdDuration + time.Second) // avatar released on next tick
	waitFor(t, func() bool { return roomEmpty(rm) })
	clk.Advance(idleTimeout + time.Second)
	waitFor(t, func() bool { return hub.roomCount() == 0 })
	select {
	case <-rm.done:
	case <-time.After(2 * time.Second):
		t.Error("idle room goroutine did not stop")
	}
}

func roomEmpty(r *room) bool {
	res := make(chan bool, 1)
	if !r.post(probeCmd{fn: func(r *room) { res <- len(r.members) == 0 && !r.emptySince.IsZero() }}) {
		return true
	}
	return <-res
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met in time")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestGatewayChatFiltered(t *testing.T) {
	e := newWSEnv(t)
	ani := e.connect(t, "tok-ani")
	budi := e.connect(t, "tok-budi")
	send(t, ani, TypeChat, 2, ChatInMsg{Text: "  wa bangsat 0812-3456-7890  ", Scope: ScopeRoom})
	env, err := next(t, budi, TypeChat)
	if err != nil {
		t.Fatal(err)
	}
	var m ChatOutMsg
	if err := json.Unmarshal(env.D, &m); err != nil {
		t.Fatal(err)
	}
	if m.Text != "wa **** [disembunyikan]" || m.From.ID != e.ani || m.From.Name != "Ani" || m.At.IsZero() {
		t.Fatalf("chat = %+v", m)
	}
	send(t, ani, TypeChat, 3, ChatInMsg{Text: "   ", Scope: ScopeNear})
	env, err = next(t, ani, TypeError)
	var em ErrorMsg
	if err != nil || json.Unmarshal(env.D, &em) != nil || em.Code != CodeBadMessage || em.Seq != 3 {
		t.Fatalf("empty chat error = %+v %v", em, err)
	}
}
