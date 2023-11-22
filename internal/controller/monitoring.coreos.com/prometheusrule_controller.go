package monitoringcoreoscom

import (
	"context"
	"github.com/go-logr/logr"
	mimirclient "github.com/grafana/mimir/pkg/mimirtool/client"
	"github.com/grafana/mimir/pkg/mimirtool/rules/rwrulefmt"
	openslov1 "github.com/oskoperator/osko/api/openslo/v1"
	"github.com/oskoperator/osko/internal/helpers"
	"github.com/oskoperator/osko/internal/mimirtool"
	"github.com/oskoperator/osko/internal/utils"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	mimirRuleNamespace = "osko"
	objectiveRef       = ".metaData.ownerReferences.name"
	datasourceRef      = ".metaData.annotations.datasource"
	mimirRuleFinalizer = "finalizer.mimir.osko.dev"
)

// PrometheusRuleReconciler reconciles a PrometheusRule object
type PrometheusRuleReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=prometheusrules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=prometheusrules/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=prometheusrules/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

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
	log := ctrllog.FromContext(ctx)

	slo := &openslov1.SLO{}
	sli := &openslov1.SLI{}
	prometheusRule := &monitoringv1.PrometheusRule{}
	newPrometheusRule := &monitoringv1.PrometheusRule{}
	mimirRuleGroup := &rwrulefmt.RuleGroup{}
	newMimirRuleGroup := &rwrulefmt.RuleGroup{}
	mimirClient := &mimirclient.MimirClient{}

	err := r.Get(ctx, req.NamespacedName, prometheusRule)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("PrometheusRule resource not found. Ignoring since object mus be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get PrometheusRule")
		return ctrl.Result{}, err
	}

	for _, ref := range prometheusRule.ObjectMeta.OwnerReferences {
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

	ds := &openslov1.Datasource{}
	if err := r.Get(ctx, client.ObjectKey{
		Namespace: prometheusRule.Namespace,
		Name:      "logging-ds",
	}, ds); err != nil {
		log.Error(err, "Failed to get Datasource")
		return ctrl.Result{}, err
	}

	mClient := mimirtool.MimirClientConfig{
		Address:  ds.Spec.ConnectionDetails.Address,
		TenantId: ds.Spec.ConnectionDetails.TargetTenant,
	}

	mimirClient, err = mClient.NewMimirClient()
	if err != nil {
		log.Error(err, "Failed to create Mimir client")
		return ctrl.Result{}, err
	}

	mimirRuleGroup = r.getMimirRuleGroup(log, mimirClient, prometheusRule)

	isPrometheusRuleMarkedToBeDeleted := prometheusRule.GetDeletionTimestamp() != nil
	if isPrometheusRuleMarkedToBeDeleted {
		log.Info("--- PrometheusRule is marked to be deleted ---")
		if utils.ContainString(prometheusRule.GetFinalizers(), mimirRuleFinalizer) {
			err := r.deleteMimirRuleGroup(log, mimirClient, mimirRuleGroup)
			if err != nil {
				return ctrl.Result{}, err
			}
			log.Info("PrometheusRule finalizers completed")
			controllerutil.RemoveFinalizer(prometheusRule, mimirRuleFinalizer)
			err = r.Update(ctx, prometheusRule)
			if err != nil {
				return ctrl.Result{}, err
			}
			log.Info("PrometheusRule can be deleted now")
			return ctrl.Result{}, nil
		}
	}

	if !utils.ContainString(prometheusRule.GetFinalizers(), mimirRuleFinalizer) {
		if err := r.addFinalizer(log, prometheusRule); err != nil {
			return ctrl.Result{}, err
		}
	}

	if apierrors.IsNotFound(err) {
		log.Info("PrometheusRule not found. Let's make one.")
		prometheusRule, err = helpers.CreatePrometheusRule(slo, sli)
		if err != nil {
			err = utils.UpdateStatus(
				ctx,
				slo,
				r.Client,
				"Ready",
				metav1.ConditionFalse,
				"FailedToCreatePrometheusRule",
				"Failed to create Prometheus Rule",
			)
			if err != nil {
				log.Error(err, "Failed to update SLO status")
				return ctrl.Result{}, err
			}
			log.Error(err, "Failed to create new PrometheusRule")
			return ctrl.Result{}, err
		}
		if err := r.Create(ctx, prometheusRule); err != nil {
			r.Recorder.Event(slo, "Error", "FailedToCreatePrometheusRule", "Failed to create Prometheus Rule")
			if err := r.Status().Update(ctx, prometheusRule); err != nil {
				log.Error(err, "Failed to update SLO status")
				slo.Status.Ready = "Failed"
				if err := r.Status().Update(ctx, slo); err != nil {
					log.Error(err, "Failed to update SLO ready status")
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, err
			}
		} else {
			// This is the main logic for the PrometheusRule update
			// Here we should take the existing PrometheusRule and update it with the new one
			log.Info("PrometheusRule already exists, we should update it")
			newPrometheusRule, err = helpers.CreatePrometheusRule(slo, sli)
			if err != nil {
				log.Error(err, "Failed to create new PrometheusRule")
				return ctrl.Result{}, err
			}
			newMimirRuleGroup, err = mimirtool.NewMimirRuleGroup(prometheusRule, ds)
			if err != nil {
				log.Error(err, "Failed to create new Mimir rule group")
				return ctrl.Result{}, err
			}

			compareResult := reflect.DeepEqual(prometheusRule, newPrometheusRule)
			if compareResult {
				log.Info("PrometheusRule is already up to date")
				return ctrl.Result{}, nil
			}

			// has to be the same as for previous object, otherwise it will not be updated and throw an error
			newPrometheusRule.ResourceVersion = prometheusRule.ResourceVersion

			log.Info("Updating PrometheusRule", "PrometheusRule Name", newPrometheusRule.Name, "PrometheusRule Namespace", newPrometheusRule.Namespace)
			if err := r.Update(ctx, newPrometheusRule); err != nil {
				log.Error(err, "Failed to update PrometheusRule")
				return ctrl.Result{}, err
			}
			if err := r.Status().Update(ctx, slo); err != nil {
				log.Error(err, "Failed to update SLO status")
				slo.Status.Ready = "Failed"
				if err := r.Status().Update(ctx, slo); err != nil {
					log.Error(err, "Failed to update SLO ready status")
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, err
			}
			err := r.updateMimirRuleGroup(log, mimirClient, mimirRuleGroup, newMimirRuleGroup)
			if err != nil {
				log.Error(err, "Failed to update Mimir rule group")
				return ctrl.Result{}, err
			}
		}
	}

	err = r.createMimirRuleGroup(log, mimirClient, prometheusRule, ds)
	if err != nil {
		log.Error(err, "Failed to create Mimir rule")
		return ctrl.Result{}, err
	}

	log.Info("PrometheusRule reconciled")
	return ctrl.Result{}, nil
}

func (r *PrometheusRuleReconciler) getMimirRuleGroup(log logr.Logger, mimirClient *mimirclient.MimirClient, rule *monitoringv1.PrometheusRule) *rwrulefmt.RuleGroup {
	mimirRuleGroup, err := mimirClient.GetRuleGroup(context.Background(), mimirRuleNamespace, rule.Name)
	if err != nil {
		log.Error(err, "Failed to get rule group")
		return nil
	}

	return mimirRuleGroup
}

func (r *PrometheusRuleReconciler) createMimirRuleGroup(log logr.Logger, mimirClient *mimirclient.MimirClient, rule *monitoringv1.PrometheusRule, ds *openslov1.Datasource) error {
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

func (r *PrometheusRuleReconciler) deleteMimirRuleGroup(log logr.Logger, mimirClient *mimirclient.MimirClient, ruleGroup *rwrulefmt.RuleGroup) error {
	if err := mimirClient.DeleteRuleGroup(context.Background(), mimirRuleNamespace, ruleGroup.Name); err != nil {
		log.Error(err, "Failed to delete rule group")
		return err
	}

	return nil
}

func (r *PrometheusRuleReconciler) updateMimirRuleGroup(log logr.Logger, mimirClient *mimirclient.MimirClient, existingGroup *rwrulefmt.RuleGroup, desiredGroup *rwrulefmt.RuleGroup) error {
	log.Info("Updating Mimir rule group")
	if reflect.DeepEqual(existingGroup, desiredGroup) {
		log.Info("Mimir rule group is already up to date")
		return nil
	}
	err := r.deleteMimirRuleGroup(log, mimirClient, existingGroup)
	if err != nil {
		return err
	}
	return nil
}

func (r *PrometheusRuleReconciler) addFinalizer(log logr.Logger, rule *monitoringv1.PrometheusRule) error {
	log.Info("Adding Finalizer for the PrometheusRule")
	controllerutil.AddFinalizer(rule, mimirRuleFinalizer)

	err := r.Update(context.Background(), rule)
	if err != nil {
		log.Error(err, "Failed to update PrometheusRule with finalizer")
		return err
	}
	return nil
}

func (r *PrometheusRuleReconciler) createIndices(mgr ctrl.Manager) error {
	return mgr.GetFieldIndexer().IndexField(
		context.TODO(),
		&monitoringv1.PrometheusRule{},
		objectiveRef,
		func(object client.Object) []string {
			pr := object.(*monitoringv1.PrometheusRule)
			if pr.ObjectMeta.OwnerReferences == nil {
				return nil
			}
			return []string{pr.ObjectMeta.OwnerReferences[0].Name}
		})
}

func (r *PrometheusRuleReconciler) findObjectsForSlo() func(ctx context.Context, a client.Object) []reconcile.Request {
	return func(ctx context.Context, a client.Object) []reconcile.Request {
		attachedSLOs := &openslov1.SLOList{}
		listOpts := &client.ListOptions{
			FieldSelector: fields.OneTermEqualSelector(objectiveRef, a.GetName()),
			Namespace:     a.GetNamespace(),
		}
		err := r.List(ctx, attachedSLOs, listOpts)
		if err != nil {
			return []reconcile.Request{}
		}

		requests := make([]reconcile.Request, len(attachedSLOs.Items))
		for i, item := range attachedSLOs.Items {
			requests[i] = reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      item.Name,
					Namespace: item.Namespace,
				},
			}
		}
		return requests
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *PrometheusRuleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := r.createIndices(mgr); err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&monitoringv1.PrometheusRule{}).
		Watches(
			&openslov1.SLO{},
			handler.EnqueueRequestsFromMapFunc(r.findObjectsForSlo()),
		).Watches(
		&openslov1.Datasource{},
		handler.EnqueueRequestsFromMapFunc(r.findObjectsForSlo())).
		Complete(r)
}
