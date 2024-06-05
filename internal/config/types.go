package config

import (
	"time"
)

type Config struct {
	MimirRuleRequeuePeriod time.Duration
	AlertingBurnRates      AlertingBurnRates
}

type AlertingBurnRates struct {
	PageShortWindow   float64
	PageLongWindow    float64
	TicketShortWindow float64
	TicketLongWindow  float64
}
