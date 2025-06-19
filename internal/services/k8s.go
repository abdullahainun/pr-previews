package services

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type K8sService struct {
	client kubernetes.Interface
}

func NewK8sService() (*K8sService, error) {
	config, err := getK8sConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get K8s config: %v", err)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create K8s client: %v", err)
	}

	return &K8sService{
		client: client,
	}, nil
}

func getK8sConfig() (*rest.Config, error) {
	// Try in-cluster config first
	if config, err := rest.InClusterConfig(); err == nil {
		return config, nil
	}

	// Try kubeconfig file
	kubeconfig := filepath.Join(homedir.HomeDir(), ".kube", "config")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build kubeconfig: %v", err)
	}

	return config, nil
}

// TestConnection tests K8s cluster connectivity
func (k *K8sService) TestConnection(ctx context.Context) error {
	_, err := k.client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		return fmt.Errorf("failed to connect to K8s cluster: %v", err)
	}
	return nil
}

// GetClusterInfo returns basic cluster information
func (k *K8sService) GetClusterInfo(ctx context.Context) (map[string]interface{}, error) {
	nodes, err := k.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes: %v", err)
	}

	namespaces, err := k.client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get namespaces: %v", err)
	}

	previewNamespaces, err := k.client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{
		LabelSelector: "preview=true",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get preview namespaces: %v", err)
	}

	info := map[string]interface{}{
		"nodes_count":        len(nodes.Items),
		"namespaces_count":   len(namespaces.Items),
		"preview_namespaces": len(previewNamespaces.Items),
		"connection_status":  "connected",
		"server_version":     "TODO",
	}

	return info, nil
}

// CreateNamespace creates a preview namespace with proper labels
func (k *K8sService) CreateNamespace(ctx context.Context, name string, prNumber int, service string) error {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"preview":     "true",
				"pr-number":   fmt.Sprintf("%d", prNumber),
				"service":     service,
				"created-by":  "pr-previews",
				"environment": "preview",
			},
			Annotations: map[string]string{
				"pr-previews.io/created-at": time.Now().Format(time.RFC3339),
				"pr-previews.io/pr-number":  fmt.Sprintf("%d", prNumber),
				"pr-previews.io/service":    service,
			},
		},
	}

	_, err := k.client.CoreV1().Namespaces().Create(ctx, namespace, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create namespace %s: %v", name, err)
	}

	return nil
}

// DeleteNamespace deletes a preview namespace
func (k *K8sService) DeleteNamespace(ctx context.Context, name string) error {
	err := k.client.CoreV1().Namespaces().Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete namespace %s: %v", name, err)
	}
	return nil
}

// ListPreviewNamespaces lists all preview namespaces
func (k *K8sService) ListPreviewNamespaces(ctx context.Context) ([]map[string]interface{}, error) {
	namespaces, err := k.client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{
		LabelSelector: "preview=true",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list preview namespaces: %v", err)
	}

	var result []map[string]interface{}
	for _, ns := range namespaces.Items {
		info := map[string]interface{}{
			"name":       ns.Name,
			"pr_number":  ns.Labels["pr-number"],
			"service":    ns.Labels["service"],
			"created_at": ns.CreationTimestamp.Format(time.RFC3339),
			"status":     string(ns.Status.Phase),
		}
		result = append(result, info)
	}

	return result, nil
}

// GetPreviewNamespacesByPR gets preview namespaces for specific PR
func (k *K8sService) GetPreviewNamespacesByPR(ctx context.Context, prNumber int) ([]map[string]interface{}, error) {
	namespaces, err := k.client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("preview=true,pr-number=%d", prNumber),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list PR %d preview namespaces: %v", prNumber, err)
	}

	var result []map[string]interface{}
	for _, ns := range namespaces.Items {
		info := map[string]interface{}{
			"name":       ns.Name,
			"service":    ns.Labels["service"],
			"created_at": ns.CreationTimestamp.Format(time.RFC3339),
			"status":     string(ns.Status.Phase),
		}
		result = append(result, info)
	}

	return result, nil
}

