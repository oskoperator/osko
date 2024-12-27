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
			PageShortWindow:   GetEnvAsFloat64("ABR_PAGE_SHORT_WINDOW", 14.4),
			PageLongWindow:    GetEnvAsFloat64("ABR_PAGE_LONG_WINDOW", 6),
			TicketShortWindow: GetEnvAsFloat64("ABR_TICKET_SHORT_WINDOW", 3),
			TicketLongWindow:  GetEnvAsFloat64("ABR_TICKET_LONG_WINDOW", 1),
		},
		DefaultBaseWindow: GetEnvAsDuration("DEFAULT_BASE_WINDOW", 5*time.Minute),
		AlertingTool:      alertingTool,
		// AlertSeverities:   AlertSeveritiesByTool(alertingTool), // I wouldn't default to opsgenie here, maybe better to default to custom and error on startup if no custom variables or valid tool is selected
	}
}
