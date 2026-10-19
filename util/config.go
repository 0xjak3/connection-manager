package util

import (
	"errors"
	"os"
)

type Config struct {
	HTTPAddr    string
	DatabaseURL string
}

// LoadConfig reads configuration from the environment.
// HTTP_ADDR defaults to ":8080"; DATABASE_URL is required.
func LoadConfig() (Config, error) {
	cfg := Config{
		HTTPAddr:    os.Getenv("HTTP_ADDR"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
	}
	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = ":8080"
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("config: DATABASE_URL is required")
	}
	return cfg, nil
}
