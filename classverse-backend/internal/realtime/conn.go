package realtime

import (
	"encoding/json"
	"sync"

	"github.com/coder/websocket"
	"github.com/google/uuid"
)

// outBuffer is the per-connection outgoing queue. A full queue means the
// client cannot keep up; the connection is closed so the room never blocks.
const outBuffer = 64

// Close codes (docs/design.md, slice 3).
const (
	closeUnauthenticated websocket.StatusCode = 4401
	closeForbidden       websocket.StatusCode = 4403
	closeNotFound        websocket.StatusCode = 4404
	closeReplaced        websocket.StatusCode = 4409
)

// outMsg is a message waiting to be written; the writer adds seq.
type outMsg struct {
	t string
	d json.RawMessage
}

func newOut(t string, v any) outMsg {
	b, err := json.Marshal(v)
	if err != nil {
		// All payloads are plain structs; a marshal error is a programming bug.
		panic("realtime: marshal " + t + ": " + err.Error())
	}
	return outMsg{t: t, d: b}
}

// conn is one client connection as seen by rooms and the hub. Methods are
// safe for concurrent use.
type conn struct {
	userID uuid.UUID
	// list is mode HP (set once before the conn is shared, read-only after).
	list   bool
	out    chan outMsg
	done   chan struct{}
	once   sync.Once
	code   websocket.StatusCode // valid after done is closed
	reason string
}

func newConn(userID uuid.UUID) *conn {
	return &conn{userID: userID, out: make(chan outMsg, outBuffer), done: make(chan struct{})}
}

// send queues m without blocking. It returns false when the connection is
// closed or its queue is full; in the latter case the connection is closed.
func (c *conn) send(m outMsg) bool {
	select {
	case <-c.done:
		return false
	default:
	}
	select {
	case c.out <- m:
		return true
	default:
		c.close(websocket.StatusPolicyViolation, "slow consumer")
		return false
	}
}

func (c *conn) sendError(e ErrorMsg) {
	c.send(newOut(TypeError, e))
}

// close marks the connection for closing with code; the first call wins.
func (c *conn) close(code websocket.StatusCode, reason string) {
	c.once.Do(func() {
		c.code, c.reason = code, reason
		close(c.done)
	})
}
