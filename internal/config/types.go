package config

import (
	"time"
)

type Config struct {
	MimirRuleRequeuePeriod time.Duration
	AlertingBurnRates      AlertingBurnRates
	DefaultBaseWindow      time.Duration
	AlertingTools          AlertingTools
}

type AlertingBurnRates struct {
	PageShortWindow   float64
	PageLongWindow    float64
	TicketShortWindow float64
	TicketLongWindow  float64
}

type AlertSeverities struct {
	P1 string
	P2 string
	P3 string
	P4 string
}

type AlertingTools struct {
	Opsgenie  AlertSeverities
	Pagerduty AlertSeverities
	Custom    AlertSeverities
}
