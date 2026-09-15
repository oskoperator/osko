package helpers

import (
	"context"
	stderrors "errors"
	"fmt"
	"strings"
	"testing"
	"time"

	openslov1 "github.com/oskoperator/osko/api/openslo/v1"
	"github.com/oskoperator/osko/internal/config"
	oskoerrors "github.com/oskoperator/osko/internal/errors"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/promql"
	"github.com/prometheus/prometheus/promql/parser"
	"github.com/prometheus/prometheus/util/teststorage"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	config.NewConfig()
}

const (
	recordingGroupName = "test-slo_recording"
	alertGroupName     = "test-slo_alert"
)

func TestValidateTarget(t *testing.T) {
	tests := []struct {
		name    string
		target  float64
		wantErr bool
	}{
		{"valid target 0.999", 0.999, false},
		{"valid target 0.99", 0.99, false},
		{"valid target 0.9", 0.9, false},
		{"valid target 0.5", 0.5, false},
		{"invalid target 1.0 (100%)", 1.0, true},
		{"invalid target 1.5", 1.5, true},
		{"invalid target 0", 0, true},
		{"invalid target -0.1", -0.1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTarget(tt.target)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateTarget(%v) error = %v, wantErr %v", tt.target, err, tt.wantErr)
			}
		})
	}
}

func TestParseTarget(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    float64
		wantErr bool
	}{
		{"valid 0.999", "0.999", 0.999, false},
		{"valid 0.99", "0.99", 0.99, false},
		{"valid 99.9 (percentage)", "99.9", 99.9, false},
		{"invalid string", "invalid", 0, true},
		{"empty string", "", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTarget(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseTarget(%v) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseTarget(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func createTestSLO(target string) *openslov1.SLO {
	return &openslov1.SLO{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slo",
			Namespace: "default",
		},
		Spec: openslov1.SLOSpec{
			Service:         "test-service",
			BudgetingMethod: "Occurrences",
			TimeWindow: []openslov1.TimeWindowSpec{
				{Duration: "28d", IsRolling: true},
			},
			Objectives: []openslov1.ObjectivesSpec{
				{Target: target},
			},
		},
	}
}

func createTestSLOWithAlerting(target string) *openslov1.SLO {
	slo := createTestSLO(target)
	slo.Annotations = map[string]string{
		"osko.dev/magicAlerting": "true",
	}
	return slo
}

func createTestSLI() *openslov1.SLI {
	return &openslov1.SLI{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sli",
			Namespace: "default",
		},
		Spec: openslov1.SLISpec{
			RatioMetric: openslov1.RatioMetricSpec{
				Counter: true,
				Total: openslov1.MetricSpec{
					MetricSource: openslov1.MetricSource{
						Type: "prometheus",
						Spec: openslov1.MetricSourceSpec{
							Query: "http_requests_total",
						},
					},
				},
				Good: openslov1.MetricSpec{
					MetricSource: openslov1.MetricSource{
						Type: "prometheus",
						Spec: openslov1.MetricSourceSpec{
							Query: "http_requests_success_total",
						},
					},
				},
			},
		},
	}
}

func createTestSLIGauge() *openslov1.SLI {
	return &openslov1.SLI{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sli-gauge",
			Namespace: "default",
		},
		Spec: openslov1.SLISpec{
			RatioMetric: openslov1.RatioMetricSpec{
				Counter: false,
				Total: openslov1.MetricSpec{
					MetricSource: openslov1.MetricSource{
						Type: "prometheus",
						Spec: openslov1.MetricSourceSpec{
							Query: "http_requests_total_gauge",
						},
					},
				},
				Good: openslov1.MetricSpec{
					MetricSource: openslov1.MetricSource{
						Type: "prometheus",
						Spec: openslov1.MetricSourceSpec{
							Query: "http_requests_success_total_gauge",
						},
					},
				},
			},
		},
	}
}

func createTestSLIWithBad() *openslov1.SLI {
	return &openslov1.SLI{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sli",
			Namespace: "default",
		},
		Spec: openslov1.SLISpec{
			RatioMetric: openslov1.RatioMetricSpec{
				Counter: true,
				Total: openslov1.MetricSpec{
					MetricSource: openslov1.MetricSource{
						Type: "prometheus",
						Spec: openslov1.MetricSourceSpec{
							Query: "http_requests_total",
						},
					},
				},
				Bad: openslov1.MetricSpec{
					MetricSource: openslov1.MetricSource{
						Type: "prometheus",
						Spec: openslov1.MetricSourceSpec{
							Query: "http_requests_error_total",
						},
					},
				},
			},
		},
	}
}

// --- test helpers -----------------------------------------------------------

func groupByName(t *testing.T, groups []monitoringv1.RuleGroup, name string) *monitoringv1.RuleGroup {
	t.Helper()
	for i, g := range groups {
		if g.Name == name {
			return &groups[i]
		}
	}
	t.Fatalf("rule group %q not found, got groups %v", name, groupNames(groups))
	return nil
}

// The alert group also carries the burn-rate recording rules the alerts read.
func alertRules(g *monitoringv1.RuleGroup) []monitoringv1.Rule {
	out := make([]monitoringv1.Rule, 0, len(g.Rules))
	for _, r := range g.Rules {
		if r.Alert != "" {
			out = append(out, r)
		}
	}
	return out
}

func groupNames(groups []monitoringv1.RuleGroup) []string {
	names := make([]string, 0, len(groups))
	for _, g := range groups {
		names = append(names, g.Name)
	}
	return names
}

func countRules(groups []monitoringv1.RuleGroup) int {
	total := 0
	for _, g := range groups {
		total += len(g.Rules)
	}
	return total
}

func allRules(groups []monitoringv1.RuleGroup) []monitoringv1.Rule {
	var rules []monitoringv1.Rule
	for _, g := range groups {
		rules = append(rules, g.Rules...)
	}
	return rules
}

func ruleFor(t *testing.T, groups []monitoringv1.RuleGroup, record, window string) monitoringv1.Rule {
	t.Helper()
	for _, r := range allRules(groups) {
		if r.Record == record && r.Labels["window"] == window {
			return r
		}
	}
	t.Fatalf("no rule with record %q and window %q found", record, window)
	return monitoringv1.Rule{}
}

