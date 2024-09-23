package helpers

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"text/template"

	openslov1 "github.com/oskoperator/osko/api/openslo/v1"
	"github.com/oskoperator/osko/internal/config"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	RecordPrefix   = "osko"
	promqlTemplate = `
	{{- if eq .RecordName "slo_target" -}}
	vector({{.Metric}})
	{{- else if and .Extended (eq .RecordName "sli_total") -}}
	sum(increase({{.Metric}}{{ "{" }}{{ .Labels }}{{ "}" }}[{{.Window}}]))
	{{- else if and .Extended (eq .RecordName "sli_good") -}}
	sum(increase({{.Metric}}{{ "{" }}{{ .Labels }}{{ "}" }}[{{.Window}}]))
	{{- else if eq .RecordName "sli_total" -}}
	sum(increase({{.Metric}}[{{.Window}}])) or vector(0)
	{{- else if eq .RecordName "sli_good" -}}
	sum(increase({{.Metric}}[{{.Window}}])) or vector(0)
	{{- else if eq .RecordName "sli_bad" -}}
	sum(increase({{.Metric}}[{{.Window}}])) or vector(0)
	{{- end -}}
	`
)

// RuleTemplateData holds data to fill the PromQL template.
type RuleTemplateData struct {
	Metric     string
	Service    string
	Window     string
	Extended   bool
	RecordName string
	Labels     string
}

// AlertRuleTemplateData holds data to fill the PromQL template for alerting rules.
type AlertRuleTemplateData struct {
	Metric     string
	Service    string
	Window     string
	RecordName string
	Labels     string
	For        string
}

type MonitoringRuleSet struct {
	Slo        *openslov1.SLO
	Sli        *openslov1.SLI
	TargetRule monitoringv1.Rule
	BaseRule   monitoringv1.Rule
	GoodRule   monitoringv1.Rule
	TotalRule  monitoringv1.Rule
	BaseWindow string
}

// mapToColonSeparatedString takes a map[string]string and returns a string
// that represents the map's key-value pairs, where each pair is concatenated
// by an equal sign and the pairs are comma-separated.
func mapToColonSeparatedString(labels map[string]string) string {
	log := ctrllog.FromContext(context.Background())

	pattern := "__.*?__"
	re, err := regexp.Compile(pattern)
	if err != nil {
		log.Error(err, "Failed to compile regex pattern")
	}

	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// We build the string by iterating over the sorted keys.
	pairs := make([]string, len(labels))
	for i, k := range keys {
		if re.MatchString(k) {
			continue
		}
		pairs[i] = fmt.Sprintf("%s=\"%s\"", k, labels[k])
	}

	// Join the key-value pairs with commas and return the result.
	return strings.Join(pairs, ", ")
}

func mergeLabels(ms ...map[string]string) map[string]string {
	labels := map[string]string{}
	for _, m := range ms {
		for k, v := range m {
			labels[k] = v
		}
	}

	return labels
}

func (mrs *MonitoringRuleSet) createBaseRuleLabels(window string) map[string]string {
	return map[string]string{
		"namespace": mrs.Slo.Namespace,
		"service":   mrs.Slo.Spec.Service,
		"sli_name":  mrs.Sli.Name,
		"slo_name":  mrs.Slo.Name,
		"window":    window,
	}
}

func (mrs *MonitoringRuleSet) createUserDefinedRuleLabels() map[string]string {
	relevantLabels := make(map[string]string)
	labelPrefix := "label.osko.dev/"
	for key, value := range mrs.Slo.ObjectMeta.Labels {
		if strings.HasPrefix(key, labelPrefix) {
			relevantKey := strings.TrimPrefix(key, labelPrefix)
			relevantLabels[relevantKey] = value
		}
	}

	return relevantLabels
}

func (mrs *MonitoringRuleSet) createErrorBudgetValueRecordingRule(sliMeasurement monitoringv1.Rule, window string) monitoringv1.Rule {
	sliMeasurementLabels := mapToColonSeparatedString(sliMeasurement.Labels)
	return monitoringv1.Rule{
		Record: fmt.Sprintf("%s_error_budget_value", RecordPrefix),
		Expr:   intstr.FromString(fmt.Sprintf("1 - %s{%s}", sliMeasurement.Record, sliMeasurementLabels)),
		Labels: mergeLabels(mrs.createBaseRuleLabels(window), mrs.createUserDefinedRuleLabels()),
	}
}

