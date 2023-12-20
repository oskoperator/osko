package osko

import (
	"context"
	"github.com/go-logr/logr"
	mimirclient "github.com/grafana/mimir/pkg/mimirtool/client"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	oskov1alpha1 "github.com/oskoperator/osko/api/osko/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

// MimirAlertManagerReconciler reconciles a MimirAlertManager object
type MimirAlertManagerReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	MimirClient *mimirclient.MimirClient
}

const (
	errGetMAM = "Failed to get MimirAlertManager"
)

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
	log := ctrllog.FromContext(ctx)

	mimirAlertManager := &oskov1alpha1.MimirAlertManager{}

	err := r.Get(ctx, req.NamespacedName, mimirAlertManager)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("MimirAlertManager resource not found. Object must have been deleted.")
			return ctrl.Result{}, nil
		}
		log.Error(err, errGetMAM)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *MimirAlertManagerReconciler) createMimirAlertManager(log logr.Logger, alertmanagerConfig *monitoringv1alpha1.AlertmanagerConfig) error {
	log.Info("Creating MimirAlertManager")

	if err := r.MimirClient.CreateAlertmanagerConfig(context.Background(), alertmanagerConfig.Name, nil); err != nil {
		log.Error(err, "Failed to create MimirAlertManager")
		return err
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MimirAlertManagerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&oskov1alpha1.MimirAlertManager{}).
		Complete(r)
}
