package controller

import (
	"context"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	openslov1 "github.com/oskoperator/osko/apis/openslo/v1"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
)

// SLOReconciler reconciles a SLO object
type SLOReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=openslo.com,resources=slos,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=openslo.com,resources=slos/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=openslo.com,resources=slos/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the SLO object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *SLOReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)
	log := log.FromContext(ctx)

	var slo openslov1.SLO

	err := r.Get(ctx, req.NamespacedName, &slo)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("SLO deleted")
			return ctrl.Result{}, nil
		}

		log.Error(err, errGetDS)
		return ctrl.Result{}, nil
	}

	// Create PrometheusRule from SLO
	rule := &monitoringv1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      slo.Name,
			Namespace: slo.Namespace,
		},
		Spec: monitoringv1.PrometheusRuleSpec{
			Groups: []monitoringv1.RuleGroup{{
				Name: slo.Name,
				Rules: []monitoringv1.Rule{{
					Alert: slo.Spec.AlertPolicies[0].Metadata.Name,
				}}}},
		},
	}

	// Set SLO instance as the owner and controller.
	if err := ctrl.SetControllerReference(&slo, rule, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	// Check if this PrometheusRule already exists
	found := &monitoringv1.PrometheusRule{}
	err = r.Get(ctx, client.ObjectKey{
		Namespace: slo.Namespace,
		Name:      slo.Name,
	}, found)

	if err != nil && apierrors.IsNotFound(err) {
		err = r.Create(ctx, rule)
		if err != nil {
			log.Error(err, "Failed to create new PrometheusRule", "PrometheusRule.Namespace", rule.Namespace, "PrometheusRule.Name", rule.Name)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	log.Info("SLO reconciled", "SLO Name", slo.Name, "SLO Namespace", slo.Namespace)

	//log.Info("SLO reconciled")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SLOReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&openslov1.SLO{}).
		Owns(&monitoringv1.PrometheusRule{}).
		Complete(r)
}