func (mrs *MonitoringRuleSet) createErrorBudgetTargetRecordingRule(window string) monitoringv1.Rule {
	return monitoringv1.Rule{
		Record: fmt.Sprintf("%s_error_budget_target", RecordPrefix),
		Expr:   intstr.FromString(fmt.Sprintf("1 - %s", mrs.Slo.Spec.Objectives[0].Target)),
		Labels: mergeLabels(mrs.createBaseRuleLabels(window), mrs.createUserDefinedRuleLabels()),
	}
}

func (mrs *MonitoringRuleSet) createSliMeasurementRecordingRule(totalRule, goodRule monitoringv1.Rule, window string) monitoringv1.Rule {
	goodLabels := mapToColonSeparatedString(goodRule.Labels)
	totalLabels := mapToColonSeparatedString(totalRule.Labels)
	return monitoringv1.Rule{
		Record: fmt.Sprintf("%s_sli_measurement", RecordPrefix),

		Expr:   intstr.FromString(fmt.Sprintf("clamp_max(%s{%s} / %s{%s}, 1)", goodRule.Record, goodLabels, totalRule.Record, totalLabels)),
		Labels: mergeLabels(mrs.createBaseRuleLabels(window), mrs.createUserDefinedRuleLabels()),
	}
}

func (mrs *MonitoringRuleSet) createBurnRateRecordingRule(errorBudgetValue, errorBudgetTarget monitoringv1.Rule, window string) monitoringv1.Rule {
	errorBudgetValueLabels := mapToColonSeparatedString(errorBudgetValue.Labels)
	errorBudgetTargetLabels := mapToColonSeparatedString(errorBudgetTarget.Labels)
	return monitoringv1.Rule{
		Record: fmt.Sprintf("%s_error_budget_burn_rate", RecordPrefix),
		Expr:   intstr.FromString(fmt.Sprintf("sum(%s{%s}) / sum(%s{%s})", errorBudgetValue.Record, errorBudgetValueLabels, errorBudgetTarget.Record, errorBudgetTargetLabels)),
		Labels: mergeLabels(mrs.createBaseRuleLabels(window), mrs.createUserDefinedRuleLabels()),
	}
}

func (mrs *MonitoringRuleSet) createAntecedentRule(metric, recordName, window string) monitoringv1.Rule {
	return monitoringv1.Rule{
		Record: fmt.Sprintf("%s_%s", RecordPrefix, recordName),
		Expr:   intstr.FromString(metric),
		Labels: mergeLabels(mrs.createBaseRuleLabels(window), mrs.createUserDefinedRuleLabels()),
	}
}

// checks if the metric source type of the metric in the SLI is Prometheus-compatible
func (mrs *MonitoringRuleSet) isPrometheusSource() bool {
	sourceString := ""
	opts := []string{mrs.Sli.Spec.RatioMetric.Total.MetricSource.Type, mrs.Sli.Spec.ThresholdMetric.MetricSource.Type}
	for _, opt := range opts {
		if opt != "" {
			sourceString = opt
			break
		}
	}
	sourceString = strings.ToLower(sourceString)
	switch sourceString {
	case
		"prometheus",
		"mimir",
		"cortex",
		"victoriametrics",
		"thanos":
		return true
	}
	return false
}

func (mrs *MonitoringRuleSet) createRecordingRule(metric, recordName, window string, extended bool) monitoringv1.Rule {
	log := ctrllog.FromContext(context.Background())
	tmpl, err := template.New("promql").Parse(promqlTemplate)
	if err != nil {
		log.Error(err, "Failed to parse the PromQL template")
		return monitoringv1.Rule{}
	}

	data := RuleTemplateData{
		Metric:     metric,
		Service:    mrs.Slo.Spec.Service,
		Window:     window,
		Extended:   extended,
		RecordName: recordName,
		Labels:     fmt.Sprintf("service=\"%s\", sli_name=\"%s\", slo_name=\"%s\", window=\"%s\"", mrs.Slo.Spec.Service, mrs.Sli.Name, mrs.Slo.Name, mrs.BaseWindow),
	}

	var promql bytes.Buffer
	if err := tmpl.Execute(&promql, data); err != nil {
		log.Error(err, "Failed to execute PromQL template")
		return monitoringv1.Rule{}
	}

	rule := monitoringv1.Rule{
		Record: fmt.Sprintf("%s_%s", RecordPrefix, recordName),
		Expr:   intstr.FromString(promql.String()),
		Labels: mergeLabels(mrs.createBaseRuleLabels(window), mrs.createUserDefinedRuleLabels()),
	}

	return rule
}

