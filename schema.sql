-- Seatlane schema. One file, plain SQL: sqlc reads it, store.Open applies it.

CREATE TABLE IF NOT EXISTS events (
    id               INTEGER PRIMARY KEY,
    name             TEXT    NOT NULL,
    venue            TEXT    NOT NULL,
    city             TEXT    NOT NULL,
    starts_at        INTEGER NOT NULL, -- unix seconds
    tagline          TEXT    NOT NULL,
    from_price_cents INTEGER NOT NULL CHECK (from_price_cents > 0),
    currency         TEXT    NOT NULL,
    on_sale          INTEGER NOT NULL DEFAULT 1
);

-- held_by = 0: nobody holds the seat. A hold is active while expires_at > now.
-- sold_to = 0: not sold. Sessions are anonymous positive integers.
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
