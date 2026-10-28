-- +goose Up
CREATE TYPE org_role AS ENUM ('admin', 'teacher', 'participant');
CREATE TYPE chat_scope AS ENUM ('near', 'zone', 'room', 'broadcast');
CREATE TYPE mod_action AS ENUM ('mute', 'unmute', 'kick', 'lock', 'unlock');
CREATE TYPE report_status AS ENUM ('open', 'reviewed', 'dismissed');

CREATE TABLE users (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email             text NOT NULL,
    email_verified_at timestamptz,
    password_hash     text,              -- null untuk akun Google saja
    google_sub        text UNIQUE,
    display_name      text NOT NULL,
    birth_date        date,              -- wajib diisi sebelum join organisasi
    avatar_key        text NOT NULL DEFAULT 'default',
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX users_email_uq ON users (lower(email));

CREATE TABLE email_otps (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash   bytea NOT NULL,
    attempts    int NOT NULL DEFAULT 0,
    expires_at  timestamptz NOT NULL,
    consumed_at timestamptz
);

CREATE TABLE refresh_tokens (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    family_id  uuid NOT NULL,
    token_hash bytea NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    used_at    timestamptz,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX refresh_tokens_family_idx ON refresh_tokens (family_id);

CREATE TABLE organizations (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name       text NOT NULL,
    slug       text NOT NULL UNIQUE,
    logo_url   text,
    created_by uuid NOT NULL REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE memberships (
    org_id    uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id   uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role      org_role NOT NULL,
    joined_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, user_id)
);
CREATE INDEX memberships_user_idx ON memberships (user_id);

CREATE TABLE invites (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    code       text NOT NULL UNIQUE,
    role       org_role NOT NULL CHECK (role <> 'admin'),
    max_uses   int,                      -- null = tanpa batas
    used_count int NOT NULL DEFAULT 0,
    created_by uuid NOT NULL REFERENCES users(id),
    expires_at timestamptz NOT NULL,     -- maks 7 hari, dicek di service
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE rooms (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id           uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name             text NOT NULL,
    map_key          text NOT NULL,      -- 'campus' | 'classroom' | 'garden'
    building_key     text,               -- gedung di peta sekolah; null untuk peta sekolah
    capacity         int NOT NULL DEFAULT 40,
    all_chat_enabled boolean NOT NULL DEFAULT true,
    scheduled_at     timestamptz,
    created_by       uuid NOT NULL REFERENCES users(id),
    created_at       timestamptz NOT NULL DEFAULT now(),
    archived_at      timestamptz
);
CREATE INDEX rooms_org_idx ON rooms (org_id) WHERE archived_at IS NULL;
CREATE UNIQUE INDEX rooms_campus_uq ON rooms (org_id) WHERE map_key = 'campus'; -- satu peta sekolah per organisasi

CREATE TABLE room_sessions (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    room_id    uuid NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    started_at timestamptz NOT NULL DEFAULT now(),
    ended_at   timestamptz
);

CREATE TABLE session_attendances (
    session_id uuid NOT NULL REFERENCES room_sessions(id) ON DELETE CASCADE,
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    joined_at  timestamptz NOT NULL,
    left_at    timestamptz,
    PRIMARY KEY (session_id, user_id, joined_at)
);

CREATE TABLE chat_messages (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    session_id uuid NOT NULL REFERENCES room_sessions(id) ON DELETE CASCADE,
    sender_id  uuid NOT NULL REFERENCES users(id),
    scope      chat_scope NOT NULL,
    zone_key   text,
    body       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX chat_messages_created_idx ON chat_messages (created_at); -- purge > 30 hari

CREATE TABLE moderation_actions (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    room_id    uuid NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    actor_id   uuid NOT NULL REFERENCES users(id),
    target_id  uuid REFERENCES users(id),  -- null untuk lock/unlock
    action     mod_action NOT NULL,
    reason     text,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE reports (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id         uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    room_id        uuid REFERENCES rooms(id) ON DELETE SET NULL,
    reporter_id    uuid NOT NULL REFERENCES users(id),
    target_user_id uuid NOT NULL REFERENCES users(id),
    message_id     bigint REFERENCES chat_messages(id) ON DELETE SET NULL,
    reason         text NOT NULL,
    status         report_status NOT NULL DEFAULT 'open',
    reviewed_by    uuid REFERENCES users(id),
    reviewed_at    timestamptz,
    created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX reports_org_status_idx ON reports (org_id, status);

-- +goose Down
DROP TABLE reports;
DROP TABLE moderation_actions;
DROP TABLE chat_messages;
DROP TABLE session_attendances;
DROP TABLE room_sessions;
DROP TABLE rooms;
DROP TABLE invites;
DROP TABLE memberships;
DROP TABLE organizations;
DROP TABLE refresh_tokens;
DROP TABLE email_otps;
DROP TABLE users;

DROP TYPE report_status;
DROP TYPE mod_action;
DROP TYPE chat_scope;
DROP TYPE org_role;
