package utils

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	openslov1 "github.com/oskoperator/osko/api/openslo/v1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"time"
)

type MetricLabel struct {
	Slo        *openslov1.SLO
	Sli        *openslov1.SLI
	TimeWindow string
	Labels     map[string]string
}

type Rule struct {
	Sli                 *openslov1.SLI
	Slo                 *openslov1.SLO
	BaseRule            *monitoringv1.Rule
	RuleType            string
	Record              string
	Expr                string
	RateWindow          string
	TimeWindow          string
	SupportiveRule      *Rule
	MetricLabelCompiler *MetricLabel
}

type BudgetRule struct {
	Record           string
	Sli              *openslov1.SLI
	Slo              *openslov1.SLO
	TotalRuleConfig  *Rule
	BadRuleConfig    *Rule
	GoodRuleConfig   *Rule
	TargetRuleConfig *Rule
}

type DataSourceConfig struct {
	DataSource *openslov1.Datasource
}

const (
	RecordPrefix    = "osko"
	TypeTotal       = "total"
	TypeBad         = "bad"
	TypeGood        = "good"
	TypeMeasurement = "sli_measurement"
	ExprFmt         = "sum(increase(%s[%s]))"
)

// UpdateCondition checks if the condition of the given type is already in the slice
// if the condition already exists and has the same status, return the unmodified conditions
// if the condition exists and has a different status, remove it and add the new one
// if the condition does not exist, add it
func updateCondition(conditions []metav1.Condition, newCondition metav1.Condition) []metav1.Condition {
	var existingCondition metav1.Condition
	exists := false

	for _, condition := range conditions {
		if condition.Type == newCondition.Type {
			existingCondition = condition
			exists = true
			break
		}
	}

	if exists && existingCondition.Status == newCondition.Status {
		return conditions
	}

	// Filter the existing condition (if it exists)
	var updatedConditions []metav1.Condition
	for _, condition := range conditions {
		if condition.Type != newCondition.Type {
			updatedConditions = append(updatedConditions, condition)
		}
	}

	// Append the new condition
	newCondition.LastTransitionTime = metav1.NewTime(time.Now())
	updatedConditions = append(updatedConditions, newCondition)

	return updatedConditions
}

func UpdateStatus(ctx context.Context, slo *openslov1.SLO, r client.Client, conditionType string, status metav1.ConditionStatus, message string) error {
	// Update the conditions based on provided arguments
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             string(status),
		Message:            message,
		LastTransitionTime: metav1.NewTime(time.Now()),
	}
	slo.Status.Conditions = updateCondition(slo.Status.Conditions, condition)
	slo.Status.Ready = string(status)
	return r.Status().Update(ctx, slo)
}

func (m MetricLabel) NewMetricLabelCompiler(rule *monitoringv1.Rule, window string) string {
	labelString := ""
	emptyRule := monitoringv1.Rule{}
	if !reflect.DeepEqual(rule, emptyRule) {
		windowVal := string(m.Slo.Spec.TimeWindow[0].Duration)
		if window != "" {
			windowVal = window
		}
		labelString = `sli_name="` + m.Sli.Name + `", slo_name="` + m.Slo.Name + `", service="` + m.Slo.Spec.Service + `", window="` + windowVal + `"`
	} else {
		for k, v := range rule.Labels {
			m.Labels[k] = v
		}
	}
	for k, v := range m.Labels {
		labelString += `, ` + k + `="` + v + `"`
	}

	return labelString
}

func (m MetricLabel) NewMetricLabelGenerator() map[string]string {
	window := string(m.Slo.Spec.TimeWindow[0].Duration)
	if m.TimeWindow != "" {
		window = m.TimeWindow
	}
	return map[string]string{
		"sli_name": m.Sli.Name,
		"slo_name": m.Slo.Name,
		"service":  m.Slo.Spec.Service,
		"window":   window,
	}
}

func (c Rule) getFieldsByType() (string, error) {
	switch c.RuleType {
	case TypeTotal:
		return c.Sli.Spec.RatioMetric.Total.MetricSource.Spec, nil
	case TypeBad:
		return c.Sli.Spec.RatioMetric.Bad.MetricSource.Spec, nil
	case TypeGood:
		return c.Sli.Spec.RatioMetric.Good.MetricSource.Spec, nil
	default:
		return "", fmt.Errorf("invalid RuleType: %s", c.RuleType)
	}
}

