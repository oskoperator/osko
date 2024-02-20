package helpers

import (
	"fmt"
	openslov1 "github.com/oskoperator/osko/api/openslo/v1"
	"github.com/oskoperator/osko/internal/utils"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	RecordPrefix = "osko"
)

func CreatePrometheusRule(slo *openslov1.SLO, sli *openslov1.SLI) (*monitoringv1.PrometheusRule, error) {
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