// CleanupPreviewNamespaces deletes all preview namespaces for a PR
func (k *K8sService) CleanupPreviewNamespaces(ctx context.Context, prNumber int) error {
	namespaces, err := k.client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("preview=true,pr-number=%d", prNumber),
	})
	if err != nil {
		return fmt.Errorf("failed to list PR %d namespaces for cleanup: %v", prNumber, err)
	}

	for _, ns := range namespaces.Items {
		err := k.client.CoreV1().Namespaces().Delete(ctx, ns.Name, metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("failed to delete namespace %s: %v", ns.Name, err)
		}
	}

	return nil
}

// DeployTestPod deploys a simple nginx pod for testing
func (k *K8sService) DeployTestPod(ctx context.Context, namespace, serviceName string) error {
	// Create deployment
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: namespace,
			Labels: map[string]string{
				"app":                serviceName,
				"managed-by":         "pr-previews",
				"preview-deployment": "true",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": serviceName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":                serviceName,
						"preview-deployment": "true",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  serviceName,
							Image: "nginx:alpine",
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 80,
									Name:          "http",
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("200m"),
									corev1.ResourceMemory: resource.MustParse("256Mi"),
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/",
										Port: intstr.FromInt(80),
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       10,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/",
										Port: intstr.FromInt(80),
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       5,
							},
						},
					},
				},
			},
		},
	}

	_, err := k.client.AppsV1().Deployments(namespace).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create deployment: %v", err)
	}

	return nil
}

// CreateService creates a Kubernetes service for the deployment
func (k *K8sService) CreateService(ctx context.Context, namespace, serviceName string) error {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: namespace,
			Labels: map[string]string{
				"app":        serviceName,
				"managed-by": "pr-previews",
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": serviceName,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       80,
					TargetPort: intstr.FromInt(80),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}

	_, err := k.client.CoreV1().Services(namespace).Create(ctx, service, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create service: %v", err)
	}

	return nil
}

// WaitForDeployment waits for deployment to be ready
func (k *K8sService) WaitForDeployment(ctx context.Context, namespace, deploymentName string, timeoutMinutes int) error {
	timeout := time.Duration(timeoutMinutes) * time.Minute
	return wait.PollImmediate(10*time.Second, timeout, func() (bool, error) {
		deployment, err := k.client.AppsV1().Deployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		// Check if deployment is ready
		if deployment.Status.ReadyReplicas == deployment.Status.Replicas && deployment.Status.Replicas > 0 {
			return true, nil
		}

		return false, nil
	})
}

// GetDeploymentStatus gets current status of deployment
func (k *K8sService) GetDeploymentStatus(ctx context.Context, namespace, deploymentName string) (map[string]interface{}, error) {
	deployment, err := k.client.AppsV1().Deployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment: %v", err)
	}

	// Get pods
	pods, err := k.client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app=%s", deploymentName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get pods: %v", err)
	}

	var podStatuses []map[string]interface{}
	for _, pod := range pods.Items {
		podStatus := map[string]interface{}{
			"name":   pod.Name,
			"status": string(pod.Status.Phase),
			"ready":  false,
		}

		// Check if pod is ready
		for _, condition := range pod.Status.Conditions {
			if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
				podStatus["ready"] = true
				break
			}
		}

		podStatuses = append(podStatuses, podStatus)
	}

	status := map[string]interface{}{
		"name":               deployment.Name,
		"namespace":          deployment.Namespace,
		"replicas":           deployment.Status.Replicas,
		"ready_replicas":     deployment.Status.ReadyReplicas,
		"available_replicas": deployment.Status.AvailableReplicas,
		"conditions":         deployment.Status.Conditions,
		"pods":               podStatuses,
		"created_at":         deployment.CreationTimestamp.Format(time.RFC3339),
	}

	return status, nil
}

// GetServiceInfo gets service information
func (k *K8sService) GetServiceInfo(ctx context.Context, namespace, serviceName string) (map[string]interface{}, error) {
	service, err := k.client.CoreV1().Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get service: %v", err)
	}

	info := map[string]interface{}{
		"name":       service.Name,
		"namespace":  service.Namespace,
		"cluster_ip": service.Spec.ClusterIP,
		"ports":      service.Spec.Ports,
		"type":       string(service.Spec.Type),
		"created_at": service.CreationTimestamp.Format(time.RFC3339),
	}

	return info, nil
}

// Helper function for int32 pointer
func int32Ptr(i int32) *int32 { return &i }
