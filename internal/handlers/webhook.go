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
		// Try to parse JSON payload, but don't fail if it's not valid JSON
		c.ShouldBindJSON(&payload)
	}

	// Initialize payload map if nil
	if payload == nil {
		payload = make(map[string]interface{})
	}

	// Get comment from query params (for testing) or payload
	commentBody := c.Query("comment")
	if commentBody == "" {
		// Check if it's in the POST payload
		if comment, ok := payload["comment"]; ok {
			if commentStr, ok := comment.(string); ok {
				commentBody = commentStr
			}
		}

		// Check if it's a GitHub webhook with issue comment
		if issue, ok := payload["issue"]; ok {
			if issueMap, ok := issue.(map[string]interface{}); ok {
				if pullRequest, exists := issueMap["pull_request"]; exists && pullRequest != nil {
					// This is a PR comment
					if comment, ok := payload["comment"]; ok {
						if commentMap, ok := comment.(map[string]interface{}); ok {
							if body, ok := commentMap["body"].(string); ok {
								commentBody = body
							}
						}
					}
				}
			}
		}
	}

	// Get user from query params or payload
	user := c.Query("user")
	if user == "" {
		// Check payload for user info
		if comment, ok := payload["comment"]; ok {
			if commentMap, ok := comment.(map[string]interface{}); ok {
				if userObj, ok := commentMap["user"]; ok {
					if userMap, ok := userObj.(map[string]interface{}); ok {
						if login, ok := userMap["login"].(string); ok {
							user = login
						}
					}
				}
			}
		}

		// Default user for testing
		if user == "" {
			user = "testuser"
		}
	}

	// Get PR number from query params or payload
	prNumber := 123 // Default for testing
	if prStr := c.Query("pr"); prStr != "" {
		// Could parse prStr to int, but keeping simple for now
	}

	// If no comment provided, return basic webhook response with examples
	if commentBody == "" {
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
				"available_commands": []string{"/help", "/status", "/plan", "/preview", "/cleanup"},
			},
		}
		c.JSON(http.StatusOK, response)
		return
	}

	// Initialize command service
	cmdService := services.NewCommandService()

	// Parse command
	cmd, err := cmdService.ParseCommand(commentBody, user, prNumber)
	if err != nil {
		response := types.Response{
			Success:   false,
			Message:   "Command parsing failed",
			Error:     err.Error(),
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"comment_received":   commentBody,
				"user":               user,
				"available_commands": []string{"/help", "/status", "/plan", "/preview", "/cleanup"},
				"example":            "Try: ?comment=/help&user=testuser",
			},
		}
		c.JSON(http.StatusBadRequest, response)
		return
	}

	// Process command
	cmdResponse := cmdService.ProcessCommand(cmd)

	// Return successful response
	response := types.Response{
		Success:   cmdResponse.Success,
		Message:   cmdResponse.Message,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"command":        cmd,
			"command_result": cmdResponse,
			"github_content": cmdResponse.Content, // This would be posted to GitHub
			"method":         c.Request.Method,
		},
	}

	// If command failed, include error
	if !cmdResponse.Success {
		response.Error = cmdResponse.Message
		response.Success = false
	}

	c.JSON(http.StatusOK, response)
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
