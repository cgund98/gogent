package core

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

// Config holds the application configuration
type Config struct {
	OpenAIAPIKey string `envconfig:"OPENAI_API_KEY"`
}

// LoadConfig loads configuration from file, environment variables, or defaults
func LoadConfig() (*Config, error) {

	// Load .env.local file first (if it exists)
	// Lowest priority - will be overridden by .env and environment variables
	// Use godotenv to load as environment variables so standard naming (DATABASE_URL) works
	if _, err := os.Stat(".env.local"); err == nil {
		if err := godotenv.Load(".env.local"); err != nil {
			return nil, fmt.Errorf("error loading .env.local file: %w", err)
		}
	}

	// Load .env file (if it exists)
	// Higher priority - will override .env.local but be overridden by environment variables
	if _, err := os.Stat(".env"); err == nil {
		if err := godotenv.Load(".env"); err != nil {
			return nil, fmt.Errorf("error loading .env file: %w", err)
		}
	}

	var config Config
	if err := envconfig.Process("", &config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return &config, nil
}
