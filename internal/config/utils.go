package config

import (
	"os"
	"strconv"
	"time"
)

// Helper function to read an environment variable or return a default value
func GetEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// Helper function to read an environment variable as an integer or return a default value
func GetEnvAsInt(key string, defaultValue int) int {
	if valueStr, exists := os.LookupEnv(key); exists {
		if value, err := strconv.Atoi(valueStr); err == nil {
			return value
		}
	}
	return defaultValue
}

// Helper function to read an environment variable as a float64 or return a default value
func GetEnvAsFloat64(key string, defaultValue float64) float64 {
	if valueStr, exists := os.LookupEnv(key); exists {
		if value, err := strconv.ParseFloat(valueStr, 64); err == nil {
			return value
		}
	}
	return defaultValue
}

// Helper function to read an environment variable as a time.Duration or return a default value
func GetEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	if valueStr, exists := os.LookupEnv(key); exists {
		if value, err := time.ParseDuration(valueStr); err == nil {
			return value
		}
	}
	return defaultValue
}
