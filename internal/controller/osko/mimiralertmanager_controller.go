package osko

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	oskov1alpha1 "github.com/oskoperator/osko/api/osko/v1alpha1"
)

// MimirAlertManagerReconciler reconciles a MimirAlertManager object
type MimirAlertManagerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=osko.openslo,resources=mimiralertmanagers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=osko.openslo,resources=mimiralertmanagers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=osko.openslo,resources=mimiralertmanagers/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the MimirAlertManager object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile
func (r *MimirAlertManagerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// TODO(user): your logic here

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MimirAlertManagerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&oskov1alpha1.MimirAlertManager{}).
		Complete(r)
}
