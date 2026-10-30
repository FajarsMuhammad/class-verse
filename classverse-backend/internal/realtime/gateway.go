package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
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

// NewGateway accepts WebSocket upgrades only from frontendURL's origin.
func NewGateway(hub *Hub, tokens TokenVerifier, admit Admitter, frontendURL string, log *slog.Logger) (*Gateway, error) {
	u, err := url.Parse(frontendURL)
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("realtime: invalid frontend url %q", frontendURL)
	}
	return &Gateway{hub: hub, tokens: tokens, admit: admit, patterns: []string{u.Scheme + "://" + u.Host}, log: log}, nil
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
	if r.Header.Get("Origin") != g.patterns[0] {
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
	userID, adm, err := g.authenticate(ctx, ws, roomID)
	if err != nil {
		return
	}
	c := newConn(userID)
	g.hub.claim(c)
	defer g.hub.release(c)

	rm, err := g.hub.join(c, adm)
	if err != nil {
		code, msg := CodeUnavailable, "Ruang sedang tidak tersedia"
		var re *roomError
		if errors.As(err, &re) {
			code, msg = re.code, re.msg
		}
		writeNow(ctx, ws, newOut(TypeError, ErrorMsg{Code: code, Message: msg}), 1)
		_ = ws.Close(websocket.StatusTryAgainLater, code)
		return
	}
	g.log.InfoContext(ctx, "ws connected", slog.String("user_id", userID.String()), slog.String("room_id", roomID.String()))

	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		g.readLoop(ctx, ws, c, rm)
	}()
	g.writeLoop(ctx, ws, c, readDone)
	rm.post(leaveCmd{c: c})
	<-readDone
	g.log.InfoContext(ctx, "ws closed", slog.String("user_id", userID.String()),
		slog.String("room_id", roomID.String()), slog.Int("code", int(c.code)), slog.String("reason", c.reason))
}

// authenticate waits for the auth message and admits the user to the room.
// On failure it has already closed the socket.
func (g *Gateway) authenticate(ctx context.Context, ws *websocket.Conn, roomID uuid.UUID) (uuid.UUID, Admission, error) {
	fail := func(status websocket.StatusCode, code, msg string) (uuid.UUID, Admission, error) {
		writeNow(ctx, ws, newOut(TypeError, ErrorMsg{Code: code, Message: msg}), 1)
		_ = ws.Close(status, code)
		return uuid.Nil, Admission{}, errors.New(code)
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
			return uuid.Nil, Admission{}, res.err
		}
		data = res.data
	case <-timer.C:
		_ = ws.Close(closeUnauthenticated, "auth timeout")
		<-got
		return uuid.Nil, Admission{}, errors.New("auth timeout")
	}
	actx, cancel := context.WithTimeout(ctx, authTimeout)
	defer cancel()
	var env Envelope
	var am AuthMsg
	if json.Unmarshal(data, &env) != nil || env.T != TypeAuth || json.Unmarshal(env.D, &am) != nil {
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
	return userID, adm, nil
}

func (g *Gateway) readLoop(ctx context.Context, ws *websocket.Conn, c *conn, rm *room) {
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
			if !rm.post(moveCmd{c: c, seq: env.Seq, m: m}) {
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
