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
	}
	return config
}
