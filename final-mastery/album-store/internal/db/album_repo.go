package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"album-store/internal/model"
)

func CreateAlbum(ctx context.Context, pool *pgxpool.Pool, album *model.Album) error {
	_, err := pool.Exec(ctx,
		`INSERT INTO albums (album_id, title, description, owner, seq_counter, created_at)
		 VALUES ($1, $2, $3, $4, 0, NOW())`,
		album.AlbumID, album.Title, album.Description, album.Owner,
	)
	if err != nil {
		return fmt.Errorf("CreateAlbum: %w", err)
	}
	return nil
}

func GetAlbum(ctx context.Context, pool *pgxpool.Pool, albumID string) (*model.Album, error) {
	row := pool.QueryRow(ctx,
		`SELECT album_id, title, description, owner, seq_counter, created_at
		 FROM albums WHERE album_id = $1`,
		albumID,
	)

	var a model.Album
	err := row.Scan(&a.AlbumID, &a.Title, &a.Description, &a.Owner, &a.SeqCounter, &a.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("GetAlbum: %w", err)
	}
	return &a, nil
}

func ListAlbums(ctx context.Context, pool *pgxpool.Pool) ([]model.Album, error) {
	rows, err := pool.Query(ctx,
		`SELECT album_id, title, description, owner, seq_counter, created_at
		 FROM albums ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("ListAlbums: %w", err)
	}
	defer rows.Close()

	var albums []model.Album
	for rows.Next() {
		var a model.Album
		if err := rows.Scan(&a.AlbumID, &a.Title, &a.Description, &a.Owner, &a.SeqCounter, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("ListAlbums scan: %w", err)
		}
		albums = append(albums, a)
	}
	return albums, rows.Err()
}
