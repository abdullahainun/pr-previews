package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"pr-previews/internal/types"
)

// Enhanced CommandService with K8s integration
type CommandServiceK8s struct {
	k8s *K8sService
}

func NewCommandServiceK8s() (*CommandServiceK8s, error) {
	k8sService, err := NewK8sService()
	if err != nil {
		return nil, fmt.Errorf("failed to create K8s service: %v", err)
	}

	return &CommandServiceK8s{
		k8s: k8sService,
	}, nil
}

// TestK8sConnection tests Kubernetes connectivity
func (cs *CommandServiceK8s) TestK8sConnection(ctx context.Context) *types.CommandResponse {
	err := cs.k8s.TestConnection(ctx)
	if err != nil {
		return &types.CommandResponse{
			Success: false,
			Message: "K8s connection failed",
			Content: fmt.Sprintf("## ❌ Kubernetes Connection Failed\n\n**Error:** %s\n\n**Troubleshooting:**\n- Check if kubectl is configured correctly\n- Verify cluster connectivity: `kubectl cluster-info`\n- Check permissions: `kubectl auth can-i create namespaces`", err.Error()),
		}
	}

	// Get cluster info
	clusterInfo, err := cs.k8s.GetClusterInfo(ctx)
	if err != nil {
		clusterInfo = map[string]interface{}{
			"error": err.Error(),
		}
	}

	return &types.CommandResponse{
		Success: true,
		Message: "K8s connection successful",
		Content: fmt.Sprintf("## ✅ Kubernetes Connection Successful\n\n**Cluster Status:** Connected\n**Nodes:** %v\n**Total Namespaces:** %v\n**Preview Namespaces:** %v\n\nReady for preview deployments! 🚀",
			clusterInfo["nodes_count"],
			clusterInfo["namespaces_count"],
			clusterInfo["preview_namespaces"]),
		Data: clusterInfo,
	}
}

// Enhanced status command with real K8s data including deployments
func (cs *CommandServiceK8s) HandleStatusK8s(ctx context.Context, cmd *types.Command) *types.CommandResponse {
	// Get preview namespaces for this PR
	previewNamespaces, err := cs.k8s.GetPreviewNamespacesByPR(ctx, cmd.PRNumber)
	if err != nil {
		return &types.CommandResponse{
			Success: false,
			Message: "Failed to get preview status",
			Content: fmt.Sprintf("❌ Error getting preview environments: %s", err.Error()),
		}
	}

	if len(previewNamespaces) == 0 {
		return &types.CommandResponse{
			Success: true,
			Message: "No preview environments found",
			Content: fmt.Sprintf("## 📊 Preview Environment Status\n\n**PR:** #%d\n\n### ℹ️ No Preview Environments Found\n\nNo preview environments are currently active for this PR.\n\n**To create preview environments:**\n- Run `/preview` to deploy all changed services\n- Run `/preview <service>` to deploy a specific service\n\n*Status checked by: @%s*", cmd.PRNumber, cmd.User),
			Data: map[string]interface{}{
				"pr_number":       cmd.PRNumber,
				"active_previews": []string{},
				"total_previews":  0,
			},
		}
	}

	// Build status content with real deployment info
	var contentBuilder strings.Builder
	contentBuilder.WriteString(fmt.Sprintf("## 📊 Preview Environment Status\n\n**PR:** #%d\n\n### 🟢 Active Preview Environments\n\n", cmd.PRNumber))

	var enrichedPreviews []map[string]interface{}

	for _, ns := range previewNamespaces {
		namespaceName := ns["name"].(string)
		serviceName := ns["service"].(string)

		contentBuilder.WriteString(fmt.Sprintf("#### %s\n- **Namespace:** `%s`\n- **Service:** %s\n- **Created:** %s\n\n", serviceName, namespaceName, serviceName, ns["created_at"]))

		// Get deployment status if exists
		deploymentStatus, err := cs.k8s.GetDeploymentStatus(ctx, namespaceName, serviceName)
		if err == nil {
			contentBuilder.WriteString(fmt.Sprintf("- **Deployment Status:** %d/%d pods ready\n- **Pods:** %d total\n", deploymentStatus["ready_replicas"], deploymentStatus["replicas"], len(deploymentStatus["pods"].([]map[string]interface{}))))

			// Add deployment info to preview data
			enrichedPreview := make(map[string]interface{})
			for k, v := range ns {
				enrichedPreview[k] = v
			}
			enrichedPreview["deployment_status"] = deploymentStatus
			enrichedPreviews = append(enrichedPreviews, enrichedPreview)
		} else {
			contentBuilder.WriteString("- **Deployment Status:** No deployment found\n")
			enrichedPreviews = append(enrichedPreviews, ns)
		}

		// Get service info if exists
		serviceInfo, err := cs.k8s.GetServiceInfo(ctx, namespaceName, serviceName)
		if err == nil {
			contentBuilder.WriteString(fmt.Sprintf("- **Service IP:** %s\n- **Service Ports:** %v\n\n", serviceInfo["cluster_ip"], serviceInfo["ports"]))
		} else {
			contentBuilder.WriteString("- **Service:** Not found\n\n")
		}
	}

	contentBuilder.WriteString(fmt.Sprintf("*Status checked by: @%s*", cmd.User))

	return &types.CommandResponse{
		Success: true,
		Message: "Preview environment status",
		Content: contentBuilder.String(),
		Data: map[string]interface{}{
			"pr_number":       cmd.PRNumber,
			"active_previews": enrichedPreviews,
			"total_previews":  len(previewNamespaces),
		},
	}
}

