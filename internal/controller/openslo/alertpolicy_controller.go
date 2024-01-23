package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	openslov1 "github.com/oskoperator/osko/api/openslo/v1"
)

// AlertPolicyReconciler reconciles a AlertPolicy object
type AlertPolicyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=openslo.com,resources=alertpolicies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=openslo.com,resources=alertpolicies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=openslo.com,resources=alertpolicies/finalizers,verbs=update

func (r *AlertPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AlertPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&openslov1.AlertPolicy{}).
		Complete(r)
}
