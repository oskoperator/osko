package helpers

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"

	openslov1 "github.com/oskoperator/osko/api/openslo/v1"
	"github.com/oskoperator/osko/internal/config"
	"github.com/oskoperator/osko/internal/errors"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/prometheus/common/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	RecordPrefix   = "osko"
	promqlTemplate = `
	{{- if eq .RecordName "slo_target" -}}
	vector({{.Metric}})
	{{- else if eq .RecordName "sli_total" -}}
	sum({{.Aggregation}}({{.Metric}}[{{.Window}}])) by ({{.Grouping}})
	{{- else if eq .RecordName "sli_good" -}}
	sum({{.Aggregation}}({{.Metric}}[{{.Window}}])) by ({{.Grouping}})
	{{- else if eq .RecordName "sli_bad" -}}
	sum({{.Aggregation}}({{.Metric}}[{{.Window}}])) by ({{.Grouping}})
	{{- end -}}
	`
	gaugeAggregation   = "avg_over_time"
	counterAggregation = "rate"
)

type RuleTemplateData struct {
	Metric      string
	Service     string
	Window      string
	RecordName  string
	Labels      string
	Aggregation string
	Grouping    string
}

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

	pairs := make([]string, 0, len(labels))
	for _, k := range keys {
		if re.MatchString(k) {
			continue
		}
		pairs = append(pairs, fmt.Sprintf("%s=\"%s\"", k, labels[k]))
	}

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

