package monitoringcoreoscom

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/grafana/mimir/pkg/mimirtool/rules/rwrulefmt"
	openslov1 "github.com/oskoperator/osko/apis/openslo/v1"
	"github.com/oskoperator/osko/internal/mimirtool"
	"github.com/oskoperator/osko/internal/utils"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/prometheus/prometheus/model/rulefmt"
	"gopkg.in/yaml.v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// PrometheusRuleReconciler reconciles a PrometheusRule object
type PrometheusRuleReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=monitoring.coreos.com,resources=prometheusrules,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=monitoring.coreos.com,resources=prometheusrules/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=monitoring.coreos.com,resources=prometheusrules/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the PrometheusRule object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.0/pkg/reconcile
func (r *PrometheusRuleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	prometheusRule := &monitoringv1.PrometheusRule{}

	err := r.Get(ctx, req.NamespacedName, prometheusRule)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("PrometheusRule deleted")
			log.Info("PrometheusRule deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get PrometheusRule")
		return ctrl.Result{}, err
	}

	ds := &openslov1.Datasource{}
	if err := r.Get(context.TODO(), client.ObjectKey{
		Namespace: prometheusRule.Namespace,
		Name:      "logging-ds",
	}, ds); err != nil {
		log.Error(err, "Failed to get Datasource")
		return ctrl.Result{}, err
	}
	err = r.createMimirRule(log, prometheusRule, ds)
	if err != nil {
		log.Error(err, "Failed to create Mimir rule")
		return ctrl.Result{}, err
	}
	log.Info("Mimir rule created")

	log.Info("PrometheusRule reconciled")
	return ctrl.Result{}, nil
}

func (r *PrometheusRuleReconciler) createMimirRule(log logr.Logger, rule *monitoringv1.PrometheusRule, ds *openslov1.Datasource) error {

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

	mimirRuleGroup := rwrulefmt.RuleGroup{
		RuleGroup: rulefmt.RuleGroup{
			Name:          rule.Name,
			SourceTenants: sourceTenants,
			Rules:         mimirRuleNodes,
		},
		RWConfigs: []rwrulefmt.RemoteWriteConfig{},
	}

	dataSource := &openslov1.Datasource{}
	if err := r.Get(context.TODO(), client.ObjectKey{
		Namespace: ds.Namespace,
		Name:      ds.Name,
	}, dataSource); err != nil {
		log.Error(err, "Failed to get Datasource")
		return err
	}

	mClient := mimirtool.MimirClientConfig{
		Address:  ds.Spec.ConnectionDetails.Address,
		TenantId: ds.Spec.ConnectionDetails.TargetTenant,
	}

	mimirClient, err := mClient.NewMimirClient()
	if err != nil {
		log.Error(err, "Failed to create Mimir client")
		return err
	}

	if err := mimirClient.CreateRuleGroup(context.Background(), "osko", mimirRuleGroup); err != nil {
		log.Error(err, "Failed to create rule group")
		return err
	}

	rules, err := mimirClient.ListRules(context.Background(), "osko")
	if err != nil {
		log.Error(err, "Failed to list rules")
		return err
	}

	log.Info(fmt.Sprintf("%+v", rules))

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PrometheusRuleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&monitoringv1.PrometheusRule{}).
		Complete(r)
}
