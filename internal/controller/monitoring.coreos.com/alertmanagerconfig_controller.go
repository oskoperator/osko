package monitoringcoreoscom

import (
	"context"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

// AlertManagerConfigReconciler reconciles a AlertManagerConfig object
type AlertManagerConfigReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

const (
	errGetAMC = "Failed to get AlertManagerConfig"
)

//+kubebuilder:rbac:groups=monitoring.coreos.com.openslo,resources=alertmanagerconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=monitoring.coreos.com.openslo,resources=alertmanagerconfigs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=monitoring.coreos.com.openslo,resources=alertmanagerconfigs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the AlertManagerConfig object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile
func (r *AlertManagerConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	alertManagerConfig := &monitoringv1alpha1.AlertmanagerConfig{}

	err := r.Get(ctx, req.NamespacedName, alertManagerConfig)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("AlertManagerConfig resource not found. Object must have been deleted.")
			return ctrl.Result{}, nil
		}
		log.Error(err, errGetAMC)
		return ctrl.Result{}, err
	}

	log.Info("AlertmanagerConfig reconciled.")

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AlertManagerConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&monitoringv1alpha1.AlertmanagerConfig{}).
		Complete(r)
}
