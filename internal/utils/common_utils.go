package utils

import (
	"context"
	"fmt"
	openslov1 "github.com/oskoperator/osko/apis/openslo/v1"
	v1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

type LabelGeneratorParams struct {
	Slo        *openslov1.SLO
	Sli        *openslov1.SLI
	TimeWindow string
}

type MetricLabelParams struct {
	Slo        *openslov1.SLO
	Sli        *openslov1.SLI
	TimeWindow string
	Labels     map[string]string
}

type RuleConfig struct {
	Sli                 *openslov1.SLI
	Slo                 *openslov1.SLO
	BaseRule            *v1.Rule
	RuleType            string
	RuleName            string
	Expr                string
	RateWindow          string
	TimeWindow          string
	LabelGenerator      LabelGeneratorParams
	SupportiveRule      *RuleConfig
	MetricLabelCompiler *MetricLabelParams
}

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
	updatedConditions := []metav1.Condition{}
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

func UpdateStatus(ctx context.Context, slo *openslov1.SLO, r client.Client, conditionType string, status metav1.ConditionStatus, reason string, message string) error {
	// Update the conditions based on provided arguments
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.NewTime(time.Now()),
	}
	slo.Status.Conditions = updateCondition(slo.Status.Conditions, condition)
	slo.Status.Ready = reason
	return r.Status().Update(ctx, slo)
}

func (m MetricLabelParams) NewMetricLabelCompiler(rule *v1.Rule, window string) string {
	labelString := ""
	emptyRule := v1.Rule{}
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

func (l LabelGeneratorParams) NewMetricLabelGenerator() map[string]string {
	window := string(l.Slo.Spec.TimeWindow[0].Duration)
	if l.TimeWindow != "" {
		window = l.TimeWindow
	}
	return map[string]string{
		"sli_name": l.Sli.Name,
		"slo_name": l.Slo.Name,
		"service":  l.Slo.Spec.Service,
		"window":   window,
	}
}

func (c RuleConfig) NewRatioRule(window string) (v1.Rule, v1.Rule) {
	rule := v1.Rule{}
	rule.Record = fmt.Sprintf("osko_%s", c.RuleName)
	expr := fmt.Sprintf(c.Expr, c.Sli.Spec.RatioMetric.Total.MetricSource.Spec, window)
	if c.RuleName != "bad" && c.Expr != "" {
		expr = fmt.Sprintf("sum(increase(%s[%s]))", c.Sli.Spec.RatioMetric.Total.MetricSource.Spec, window)
	}
	rule.Expr = intstr.Parse(expr)
	c.TimeWindow = window
	c.LabelGenerator.TimeWindow = c.TimeWindow
	rule.Labels = c.LabelGenerator.NewMetricLabelGenerator()

	supportiveRule := c.NewSupportiveRule(window, rule)

	return rule, supportiveRule
}

func (c RuleConfig) NewSupportiveRule(window string, baseRule v1.Rule) v1.Rule {
	rule := v1.Rule{}
	rule.Record = fmt.Sprintf("osko_%s", c.RuleName)
	labels := c.SupportiveRule.MetricLabelCompiler.NewMetricLabelCompiler(&baseRule, baseRule.Labels["window"])
	expr := fmt.Sprintf("sum(increase(%s{%s}[%s])) by (service, sli_name, slo_name)", baseRule.Record, labels, c.SupportiveRule.TimeWindow)
	rule.Expr = intstr.Parse(expr)

	c.LabelGenerator.TimeWindow = c.SupportiveRule.TimeWindow
	rule.Labels = c.LabelGenerator.NewMetricLabelGenerator()

	return rule
}
