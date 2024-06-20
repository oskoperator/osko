package osko

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"time"

	mimirclient "github.com/grafana/mimir/pkg/mimirtool/client"
	"github.com/oskoperator/osko/internal/helpers"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	openslov1 "github.com/oskoperator/osko/api/openslo/v1"
	oskov1alpha1 "github.com/oskoperator/osko/api/osko/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AlertManagerConfigReconciler reconciles a AlertManagerConfig object
type AlertManagerConfigReconciler struct {
	client.Client
	Scheme               *runtime.Scheme
	Recorder             record.EventRecorder
	MimirClient          *mimirclient.MimirClient
	SecretNamespacedName types.NamespacedName
}

const (
	errGetAMC = "Failed to get AlertmanagerConfig"
)

// +kubebuilder:rbac:groups=osko.dev,resources=alertmanagerconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osko.dev,resources=alertmanagerconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=osko.dev,resources=alertmanagerconfigs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the AlertManagerConfig object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.18.2/pkg/reconcile
func (r *AlertManagerConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	amc := &oskov1alpha1.AlertManagerConfig{}
	ds := &openslov1.Datasource{}
	secret := &corev1.Secret{}

	err := r.Get(ctx, req.NamespacedName, amc)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("MimirAlertManager resource not found. Object must have been deleted.")
			return ctrl.Result{}, nil
		}
		log.Error(err, errGetAMC)
		return ctrl.Result{}, err
	}

	if apierrors.IsNotFound(err) {
		log.V(1).Info("AlertmanagerConfig resource not found. Creating new AlertmanagerConfig")
		if err = r.Create(ctx, amc); err != nil {
			log.Error(err, "Failed to create AlertmanagerConfig")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Get DS from AMC's ref
	err = r.Get(ctx, client.ObjectKey{Name: amc.ObjectMeta.Annotations["osko.dev/datasourceRef"], Namespace: amc.Namespace}, ds)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info(fmt.Sprintf("datasourceRef: %v", "errGetDS"))
			//amc.Status.Ready = "False"
			r.Recorder.Event(amc, "Warning", "datasourceRef", "errDatasourceRef")
			if err := r.Status().Update(ctx, amc); err != nil {
				log.Error(err, "Failed to update amc ready status")
				return ctrl.Result{}, err
			}
			return ctrl.Result{RequeueAfter: time.Second * 5}, nil
		}
		log.Error(err, "errGetDS")
		return ctrl.Result{}, err
	}

	if amc.Spec.SecretRef.Namespace == "" {
		amc.Spec.SecretRef.Namespace = req.Namespace
	}

	err = r.Get(ctx, client.ObjectKey{Namespace: amc.Spec.SecretRef.Namespace, Name: amc.Spec.SecretRef.Name}, secret)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Error(err, "Secret not found")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get secret")
		return ctrl.Result{}, nil
	}

	r.SecretNamespacedName = types.NamespacedName{
		Name:      secret.Name,
		Namespace: secret.Namespace,
	}

	//log.V(1).Info("Getting secret", "secretData", secret.Data["alertmanagerconfig.yaml"])

	yamlData, ok := secret.Data["alertmanagerconfig.yaml"]
	if !ok {
		log.Error(err, "alertmanagerconfig.yaml not found in secret")
		return ctrl.Result{}, nil
	}

	mClient := helpers.MimirClientConfig{
		Address:  ds.Spec.ConnectionDetails.Address,
		TenantId: ds.Spec.ConnectionDetails.TargetTenant,
	}

	r.MimirClient, err = mClient.NewMimirClient()

	err = r.MimirClient.CreateAlertmanagerConfig(ctx, string(yamlData), nil)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *AlertManagerConfigReconciler) findObjectsForSecret(secretNamespacedName types.NamespacedName) func(ctx context.Context, a client.Object) []reconcile.Request {
	return func(ctx context.Context, a client.Object) []reconcile.Request {
		if secretNamespacedName == (types.NamespacedName{}) {
			return []reconcile.Request{}
		}
		return []reconcile.Request{{NamespacedName: secretNamespacedName}}
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *AlertManagerConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&oskov1alpha1.AlertManagerConfig{}).
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.findObjectsForSecret(r.SecretNamespacedName)),
		).
		Complete(r)
}
