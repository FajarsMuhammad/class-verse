package main

import (
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/realtime"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/room"
)

func TestRoomOccupancyMapsBusy(t *testing.T) {
	maps, err := realtime.LoadMaps("campus")
	if err != nil {
		t.Fatal(err)
	}
	hub := realtime.NewHub(maps, nil, time.Now, slog.New(slog.NewTextHandler(io.Discard, nil)))
	t.Cleanup(func() { _ = hub.Shutdown(t.Context()) })
	occ := roomOccupancy{hub}
	id := uuid.New()
	inner := occ.WhileEmpty(t.Context(), id, func() error {
		return occ.WhileEmpty(t.Context(), id, func() error { return nil })
	})
	if !errors.Is(inner, room.ErrBusy) {
		t.Fatalf("concurrent archive error = %v, want room.ErrBusy", inner)
	}
}
