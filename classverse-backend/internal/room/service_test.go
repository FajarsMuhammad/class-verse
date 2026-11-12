package room

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/org"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/dbtest"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

type testBuildings struct{}

func (testBuildings) Building(key string) (bool, bool) {
	return key == staffKey, key == staffKey || key == "kelas-a"
}

type emptyRooms struct{}

func (emptyRooms) Occupancy(uuid.UUID) (int, bool)                                  { return 0, false }
func (emptyRooms) SetAllChat(uuid.UUID, bool)                                       {}
func (emptyRooms) WhileEmpty(_ context.Context, _ uuid.UUID, fn func() error) error { return fn() }

func TestValidateName(t *testing.T) {
	for _, tt := range []struct {
		name, input, want string
		invalid           bool
	}{
		{"normalize", "  Cafe\u0301   pagi ", "Café pagi", false},
		{"short", "ab", "ab", true},
		{"limit", strings.Repeat("界", 60), strings.Repeat("界", 60), false},
		{"long", strings.Repeat("界", 61), strings.Repeat("界", 61), true},
		{"control", "Kelas\nbaru", "Kelas baru", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var v ValidationError
			got := validateName(&v, tt.input)
			if got != tt.want || (v.Fields != nil) != tt.invalid {
				t.Fatalf("name=%q fields=%v", got, v.Fields)
			}
		})
	}
}

func TestRoomLifecycle(t *testing.T) {
	pool := dbtest.New(t)
	ctx := t.Context()
	now := time.Date(2026, 3, 1, 8, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	q := query.New(pool)
	user, err := q.CreateTestUser(ctx, query.CreateTestUserParams{Email: "rooms@example.test", DisplayName: "Teacher", BirthDate: pgtype.Date{Time: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true}})
	if err != nil {
		t.Fatal(err)
	}
	authorizer := org.NewService(org.NewStore(pool), clock, log)
	school, err := authorizer.Create(ctx, user.ID, org.CreateInput{Name: "Test School", Slug: "test-school"})
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(NewStore(pool), authorizer, testBuildings{}, emptyRooms{}, clock, log)
	for _, tt := range []struct {
		name  string
		input CreateInput
	}{
		{"unknown building", CreateInput{Name: "Biologi", MapKey: MapClassroom, BuildingKey: "fake"}},
		{"campus map", CreateInput{Name: "Biologi", MapKey: "campus", BuildingKey: "kelas-a"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Create(ctx, user.ID, school.ID, tt.input)
			var validation *ValidationError
			if !errors.As(err, &validation) {
				t.Fatalf("error=%v", err)
			}
		})
	}
	scheduled := now.Add(time.Hour)
	created, err := svc.Create(ctx, user.ID, school.ID, CreateInput{Name: " Biologi ", MapKey: MapClassroom, BuildingKey: "kelas-a", ScheduledAt: &scheduled})
	if err != nil {
		t.Fatal(err)
	}
	if created.Capacity != 40 || created.Name != "Biologi" {
		t.Fatalf("room=%+v", created)
	}
	if lib, err := svc.Create(ctx, user.ID, school.ID, CreateInput{Name: "Literasi", MapKey: MapLibrary, BuildingKey: "kelas-a"}); err != nil || lib.MapKey != MapLibrary {
		t.Fatalf("library room=%+v err=%v", lib, err)
	}
	renamed := "Kimia"
	updated, err := svc.Update(ctx, user.ID, created.ID, UpdateInput{Name: &renamed})
	if err != nil || updated.ScheduledAt == nil || !updated.ScheduledAt.Equal(scheduled) {
		t.Fatalf("partial update=%+v err=%v", updated, err)
	}
	opened, err := svc.Update(ctx, user.ID, created.ID, UpdateInput{SetSchedule: true})
	if err != nil || opened.ScheduledAt != nil {
		t.Fatalf("open now=%+v err=%v", opened, err)
	}
	page, err := svc.List(ctx, user.ID, school.ID, "kelas-a", 1, "")
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != created.ID {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	if _, err := svc.Get(ctx, uuid.New(), created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("outsider error=%v", err)
	}
	session, err := q.OpenRoomSession(ctx, query.OpenRoomSessionParams{RoomID: created.ID, Now: pgTime(now)})
	if err != nil {
		t.Fatal(err)
	}
	again, err := q.OpenRoomSession(ctx, query.OpenRoomSessionParams{RoomID: created.ID, Now: pgTime(now)})
	if err != nil || session != again {
		t.Fatalf("duplicate session: %v", err)
	}
	if err := q.StartAttendance(ctx, query.StartAttendanceParams{SessionID: session, UserID: user.ID, Now: pgTime(now)}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Archive(ctx, user.ID, created.ID); !errors.Is(err, ErrBusy) {
		t.Fatalf("archive open session: %v", err)
	}
	if err := q.CloseRoomSession(ctx, query.CloseRoomSessionParams{RoomID: created.ID, Now: pgTime(now.Add(time.Minute))}); err != nil {
		t.Fatal(err)
	}
	var openAttendances int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM session_attendances WHERE session_id=$1 AND left_at IS NULL", session).Scan(&openAttendances); err != nil || openAttendances != 0 {
		t.Fatalf("attendance not closed: %d err=%v", openAttendances, err)
	}
	if err := svc.Archive(ctx, user.ID, created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Get(ctx, user.ID, created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("archived error=%v", err)
	}
}
