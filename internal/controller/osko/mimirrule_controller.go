package osko

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	mimirclient "github.com/grafana/mimir/pkg/mimirtool/client"
	"github.com/grafana/mimir/pkg/mimirtool/rules/rwrulefmt"
	openslov1 "github.com/oskoperator/osko/api/openslo/v1"
	"github.com/oskoperator/osko/internal/helpers"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/prometheus/prometheus/model/rulefmt"
	"gopkg.in/yaml.v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
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
	Scheme             *runtime.Scheme
	Recorder           record.EventRecorder
	MimirClient        *mimirclient.MimirClient
	RequeueAfterPeriod time.Duration
}

const (
	mimirRuleNamespace      = "osko"
	mimirRuleFinalizer      = "finalizer.mimir.osko.dev"
	prometheusRuleFinalizer = "finalizer.prometheusrule.osko.dev"

	errFinalizerAddFailed    = "Failed to add the finalizer to the"
	errFinalizerRemoveFailed = "Failed to remove the finalizer from the"
)

// +kubebuilder:rbac:groups=osko.dev,resources=mimirrules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osko.dev,resources=mimirrules/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=osko.dev,resources=mimirrules/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

func (r *MimirRuleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	slo := &openslov1.SLO{}
	prometheusRule := &monitoringv1.PrometheusRule{}
	mimirRule := &oskov1alpha1.MimirRule{}
	newMimirRule := &oskov1alpha1.MimirRule{}

	err := r.Get(ctx, req.NamespacedName, prometheusRule)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("PrometheusRule resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, err
		} else {
			log.Error(err, "Failed to get PrometheusRule")
			return ctrl.Result{}, err
		}
	}

	err = r.Get(ctx, req.NamespacedName, mimirRule)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("MimirRule resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get MimirRule")
		return ctrl.Result{}, err
	}

	if err := r.newMimirClient(mimirRule.Spec.ConnectionDetails); err != nil {
		log.Error(err, "Failed to create MimirClient")
		return ctrl.Result{}, err
	}

	// TODO: This logic is total bullshit. We should revise the reconciliation logic and make it more clear.
	rgs, err := helpers.NewMimirRuleGroups(prometheusRule, mimirRule.Spec.ConnectionDetails)
	if err != nil {
		log.Error(err, "Failed to convert MimirRuleGroup")
	}

	isMimirRuleMarkedToBeDeleted := mimirRule.GetDeletionTimestamp() != nil
	if isMimirRuleMarkedToBeDeleted {
		for _, rg := range rgs {
			if err := r.deleteMimirRuleGroupAPI(log, rg.Name); err != nil {
				log.Error(err, "Failed to delete MimirRule from the Mimir API")
				return ctrl.Result{}, err
			}
		}
		if controllerutil.ContainsFinalizer(mimirRule, mimirRuleFinalizer) {
			controllerutil.RemoveFinalizer(mimirRule, mimirRuleFinalizer)
			if err := r.Update(ctx, mimirRule); err != nil {
				log.Error(err, fmt.Sprintf("%s MimirRule", errFinalizerRemoveFailed))
				return ctrl.Result{}, err
			}
		}
		if controllerutil.ContainsFinalizer(prometheusRule, prometheusRuleFinalizer) {
			controllerutil.RemoveFinalizer(prometheusRule, prometheusRuleFinalizer)
			if err := r.Update(ctx, prometheusRule); err != nil {
				log.Error(err, fmt.Sprintf("%s PrometheusRule", errFinalizerRemoveFailed))
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if apierrors.IsNotFound(err) {
		log.V(1).Info("MimirRule not found. Let's make one.")
		mimirRule, err = helpers.NewMimirRule(slo, prometheusRule, mimirRule.Spec.ConnectionDetails)

		if err = r.Create(ctx, mimirRule); err != nil {
			r.Recorder.Event(mimirRule, "Error", "FailedToCreateMimirRule", "Failed to create Mimir Rule")
			if err = r.Status().Update(ctx, mimirRule); err != nil {
				log.Error(err, "Failed to update MimirRule status")
				return ctrl.Result{}, err
			} else {
				log.V(1).Info("MimirRule created successfully")
				r.Recorder.Event(mimirRule, "Normal", "MimirRuleCreated", "MimirRule created successfully")
				mimirRule.Status.Ready = "True"
				if err := r.Status().Update(ctx, mimirRule); err != nil {
					log.Error(err, "Failed to update MimirRule ready status")
					return ctrl.Result{}, err
				}
				return ctrl.Result{RequeueAfter: r.RequeueAfterPeriod}, nil
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

	for _, rg := range rgs {
		if err := r.createMimirRuleGroupAPI(log, &rg); err != nil {
			log.Error(err, "Failed to create MimirRuleGroup")
			return ctrl.Result{}, err
		}
	}

	if !controllerutil.ContainsFinalizer(mimirRule, mimirRuleFinalizer) {
		controllerutil.AddFinalizer(mimirRule, mimirRuleFinalizer)
		if err := r.Update(ctx, mimirRule); err != nil {
			log.Error(err, fmt.Sprintf("%s MimirRule", errFinalizerAddFailed))
			return ctrl.Result{}, err
		}
	}

	if !controllerutil.ContainsFinalizer(prometheusRule, prometheusRuleFinalizer) {
		controllerutil.AddFinalizer(prometheusRule, prometheusRuleFinalizer)
		if err := r.Update(ctx, prometheusRule); err != nil {
			log.Error(err, fmt.Sprintf("%s PrometheusRule", errFinalizerAddFailed))
			return ctrl.Result{}, err
		}
	}

	log.V(1).Info("MimirRule already exists, we should update it.")
	newMimirRule, err = helpers.NewMimirRule(slo, prometheusRule, mimirRule.Spec.ConnectionDetails)
	if err != nil {
		log.Error(err, "Failed to create new MimirRule")
		return ctrl.Result{}, err
	}

	compareResult := reflect.DeepEqual(mimirRule.Spec, newMimirRule.Spec)
	if compareResult {
		log.V(1).Info("MimirRule is up to date")
		return ctrl.Result{RequeueAfter: r.RequeueAfterPeriod}, nil
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

	mimirRule.Status.Ready = "True"
	if err := r.Status().Update(ctx, mimirRule); err != nil {
		log.Error(err, "Failed to update MimirRule ready status")
	}

	r.Recorder.Event(mimirRule, "Normal", "MimirRuleUpdated", "MimirRule updated successfully")

	log.V(1).Info("MimirRule reconciled")
	return ctrl.Result{RequeueAfter: r.RequeueAfterPeriod}, nil
}

func (r *MimirRuleReconciler) newMimirClient(connectionDetails oskov1alpha1.ConnectionDetails) error {
	mClientConfig := helpers.MimirClientConfig{
		Address:  connectionDetails.Address,
		TenantId: connectionDetails.TargetTenant,
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
	mimirRuleNode := rulefmt.RuleNode{}
	for _, r := range rule.Rules {
		if r.Alert == "" {
			mimirRuleNode = rulefmt.RuleNode{
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
		} else {
			mimirRuleNode = rulefmt.RuleNode{
				Alert: yaml.Node{
					Kind:  8,
					Value: r.Alert,
				},
				Expr: yaml.Node{
					Kind:  8,
					Value: r.Expr,
				},
				For:    r.For,
				Labels: r.Labels,
			}
		}
		mimirRuleNodes = append(mimirRuleNodes, mimirRuleNode)
	}

	log.V(1).Info("Source tenants", "SourceTenants", rule.SourceTenants)

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

	return nil
}

func (r *MimirRuleReconciler) deleteMimirRuleGroupAPI(log logr.Logger, name string) error {
	if err := r.MimirClient.DeleteRuleGroup(context.Background(), mimirRuleNamespace, name); err != nil {
		log.Error(err, "Failed to delete rule group")
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
