package osko

import (
	"context"
	mimirclient "github.com/grafana/mimir/pkg/mimirtool/client"
	"github.com/grafana/mimir/pkg/mimirtool/rules/rwrulefmt"
	openslov1 "github.com/oskoperator/osko/api/openslo/v1"
	"github.com/oskoperator/osko/internal/helpers"
	"github.com/oskoperator/osko/internal/mimirtool"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"reflect"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	oskov1alpha1 "github.com/oskoperator/osko/api/osko/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MimirRuleReconciler reconciles a MimirRule object
type MimirRuleReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=osko.openslo,resources=mimirrules,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=osko.openslo,resources=mimirrules/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=osko.openslo,resources=mimirrules/finalizers,verbs=update

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

	slo := &openslov1.SLO{}
	sli := &openslov1.SLI{}
	prometheusRule := &monitoringv1.PrometheusRule{}
	newPrometheusRule := &monitoringv1.PrometheusRule{}
	mimirRuleGroup := &rwrulefmt.RuleGroup{}
	newMimirRuleGroup := &rwrulefmt.RuleGroup{}
	mimirClient := &mimirclient.MimirClient{}
	mimirRule := &oskov1alpha1.MimirRule{}

	err := r.Get(ctx, req.NamespacedName, mimirRule)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("MimirRule resource not found. Ignoring since object mus be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get MimirRule")
		return ctrl.Result{}, err
	}

	err = r.Get(ctx, req.NamespacedName, prometheusRule)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("PrometheusRule resource not found. Ignoring since object mus be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get PrometheusRule")
		return ctrl.Result{}, err
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

	mimirRuleGroup = helpers.GetMimirRuleGroup(log, mimirClient, prometheusRule)

	if apierrors.IsNotFound(err) {
		log.Info("PrometheusRule not found. Let's make one.")
		prometheusRule, err = helpers.CreatePrometheusRule(slo, sli)
		if err != nil {
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
			err := helpers.UpdateMimirRuleGroup(log, mimirClient, mimirRuleGroup, newMimirRuleGroup)
			if err != nil {
				log.Error(err, "Failed to update Mimir rule group")
				return ctrl.Result{}, err
			}
		}
	}

	err = helpers.CreateMimirRuleGroup(log, mimirClient, prometheusRule, ds)
	if err != nil {
		log.Error(err, "Failed to create Mimir rule")
		return ctrl.Result{}, err
	}

	log.Info("MimirRule reconciled")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MimirRuleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&oskov1alpha1.MimirRule{}).
		Complete(r)
}
