// Package room manages classrooms and discussion gardens inside an
// organization (create, list, rename, schedule, archive).
package room

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/text/unicode/norm"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/org"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

const (
	minNameLen   = 3
	maxNameLen   = 60
	defaultLimit = 20
	maxLimit     = 100
	staffKey     = "ruang-guru"
)

// Map keys a room can be created with.
const (
	MapClassroom = "classroom"
	MapGarden    = "garden"
	MapLab       = "lab"
	MapReading   = "baca"
	MapLibrary   = "library" // quiet zone: chat only, no voice
	MapScience   = "science"
	MapStaff     = "staff"
)

// Domain errors.
var (
	ErrNotFound      = errors.New("room not found")
	ErrBusy          = errors.New("room busy")
	ErrInvalidCursor = errors.New("invalid cursor")
)

// ValidationError lists invalid input fields.
type ValidationError struct{ Fields map[string]string }

func (e *ValidationError) Error() string { return "invalid room fields" }

func (e *ValidationError) add(field, msg string) {
	if e.Fields == nil {
		e.Fields = map[string]string{}
	}
	e.Fields[field] = msg
}

// Authorizer checks a user's role in an organization (implemented by org.Service).
type Authorizer interface {
	Authorize(ctx context.Context, userID, orgID uuid.UUID, minRole org.Role) (org.Role, error)
}

// Buildings reports which campus building keys exist and whether they are
// staff-only (implemented from the campus map in cmd/server).
type Buildings interface {
	Building(key string) (staffOnly, ok bool)
}

// Occupancy is the live state of rooms (implemented by realtime.Hub).
type Occupancy interface {
	Occupancy(roomID uuid.UUID) (people int, live bool)
	// Lock blocks travel into roomID while fn runs, so archive cannot race
	// with an admission. fn runs only when the room is empty.
	WhileEmpty(ctx context.Context, roomID uuid.UUID, fn func() error) error
	// SetAllChat tells a running room that the "Semua" chat tab was toggled.
	SetAllChat(roomID uuid.UUID, enabled bool)
}

// Room is a room as returned by the API.
type Room struct {
	ID          uuid.UUID
	OrgID       uuid.UUID
	Name        string
	MapKey      string
	BuildingKey string
	Capacity    int
	ScheduledAt *time.Time
	CreatedAt   time.Time
	People      int
	Live        bool
	AllChat     bool
}

// CreateInput is the body of POST /api/orgs/{org_id}/rooms.
type CreateInput struct {
	Name        string
	MapKey      string
	BuildingKey string
	ScheduledAt *time.Time
}

// UpdateInput changes the name and/or schedule. SetSchedule distinguishes an
// omitted field from an explicit null (open now).
type UpdateInput struct {
	Name        *string
	SetSchedule bool
	ScheduledAt *time.Time
	AllChat     *bool
}

// Page is one page of rooms.
type Page struct {
	Items      []Room
	NextCursor string
}

// Service implements room rules.
type Service struct {
	store     *Store
	auth      Authorizer
	buildings Buildings
	live      Occupancy
	now       func() time.Time
	log       *slog.Logger
}

// NewService wires the room service.
func NewService(store *Store, auth Authorizer, buildings Buildings, live Occupancy, now func() time.Time, log *slog.Logger) *Service {
	return &Service{store: store, auth: auth, buildings: buildings, live: live, now: now, log: log}
}

// Create adds a room; only teachers and admins may.
func (s *Service) Create(ctx context.Context, userID, orgID uuid.UUID, in CreateInput) (Room, error) {
	if _, err := s.auth.Authorize(ctx, userID, orgID, org.RoleTeacher); err != nil {
		return Room{}, err
	}
	var v ValidationError
	name := validateName(&v, in.Name)
	switch in.MapKey {
	case MapClassroom, MapGarden, MapLab, MapReading, MapLibrary, MapScience, MapStaff:
	default:
		v.add("map_key", "Pilih peta ruang yang tersedia")
	}
	if in.MapKey == MapStaff && in.BuildingKey != staffKey {
		v.add("building_key", "Interior Ruang Guru hanya tersedia di gedung Ruang Guru")
	}
	if _, ok := s.buildings.Building(in.BuildingKey); !ok {
		v.add("building_key", "Gedung tidak ditemukan")
	}
	if in.ScheduledAt != nil && in.ScheduledAt.Before(s.now().Add(-time.Minute)) {
		v.add("scheduled_at", "Jadwal tidak boleh di masa lalu")
	}
	if v.Fields != nil {
		return Room{}, &v
	}
	row, err := s.store.CreateRoom(ctx, query.CreateRoomParams{
		OrgID: orgID, Name: name, MapKey: in.MapKey, BuildingKey: pgtype.Text{String: in.BuildingKey, Valid: true},
		ScheduledAt: pgTimePtr(in.ScheduledAt), CreatedBy: userID, Now: pgTime(s.now()),
	})
	if err != nil {
		return Room{}, fmt.Errorf("create room: %w", err)
	}
	s.log.InfoContext(ctx, "room created", slog.String("org_id", orgID.String()), slog.String("room_id", row.ID.String()))
	return s.withLive(toRoom(row)), nil
}

