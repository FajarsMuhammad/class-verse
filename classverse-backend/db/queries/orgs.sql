-- name: CreateOrg :one
INSERT INTO organizations (name, slug, logo_url, created_by, created_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: CreateMembership :one
-- Returns no row when the user is already a member (ON CONFLICT DO NOTHING).
INSERT INTO memberships (org_id, user_id, role, joined_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (org_id, user_id) DO NOTHING
RETURNING *;

-- name: CreateCampusRoom :one
INSERT INTO rooms (org_id, name, map_key, capacity, created_by, created_at)
VALUES (sqlc.arg(org_id), 'Peta sekolah', 'campus', 200, sqlc.arg(created_by), sqlc.arg(created_at))
RETURNING id;

-- name: GetOrgForMember :one
-- Organization plus the caller's role and campus room; no row when the user is not a member.
SELECT o.id, o.name, o.slug, o.logo_url, o.created_at, m.role, r.id AS campus_room_id
FROM organizations o
JOIN memberships m ON m.org_id = o.id
JOIN rooms r ON r.org_id = o.id AND r.map_key = 'campus'
WHERE o.id = sqlc.arg(org_id) AND m.user_id = sqlc.arg(user_id);

-- name: GetMemberRole :one
SELECT role FROM memberships
WHERE org_id = $1 AND user_id = $2;

-- name: ListUserOrgs :many
SELECT o.id, o.name, o.slug, m.role
FROM memberships m
JOIN organizations o ON o.id = m.org_id
WHERE m.user_id = $1
ORDER BY m.joined_at, o.id;

-- name: GetUserEligibility :one
SELECT email_verified_at, birth_date FROM users
WHERE id = $1;
