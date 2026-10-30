package realtime

import (
	"encoding/json"

	"github.com/google/uuid"
)

// Envelope is the shape of every WebSocket message: {"t", "seq", "d"}.
// Message types are defined only here and mirrored in
// classverse-frontend/src/realtime/messages.ts.
type Envelope struct {
	T   string          `json:"t"`
	Seq uint64          `json:"seq"`
	D   json.RawMessage `json:"d,omitempty"`
}

// Client → server message types.
const (
	TypeAuth = "auth"
	TypeMove = "move"
	TypePing = "ping"
)

// Server → client message types.
const (
	TypeWelcome  = "welcome"
	TypeSnapshot = "snapshot"
	TypeJoined   = "joined"
	TypeLeft     = "left"
	TypePong     = "pong"
	TypeError    = "error"
)

// Direction an avatar faces.
type Direction string

// Valid directions.
const (
	DirUp    Direction = "up"
	DirDown  Direction = "down"
	DirLeft  Direction = "left"
	DirRight Direction = "right"
)

func (d Direction) valid() bool {
	return d == DirUp || d == DirDown || d == DirLeft || d == DirRight
}

// AuthMsg is the first message a client must send.
type AuthMsg struct {
	Token string `json:"token"`
}

// MoveMsg is a requested position in world pixels.
type MoveMsg struct {
	X   int       `json:"x"`
	Y   int       `json:"y"`
	Dir Direction `json:"dir"`
}

// PingMsg carries a client timestamp echoed back in PongMsg.
type PingMsg struct {
	TS int64 `json:"ts"`
}

// PongMsg answers a ping.
type PongMsg struct {
	TS int64 `json:"ts"`
}

// Participant is a person in the room as seen by others.
type Participant struct {
	ID     uuid.UUID `json:"id"`
	Name   string    `json:"name"`
	Avatar string    `json:"avatar"`
	Role   string    `json:"role"`
	X      int       `json:"x"`
	Y      int       `json:"y"`
	Dir    Direction `json:"dir"`
}

// WelcomeMsg is sent after a successful auth.
type WelcomeMsg struct {
	SelfID       uuid.UUID     `json:"self_id"`
	RoomID       uuid.UUID     `json:"room_id"`
	MapKey       string        `json:"map_key"`
	Participants []Participant `json:"participants"`
}

// LeftMsg tells that a participant left the room.
type LeftMsg struct {
	ID uuid.UUID `json:"id"`
}

// Position is one entry of a snapshot.
type Position struct {
	ID  uuid.UUID `json:"id"`
	X   int       `json:"x"`
	Y   int       `json:"y"`
	Dir Direction `json:"dir"`
}

// SnapshotMsg lists changed positions near the receiver and ids that left
// the receiver's area of interest.
type SnapshotMsg struct {
	P   []Position  `json:"p"`
	Out []uuid.UUID `json:"out,omitempty"`
}

// ErrorMsg rejects a client message. For move_rejected, X and Y carry the
// authoritative position.
type ErrorMsg struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Seq     uint64 `json:"seq,omitempty"`
	X       *int   `json:"x,omitempty"`
	Y       *int   `json:"y,omitempty"`
}

// Error codes sent in ErrorMsg.
const (
	CodeUnauthenticated = "unauthenticated"
	CodeForbidden       = "forbidden"
	CodeNotFound        = "not_found"
	CodeBadMessage      = "bad_message"
	CodeMoveRejected    = "move_rejected"
	CodeRoomFull        = "room_full"
	CodeReplaced        = "replaced"
	CodeUnavailable     = "unavailable"
)
