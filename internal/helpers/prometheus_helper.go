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
		Record: fmt.Sprintf("%s_error_budget_available", RecordPrefix),
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

func (mrs *MonitoringRuleSet) createBurnRateRecordingRule(errorBudgetAvailable, errorBudgetTarget monitoringv1.Rule, window string) monitoringv1.Rule {
	errorBudgetAvailableLabels := mapToColonSeparatedString(errorBudgetAvailable.Labels)
	errorBudgetTargetLabels := mapToColonSeparatedString(errorBudgetTarget.Labels)
	return monitoringv1.Rule{
		Record: fmt.Sprintf("%s_error_budget_burn_rate", RecordPrefix),
		Expr:   intstr.FromString(fmt.Sprintf("sum(%s{%s}) / sum(%s{%s})", errorBudgetAvailable.Record, errorBudgetAvailableLabels, errorBudgetTarget.Record, errorBudgetTargetLabels)),
		Labels: mergeLabels(mrs.createBaseRuleLabels(window), mrs.createUserDefinedRuleLabels()),
	}
}

func (mrs *MonitoringRuleSet) createAntecedentRule(metric, recordName, window string) monitoringv1.Rule {
	return monitoringv1.Rule{
		Record: recordName,
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
	baseWindow := mrs.BaseWindow //Should configurable somewhere as agreed on product workshop
	extendedWindow := "28d"      //Default to 28d if not specified in the SLO

	if len(mrs.Slo.Spec.TimeWindow) > 0 && mrs.Slo.Spec.TimeWindow[0].Duration != "" {
		extendedWindow = string(mrs.Slo.Spec.TimeWindow[0].Duration)
	}

	if !mrs.isPrometheusSource() {
		return []monitoringv1.RuleGroup{}, fmt.Errorf("Unsupported metric source type")
	}

	targetRuleBase := mrs.createRecordingRule(mrs.Slo.Spec.Objectives[0].Target, "slo_target", baseWindow, false)
	targetRuleExtended := mrs.createRecordingRule(mrs.Slo.Spec.Objectives[0].Target, "slo_target", extendedWindow, true)
	targetRuleExtendedPageFast := mrs.createRecordingRule(mrs.Slo.Spec.Objectives[0].Target, "slo_target", "1h", true)

	totalRuleBase := mrs.createRecordingRule(mrs.Sli.Spec.RatioMetric.Total.MetricSource.Spec.Query, "sli_total", baseWindow, false)

	var goodRuleBase monitoringv1.Rule
	if mrs.Sli.Spec.RatioMetric.Good.MetricSource.Spec.Query != "" {
		goodRuleBase = mrs.createRecordingRule(mrs.Sli.Spec.RatioMetric.Good.MetricSource.Spec.Query, "sli_good", baseWindow, false)
	} else {
		badRuleBase := mrs.createRecordingRule(mrs.Sli.Spec.RatioMetric.Bad.MetricSource.Spec.Query, "sli_bad", baseWindow, false)
		goodRuleBase = mrs.createAntecedentRule(
			fmt.Sprintf("%v - %v",
				totalRuleBase.Expr.String(),
				badRuleBase.Expr.String(),
			), "sli_good", baseWindow)
	}

	totalRuleExtended := mrs.createRecordingRule(totalRuleBase.Record, "sli_total", extendedWindow, true)
	totalRuleExtendedPageFast := mrs.createRecordingRule(totalRuleBase.Record, "sli_total", "1h", true)
	goodRuleExtended := mrs.createRecordingRule(goodRuleBase.Record, "sli_good", extendedWindow, true)
	goodRuleExtendedPageFast := mrs.createRecordingRule(goodRuleBase.Record, "sli_good", "1h", true)

	sliMeasurementBase := mrs.createSliMeasurementRecordingRule(totalRuleBase, goodRuleBase, baseWindow)
	sliMeasurementExtended := mrs.createSliMeasurementRecordingRule(totalRuleExtended, goodRuleExtended, extendedWindow)
	sliMeasurementExtendedPageFast := mrs.createSliMeasurementRecordingRule(totalRuleExtendedPageFast, goodRuleExtendedPageFast, "1h")

	errorBudgetAvailableBase := mrs.createErrorBudgetValueRecordingRule(sliMeasurementBase, baseWindow)
	errorBudgetAvailableExtended := mrs.createErrorBudgetValueRecordingRule(sliMeasurementExtended, extendedWindow)
	errorBudgetAvailableExtendedPageFast := mrs.createErrorBudgetValueRecordingRule(sliMeasurementExtendedPageFast, "1h")

	errorBudgetTargetBase := mrs.createErrorBudgetTargetRecordingRule(baseWindow)
	errorBudgetTargetExtended := mrs.createErrorBudgetTargetRecordingRule(extendedWindow)
	errorBudgetTargetExtendedPageFast := mrs.createErrorBudgetTargetRecordingRule("1h")

	burnRateBase := mrs.createBurnRateRecordingRule(errorBudgetAvailableBase, errorBudgetTargetBase, baseWindow)
	burnRateExtended := mrs.createBurnRateRecordingRule(errorBudgetAvailableExtended, errorBudgetTargetExtended, extendedWindow)
	burnRateExtendedPageFast := mrs.createBurnRateRecordingRule(errorBudgetAvailableExtendedPageFast, errorBudgetTargetExtendedPageFast, "1h")

	targetRules := []monitoringv1.Rule{targetRuleBase, targetRuleExtended, targetRuleExtendedPageFast}
	goodRules := []monitoringv1.Rule{goodRuleBase, goodRuleExtended, goodRuleExtendedPageFast}
	totalRules := []monitoringv1.Rule{totalRuleBase, totalRuleExtended, totalRuleExtendedPageFast}
	sliMeasurementRules := []monitoringv1.Rule{sliMeasurementBase, sliMeasurementExtended, sliMeasurementExtendedPageFast}
	errorBudgetRules := []monitoringv1.Rule{
		errorBudgetAvailableBase,
		errorBudgetAvailableExtended,
		errorBudgetAvailableExtendedPageFast,
		errorBudgetTargetBase,
		errorBudgetTargetExtended,
		errorBudgetTargetExtendedPageFast,
	}
	burnRateRules := []monitoringv1.Rule{burnRateBase, burnRateExtended, burnRateExtendedPageFast}

	sloName := mrs.Slo.Name
	ruleGroups := []monitoringv1.RuleGroup{
		{Name: fmt.Sprintf("%s_slo_target", sloName), Rules: targetRules},
		{Name: fmt.Sprintf("%s_sli_good", sloName), Rules: goodRules},
		{Name: fmt.Sprintf("%s_sli_total", sloName), Rules: totalRules},
		{Name: fmt.Sprintf("%s_sli_measurement", sloName), Rules: sliMeasurementRules},
		{Name: fmt.Sprintf("%s_error_budget", sloName), Rules: errorBudgetRules},
		{Name: fmt.Sprintf("%s_burn_rate", sloName), Rules: burnRateRules},
	}
	return ruleGroups, nil
}

func CreatePrometheusRule(slo *openslov1.SLO, sli *openslov1.SLI) (*monitoringv1.PrometheusRule, error) {
	baseWindow := "5m"
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
