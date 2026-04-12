package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RunMigrations applies all schema migrations idempotently.
// Safe to call on every startup.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	migrations := []struct {
		name string
		sql  string
	}{
		{
			name: "001_create_albums",
			sql: `CREATE TABLE IF NOT EXISTS albums (
				album_id    UUID PRIMARY KEY,
				title       TEXT NOT NULL,
				description TEXT NOT NULL DEFAULT '',
				owner       TEXT NOT NULL,
				seq_counter INT  NOT NULL DEFAULT 0,
				created_at  TIMESTAMPTZ DEFAULT NOW()
			)`,
		},
		{
			name: "002_create_photos",
			sql: `CREATE TABLE IF NOT EXISTS photos (
				photo_id   UUID PRIMARY KEY,
				album_id   UUID NOT NULL REFERENCES albums(album_id),
				seq        INT  NOT NULL,
				status     TEXT NOT NULL DEFAULT 'processing',
				url        TEXT,
				s3_key     TEXT,
				created_at TIMESTAMPTZ DEFAULT NOW(),
				UNIQUE (album_id, seq)
			)`,
		},
		{
			name: "002_create_photos_idx_album",
			sql:  `CREATE INDEX IF NOT EXISTS idx_photos_album_id ON photos(album_id)`,
		},
		{
			name: "002_create_photos_idx_status",
			sql:  `CREATE INDEX IF NOT EXISTS idx_photos_status ON photos(status)`,
		},
	}

	for _, m := range migrations {
		if _, err := pool.Exec(ctx, m.sql); err != nil {
			return fmt.Errorf("migration %s failed: %w", m.name, err)
		}
	}
	return nil
}