func (c Rule) NewRatioRule(window string) (*monitoringv1.Rule, *monitoringv1.Rule) {
	//
	field, err := c.getFieldsByType()
	if err != nil || field == "" {
		return nil, nil
	}

	expr := fmt.Sprintf(ExprFmt, field, window)

	rule := monitoringv1.Rule{
		Record: fmt.Sprintf("%s_%s", RecordPrefix, c.Record),
		Expr:   intstr.Parse(expr),
	}

	c.TimeWindow = window
	c.MetricLabelCompiler.TimeWindow = c.TimeWindow
	rule.Labels = c.MetricLabelCompiler.NewMetricLabelGenerator()

	supportiveRule := c.NewSupportiveRule(rule)

	return &rule, &supportiveRule
}

func (c Rule) NewSupportiveRule(baseRule monitoringv1.Rule) (rule monitoringv1.Rule) {
	rule.Record = fmt.Sprintf("%s_%s", RecordPrefix, c.Record)
	labels := c.SupportiveRule.MetricLabelCompiler.NewMetricLabelCompiler(&baseRule, baseRule.Labels["window"])
	expr := fmt.Sprintf("sum(increase(%s{%s}[%s])) by (service, sli_name, slo_name)", baseRule.Record, labels, c.SupportiveRule.TimeWindow)
	rule.Expr = intstr.Parse(expr)

	c.MetricLabelCompiler.TimeWindow = c.SupportiveRule.TimeWindow
	rule.Labels = c.MetricLabelCompiler.NewMetricLabelGenerator()

	return rule
}

func (c Rule) NewTargetRule() (rule monitoringv1.Rule) {
	rule.Record = fmt.Sprintf("%s_%s", RecordPrefix, c.Record)
	rule.Expr = intstr.Parse(fmt.Sprintf("vector(%s)", c.Slo.Spec.Objectives[0].Target))
	rule.Labels = c.MetricLabelCompiler.NewMetricLabelGenerator()
	return rule
}

func (b BudgetRule) NewBudgetRule() (budgetRule monitoringv1.Rule, sliMeasurement monitoringv1.Rule) {
	log := ctrllog.FromContext(context.Background())

	goodRuleSpec := b.GoodRuleConfig.Sli.Spec.RatioMetric.Good.MetricSource.Spec
	sloIndicatorSpec := b.GoodRuleConfig.Slo.Spec.Indicator.Spec.RatioMetric.Good.MetricSource.Spec
	gbRule := getRelevantRule(b, goodRuleSpec, sloIndicatorSpec, log)

	sliMeasurement = createSLIMeasurement(gbRule, b.TotalRuleConfig)
	budgetRule = createBudgetRule(b, gbRule)

	return budgetRule, sliMeasurement
}

func getRelevantRule(b BudgetRule, goodRuleSpec, sloIndicatorSpec string, log logr.Logger) *Rule {
	if goodRuleSpec == "" || sloIndicatorSpec == "" {
		log.Info("Good rule not provided, calculating bad as (total - bad)")
		return b.BadRuleConfig
	}
	log.Info("Good rule provided")
	return b.GoodRuleConfig
}

func createSLIMeasurement(gbRule, totalRuleConfig *Rule) monitoringv1.Rule {
	measurement := monitoringv1.Rule{}
	measurement.Record = fmt.Sprintf("%s_%s", RecordPrefix, TypeMeasurement)
	exprFormat := "%s_%s{%s} / %s_%s{%s}"
	measurement.Expr = intstr.Parse(fmt.Sprintf(exprFormat,
		RecordPrefix, gbRule.Record, gbRule.MetricLabelCompiler.NewMetricLabelCompiler(nil, ""),
		RecordPrefix, totalRuleConfig.Record, totalRuleConfig.MetricLabelCompiler.NewMetricLabelCompiler(nil, ""),
	))
	return measurement
}

func createBudgetRule(b BudgetRule, gbRule *Rule) monitoringv1.Rule {
	bRule := monitoringv1.Rule{}
	bRule.Record = fmt.Sprintf("%s_%s", RecordPrefix, b.Record)
	exprFormat := "(1 - %s_%s{%s}) * (%s_%s{%s} / %s_%s{%s})"
	bRule.Expr = intstr.Parse(fmt.Sprintf(exprFormat,
		RecordPrefix, b.TargetRuleConfig.Record, b.TargetRuleConfig.MetricLabelCompiler.NewMetricLabelCompiler(nil, ""),
		RecordPrefix, gbRule.Record, gbRule.MetricLabelCompiler.NewMetricLabelCompiler(nil, ""),
		RecordPrefix, b.TotalRuleConfig.Record, b.TotalRuleConfig.MetricLabelCompiler.NewMetricLabelCompiler(nil, ""),
	))
	return bRule
}

func (d DataSourceConfig) ParseTenantAnnotation() (tenants []string) {
	if len(d.DataSource.Spec.ConnectionDetails.SourceTenants) != 0 {
		for _, tenant := range d.DataSource.Spec.ConnectionDetails.SourceTenants {
			tenants = append(tenants, tenant)
		}
	}
	return tenants
}
