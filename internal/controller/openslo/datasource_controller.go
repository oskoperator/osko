package controller

import (
	"context"
	"fmt"
	"net/http"
	"time"

	openslov1 "github.com/oskoperator/osko/api/openslo/v1"
	"github.com/oskoperator/osko/internal/backend"
	"github.com/oskoperator/osko/internal/errors"
	"github.com/oskoperator/osko/internal/utils"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	errGetDS     = "could not get Datasource"
	errConnectDS = "could not connect to Datasource"
	errQueryAPI  = "could not query API"
)

// DatasourceReconciler reconciles a Datasource object
type DatasourceReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

type CustomRoundTripper struct {
	Transport http.RoundTripper
	// TenantHeader is the HTTP header carrying the tenant identifier for the
	// backend, or empty when the backend has no tenancy header.
	TenantHeader string
	TenantID     string
}

//+kubebuilder:rbac:groups=openslo.com,resources=datasources,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=openslo.com,resources=datasources/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=openslo.com,resources=datasources/finalizers,verbs=update

func (r *DatasourceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	ds := &openslov1.Datasource{}

	err := r.Get(ctx, req.NamespacedName, ds)
	if err != nil {
		// ignore Datasource deletion
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Datasource deleted")
			return ctrl.Result{}, nil
		}

		log.Error(err, errGetDS)
		return ctrl.Result{}, errors.Transient(err, 5*time.Second)
	}
	backendType, err := backend.Parse(ds.Spec.Type)
	if err != nil {
		log.Error(err, "unsupported datasource type", "type", ds.Spec.Type)
		r.Recorder.Event(ds, "Warning", "UnsupportedDatasourceType", err.Error())
		if statusErr := utils.UpdateStatus(ctx, ds, r.Client, "Ready", metav1.ConditionFalse, err.Error()); statusErr != nil {
			log.Error(statusErr, "Failed to update Datasource status")
			return ctrl.Result{}, errors.Transient(statusErr, 5*time.Second)
		}
		return ctrl.Result{}, errors.Permanent(err)
	}

	switch backendType {
	case backend.Mimir, backend.Thanos, backend.Prometheus, backend.VictoriaMetrics:
		log.Info("Connecting Datasource", "type", string(backendType), "address", ds.Spec.ConnectionDetails.Address)
		if err := r.connectDatasource(ctx, ds, backendType); err != nil {
			log.Error(err, errConnectDS)
			if statusErr := utils.UpdateStatus(ctx, ds, r.Client, "Ready", metav1.ConditionFalse, errConnectDS); statusErr != nil {
				log.Error(statusErr, "Failed to update Datasource status")
			}
			return ctrl.Result{}, errors.Transient(err, 5*time.Second)
		}
	case backend.Cortex:
		log.Info("Datasource Type is Cortex", "address", ds.Spec.ConnectionDetails.Address)
		r.Recorder.Event(ds, "Warning", "NotImplemented", "Cortex support is not implemented yet")
		if err := utils.UpdateStatus(ctx, ds, r.Client, "Ready", metav1.ConditionFalse,
			"Cortex support is not implemented yet"); err != nil {
			log.Error(err, "Failed to update Datasource status")
			return ctrl.Result{}, errors.Transient(err, 5*time.Second)
		}
		return ctrl.Result{}, nil
	default:
		// Reachable only if a backend.Type is added to Parse without being
		// given an arm here. Falling through would report Ready=True on a
		// Datasource that was never contacted.
		err := fmt.Errorf("datasource type %q is not handled by the Datasource controller", string(backendType))
		log.Error(err, "Unhandled datasource type")
		r.Recorder.Event(ds, "Warning", "UnhandledDatasourceType", err.Error())
		if statusErr := utils.UpdateStatus(ctx, ds, r.Client, "Ready", metav1.ConditionFalse, err.Error()); statusErr != nil {
			log.Error(statusErr, "Failed to update Datasource status")
			return ctrl.Result{}, errors.Transient(statusErr, 5*time.Second)
		}
		return ctrl.Result{}, errors.Permanent(err)
	}

	if backendType == backend.Thanos && len(ds.Spec.ConnectionDetails.SourceTenants) > 0 {
		r.Recorder.Event(ds, "Warning", "SourceTenantsIgnored",
			"sourceTenants has no Thanos equivalent and is ignored")
	}

	if err := utils.UpdateStatus(ctx, ds, r.Client, "Ready", metav1.ConditionTrue, "Datasource reconciled"); err != nil {
		log.Error(err, "Failed to update Datasource status")
		return ctrl.Result{}, errors.Transient(err, 5*time.Second)
	}

	log.V(1).Info("Datasource reconciled")
	r.Recorder.Event(ds, "Normal", "DatasourceReconciled", "Datasource reconciled")

	return ctrl.Result{}, nil
}

func (r *DatasourceReconciler) connectDatasource(ctx context.Context, ds *openslov1.Datasource, backendType backend.Type) error {
	datasourceAddress := backendType.QueryURL(ds.Spec.ConnectionDetails.Address)

	customRoundtripper := &CustomRoundTripper{
		Transport:    api.DefaultRoundTripper,
		TenantHeader: backendType.TenantHeader(),
		TenantID:     ds.Spec.ConnectionDetails.TargetTenant,
	}

	newDsClient, err := api.NewClient(api.Config{
		Address:      datasourceAddress,
		RoundTripper: customRoundtripper,
	})
	if err != nil {
		r.Recorder.Event(ds, "Warning", "DatasourceConnectionFailed", "Datasource connection failed")
		return err
	}

	newAPI := v1.NewAPI(newDsClient)
	result, _, err := newAPI.Query(ctx, "up", time.Now())
	if err != nil {
		r.Recorder.Event(ds, "Warning", "DatasourceConnectionFailed", fmt.Sprintf("API query failed to address: %s with error: %s", datasourceAddress, err.Error()))
		return err
	}
	r.Recorder.Event(ds, "Normal", "DatasourceConnected", fmt.Sprintf("Datasource successfully connected - %s", result.String()))
	return nil
}

func (c *CustomRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if c.TenantHeader != "" && c.TenantID != "" {
		req.Header.Set(c.TenantHeader, c.TenantID)
	}
	return c.Transport.RoundTrip(req)
}

// SetupWithManager sets up the controller with the Manager.
func (r *DatasourceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&openslov1.Datasource{}).
		Complete(r)
}
