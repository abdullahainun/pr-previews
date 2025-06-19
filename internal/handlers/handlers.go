package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"pr-previews/internal/config"
	"pr-previews/internal/types"
)

type Handler struct {
	config *config.Config
}

func New(cfg *config.Config) *Handler {
	return &Handler{config: cfg}
}

func (h *Handler) Health(c *gin.Context) {
	response := types.Response{
		Success:   true,
		Message:   "pr-previews service is healthy",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"service": "pr-previews",
			"version": "0.1.0",
			"status":  "healthy",
		},
	}
	c.JSON(http.StatusOK, response)
}

func (h *Handler) Metrics(c *gin.Context) {
	response := types.Response{
		Success:   true,
		Message:   "Metrics endpoint",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"webhooks_received":  "TODO",
			"active_previews":    "TODO",
			"commands_processed": "TODO",
		},
	}
	c.JSON(http.StatusOK, response)
}