// SetupRules constructs rule groups for monitoring based on SLO and SLI configurations.
func (mrs *MonitoringRuleSet) SetupRules() ([]monitoringv1.RuleGroup, error) {
	//log := ctrllog.FromContext(context.Background())

	baseWindow := mrs.BaseWindow //Should configurable somewhere as agreed on product workshop
	extendedWindow := "28d"      //Default to 28d if not specified in the SLO

	if len(mrs.Slo.Spec.TimeWindow) > 0 && mrs.Slo.Spec.TimeWindow[0].Duration != "" {
		extendedWindow = string(mrs.Slo.Spec.TimeWindow[0].Duration)
	}

	if !mrs.isPrometheusSource() {
		return []monitoringv1.RuleGroup{}, fmt.Errorf("Unsupported metric source type")
	}

	var rules = map[string]map[string]monitoringv1.Rule{
		"targetRule":        {},
		"totalRule":         {},
		"goodRule":          {},
		"badRule":           {},
		"sliMeasurement":    {},
		"errorBudgetValue":  {},
		"errorBudgetTarget": {},
		"burnRate":          {},
	}

	windows := []string{baseWindow, extendedWindow, "5m", "30m", "1h", "2h", "6h", "24h", "3d"}
	var alertRuleErrorBudgets []monitoringv1.Rule

	// BASE WINDOW
	rules["targetRule"][baseWindow] = mrs.createRecordingRule(mrs.Slo.Spec.Objectives[0].Target, "slo_target", baseWindow, false)
	rules["totalRule"][baseWindow] = mrs.createRecordingRule(mrs.Sli.Spec.RatioMetric.Total.MetricSource.Spec.Query, "sli_total", baseWindow, false)

	if mrs.Sli.Spec.RatioMetric.Good.MetricSource.Spec.Query != "" {
		rules["goodRule"][baseWindow] = mrs.createRecordingRule(mrs.Sli.Spec.RatioMetric.Good.MetricSource.Spec.Query, "sli_good", baseWindow, false)
	} else {
		rules["badRule"][baseWindow] = mrs.createRecordingRule(mrs.Sli.Spec.RatioMetric.Bad.MetricSource.Spec.Query, "sli_bad", baseWindow, false)
		rules["goodRule"][baseWindow] = mrs.createAntecedentRule(
			fmt.Sprintf("%v - %v",
				rules["totalRule"][baseWindow].Record,
				rules["badRule"][baseWindow].Record,
			), "sli_good", baseWindow)
	}

	rules["sliMeasurement"][baseWindow] = mrs.createSliMeasurementRecordingRule(rules["totalRule"][baseWindow], rules["goodRule"][baseWindow], baseWindow)
	rules["errorBudgetValue"][baseWindow] = mrs.createErrorBudgetValueRecordingRule(rules["sliMeasurement"][baseWindow], baseWindow)
	rules["errorBudgetTarget"][baseWindow] = mrs.createErrorBudgetTargetRecordingRule(baseWindow)
	rules["burnRate"][baseWindow] = mrs.createBurnRateRecordingRule(rules["errorBudgetValue"][baseWindow], rules["errorBudgetTarget"][baseWindow], baseWindow)

	for _, window := range windows {
		if window == baseWindow {
			continue
		}
		// rules["targetRule"][window] = mrs.createRecordingRule(mrs.Slo.Spec.Objectives[0].Target, "slo_target", window, true)
		rules["totalRule"][window] = mrs.createRecordingRule(rules["totalRule"][baseWindow].Record, "sli_total", window, true)

		if mrs.Sli.Spec.RatioMetric.Good.MetricSource.Spec.Query != "" {
			rules["goodRule"][window] = mrs.createRecordingRule(rules["goodRule"][baseWindow].Record, "sli_good", window, true)
		} else {
			rules["badRule"][window] = mrs.createRecordingRule(rules["badRule"][baseWindow].Record, "sli_bad", window, true)
			rules["goodRule"][window] = mrs.createAntecedentRule(
				fmt.Sprintf("%v - %v",
					rules["totalRule"][window].Record,
					rules["badRule"][window].Record,
				), "sli_good", window)
		}

		rules["sliMeasurement"][window] = mrs.createSliMeasurementRecordingRule(rules["totalRule"][window], rules["goodRule"][window], window)
		rules["errorBudgetValue"][window] = mrs.createErrorBudgetValueRecordingRule(rules["sliMeasurement"][window], window)
		rules["errorBudgetTarget"][window] = mrs.createErrorBudgetTargetRecordingRule(window)
		rules["burnRate"][window] = mrs.createBurnRateRecordingRule(rules["errorBudgetValue"][window], rules["errorBudgetTarget"][window], window)
		if window == "5m" || window == "30m" || window == "1h" || window == "6h" || window == "24h" || window == "3d" || window == "2h" {
			alertRuleErrorBudgets = append(alertRuleErrorBudgets, rules["burnRate"][window])
		}
	}

	rulesByType := make(map[string][]monitoringv1.Rule)
	for ruleKey, nestedMap := range rules {
		for _, window := range windows {
			if rule, exists := nestedMap[window]; exists {
				rulesByType[ruleKey] = append(rulesByType[ruleKey], rule)
			}
		}
	}

	sloName := mrs.Slo.Name
	ruleGroups := []monitoringv1.RuleGroup{
		{Name: fmt.Sprintf("%s_slo_target", sloName), Rules: rulesByType["targetRule"]},
		{Name: fmt.Sprintf("%s_sli_good", sloName), Rules: rulesByType["goodRule"]},
		{Name: fmt.Sprintf("%s_sli_total", sloName), Rules: rulesByType["totalRule"]},
		{Name: fmt.Sprintf("%s_sli_measurement", sloName), Rules: rulesByType["sliMeasurement"]},
		{Name: fmt.Sprintf("%s_error_budget", sloName), Rules: rulesByType["errorBudgetValue"]},
		{Name: fmt.Sprintf("%s_burn_rate", sloName), Rules: rulesByType["burnRate"]},
	}

	// TODO: having the magic alerting we still need to figure out how to pass the severity into the alerting rule when we have more than one alerting tool.
	// The idea is to have a map of the alerting tool and the severity and then iterate over all connected AlertNotificationTargets.

	if mrs.Slo.ObjectMeta.Annotations["osko.dev/magicAlerting"] == "true" {
		duration := monitoringv1.Duration("5m")
		var alertRules []monitoringv1.Rule
		alertRules = append(alertRules, mrs.createMagicMultiBurnRateAlert(alertRuleErrorBudgets, "0.001", &duration, config.GetAlertingSeveritiesMap(config.AlertingTool{Name: "opsgenie"}).HighFast))
		ruleGroups = append(ruleGroups, monitoringv1.RuleGroup{
			Name: fmt.Sprintf("%s_slo_alert", sloName), Rules: alertRules,
		})
	}
	return ruleGroups, nil
}