// Enhanced preview command with real K8s deployment including pods
func (cs *CommandServiceK8s) HandlePreviewK8s(ctx context.Context, cmd *types.Command) *types.CommandResponse {
	serviceName := cmd.Service
	if serviceName == "" {
		serviceName = "nginx-test" // Default test service
	}

	// Clean service name for K8s compatibility
	cleanServiceName := strings.ReplaceAll(serviceName, "/", "-")

	// Generate namespace name
	namespaceName := fmt.Sprintf("preview-pr-%d-%s", cmd.PRNumber, cleanServiceName)

	// Step 1: Create namespace
	err := cs.k8s.CreateNamespace(ctx, namespaceName, cmd.PRNumber, serviceName)
	if err != nil {
		return &types.CommandResponse{
			Success: false,
			Message: "Preview deployment failed",
			Content: fmt.Sprintf("## ❌ Preview Deployment Failed\n\n**Error:** %s\n\n**Service:** %s\n**Namespace:** %s\n\n*Please check cluster permissions and try again.*", err.Error(), serviceName, namespaceName),
		}
	}

	// Step 2: Deploy pod
	err = cs.k8s.DeployTestPod(ctx, namespaceName, cleanServiceName)
	if err != nil {
		return &types.CommandResponse{
			Success: false,
			Message: "Pod deployment failed",
			Content: fmt.Sprintf("## ❌ Pod Deployment Failed\n\n**Error:** %s\n\n**Service:** %s\n**Namespace:** %s\n\n*Namespace created but pod deployment failed.*", err.Error(), serviceName, namespaceName),
		}
	}

	// Step 3: Create service
	err = cs.k8s.CreateService(ctx, namespaceName, cleanServiceName)
	if err != nil {
		return &types.CommandResponse{
			Success: false,
			Message: "Service creation failed",
			Content: fmt.Sprintf("## ❌ Service Creation Failed\n\n**Error:** %s\n\n**Service:** %s\n**Namespace:** %s\n\n*Pod deployed but service creation failed.*", err.Error(), serviceName, namespaceName),
		}
	}

	// Step 4: Wait for deployment (non-blocking)
	go func() {
		cs.k8s.WaitForDeployment(ctx, namespaceName, cleanServiceName, 3)
	}()

	return &types.CommandResponse{
		Success: true,
		Message: "Preview deployment started",
		Content: fmt.Sprintf("## 🚀 Preview Deployment Started\n\n**👤 Triggered by:** @%s\n**🎯 Service:** %s\n**🔗 PR:** #%d\n**📦 Namespace:** `%s`\n\n### 📋 Deployment Status\n- ✅ Namespace created successfully\n- ✅ Pod deployment initiated (nginx:alpine)\n- ✅ Service created for pod exposure\n- 🔄 Pod startup in progress...\n\n### 📊 Resources Created\n- **Deployment:** `%s`\n- **Service:** `%s` (ClusterIP)\n- **Labels:** preview=true, pr-number=%d\n\n**Estimated ready time:** 30-60 seconds\n\n*Use `/status` to check deployment progress*",
			cmd.User, serviceName, cmd.PRNumber, namespaceName,
			cleanServiceName, cleanServiceName, cmd.PRNumber),
		Data: map[string]interface{}{
			"service":            serviceName,
			"clean_service_name": cleanServiceName,
			"namespace":          namespaceName,
			"pr_number":          cmd.PRNumber,
			"status":             "deploying",
		},
	}
}

