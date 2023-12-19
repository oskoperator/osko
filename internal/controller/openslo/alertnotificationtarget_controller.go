package controller

import (
	"context"
	"github.com/oskoperator/osko/internal/helpers"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	openslov1 "github.com/oskoperator/osko/api/openslo/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AlertNotificationTargetReconciler reconciles a AlertNotificationTarget object
type AlertNotificationTargetReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=openslo.com,resources=alertnotificationtargets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=openslo.com,resources=alertnotificationtargets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=openslo.com,resources=alertnotificationtargets/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the AlertNotificationTarget object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *AlertNotificationTargetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	alertNotificationTarget := &openslov1.AlertNotificationTarget{}
	alertmanagerConfig := &monitoringv1alpha1.AlertmanagerConfig{}

	err := r.Get(ctx, req.NamespacedName, alertNotificationTarget)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("AlertNotificationTarget resource not found. Object must have been deleted.")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get AlertNotificationTarget")
		return ctrl.Result{}, nil
	}

	err = r.Get(ctx, types.NamespacedName{
		Name:      alertNotificationTarget.Name,
		Namespace: alertNotificationTarget.Namespace,
	}, alertmanagerConfig)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Creating AlertNotificationTarget.")
			alertmanagerConfig, err = helpers.CreateAlertManagerConfig(alertNotificationTarget)
			if err != nil {
				log.Error(err, "Failed to create AlertManagerConfig")
				return ctrl.Result{}, nil
			}

			if err = r.Create(ctx, alertmanagerConfig); err != nil {
				log.Error(err, "Failed to create AlertManagerConfig")
				return ctrl.Result{}, nil
			}
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get AlertManagerConfig")
		return ctrl.Result{}, err
	}

	log.Info("AlertNotificationTarget reconciled.")

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AlertNotificationTargetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&openslov1.AlertNotificationTarget{}).
		Owns(&monitoringv1alpha1.AlertmanagerConfig{}).
		Complete(r)
}