func setupRules(t *testing.T, slo *openslov1.SLO, sli *openslov1.SLI, baseWindow string) []monitoringv1.RuleGroup {
	t.Helper()
	mrs := &MonitoringRuleSet{Slo: slo, Sli: sli, BaseWindow: baseWindow}
	groups, err := mrs.SetupRules()
	if err != nil {
		t.Fatalf("SetupRules() error = %v", err)
	}
	return groups
}

// --- tests ------------------------------------------------------------------

func TestSetupRules_TargetValidation(t *testing.T) {
	tests := []struct {
		name    string
		target  string
		wantErr bool
	}{
		{"valid target 0.999", "0.999", false},
		{"valid target 0.99", "0.99", false},
		{"invalid target 1.0", "1.0", true},
		{"invalid target 1", "1", true},
		{"invalid target 0", "0", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mrs := &MonitoringRuleSet{
				Slo:        createTestSLO(tt.target),
				Sli:        createTestSLI(),
				BaseWindow: "5m",
			}

			_, err := mrs.SetupRules()
			if (err != nil) != tt.wantErr {
				t.Errorf("SetupRules() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSetupRules_SpecValidation(t *testing.T) {
	thresholdSLI := func() *openslov1.SLI {
		sli := createTestSLI()
		sli.Spec.RatioMetric = openslov1.RatioMetricSpec{}
		sli.Spec.ThresholdMetric = openslov1.ThresholdMetricSpec{
			MetricSource: openslov1.MetricSource{
				Type: "prometheus",
				Spec: openslov1.MetricSourceSpec{Query: "http_request_duration_seconds"},
			},
		}
		return sli
	}

	noTotalSLI := func() *openslov1.SLI {
		sli := createTestSLI()
		sli.Spec.RatioMetric.Total.MetricSource.Spec.Query = ""
		return sli
	}

	noGoodNoBadSLI := func() *openslov1.SLI {
		sli := createTestSLI()
		sli.Spec.RatioMetric.Good.MetricSource.Spec.Query = ""
		sli.Spec.RatioMetric.Bad.MetricSource.Spec.Query = ""
		return sli
	}

	timeslicesSLO := func() *openslov1.SLO {
		slo := createTestSLO("0.999")
		slo.Spec.BudgetingMethod = "Timeslices"
		return slo
	}

	tests := []struct {
		name        string
		slo         *openslov1.SLO
		sli         *openslov1.SLI
		wantErrIs   error
		wantErrPart string
	}{
		{
			name:        "unsupported budgeting method",
			slo:         timeslicesSLO(),
			sli:         createTestSLI(),
			wantErrIs:   oskoerrors.ErrUnsupportedBudgetingMethod,
			wantErrPart: "Timeslices",
		},
		{
			name:        "threshold metric not supported",
			slo:         createTestSLO("0.999"),
			sli:         thresholdSLI(),
			wantErrIs:   oskoerrors.ErrInvalidSLIConfiguration,
			wantErrPart: "thresholdMetric",
		},
		{
			name:        "missing total query",
			slo:         createTestSLO("0.999"),
			sli:         noTotalSLI(),
			wantErrIs:   oskoerrors.ErrInvalidSLIConfiguration,
			wantErrPart: "ratioMetric.total",
		},
		{
			name:        "missing good and bad query",
			slo:         createTestSLO("0.999"),
			sli:         noGoodNoBadSLI(),
			wantErrIs:   oskoerrors.ErrInvalidSLIConfiguration,
			wantErrPart: "ratioMetric.bad",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mrs := &MonitoringRuleSet{Slo: tt.slo, Sli: tt.sli, BaseWindow: "5m"}
			_, err := mrs.SetupRules()
			if err == nil {
				t.Fatalf("SetupRules() expected error, got nil")
			}
			if !stderrors.Is(err, tt.wantErrIs) {
				t.Errorf("SetupRules() error = %v, want errors.Is(%v)", err, tt.wantErrIs)
			}
			if !strings.Contains(err.Error(), tt.wantErrPart) {
				t.Errorf("SetupRules() error = %q, want it to mention %q", err.Error(), tt.wantErrPart)
			}
		})
	}
}

func TestSetupRules_RuleAndGroupCount(t *testing.T) {
	tests := []struct {
		name          string
		slo           *openslov1.SLO
		wantGroups    []string
		wantRuleCount int
	}{
		{
			name:          "magic alerting enabled",
			slo:           createTestSLOWithAlerting("0.999"),
			wantGroups:    []string{recordingGroupName, alertGroupName},
			wantRuleCount: 23,
		},
		{
			name:          "magic alerting disabled",
			slo:           createTestSLO("0.999"),
			wantGroups:    []string{recordingGroupName},
			wantRuleCount: 19,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			groups := setupRules(t, tt.slo, createTestSLI(), "5m")

			if got := groupNames(groups); len(got) != len(tt.wantGroups) {
				t.Fatalf("expected groups %v, got %v", tt.wantGroups, got)
			}
			for i, want := range tt.wantGroups {
				if groups[i].Name != want {
					t.Errorf("group[%d] = %q, want %q", i, groups[i].Name, want)
				}
			}

			if got := countRules(groups); got != tt.wantRuleCount {
				t.Errorf("expected %d rules, got %d", tt.wantRuleCount, got)
			}
		})
	}
}

func TestSetupRules_RuleBreakdown(t *testing.T) {
	groups := setupRules(t, createTestSLOWithAlerting("0.999"), createTestSLI(), "5m")

	counts := map[string]int{}
	alerts := 0
	for _, r := range allRules(groups) {
		if r.Record != "" {
			counts[r.Record]++
		}
		if r.Alert != "" {
			alerts++
		}
	}

	want := map[string]int{
		"osko_slo_target":             1,
		"osko_sli_total":              2,
		"osko_sli_measurement":        8,
		"osko_error_budget_burn_rate": 8,
	}

	for record, wantCount := range want {
		if counts[record] != wantCount {
			t.Errorf("record %s: got %d rules, want %d", record, counts[record], wantCount)
		}
	}
	for record := range counts {
		if _, ok := want[record]; !ok {
			t.Errorf("unexpected recording rule %s emitted", record)
		}
	}
	if alerts != 4 {
		t.Errorf("expected 4 alert rules, got %d", alerts)
	}
}

func TestSetupRules_RemovedRecordingRules(t *testing.T) {
	for _, sli := range []*openslov1.SLI{createTestSLI(), createTestSLIWithBad()} {
		groups := setupRules(t, createTestSLOWithAlerting("0.999"), sli, "5m")
		for _, r := range allRules(groups) {
			switch r.Record {
			case "osko_sli_good", "osko_sli_bad", "osko_error_budget_ratio":
				t.Errorf("rule %s should no longer be emitted, got expr %q", r.Record, r.Expr.StrVal)
			}
		}
	}
}

func TestSetupRules_ExpressionLiterals(t *testing.T) {
	groups := setupRules(t, createTestSLOWithAlerting("0.999"), createTestSLI(), "5m")

	const selector5m = `namespace="default", service="test-service", sli_name="test-sli", slo_name="test-slo", window="5m"`

	tests := []struct {
		name   string
		record string
		window string
		want   string
	}{
		{
			name:   "slo target",
			record: "osko_slo_target",
			window: "28d",
			want:   "vector(0.999)",
		},
		{
			name:   "sli total base window",
			record: "osko_sli_total",
			window: "5m",
			want:   "sum(rate(http_requests_total[5m]))",
		},
		{
			name:   "sli total reporting window derived",
			record: "osko_sli_total",
			window: "28d",
			want:   "avg_over_time(osko_sli_total{" + selector5m + "}[28d])",
		},
		{
			name:   "sli measurement 5m",
			record: "osko_sli_measurement",
			window: "5m",
			want:   "clamp_min(clamp_max(sum(rate(http_requests_success_total[5m])) / sum(rate(http_requests_total[5m])), 1), 0)",
		},
		{
			name:   "sli measurement 28d derived",
			record: "osko_sli_measurement",
			window: "28d",
			want:   "clamp_min(clamp_max(avg_over_time(osko_sli_measurement{" + selector5m + "}[28d]), 1), 0)",
		},
		{
			name:   "burn rate 1h",
			record: "osko_error_budget_burn_rate",
			window: "1h",
			want:   `(1 - osko_sli_measurement{namespace="default", service="test-service", sli_name="test-sli", slo_name="test-slo", window="1h"}) / 0.0010000000`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := ruleFor(t, groups, tt.record, tt.window)
			if rule.Expr.StrVal != tt.want {
				t.Errorf("expr mismatch\n got: %s\nwant: %s", rule.Expr.StrVal, tt.want)
			}
		})
	}
}

func TestSetupRules_AlertExpressionLiteral(t *testing.T) {
	groups := setupRules(t, createTestSLOWithAlerting("0.999"), createTestSLI(), "5m")
	alertGroup := groupByName(t, groups, alertGroupName)

	want := `(osko_error_budget_burn_rate{namespace="default", service="test-service", sli_name="test-sli", slo_name="test-slo", window="5m"} > 14.4 and ignoring(window) osko_error_budget_burn_rate{namespace="default", service="test-service", sli_name="test-sli", slo_name="test-slo", window="1h"} > 14.4)`

	var found bool
	for _, r := range alertGroup.Rules {
		if r.Alert != "test-slo_alert_page_critical" {
			continue
		}
		found = true
		if r.Expr.StrVal != want {
			t.Errorf("alert expr mismatch\n got: %s\nwant: %s", r.Expr.StrVal, want)
		}
	}
	if !found {
		t.Fatal("expected test-slo_alert_page_critical alert")
	}
}

func TestSetupRules_AlertForDurations(t *testing.T) {
	groups := setupRules(t, createTestSLOWithAlerting("0.999"), createTestSLI(), "5m")
	alertGroup := groupByName(t, groups, alertGroupName)

	want := map[string]monitoringv1.Duration{
		"test-slo_alert_page_critical": "2m",
		"test-slo_alert_page_high":     "2m",
		"test-slo_alert_ticket_high":   "15m",
		"test-slo_alert_ticket_medium": "15m",
	}

	seen := map[string]bool{}
	for _, r := range alertRules(alertGroup) {
		wantFor, ok := want[r.Alert]
		if !ok {
			t.Errorf("unexpected alert %s", r.Alert)
			continue
		}
		seen[r.Alert] = true
		if r.For == nil {
			t.Errorf("alert %s has no `for` duration", r.Alert)
			continue
		}
		if *r.For != wantFor {
			t.Errorf("alert %s for = %s, want %s", r.Alert, *r.For, wantFor)
		}
	}
	for alert := range want {
		if !seen[alert] {
			t.Errorf("expected alert %s to be emitted", alert)
		}
	}
}

func TestSetupRules_BadMetricInlined(t *testing.T) {
	groups := setupRules(t, createTestSLOWithAlerting("0.999"), createTestSLIWithBad(), "5m")

	rule := ruleFor(t, groups, "osko_sli_measurement", "5m")
	want := "clamp_min(clamp_max((sum(rate(http_requests_total[5m])) - sum(rate(http_requests_error_total[5m]))) / sum(rate(http_requests_total[5m])), 1), 0)"
	if rule.Expr.StrVal != want {
		t.Errorf("bad-metric expr mismatch\n got: %s\nwant: %s", rule.Expr.StrVal, want)
	}

	if !strings.Contains(rule.Expr.StrVal, "- sum(rate(http_requests_error_total[5m]))") {
		t.Errorf("expected inlined bad metric subtraction, got: %s", rule.Expr.StrVal)
	}

	for _, r := range allRules(groups) {
		if r.Record == "osko_sli_good" || r.Record == "osko_sli_bad" {
			t.Errorf("unexpected %s rule emitted", r.Record)
		}
	}
}

func TestSetupRules_DefaultGroupingHasNoByClause(t *testing.T) {
	groups := setupRules(t, createTestSLOWithAlerting("0.999"), createTestSLI(), "5m")

	for _, r := range allRules(groups) {
		if strings.Contains(r.Expr.StrVal, " by (") {
			t.Errorf("expected no `by (` clause by default, got: %s", r.Expr.StrVal)
		}
	}

	for _, record := range []string{"osko_sli_total", "osko_sli_measurement"} {
		rule := ruleFor(t, groups, record, "5m")
		if !strings.Contains(rule.Expr.StrVal, "sum(rate(") {
			t.Errorf("expected %s to contain sum(rate(, got: %s", record, rule.Expr.StrVal)
		}
	}
}

func TestSetupRules_ExplicitGroupBy(t *testing.T) {
	slo := createTestSLOWithAlerting("0.999")
	slo.Annotations["osko.dev/groupBy"] = "region, cluster"

	groups := setupRules(t, slo, createTestSLI(), "5m")

	total := ruleFor(t, groups, "osko_sli_total", "5m")
	if !strings.Contains(total.Expr.StrVal, "by (region, cluster)") {
		t.Errorf("expected grouping clause in %s, got: %s", total.Record, total.Expr.StrVal)
	}

	measurement := ruleFor(t, groups, "osko_sli_measurement", "5m")
	if !strings.Contains(measurement.Expr.StrVal, "by (region, cluster)") {
		t.Errorf("expected grouping clause in %s, got: %s", measurement.Record, measurement.Expr.StrVal)
	}

	for _, r := range allRules(groups) {
		if r.Record == "" {
			continue
		}
		for _, forbidden := range []string{"region", "cluster"} {
			if _, ok := r.Labels[forbidden]; ok {
				t.Errorf("rule %s must not stamp the grouped label %q, labels: %v", r.Record, forbidden, r.Labels)
			}
		}
	}
}

func TestSetupRules_ReportingWindowSliTotal(t *testing.T) {
	countTotals := func(groups []monitoringv1.RuleGroup) int {
		n := 0
		for _, r := range allRules(groups) {
			if r.Record == "osko_sli_total" {
				n++
			}
		}
		return n
	}

	t.Run("raw when long window strategy is raw", func(t *testing.T) {
		slo := createTestSLOWithAlerting("0.999")
		slo.Annotations["osko.dev/longWindowStrategy"] = "raw"

		groups := setupRules(t, slo, createTestSLI(), "5m")

		if got := countTotals(groups); got != 2 {
			t.Fatalf("expected 2 osko_sli_total rules, got %d", got)
		}
		rule := ruleFor(t, groups, "osko_sli_total", "28d")
		want := "sum(rate(http_requests_total[28d]))"
		if rule.Expr.StrVal != want {
			t.Errorf("expr mismatch\n got: %s\nwant: %s", rule.Expr.StrVal, want)
		}
	})

	t.Run("raw when base window exceeds the raw threshold", func(t *testing.T) {
		groups := setupRules(t, createTestSLOWithAlerting("0.999"), createTestSLI(), "24h")

		rule := ruleFor(t, groups, "osko_sli_total", "28d")
		if strings.Contains(rule.Expr.StrVal, "avg_over_time(osko_sli_total{") {
			t.Errorf("must not derive from a base window that is itself long, got: %s", rule.Expr.StrVal)
		}
	})

	t.Run("no duplicate when reporting window equals base window", func(t *testing.T) {
		slo := createTestSLOWithAlerting("0.999")
		slo.Spec.TimeWindow = []openslov1.TimeWindowSpec{{Duration: "1h", IsRolling: true}}

		groups := setupRules(t, slo, createTestSLI(), "1h")

		if got := countTotals(groups); got != 1 {
			t.Errorf("expected exactly 1 osko_sli_total when base == reporting, got %d", got)
		}
		rule := ruleFor(t, groups, "osko_sli_total", "1h")
		want := "sum(rate(http_requests_total[1h]))"
		if rule.Expr.StrVal != want {
			t.Errorf("expr mismatch\n got: %s\nwant: %s", rule.Expr.StrVal, want)
		}
	})
}

func TestSetupRules_LongWindowStrategy(t *testing.T) {
	tests := []struct {
		name         string
		strategy     string
		wantDerived  bool
		derivedNames []string
	}{
		{
			name:         "derived by default",
			strategy:     "",
			wantDerived:  true,
			derivedNames: []string{"24h", "3d", "28d"},
		},
		{
			name:         "raw when requested",
			strategy:     "raw",
			wantDerived:  false,
			derivedNames: []string{"24h", "3d", "28d"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slo := createTestSLOWithAlerting("0.999")
			if tt.strategy != "" {
				slo.Annotations["osko.dev/longWindowStrategy"] = tt.strategy
			}

			groups := setupRules(t, slo, createTestSLI(), "5m")

			for _, w := range tt.derivedNames {
				rule := ruleFor(t, groups, "osko_sli_measurement", w)
				hasDerived := strings.Contains(rule.Expr.StrVal, "avg_over_time(osko_sli_measurement{")
				hasRaw := strings.Contains(rule.Expr.StrVal, "http_requests_total")

				if tt.wantDerived {
					if !hasDerived {
						t.Errorf("window %s: expected derived expression, got: %s", w, rule.Expr.StrVal)
					}
					if hasRaw {
						t.Errorf("window %s: derived expression must not reference the raw metric, got: %s", w, rule.Expr.StrVal)
					}
					continue
				}

				if !hasRaw {
					t.Errorf("window %s: expected raw metric reference, got: %s", w, rule.Expr.StrVal)
				}
				if hasDerived {
					t.Errorf("window %s: raw strategy must not derive from osko_sli_measurement, got: %s", w, rule.Expr.StrVal)
				}
			}

			// Short windows are always raw.
			short := ruleFor(t, groups, "osko_sli_measurement", "5m")
			if !strings.Contains(short.Expr.StrVal, "http_requests_total") {
				t.Errorf("5m measurement should always be raw, got: %s", short.Expr.StrVal)
			}
		})
	}
}

func TestSetupRules_LongBaseWindowForcesRaw(t *testing.T) {
	slo := createTestSLOWithAlerting("0.999")
	groups := setupRules(t, slo, createTestSLI(), "24h")

	rule := ruleFor(t, groups, "osko_sli_measurement", "28d")
	if strings.Contains(rule.Expr.StrVal, "avg_over_time(osko_sli_measurement{") {
		t.Errorf("a base window longer than the raw threshold must not be derived from, got: %s", rule.Expr.StrVal)
	}

	base := ruleFor(t, groups, "osko_sli_measurement", "24h")
	if strings.Contains(base.Expr.StrVal, "avg_over_time(osko_sli_measurement{") {
		t.Errorf("base window measurement must never self-reference, got: %s", base.Expr.StrVal)
	}
}

func TestSetupRules_RecordingGroupOrdering(t *testing.T) {
	groups := setupRules(t, createTestSLOWithAlerting("0.999"), createTestSLI(), "5m")
	recording := groupByName(t, groups, recordingGroupName)

	lastMeasurement := -1
	firstBurnRate := len(recording.Rules)
	baseMeasurement := -1
	firstDerived := len(recording.Rules)
	baseTotal := -1
	derivedTotal := -1
	firstMeasurement := len(recording.Rules)

	for i, r := range recording.Rules {
		switch r.Record {
		case "osko_sli_total":
			if strings.Contains(r.Expr.StrVal, "avg_over_time(osko_sli_total{") {
				derivedTotal = i
			} else {
				baseTotal = i
			}
		case "osko_sli_measurement":
			if i < firstMeasurement {
				firstMeasurement = i
			}
			lastMeasurement = i
			if r.Labels["window"] == "5m" {
				baseMeasurement = i
			}
			if strings.Contains(r.Expr.StrVal, "avg_over_time(osko_sli_measurement{") && i < firstDerived {
				firstDerived = i
			}
		case "osko_error_budget_burn_rate":
			if i < firstBurnRate {
				firstBurnRate = i
			}
		}
	}

	if recording.Rules[0].Record != "osko_slo_target" {
		t.Errorf("expected osko_slo_target first, got %s", recording.Rules[0].Record)
	}
	if recording.Rules[1].Record != "osko_sli_total" || recording.Rules[2].Record != "osko_sli_total" {
		t.Errorf("expected both osko_sli_total rules at positions 1 and 2, got %s and %s",
			recording.Rules[1].Record, recording.Rules[2].Record)
	}
	if baseTotal == -1 || derivedTotal == -1 {
		t.Fatalf("expected a base and a derived osko_sli_total (baseTotal=%d, derivedTotal=%d)", baseTotal, derivedTotal)
	}
	if baseTotal >= derivedTotal {
		t.Errorf("the derived osko_sli_total must follow the base window one (baseTotal=%d, derivedTotal=%d)", baseTotal, derivedTotal)
	}
	if derivedTotal >= firstMeasurement {
		t.Errorf("both osko_sli_total rules must precede the measurement rules (derivedTotal=%d, firstMeasurement=%d)", derivedTotal, firstMeasurement)
	}
	if lastMeasurement >= firstBurnRate {
		t.Errorf("every burn rate rule must follow every measurement rule (lastMeasurement=%d, firstBurnRate=%d)", lastMeasurement, firstBurnRate)
	}
	if baseMeasurement == -1 {
		t.Fatal("expected a base window measurement rule")
	}
	if firstDerived <= baseMeasurement {
		t.Errorf("derived measurements must follow the base window measurement (baseMeasurement=%d, firstDerived=%d)", baseMeasurement, firstDerived)
	}
}

func TestSetupRules_WindowOrderingIsDeterministic(t *testing.T) {
	first := setupRules(t, createTestSLOWithAlerting("0.999"), createTestSLI(), "5m")
	second := setupRules(t, createTestSLOWithAlerting("0.999"), createTestSLI(), "5m")

	if len(first) != len(second) {
		t.Fatalf("group count differs between runs: %d vs %d", len(first), len(second))
	}
	for gi := range first {
		if len(first[gi].Rules) != len(second[gi].Rules) {
			t.Fatalf("rule count differs in group %s", first[gi].Name)
		}
		for ri := range first[gi].Rules {
			if first[gi].Rules[ri].Expr.StrVal != second[gi].Rules[ri].Expr.StrVal {
				t.Errorf("rule %d in group %s differs between runs:\n%s\n%s",
					ri, first[gi].Name, first[gi].Rules[ri].Expr.StrVal, second[gi].Rules[ri].Expr.StrVal)
			}
		}
	}

	wantOrder := []string{"5m", "30m", "1h", "2h", "6h", "24h", "3d", "28d"}
	var gotOrder []string
	for _, r := range allRules(first) {
		if r.Record == "osko_error_budget_burn_rate" {
			gotOrder = append(gotOrder, r.Labels["window"])
		}
	}
	if strings.Join(gotOrder, ",") != strings.Join(wantOrder, ",") {
		t.Errorf("burn rate window order = %v, want %v", gotOrder, wantOrder)
	}
}

func TestSetupRules_BurnRateFormula(t *testing.T) {
	groups := setupRules(t, createTestSLO("0.999"), createTestSLI(), "5m")

	found := 0
	for _, r := range allRules(groups) {
		if r.Record != "osko_error_budget_burn_rate" {
			continue
		}
		found++
		if !strings.Contains(r.Expr.StrVal, "1 - osko_sli_measurement{") {
			t.Errorf("burn rate should be derived from osko_sli_measurement, got: %s", r.Expr.StrVal)
		}
		if !strings.Contains(r.Expr.StrVal, "/") {
			t.Errorf("burn rate should divide by the error budget target, got: %s", r.Expr.StrVal)
		}
	}
	if found == 0 {
		t.Fatal("expected burn rate recording rules")
	}
}

// BUG-2 (a selector-less `osko_sli_total - osko_sli_bad` expression that
// collapsed every SLO onto one labelset) shipped because nothing ever checked
// that the generated PromQL was well formed. This is that check: every
// expression must parse, and every emitted name must satisfy the same
// constraints Prometheus applies in model/rulefmt.
func TestSetupRules_GeneratedRulesAreValidPromQL(t *testing.T) {
	withAnnotation := func(slo *openslov1.SLO, key, value string) *openslov1.SLO {
		slo.Annotations[key] = value
		return slo
	}

	cases := []struct {
		name       string
		slo        *openslov1.SLO
		sli        *openslov1.SLI
		baseWindow string
	}{
		{"good path", createTestSLOWithAlerting("0.999"), createTestSLI(), "5m"},
		{"bad path", createTestSLOWithAlerting("0.999"), createTestSLIWithBad(), "5m"},
		{
			"group by",
			withAnnotation(createTestSLOWithAlerting("0.999"), "osko.dev/groupBy", "region, cluster"),
			createTestSLI(), "5m",
		},
		{
			"group by with bad path",
			withAnnotation(createTestSLOWithAlerting("0.999"), "osko.dev/groupBy", "region, cluster"),
			createTestSLIWithBad(), "5m",
		},
		{
			"raw long windows good path",
			withAnnotation(createTestSLOWithAlerting("0.999"), "osko.dev/longWindowStrategy", "raw"),
			createTestSLI(), "5m",
		},
		{
			"raw long windows bad path",
			withAnnotation(createTestSLOWithAlerting("0.999"), "osko.dev/longWindowStrategy", "raw"),
			createTestSLIWithBad(), "5m",
		},
		{"gauge", createTestSLOWithAlerting("0.999"), createTestSLIGauge(), "5m"},
		{"long base window", createTestSLOWithAlerting("0.999"), createTestSLI(), "24h"},
	}

	checked := 0
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			groups := setupRules(t, tc.slo, tc.sli, tc.baseWindow)

			for _, g := range groups {
				for _, r := range g.Rules {
					expr := r.Expr.String()
					checked++

					if _, err := parser.ParseExpr(expr); err != nil {
						t.Errorf("%s: unparseable PromQL: %v\n  %s", g.Name, err, expr)
					}

					if r.Record != "" && !model.IsValidMetricName(model.LabelValue(r.Record)) {
						t.Errorf("%s: invalid recording rule name %q", g.Name, r.Record)
					}

					// Prometheus stores the alert name in the `alertname` label,
					// so rulefmt validates it as a label value, not a metric name.
					if r.Alert != "" && !model.LabelValue(r.Alert).IsValid() {
						t.Errorf("%s: invalid alert name %q", g.Name, r.Alert)
					}

					for k, v := range r.Labels {
						if !model.LabelName(k).IsValid() || k == model.MetricNameLabel {
							t.Errorf("%s: invalid label name %q on %s%s", g.Name, k, r.Record, r.Alert)
						}
						if !model.LabelValue(v).IsValid() {
							t.Errorf("%s: invalid label value %q for %q", g.Name, v, k)
						}
					}

					for k := range r.Annotations {
						if !model.LabelName(k).IsValid() {
							t.Errorf("%s: invalid annotation name %q", g.Name, k)
						}
					}
				}
			}
		})
	}

	if checked == 0 {
		t.Fatal("expected the matrix to produce rules to validate")
	}
	t.Logf("validated %d generated expressions across %d configurations", checked, len(cases))
}

