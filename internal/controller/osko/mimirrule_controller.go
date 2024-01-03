package osko

import (
	"context"
	"github.com/go-logr/logr"
	mimirclient "github.com/grafana/mimir/pkg/mimirtool/client"
	"github.com/grafana/mimir/pkg/mimirtool/rules/rwrulefmt"
	openslov1 "github.com/oskoperator/osko/api/openslo/v1"
	"github.com/oskoperator/osko/internal/helpers"
	"github.com/oskoperator/osko/internal/utils"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/prometheus/prometheus/model/rulefmt"
	"gopkg.in/yaml.v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	oskov1alpha1 "github.com/oskoperator/osko/api/osko/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MimirRuleReconciler reconciles a MimirRule object
type MimirRuleReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	Recorder    record.EventRecorder
	MimirClient *mimirclient.MimirClient
}

const (
	mimirRuleFinalizer = "finalizer.mimir.osko.dev"
	mimirRuleNamespace = "osko"
)

// +kubebuilder:rbac:groups=osko.dev,resources=mimirrules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osko.dev,resources=mimirrules/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=osko.dev,resources=mimirrules/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the MimirRule object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile
func (r *MimirRuleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	ds := &openslov1.Datasource{}
	slo := &openslov1.SLO{}
	prometheusRule := &monitoringv1.PrometheusRule{}
	mimirRule := &oskov1alpha1.MimirRule{}
	newMimirRule := &oskov1alpha1.MimirRule{}

	err := r.Get(ctx, req.NamespacedName, prometheusRule)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("PrometheusRule resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get PrometheusRule")
		return ctrl.Result{}, err
	}

	err = r.Get(ctx, req.NamespacedName, mimirRule)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("MimirRule resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get MimirRule")
		return ctrl.Result{}, err
	}

	if apierrors.IsNotFound(err) {
		log.Info("MimirRule not found. Let's make one.")
		mimirRule, err = helpers.NewMimirRule(slo, prometheusRule, ds)

		if err = r.Create(ctx, mimirRule); err != nil {
			r.Recorder.Event(mimirRule, "Error", "FailedToCreateMimirRule", "Failed to create Mimir Rule")
			if err = r.Status().Update(ctx, mimirRule); err != nil {
				log.Error(err, "Failed to update MimirRule status")
				return ctrl.Result{}, err
			} else {
				log.Info("MimirRule created successfully")
				r.Recorder.Event(mimirRule, "Normal", "MimirRuleCreated", "MimirRule created successfully")
				mimirRule.Status.Ready = "True"
				if err := r.Status().Update(ctx, mimirRule); err != nil {
					log.Error(err, "Failed to update MimirRule ready status")
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, nil
			}
		}
	}

	for _, ref := range mimirRule.ObjectMeta.OwnerReferences {
		if ref.Kind == "SLO" {
			sloNamespacedName := types.NamespacedName{
				Name:      ref.Name,
				Namespace: req.Namespace,
			}

			if err := r.Get(ctx, sloNamespacedName, slo); err != nil {
				log.Error(err, "Failed to get SLO")
				return ctrl.Result{}, err
			}
		}
	}

	if err := r.Get(ctx, client.ObjectKey{
		Namespace: prometheusRule.Namespace,
		Name:      slo.ObjectMeta.Annotations["osko.dev/datasourceRef"],
	}, ds); err != nil {
		log.Error(err, "Failed to get Datasource")
		return ctrl.Result{}, err
	}

	log.Info("Datasource found", "Datasource", ds)

	if err := r.newMimirClient(ds); err != nil {
		log.Error(err, "Failed to create MimirClient")
		return ctrl.Result{}, err
	}

	rgs, err := helpers.NewMimirRuleGroup(prometheusRule)
	if err != nil {
		log.Error(err, "Failed to convert MimirRuleGroup")
		return ctrl.Result{}, err
	}

	if err := r.createMimirRuleGroupAPI(log, rgs); err != nil {
		log.Error(err, "Failed to create MimirRuleGroup")
		return ctrl.Result{}, err
	}

	if !utils.ContainString(mimirRule.GetFinalizers(), mimirRuleFinalizer) {
		if err := r.addFinalizer(log, mimirRule); err != nil {
			return ctrl.Result{}, err
		}
	}

	log.Info("MmimirRule already exists, we should update it.")
	newMimirRule, err = helpers.NewMimirRule(slo, prometheusRule, ds)
	if err != nil {
		log.Error(err, "Failed to create new MimirRule")
		return ctrl.Result{}, err
	}

	compareResult := reflect.DeepEqual(mimirRule.Spec, newMimirRule.Spec)
	if compareResult {
		log.Info("MimirRule is up to date")
		return ctrl.Result{}, nil
	}

	newMimirRule.ResourceVersion = mimirRule.ResourceVersion

	if err := r.Update(ctx, newMimirRule); err != nil {
		log.Error(err, "Failed to update MimirRule")
		mimirRule.Status.Ready = "False"
		if err := r.Status().Update(ctx, mimirRule); err != nil {
			log.Error(err, "Failed to update SLO status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, err
	}

	log.Info("MimirRule reconciled")
	return ctrl.Result{}, nil
}

func (r *MimirRuleReconciler) newMimirClient(ds *openslov1.Datasource) error {
	mClientConfig := helpers.MimirClientConfig{
		Address:  ds.Spec.ConnectionDetails.Address,
		TenantId: ds.Spec.ConnectionDetails.TargetTenant,
	}

	mimirClient, err := mClientConfig.NewMimirClient()
	if err != nil {
		return err
	}

	r.MimirClient = mimirClient

	return nil
}

func (r *MimirRuleReconciler) createMimirRuleGroupAPI(log logr.Logger, rule *oskov1alpha1.RuleGroup) error {
	var mimirRuleNodes []rulefmt.RuleNode
	for _, r := range rule.Rules {
		mimirRuleNode := rulefmt.RuleNode{
			Record: yaml.Node{
				Kind:  8,
				Value: r.Record,
			},
			Alert: yaml.Node{},
			Expr: yaml.Node{
				Kind:  8,
				Value: r.Expr,
			},
			Labels: r.Labels,
		}
		mimirRuleNodes = append(mimirRuleNodes, mimirRuleNode)
	}

	log.Info("Source tenants", "SourceTenants", rule.SourceTenants)

	mimirRule := rwrulefmt.RuleGroup{
		RuleGroup: rulefmt.RuleGroup{
			Name:          rule.Name,
			Rules:         mimirRuleNodes,
			SourceTenants: rule.SourceTenants,
		},
	}

	err := r.MimirClient.CreateRuleGroup(context.Background(), mimirRuleNamespace, mimirRule)
	if err != nil {
		log.Error(err, "Failed to create rule group")
		return err
	}

	// TODO: move finalizer addition here

	return nil
}

func (r *MimirRuleReconciler) getMimirRuleGroupAPI(log logr.Logger, rule *monitoringv1.PrometheusRule) *rwrulefmt.RuleGroup {
	mimirRuleGroup, err := r.MimirClient.GetRuleGroup(context.Background(), mimirRuleNamespace, rule.Name)
	if err != nil {
		log.Error(err, "Failed to get rule group")
		return nil
	}

	return mimirRuleGroup
}

//func (r *MimirRuleReconciler) createMimirRuleGroup(log logr.Logger, mimirClient *mimirclient.MimirClient, rule *monitoringv1.PrometheusRule, ds *openslov1.Datasource) error {
//	mimirRuleGroup, err := helpers.NewMimirRuleGroup(rule)
//	if err != nil {
//		log.Error(err, "Failed to create Mimir rule group")
//		return err
//	}
//
//	if err := mimirClient.CreateRuleGroup(context.Background(), mimirRuleNamespace, *mimirRuleGroup); err != nil {
//		log.Error(err, "Failed to create rule group")
//		return err
//	}
//
//	return nil
//}

func (r *MimirRuleReconciler) deleteMimirRuleGroupAPI(log logr.Logger, mimirClient *mimirclient.MimirClient, ruleGroup *rwrulefmt.RuleGroup) error {
	if err := mimirClient.DeleteRuleGroup(context.Background(), mimirRuleNamespace, ruleGroup.Name); err != nil {
		log.Error(err, "Failed to delete rule group")
		return err
	}

	return nil
}

func (r *MimirRuleReconciler) addFinalizer(log logr.Logger, rule *oskov1alpha1.MimirRule) error {
	log.Info("Adding Finalizer for the MimirRule")
	controllerutil.AddFinalizer(rule, mimirRuleFinalizer)

	err := r.Update(context.Background(), rule)
	if err != nil {
		log.Error(err, "Failed to update MimirRule with finalizer")
		return err
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MimirRuleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&oskov1alpha1.MimirRule{}).
		Complete(r)
}
