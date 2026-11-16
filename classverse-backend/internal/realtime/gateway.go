package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/moderation"
)

const (
	authTimeout  = 5 * time.Second
	writeTimeout = 5 * time.Second
	readLimit    = 4 << 10 // bytes per message; client messages are tiny
)

// TokenVerifier checks an access token and returns the user id
// (implemented by auth.Tokens).
type TokenVerifier interface {
	Verify(raw string) (uuid.UUID, error)
}

// Admitter looks up a room and the user's membership in its organization
// (implemented in cmd/server over the rooms, memberships, and users tables).
// It returns ErrNoAccess when the room does not exist or the user is not a
// member, so the two cases are indistinguishable to the client.
type Admitter interface {
	Admit(ctx context.Context, roomID, userID uuid.UUID) (Admission, error)
}

// ErrNoAccess means the room is missing or not visible to the user.
var ErrNoAccess = errors.New("no access to room")

// Gateway upgrades HTTP requests to WebSocket connections and feeds them to
// the hub.
type Gateway struct {
	hub      *Hub
	tokens   TokenVerifier
	admit    Admitter
	patterns []string
	log      *slog.Logger
}

// NewGateway accepts WebSocket upgrades only from frontendURL's origin, plus
// devOrigins (development LAN testing; config refuses them in production).
func NewGateway(hub *Hub, tokens TokenVerifier, admit Admitter, frontendURL string, log *slog.Logger, devOrigins ...string) (*Gateway, error) {
	u, err := url.Parse(frontendURL)
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("realtime: invalid frontend url %q", frontendURL)
	}
	patterns := append([]string{u.Scheme + "://" + u.Host}, devOrigins...)
	return &Gateway{hub: hub, tokens: tokens, admit: admit, patterns: patterns, log: log}, nil
}

func (g *Gateway) allowed(origin string) bool {
	return origin != "" && slices.Contains(g.patterns, origin)
}

// Routes mounts GET /ws/rooms/{room_id}.
func (g *Gateway) Routes(r chi.Router) {
	r.Get("/ws/rooms/{room_id}", g.serve)
}

func (g *Gateway) serve(w http.ResponseWriter, r *http.Request) {
	// Browsers always send Origin on WebSocket upgrades; requiring it blocks
	// cross-site WebSocket hijacking with the user's cookies.
	// Accept also permits same-host origins by default. Enforce the configured
	// scheme, host and port explicitly before handing off to the library.
	if !g.allowed(r.Header.Get("Origin")) {
		http.Error(w, "origin not allowed", http.StatusForbidden)
		return
	}
	roomID, err := uuid.Parse(chi.URLParam(r, "room_id"))
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: g.patterns})
	if err != nil {
		return // Accept already wrote the HTTP error
	}
	ws.SetReadLimit(readLimit)
	ctx := context.WithoutCancel(r.Context())
	g.session(ctx, ws, roomID)
}

// session runs one connection until it closes.
func (g *Gateway) session(ctx context.Context, ws *websocket.Conn, roomID uuid.UUID) {
	userID, adm, list, err := g.authenticate(ctx, ws, roomID)
	if err != nil {
		return
	}
	c := newConn(userID)
	c.list = list
	g.hub.claim(c)
	defer g.hub.release(c)

	adm, err = g.entry(ctx, userID, adm)
	var rm *room
	if err == nil {
		rm, err = g.hub.join(c, adm)
	}
	if err != nil {
		code, msg, status := CodeUnavailable, "Ruang sedang tidak tersedia", websocket.StatusTryAgainLater
		var re *roomError
		if errors.As(err, &re) {
			code, msg = re.code, re.msg
			if re == errNotAtDoor {
				status = closeForbidden
			}
		}
		writeNow(ctx, ws, newOut(TypeError, ErrorMsg{Code: code, Message: msg}), 1)
		_ = ws.Close(status, code)
		return
	}
	g.log.InfoContext(ctx, "ws connected", slog.String("user_id", userID.String()), slog.String("room_id", rm.id.String()))

	var cur atomic.Pointer[room]
	cur.Store(rm)
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		g.readLoop(ctx, ws, c, &cur)
	}()
	g.writeLoop(ctx, ws, c, readDone)
	<-readDone
	last := cur.Load()
	last.post(leaveCmd{c: c})
	g.log.InfoContext(ctx, "ws closed", slog.String("user_id", userID.String()),
		slog.String("room_id", last.id.String()), slog.Int("code", int(c.code)), slog.String("reason", c.reason))
}

