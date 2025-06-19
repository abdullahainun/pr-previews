package types

import "time"

type Response struct {
	Success   bool        `json:"success"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
	Error     string      `json:"error,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

type Command struct {
	Type     string `json:"type"`    // preview, plan, cleanup, status, help
	Service  string `json:"service"` // specific service to deploy
	User     string `json:"user"`    // GitHub username
	PRNumber int    `json:"pr_number"`
}

// CommandResponse represents the result of command processing
type CommandResponse struct {
	Success bool                   `json:"success"`
	Message string                 `json:"message"`
	Content string                 `json:"content,omitempty"` // Markdown content for GitHub
	Data    map[string]interface{} `json:"data,omitempty"`
}