// List returns rooms visible to userID, newest first.
// search matches room names literally (case-insensitive); up to 100 characters.
func (s *Service) List(ctx context.Context, userID, orgID uuid.UUID, building string, limit int, cursor string, search ...string) (Page, error) {
	role, err := s.auth.Authorize(ctx, userID, orgID, org.RoleParticipant)
	if err != nil {
		return Page{}, err
	}
	if limit <= 0 {
		limit = defaultLimit
	}
	limit = min(limit, maxLimit)
	params := query.ListRoomsParams{OrgID: orgID, IncludeStaff: role.AtLeast(org.RoleTeacher), RowLimit: int32(limit + 1)} //nolint:gosec // bounded by maxLimit
	if building != "" {
		params.BuildingKey = pgtype.Text{String: building, Valid: true}
	}
	if len(search) > 0 {
		if q := strings.TrimSpace(search[0]); q != "" {
			if utf8.RuneCountInString(q) > 100 {
				return Page{}, &ValidationError{Fields: map[string]string{"q": "Pencarian maksimal 100 karakter"}}
			}
			params.Pattern = pgtype.Text{String: "%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(q) + "%", Valid: true}
		}
	}
	if cursor != "" {
		at, id, err := decodeCursor(cursor)
		if err != nil {
			return Page{}, ErrInvalidCursor
		}
		params.CursorCreatedAt = pgTime(at)
		params.CursorID = uuid.NullUUID{UUID: id, Valid: true}
	}
	rows, err := s.store.ListRooms(ctx, params)
	if err != nil {
		return Page{}, fmt.Errorf("list rooms: %w", err)
	}
	page := Page{Items: make([]Room, 0, min(len(rows), limit))}
	for i, r := range rows {
		if i == limit {
			last := rows[limit-1]
			page.NextCursor = encodeCursor(last.CreatedAt.Time, last.ID)
			break
		}
		page.Items = append(page.Items, s.withLive(toRoom(r)))
	}
	return page, nil
}

// Get returns one room. Missing, archived, campus, other-organization, and
// staff-only rooms (for participants) all look like ErrNotFound.
func (s *Service) Get(ctx context.Context, userID, roomID uuid.UUID) (Room, error) {
	r, _, err := s.load(ctx, userID, roomID, org.RoleParticipant)
	return r, err
}

// Update renames or reschedules a room; teachers and admins only.
func (s *Service) Update(ctx context.Context, userID, roomID uuid.UUID, in UpdateInput) (Room, error) {
	cur, _, err := s.load(ctx, userID, roomID, org.RoleTeacher)
	if err != nil {
		return Room{}, err
	}
	var v ValidationError
	params := query.UpdateRoomParams{ID: roomID, OrgID: cur.OrgID, SetSchedule: in.SetSchedule}
	if in.Name != nil {
		params.Name = pgtype.Text{String: validateName(&v, *in.Name), Valid: true}
	}
	if in.SetSchedule {
		if in.ScheduledAt != nil && in.ScheduledAt.Before(s.now().Add(-time.Minute)) {
			v.add("scheduled_at", "Jadwal tidak boleh di masa lalu")
		}
		params.ScheduledAt = pgTimePtr(in.ScheduledAt)
	}
	if in.AllChat != nil {
		params.AllChatEnabled = pgtype.Bool{Bool: *in.AllChat, Valid: true}
	}
	if v.Fields != nil {
		return Room{}, &v
	}
	row, err := s.store.UpdateRoom(ctx, params)
	if errors.Is(err, pgx.ErrNoRows) {
		return Room{}, ErrNotFound
	}
	if err != nil {
		return Room{}, fmt.Errorf("update room: %w", err)
	}
	if in.AllChat != nil {
		s.live.SetAllChat(roomID, row.AllChatEnabled)
	}
	return s.withLive(toRoom(row)), nil
}

// Archive soft-deletes an empty room; teachers and admins only.
func (s *Service) Archive(ctx context.Context, userID, roomID uuid.UUID) error {
	cur, _, err := s.load(ctx, userID, roomID, org.RoleTeacher)
	if err != nil {
		return err
	}
	return s.live.WhileEmpty(ctx, roomID, func() error {
		_, err := s.store.ArchiveRoom(ctx, query.ArchiveRoomParams{Now: pgTime(s.now()), ID: roomID, OrgID: cur.OrgID})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrBusy // an open session is still within its empty timeout
		}
		if err != nil {
			return fmt.Errorf("archive room: %w", err)
		}
		s.log.InfoContext(ctx, "room archived", slog.String("org_id", cur.OrgID.String()), slog.String("room_id", roomID.String()))
		return nil
	})
}

