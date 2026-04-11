package config

import (
	"log/slog"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port    string
	LogInfo bool
}

// LoadConfig reads the .env file and sets up the configuration.
func LoadConfig() *Config {
	// Load .env file if it exists, otherwise ignore error (useful for docker/env vars)
	_ = godotenv.Load()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	logInfoStr := os.Getenv("LOG_INFO")
	logInfo := logInfoStr == "true"

	return &Config{
		Port:    port,
		LogInfo: logInfo,
	}
}

// SetupLogger initializes a centralized logger using log/slog.
// It sets the default level based on the LogInfo configuration.
func SetupLogger(cfg *Config) {
	var level slog.Level
	if cfg.LogInfo {
		// LogInfo=true -> Show INFO and DEBUG
		level = slog.LevelDebug
	} else {
		// LogInfo=false -> Show WARNING and ERROR only
		level = slog.LevelWarn
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}
	
	handler := slog.NewTextHandler(os.Stdout, opts)
	logger := slog.New(handler)
	slog.SetDefault(logger)
}
