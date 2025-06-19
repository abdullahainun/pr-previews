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

	// Setup routes - Accept both GET and POST for testing
	r.GET("/health", h.Health)
	r.GET("/metrics", h.Metrics)
	r.GET("/webhook/github", h.GitHubWebhook)  // For testing with query params
	r.POST("/webhook/github", h.GitHubWebhook) // For real GitHub webhooks

	// Start server
	fmt.Printf("🚀 pr-previews server starting on port %s\n", cfg.Server.Port)
	fmt.Printf("📊 Health: http://localhost:%s/health\n", cfg.Server.Port)
	fmt.Printf("🪝 Webhook: http://localhost:%s/webhook/github\n", cfg.Server.Port)
	fmt.Printf("🧪 Test: http://localhost:%s/webhook/github?comment=/help&user=testuser\n", cfg.Server.Port)

	// Graceful shutdown
	go func() {
		r.Run(":" + cfg.Server.Port)
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("\n✅ Server shut down gracefully")
}