func TestCreatePrometheusRule(t *testing.T) {
	rule, err := CreatePrometheusRule(createTestSLO("0.999"), createTestSLI())
	if err != nil {
		t.Fatalf("CreatePrometheusRule() error = %v", err)
	}

	if rule.Name != "test-slo" {
		t.Errorf("Expected rule name test-slo, got %s", rule.Name)
	}

	if len(rule.Spec.Groups) == 0 {
		t.Error("Expected rule groups to be created")
	}

	foundTarget := false
	for _, g := range rule.Spec.Groups {
		if g.Interval != nil {
			t.Errorf("rule group %s must not set an interval, got %s", g.Name, *g.Interval)
		}
		for _, r := range g.Rules {
			if r.Record == "osko_slo_target" {
				foundTarget = true
				if !strings.Contains(r.Expr.StrVal, "vector(0.999)") {
					t.Errorf("Expected target rule to contain vector(0.999), got %s", r.Expr.StrVal)
				}
				if r.Labels["window"] != "28d" {
					t.Errorf("Expected target rule window label 28d, got %s", r.Labels["window"])
				}
			}
		}
	}
	if !foundTarget {
		t.Error("Expected to find osko_slo_target recording rule")
	}
}

func TestBurnRateWindows_HasWindows(t *testing.T) {
	brw := &burnRateWindows{
		windows: map[string]monitoringv1.Rule{
			"5m":  {Record: "osko_burn_rate_5m"},
			"1h":  {Record: "osko_burn_rate_1h"},
			"30m": {Record: "osko_burn_rate_30m"},
		},
	}

	if !brw.hasWindows("5m", "1h") {
		t.Error("Expected hasWindows(5m, 1h) to be true")
	}

	if brw.hasWindows("5m", "6h") {
		t.Error("Expected hasWindows(5m, 6h) to be false (6h missing)")
	}

	if brw.hasWindows("24h") {
		t.Error("Expected hasWindows(24h) to be false")
	}
}

