package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	openslov1 "github.com/oskoperator/osko/api/openslo/v1"
)

// AlertNotificationTargetReconciler reconciles a AlertNotificationTarget object
type AlertNotificationTargetReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=openslo.com,resources=alertnotificationtargets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=openslo.com,resources=alertnotificationtargets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=openslo.com,resources=alertnotificationtargets/finalizers,verbs=update

// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *AlertNotificationTargetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AlertNotificationTargetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&openslov1.AlertNotificationTarget{}).
		Complete(r)
}
