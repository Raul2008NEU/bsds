package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"album-store/internal/db"
	"album-store/internal/model"
)

type AlbumHandler struct {
	pool *pgxpool.Pool
}

func NewAlbumHandler(pool *pgxpool.Pool) *AlbumHandler {
	return &AlbumHandler{pool: pool}
}

// PUT /albums/:id — atomic upsert, race-safe under concurrent requests.
func (h *AlbumHandler) Upsert(c *gin.Context) {
	albumID := c.Param("id")
	if _, err := uuid.Parse(albumID); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "album_id must be a valid UUID"})
		return
	}

	var body struct {
		Title       string `json:"title" binding:"required"`
		Description string `json:"description"`
		Owner       string `json:"owner" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: err.Error()})
		return
	}

	// Single atomic upsert — no read-then-write race condition.
	// xmax=0 means the row was inserted (not updated).
	var a model.Album
	var isInsert bool
	err := h.pool.QueryRow(c.Request.Context(),
		`INSERT INTO albums (album_id, title, description, owner, seq_counter, created_at)
		 VALUES ($1, $2, $3, $4, 0, NOW())
		 ON CONFLICT (album_id) DO UPDATE
		     SET title = EXCLUDED.title,
		         description = EXCLUDED.description,
		         owner = EXCLUDED.owner
		 RETURNING album_id, title, description, owner, seq_counter, created_at, (xmax = 0)`,
		albumID, body.Title, body.Description, body.Owner,
	).Scan(&a.AlbumID, &a.Title, &a.Description, &a.Owner, &a.SeqCounter, &a.CreatedAt, &isInsert)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: "database error"})
		return
	}

	if isInsert {
		c.JSON(http.StatusCreated, a)
	} else {
		c.JSON(http.StatusOK, a)
	}
}

// GET /albums/:id
func (h *AlbumHandler) Get(c *gin.Context) {
	albumID := c.Param("id")

	album, err := db.GetAlbum(c.Request.Context(), h.pool, albumID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, model.ErrorResponse{Error: "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: "database error"})
		return
	}

	c.JSON(http.StatusOK, album)
}

// GET /albums
func (h *AlbumHandler) List(c *gin.Context) {
	albums, err := db.ListAlbums(c.Request.Context(), h.pool)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: "database error"})
		return
	}

	if albums == nil {
		albums = []model.Album{}
	}
	c.JSON(http.StatusOK, albums)
}
