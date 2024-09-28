package controller

import (
	"context"
	"fmt"
	"net/http"
	"time"

	openslov1 "github.com/oskoperator/osko/api/openslo/v1"
	"github.com/oskoperator/osko/internal/helpers"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	TenantID  string
}

type ConnectionDetails struct {
	Address             string   `json:"address,omitempty"`
	TargetTenant        string   `json:"targetTenant,omitempty"`
	SourceTenants       []string `json:"sourceTenants,omitempty"`
	SyncPrometheusRules bool     `json:"syncPrometheusRules,omitempty"`
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
		return ctrl.Result{}, nil
	}

	connectionDetails := helpers.ConstructConnectionDetails(ds)

	switch ds.Spec.Type {
	case "mimir":
		log.Info("Datasource Type is Mimir", "address", connectionDetails.Address)
		err = r.connectDatasource(ctx, ds)
		if err != nil {
			log.Error(err, errConnectDS)
			return ctrl.Result{}, err
		}
	case "cortex":
		log.Info("Datasource Type is Cortex", "address", connectionDetails.Address)
		r.Recorder.Event(ds, "Warning", "NotImplemented", "Cortex support is not implemented yet")
	}

	log.V(1).Info("Datasource reconciled")
	r.Recorder.Event(ds, "Normal", "DatasourceReconciled", "Datasource reconciled")

	return ctrl.Result{}, nil
}

func (r *DatasourceReconciler) connectDatasource(ctx context.Context, ds *openslov1.Datasource) error {
	datasourceAddress := ""

	connectionDetails := helpers.ConstructConnectionDetails(ds)

	if ds.Spec.Type != "mimir" {
		return fmt.Errorf("unsupported datasource type: %s", ds.Spec.Type)
	} else {
		datasourceAddress = connectionDetails.Address + "/prometheus"
	}

	customRoundtripper := &CustomRoundTripper{
		Transport: api.DefaultRoundTripper,
		TenantID:  connectionDetails.TargetTenant,
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
	req.Header.Add("X-Scope-OrgId", c.TenantID)
	return c.Transport.RoundTrip(req)
}

// SetupWithManager sets up the controller with the Manager.
func (r *DatasourceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&openslov1.Datasource{}).
		Complete(r)
}