// entry decides which room a new connection joins. A reconnect within the
// grace period resumes the last committed room of the same organization;
// otherwise only the campus can be joined directly, so the URL is never a way
// around doors, schedules, or Ruang Guru.
func (g *Gateway) entry(ctx context.Context, userID uuid.UUID, requested Admission) (Admission, error) {
	if last := g.hub.resume(userID); last != uuid.Nil {
		if last == requested.RoomID && !requested.Banned {
			return requested, nil
		}
		adm, err := g.admit.Admit(ctx, last, userID)
		if err == nil && !adm.Banned && adm.OrgID == requested.OrgID && (adm.BuildingKey != staffBuildingKey || adm.staff()) {
			return adm, nil
		}
	}
	if requested.MapKey != campusKey {
		return Admission{}, errNotAtDoor
	}
	return requested, nil
}

// authenticate waits for the auth message and admits the user to the room.
// It also reports whether the client declared list mode (mode HP). On failure
// it has already closed the socket.
func (g *Gateway) authenticate(ctx context.Context, ws *websocket.Conn, roomID uuid.UUID) (uuid.UUID, Admission, bool, error) {
	fail := func(status websocket.StatusCode, code, msg string) (uuid.UUID, Admission, bool, error) {
		writeNow(ctx, ws, newOut(TypeError, ErrorMsg{Code: code, Message: msg}), 1)
		_ = ws.Close(status, code)
		return uuid.Nil, Admission{}, false, errors.New(code)
	}

	// Read in a goroutine: cancelling a Read context makes the library drop
	// the TCP connection without a close frame, but the client should see 4401.
	type readResult struct {
		data []byte
		err  error
	}
	got := make(chan readResult, 1)
	go func() {
		_, data, err := ws.Read(ctx)
		got <- readResult{data, err}
	}()
	timer := time.NewTimer(authTimeout)
	defer timer.Stop()
	var data []byte
	select {
	case res := <-got:
		if res.err != nil {
			_ = ws.CloseNow()
			return uuid.Nil, Admission{}, false, res.err
		}
		data = res.data
	case <-timer.C:
		_ = ws.Close(closeUnauthenticated, "auth timeout")
		<-got
		return uuid.Nil, Admission{}, false, errors.New("auth timeout")
	}
	actx, cancel := context.WithTimeout(ctx, authTimeout)
	defer cancel()
	var env Envelope
	var am AuthMsg
	if json.Unmarshal(data, &env) != nil || env.T != TypeAuth || json.Unmarshal(env.D, &am) != nil ||
		(am.Mode != "" && am.Mode != ModeMap && am.Mode != ModeList) {
		return fail(closeUnauthenticated, CodeUnauthenticated, "Pesan pertama harus auth")
	}
	userID, err := g.tokens.Verify(am.Token)
	if err != nil {
		return fail(closeUnauthenticated, CodeUnauthenticated, "Sesi berakhir. Silakan masuk lagi.")
	}
	adm, err := g.admit.Admit(actx, roomID, userID)
	switch {
	case errors.Is(err, ErrNoAccess):
		return fail(closeNotFound, CodeNotFound, "Ruang tidak ditemukan")
	case err != nil:
		g.log.ErrorContext(ctx, "ws admit failed", slog.Any("err", err))
		return fail(websocket.StatusInternalError, CodeUnavailable, "Ruang sedang tidak tersedia")
	}
	return userID, adm, am.Mode == ModeList, nil
}

func (g *Gateway) readLoop(ctx context.Context, ws *websocket.Conn, c *conn, cur *atomic.Pointer[room]) {
	defer c.close(websocket.StatusNormalClosure, "")
	for {
		_, data, err := ws.Read(ctx)
		if err != nil {
			return
		}
		var env Envelope
		if json.Unmarshal(data, &env) != nil {
			c.sendError(ErrorMsg{Code: CodeBadMessage, Message: "Pesan tidak valid"})
			continue
		}
		switch env.T {
		case TypeMove:
			var m MoveMsg
			if json.Unmarshal(env.D, &m) != nil {
				c.sendError(ErrorMsg{Code: CodeBadMessage, Message: "Pesan tidak valid", Seq: env.Seq})
				continue
			}
			if !cur.Load().post(moveCmd{c: c, seq: env.Seq, m: m}) {
				return
			}
		case TypeTravel:
			var t TravelMsg
			if json.Unmarshal(env.D, &t) != nil {
				c.sendError(ErrorMsg{Code: CodeBadMessage, Message: "Pesan tidak valid", Seq: env.Seq})
				continue
			}
			to, err := g.travel(ctx, c, cur.Load(), t.RoomID)
			if err != nil {
				c.sendError(travelError(err, env.Seq))
				continue
			}
			cur.Store(to)
		case TypeEmote:
			var m EmoteMsg
			if json.Unmarshal(env.D, &m) != nil {
				c.sendError(ErrorMsg{Code: CodeBadMessage, Message: "Emote tidak valid", Seq: env.Seq})
				continue
			}
			if !cur.Load().post(emoteCmd{c: c, seq: env.Seq, key: m.Key}) {
				return
			}
		case TypeSeat:
			var m SeatMsg
			if json.Unmarshal(env.D, &m) != nil {
				c.sendError(ErrorMsg{Code: CodeBadMessage, Message: "Pesan tidak valid", Seq: env.Seq})
				continue
			}
			if !cur.Load().post(seatCmd{c: c, seq: env.Seq, zone: m.Zone}) {
				return
			}
		case TypeCmd:
			var m CommandMsg
			if err := json.Unmarshal(env.D, &m); err != nil {
				c.sendError(ErrorMsg{Code: CodeBadMessage, Message: "Perintah tidak valid", Seq: env.Seq})
				continue
			}
			if !cur.Load().post(commandCmd{c: c, seq: env.Seq, name: m.Name, target: m.Target, step: m.Step}) {
				return
			}
		case TypeChat:
			var m ChatInMsg
			if json.Unmarshal(env.D, &m) != nil {
				c.sendError(ErrorMsg{Code: CodeBadMessage, Message: "Pesan tidak valid", Seq: env.Seq})
				continue
			}
			text, err := moderation.FilterChat(m.Text)
			if err != nil {
				c.sendError(ErrorMsg{Code: CodeBadMessage, Message: chatTextError(err), Seq: env.Seq})
				continue
			}
			if !cur.Load().post(chatCmd{c: c, seq: env.Seq, text: text, scope: m.Scope}) {
				return
			}
		case TypePing:
			var p PingMsg
			_ = json.Unmarshal(env.D, &p)
			c.send(newOut(TypePong, PongMsg(p)))
		default:
			c.sendError(ErrorMsg{Code: CodeBadMessage, Message: "Tipe pesan tidak dikenal", Seq: env.Seq})
		}
	}
}

