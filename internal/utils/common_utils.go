package utils

import (
	"context"
	openslov1 "github.com/oskoperator/osko/apis/openslo/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
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

func ExtractMetricNameFromQuery(query string) string {
	index := strings.Index(query, "{")
	if index == -1 {
		return ""
	}

	subStr := query[:index]
	return subStr
}

func (m MetricLabelParams) NewMetricLabelCompiler() string {
	window := string(m.Slo.Spec.TimeWindow[0].Duration)
	if m.TimeWindow != "" {
		window = m.TimeWindow
	}

	labelString := `sli_name="` + m.Sli.Name + `", slo_name="` + m.Slo.Name + `", service="` + m.Slo.Spec.Service + `", window="` + window + `"`
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

func MergeLabels(ms ...map[string]string) map[string]string {
	res := map[string]string{}
	for _, m := range ms {
		for k, v := range m {
			res[k] = v
		}
	}

	return res
}