// createMagicMultiBurnRateAlert creates a Prometheus alert rule for multi-burn rate alerting.
func (mrs *MonitoringRuleSet) createMagicMultiBurnRateAlert(burnRates []monitoringv1.Rule, threshold string, duration *monitoringv1.Duration, severity string) monitoringv1.Rule {
	log := ctrllog.FromContext(context.Background())
	cfg := config.NewConfig()

	alertingPageWindowsOrder := []string{"1h", "5m", "6h", "30m", "24h", "2h", "3d"}

	alertingPageWindows := map[string]monitoringv1.Rule{
		"1h":  {},
		"5m":  {},
		"6h":  {},
		"30m": {},
		"24h": {},
		"2h":  {},
		"3d":  {},
	}

	// Populate the alertingPageWindows map with the actual burn rates
	for _, br := range burnRates {
		if _, exists := alertingPageWindows[br.Labels["window"]]; exists {
			alertingPageWindows[br.Labels["window"]] = br
		}
	}

	// Define the alert expressions for different severities and durations
	var alertExpression string
	if severity == "page" {
		alertExpression = fmt.Sprintf(
			"(%s{%s} > (%.1f * %s) and %s{%s} > (%.1f * %s)) or (%s{%s} > (%.1f * %s) and %s{%s} > (%.1f * %s))",
			alertingPageWindows[alertingPageWindowsOrder[2]].Record, mapToColonSeparatedString(burnRates[2].Labels), cfg.AlertingBurnRates.PageShortWindow, threshold,
			alertingPageWindows[alertingPageWindowsOrder[0]].Record, mapToColonSeparatedString(burnRates[0].Labels), cfg.AlertingBurnRates.PageShortWindow, threshold,
			alertingPageWindows[alertingPageWindowsOrder[3]].Record, mapToColonSeparatedString(burnRates[3].Labels), cfg.AlertingBurnRates.PageLongWindow, threshold,
			alertingPageWindows[alertingPageWindowsOrder[1]].Record, mapToColonSeparatedString(burnRates[1].Labels), cfg.AlertingBurnRates.PageLongWindow, threshold,
		)
	} else if severity == "ticket" {
		alertExpression = fmt.Sprintf(
			"(%s{%s} > (%.1f * %s) and %s{%s} > (%.1f * %s)) or (%s{%s} > %.3f and %s{%s} > %.3f)",
			alertingPageWindows[alertingPageWindowsOrder[4]].Record, mapToColonSeparatedString(burnRates[4].Labels), cfg.AlertingBurnRates.TicketShortWindow, threshold,
			alertingPageWindows[alertingPageWindowsOrder[5]].Record, mapToColonSeparatedString(burnRates[5].Labels), cfg.AlertingBurnRates.TicketShortWindow, threshold,
			alertingPageWindows[alertingPageWindowsOrder[6]].Record, mapToColonSeparatedString(burnRates[6].Labels), cfg.AlertingBurnRates.TicketLongWindow,
			alertingPageWindows[alertingPageWindowsOrder[3]].Record, mapToColonSeparatedString(burnRates[3].Labels), cfg.AlertingBurnRates.TicketLongWindow,
		)
	}

	if alertExpression == "" {
		log.Info("Creation of alerting rule failed", "EmptyExpression", "Failed to create the alert expression, expression is empty")
		return monitoringv1.Rule{}
	}

	return monitoringv1.Rule{
		Alert: fmt.Sprintf("%s_alert_%s", burnRates[0].Record, severity),
		Expr:  intstr.FromString(alertExpression),
		For:   duration,
		Labels: map[string]string{
			"severity": severity,
		},
		Annotations: map[string]string{
			"summary":     "SLO Burn Rate Alert",
			"description": fmt.Sprintf("The burn rate of the SLO %s is higher than the %s threshold", mrs.Slo.Name, threshold),
		},
	}
}

