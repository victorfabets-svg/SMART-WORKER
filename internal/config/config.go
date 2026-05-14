package config

import (
	"os"
)

type Config struct {
	DatabaseURL   string
	SupabaseURL   string
	SupabaseAnonKey string
	SupabaseServiceRole string
	OpenAIAPIKey  string
	OpenHandsAPIKey string
	OpenHandsBaseURL string
	LogLevel     string
}

func Load() (*Config, error) {
	openhandsCfg, _ := LoadOpenHands()
	
	return &Config{
		DatabaseURL:         getEnv("DATABASE_URL", ""),
		SupabaseURL:         getEnv("SUPABASE_URL", ""),
		SupabaseAnonKey:     getEnv("SUPABASE_ANON_KEY", ""),
		SupabaseServiceRole: getEnv("SUPABASE_SERVICE_ROLE", ""),
		OpenAIAPIKey:        getEnv("OPENAI_API_KEY", ""),
		OpenHandsAPIKey:     getEnv("OPENHANDS_API_KEY", ""),
		OpenHandsBaseURL:    getEnv("OPENHANDS_BASE_URL", "https://app.all-hands.dev"),
		LogLevel:            getEnv("LOG_LEVEL", "info"),
	}, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}