package realtime

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
	return Admission{
		RoomID: row.RoomID, OrgID: row.OrgID, MapKey: row.MapKey, Capacity: int(row.Capacity),
		UserID: userID, Name: row.DisplayName, Avatar: row.AvatarKey, Role: string(row.Role),
	}, nil
}
