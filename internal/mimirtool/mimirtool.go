package mimirtool

import (
	mimirtool "github.com/grafana/mimir/pkg/mimirtool/client"
	"github.com/grafana/mimir/pkg/mimirtool/rules/rwrulefmt"
	openslov1 "github.com/oskoperator/osko/api/openslo/v1"
	"github.com/oskoperator/osko/internal/utils"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/prometheus/prometheus/model/rulefmt"
	"gopkg.in/yaml.v3"
)

type MimirClientConfig struct {
	Address  string
	TenantId string
}

func (m *MimirClientConfig) NewMimirClient() (*mimirtool.MimirClient, error) {
	return mimirtool.New(
		mimirtool.Config{
			Address: m.Address,
			ID:      m.TenantId,
		},
	)
}

func NewMimirRuleGroup(rule *monitoringv1.PrometheusRule, ds *openslov1.Datasource) (*rwrulefmt.RuleGroup, error) {
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

	dsConfig := utils.DataSourceConfig{DataSource: ds}
	sourceTenants := dsConfig.ParseTenantAnnotation()

	mimirRuleGroup := &rwrulefmt.RuleGroup{
		RuleGroup: rulefmt.RuleGroup{
			Name:          rule.Name,
			SourceTenants: sourceTenants,
			Rules:         mimirRuleNodes,
		},
		RWConfigs: []rwrulefmt.RemoteWriteConfig{},
	}

	return mimirRuleGroup, nil
}
