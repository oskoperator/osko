package controller

import (
	"context"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	openslov1 "github.com/oskoperator/osko/api/openslo/v1"
)

const (
	errGetSLI = "could not get SLI Object"
)

// SLIReconciler reconciles a SLI object
type SLIReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=openslo.com,resources=slis,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=openslo.com,resources=slis/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=openslo.com,resources=slis/finalizers,verbs=update

func (r *SLIReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	sli := &openslov1.SLI{}
	err := r.Get(ctx, req.NamespacedName, sli)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("SLI resource not found. Object must have been deleted.")
			return ctrl.Result{}, nil
		}

		log.Error(err, errGetSLI)
		return ctrl.Result{}, nil
	}

	log.V(1).Info("SLI reconciled", "SLI Name", sli.Name, "SLI Namespace", sli.Namespace)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SLIReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&openslov1.SLI{}).
		Complete(r)
}
