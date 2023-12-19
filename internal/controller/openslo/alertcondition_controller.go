package controller

import (
	"context"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	openslov1 "github.com/oskoperator/osko/api/openslo/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AlertConditionReconciler reconciles a AlertCondition object
type AlertConditionReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

const (
	errGetAlertCondition = "unable to get AlertCondition"
)

//+kubebuilder:rbac:groups=openslo.com,resources=alertconditions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=openslo.com,resources=alertconditions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=openslo.com,resources=alertconditions/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the AlertCondition object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *AlertConditionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	alertCondition := &openslov1.AlertCondition{}
	err := r.Get(ctx, req.NamespacedName, alertCondition)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("AlertCondition resource not found. Object must have been deleted.")
			return ctrl.Result{}, nil
		}

		log.Error(err, errGetAlertCondition)
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AlertConditionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&openslov1.AlertCondition{}).
		Owns(&openslov1.AlertNotificationTarget{}).
		Complete(r)
}
