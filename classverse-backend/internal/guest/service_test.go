package guest

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/auth"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/org"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/dbtest"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/realtime"
)

func TestServiceGuestLifecycle(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.New(t)
	q := query.New(pool)
	now := time.Now().UTC()
	clock := func() time.Time { return now }
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	host, err := q.CreateTestUser(ctx, query.CreateTestUserParams{Email: "host@example.test", DisplayName: "Host"})
	if err != nil {
		t.Fatal(err)
	}
	school, err := q.CreateOrg(ctx, query.CreateOrgParams{Name: "School", Slug: "guest", CreatedBy: host.ID, CreatedAt: ts(now)})
	if err != nil {
		t.Fatal(err)
	}
	_, err = q.CreateMembership(ctx, query.CreateMembershipParams{OrgID: school.ID, UserID: host.ID, Role: query.OrgRoleTeacher, JoinedAt: ts(now)})
	if err != nil {
		t.Fatal(err)
	}
	room, err := q.CreateRoom(ctx, query.CreateRoomParams{OrgID: school.ID, Name: "Meeting", MapKey: "classroom", CreatedBy: host.ID, Now: ts(now)})
	if err != nil {
		t.Fatal(err)
	}
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	tokens := auth.NewTokens(key, clock)
	svc := NewService(pool, org.NewService(org.NewStore(pool), clock, log), tokens, clock, log)
	rt := realtime.NewStore(pool)
	rt.SetClock(clock)
	staffRoom, err := q.CreateRoom(ctx, query.CreateRoomParams{OrgID: school.ID, Name: "Staff", MapKey: "staff", BuildingKey: pgtype.Text{String: "ruang-guru", Valid: true}, CreatedBy: host.ID, Now: ts(now)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateLink(ctx, host.ID, staffRoom.ID, nil, nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("staff room guest link: %v", err)
	}
	limit := 1
	link, err := svc.CreateLink(ctx, host.ID, room.ID, nil, &limit)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateLink(ctx, uuid.New(), room.ID, nil, nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-org: %v", err)
	}
	requestKey := uuid.New()
	results := make(chan Status, 8)
	errs := make(chan error, 8)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, e := svc.Request(ctx, link.Code, "Guest One", requestKey)
			results <- s
			errs <- e
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var id uuid.UUID
	for s := range results {
		got, e := tokens.VerifyGuest(s.Token)
		if e != nil {
			t.Fatal(e)
		}
		if id != uuid.Nil && id != got {
			t.Fatal("retry created another identity")
		}
		id = got
		if s.Status != "pending" {
			t.Fatal(s.Status)
		}
	}
	var count int
	if err := pool.QueryRow(ctx, "SELECT used_count FROM guest_links WHERE id=$1", link.ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("quota %d %v", count, err)
	}
	var email pgtype.Text
	if err := pool.QueryRow(ctx, "SELECT email FROM users WHERE id=$1", id).Scan(&email); err != nil || email.Valid {
		t.Fatal("guest has email", err)
	}
	if _, err := q.GetUserByID(ctx, id); err == nil {
		t.Fatal("account lookup accepted guest")
	}
	if _, err := q.CreateMembership(ctx, query.CreateMembershipParams{OrgID: school.ID, UserID: id, Role: query.OrgRoleParticipant, JoinedAt: ts(now)}); err == nil {
		t.Fatal("guest received membership")
	}
	if err := q.CreateRefreshToken(ctx, query.CreateRefreshTokenParams{UserID: id, FamilyID: uuid.New(), TokenHash: []byte("test"), ExpiresAt: ts(now.Add(time.Hour)), CreatedAt: ts(now)}); err == nil {
		t.Fatal("guest received refresh token")
	}
	if _, err := svc.Request(ctx, link.Code, "Other Guest", uuid.New()); !errors.Is(err, ErrExpired) {
		t.Fatalf("quota bypass: %v", err)
	}
	if _, err := rt.AdmitGuest(ctx, room.ID, id); !errors.Is(err, realtime.ErrNoAccess) {
		t.Fatal("pending admitted", err)
	}
	requests, err := svc.Requests(ctx, host.ID, room.ID)
	if err != nil || len(requests) != 1 {
		t.Fatal(requests, err)
	}
	decisions := make(chan error, 2)
	for range 2 {
		go func() { _, e := svc.Decide(ctx, host.ID, room.ID, requests[0].ID, true); decisions <- e }()
	}
	wins := 0
	for range 2 {
		if e := <-decisions; e == nil {
			wins++
		} else if !errors.Is(e, ErrNotFound) {
			t.Fatal(e)
		}
	}
	if wins != 1 {
		t.Fatal("concurrent decisions", wins)
	}
	if _, err := rt.AdmitGuest(ctx, room.ID, id); err != nil {
		t.Fatal(err)
	}
	if _, err := rt.AdmitGuest(ctx, uuid.New(), id); !errors.Is(err, realtime.ErrNoAccess) {
		t.Fatal("other room admitted", err)
	}
	if _, err := rt.Admit(ctx, room.ID, id); !errors.Is(err, realtime.ErrNoAccess) {
		t.Fatal("account admission accepted guest", err)
	}
	now = now.Add(10 * time.Minute)
	status, err := svc.Status(ctx, id)
	if err != nil || status.Status != "approved" {
		t.Fatal(status.Status, err)
	}
	if _, err := tokens.VerifyGuest(status.Token); err != nil {
		t.Fatal("renewal", err)
	}
	if err := svc.RevokeLink(ctx, host.ID, room.ID, link.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Status(ctx, id); !errors.Is(err, ErrExpired) {
		t.Fatal("revoked status", err)
	}
	if _, err := rt.AdmitGuest(ctx, room.ID, id); !errors.Is(err, realtime.ErrNoAccess) {
		t.Fatal("revoked admitted", err)
	}
	if _, err := svc.Request(ctx, link.Code, "Guest One", requestKey); !errors.Is(err, ErrExpired) {
		t.Fatal("revoked retry", err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM guest_decisions WHERE link_id=$1", link.ID).Scan(&count); err != nil || count != 2 {
		t.Fatalf("audit %d %v", count, err)
	}

	t.Run("approval races revocation", func(t *testing.T) {
		for range 6 {
			l, err := svc.CreateLink(ctx, host.ID, room.ID, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			session, err := svc.Request(ctx, l.Code, "Racing Guest", uuid.New())
			if err != nil {
				t.Fatal(err)
			}
			guestID, err := tokens.VerifyGuest(session.Token)
			if err != nil {
				t.Fatal(err)
			}
			requests, err := svc.Requests(ctx, host.ID, room.ID)
			if err != nil || len(requests) != 1 {
				t.Fatalf("requests %v: %v", requests, err)
			}
			start := make(chan struct{})
			results := make(chan error, 2)
			go func() { <-start; _, err := svc.Decide(ctx, host.ID, room.ID, requests[0].ID, true); results <- err }()
			go func() { <-start; results <- svc.RevokeLink(ctx, host.ID, room.ID, l.ID) }()
			close(start)
			for range 2 {
				if err := <-results; err != nil && !errors.Is(err, ErrNotFound) {
					t.Fatal(err)
				}
			}
			if _, err := rt.AdmitGuest(ctx, room.ID, guestID); !errors.Is(err, realtime.ErrNoAccess) {
				t.Fatal("revoked grant admitted", err)
			}
			if _, err := svc.Status(ctx, guestID); !errors.Is(err, ErrExpired) {
				t.Fatal("revoked grant renewed", err)
			}
		}
	})

	for _, decision := range []string{"denied", "expired"} {
		t.Run(decision, func(t *testing.T) {
			l, e := svc.CreateLink(ctx, host.ID, room.ID, nil, nil)
			if e != nil {
				t.Fatal(e)
			}
			key := uuid.New()
			s, e := svc.Request(ctx, l.Code, "Next Guest", key)
			if e != nil {
				t.Fatal(e)
			}
			uid, e := tokens.VerifyGuest(s.Token)
			if e != nil {
				t.Fatal(e)
			}
			if decision == "denied" {
				rs, e := svc.Requests(ctx, host.ID, room.ID)
				if e != nil || len(rs) != 1 {
					t.Fatal(rs, e)
				}
				if _, e = svc.Decide(ctx, host.ID, room.ID, rs[0].ID, false); e != nil {
					t.Fatal(e)
				}
				s, e = svc.Request(ctx, l.Code, "Changed Name", key)
				if e != nil || s.Status != "denied" || s.Name != "Next Guest" {
					t.Fatal(s.Status, s.Name, e)
				}
			} else {
				now = now.Add(3 * time.Hour)
				if _, e = svc.Status(ctx, uid); !errors.Is(e, ErrExpired) {
					t.Fatal(e)
				}
			}
			if _, e = rt.AdmitGuest(ctx, room.ID, uid); !errors.Is(e, realtime.ErrNoAccess) {
				t.Fatal("inactive admitted", e)
			}
		})
	}
}
