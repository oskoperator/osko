package helpers

import (
	"context"
	"github.com/go-logr/logr"
	mimirclient "github.com/grafana/mimir/pkg/mimirtool/client"
	"github.com/grafana/mimir/pkg/mimirtool/rules/rwrulefmt"
	openslov1 "github.com/oskoperator/osko/api/openslo/v1"
	oskov1alpha1 "github.com/oskoperator/osko/api/osko/v1alpha1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
)

const (
	mimirRuleNamespace = "osko"
)

type MimirClientConfig struct {
	Address  string
	TenantId string
}

func (m *MimirClientConfig) NewMimirClient() (*mimirclient.MimirClient, error) {
	return mimirclient.New(
		mimirclient.Config{
			Address: m.Address,
			ID:      m.TenantId,
		},
	)
}

func NewMimirRule(slo *openslov1.SLO, rule *monitoringv1.PrometheusRule, connectionDetails *oskov1alpha1.ConnectionDetails) (mimirRule *oskov1alpha1.MimirRule, err error) {
	ownerRef := []metav1.OwnerReference{
		*metav1.NewControllerRef(
			slo,
			openslov1.GroupVersion.WithKind("SLO"),
		),
	}

	objectMeta := metav1.ObjectMeta{
		Name:            rule.Name,
		Namespace:       rule.Namespace,
		Labels:          rule.Labels,
		Annotations:     rule.Annotations,
		OwnerReferences: ownerRef,
	}

	var ruleGroups []oskov1alpha1.RuleGroup
	for _, group := range rule.Spec.Groups {
		var mimirRules []oskov1alpha1.Rule
		rg := oskov1alpha1.RuleGroup{
			Name:          group.Name,
			SourceTenants: connectionDetails.SourceTenants,
		}
		for _, r := range group.Rules {
			mimirRuleNode := oskov1alpha1.Rule{
				Record: r.Record,
				Expr:   r.Expr.String(),
				Labels: r.Labels,
			}
			mimirRules = append(mimirRules, mimirRuleNode)
		}
		rg.Rules = mimirRules
		ruleGroups = append(ruleGroups, rg)
	}

	mimirRule = &oskov1alpha1.MimirRule{
		ObjectMeta: objectMeta,
		Spec: oskov1alpha1.MimirRuleSpec{
			ConnectionDetails: *connectionDetails,
			Groups:            ruleGroups},
	}
	return mimirRule, nil
}

func NewMimirRuleGroups(rule *monitoringv1.PrometheusRule, connectionDetails *oskov1alpha1.ConnectionDetails) ([]oskov1alpha1.RuleGroup, error) {
	var ruleGroups []oskov1alpha1.RuleGroup
	for _, group := range rule.Spec.Groups {
		var mimirRules []oskov1alpha1.Rule
		rg := oskov1alpha1.RuleGroup{
			Name:          group.Name,
			SourceTenants: connectionDetails.SourceTenants,
		}
		for _, r := range group.Rules {
			mimirRuleNode := oskov1alpha1.Rule{
				Record: r.Record,
				Expr:   r.Expr.String(),
				Labels: r.Labels,
			}
			mimirRules = append(mimirRules, mimirRuleNode)
		}
		rg.Rules = mimirRules
		ruleGroups = append(ruleGroups, rg)
	}
	return ruleGroups, nil
}

func GetMimirRuleGroup(log logr.Logger, mimirClient *mimirclient.MimirClient, rule *monitoringv1.PrometheusRule) *rwrulefmt.RuleGroup {
	mimirRuleGroup, err := mimirClient.GetRuleGroup(context.Background(), mimirRuleNamespace, rule.Name)
	if err != nil {
		log.Error(err, "Failed to get rule group")
		return nil
	}

	return mimirRuleGroup
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
