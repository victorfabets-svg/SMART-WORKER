package config

import (
	"fmt"
	"os"
)

// OpenHandsConfig holds the OpenHands runtime configuration
type OpenHandsConfig struct {
	APIKey    string
	BaseURL   string
	IsValid  bool
}

// LoadOpenHands loads and validates the OpenHands configuration
func LoadOpenHands() (*OpenHandsConfig, error) {
	apiKey := os.Getenv("OPENHANDS_API_KEY")
	baseURL := getEnv("OPENHANDS_BASE_URL", "https://app.all-hands.dev")
	
	config := &OpenHandsConfig{
		APIKey:  apiKey,
		BaseURL: baseURL,
		IsValid: apiKey != "",
	}
	
	if !config.IsValid {
		return config, fmt.Errorf("OPENHANDS_API_KEY is not configured")
	}
	
	return config, nil
}

// Validate checks if the OpenHands configuration is valid
func (c *OpenHandsConfig) Validate() error {
	if c.APIKey == "" {
		return fmt.Errorf("OPENHANDS_API_KEY is not set")
	}
	if c.BaseURL == "" {
		return fmt.Errorf("OPENHANDS_BASE_URL is not set")
	}
	return nil
}

// GetMaskedKey returns a safely masked version of the API key for logging
func (c *OpenHandsConfig) GetMaskedKey() string {
	if len(c.APIKey) < 8 {
		return "****"
	}
	return c.APIKey[:4] + "****" + c.APIKey[len(c.APIKey)-4:]
}