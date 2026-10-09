-- Seatlane schema. One file, plain SQL: sqlc reads it, store.Open applies it.

CREATE TABLE IF NOT EXISTS events (
    id               INTEGER PRIMARY KEY,
    name             TEXT    NOT NULL,
    venue            TEXT    NOT NULL,
    city             TEXT    NOT NULL,
    starts_at        INTEGER NOT NULL, -- unix seconds
    tagline          TEXT    NOT NULL,
    from_price_cents INTEGER NOT NULL CHECK (from_price_cents > 0),  -- cheapest seat
    to_price_cents   INTEGER NOT NULL CHECK (to_price_cents >= from_price_cents), -- dearest seat
    currency         TEXT    NOT NULL,
    on_sale          INTEGER NOT NULL DEFAULT 1
);

-- held_by = 0: nobody holds the seat. A hold is active while expires_at > now.
-- sold_to = 0: not sold. Otherwise both are a signed-in user's id
-- (users.id, from 1 up), set from the server-set `user` input, never from
-- the request body. No foreign key: 0 means nobody.
CREATE TABLE IF NOT EXISTS seats (
    id           INTEGER PRIMARY KEY,
    event_id     INTEGER NOT NULL REFERENCES events (id),
    section      TEXT    NOT NULL,
    section_rank INTEGER NOT NULL,
    row_label    TEXT    NOT NULL,
    seat_number  INTEGER NOT NULL,
    price_cents  INTEGER NOT NULL CHECK (price_cents > 0),
    held_by      INTEGER NOT NULL DEFAULT 0,
    held_at      INTEGER NOT NULL DEFAULT 0,
    expires_at   INTEGER NOT NULL DEFAULT 0,
    sold_to      INTEGER NOT NULL DEFAULT 0,
    sold_at      INTEGER NOT NULL DEFAULT 0,
    UNIQUE (event_id, section, row_label, seat_number)
);

CREATE INDEX IF NOT EXISTS seats_by_event ON seats (event_id, id);

-- Accounts (internal/auth, outside bridge-en). email is trimmed and lower
-- case; password_hash is bcrypt (golang.org/x/crypto/bcrypt). role is one of
-- the app's roles (cmd/server AppRoles); sign-up always creates "customer".
CREATE TABLE IF NOT EXISTS users (
    id            INTEGER PRIMARY KEY,
    email         TEXT    NOT NULL UNIQUE,
    password_hash TEXT    NOT NULL,
    role          TEXT    NOT NULL CHECK (role IN ('customer', 'organizer', 'admin')),
    created_at    INTEGER NOT NULL
);

-- Sign-in sessions. The cookie seatlane_auth carries a random token; only
-- its SHA-256 (hex) is stored, so a copy of the database signs nobody in.
-- A session is valid while expires_at is later than now.
CREATE TABLE IF NOT EXISTS auth_sessions (
    token_hash TEXT    PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users (id),
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS auth_sessions_by_user ON auth_sessions (user_id);

-- Sign-in rate limit, one row per client IP + email (key "<ip>|<email>"):
-- failures counted in the window that started at window_start.
CREATE TABLE IF NOT EXISTS sign_in_attempts (
    key          TEXT    PRIMARY KEY,
    window_start INTEGER NOT NULL,
    failures     INTEGER NOT NULL
);
