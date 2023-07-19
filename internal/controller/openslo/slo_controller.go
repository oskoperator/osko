package controller

import (
	"context"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	openslov1 "github.com/oskoperator/osko/apis/openslo/v1"
)

// SLOReconciler reconciles a SLO object
type SLOReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=openslo.openslo,resources=slos,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=openslo.openslo,resources=slos/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=openslo.openslo,resources=slos/finalizers,verbs=update

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
	logger := log.FromContext(ctx)

	// TODO(user): your logic here

	// TODO(user): your logic here
	var slo openslov1.SLO

	err := r.Get(ctx, req.NamespacedName, &slo)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// This is what happens when manifest is deleted??
			logger.Info("SLO deleted")
			return ctrl.Result{}, nil
		}

		logger.Error(err, errGetDS)
		return ctrl.Result{}, nil
	}

	if slo.Annotations["oskoperator.com/implementation"] == "mimir" {
		logger.Info("This is the SLO Annotations", "Annotations", "It's Mimir biatch!")
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SLOReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&openslov1.SLO{}).
		Complete(r)
}