// Enhanced cleanup command with real K8s cleanup
func (cs *CommandServiceK8s) HandleCleanupK8s(ctx context.Context, cmd *types.Command) *types.CommandResponse {
	// Get existing namespaces first
	previewNamespaces, err := cs.k8s.GetPreviewNamespacesByPR(ctx, cmd.PRNumber)
	if err != nil {
		return &types.CommandResponse{
			Success: false,
			Message: "Cleanup failed",
			Content: fmt.Sprintf("❌ Error getting preview namespaces: %s", err.Error()),
		}
	}

	if len(previewNamespaces) == 0 {
		return &types.CommandResponse{
			Success: true,
			Message: "Nothing to cleanup",
			Content: fmt.Sprintf("## ℹ️ Manual Cleanup - Nothing to Clean\n\nNo preview environments were found for PR #%d.\n\nAll preview resources appear to already be cleaned up.\n\n*Cleanup triggered by: @%s*", cmd.PRNumber, cmd.User),
		}
	}

	// Perform cleanup
	err = cs.k8s.CleanupPreviewNamespaces(ctx, cmd.PRNumber)
	if err != nil {
		return &types.CommandResponse{
			Success: false,
			Message: "Cleanup failed",
			Content: fmt.Sprintf("## ❌ Cleanup Failed\n\n**Error:** %s\n\n**PR:** #%d\n\n*Please check cluster permissions and try again.*", err.Error(), cmd.PRNumber),
		}
	}

	// Build cleanup summary
	var namespaceNames []string
	for _, ns := range previewNamespaces {
		if name, ok := ns["name"].(string); ok {
			namespaceNames = append(namespaceNames, name)
		}
	}

	return &types.CommandResponse{
		Success: true,
		Message: "Cleanup completed",
		Content: fmt.Sprintf("## 🧹 Manual Cleanup Completed\n\nSuccessfully cleaned up preview environments for PR #%d:\n\n%s\n### 📋 Resources Cleaned Up\n- ✅ Namespaces deleted (%d total)\n- ✅ Deployments and pods removed\n- ✅ Services and endpoints cleaned up\n- ✅ Labels and annotations removed\n\n*Cleanup triggered by: @%s*", cmd.PRNumber, formatNamespaceList(namespaceNames), len(namespaceNames), cmd.User),
		Data: map[string]interface{}{
			"pr_number":          cmd.PRNumber,
			"cleaned_namespaces": namespaceNames,
			"total_cleaned":      len(namespaceNames),
		},
	}
}

func formatNamespaceList(names []string) string {
	var result strings.Builder
	for _, name := range names {
		result.WriteString(fmt.Sprintf("- `%s`\n", name))
	}
	return result.String()
}

func (cs *CommandServiceK8s) GetAvailableServicesWithManifest(repoPath string) []string {
	services := []string{"nginx (default)"}

	// Scan for manifest files
	manifestServices := cs.scanForManifestServices(repoPath)
	services = append(services, manifestServices...)

	return services
}

func (cs *CommandServiceK8s) scanForManifestServices(repoPath string) []string {
	var manifestServices []string

	// Define scan paths
	scanPaths := []string{
		"k8s/",
		"kubernetes/",
		"manifests/",
		"deploy/",
	}

	for _, scanPath := range scanPaths {
		fullScanPath := filepath.Join(repoPath, scanPath)

		// Check if directory exists
		if _, err := os.Stat(fullScanPath); os.IsNotExist(err) {
			continue
		}

		// Scan directory for YAML files
		files, err := filepath.Glob(filepath.Join(fullScanPath, "*.yaml"))
		if err != nil {
			continue
		}

		yamlFiles, err := filepath.Glob(filepath.Join(fullScanPath, "*.yml"))
		if err == nil {
			files = append(files, yamlFiles...)
		}

		for _, file := range files {
			serviceName := cs.extractServiceNameFromPath(file)
			if serviceName != "" {
				manifestServices = append(manifestServices, fmt.Sprintf("%s (manifest from %s)", serviceName, scanPath))
			}
		}
	}

	return manifestServices
}

