package handler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"album-store/internal/db"
	"album-store/internal/model"
	"album-store/internal/storage"
	"album-store/internal/worker"
)

type PhotoHandler struct {
	pool   *pgxpool.Pool
	s3     *storage.Client
	bucket string
	jobs   *worker.Pool
}

func NewPhotoHandler(pool *pgxpool.Pool, s3 *storage.Client, bucket string, jobs *worker.Pool) *PhotoHandler {
	return &PhotoHandler{pool: pool, s3: s3, bucket: bucket, jobs: jobs}
}

// POST /albums/:id/photos
func (h *PhotoHandler) Upload(c *gin.Context) {
	albumID := c.Param("id")

	file, header, err := c.Request.FormFile("photo")
	if err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "missing 'photo' field in multipart form"})
		return
	}
	defer file.Close()

	// Read into memory — avoids two disk I/O syscalls (write + re-read) per request.
	// Pre-allocate based on reported size to reduce GC pressure.
	size := header.Size
	if size <= 0 {
		size = 512
	}
	buf := make([]byte, 0, size)
	bb := bytes.NewBuffer(buf)
	if _, err := bb.ReadFrom(file); err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: "failed to read photo data"})
		return
	}
	data := bb.Bytes()

	photoID := uuid.New().String()
	s3Key := fmt.Sprintf("albums/%s/photos/%s", albumID, photoID)
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	photo := &model.Photo{
		PhotoID: photoID,
		AlbumID: albumID,
		S3Key:   s3Key,
		Status:  "processing",
	}

	if err := db.CreatePhoto(c.Request.Context(), h.pool, photo); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, model.ErrorResponse{Error: "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: "failed to create photo record"})
		return
	}

	if err := h.jobs.Submit(c.Request.Context(), worker.PhotoJob{
		PhotoID:     photoID,
		AlbumID:     albumID,
		S3Key:       s3Key,
		Data:        data,
		ContentType: contentType,
	}); err != nil {
		// Mark failed so it doesn't stay stuck in processing
		_ = db.UpdatePhotoStatus(c.Request.Context(), h.pool, photoID, "failed", "")
		c.JSON(http.StatusServiceUnavailable, model.ErrorResponse{Error: "worker pool busy, try again"})
		return
	}

	c.JSON(http.StatusAccepted, photo)
}

// GET /albums/:id/photos/:photoId
func (h *PhotoHandler) Get(c *gin.Context) {
	albumID := c.Param("id")
	photoID := c.Param("photoId")

	photo, err := db.GetPhoto(c.Request.Context(), h.pool, albumID, photoID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, model.ErrorResponse{Error: "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: "database error"})
		return
	}

	c.JSON(http.StatusOK, photo)
}

// DELETE /albums/:id/photos/:photoId
func (h *PhotoHandler) Delete(c *gin.Context) {
	albumID := c.Param("id")
	photoID := c.Param("photoId")

	// Single DB roundtrip: delete and return s3_key + status.
	s3Key, status, err := db.DeletePhotoReturning(c.Request.Context(), h.pool, albumID, photoID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusNotFound, model.ErrorResponse{Error: "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: "failed to delete photo record"})
		return
	}

	// Fire-and-forget S3 cleanup — no need to block the response.
	if s3Key != "" && status == "completed" {
		go h.s3.DeleteObject(context.Background(), h.bucket, s3Key)
	}

	c.Status(http.StatusNoContent)
}
