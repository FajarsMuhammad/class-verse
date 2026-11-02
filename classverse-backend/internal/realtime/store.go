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

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

// Store admits users to rooms from the database.
type Store struct {
	q *query.Queries
}

// NewStore wraps a pgx pool.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{q: query.New(pool)}
}

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
		RoomName: row.RoomName, BuildingKey: row.BuildingKey.String,
		UserID: userID, Name: row.DisplayName, Avatar: row.AvatarKey, Role: string(row.Role),
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

// CloseOpenSessions ends sessions left open by a previous process.
func (s *Store) CloseOpenSessions(ctx context.Context, at time.Time) error {
	if err := s.q.CloseOpenSessions(ctx, pgTime(at)); err != nil {
		return fmt.Errorf("close stale sessions: %w", err)
	}
	return nil
}

func pgTime(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t.UTC(), Valid: true} }
