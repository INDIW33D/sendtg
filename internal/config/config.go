package config

import (
	"fmt"
	"strconv"
)

// These variables are set at build time using -ldflags
var (
	apiID   string = "0"
	apiHash string = ""
)

// Config holds the application configuration
type Config struct {
	APIID   int
	APIHash string
}

// Load returns the embedded configuration
func Load() (*Config, error) {
	id, err := strconv.Atoi(apiID)
	if err != nil {
		return nil, fmt.Errorf("invalid API_ID: %w", err)
	}

	if id == 0 {
		return nil, fmt.Errorf("API_ID is not configured. Please rebuild with proper credentials")
	}

	if apiHash == "" {
		return nil, fmt.Errorf("API_HASH is not configured. Please rebuild with proper credentials")
	}

	return &Config{
		APIID:   id,
		APIHash: apiHash,
	}, nil
}
