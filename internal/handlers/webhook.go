package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"pr-previews/internal/services"
	"pr-previews/internal/types"
)

func (h *Handler) GitHubWebhook(c *gin.Context) {
	var payload map[string]interface{}

	if c.Request.Method == "POST" {
		c.ShouldBindJSON(&payload)
	}

	if payload == nil {
		payload = make(map[string]interface{})
	}

	commentBody := c.Query("comment")
	if commentBody == "" {
		response := types.Response{
			Success:   true,
			Message:   "GitHub webhook received",
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"note":   "Add ?comment=/help&user=yourname to test",
				"method": c.Request.Method,
				"examples": map[string]string{
					"help":    "/webhook/github?comment=/help&user=testuser",
					"status":  "/webhook/github?comment=/status&user=testuser",
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

	// Create services
	cmdService, err := services.NewCommandServiceK8s()
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, "Failed to create K8s service", err)
		return
	}

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

	// Process command
	var cmdResponse *types.CommandResponse

	switch cmd.Type {
	case "help":
		cmdResponse = basicService.ProcessCommand(cmd)
		if cmdResponse.Success {
			// Add manifest services info to help
			repoPath := "."
			availableServices := cmdService.GetAvailableServicesWithManifest(repoPath)
			manifestInfo := "\n\n### 📁 Available Services\n"
			for _, svc := range availableServices {
				manifestInfo += fmt.Sprintf("- `%s`\n", svc)
			}
			manifestInfo += "\n**To add new services:** Create YAML manifests in `k8s/`, `kubernetes/`, `manifests/`, or `deploy/` folders."
			cmdResponse.Content += manifestInfo
		}
	case "status":
		cmdResponse = cmdService.HandleStatusK8s(c.Request.Context(), cmd)
	case "plan":
		cmdResponse = basicService.ProcessCommand(cmd)
	case "preview":
		if !hasDeploymentPermission(cmd.User) {
			cmdResponse = &types.CommandResponse{
				Success: false,
				Message: "Access denied",
				Content: "🔒 Access denied. Only core team can deploy.",
			}
		} else {
			// Use enhanced preview with manifest support
			repoPath := "." // Current directory
			cmdResponse = cmdService.HandlePreviewK8sEnhanced(c.Request.Context(), cmd, repoPath)
		}
	case "cleanup":
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

func hasDeploymentPermission(user string) bool {
	coreTeam := []string{"abdullahainun"}
	for _, member := range coreTeam {
		if user == member {
			return true
		}
	}
	return false
}
