package worker

import (
	"bytes"
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"album-store/internal/db"
	"album-store/internal/storage"
)

type PhotoJob struct {
	PhotoID     string
	AlbumID     string
	S3Key       string
	Data        []byte // in-memory photo data — avoids disk I/O
	ContentType string
}

type Pool struct {
	jobs chan PhotoJob
}

func NewPool(bufferSize int) *Pool {
	return &Pool{
		jobs: make(chan PhotoJob, bufferSize),
	}
}

// Submit enqueues a job. Non-blocking: returns an error only if the buffer is completely full.
func (p *Pool) Submit(_ context.Context, job PhotoJob) error {
	select {
	case p.jobs <- job:
		return nil
	default:
		return context.DeadlineExceeded
	}
}

// Start launches numWorkers goroutines. Goroutines exit when ctx is cancelled.
func (p *Pool) Start(ctx context.Context, dbPool *pgxpool.Pool, s3Client *storage.Client, bucket string, numWorkers int) {
	for i := 0; i < numWorkers; i++ {
		go p.runWorker(ctx, dbPool, s3Client, bucket)
	}
	log.Printf("worker pool started with %d workers", numWorkers)
}

func (p *Pool) runWorker(ctx context.Context, dbPool *pgxpool.Pool, s3Client *storage.Client, bucket string) {
	for {
		select {
		case <-ctx.Done():
			return
		case job, ok := <-p.jobs:
			if !ok {
				return
			}
			p.process(ctx, dbPool, s3Client, bucket, job)
		}
	}
}

func (p *Pool) process(ctx context.Context, dbPool *pgxpool.Pool, s3Client *storage.Client, bucket string, job PhotoJob) {
	contentType := job.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	if _, err := s3Client.UploadPhoto(ctx, bucket, job.S3Key, bytes.NewReader(job.Data), contentType); err != nil {
		log.Printf("worker: S3 upload failed for photo %s: %v", job.PhotoID, err)
		_ = db.UpdatePhotoStatus(ctx, dbPool, job.PhotoID, "failed", "")
		return
	}

	// Release memory early — data already in S3.
	job.Data = nil

	// Generate a 7-day presigned GET URL so ChaosArena can fetch the photo.
	presigned, err := s3Client.PresignedURL(ctx, bucket, job.S3Key, 7*24*time.Hour)
	if err != nil {
		log.Printf("worker: failed to presign URL for photo %s: %v", job.PhotoID, err)
		_ = db.UpdatePhotoStatus(ctx, dbPool, job.PhotoID, "failed", "")
		return
	}

	if err := db.UpdatePhotoStatus(ctx, dbPool, job.PhotoID, "completed", presigned); err != nil {
		log.Printf("worker: failed to mark photo %s as completed: %v", job.PhotoID, err)
		return
	}
}

// RequeueStale marks photos stuck in 'processing' > 60s as failed.
// Raw bytes can't be recovered after a restart, so we fail them so clients can retry.
func (p *Pool) RequeueStale(ctx context.Context, dbPool *pgxpool.Pool) {
	photos, err := db.RequeueStalePhotos(ctx, dbPool)
	if err != nil {
		log.Printf("RequeueStale: failed to query stale photos: %v", err)
		return
	}
	for _, photo := range photos {
		_ = db.UpdatePhotoStatus(ctx, dbPool, photo.PhotoID, "failed", "")
		log.Printf("RequeueStale: marked stale photo %s as failed", photo.PhotoID)
	}
	if len(photos) > 0 {
		log.Printf("RequeueStale: marked %d stale photos as failed", len(photos))
	}
}