func TestSetupRules_MagicAlerting_WindowPairs(t *testing.T) {
	groups := setupRules(t, createTestSLOWithAlerting("0.999"), createTestSLI(), "5m")
	alertGroup := groupByName(t, groups, alertGroupName)

	expectedAlerts := 4
	if got := len(alertRules(alertGroup)); got != expectedAlerts {
		t.Errorf("Expected %d alert rules, got %d", expectedAlerts, got)
	}

	alertNames := make(map[string]bool)
	for _, rule := range alertRules(alertGroup) {
		alertNames[rule.Alert] = true
	}

	for _, suffix := range []string{"page_critical", "page_high", "ticket_high", "ticket_medium"} {
		found := false
		for name := range alertNames {
			if strings.Contains(name, suffix) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected alert with suffix %s", suffix)
		}
	}
}

func TestSetupRules_MagicAlerting_CorrectWindowPairs(t *testing.T) {
	groups := setupRules(t, createTestSLOWithAlerting("0.999"), createTestSLI(), "5m")
	alertGroup := groupByName(t, groups, alertGroupName)

	expectedPairs := []struct {
		shortWindow string
		longWindow  string
	}{
		{"5m", "1h"},
		{"30m", "6h"},
		{"2h", "24h"},
		{"6h", "3d"},
	}

	for _, rule := range alertRules(alertGroup) {
		shortWindow := rule.Labels["short_window"]
		longWindow := rule.Labels["long_window"]

		found := false
		for _, pair := range expectedPairs {
			if shortWindow == pair.shortWindow && longWindow == pair.longWindow {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("Unexpected window pair: short=%s, long=%s", shortWindow, longWindow)
		}
	}
}

func TestSetupRules_GaugeMetricsUseAvgOverTime(t *testing.T) {
	groups := setupRules(t, createTestSLO("0.999"), createTestSLIGauge(), "5m")

	for _, record := range []string{"osko_sli_total", "osko_sli_measurement"} {
		rule := ruleFor(t, groups, record, "5m")
		if !strings.Contains(rule.Expr.StrVal, "avg_over_time(") {
			t.Errorf("%s for gauge should use avg_over_time(), got: %s", record, rule.Expr.StrVal)
		}
		if strings.Contains(rule.Expr.StrVal, "rate(") {
			t.Errorf("%s for gauge should NOT use rate(), got: %s", record, rule.Expr.StrVal)
		}
	}
}

func TestSetupRules_CounterMetricsUseRate(t *testing.T) {
	groups := setupRules(t, createTestSLO("0.999"), createTestSLI(), "5m")

	for _, record := range []string{"osko_sli_total", "osko_sli_measurement"} {
		rule := ruleFor(t, groups, record, "5m")
		if !strings.Contains(rule.Expr.StrVal, "rate(") {
			t.Errorf("%s for counter should use rate(), got: %s", record, rule.Expr.StrVal)
		}
		if strings.Contains(rule.Expr.StrVal, "avg_over_time(") {
			t.Errorf("%s for counter should NOT use avg_over_time(), got: %s", record, rule.Expr.StrVal)
		}
	}
}

// evalBurnRateExpr writes one osko_error_budget_burn_rate sample per window into
// an in-memory TSDB and evaluates expr against it, returning the result series.
func evalBurnRateExpr(t *testing.T, expr string, burnRates map[string]float64) int {
	t.Helper()

	store := teststorage.New(t)
	t.Cleanup(func() { _ = store.Close() })

	at := time.Unix(0, 0)
	app := store.Appender(context.Background())
	for window, value := range burnRates {
		series := labels.FromStrings(
			labels.MetricName, "osko_error_budget_burn_rate",
			"namespace", "default",
			"service", "test-service",
			"sli_name", "test-sli",
			"slo_name", "test-slo",
			"window", window,
		)
		if _, err := app.Append(0, series, at.UnixMilli(), value); err != nil {
			t.Fatalf("append window %s: %v", window, err)
		}
	}
	if err := app.Commit(); err != nil {
		t.Fatalf("commit samples: %v", err)
	}

	engine := promql.NewEngine(promql.EngineOpts{
		MaxSamples: 10_000,
		Timeout:    10 * time.Second,
	})
	query, err := engine.NewInstantQuery(context.Background(), store, nil, expr, at)
	if err != nil {
		t.Fatalf("prepare query %q: %v", expr, err)
	}
	defer query.Close()

	result := query.Exec(context.Background())
	if result.Err != nil {
		t.Fatalf("evaluate query %q: %v", expr, result.Err)
	}
	vector, err := result.Vector()
	if err != nil {
		t.Fatalf("result as vector: %v", err)
	}

	return len(vector)
}

// The burn rate windows differ in their `window` label, so an alert joining them
// with a plain `and` silently matches nothing and can never fire. Asserting on
// the expression string cannot catch that, so evaluate it for real instead.
func TestSetupRules_AlertFiresOnlyWhenBothWindowsBreach(t *testing.T) {
	groups := setupRules(t, createTestSLOWithAlerting("0.999"), createTestSLI(), "5m")
	alertGroup := groupByName(t, groups, alertGroupName)

	var expr string
	for _, rule := range alertGroup.Rules {
		if rule.Alert == "test-slo_alert_page_critical" {
			expr = rule.Expr.StrVal
		}
	}
	if expr == "" {
		t.Fatal("expected test-slo_alert_page_critical alert")
	}

	const pageCriticalThreshold = 14.4
	over, under := pageCriticalThreshold+7, pageCriticalThreshold-12

	tests := []struct {
		name      string
		burnRates map[string]float64
		want      int
	}{
		{"both windows breach", map[string]float64{"5m": over, "1h": over}, 1},
		{"only short window breaches", map[string]float64{"5m": over, "1h": under}, 0},
		{"only long window breaches", map[string]float64{"5m": under, "1h": over}, 0},
		{"neither window breaches", map[string]float64{"5m": under, "1h": under}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := evalBurnRateExpr(t, expr, tt.burnRates); got != tt.want {
				t.Errorf("got %d series, want %d\nexpr: %s", got, tt.want, expr)
			}
		})
	}
}

