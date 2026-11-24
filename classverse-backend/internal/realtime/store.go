package realtime

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

// Store admits users to rooms from the database.
type Store struct {
	q   *query.Queries
	now func() time.Time
}

// NewStore wraps a pgx pool.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{q: query.New(pool), now: time.Now}
}

// SetClock injects the admission clock before serving requests.
func (s *Store) SetClock(now func() time.Time) { s.now = now }

// Admit implements Admitter: the room must exist, not be archived, and the
// user must be a member of its organization.
func (s *Store) Admit(ctx context.Context, roomID, userID uuid.UUID) (Admission, error) {
	row, err := s.q.GetRoomAdmission(ctx, query.GetRoomAdmissionParams{RoomID: roomID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return Admission{}, ErrNoAccess
	}
	if err != nil {
		return Admission{}, fmt.Errorf("get room admission: %w", err)
	}
	adm := Admission{
		RoomID: row.RoomID, OrgID: row.OrgID, MapKey: row.MapKey, Capacity: int(row.Capacity),
		RoomName: row.RoomName, BuildingKey: row.BuildingKey.String, AllChat: row.AllChatEnabled,
		UserID: userID, Name: row.DisplayName, Avatar: row.AvatarKey, Role: string(row.Role),
		Banned: row.Banned,
	}
	if row.ScheduledAt.Valid {
		t := row.ScheduledAt.Time.UTC()
		adm.ScheduledAt = &t
	}
	if adm.MapKey != campusKey {
		adm.Pin, err = s.learningPin(ctx, roomID)
		if err != nil {
			return Admission{}, fmt.Errorf("load learning pin: %w", err)
		}
	}
	return adm, nil
}

// AdmitGuest admits an approved guest to the one room of their link (G1).
// Any other room, or an inactive link, is ErrNoAccess.
func (s *Store) AdmitGuest(ctx context.Context, roomID, guestID uuid.UUID) (Admission, error) {
	row, err := s.q.GetGuestAdmission(ctx, query.GetGuestAdmissionParams{UserID: guestID, RoomID: roomID, Now: pgTime(s.now())})
	if errors.Is(err, pgx.ErrNoRows) {
		return Admission{}, ErrNoAccess
	}
	if err != nil {
		return Admission{}, fmt.Errorf("get guest admission: %w", err)
	}
	adm := Admission{
		RoomID: row.RoomID, OrgID: row.OrgID, MapKey: row.MapKey, Capacity: int(row.Capacity),
		RoomName: row.RoomName, BuildingKey: row.BuildingKey.String, AllChat: row.AllChatEnabled,
		GuestExpiresAt: row.ExpiresAt.Time, UserID: guestID, Name: row.DisplayName, Avatar: row.AvatarKey, Role: RoleGuest, Banned: row.Banned,
	}
	if row.ScheduledAt.Valid {
		t := row.ScheduledAt.Time.UTC()
		adm.ScheduledAt = &t
	}
	return adm, nil
}

// OpenSession starts (or reuses) the open session of roomID.
func (s *Store) OpenSession(ctx context.Context, roomID uuid.UUID, at time.Time) (uuid.UUID, error) {
	id, err := s.q.OpenRoomSession(ctx, query.OpenRoomSessionParams{RoomID: roomID, Now: pgTime(at)})
	if err != nil {
		return uuid.Nil, fmt.Errorf("open session: %w", err)
	}
	return id, nil
}

// CloseSession ends the open session of roomID and any open attendance.
func (s *Store) CloseSession(ctx context.Context, roomID uuid.UUID, at time.Time) error {
	if err := s.q.CloseRoomSession(ctx, query.CloseRoomSessionParams{RoomID: roomID, Now: pgTime(at)}); err != nil {
		return fmt.Errorf("close session: %w", err)
	}
	return nil
}

// Attend records that userID entered the session.
func (s *Store) Attend(ctx context.Context, sessionID, userID uuid.UUID, at time.Time) error {
	if err := s.q.StartAttendance(ctx, query.StartAttendanceParams{SessionID: sessionID, UserID: userID, Now: pgTime(at)}); err != nil {
		return fmt.Errorf("start attendance: %w", err)
	}
	return nil
}

// Leave records that userID left the session.
func (s *Store) Leave(ctx context.Context, sessionID, userID uuid.UUID, at time.Time) error {
	if err := s.q.EndAttendance(ctx, query.EndAttendanceParams{SessionID: sessionID, UserID: userID, Now: pgTime(at)}); err != nil {
		return fmt.Errorf("end attendance: %w", err)
	}
	return nil
}

// SaveChat stores msg in the open session of roomID. ok is false when the
// room has no open session (nothing is stored).
func (s *Store) SaveChat(ctx context.Context, roomID uuid.UUID, msg ChatOutMsg) (int64, bool, error) {
	zone := pgtype.Text{String: msg.Zone, Valid: msg.Zone != ""}
	id, err := s.q.InsertChatMessage(ctx, query.InsertChatMessageParams{
		SenderID: msg.From.ID, Scope: query.ChatScope(msg.Scope), ZoneKey: zone, Body: msg.Text,
		Now: pgTime(msg.At), RoomID: roomID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("save chat: %w", err)
	}
	return id, true, nil
}

// PurgeChat deletes chat messages created before cutoff (30-day retention).
func (s *Store) PurgeChat(ctx context.Context, cutoff time.Time) (int64, error) {
	n, err := s.q.PurgeChatMessages(ctx, pgTime(cutoff))
	if err != nil {
		return 0, fmt.Errorf("purge chat: %w", err)
	}
	return n, nil
}

// CloseOpenSessions ends sessions left open by a previous process.
func (s *Store) CloseOpenSessions(ctx context.Context, at time.Time) error {
	if err := s.q.CloseOpenSessions(ctx, pgTime(at)); err != nil {
		return fmt.Errorf("close stale sessions: %w", err)
	}
	return nil
}

func pgTime(t time.Time) pgtype.Timestamptz { return db.Time(t) }

// Kick bans target from the open session of roomID and audits it atomically.
// ok is false when nothing changed: no open session, or the actor/target
// roles or organization do not allow it.
func (s *Store) Kick(ctx context.Context, roomID, actorID, targetID uuid.UUID, at time.Time) (bool, error) {
	n, err := s.q.KickFromOpenSession(ctx, query.KickFromOpenSessionParams{RoomID: roomID, ActorID: actorID, TargetID: targetID, Now: pgTime(at)})
	if err != nil {
		return false, fmt.Errorf("kick: %w", err)
	}
	return n == 1, nil
}

// Allow revokes target's ban from the open session of roomID, with audit.
func (s *Store) Allow(ctx context.Context, roomID, actorID, targetID uuid.UUID, at time.Time) (bool, error) {
	n, err := s.q.AllowInOpenSession(ctx, query.AllowInOpenSessionParams{RoomID: roomID, ActorID: actorID, TargetID: targetID, Now: pgTime(at)})
	if err != nil {
		return false, fmt.Errorf("allow: %w", err)
	}
	return n == 1, nil
}

// Banned lists users banned from the open session of roomID.
func (s *Store) Banned(ctx context.Context, roomID uuid.UUID) ([]BannedUser, error) {
	rows, err := s.q.ListOpenSessionBans(ctx, roomID)
	if err != nil {
		return nil, fmt.Errorf("list bans: %w", err)
	}
	out := make([]BannedUser, 0, len(rows))
	for _, r := range rows {
		out = append(out, BannedUser{ID: r.ID, Name: r.DisplayName})
	}
	return out, nil
}

// RecordAction audits mute/unmute/lock/unlock for the open session of roomID.
// target is uuid.Nil for lock/unlock. ok is false when the roles,
// organization or session state do not allow it.
func (s *Store) RecordAction(ctx context.Context, roomID, actorID, targetID uuid.UUID, action string, at time.Time) (bool, error) {
	n, err := s.q.RecordRoomAction(ctx, query.RecordRoomActionParams{
		RoomID: roomID, ActorID: actorID, TargetID: uuid.NullUUID{UUID: targetID, Valid: targetID != uuid.Nil},
		Action: query.ModAction(action), Now: pgTime(at),
	})
	if err != nil {
		return false, fmt.Errorf("record %s: %w", action, err)
	}
	return n == 1, nil
}

var (
	_ SessionRecorder = (*Store)(nil)
	_ Moderator       = (*Store)(nil)
)