func (cs *CommandServiceK8s) extractServiceNameFromPath(manifestPath string) string {
	fileName := filepath.Base(manifestPath)
	serviceName := strings.TrimSuffix(fileName, filepath.Ext(fileName))

	// Clean up common generic names
	if serviceName == "deployment" || serviceName == "service" || serviceName == "app" {
		// Use directory name instead
		dir := filepath.Dir(manifestPath)
		dirName := filepath.Base(dir)
		if dirName != "." && dirName != "/" {
			return dirName
		}
	}

	return serviceName
}

func (cs *CommandServiceK8s) isManifestBasedService(serviceName, repoPath string) bool {
	// Check if service has corresponding manifest files
	manifestPaths := []string{
		fmt.Sprintf("k8s/%s.yaml", serviceName),
		fmt.Sprintf("k8s/%s.yml", serviceName),
		fmt.Sprintf("k8s/%s-deployment.yaml", serviceName),
		fmt.Sprintf("kubernetes/%s.yaml", serviceName),
		fmt.Sprintf("manifests/%s.yaml", serviceName),
		fmt.Sprintf("deploy/%s.yaml", serviceName),
	}

	for _, manifestPath := range manifestPaths {
		fullPath := filepath.Join(repoPath, manifestPath)
		if _, err := os.Stat(fullPath); err == nil {
			return true
		}
	}

	return false
}

func (cs *CommandServiceK8s) getManifestPath(serviceName, repoPath string) string {
	manifestPaths := []string{
		fmt.Sprintf("k8s/%s.yaml", serviceName),
		fmt.Sprintf("k8s/%s.yml", serviceName),
		fmt.Sprintf("kubernetes/%s.yaml", serviceName),
		fmt.Sprintf("manifests/%s.yaml", serviceName),
	}

	for _, manifestPath := range manifestPaths {
		fullPath := filepath.Join(repoPath, manifestPath)
		if _, err := os.Stat(fullPath); err == nil {
			return fullPath
		}
	}

	return ""
}

// Enhanced preview command with manifest awareness

