-- +goose Up
-- G1 meeting guests (ADR-012). A guest is a users row without email login,
-- password, Google identity, membership, or refresh token; every realtime
-- table keeps referencing users(id). Access comes only from an approved
-- guest_requests row for one room.
ALTER TABLE users ALTER COLUMN email DROP NOT NULL;
ALTER TABLE users ADD COLUMN is_guest boolean NOT NULL DEFAULT false;
ALTER TABLE users ADD CONSTRAINT users_guest_no_login CHECK (
    (NOT is_guest AND email IS NOT NULL) OR
    (is_guest AND email IS NULL AND password_hash IS NULL AND google_sub IS NULL AND email_verified_at IS NULL AND birth_date IS NULL)
);

ALTER TABLE users ADD CONSTRAINT users_id_kind_uq UNIQUE (id, is_guest);
ALTER TABLE rooms ADD CONSTRAINT rooms_id_org_uq UNIQUE (id, org_id);

CREATE TABLE guest_links (
    id          uuid PRIMARY KEY,
    org_id      uuid NOT NULL,
    room_id     uuid NOT NULL,
    code        text NOT NULL UNIQUE,
    max_guests  int  NOT NULL CHECK (max_guests BETWEEN 1 AND 40),
    used_count  int  NOT NULL DEFAULT 0 CHECK (used_count >= 0),
    expires_at  timestamptz NOT NULL,
    revoked_at  timestamptz,
    created_by  uuid NOT NULL REFERENCES users(id),
    created_at  timestamptz NOT NULL,
    FOREIGN KEY (room_id, org_id) REFERENCES rooms(id, org_id) ON DELETE CASCADE,
    FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE,
    CHECK (expires_at > created_at AND expires_at <= created_at + interval '24 hours'),
    CHECK (used_count <= max_guests)
);
ALTER TABLE guest_links ADD CONSTRAINT guest_links_id_room_uq UNIQUE (id, room_id);
CREATE INDEX guest_links_room_idx ON guest_links (room_id, created_at DESC);

CREATE TYPE guest_status AS ENUM ('pending', 'approved', 'denied');

CREATE TABLE guest_requests (
    id          uuid PRIMARY KEY,
    link_id     uuid NOT NULL REFERENCES guest_links(id) ON DELETE CASCADE,
    request_key bytea NOT NULL UNIQUE CHECK (octet_length(request_key) = 32),
    user_id     uuid NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    is_guest    boolean NOT NULL DEFAULT true CHECK (is_guest),
    FOREIGN KEY (user_id, is_guest) REFERENCES users(id, is_guest),
    status      guest_status NOT NULL DEFAULT 'pending',
    decided_by  uuid REFERENCES users(id),
    decided_at  timestamptz,
    created_at  timestamptz NOT NULL
);
CREATE INDEX guest_requests_link_idx ON guest_requests (link_id, created_at);

-- Guard account-only credential and membership writers, including older binaries.
-- +goose StatementBegin
CREATE FUNCTION reject_guest_account_access() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF EXISTS (SELECT 1 FROM users WHERE id = NEW.user_id AND is_guest) THEN
        RAISE EXCEPTION 'guest cannot receive account access' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER memberships_no_guest BEFORE INSERT OR UPDATE ON memberships
FOR EACH ROW EXECUTE FUNCTION reject_guest_account_access();
CREATE TRIGGER refresh_tokens_no_guest BEFORE INSERT OR UPDATE ON refresh_tokens
FOR EACH ROW EXECUTE FUNCTION reject_guest_account_access();
CREATE TRIGGER email_otps_no_guest BEFORE INSERT OR UPDATE ON email_otps
FOR EACH ROW EXECUTE FUNCTION reject_guest_account_access();

-- Decisions retain actor, target, room and timestamp independently of realtime sessions.
CREATE TABLE guest_decisions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    room_id uuid NOT NULL REFERENCES rooms(id),
    link_id uuid NOT NULL REFERENCES guest_links(id),
    actor_id uuid NOT NULL REFERENCES users(id),
    target_id uuid REFERENCES users(id),
    action text NOT NULL CHECK (action IN ('approve', 'deny', 'revoke')),
    CHECK ((action = 'revoke') = (target_id IS NULL)),
    created_at timestamptz NOT NULL,
    FOREIGN KEY (link_id, room_id) REFERENCES guest_links(id, room_id)
);

-- +goose Down
-- Non-destructive rollback: stop if guest identities exist; restore application
-- version only after an explicitly approved guest-data retention/export plan.
-- +goose StatementBegin
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM users WHERE is_guest) THEN
        RAISE EXCEPTION 'guest data exists; rollback requires an explicit retention plan';
    END IF;
END $$;
-- +goose StatementEnd
DROP TABLE guest_decisions;
DROP TRIGGER email_otps_no_guest ON email_otps;
DROP TRIGGER refresh_tokens_no_guest ON refresh_tokens;
DROP TRIGGER memberships_no_guest ON memberships;
DROP FUNCTION reject_guest_account_access();
DROP TABLE guest_requests;
DROP TYPE guest_status;
DROP TABLE guest_links;
ALTER TABLE users DROP CONSTRAINT users_id_kind_uq;
ALTER TABLE users DROP CONSTRAINT users_guest_no_login;
ALTER TABLE users DROP COLUMN is_guest;
ALTER TABLE users ALTER COLUMN email SET NOT NULL;
ALTER TABLE rooms DROP CONSTRAINT rooms_id_org_uq;
