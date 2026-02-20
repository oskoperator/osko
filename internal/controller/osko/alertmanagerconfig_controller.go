package osko

import (
	"context"
	"fmt"
	"time"

	"github.com/oskoperator/osko/internal/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"

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
	Scheme      *runtime.Scheme
	Recorder    record.EventRecorder
	MimirClient *mimirclient.MimirClient
}

const (
	errGetAMC                   = "Failed to get AlertmanagerConfig"
	alertmanagerConfigFinalizer = "alertmanagerconfig.osko.dev/finalizer"
)

// +kubebuilder:rbac:groups=osko.dev,resources=alertmanagerconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osko.dev,resources=alertmanagerconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=osko.dev,resources=alertmanagerconfigs/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch

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

	if !controllerutil.ContainsFinalizer(amc, alertmanagerConfigFinalizer) {
		controllerutil.AddFinalizer(amc, alertmanagerConfigFinalizer)
		if err := r.Update(ctx, amc); err != nil {
			log.Error(err, "Failed to update AlertmanagerConfig finalizer")
			return ctrl.Result{}, err
		}
	}

	isAlertmanagerConfigMarkedForDeletion := amc.GetDeletionTimestamp() != nil
	if isAlertmanagerConfigMarkedForDeletion {
		if controllerutil.ContainsFinalizer(amc, alertmanagerConfigFinalizer) {
			if r.MimirClient == nil {
				dsRef := amc.ObjectMeta.Annotations["osko.dev/datasourceRef"]
				if dsRef != "" {
					if err := r.Get(ctx, client.ObjectKey{Name: dsRef, Namespace: amc.Namespace}, ds); err == nil {
						mClient := helpers.MimirClientConfig{
							Address:  ds.Spec.ConnectionDetails.Address,
							TenantId: ds.Spec.ConnectionDetails.TargetTenant,
						}
						if mc, err := mClient.NewMimirClient(); err == nil {
							r.MimirClient = mc
						} else {
							log.V(1).Info("Failed to initialize MimirClient for cleanup", "error", err)
						}
					} else {
						log.V(1).Info("Datasource not found for cleanup, skipping Mimir API deletion", "datasourceRef", dsRef)
					}
				}
			}

			if r.MimirClient != nil {
				if err := r.deleteAlertmanagerConfigAPI(); err != nil {
					log.Error(err, "Failed to delete AlertmanagerConfig from Mimir API, proceeding with resource cleanup")
				}
			} else {
				log.V(1).Info("MimirClient not available, skipping Mimir API cleanup")
			}

			controllerutil.RemoveFinalizer(amc, alertmanagerConfigFinalizer)
			if err := r.Update(ctx, amc); err != nil {
				log.Error(err, "Failed to update AlertmanagerConfig finalizer")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	log.V(1).Info("Getting datasourceRef", "datasourceRef", amc.ObjectMeta.Annotations["osko.dev/datasourceRef"])

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
			if err = utils.UpdateStatus(ctx, amc, r.Client, "Ready", metav1.ConditionFalse, "Secret from secretRef not found"); err != nil {
				log.Error(err, "Failed to update amc status")
				return ctrl.Result{}, err
			}
			r.Recorder.Event(amc, "Warning", "SecretNotFound", "Secret from secretRef not found")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get secret")
		return ctrl.Result{}, nil
	}

	yamlData, ok := secret.Data["alertmanager.yaml"]
	if !ok {
		if err = utils.UpdateStatus(ctx, amc, r.Client, "Ready", metav1.ConditionFalse, "alertmanager.yaml key not found in secret"); err != nil {
			log.Error(err, "Failed to update amc status")
			return ctrl.Result{}, err
		}
		r.Recorder.Event(amc, "Warning", "KeyNotFound", "alertmanager.yaml key not found in secret")
		return ctrl.Result{}, nil
	}

	mClient := helpers.MimirClientConfig{
		Address:  ds.Spec.ConnectionDetails.Address,
		TenantId: ds.Spec.ConnectionDetails.TargetTenant,
	}

	r.MimirClient, err = mClient.NewMimirClient()
	if err != nil {
		log.Error(err, "Failed to create MimirClient")
		return ctrl.Result{}, err
	}

	err = r.MimirClient.CreateAlertmanagerConfig(ctx, string(yamlData), nil)
	if err != nil {
		return ctrl.Result{}, err
	}

	r.Recorder.Event(amc, "Normal", "AlertManagerConfigCreated", "AlertManagerConfig created successfully")
	if err = utils.UpdateStatus(ctx, amc, r.Client, "Ready", metav1.ConditionTrue, "PrometheusRule created"); err != nil {
		log.V(1).Error(err, "Failed to update SLO status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *AlertManagerConfigReconciler) findObjectsForSecret() func(ctx context.Context, a client.Object) []reconcile.Request {
	return func(ctx context.Context, a client.Object) []reconcile.Request {
		log := ctrllog.FromContext(ctx)
		amc := &oskov1alpha1.AlertManagerConfig{}
		namespacedName := types.NamespacedName{
			Name:      a.GetName(),
			Namespace: a.GetNamespace(),
		}
		err := r.Get(ctx, namespacedName, amc)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return []reconcile.Request{}
			}
			log.Error(err, errGetAMC)
			return []reconcile.Request{}
		}
		if amc.Spec.SecretRef.Namespace == "" {
			amc.Spec.SecretRef.Namespace = a.GetNamespace()
		}
		secretNamespacedName := types.NamespacedName{
			Name:      amc.Spec.SecretRef.Name,
			Namespace: amc.Spec.SecretRef.Namespace,
		}

		return []reconcile.Request{{NamespacedName: secretNamespacedName}}
	}
}

func (r *AlertManagerConfigReconciler) deleteAlertmanagerConfigAPI() error {
	if err := r.MimirClient.DeleteAlermanagerConfig(context.Background()); err != nil {
		return err
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AlertManagerConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&oskov1alpha1.AlertManagerConfig{}).
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.findObjectsForSecret()),
		).
		Complete(r)
}
