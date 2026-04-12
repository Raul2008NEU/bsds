package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"album-store/internal/model"
)

func Health(c *gin.Context) {
	c.JSON(http.StatusOK, model.HealthResponse{Status: "ok"})
}