func uniqueStrings(input []string) []string {
	seen := make(map[string]bool)
	result := []string{}

	for _, v := range input {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	return result
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

func (mrs *MonitoringRuleSet) createSliMeasurementRecordingRule(totalRule, goodRule monitoringv1.Rule, window string) monitoringv1.Rule {
	goodLabels := mapToColonSeparatedString(goodRule.Labels)
	totalLabels := mapToColonSeparatedString(totalRule.Labels)
	return monitoringv1.Rule{
		Record: fmt.Sprintf("%s_sli_measurement", RecordPrefix),
		Expr:   intstr.FromString(fmt.Sprintf("clamp_max(%s{%s} / %s{%s}, 1)", goodRule.Record, goodLabels, totalRule.Record, totalLabels)),
		Labels: mergeLabels(mrs.createBaseRuleLabels(window), mrs.createUserDefinedRuleLabels()),
	}
}

func (mrs *MonitoringRuleSet) createErrorBudgetRatioRecordingRule(sliMeasurement monitoringv1.Rule, window string) monitoringv1.Rule {
	sliMeasurementLabels := mapToColonSeparatedString(sliMeasurement.Labels)
	return monitoringv1.Rule{
		Record: fmt.Sprintf("%s_error_budget_ratio", RecordPrefix),
		Expr:   intstr.FromString(fmt.Sprintf("1 - %s{%s}", sliMeasurement.Record, sliMeasurementLabels)),
		Labels: mergeLabels(mrs.createBaseRuleLabels(window), mrs.createUserDefinedRuleLabels()),
	}
}

func (mrs *MonitoringRuleSet) createBurnRateRecordingRule(errorBudgetRatio monitoringv1.Rule, errorBudgetTarget float64, window string) monitoringv1.Rule {
	errorBudgetRatioLabels := mapToColonSeparatedString(errorBudgetRatio.Labels)
	return monitoringv1.Rule{
		Record: fmt.Sprintf("%s_error_budget_burn_rate", RecordPrefix),
		Expr:   intstr.FromString(fmt.Sprintf("%s{%s} / %.10f", errorBudgetRatio.Record, errorBudgetRatioLabels, errorBudgetTarget)),
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

func parseTarget(target string) (float64, error) {
	return strconv.ParseFloat(target, 64)
}

func validateTarget(target float64) error {
	if target >= 1.0 {
		return fmt.Errorf("SLO target must be less than 1.0 (100%%), got %.4f: %w", target, errors.ErrInvalidTarget)
	}
	if target <= 0 {
		return fmt.Errorf("SLO target must be greater than 0, got %.4f: %w", target, errors.ErrInvalidTarget)
	}
	return nil
}

func (mrs *MonitoringRuleSet) createRecordingRule(metric, recordName, window string) monitoringv1.Rule {
	log := ctrllog.FromContext(context.Background())
	tmpl, err := template.New("promql").Parse(promqlTemplate)
	if err != nil {
		log.Error(err, "Failed to parse the PromQL template")
		return monitoringv1.Rule{}
	}

	isCounter := mrs.Sli.Spec.RatioMetric.Counter
	aggregation := counterAggregation
	if !isCounter {
		aggregation = gaugeAggregation
	}

	grouping := fmt.Sprintf("namespace, service, sli_name, slo_name")

	data := RuleTemplateData{
		Metric:      metric,
		Service:     mrs.Slo.Spec.Service,
		Window:      window,
		RecordName:  recordName,
		Aggregation: aggregation,
		Grouping:    grouping,
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

func (mrs *MonitoringRuleSet) SetupRules() ([]monitoringv1.RuleGroup, error) {
	log := ctrllog.FromContext(context.Background())

	baseWindow := mrs.BaseWindow
	log.V(1).Info("Starting SetupRules", "baseWindow", baseWindow)
	extendedWindow := "28d"

	if len(mrs.Slo.Spec.TimeWindow) > 0 && mrs.Slo.Spec.TimeWindow[0].Duration != "" {
		extendedWindow = string(mrs.Slo.Spec.TimeWindow[0].Duration)
	}

	if !mrs.isPrometheusSource() {
		return []monitoringv1.RuleGroup{}, fmt.Errorf("unsupported metric source type")
	}

	target, err := parseTarget(mrs.Slo.Spec.Objectives[0].Target)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SLO target: %w", err)
	}

	if err := validateTarget(target); err != nil {
		return nil, err
	}

	errorBudgetTarget := 1.0 - target
	log.V(1).Info("SLO configuration", "target", target, "errorBudgetTarget", errorBudgetTarget)

	var rules = map[string]map[string]monitoringv1.Rule{
		"targetRule":       {},
		"totalRule":        {},
		"goodRule":         {},
		"badRule":          {},
		"sliMeasurement":   {},
		"errorBudgetRatio": {},
		"burnRate":         {},
	}

	windows := []string{baseWindow, extendedWindow, "5m", "30m", "1h", "2h", "6h", "24h", "3d"}
	windows = uniqueStrings(windows)

	var alertingBurnRates []monitoringv1.Rule

	rules["targetRule"][baseWindow] = monitoringv1.Rule{
		Record: fmt.Sprintf("%s_slo_target", RecordPrefix),
		Expr:   intstr.FromString(fmt.Sprintf("vector(%s)", mrs.Slo.Spec.Objectives[0].Target)),
		Labels: mergeLabels(mrs.createBaseRuleLabels(baseWindow), mrs.createUserDefinedRuleLabels()),
	}

	for _, window := range windows {
		log.V(1).Info("Processing window", "window", window)

		rules["totalRule"][window] = mrs.createRecordingRule(mrs.Sli.Spec.RatioMetric.Total.MetricSource.Spec.Query, "sli_total", window)

		if mrs.Sli.Spec.RatioMetric.Good.MetricSource.Spec.Query != "" {
			rules["goodRule"][window] = mrs.createRecordingRule(mrs.Sli.Spec.RatioMetric.Good.MetricSource.Spec.Query, "sli_good", window)
		} else {
			rules["badRule"][window] = mrs.createRecordingRule(mrs.Sli.Spec.RatioMetric.Bad.MetricSource.Spec.Query, "sli_bad", window)
			rules["goodRule"][window] = mrs.createAntecedentRule(
				fmt.Sprintf("%s - %s",
					rules["totalRule"][window].Record,
					rules["badRule"][window].Record,
				), "sli_good", window)
		}

		rules["sliMeasurement"][window] = mrs.createSliMeasurementRecordingRule(rules["totalRule"][window], rules["goodRule"][window], window)
		rules["errorBudgetRatio"][window] = mrs.createErrorBudgetRatioRecordingRule(rules["sliMeasurement"][window], window)
		rules["burnRate"][window] = mrs.createBurnRateRecordingRule(rules["errorBudgetRatio"][window], errorBudgetTarget, window)

		if window == "5m" || window == "30m" || window == "1h" || window == "2h" ||
			window == "6h" || window == "24h" || window == "3d" {
			alertingBurnRates = append(alertingBurnRates, rules["burnRate"][window])
		}
	}

	log.V(1).Info("Final burn rates collection",
		"count", len(alertingBurnRates),
		"windows", func() []string {
			ws := make([]string, 0)
			for _, r := range alertingBurnRates {
				ws = append(ws, r.Labels["window"])
			}
			return ws
		}())

	rulesByType := make(map[string][]monitoringv1.Rule)

	if rule, exists := rules["targetRule"][baseWindow]; exists {
		rulesByType["targetRule"] = []monitoringv1.Rule{rule}
	}

	for ruleKey, nestedMap := range rules {
		if ruleKey == "targetRule" {
			continue
		}
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
		{Name: fmt.Sprintf("%s_error_budget_ratio", sloName), Rules: rulesByType["errorBudgetRatio"]},
		{Name: fmt.Sprintf("%s_burn_rate", sloName), Rules: rulesByType["burnRate"]},
	}

	log.V(1).Info("Magic alerting", "SLO", sloName, "enabled", mrs.Slo.ObjectMeta.Annotations["osko.dev/magicAlerting"])
	if mrs.Slo.ObjectMeta.Annotations["osko.dev/magicAlerting"] == "true" {
		duration := monitoringv1.Duration("5m")
		var alertRules []monitoringv1.Rule

		burnRateWindows := mrs.getBurnRateWindows(alertingBurnRates)

		if burnRateWindows.hasWindows("5m", "1h") {
			alertRules = append(alertRules,
				mrs.createMultiBurnRateAlert(
					burnRateWindows,
					errorBudgetTarget,
					&duration,
					config.PageCritical,
				),
			)
		}

		if burnRateWindows.hasWindows("30m", "6h") {
			alertRules = append(alertRules,
				mrs.createMultiBurnRateAlert(
					burnRateWindows,
					errorBudgetTarget,
					&duration,
					config.PageHigh,
				),
			)
		}

		if burnRateWindows.hasWindows("2h", "24h") {
			alertRules = append(alertRules,
				mrs.createMultiBurnRateAlert(
					burnRateWindows,
					errorBudgetTarget,
					&duration,
					config.TicketHigh,
				),
			)
		}

		if burnRateWindows.hasWindows("6h", "3d") {
			alertRules = append(alertRules,
				mrs.createMultiBurnRateAlert(
					burnRateWindows,
					errorBudgetTarget,
					&duration,
					config.TicketMedium,
				),
			)
		}

		ruleGroups = append(ruleGroups, monitoringv1.RuleGroup{
			Name:  fmt.Sprintf("%s_slo_alert", sloName),
			Rules: alertRules,
		})
	}
	return ruleGroups, nil
}

type burnRateWindows struct {
	windows map[string]monitoringv1.Rule
}

func (brw *burnRateWindows) hasWindows(required ...string) bool {
	for _, w := range required {
		if _, exists := brw.windows[w]; !exists {
			return false
		}
	}
	return true
}

func (brw *burnRateWindows) get(window string) monitoringv1.Rule {
	return brw.windows[window]
}

func (mrs *MonitoringRuleSet) getBurnRateWindows(burnRates []monitoringv1.Rule) *burnRateWindows {
	windows := make(map[string]monitoringv1.Rule)
	for _, br := range burnRates {
		if window, ok := br.Labels["window"]; ok && window != "" {
			windows[window] = br
		}
	}
	return &burnRateWindows{windows: windows}
}

func isValidRule(rule monitoringv1.Rule) bool {
	return rule.Record != "" && rule.Expr.String() != ""
}

func (mrs *MonitoringRuleSet) createMultiBurnRateAlert(
	brw *burnRateWindows,
	errorBudgetTarget float64,
	duration *monitoringv1.Duration,
	sreSeverity config.SREAlertSeverity,
) monitoringv1.Rule {
	log := ctrllog.FromContext(context.Background())

	var shortWindow, longWindow monitoringv1.Rule
	var shortThreshold, longThreshold float64

	switch sreSeverity {
	case config.PageCritical:
		shortWindow = brw.get("5m")
		longWindow = brw.get("1h")
		shortThreshold = config.Cfg.AlertingBurnRates.PageShortWindow
		longThreshold = config.Cfg.AlertingBurnRates.PageShortWindow
	case config.PageHigh:
		shortWindow = brw.get("30m")
		longWindow = brw.get("6h")
		shortThreshold = config.Cfg.AlertingBurnRates.PageLongWindow
		longThreshold = config.Cfg.AlertingBurnRates.PageLongWindow
	case config.TicketHigh:
		shortWindow = brw.get("2h")
		longWindow = brw.get("24h")
		shortThreshold = config.Cfg.AlertingBurnRates.TicketShortWindow
		longThreshold = config.Cfg.AlertingBurnRates.TicketShortWindow
	case config.TicketMedium:
		shortWindow = brw.get("6h")
		longWindow = brw.get("3d")
		shortThreshold = config.Cfg.AlertingBurnRates.TicketLongWindow
		longThreshold = config.Cfg.AlertingBurnRates.TicketLongWindow
	}

	if !isValidRule(shortWindow) || !isValidRule(longWindow) {
		log.V(1).Info("Missing or invalid burn rate windows for alert",
			"severity", sreSeverity,
			"shortWindowValid", isValidRule(shortWindow),
			"longWindowValid", isValidRule(longWindow))
		return monitoringv1.Rule{}
	}

	shortLabels := mapToColonSeparatedString(shortWindow.Labels)
	longLabels := mapToColonSeparatedString(longWindow.Labels)

	alertExpression := fmt.Sprintf(
		"(%s{%s} > %.1f and %s{%s} > %.1f)",
		shortWindow.Record, shortLabels, shortThreshold,
		longWindow.Record, longLabels, longThreshold,
	)

	alertingTool := mrs.Slo.ObjectMeta.Annotations["osko.dev/alertingTool"]
	if alertingTool == "" {
		alertingTool = config.Cfg.AlertingTool
	}

	severities := config.AlertSeveritiesByTool(alertingTool)
	toolSeverity := severities.GetSeverity(sreSeverity)

	log.V(1).Info("Alerting rule", "sreSeverity", sreSeverity, "toolSeverity", toolSeverity)

	return monitoringv1.Rule{
		Alert: fmt.Sprintf("%s_alert_%s", mrs.Slo.Name, sreSeverity),
		Expr:  intstr.FromString(alertExpression),
		For:   duration,
		Labels: map[string]string{
			"severity":     toolSeverity,
			"slo_name":     mrs.Slo.Name,
			"sli_name":     mrs.Sli.Name,
			"short_window": shortWindow.Labels["window"],
			"long_window":  longWindow.Labels["window"],
		},
		Annotations: map[string]string{
			"summary":     "SLO Burn Rate Alert",
			"description": fmt.Sprintf("The burn rate of SLO %s is consuming error budget faster than acceptable. Short window: %s, Long window: %s", mrs.Slo.Name, shortWindow.Labels["window"], longWindow.Labels["window"]),
		},
	}
}

func CreateAlertingRule() (*monitoringv1.PrometheusRule, error) {
	return nil, nil
}

func CreatePrometheusRule(slo *openslov1.SLO, sli *openslov1.SLI) (*monitoringv1.PrometheusRule, error) {
	baseWindow := model.Duration(config.Cfg.DefaultBaseWindow).String()
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