func (cs *CommandServiceK8s) HandlePreviewK8sEnhanced(ctx context.Context, cmd *types.Command, repoPath string) *types.CommandResponse {
	serviceName := cmd.Service
	if serviceName == "" {
		serviceName = "nginx" // Default
	}

	// Check if service is manifest-based
	isManifest := cs.isManifestBasedService(serviceName, repoPath)
	manifestPath := ""
	deploymentMethod := "default (nginx:alpine)"

	if isManifest {
		manifestPath = cs.getManifestPath(serviceName, repoPath)
		deploymentMethod = "manifest-deployment"
	}

	// Show available services if service not found (except default nginx)
	if serviceName != "nginx" && !isManifest {
		availableServices := cs.GetAvailableServicesWithManifest(repoPath)
		return &types.CommandResponse{
			Success: false,
			Message: "Service not found",
			Content: fmt.Sprintf("## ❌ Service Not Found\n\n**Service:** `%s`\n\n**Available services:**\n%s\n\n**Usage Examples:**\n- `/preview` - Deploy nginx (default)\n- `/preview myapp` - Deploy from k8s/myapp.yaml\n- `/preview frontend` - Deploy from k8s/frontend.yaml\n\n**To add new services:**\nCreate YAML manifest files in `k8s/`, `kubernetes/`, `manifests/`, or `deploy/` folders.",
				serviceName, formatAvailableServicesList(availableServices)),
		}
	}

	// Create namespace
	cleanServiceName := strings.ReplaceAll(serviceName, "/", "-")
	namespaceName := fmt.Sprintf("preview-pr-%d-%s", cmd.PRNumber, cleanServiceName)

	// Step 1: Create namespace
	err := cs.k8s.CreateNamespace(ctx, namespaceName, cmd.PRNumber, serviceName)
	if err != nil {
		return &types.CommandResponse{
			Success: false,
			Message: "Preview deployment failed",
			Content: fmt.Sprintf("## ❌ Preview Deployment Failed\n\n**Error:** %s", err.Error()),
		}
	}

	// Step 2: Deploy based on method
	var deployedResources []string

	if isManifest {
		// Parse and deploy from manifest
		parser := NewManifestParser()
		parsed, err := parser.ParseManifestFile(manifestPath)
		if err != nil {
			return &types.CommandResponse{
				Success: false,
				Message: "Manifest parsing failed",
				Content: fmt.Sprintf("## ❌ Manifest Parsing Failed\n\n**Error:** %s\n\n**Manifest File:** %s", err.Error(), manifestPath),
			}
		}

		// Deploy from parsed manifest
		err = cs.k8s.DeployFromParsedManifest(ctx, namespaceName, parsed)
		if err != nil {
			return &types.CommandResponse{
				Success: false,
				Message: "Manifest deployment failed",
				Content: fmt.Sprintf("## ❌ Manifest Deployment Failed\n\n**Error:** %s\n\n**Manifest File:** %s", err.Error(), manifestPath),
			}
		}

		// Build deployed resources list
		for _, dep := range parsed.Deployments {
			deployedResources = append(deployedResources, fmt.Sprintf("Deployment/%s", dep.Name))
		}
		for _, svc := range parsed.Services {
			deployedResources = append(deployedResources, fmt.Sprintf("Service/%s", svc.Name))
		}
		for _, cm := range parsed.ConfigMaps {
			deployedResources = append(deployedResources, fmt.Sprintf("ConfigMap/%s", cm.Name))
		}

	} else {
		// Regular nginx deployment
		err = cs.k8s.DeployTestPod(ctx, namespaceName, cleanServiceName)
		if err != nil {
			return &types.CommandResponse{
				Success: false,
				Message: "Pod deployment failed",
				Content: fmt.Sprintf("## ❌ Pod Deployment Failed\n\n**Error:** %s", err.Error()),
			}
		}

		err = cs.k8s.CreateService(ctx, namespaceName, cleanServiceName)
		if err != nil {
			return &types.CommandResponse{
				Success: false,
				Message: "Service creation failed",
				Content: fmt.Sprintf("## ❌ Service Creation Failed\n\n**Error:** %s", err.Error()),
			}
		}

		deployedResources = []string{
			fmt.Sprintf("Deployment/%s", cleanServiceName),
			fmt.Sprintf("Service/%s", cleanServiceName),
		}
	}

	// Build success response
	var manifestNote string
	var resourcesList string

	if isManifest {
		manifestNote = fmt.Sprintf("\n\n🎯 **Manifest Deployed:** Successfully deployed from `%s`\n📋 **Real Deployment:** Resources deployed directly from your manifest!", manifestPath)
		resourcesList = strings.Join(deployedResources, ", ")
	} else {
		resourcesList = strings.Join(deployedResources, ", ")
	}

	return &types.CommandResponse{
		Success: true,
		Message: "Preview deployment started",
		Content: fmt.Sprintf("## 🚀 Preview Deployment Started\n\n**👤 Triggered by:** @%s\n**🎯 Service:** %s\n**📄 Method:** %s\n**🔗 PR:** #%d\n**📦 Namespace:** `%s`\n\n### 📋 Deployment Status\n- ✅ Namespace created successfully\n- ✅ Resources deployed: %s\n- 🔄 Pod startup in progress...\n\n### 📊 Resources Created\n%s\n\n**Estimated ready time:** 30-60 seconds%s",
			cmd.User, serviceName, deploymentMethod, cmd.PRNumber, namespaceName,
			resourcesList, cs.formatResourcesList(deployedResources), manifestNote),
		Data: map[string]interface{}{
			"service":            serviceName,
			"clean_service_name": cleanServiceName,
			"namespace":          namespaceName,
			"deployment_method":  deploymentMethod,
			"manifest_detected":  isManifest,
			"manifest_path":      manifestPath,
			"deployed_resources": deployedResources,
			"pr_number":          cmd.PRNumber,
			"status":             "deploying",
		},
	}
}

// Helper function for formatting service list
func formatAvailableServicesList(services []string) string {
	var result strings.Builder
	for _, service := range services {
		result.WriteString(fmt.Sprintf("- `%s`\n", service))
	}
	return result.String()
}

// Helper method for formatting resources list
func (cs *CommandServiceK8s) formatResourcesList(resources []string) string {
	var result strings.Builder
	for _, resource := range resources {
		result.WriteString(fmt.Sprintf("- **%s**\n", resource))
	}
	return result.String()
}
