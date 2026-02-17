package helpers

import (
	"strings"
	"testing"

	openslov1 "github.com/oskoperator/osko/api/openslo/v1"
	"github.com/oskoperator/osko/internal/config"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	config.NewConfig()
}

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
			Service: "test-service",
			Objectives: []openslov1.ObjectivesSpec{
				{Target: target},
			},
		},
	}
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

func TestSetupRules_BurnRateFormula(t *testing.T) {
	mrs := &MonitoringRuleSet{
		Slo:        createTestSLO("0.999"),
		Sli:        createTestSLI(),
		BaseWindow: "5m",
	}

	ruleGroups, err := mrs.SetupRules()
	if err != nil {
		t.Fatalf("SetupRules() error = %v", err)
	}

	var burnRateGroup *monitoringv1.RuleGroup
	for i, rg := range ruleGroups {
		if strings.HasSuffix(rg.Name, "_burn_rate") {
			burnRateGroup = &ruleGroups[i]
			break
		}
	}

	if burnRateGroup == nil {
		t.Fatal("Expected to find burn_rate rule group")
	}

	for _, rule := range burnRateGroup.Rules {
		if !strings.Contains(rule.Expr.StrVal, "error_budget_ratio") {
			t.Errorf("Burn rate should use error_budget_ratio, got: %s", rule.Expr.StrVal)
		}
		if !strings.Contains(rule.Expr.StrVal, "/") {
			t.Errorf("Burn rate should use division, got: %s", rule.Expr.StrVal)
		}
	}
}

