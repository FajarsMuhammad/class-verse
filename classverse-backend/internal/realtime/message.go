package realtime

import (
	"encoding/json"
	"time"

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
	TypeCmd    = "cmd"
	TypeAuth   = "auth"
	TypeMove   = "move"
	TypePing   = "ping"
	TypeTravel = "travel"
	TypeChat   = "chat" // both directions
)

// Server → client message types.
const (
	TypeEvent          = "event"
	TypeWelcome        = "welcome"
	TypeSnapshot       = "snapshot"
	TypeJoined         = "joined"
	TypeLeft           = "left"
	TypePong           = "pong"
	TypeError          = "error"
	TypeProfileUpdated = "profile_updated"
	TypeTraveled       = "traveled"
	TypeBuildings      = "buildings"
	TypeZone           = "zone"
	TypeRoomState      = "room_state"
)

// CommandMsg requests a teacher action in the current room.
type CommandMsg struct {
	Name   string    `json:"name"`
	Target uuid.UUID `json:"target,omitempty"`
}

// ModerationEvent tells staff the result of kick/allow and the open-session
// ban list; the kicked participant only receives an error before close.
type ModerationEvent struct {
	Name   string       `json:"name"`
	By     uuid.UUID    `json:"by"`
	Target uuid.UUID    `json:"target"`
	Banned []BannedUser `json:"banned,omitempty"`
	Muted  []uuid.UUID  `json:"muted"`
}

// BannedUser is a participant kicked from the open session.
type BannedUser struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// GatherEvent resets authoritative positions and client prediction.
type GatherEvent struct {
	Name         string        `json:"name"`
	By           uuid.UUID     `json:"by"`
	Participants []Participant `json:"participants"`
}

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

// WelcomeMsg is sent after a successful auth, and as "traveled" after a
// successful travel. RoomName and BuildingKey are empty on the campus.
type WelcomeMsg struct {
	SelfID       uuid.UUID     `json:"self_id"`
	RoomID       uuid.UUID     `json:"room_id"`
	MapKey       string        `json:"map_key"`
	RoomName     string        `json:"room_name,omitempty"`
	BuildingKey  string        `json:"building_key,omitempty"`
	Participants []Participant `json:"participants"`
	// AllChatEnabled tells whether the "Semua" tab may send (always true on the campus).
	AllChatEnabled bool `json:"all_chat_enabled"`
	Locked         bool `json:"locked"`
	Muted          bool `json:"muted"`
}

// Chat scopes. Broadcast is restricted to teachers and admins.
const (
	ScopeNear      = "near"
	ScopeZone      = "zone"
	ScopeRoom      = "room"
	ScopeBroadcast = "broadcast"
)

// ChatInMsg is a chat message sent by a client.
type ChatInMsg struct {
	Text  string `json:"text"`
	Scope string `json:"scope"`
}

// ChatSender identifies who wrote a chat message.
type ChatSender struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	Role string    `json:"role"`
}

// ChatOutMsg is a filtered chat message delivered to its recipients. ID is
// set only when the message was stored.
type ChatOutMsg struct {
	ID    int64      `json:"id,omitempty"`
	From  ChatSender `json:"from"`
	Text  string     `json:"text"`
	Scope string     `json:"scope"`
	Zone  string     `json:"zone,omitempty"`
	At    time.Time  `json:"at"`
}

// ZoneMsg tells a client which zone its avatar is in; an empty Key means none.
type ZoneMsg struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

// RoomStateMsg carries room settings that can change during a session.
type RoomStateMsg struct {
	AllChatEnabled bool `json:"all_chat_enabled"`
	// Locked rooms reject new participants (slice 7).
	Locked bool `json:"locked"`
	// Muted is personal: true when this receiver may not chat.
	Muted bool `json:"muted"`
}

// TravelMsg asks to move to another room of the same organization.
type TravelMsg struct {
	RoomID uuid.UUID `json:"room_id"`
}

// BuildingCount is the live state of one campus building.
type BuildingCount struct {
	BuildingKey string `json:"building_key"`
	People      int    `json:"people"`
	LiveRooms   int    `json:"live_rooms"`
}

// BuildingsMsg is sent every 5 s on the campus only.
type BuildingsMsg struct {
	Items []BuildingCount `json:"items"`
}

// ProfileUpdatedMsg changes identity without altering presence or position.
type ProfileUpdatedMsg struct {
	ID     uuid.UUID `json:"id"`
	Name   string    `json:"name"`
	Avatar string    `json:"avatar"`
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
	CodeUnauthenticated  = "unauthenticated"
	CodeForbidden        = "forbidden"
	CodeKicked           = "kicked"
	CodeMuted            = "muted"
	CodeRoomLocked       = "room_locked"
	CodeNotFoundTarget   = "target_not_found"
	CodeNotFound         = "not_found"
	CodeBadMessage       = "bad_message"
	CodeMoveRejected     = "move_rejected"
	CodeRoomFull         = "room_full"
	CodeReplaced         = "replaced"
	CodeUnavailable      = "unavailable"
	CodeNotAtDoor        = "not_at_door"
	CodeScheduled        = "scheduled"
	CodeTravelInProgress = "travel_in_progress"
	CodeRateLimited      = "rate_limited"
	CodeNotInZone        = "not_in_zone"
	CodeAllChatDisabled  = "all_chat_disabled"
)
