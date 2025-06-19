package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"pr-previews/internal/config"
	"pr-previews/internal/services"
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

func (h *Handler) TestK8s(c *gin.Context) {
	cmdService, err := services.NewCommandServiceK8s()
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, "Failed to create K8s service", err)
		return
	}

	result := cmdService.TestK8sConnection(c.Request.Context())

	response := types.Response{
		Success:   result.Success,
		Message:   result.Message,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"k8s_test_result": result,
			"content":         result.Content,
		},
	}

	if !result.Success {
		response.Error = result.Message
	}

	c.JSON(http.StatusOK, response)
}
