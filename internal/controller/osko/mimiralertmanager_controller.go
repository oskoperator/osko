package osko

import (
	"context"
	"github.com/go-logr/logr"
	mimirclient "github.com/grafana/mimir/pkg/mimirtool/client"
	"github.com/oskoperator/osko/internal/helpers"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"gopkg.in/yaml.v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"os"

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
	Recorder    record.EventRecorder
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
	alertmanagerConfig := &monitoringv1alpha1.AlertmanagerConfig{}

	err := r.Get(ctx, req.NamespacedName, mimirAlertManager)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("MimirAlertManager resource not found. Object must have been deleted.")
			return ctrl.Result{}, nil
		}
		log.Error(err, errGetMAM)
		return ctrl.Result{}, err
	}

	err = r.Get(ctx, req.NamespacedName, alertmanagerConfig)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("AlertmanagerConfig resource not found. Object must have been deleted.")
			return ctrl.Result{}, nil
		}
		log.Error(err, errGetMAM)
		return ctrl.Result{}, err
	}

	// TODO: Hardcoded for now, should be taken from annotation or datasource
	if err := r.newMimirClient("https://mimir.monitoring.dev.heu.group", "billing"); err != nil {
		log.Error(err, "Failed to create MimirClient")
		return ctrl.Result{}, err
	}

	if err := r.createMimirAlertManagerAPI(log, mimirAlertManager); err != nil {
		log.Error(err, "Failed to create MimirAlertManagerAPI")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *MimirAlertManagerReconciler) newMimirClient(address string, tenantId string) error {
	mClientConfig := helpers.MimirClientConfig{
		Address:  address,
		TenantId: tenantId,
	}

	mimirClient, err := mClientConfig.NewMimirClient()
	if err != nil {
		return err
	}

	r.MimirClient = mimirClient

	return nil
}

func (r *MimirAlertManagerReconciler) createMimirAlertManagerAPI(log logr.Logger, alertmanagerConfig *oskov1alpha1.MimirAlertManager) error {
	log.Info("Creating MimirAlertManager")

	alertmanagerConfigYAML, err := yaml.Marshal(alertmanagerConfig.Spec)
	if err != nil {
		log.Error(err, "Failed to marshal AlertmanagerConfig")
		return err
	}

	if err := os.WriteFile("/tmp/alertmanager.yaml", alertmanagerConfigYAML, 0644); err != nil {
		log.Error(err, "Failed to open file")
		return err
	}

	//TODO: New type for MimirAlertManager so I can push it to Mimir

	log.V(1).Info("AlertmanagerConfig YAML", "YAML", string(alertmanagerConfigYAML))

	//if err := r.MimirClient.CreateAlertmanagerConfig(context.Background(), string(alertmanagerConfigYAML), nil); err != nil {
	//	log.Error(err, "Failed to create MimirAlertManager")
	//	return err
	//}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MimirAlertManagerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&oskov1alpha1.MimirAlertManager{}).
		Complete(r)
}