func TestSetupRules_ExtendedWindowUsesIndependentRate(t *testing.T) {
	mrs := &MonitoringRuleSet{
		Slo:        createTestSLO("0.999"),
		Sli:        createTestSLI(),
		BaseWindow: "5m",
	}

	ruleGroups, err := mrs.SetupRules()
	if err != nil {
		t.Fatalf("SetupRules() error = %v", err)
	}

	for _, rg := range ruleGroups {
		if strings.HasSuffix(rg.Name, "_sli_total") {
			for _, rule := range rg.Rules {
				if strings.Contains(rule.Expr.StrVal, "increase(") {
					t.Errorf("Extended window should use rate(), not increase(), got: %s", rule.Expr.StrVal)
				}
				if strings.Contains(rule.Expr.StrVal, "osko_sli_total") {
					t.Errorf("Extended window should calculate from raw metrics, not recording rules, got: %s", rule.Expr.StrVal)
				}
			}
		}
	}
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
		for _, r := range g.Rules {
			if r.Record == "osko_slo_target" {
				foundTarget = true
				if !strings.Contains(r.Expr.StrVal, "vector(0.999)") {
					t.Errorf("Expected target rule to contain vector(0.999), got %s", r.Expr.StrVal)
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
	slo := createTestSLO("0.999")
	slo.Annotations = map[string]string{
		"osko.dev/magicAlerting": "true",
	}

	mrs := &MonitoringRuleSet{
		Slo:        slo,
		Sli:        createTestSLI(),
		BaseWindow: "5m",
	}

	ruleGroups, err := mrs.SetupRules()
	if err != nil {
		t.Fatalf("SetupRules() error = %v", err)
	}

	var alertGroup *monitoringv1.RuleGroup
	for i, rg := range ruleGroups {
		if strings.HasSuffix(rg.Name, "_slo_alert") {
			alertGroup = &ruleGroups[i]
			break
		}
	}

	if alertGroup == nil {
		t.Fatal("Expected to find alert rule group when magicAlerting is enabled")
	}

	expectedAlerts := 4
	if len(alertGroup.Rules) != expectedAlerts {
		t.Errorf("Expected %d alert rules, got %d", expectedAlerts, len(alertGroup.Rules))
	}

	alertNames := make(map[string]bool)
	for _, rule := range alertGroup.Rules {
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
	slo := createTestSLO("0.999")
	slo.Annotations = map[string]string{
		"osko.dev/magicAlerting": "true",
	}

	mrs := &MonitoringRuleSet{
		Slo:        slo,
		Sli:        createTestSLI(),
		BaseWindow: "5m",
	}

	ruleGroups, err := mrs.SetupRules()
	if err != nil {
		t.Fatalf("SetupRules() error = %v", err)
	}

	var alertGroup *monitoringv1.RuleGroup
	for i, rg := range ruleGroups {
		if strings.HasSuffix(rg.Name, "_slo_alert") {
			alertGroup = &ruleGroups[i]
			break
		}
	}

	if alertGroup == nil {
		t.Fatal("Expected to find alert rule group")
	}

	expectedPairs := []struct {
		shortWindow string
		longWindow  string
	}{
		{"5m", "1h"},
		{"30m", "6h"},
		{"2h", "24h"},
		{"6h", "3d"},
	}

	for _, rule := range alertGroup.Rules {
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

func TestSetupRules_BadMetric(t *testing.T) {
	mrs := &MonitoringRuleSet{
		Slo:        createTestSLO("0.999"),
		Sli:        createTestSLIWithBad(),
		BaseWindow: "5m",
	}

	ruleGroups, err := mrs.SetupRules()
	if err != nil {
		t.Fatalf("SetupRules() error = %v", err)
	}

	var goodGroup *monitoringv1.RuleGroup
	for i, rg := range ruleGroups {
		if strings.HasSuffix(rg.Name, "_sli_good") {
			goodGroup = &ruleGroups[i]
			break
		}
	}

	if goodGroup == nil {
		t.Fatal("Expected to find sli_good rule group")
	}

	foundGoodFromBad := false
	for _, rule := range goodGroup.Rules {
		if strings.Contains(rule.Expr.StrVal, "osko_sli_total - osko_sli_bad") {
			foundGoodFromBad = true
			break
		}
	}

	if !foundGoodFromBad {
		t.Error("Expected good metric to be calculated from total - bad")
	}
}

func TestSetupRules_GaugeMetricsUseAvgOverTime(t *testing.T) {
	mrs := &MonitoringRuleSet{
		Slo:        createTestSLO("0.999"),
		Sli:        createTestSLIGauge(),
		BaseWindow: "5m",
	}

	ruleGroups, err := mrs.SetupRules()
	if err != nil {
		t.Fatalf("SetupRules() error = %v", err)
	}

	for _, rg := range ruleGroups {
		if strings.HasSuffix(rg.Name, "_sli_total") {
			for _, rule := range rg.Rules {
				if !strings.Contains(rule.Expr.StrVal, "avg_over_time(") {
					t.Errorf("SLI total rule for gauge should use avg_over_time(), got: %s", rule.Expr.StrVal)
				}
				if strings.Contains(rule.Expr.StrVal, "rate(") {
					t.Errorf("SLI total rule for gauge should NOT use rate(), got: %s", rule.Expr.StrVal)
				}
			}
		}
		if strings.HasSuffix(rg.Name, "_sli_good") {
			for _, rule := range rg.Rules {
				if !strings.Contains(rule.Expr.StrVal, "avg_over_time(") {
					t.Errorf("SLI good rule for gauge should use avg_over_time(), got: %s", rule.Expr.StrVal)
				}
			}
		}
	}
}

func TestSetupRules_CounterMetricsUseRate(t *testing.T) {
	mrs := &MonitoringRuleSet{
		Slo:        createTestSLO("0.999"),
		Sli:        createTestSLI(),
		BaseWindow: "5m",
	}

	ruleGroups, err := mrs.SetupRules()
	if err != nil {
		t.Fatalf("SetupRules() error = %v", err)
	}

	for _, rg := range ruleGroups {
		if strings.HasSuffix(rg.Name, "_sli_total") {
			for _, rule := range rg.Rules {
				if !strings.Contains(rule.Expr.StrVal, "rate(") {
					t.Errorf("SLI total rule for counter should use rate(), got: %s", rule.Expr.StrVal)
				}
				if strings.Contains(rule.Expr.StrVal, "avg_over_time(") {
					t.Errorf("SLI total rule for counter should NOT use avg_over_time(), got: %s", rule.Expr.StrVal)
				}
			}
		}
		if strings.HasSuffix(rg.Name, "_sli_good") {
			for _, rule := range rg.Rules {
				if !strings.Contains(rule.Expr.StrVal, "rate(") {
					t.Errorf("SLI good rule for counter should use rate(), got: %s", rule.Expr.StrVal)
				}
			}
		}
	}
}
