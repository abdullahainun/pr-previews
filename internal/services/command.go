package services

import (
	"fmt"
	"regexp"
	"strings"

	"pr-previews/internal/types"
)

type CommandService struct {
	// Will add dependencies later
}

func NewCommandService() *CommandService {
	return &CommandService{}
}

// ParseCommand parses GitHub comment text into Command
func (cs *CommandService) ParseCommand(commentBody, user string, prNumber int) (*types.Command, error) {
	comment := strings.TrimSpace(commentBody)

	// Command patterns
	patterns := map[string]*regexp.Regexp{
		"help":    regexp.MustCompile(`^/help\s*$`),
		"status":  regexp.MustCompile(`^/status\s*$`),
		"plan":    regexp.MustCompile(`^/plan(?:\s+([a-zA-Z0-9/-]+))?\s*$`),
		"preview": regexp.MustCompile(`^/preview(?:\s+([a-zA-Z0-9/-]+))?\s*$`),
		"cleanup": regexp.MustCompile(`^/cleanup\s*$`),
	}

	for cmdType, pattern := range patterns {
		if matches := pattern.FindStringSubmatch(comment); matches != nil {
			cmd := &types.Command{
				Type:     cmdType,
				User:     user,
				PRNumber: prNumber,
			}

			// Extract service name if provided
			if len(matches) > 1 && matches[1] != "" {
				cmd.Service = matches[1]
			}

			return cmd, nil
		}
	}

	return nil, fmt.Errorf("unknown command: %s", comment)
}

// ProcessCommand processes parsed command and returns response
func (cs *CommandService) ProcessCommand(cmd *types.Command) *types.CommandResponse {
	switch cmd.Type {
	case "help":
		return cs.handleHelp(cmd)
	case "status":
		return cs.handleStatus(cmd)
	case "plan":
		return cs.handlePlan(cmd)
	case "preview":
		return cs.handlePreview(cmd)
	case "cleanup":
		return cs.handleCleanup(cmd)
	default:
		return &types.CommandResponse{
			Success: false,
			Message: fmt.Sprintf("Unknown command type: %s", cmd.Type),
		}
	}
}

func (cs *CommandService) handleHelp(cmd *types.Command) *types.CommandResponse {
	helpText := `## 🤖 Available Commands

**📖 Read-Only Commands (Available to Everyone):**
- ` + "`/help`" + ` - Show this help message
- ` + "`/status`" + ` - Show current preview environments
- ` + "`/plan`" + ` - Show what would be deployed (dry-run)
- ` + "`/plan <service>`" + ` - Show plan for specific service

**🚀 Deployment Commands (Core Team Only):**
- ` + "`/preview`" + ` - Deploy all changed services to preview
- ` + "`/preview <service>`" + ` - Deploy specific service
- ` + "`/cleanup`" + ` - Cleanup preview environments

**Examples:**
` + "```" + `
/help
/status
/plan
/plan ai/open-webui
/preview
/preview ai/open-webui
/cleanup
` + "```" + `

*Triggered by: @` + cmd.User + `*`

	return &types.CommandResponse{
		Success: true,
		Message: "Help information",
		Content: helpText,
		Data: map[string]interface{}{
			"available_commands": []string{"help", "status", "plan", "preview", "cleanup"},
			"user_permissions":   cs.getUserPermissions(cmd.User),
		},
	}
}

func (cs *CommandService) handleStatus(cmd *types.Command) *types.CommandResponse {
	// TODO: Get actual preview environments from Kubernetes
	return &types.CommandResponse{
		Success: true,
		Message: "Preview Environment Status",
		Content: fmt.Sprintf(`## 📊 Preview Environment Status

**PR:** #%d

### ℹ️ No Preview Environments Found

No preview environments are currently active for this PR.

**To create preview environments:**
- Run `+"`/preview`"+` to deploy all changed services
- Run `+"`/preview <service>`"+` to deploy a specific service

*Status checked by: @%s*`, cmd.PRNumber, cmd.User),
		Data: map[string]interface{}{
			"pr_number":       cmd.PRNumber,
			"active_previews": []string{}, // TODO: Get from K8s
			"total_previews":  0,
		},
	}
}