func TestSetupRules_GroupsStayUnderMimirRuleLimit(t *testing.T) {
	// Mimir's ruler_max_rules_per_rule_group defaults to 20 and rejects the
	// entire group with HTTP 400 above it, so the rules never evaluate. The
	// operator has no way to detect this, hence the compile-time guard.
	const mimirMaxRulesPerGroup = 20

	tests := []struct {
		name        string
		baseWindow  string
		wantWindows int
	}{
		{name: "default base window", baseWindow: "5m", wantWindows: 8},
		{name: "custom base window adds a window", baseWindow: "1m", wantWindows: 9},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			groups := setupRules(t, createTestSLOWithAlerting("0.999"), createTestSLI(), tt.baseWindow)

			measurements := 0
			for _, g := range groups {
				for _, r := range g.Rules {
					if r.Record == "osko_sli_measurement" {
						measurements++
					}
				}
			}
			if measurements != tt.wantWindows {
				t.Fatalf("got %d windows, want %d", measurements, tt.wantWindows)
			}

			for _, g := range groups {
				if len(g.Rules) > mimirMaxRulesPerGroup {
					t.Errorf("group %q has %d rules, exceeds Mimir's limit of %d", g.Name, len(g.Rules), mimirMaxRulesPerGroup)
				}
			}
		})
	}
}

