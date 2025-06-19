package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"pr-previews/internal/config"
	"pr-previews/internal/handlers"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Create router
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	// Initialize handlers
	h := handlers.New(cfg)

	// Setup routes
	r.GET("/health", h.Health)
	r.GET("/metrics", h.Metrics)
	r.GET("/webhook/github", h.GitHubWebhook)
	r.POST("/webhook/github", h.GitHubWebhook)
	r.GET("/test/k8s", h.TestK8s) // ← New K8s test endpoint

	// Start server
	fmt.Printf("🚀 pr-previews server starting on port %s\n", cfg.Server.Port)
	fmt.Printf("📊 Health: http://localhost:%s/health\n", cfg.Server.Port)
	fmt.Printf("🪝 Webhook: http://localhost:%s/webhook/github\n", cfg.Server.Port)
	fmt.Printf("☸️  K8s Test: http://localhost:%s/test/k8s\n", cfg.Server.Port)

	// Graceful shutdown
	go func() {
		r.Run(":" + cfg.Server.Port)
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("\n✅ Server shut down gracefully")
}
