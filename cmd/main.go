package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

func main() {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"service":   "pr-previews",
			"version":   "0.1.0",
			"timestamp": time.Now().Format(time.RFC3339),
		})
	})

	r.POST("/webhook/github", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "received",
			"message": "GitHub webhook received successfully",
		})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("🚀 pr-previews server starting on port %s\n", port)
	fmt.Printf("📊 Health: http://localhost:%s/health\n", port)

	r.Run(":" + port)
}
