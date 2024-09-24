package config

import (
	"time"
)

type Config struct {
	MimirRuleRequeuePeriod time.Duration
	AlertingBurnRates      AlertingBurnRates
	DefaultBaseWindow      time.Duration
	AlertSeverities        AlertSeverities
}

type AlertingBurnRates struct {
	PageShortWindow   float64
	PageLongWindow    float64
	TicketShortWindow float64
	TicketLongWindow  float64
}

type AlertSeverities struct {
	Critical string
	HighFast string
	HighSlow string
	Low      string
}
