package config

import "os"

type Config struct {
	Server struct {
		Port string
		Host string
	}
	GitHub struct {
		WebhookSecret string
		Token         string
		CoreTeam      []string
	}
}

func Load() *Config {
	cfg := &Config{}
	cfg.Server.Host = getEnv("SERVER_HOST", "0.0.0.0")
	cfg.Server.Port = getEnv("SERVER_PORT", "8080")
	cfg.GitHub.WebhookSecret = getEnv("GITHUB_WEBHOOK_SECRET", "")
	cfg.GitHub.Token = getEnv("GITHUB_TOKEN", "")
	cfg.GitHub.CoreTeam = []string{"abdullahainun"}
	return cfg
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
