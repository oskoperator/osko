package helpers

import (
	"context"
	"github.com/go-logr/logr"
	mimirclient "github.com/grafana/mimir/pkg/mimirtool/client"
	"github.com/grafana/mimir/pkg/mimirtool/rules/rwrulefmt"
	openslov1 "github.com/oskoperator/osko/api/openslo/v1"
	"github.com/oskoperator/osko/internal/mimirtool"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"reflect"
)

const (
	mimirRuleNamespace = "osko"
)

func GetMimirRuleGroup(log logr.Logger, mimirClient *mimirclient.MimirClient, rule *monitoringv1.PrometheusRule) *rwrulefmt.RuleGroup {
	mimirRuleGroup, err := mimirClient.GetRuleGroup(context.Background(), mimirRuleNamespace, rule.Name)
	if err != nil {
		log.Error(err, "Failed to get rule group")
		return nil
	}

	return mimirRuleGroup
}

func CreateMimirRuleGroup(log logr.Logger, mimirClient *mimirclient.MimirClient, rule *monitoringv1.PrometheusRule, ds *openslov1.Datasource) error {
	mimirRuleGroup, err := mimirtool.NewMimirRuleGroup(rule, ds)
	if err != nil {
		log.Error(err, "Failed to create Mimir rule group")
		return err
	}

	if err := mimirClient.CreateRuleGroup(context.Background(), mimirRuleNamespace, *mimirRuleGroup); err != nil {
		log.Error(err, "Failed to create rule group")
		return err
	}

	return nil
}

func UpdateMimirRuleGroup(log logr.Logger, mimirClient *mimirclient.MimirClient, existingGroup *rwrulefmt.RuleGroup, desiredGroup *rwrulefmt.RuleGroup) error {
	log.Info("Updating Mimir rule group")
	if reflect.DeepEqual(existingGroup, desiredGroup) {
		log.Info("Mimir rule group is already up to date")
		return nil
	}
	err := DeleteMimirRuleGroup(log, mimirClient, existingGroup)
	if err != nil {
		return err
	}
	return nil
}

func DeleteMimirRuleGroup(log logr.Logger, mimirClient *mimirclient.MimirClient, ruleGroup *rwrulefmt.RuleGroup) error {
	if err := mimirClient.DeleteRuleGroup(context.Background(), mimirRuleNamespace, ruleGroup.Name); err != nil {
		log.Error(err, "Failed to delete rule group")
		return err
	}

	return nil
}
