package org

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// Role is a member's role in one organization.
type Role string

// Roles, from most to least privileged.
const (
	RoleAdmin       Role = "admin"
	RoleTeacher     Role = "teacher"
	RoleParticipant Role = "participant"
)

func (r Role) rank() int {
	switch r {
	case RoleAdmin:
		return 3
	case RoleTeacher:
		return 2
	case RoleParticipant:
		return 1
	default:
		return 0
	}
}

// AtLeast reports whether r has at least the privileges of min.
func (r Role) AtLeast(minRole Role) bool {
	return r.rank() >= minRole.rank() && minRole.rank() > 0
}

// AdultAge is the minimum age to create an organization (spec, Keamanan 7).
const AdultAge = 18

// Age returns the full years between birth and the UTC date of now.
func Age(birth, now time.Time) int {
	now = now.UTC()
	years := now.Year() - birth.Year()
	if now.Month() < birth.Month() || (now.Month() == birth.Month() && now.Day() < birth.Day()) {
		years--
	}
	return years
}

func pgTime(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t.UTC(), Valid: true}
}