// TestAlertTiers_ExpressionMatchesTable pins the generated alerts to the SRE
// Workbook's table twice over: that alertTiers() carries the Workbook's own
// numbers, and that each alert uses its own row. Asserting only the second would
// be self-referential, since a row wired to the wrong config field still agrees
// with the expression built from that row.
func TestAlertTiers_ExpressionMatchesTable(t *testing.T) {
	// https://sre.google/workbook/alerting-on-slos/#6-multiwindow-multi-burn-rate-alerts
	want := map[config.SREAlertSeverity]struct {
		short    string
		long     string
		burnRate float64
		wait     monitoringv1.Duration
	}{
		config.PageCritical: {"5m", "1h", 14.4, "2m"},
		config.PageHigh:     {"30m", "6h", 6, "2m"},
		config.TicketHigh:   {"2h", "24h", 3, "15m"},
		config.TicketMedium: {"6h", "3d", 1, "15m"},
	}

	tiers := alertTiers()
	if len(tiers) != len(want) {
		t.Fatalf("alertTiers() returned %d tiers, want %d", len(tiers), len(want))
	}

	groups := setupRules(t, createTestSLOWithAlerting("0.999"), createTestSLI(), "5m")
	byAlert := map[string]monitoringv1.Rule{}
	for _, r := range alertRules(groupByName(t, groups, alertGroupName)) {
		byAlert[r.Alert] = r
	}

	for _, tier := range tiers {
		t.Run(string(tier.severity), func(t *testing.T) {
			w, ok := want[tier.severity]
			if !ok {
				t.Fatalf("unexpected tier %s", tier.severity)
			}

			if tier.short != w.short || tier.long != w.long {
				t.Errorf("windows = %s/%s, want %s/%s", tier.short, tier.long, w.short, w.long)
			}
			if tier.burnRate != w.burnRate {
				t.Errorf("burnRate = %v, want %v; is this tier wired to the wrong config field?", tier.burnRate, w.burnRate)
			}
			if tier.wait != w.wait {
				t.Errorf("wait = %s, want %s", tier.wait, w.wait)
			}

			rule, ok := byAlert[fmt.Sprintf("test-slo_alert_%s", tier.severity)]
			if !ok {
				t.Fatalf("no alert generated for tier %s", tier.severity)
			}

			if got := rule.Labels["short_window"]; got != w.short {
				t.Errorf("short_window = %q, want %q", got, w.short)
			}
			if got := rule.Labels["long_window"]; got != w.long {
				t.Errorf("long_window = %q, want %q", got, w.long)
			}

			threshold := fmt.Sprintf("> %.1f", w.burnRate)
			if n := strings.Count(rule.Expr.String(), threshold); n != 2 {
				t.Errorf("expr %q compares against %q %d times, want 2", rule.Expr.String(), threshold, n)
			}

			if rule.For == nil {
				t.Fatalf("alert for tier %s has no `for` duration", tier.severity)
			}
			if *rule.For != w.wait {
				t.Errorf("for = %s, want %s", *rule.For, w.wait)
			}
		})
	}
}

