package config

import "testing"

func TestAlertSeveritiesByTool(t *testing.T) {
	tests := []struct {
		name string
		tool string
		want AlertToolSeverityMap
	}{
		{
			name: "opsgenie",
			tool: "opsgenie",
			want: AlertToolSeverityMap{
				PageCritical: "P1", PageHigh: "P2", TicketHigh: "P3", TicketMedium: "P4",
			},
		},
		{
			name: "pagerduty",
			tool: "pagerduty",
			want: AlertToolSeverityMap{
				PageCritical: "SEV_1", PageHigh: "SEV_2", TicketHigh: "SEV_3", TicketMedium: "SEV_4",
			},
		},
		{
			name: "custom defaults",
			tool: "custom",
			want: AlertToolSeverityMap{
				PageCritical: "critical", PageHigh: "high", TicketHigh: "medium", TicketMedium: "low",
			},
		},
		{
			name: "an unknown tool falls back to custom rather than failing",
			tool: "pagrduty",
			want: AlertToolSeverityMap{
				PageCritical: "critical", PageHigh: "high", TicketHigh: "medium", TicketMedium: "low",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AlertSeveritiesByTool(tt.tool)
			for tier, want := range tt.want {
				if got[tier] != want {
					t.Errorf("%s[%s] = %q, want %q", tt.tool, tier, got[tier], want)
				}
			}
		})
	}
}

// TestAlertSeveritiesByTool_CustomTiersAreIndependent guards a collapse that breaks
// both routing and inhibition: every tier must read its own environment variable, so
// overriding one never silently changes another. Two tiers sharing a severity value
// makes them indistinguishable to Alertmanager matchers.
func TestAlertSeveritiesByTool_CustomTiersAreIndependent(t *testing.T) {
	t.Setenv("OSKO_ALERTING_SEVERITY_CRITICAL", "sev-crit")
	t.Setenv("OSKO_ALERTING_SEVERITY_HIGH", "sev-high")
	t.Setenv("OSKO_ALERTING_SEVERITY_MEDIUM", "sev-med")
	t.Setenv("OSKO_ALERTING_SEVERITY_LOW", "sev-low")

	got := AlertSeveritiesByTool("custom")

	want := AlertToolSeverityMap{
		PageCritical: "sev-crit",
		PageHigh:     "sev-high",
		TicketHigh:   "sev-med",
		TicketMedium: "sev-low",
	}
	for tier, w := range want {
		if got[tier] != w {
			t.Errorf("%s = %q, want %q", tier, got[tier], w)
		}
	}

	seen := map[string]SREAlertSeverity{}
	for tier, value := range got {
		if other, dup := seen[value]; dup {
			t.Errorf("%s and %s both resolve to %q; Alertmanager cannot route or inhibit between them",
				other, tier, value)
		}
		seen[value] = tier
	}
}
