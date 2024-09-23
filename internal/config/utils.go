package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// GetEnv Helper function to read an environment variable or return a default value
func GetEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// GetEnvAsInt Helper function to read an environment variable as an integer or return a default value
func GetEnvAsInt(key string, defaultValue int) int {
	if valueStr, exists := os.LookupEnv(key); exists {
		if value, err := strconv.Atoi(valueStr); err == nil {
			return value
		}
	}
	return defaultValue
}

// GetEnvAsFloat64 Helper function to read an environment variable as a float64 or return a default value
func GetEnvAsFloat64(key string, defaultValue float64) float64 {
	if valueStr, exists := os.LookupEnv(key); exists {
		if value, err := strconv.ParseFloat(valueStr, 64); err == nil {
			return value
		}
	}
	return defaultValue
}

// GetEnvAsDuration Helper function to read an environment variable as a time.Duration or return a default value
func GetEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	if valueStr, exists := os.LookupEnv(key); exists {
		if value, err := time.ParseDuration(valueStr); err == nil {
			return value
		}
	}
	return defaultValue
}

func GetAlertingSeveritiesMap(tool AlertingTool) *AlertSeverities {
	switch strings.ToLower(tool.Name) {
	case "opsgenie":
		return &AlertSeverities{
			Critical: "P1",
			HighFast: "P2",
			HighSlow: "P3",
			Low:      "P4",
			NoSlo:    "P5",
		}

	case "pagerduty":
		return &AlertSeverities{
			Critical: "SEV-1",
			HighFast: "SEV-2",
			HighSlow: "SEV-3",
			Low:      "SEV-4",
			NoSlo:    "SEV-5",
		}
	}
	return &AlertSeverities{}
}
