package helpers

import (
	"bytes"
	"context"
	"fmt"
	openslov1 "github.com/oskoperator/osko/api/openslo/v1"
	"github.com/oskoperator/osko/internal/utils"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sort"
	"strings"
	"text/template"
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
	sum(increase({{.Metric}}[{{.Window}}]))
	{{- else if eq .RecordName "sli_good" -}}
	sum(increase({{.Metric}}[{{.Window}}]))
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
// by a equal sign and the pairs are comma-separated.
func mapToColonSeparatedString(labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// We build the string by iterating over the sorted keys.
	pairs := make([]string, len(labels))
	for i, k := range keys {
		pairs[i] = fmt.Sprintf("%s=\"%s\"", k, labels[k])
	}

	// Join the key-value pairs with commas and return the result.
	return strings.Join(pairs, ", ")
}

func (mrs *MonitoringRuleSet) createErrorBudgetValueRecordingRule(sliMeasurement monitoringv1.Rule, window string) (monitoringv1.Rule, error) {
	return monitoringv1.Rule{
		Record: fmt.Sprintf("%s_error_budget_available", RecordPrefix),
		Expr:   intstr.FromString(fmt.Sprintf("1 - %s{%s}", sliMeasurement.Record, mapToColonSeparatedString(sliMeasurement.Labels))),
		Labels: map[string]string{
			"service":  mrs.Slo.Spec.Service,
			"sli_name": mrs.Sli.Name,
			"slo_name": mrs.Slo.Name,
			"window":   window,
		},
	}, nil
}

func (mrs *MonitoringRuleSet) createErrorBudgetTargetRecordingRule(window string) (monitoringv1.Rule, error) {
	return monitoringv1.Rule{
		Record: fmt.Sprintf("%s_error_budget_target", RecordPrefix),
		Expr:   intstr.FromString(fmt.Sprintf("1 - %s", mrs.Slo.Spec.Objectives[0].Target)),
		Labels: map[string]string{
			"service":  mrs.Slo.Spec.Service,
			"sli_name": mrs.Sli.Name,
			"slo_name": mrs.Slo.Name,
			"window":   window,
		},
	}, nil
}

func (mrs *MonitoringRuleSet) createSliMeasurementRecordingRule(totalRule, goodRule monitoringv1.Rule, window string) (monitoringv1.Rule, error) {
	return monitoringv1.Rule{
		Record: fmt.Sprintf("%s_sli_measurement", RecordPrefix),
		Expr:   intstr.FromString(fmt.Sprintf("%s{%s} / %s{%s}", goodRule.Record, mapToColonSeparatedString(goodRule.Labels), totalRule.Record, mapToColonSeparatedString(totalRule.Labels))),
		Labels: map[string]string{
			"service":  mrs.Slo.Spec.Service,
			"sli_name": mrs.Sli.Name,
			"slo_name": mrs.Slo.Name,
			"window":   window,
		},
	}, nil
}

func (mrs *MonitoringRuleSet) createBurnRateRecordingRule(errorBudgetAvailable, errorBudgetTarget monitoringv1.Rule, window string) (monitoringv1.Rule, error) {
	return monitoringv1.Rule{
		Record: fmt.Sprintf("%s_burn_rate", RecordPrefix),
		Expr:   intstr.FromString(fmt.Sprintf("sum(%s{%s}) / sum(%s{%s})", errorBudgetAvailable.Record, mapToColonSeparatedString(errorBudgetAvailable.Labels), errorBudgetTarget.Record, mapToColonSeparatedString(errorBudgetTarget.Labels))),
		Labels: map[string]string{
			"service":  mrs.Slo.Spec.Service,
			"sli_name": mrs.Sli.Name,
			"slo_name": mrs.Slo.Name,
			"window":   window,
		},
	}, nil
}

func (mrs *MonitoringRuleSet) createRecordingRule(metric, recordName, window string, extended bool) (monitoringv1.Rule, error) {
	tmpl, err := template.New("promql").Parse(promqlTemplate)
	if err != nil {
		return monitoringv1.Rule{}, err
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
		return monitoringv1.Rule{}, err
	}

	rule := monitoringv1.Rule{
		Record: fmt.Sprintf("%s_%s", RecordPrefix, recordName),
		Expr:   intstr.FromString(promql.String()),
		Labels: map[string]string{
			"service":  mrs.Slo.Spec.Service,
			"sli_name": mrs.Sli.Name,
			"slo_name": mrs.Slo.Name,
			"window":   window,
		},
	}

	return rule, nil
}

// SetupRules creates Prometheus recording rules for the SLO and SLI
// and returns a slice of monitoringv1.Rule.
func (mrs *MonitoringRuleSet) SetupRules() ([]monitoringv1.Rule, error) {
	baseWindow := mrs.BaseWindow //Should configurable somewhere as agreed on product workshop
	extendedWindow := "28d"      //Default to 28d if not specified in the SLO

	if len(mrs.Slo.Spec.TimeWindow) > 0 && mrs.Slo.Spec.TimeWindow[0].Duration != "" {
		extendedWindow = string(mrs.Slo.Spec.TimeWindow[0].Duration)
	}

	targetRuleBase, _ := mrs.createRecordingRule(mrs.Slo.Spec.Objectives[0].Target, "slo_target", baseWindow, false)
	totalRuleBase, _ := mrs.createRecordingRule(mrs.Sli.Spec.RatioMetric.Total.MetricSource.Spec, "sli_total", baseWindow, false)
	goodRuleBase, _ := mrs.createRecordingRule(mrs.Sli.Spec.RatioMetric.Good.MetricSource.Spec, "sli_good", baseWindow, false)

	totalRuleExtended, _ := mrs.createRecordingRule(totalRuleBase.Record, "sli_total", extendedWindow, true)
	goodRuleExtended, _ := mrs.createRecordingRule(goodRuleBase.Record, "sli_good", extendedWindow, true)

	sliMeasurementBase, _ := mrs.createSliMeasurementRecordingRule(totalRuleBase, goodRuleBase, baseWindow)
	sliMeasurementExtended, _ := mrs.createSliMeasurementRecordingRule(totalRuleExtended, goodRuleExtended, extendedWindow)

	errorBudgetAvailableBase, _ := mrs.createErrorBudgetValueRecordingRule(sliMeasurementBase, baseWindow)
	errorBudgetAvailableExtended, _ := mrs.createErrorBudgetValueRecordingRule(sliMeasurementExtended, extendedWindow)

	errorBudgetTargetBase, _ := mrs.createErrorBudgetTargetRecordingRule(baseWindow)
	errorBudgetTargetExtended, _ := mrs.createErrorBudgetTargetRecordingRule(extendedWindow)

	burnRateBase, _ := mrs.createBurnRateRecordingRule(errorBudgetAvailableBase, errorBudgetTargetBase, baseWindow)
	burnRateExtended, _ := mrs.createBurnRateRecordingRule(errorBudgetAvailableExtended, errorBudgetTargetExtended, extendedWindow)

	rules := []monitoringv1.Rule{
		targetRuleBase,
		totalRuleBase,
		goodRuleBase,
		totalRuleExtended,
		goodRuleExtended,
		sliMeasurementBase,
		sliMeasurementExtended,
		errorBudgetAvailableBase,
		errorBudgetAvailableExtended,
		errorBudgetTargetBase,
		errorBudgetTargetExtended,
		burnRateBase,
		burnRateExtended,
	}

	return rules, nil
}

func CreatePrometheusRule(slo *openslov1.SLO, sli *openslov1.SLI) (*monitoringv1.PrometheusRule, error) {
	log := ctrllog.FromContext(context.Background())

	mrs := &MonitoringRuleSet{
		Slo:        slo,
		Sli:        sli,
		BaseWindow: "5m",
	}

	rules, err := mrs.SetupRules()
	if err != nil {
		log.V(1).Error(err, "Failed to create PrometheusRule because of some shit")
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

	ruleGroup := []monitoringv1.RuleGroup{
		{
			Name:  slo.Name,
			Rules: rules,
		},
	}

	typeMeta := metav1.TypeMeta{
		APIVersion: "monitoring.coreos.com/v1",
		Kind:       "PrometheusRule",
	}

	prometheusRule := monitoringv1.PrometheusRule{}

	prometheusRule.TypeMeta = typeMeta
	prometheusRule.ObjectMeta = objectMeta
	prometheusRule.Spec = monitoringv1.PrometheusRuleSpec{
		Groups: ruleGroup,
	}

	return &prometheusRule, nil
}

func (mrs *MonitoringRuleSet) addLabelsToRule(rule *monitoringv1.Rule) {
	window := string(mrs.Slo.Spec.TimeWindow[0].Duration)
	if len(mrs.Slo.Spec.TimeWindow) > 0 && mrs.Slo.Spec.TimeWindow[0].Duration != "" {
		window = string(mrs.Slo.Spec.TimeWindow[0].Duration)
	}
	rule.Labels = map[string]string{
		"sli_name": mrs.Sli.Name,
		"slo_name": mrs.Slo.Name,
		"service":  mrs.Slo.Spec.Service,
		"window":   window,
	}
}

func CreatePrometheusRule2(slo *openslov1.SLO, sli *openslov1.SLI) (*monitoringv1.PrometheusRule, error) {
	var monitoringRules []monitoringv1.Rule
	var targetVector monitoringv1.Rule
	defaultRateWindow := "1m"
	//burnRateTimeWindows := []string{"1h", "6h", "3d"}
	sloTimeWindowDuration := string(slo.Spec.TimeWindow[0].Duration)
	m := utils.MetricLabel{Slo: slo, Sli: sli}

	targetVector.Record = "osko_slo_target"
	targetVector.Expr = intstr.Parse(fmt.Sprintf("vector(%s)", slo.Spec.Objectives[0].Value))
	m.TimeWindow = sloTimeWindowDuration
	targetVector.Labels = m.NewMetricLabelGenerator()

	// for now, total and good are required. bad is optional and is calculated as (total - good) if not provided
	// TODO: validate that the SLO budgeting method is Occurrences and that the SLIs are all ratio metrics in other case throw an error
	targetVectorConfig := utils.Rule{
		Record:              "slo_target",
		Expr:                "",
		TimeWindow:          sloTimeWindowDuration,
		Slo:                 slo,
		Sli:                 sli,
		MetricLabelCompiler: &m,
	}

	totalRule28Config := utils.Rule{
		RuleType:            "total",
		Record:              "sli_ratio_total",
		Expr:                "sum(increase(%s[%s]))",
		TimeWindow:          sloTimeWindowDuration,
		Slo:                 slo,
		Sli:                 sli,
		MetricLabelCompiler: &m,
	}

	goodRule28Config := utils.Rule{
		RuleType:            "good",
		Record:              "sli_ratio_total",
		Expr:                "sum(increase(%s[%s]))",
		TimeWindow:          sloTimeWindowDuration,
		Slo:                 slo,
		Sli:                 sli,
		MetricLabelCompiler: &m,
	}

	badRule28Config := utils.Rule{
		RuleType:            "bad",
		Record:              "sli_ratio_total",
		Expr:                "sum(increase(%s[%s]))",
		TimeWindow:          sloTimeWindowDuration,
		Slo:                 slo,
		Sli:                 sli,
		MetricLabelCompiler: &m,
	}

	totalRuleConfig := utils.Rule{
		RuleType:            "total",
		Record:              "sli_ratio_total",
		Expr:                "sum(increase(%s[%s]))",
		TimeWindow:          defaultRateWindow,
		Slo:                 slo,
		Sli:                 sli,
		SupportiveRule:      &totalRule28Config,
		MetricLabelCompiler: &m,
	}

	goodRuleConfig := utils.Rule{
		RuleType:            "good",
		Record:              "sli_ratio_good",
		Expr:                "sum(increase(%s[%s]))",
		TimeWindow:          defaultRateWindow,
		Slo:                 slo,
		Sli:                 sli,
		SupportiveRule:      &goodRule28Config,
		MetricLabelCompiler: &m,
	}

	badRuleConfig := utils.Rule{
		RuleType:            "bad",
		Record:              "sli_ratio_bad",
		Expr:                "sum(increase(%s[%s]))",
		TimeWindow:          defaultRateWindow,
		Slo:                 slo,
		Sli:                 sli,
		SupportiveRule:      &badRule28Config,
		MetricLabelCompiler: &m,
	}

	errorBudgetRuleConfig := utils.BudgetRule{
		Record:           "error_budget_available",
		Slo:              slo,
		Sli:              sli,
		TargetRuleConfig: &targetVectorConfig,
		TotalRuleConfig:  &totalRuleConfig,
		BadRuleConfig:    &badRuleConfig,
		GoodRuleConfig:   &goodRuleConfig,
	}

	configs := []utils.Rule{
		totalRuleConfig,
		goodRuleConfig,
		badRuleConfig,
	}

	for _, config := range configs {
		rule, supportiveRule := config.NewRatioRule(config.TimeWindow)
		if rule == nil || supportiveRule == nil {
			continue
		}
		monitoringRules = append(monitoringRules, *rule)
		monitoringRules = append(monitoringRules, *supportiveRule)
	}

	monitoringRules = append(monitoringRules, targetVectorConfig.NewTargetRule())
	budgetRule, sliMeasurement := errorBudgetRuleConfig.NewBudgetRule()
	monitoringRules = append(monitoringRules, budgetRule)
	monitoringRules = append(monitoringRules, sliMeasurement)

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

	ruleGroup := []monitoringv1.RuleGroup{
		{
			Name:  slo.Name,
			Rules: monitoringRules,
		},
	}

	rule := &monitoringv1.PrometheusRule{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "monitoring.coreos.com/v1",
			Kind:       "PrometheusRule",
		},
		ObjectMeta: objectMeta,
		Spec: monitoringv1.PrometheusRuleSpec{
			Groups: ruleGroup,
		},
	}

	return rule, nil
}
