package config

import (
	"time"
)

type Config struct {
	MimirRuleRequeuePeriod time.Duration
	AlertingBurnRates      AlertingBurnRates
	DefaultBaseWindow      time.Duration
	AlertingTool           string
	AlertSeverities        AlertSeverities
}

type AlertingBurnRates struct {
	PageShortWindow   float64
	PageLongWindow    float64
	TicketShortWindow float64
	TicketLongWindow  float64
}

type AlertToolConfig struct {
	Tool       string
	Severities map[string]string
}

type SREAlertSeverity string

const (
	PageCritical SREAlertSeverity = "page_critical"
	PageHigh     SREAlertSeverity = "page_high"
	TicketHigh   SREAlertSeverity = "ticket_high"
	TicketMedium SREAlertSeverity = "ticket_medium"
)

type AlertToolSeverityMap map[SREAlertSeverity]string

type AlertSeverities struct {
	Critical string
	HighFast string
	HighSlow string
	Low      string
	Tool     string
}

func (m AlertToolSeverityMap) GetSeverity(sreSeverity SREAlertSeverity) string {
	if sev, ok := m[sreSeverity]; ok {
		return sev
	}
	return m[TicketMedium] // default to lowest severity
}
