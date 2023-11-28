package helpers

import (
	"context"
	"github.com/go-logr/logr"
	mimirclient "github.com/grafana/mimir/pkg/mimirtool/client"
	"github.com/grafana/mimir/pkg/mimirtool/rules/rwrulefmt"
	openslov1 "github.com/oskoperator/osko/api/openslo/v1"
	oskov1alpha1 "github.com/oskoperator/osko/api/osko/v1alpha1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/prometheus/prometheus/model/rulefmt"
	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
)

const (
	mimirRuleNamespace = "osko"
)

func NewMimirRule(slo *openslov1.SLO, rule *monitoringv1.PrometheusRule) (mimirRule *oskov1alpha1.MimirRule, err error) {
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

	mimirRule = &oskov1alpha1.MimirRule{
		ObjectMeta: objectMeta,
		Spec: oskov1alpha1.MimirRuleSpec{
			Groups: []oskov1alpha1.RuleGroup{
				{
					Name:          rule.Name,
					SourceTenants: nil,
					Rules: []oskov1alpha1.Rule{
						{
							Record:      rule.Spec.Groups[0].Rules[0].Record,
							Expr:        rule.Spec.Groups[0].Rules[0].Expr.String(),
							Labels:      rule.Spec.Groups[0].Rules[0].Labels,
							Annotations: rule.Spec.Groups[0].Rules[0].Annotations,
						},
					},
				},
			},
		},
	}
	return mimirRule, nil
}

func NewMimirRuleGroup(rule *monitoringv1.PrometheusRule) (*rwrulefmt.RuleGroup, error) {
	var mimirRuleNodes []rulefmt.RuleNode

	for _, group := range rule.Spec.Groups {
		for _, r := range group.Rules {
			mimirRuleNode := rulefmt.RuleNode{
				Record: yaml.Node{
					Kind:  8,
					Value: r.Record,
				},
				Alert: yaml.Node{},
				Expr: yaml.Node{
					Kind:  8,
					Value: r.Expr.StrVal,
				},
				Labels: rule.Labels,
			}
			mimirRuleNodes = append(mimirRuleNodes, mimirRuleNode)
		}
	}

	//dsConfig := utils.DataSourceConfig{DataSource: ds}
	//sourceTenants := dsConfig.ParseTenantAnnotation()

	mimirRuleGroup := &rwrulefmt.RuleGroup{
		RuleGroup: rulefmt.RuleGroup{
			Name: rule.Name,
			//SourceTenants: sourceTenants,
			Rules: mimirRuleNodes,
		},
		RWConfigs: []rwrulefmt.RemoteWriteConfig{},
	}

	return mimirRuleGroup, nil
}

func GetMimirRuleGroup(log logr.Logger, mimirClient *mimirclient.MimirClient, rule *monitoringv1.PrometheusRule) *rwrulefmt.RuleGroup {
	mimirRuleGroup, err := mimirClient.GetRuleGroup(context.Background(), mimirRuleNamespace, rule.Name)
	if err != nil {
		log.Error(err, "Failed to get rule group")
		return nil
	}

	return mimirRuleGroup
}

func CreateMimirRuleGroupAPI(log logr.Logger, mimirClient *mimirclient.MimirClient, rule *oskov1alpha1.RuleGroup, ds *openslov1.Datasource) error {
	mimirRule := &rwrulefmt.RuleGroup{
		RuleGroup: rulefmt.RuleGroup{
			Name: rule.Name,
			Rules: []rulefmt.RuleNode{
				{
					Record: yaml.Node{
						Kind:  8,
						Value: rule.Rules[0].Record,
					},
					Alert: yaml.Node{},
					Expr: yaml.Node{
						Kind:  8,
						Value: rule.Rules[0].Expr,
					},
				},
			},
		},
	}

	if err := mimirClient.CreateRuleGroup(context.Background(), mimirRuleNamespace, *mimirRule); err != nil {
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
