package org

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/dbtest"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

var testNow = time.Date(2026, 3, 1, 8, 0, 0, 0, time.UTC)

func TestAge(t *testing.T) {
	tests := []struct {
		birth string
		want  int
	}{
		{"2008-03-01", 18},
		{"2008-03-02", 17},
		{"2008-02-29", 18},
		{"2010-12-31", 15},
	}
	for _, tt := range tests {
		b, _ := time.Parse(time.DateOnly, tt.birth)
		if got := Age(b, testNow); got != tt.want {
			t.Errorf("Age(%s) = %d, want %d", tt.birth, got, tt.want)
		}
	}
}

func TestRoleAtLeast(t *testing.T) {
	if !RoleAdmin.AtLeast(RoleTeacher) || RoleParticipant.AtLeast(RoleTeacher) || !RoleTeacher.AtLeast(RoleTeacher) {
		t.Error("role ordering wrong")
	}
	if Role("bogus").AtLeast(RoleParticipant) {
		t.Error("unknown role must not pass")
	}
}

func newUser(t *testing.T, q *query.Queries, email, birth string) uuid.UUID {
	t.Helper()
	var bd pgtype.Date
	if birth != "" {
		d, _ := time.Parse(time.DateOnly, birth)
		bd = pgtype.Date{Time: d, Valid: true}
	}
	u, err := q.CreateTestUser(context.Background(), query.CreateTestUserParams{Email: email, DisplayName: "User", BirthDate: bd})
	if err != nil {
		t.Fatal(err)
	}
	return u.ID
}

func TestCreateAndAuthorize(t *testing.T) {
	pool := dbtest.New(t)
	store := NewStore(pool)
	svc := NewService(store, func() time.Time { return testNow }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx := context.Background()

	adult := newUser(t, store.Queries, "adult@b.co", "1990-01-01")
	minor := newUser(t, store.Queries, "minor@b.co", "2009-01-01")
	noBirth := newUser(t, store.Queries, "nobirth@b.co", "")
	stranger := newUser(t, store.Queries, "x@b.co", "1990-01-01")

	tests := []struct {
		name    string
		user    uuid.UUID
		in      CreateInput
		want    error
		invalid bool
	}{
		{"minor rejected", minor, CreateInput{Name: "SMA Satu", Slug: "sma-satu"}, ErrNotAdult, false},
		{"no birth date", noBirth, CreateInput{Name: "SMA Satu", Slug: "sma-satu"}, ErrNotEligible, false},
		{"bad slug", adult, CreateInput{Name: "SMA Satu", Slug: "SMA Satu"}, nil, true},
		{"http logo", adult, CreateInput{Name: "SMA Satu", Slug: "sma-satu", LogoURL: ptr("http://x.id/a.png")}, nil, true},
		{"ok", adult, CreateInput{Name: "SMA Satu", Slug: "sma-satu"}, nil, false},
		{"slug taken", adult, CreateInput{Name: "SMA Satu Lagi", Slug: "sma-satu"}, ErrSlugTaken, false},
	}
	var orgID uuid.UUID
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o, err := svc.Create(ctx, tt.user, tt.in)
			var verr *ValidationError
			if tt.invalid {
				if !errors.As(err, &verr) {
					t.Fatalf("err = %v, want ValidationError", err)
				}
			} else if !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
			if err == nil {
				orgID = o.ID
				if o.Role != RoleAdmin {
					t.Errorf("role = %s", o.Role)
				}
			}
		})
	}
	if orgID == uuid.Nil {
		t.Fatal("no org created")
	}

	var campus int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM rooms WHERE org_id=$1 AND map_key='campus' AND capacity=200", orgID).Scan(&campus); err != nil || campus != 1 {
		t.Errorf("campus rooms = %d, %v", campus, err)
	}
	var orgs int
	_ = pool.QueryRow(ctx, "SELECT count(*) FROM organizations").Scan(&orgs)
	if orgs != 1 {
		t.Errorf("organizations = %d, want 1 (failed creates must roll back)", orgs)
	}

	if _, err := svc.Get(ctx, stranger, orgID); !errors.Is(err, ErrNotMember) {
		t.Errorf("Get(stranger) = %v, want ErrNotMember", err)
	}
	if _, err := svc.Authorize(ctx, adult, orgID, RoleAdmin); err != nil {
		t.Errorf("Authorize(admin) = %v", err)
	}
	list, err := svc.ListForUser(ctx, adult)
	if err != nil || len(list) != 1 || list[0].Slug != "sma-satu" {
		t.Errorf("ListForUser = %+v, %v", list, err)
	}
}

func ptr[T any](v T) *T { return &v }
