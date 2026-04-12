CREATE TABLE IF NOT EXISTS photos (
    photo_id   UUID PRIMARY KEY,
    album_id   UUID NOT NULL REFERENCES albums(album_id),
    seq        INT  NOT NULL,
    status     TEXT NOT NULL DEFAULT 'processing',
    url        TEXT,
    s3_key     TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (album_id, seq)
);

CREATE INDEX IF NOT EXISTS idx_photos_album_id ON photos(album_id);
CREATE INDEX IF NOT EXISTS idx_photos_status   ON photos(status);