func TestCreatePrometheusRuleMarkerLabels(t *testing.T) {
	slo := &openslov1.SLO{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "checkout-availability",
			Namespace: "default",
			Labels: map[string]string{
				"team": "payments",
			},
		},
		Spec: openslov1.SLOSpec{
			Service:         "checkout",
			BudgetingMethod: "Occurrences",
			Objectives:      []openslov1.ObjectivesSpec{{Target: "0.99"}},
			TimeWindow:      []openslov1.TimeWindowSpec{{Duration: "28d", IsRolling: true}},
		},
	}

	sli := &openslov1.SLI{
		ObjectMeta: metav1.ObjectMeta{Name: "checkout-sli", Namespace: "default"},
		Spec: openslov1.SLISpec{
			RatioMetric: openslov1.RatioMetricSpec{
				Counter: true,
				Good: openslov1.MetricSpec{
					MetricSource: openslov1.MetricSource{
						MetricSourceRef: "thanos-ds",
						Type:            "Thanos",
						Spec:            openslov1.MetricSourceSpec{Query: "sum(rate(good_total[5m]))"},
					},
				},
				Total: openslov1.MetricSpec{
					MetricSource: openslov1.MetricSource{
						MetricSourceRef: "thanos-ds",
						Type:            "Thanos",
						Spec:            openslov1.MetricSourceSpec{Query: "sum(rate(requests_total[5m]))"},
					},
				},
			},
		},
	}

	rule, err := CreatePrometheusRule(slo, sli)
	if err != nil {
		t.Fatalf("CreatePrometheusRule() error = %v", err)
	}

	if got := rule.Labels[LabelManagedBy]; got != LabelManagedByValue {
		t.Errorf("rule.Labels[%q] = %q, want %q; ThanosRuler.ruleSelector needs a stable marker to select on",
			LabelManagedBy, got, LabelManagedByValue)
	}
	if got := rule.Labels[LabelSLOName]; got != "checkout-availability" {
		t.Errorf("rule.Labels[%q] = %q, want %q", LabelSLOName, got, "checkout-availability")
	}
	if got := rule.Labels["team"]; got != "payments" {
		t.Errorf("rule.Labels[\"team\"] = %q, want %q; labels inherited from the SLO must be preserved", got, "payments")
	}
}

