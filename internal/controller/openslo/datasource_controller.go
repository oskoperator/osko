package controller

import (
	"context"
	openslov1 "github.com/oskoperator/osko/api/openslo/v1"
	"github.com/prometheus/client_golang/api"
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
	switch ds.Spec.Type {
	case "mimir":
		log.Info("Datasource Type is Mimir", "address", ds.Spec.ConnectionDetails.Address)
		err = r.connectDatasource(ctx, ds)
		if err != nil {
			log.Error(err, errConnectDS)
			return ctrl.Result{}, err
		}
	case "cortex":
		log.Info("Datasource Type is Cortex", "address", ds.Spec.ConnectionDetails.Address)
		r.Recorder.Event(ds, "Warning", "NotImplemented", "Cortex support is not implemented yet")
	}

	log.V(1).Info("Datasource reconciled")
	r.Recorder.Event(ds, "Normal", "DatasourceReconciled", "Datasource reconciled")

	return ctrl.Result{}, nil
}

func (r *DatasourceReconciler) connectDatasource(ctx context.Context, ds *openslov1.Datasource) error {
	_, err := api.NewClient(api.Config{
		Address: ds.Spec.ConnectionDetails.Address + "/prometheus",
	})
	if err != nil {
		r.Recorder.Event(ds, "Warning", "DatasourceConnectionFailed", "Datasource connection failed")
		return err
	}
	//api := v1.NewAPI(client)
	//result, _, err := api.Query(ctx, "up", time.Now())
	//if err != nil {
	//	r.Recorder.Event(ds, "Warning", "DatasourceConnectionFailed", fmt.Sprintf("API query failed %s", result))
	//	return err
	//}
	//r.Recorder.Event(ds, "Normal", "DatasourceConnected", "Datasource successfully connected")
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DatasourceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&openslov1.Datasource{}).
		Complete(r)
}
