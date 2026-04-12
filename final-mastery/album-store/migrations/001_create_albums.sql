CREATE TABLE IF NOT EXISTS albums (
    album_id    UUID PRIMARY KEY,
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    owner       TEXT NOT NULL,
    seq_counter INT  NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);
