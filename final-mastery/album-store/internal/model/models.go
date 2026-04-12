package model

import "time"

type Album struct {
	AlbumID     string    `json:"album_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Owner       string    `json:"owner"`
	SeqCounter  int       `json:"seq_counter"`
	CreatedAt   time.Time `json:"created_at"`
}

type Photo struct {
	PhotoID   string    `json:"photo_id"`
	AlbumID   string    `json:"album_id"`
	Seq       int       `json:"seq"`
	Status    string    `json:"status"`
	URL       string    `json:"url,omitempty"`
	S3Key     string    `json:"s3_key,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type HealthResponse struct {
	Status string `json:"status"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