func TestCreatePrometheusRuleMarkerLabelsOverrideConflictingSLOLabel(t *testing.T) {
	slo := &openslov1.SLO{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "checkout-availability",
			Namespace: "default",
			Labels: map[string]string{
				LabelManagedBy: "someone-else",
			},
		},
		Spec: openslov1.SLOSpec{
			Service:         "checkout",
			BudgetingMethod: "Occurrences",
			Objectives:      []openslov1.ObjectivesSpec{{Target: "0.99"}},
			TimeWindow:      []openslov1.TimeWindowSpec{{Duration: "28d", IsRolling: true}},
		},
	}

	sli := &openslov1.SLI{
		ObjectMeta: metav1.ObjectMeta{Name: "checkout-sli", Namespace: "default"},
		Spec: openslov1.SLISpec{
			RatioMetric: openslov1.RatioMetricSpec{
				Counter: true,
				Good: openslov1.MetricSpec{
					MetricSource: openslov1.MetricSource{
						MetricSourceRef: "thanos-ds",
						Type:            "Thanos",
						Spec:            openslov1.MetricSourceSpec{Query: "sum(rate(good_total[5m]))"},
					},
				},
				Total: openslov1.MetricSpec{
					MetricSource: openslov1.MetricSource{
						MetricSourceRef: "thanos-ds",
						Type:            "Thanos",
						Spec:            openslov1.MetricSourceSpec{Query: "sum(rate(requests_total[5m]))"},
					},
				},
			},
		},
	}

	rule, err := CreatePrometheusRule(slo, sli)
	if err != nil {
		t.Fatalf("CreatePrometheusRule() error = %v", err)
	}

	if got := rule.Labels[LabelManagedBy]; got != LabelManagedByValue {
		t.Errorf("rule.Labels[%q] = %q, want %q; the OSKO marker must win over a conflicting SLO label, "+
			"since mergeLabels resolves conflicts by later-map-wins ordering", LabelManagedBy, got, LabelManagedByValue)
	}
}
