package config

import (
	"time"
)

func NewConfig() Config {
	config := Config{
		MimirRuleRequeuePeriod: GetEnvAsDuration("MIMIR_RULE_REQUEUE_PERIOD", 60*time.Second),
		AlertingBurnRates: AlertingBurnRates{
			PageShortWindow:   GetEnvAsFloat64("ABR_PAGE_SHORT_WINDOW", 14.4),
			PageLongWindow:    GetEnvAsFloat64("ABR_PAGE_LONG_WINDOW", 6),
			TicketShortWindow: GetEnvAsFloat64("ABR_TICKET_SHORT_WINDOW", 3),
			TicketLongWindow:  GetEnvAsFloat64("ABR_TICKET_LONG_WINDOW", 1),
		},
		DefaultBaseWindow: GetEnvAsDuration("DEFAULT_BASE_WINDOW", 5*time.Minute),
		AlertingTools: AlertingTools{
			Opsgenie: AlertSeverities{
				P1: "P1",
				P2: "P2",
				P3: "P3",
				P4: "P4",
			},
			Pagerduty: AlertSeverities{
				P1: "SEV-1",
				P2: "SEV-2",
				P3: "SEV-3",
				P4: "SEV-4",
			},
			Custom: AlertSeverities{
				P1: GetEnv("OSKO_ALERTING_P1", "P1"),
				P2: GetEnv("OSKO_ALERTING_P2", "P2"),
				P3: GetEnv("OSKO_ALERTING_P3", "P3"),
				P4: GetEnv("OSKO_ALERTING_P4", "P4"),
			},
		},
	}
	return config
}
