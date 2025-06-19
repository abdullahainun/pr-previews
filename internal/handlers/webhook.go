package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"pr-previews/internal/services"
	"pr-previews/internal/types"
)

func (h *Handler) GitHubWebhook(c *gin.Context) {
	var payload map[string]interface{}

	// Only parse JSON for POST requests
	if c.Request.Method == "POST" {
		c.ShouldBindJSON(&payload)
	}

	if payload == nil {
		payload = make(map[string]interface{})
	}

	// Get comment from query params or payload
	commentBody := c.Query("comment")
	if commentBody == "" {
		// Basic webhook response with examples
		response := types.Response{
			Success:   true,
			Message:   "GitHub webhook received",
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"note":   "Add ?comment=/help&user=yourname to test command parsing",
				"method": c.Request.Method,
				"examples": map[string]string{
					"help":    "/webhook/github?comment=/help&user=testuser",
					"status":  "/webhook/github?comment=/status&user=testuser",
					"plan":    "/webhook/github?comment=/plan&user=testuser",
					"preview": "/webhook/github?comment=/preview&user=abdullahainun",
					"cleanup": "/webhook/github?comment=/cleanup&user=abdullahainun",
				},
			},
		}
		c.JSON(http.StatusOK, response)
		return
	}

	user := c.Query("user")
	if user == "" {
		user = "testuser"
	}

	prNumber := 123

	// 🔧 Use K8s-enhanced command service instead of basic one
	cmdService, err := services.NewCommandServiceK8s()
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, "Failed to create K8s service", err)
		return
	}

	// Parse command using basic service
	basicService := services.NewCommandService()
	cmd, err := basicService.ParseCommand(commentBody, user, prNumber)
	if err != nil {
		response := types.Response{
			Success:   false,
			Message:   "Command parsing failed",
			Error:     err.Error(),
			Timestamp: time.Now(),
		}
		c.JSON(http.StatusBadRequest, response)
		return
	}

	// Process command using K8s-enhanced service based on command type
	var cmdResponse *types.CommandResponse

	switch cmd.Type {
	case "help":
		// Use basic service for help
		cmdResponse = basicService.ProcessCommand(cmd)
	case "status":
		// Use K8s service for real status
		cmdResponse = cmdService.HandleStatusK8s(c.Request.Context(), cmd)
	case "plan":
		// Use basic service for plan (read-only)
		cmdResponse = basicService.ProcessCommand(cmd)
	case "preview":
		// Use K8s service for real preview deployment
		if !hasDeploymentPermission(cmd.User) {
			cmdResponse = &types.CommandResponse{
				Success: false,
				Message: "Access denied",
				Content: "🔒 Access denied. Only core team can deploy.",
			}
		} else {
			cmdResponse = cmdService.HandlePreviewK8s(c.Request.Context(), cmd)
		}
	case "cleanup":
		// Use K8s service for real cleanup
		if !hasDeploymentPermission(cmd.User) {
			cmdResponse = &types.CommandResponse{
				Success: false,
				Message: "Access denied",
				Content: "🔒 Access denied. Only core team can cleanup.",
			}
		} else {
			cmdResponse = cmdService.HandleCleanupK8s(c.Request.Context(), cmd)
		}
	default:
		cmdResponse = &types.CommandResponse{
			Success: false,
			Message: "Unknown command",
		}
	}

	// Return response
	response := types.Response{
		Success:   cmdResponse.Success,
		Message:   cmdResponse.Message,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"command":        cmd,
			"command_result": cmdResponse,
			"github_content": cmdResponse.Content,
			"method":         c.Request.Method,
		},
	}

	if !cmdResponse.Success {
		response.Error = cmdResponse.Message
		response.Success = false
	}

	c.JSON(http.StatusOK, response)
}

// Helper function for permission checking
func hasDeploymentPermission(user string) bool {
	coreTeam := []string{"abdullahainun"}
	for _, member := range coreTeam {
		if user == member {
			return true
		}
	}
	return false
}

func (h *Handler) respondError(c *gin.Context, status int, message string, err error) {
	response := types.Response{
		Success:   false,
		Message:   message,
		Timestamp: time.Now(),
	}

	if err != nil {
		response.Error = err.Error()
	}

	c.JSON(status, response)
}
