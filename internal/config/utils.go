package config

import (
	"os"
	"strconv"
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

func AlertSeveritiesByTool(tool string) AlertToolSeverityMap {
	severityMaps := map[string]AlertToolSeverityMap{
		"opsgenie": {
			PageCritical: "P1",
			PageHigh:     "P2",
			TicketHigh:   "P3",
			TicketMedium: "P4",
		},
		"pagerduty": {
			PageCritical: "SEV_1",
			PageHigh:     "SEV_2",
			TicketHigh:   "SEV_3",
			TicketMedium: "SEV_4",
		},
		"custom": {
			PageCritical: GetEnv("OSKO_ALERTING_SEVERITY_CRITICAL", "critical"),
			PageHigh:     GetEnv("OSKO_ALERTING_SEVERITY_HIGH", "high"),
			TicketHigh:   GetEnv("OSKO_ALERTING_SEVERITY_HIGH", "medium"),
			TicketMedium: GetEnv("OSKO_ALERTING_SEVERITY_LOW", "low"),
		},
	}

	if toolMap, exists := severityMaps[tool]; exists {
		return toolMap
	}

	return severityMaps["custom"]
}