func (s *Service) load(ctx context.Context, userID, roomID uuid.UUID, minRole org.Role) (Room, org.Role, error) {
	row, err := s.store.GetRoomForMember(ctx, query.GetRoomForMemberParams{UserID: userID, ID: roomID})
	if errors.Is(err, pgx.ErrNoRows) {
		return Room{}, "", ErrNotFound
	}
	if err != nil {
		return Room{}, "", fmt.Errorf("get room: %w", err)
	}
	role := org.Role(row.MemberRole)
	if row.MapKey == "campus" || (row.BuildingKey.String == staffKey && !role.AtLeast(org.RoleTeacher)) {
		return Room{}, "", ErrNotFound
	}
	if !role.AtLeast(minRole) {
		return Room{}, "", org.ErrForbidden
	}
	r := s.withLive(Room{
		ID: row.ID, OrgID: row.OrgID, Name: row.Name, MapKey: row.MapKey, BuildingKey: row.BuildingKey.String,
		Capacity: int(row.Capacity), ScheduledAt: timePtr(row.ScheduledAt), CreatedAt: row.CreatedAt.Time,
		AllChat: row.AllChatEnabled,
	})
	return r, role, nil
}

func (s *Service) withLive(r Room) Room {
	r.People, r.Live = s.live.Occupancy(r.ID)
	return r
}

// validateName applies NFC, trims and collapses spaces, and checks 3–60 code points.
func validateName(v *ValidationError, name string) string {
	for _, r := range name {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			v.add("name", "Nama ruang tidak boleh mengandung karakter kontrol")
			break
		}
	}
	name = norm.NFC.String(strings.Join(strings.Fields(name), " "))
	switch n := utf8.RuneCountInString(name); {
	case n < minNameLen:
		v.add("name", "Nama ruang minimal 3 karakter")
	case n > maxNameLen:
		v.add("name", "Nama ruang maksimal 60 karakter")
	}
	return name
}

func toRoom(r query.Room) Room {
	return Room{
		ID: r.ID, OrgID: r.OrgID, Name: r.Name, MapKey: r.MapKey, BuildingKey: r.BuildingKey.String,
		Capacity: int(r.Capacity), ScheduledAt: timePtr(r.ScheduledAt), CreatedAt: r.CreatedAt.Time,
		AllChat: r.AllChatEnabled,
	}
}

func timePtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	u := t.Time.UTC()
	return &u
}

func pgTime(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t.UTC(), Valid: true} }

func pgTimePtr(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgTime(*t)
}

// Cursor is opaque to clients: base64url("<unix nanos>|<uuid>").
func encodeCursor(at time.Time, id uuid.UUID) string {
	return base64.RawURLEncoding.EncodeToString(fmt.Appendf(nil, "%d|%s", at.UnixNano(), id))
}

func decodeCursor(c string) (time.Time, uuid.UUID, error) {
	b, err := base64.RawURLEncoding.DecodeString(c)
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}
	ts, rawID, ok := strings.Cut(string(b), "|")
	if !ok {
		return time.Time{}, uuid.Nil, ErrInvalidCursor
	}
	var nanos int64
	if _, err := fmt.Sscan(ts, &nanos); err != nil {
		return time.Time{}, uuid.Nil, err
	}
	id, err := uuid.Parse(rawID)
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}
	return time.Unix(0, nanos).UTC(), id, nil
}
