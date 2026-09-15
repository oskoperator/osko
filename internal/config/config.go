package config

import (
	"time"
)

var Cfg Config

func NewConfig() {
	alertingTool := GetEnv("OSKO_ALERTING_TOOL", "opsgenie")

	Cfg = Config{
		MimirRuleRequeuePeriod: GetEnvAsDuration("MIMIR_RULE_REQUEUE_PERIOD", 60*time.Second),
		AlertingBurnRates: AlertingBurnRates{
			PageCriticalBurnRate: GetEnvAsFloat64("ABR_PAGE_CRITICAL_BURN_RATE", 14.4),
			PageHighBurnRate:     GetEnvAsFloat64("ABR_PAGE_HIGH_BURN_RATE", 6),
			TicketHighBurnRate:   GetEnvAsFloat64("ABR_TICKET_HIGH_BURN_RATE", 3),
			TicketMediumBurnRate: GetEnvAsFloat64("ABR_TICKET_MEDIUM_BURN_RATE", 1),
		},
		DefaultBaseWindow: GetEnvAsDuration("DEFAULT_BASE_WINDOW", 5*time.Minute),
		AlertingTool:      alertingTool,
		// AlertSeverities:   AlertSeveritiesByTool(alertingTool), // I wouldn't default to opsgenie here, maybe better to default to custom and error on startup if no custom variables or valid tool is selected
	}
}