func CreateAlertingRule() (*monitoringv1.PrometheusRule, error) {
	return nil, nil
}

func CreatePrometheusRule(slo *openslov1.SLO, sli *openslov1.SLI) (*monitoringv1.PrometheusRule, error) {
	cfg := config.NewConfig()
	baseWindow := cfg.DefaultBaseWindow.String()
	if slo.ObjectMeta.Annotations["osko.dev/baseWindow"] != "" {
		baseWindow = slo.ObjectMeta.Annotations["osko.dev/baseWindow"]
	}

	mrs := &MonitoringRuleSet{
		Slo:        slo,
		Sli:        sli,
		BaseWindow: baseWindow,
	}

	ruleGroups, err := mrs.SetupRules()
	if err != nil {
		// log.V(1).Error(err, "Failed to create the PrometheusRule")
		return nil, err
	}

	ownerRef := []metav1.OwnerReference{
		*metav1.NewControllerRef(
			slo,
			openslov1.GroupVersion.WithKind("SLO"),
		),
	}

	objectMeta := metav1.ObjectMeta{
		Name:            slo.Name,
		Namespace:       slo.Namespace,
		Labels:          slo.Labels,
		Annotations:     slo.Annotations,
		OwnerReferences: ownerRef,
	}

	typeMeta := metav1.TypeMeta{
		APIVersion: "monitoring.coreos.com/v1",
		Kind:       "PrometheusRule",
	}

	prometheusRule := monitoringv1.PrometheusRule{}

	prometheusRule.TypeMeta = typeMeta
	prometheusRule.ObjectMeta = objectMeta
	prometheusRule.Spec = monitoringv1.PrometheusRuleSpec{
		Groups: ruleGroups,
	}

	return &prometheusRule, nil
}