// travel admits the user to roomID and asks the hub to move them there.
func (g *Gateway) travel(ctx context.Context, c *conn, from *room, roomID uuid.UUID) (*room, error) {
	actx, cancel := context.WithTimeout(ctx, authTimeout)
	defer cancel()
	dest, err := g.admit.Admit(actx, roomID, c.userID)
	if err != nil {
		return nil, err
	}
	if dest.OrgID != from.orgID {
		return nil, ErrNoAccess // other organizations look like missing rooms
	}
	if dest.BuildingKey == staffBuildingKey && !dest.staff() {
		return nil, &roomError{code: CodeForbidden, msg: "Ruang Guru khusus pengajar dan admin"}
	}
	if dest.Banned {
		return nil, errKicked
	}
	if dest.ScheduledAt != nil && dest.ScheduledAt.After(g.hub.now()) && !dest.staff() {
		return nil, &roomError{code: CodeScheduled, msg: "Ruang belum dibuka. Tunggu jadwalnya."}
	}
	return g.hub.travel(c, from, dest)
}

func chatTextError(err error) string {
	switch {
	case errors.Is(err, moderation.ErrEmpty):
		return "Pesan kosong"
	case errors.Is(err, moderation.ErrTooLong):
		return "Pesan maksimal 500 karakter"
	}
	return "Pesan mengandung karakter yang tidak diperbolehkan"
}

func travelError(err error, seq uint64) ErrorMsg {
	var re *roomError
	switch {
	case errors.As(err, &re):
		return ErrorMsg{Code: re.code, Message: re.msg, Seq: seq}
	case errors.Is(err, ErrNoAccess):
		return ErrorMsg{Code: CodeNotFound, Message: "Ruang tidak ditemukan", Seq: seq}
	}
	return ErrorMsg{Code: CodeUnavailable, Message: "Ruang sedang tidak tersedia", Seq: seq}
}

// writeLoop drains the connection's queue until it is closed, then sends the
// close frame with the recorded code.
func (g *Gateway) writeLoop(ctx context.Context, ws *websocket.Conn, c *conn, readDone <-chan struct{}) {
	var seq uint64
	for {
		select {
		case m := <-c.out:
			seq++
			if err := write(ctx, ws, m, seq); err != nil {
				c.close(websocket.StatusAbnormalClosure, "write failed")
				_ = ws.CloseNow()
				return
			}
		case <-c.done:
			// Flush what is already queued (e.g. the error before a close).
			for {
				select {
				case m := <-c.out:
					seq++
					if write(ctx, ws, m, seq) != nil {
						_ = ws.CloseNow()
						return
					}
				default:
					_ = ws.Close(c.code, c.reason)
					return
				}
			}
		case <-readDone:
			_ = ws.CloseNow()
			return
		}
	}
}

func write(ctx context.Context, ws *websocket.Conn, m outMsg, seq uint64) error {
	b, err := json.Marshal(Envelope{T: m.t, Seq: seq, D: m.d})
	if err != nil {
		return err
	}
	wctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	return ws.Write(wctx, websocket.MessageText, b)
}

func writeNow(ctx context.Context, ws *websocket.Conn, m outMsg, seq uint64) {
	_ = write(ctx, ws, m, seq)
}
