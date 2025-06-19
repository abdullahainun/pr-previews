package services

import (
	"fmt"
	"io/ioutil"
	"strings"

	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
)

type ManifestParser struct {
	decoder runtime.Decoder
}

func NewManifestParser() *ManifestParser {
	codecFactory := serializer.NewCodecFactory(scheme.Scheme)
	decoder := codecFactory.UniversalDeserializer()

	return &ManifestParser{
		decoder: decoder,
	}
}

type ParsedManifest struct {
	Deployments []appsv1.Deployment `json:"deployments"`
	Services    []corev1.Service    `json:"services"`
	ConfigMaps  []corev1.ConfigMap  `json:"configmaps"`
}

func (mp *ManifestParser) ParseManifestFile(filePath string) (*ParsedManifest, error) {
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest file: %v", err)
	}

	parsed := &ParsedManifest{
		Deployments: []appsv1.Deployment{},
		Services:    []corev1.Service{},
		ConfigMaps:  []corev1.ConfigMap{},
	}

	// Split by --- for multi-document YAML
	documents := strings.Split(string(content), "---")

	for _, doc := range documents {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}

		err := mp.parseDocument(doc, parsed)
		if err != nil {
			// Log warning but continue parsing other documents
			fmt.Printf("Warning: failed to parse document in %s: %v\n", filePath, err)
			continue
		}
	}

	return parsed, nil
}

func (mp *ManifestParser) parseDocument(content string, parsed *ParsedManifest) error {
	// First parse as generic to check kind
	var obj map[string]interface{}
	err := yaml.Unmarshal([]byte(content), &obj)
	if err != nil {
		return fmt.Errorf("failed to parse YAML: %v", err)
	}

	kind, ok := obj["kind"].(string)
	if !ok {
		return fmt.Errorf("no kind specified")
	}

	// Parse based on kind
	switch kind {
	case "Deployment":
		var deployment appsv1.Deployment
		objRuntime, _, err := mp.decoder.Decode([]byte(content), nil, &deployment)
		if err != nil {
			return fmt.Errorf("failed to decode deployment: %v", err)
		}
		parsed.Deployments = append(parsed.Deployments, *objRuntime.(*appsv1.Deployment))

	case "Service":
		var service corev1.Service
		objRuntime, _, err := mp.decoder.Decode([]byte(content), nil, &service)
		if err != nil {
			return fmt.Errorf("failed to decode service: %v", err)
		}
		parsed.Services = append(parsed.Services, *objRuntime.(*corev1.Service))

	case "ConfigMap":
		var configMap corev1.ConfigMap
		objRuntime, _, err := mp.decoder.Decode([]byte(content), nil, &configMap)
		if err != nil {
			return fmt.Errorf("failed to decode configmap: %v", err)
		}
		parsed.ConfigMaps = append(parsed.ConfigMaps, *objRuntime.(*corev1.ConfigMap))

	default:
		// Skip unsupported resource types
		fmt.Printf("Skipping unsupported resource type: %s\n", kind)
	}

	return nil
}