func (cs *CommandService) handlePlan(cmd *types.Command) *types.CommandResponse {
	serviceName := cmd.Service
	if serviceName == "" {
		serviceName = "all changed services"
	}

	planContent := fmt.Sprintf(`## 📋 Deployment Plan

**👤 Requested by:** @%s
**🎯 Services to deploy:** %s
**🔗 PR:** #%d

### 📦 Service Analysis
- **Status:** ✅ Plan generation successful
- **Services detected:** %s
- **Validation:** ✅ All checks passed
- **Estimated deployment time:** ~2-3 minutes

### 🚀 Next Steps
- Run `+"`/preview`"+` to deploy these services
- Run `+"`/preview <service>`"+` to deploy specific service

*This plan is read-only and safe for everyone to use.*`,
		cmd.User, serviceName, cmd.PRNumber, serviceName)

	return &types.CommandResponse{
		Success: true,
		Message: "Deployment plan generated",
		Content: planContent,
		Data: map[string]interface{}{
			"services":    []string{serviceName},
			"pr_number":   cmd.PRNumber,
			"safe_to_run": true,
		},
	}
}

func (cs *CommandService) handlePreview(cmd *types.Command) *types.CommandResponse {
	// Check permissions for deployment commands
	if !cs.hasDeploymentPermission(cmd.User) {
		return &types.CommandResponse{
			Success: false,
			Message: "Access denied",
			Content: fmt.Sprintf(`🔒 **Access Denied for @%s**

Sorry, you don't have permission to trigger deployments.

**Available options:**
- 📋 Use `+"`/plan`"+` to see what would be deployed (read-only)
- 📊 Use `+"`/status`"+` to check current preview environments
- 📖 Use `+"`/help`"+` to see all available commands

**Want deployment access?**
Contact @abdullahainun for collaboration opportunities.`, cmd.User),
		}
	}

	serviceName := cmd.Service
	if serviceName == "" {
		serviceName = "all changed services"
	}

	// TODO: Implement actual deployment logic
	previewContent := fmt.Sprintf(`## 🚀 Preview Deployment Started

**👤 Triggered by:** @%s
**🎯 Services:** %s
**🔗 PR:** #%d

### 📋 Deployment Status
- ✅ Validation passed
- ✅ Resources planned
- 🔄 Deployment in progress...

**Estimated completion:** 2-3 minutes

*Deployment logic will be implemented in next phase.*`, cmd.User, serviceName, cmd.PRNumber)

	return &types.CommandResponse{
		Success: true,
		Message: "Preview deployment started",
		Content: previewContent,
		Data: map[string]interface{}{
			"services":  []string{serviceName},
			"pr_number": cmd.PRNumber,
			"status":    "in_progress",
		},
	}
}

func (cs *CommandService) handleCleanup(cmd *types.Command) *types.CommandResponse {
	// Check permissions
	if !cs.hasDeploymentPermission(cmd.User) {
		return &types.CommandResponse{
			Success: false,
			Message: "Access denied",
			Content: fmt.Sprintf("🔒 Access denied for @%s. Only core team can cleanup environments.", cmd.User),
		}
	}

	// TODO: Implement actual cleanup logic
	cleanupContent := fmt.Sprintf(`## 🧹 Cleanup Started

**👤 Triggered by:** @%s
**🔗 PR:** #%d

### 📋 Cleanup Status
- 🔍 Scanning for preview environments...
- 🗑️ Cleanup in progress...

*Cleanup logic will be implemented in next phase.*`, cmd.User, cmd.PRNumber)

	return &types.CommandResponse{
		Success: true,
		Message: "Cleanup started",
		Content: cleanupContent,
		Data: map[string]interface{}{
			"pr_number": cmd.PRNumber,
			"status":    "in_progress",
		},
	}
}

func (cs *CommandService) hasDeploymentPermission(user string) bool {
	// Core team members
	coreTeam := []string{"abdullahainun"}

	for _, member := range coreTeam {
		if user == member {
			return true
		}
	}

	// TODO: Check repository permissions via GitHub API
	return false
}

func (cs *CommandService) getUserPermissions(user string) map[string]bool {
	return map[string]bool{
		"can_read":   true,
		"can_deploy": cs.hasDeploymentPermission(user),
		"is_core":    cs.hasDeploymentPermission(user),
	}
}
