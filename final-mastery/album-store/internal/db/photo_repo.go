package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"album-store/internal/model"
)

// ErrNotFound is returned when a record does not exist.
var ErrNotFound = errors.New("not found")

// CreatePhoto atomically increments the album's seq_counter and inserts the photo row
// using a single CTE — no explicit transaction required, shorter lock hold time.
func CreatePhoto(ctx context.Context, pool *pgxpool.Pool, photo *model.Photo) error {
	err := pool.QueryRow(ctx,
		`WITH new_seq AS (
			UPDATE albums
			SET seq_counter = seq_counter + 1
			WHERE album_id = $1
			RETURNING seq_counter
		)
		INSERT INTO photos (photo_id, album_id, seq, status, s3_key, created_at)
		SELECT $2, $1, seq_counter, 'processing', $3, NOW()
		FROM new_seq
		RETURNING seq`,
		photo.AlbumID, photo.PhotoID, photo.S3Key,
	).Scan(&photo.Seq)
	if err != nil {
		return fmt.Errorf("CreatePhoto: %w", err)
	}
	return nil
}

func GetPhoto(ctx context.Context, pool *pgxpool.Pool, albumID, photoID string) (*model.Photo, error) {
	row := pool.QueryRow(ctx,
		`SELECT photo_id, album_id, seq, status, COALESCE(url, ''), COALESCE(s3_key, ''), created_at
		 FROM photos WHERE photo_id = $1 AND album_id = $2`,
		photoID, albumID,
	)

	var p model.Photo
	if err := row.Scan(&p.PhotoID, &p.AlbumID, &p.Seq, &p.Status, &p.URL, &p.S3Key, &p.CreatedAt); err != nil {
		return nil, fmt.Errorf("GetPhoto: %w", err)
	}
	return &p, nil
}

// DeletePhotoReturning deletes a photo and returns its s3_key and status.
// Returns ErrNotFound if the row does not exist.
func DeletePhotoReturning(ctx context.Context, pool *pgxpool.Pool, albumID, photoID string) (s3Key, status string, err error) {
	err = pool.QueryRow(ctx,
		`DELETE FROM photos WHERE photo_id = $1 AND album_id = $2
		 RETURNING COALESCE(s3_key, ''), status`,
		photoID, albumID,
	).Scan(&s3Key, &status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", ErrNotFound
		}
		return "", "", fmt.Errorf("DeletePhotoReturning: %w", err)
	}
	return s3Key, status, nil
}

func UpdatePhotoStatus(ctx context.Context, pool *pgxpool.Pool, photoID, status, url string) error {
	_, err := pool.Exec(ctx,
		`UPDATE photos SET status = $1, url = $2 WHERE photo_id = $3`,
		status, url, photoID,
	)
	if err != nil {
		return fmt.Errorf("UpdatePhotoStatus: %w", err)
	}
	return nil
}

// RequeueStalePhotos returns photos stuck in 'processing' for longer than 60s.
// The caller is responsible for re-submitting them to the worker channel.
func RequeueStalePhotos(ctx context.Context, pool *pgxpool.Pool) ([]model.Photo, error) {
	cutoff := time.Now().Add(-60 * time.Second)
	rows, err := pool.Query(ctx,
		`SELECT photo_id, album_id, seq, status, COALESCE(url, ''), COALESCE(s3_key, ''), created_at
		 FROM photos
		 WHERE status = 'processing' AND created_at < $1`,
		cutoff,
	)
	if err != nil {
		return nil, fmt.Errorf("RequeueStalePhotos: %w", err)
	}
	defer rows.Close()

	var photos []model.Photo
	for rows.Next() {
		var p model.Photo
		if err := rows.Scan(&p.PhotoID, &p.AlbumID, &p.Seq, &p.Status, &p.URL, &p.S3Key, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("RequeueStalePhotos scan: %w", err)
		}
		photos = append(photos, p)
	}
	return photos, rows.Err()
}
