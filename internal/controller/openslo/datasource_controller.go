package controller

import (
	"context"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"time"

	openslov1 "github.com/oskoperator/osko/apis/openslo/v1"
)

const (
	errGetDS     = "could not get Datasource"
	errConnectDS = "could not connect to Datasource"
	errQueryAPI  = "could not query API"
)

// DatasourceReconciler reconciles a Datasource object
type DatasourceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=openslo.com,resources=datasources,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=openslo.com,resources=datasources/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=openslo.com,resources=datasources/finalizers,verbs=update

func (r *DatasourceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var ds openslov1.Datasource

	err := r.Get(ctx, req.NamespacedName, &ds)
	if err != nil {
		// ignore Datasource deletion
		if apierrors.IsNotFound(err) {
			log.Info("Datasource deleted")
			return ctrl.Result{}, nil
		}

		log.Error(err, errGetDS)
		return ctrl.Result{}, nil
	}
	if ds.Spec.Type == "mimir" {
		log.Info("Datasource Type is Mimir", "address", ds.Spec.ConnectionDetails.Address)
		client, err := api.NewClient(api.Config{
			Address: ds.Spec.ConnectionDetails.Address + "/prometheus",
		})
		if err != nil {
			log.Error(err, errConnectDS)
			return ctrl.Result{}, nil
		}
		api := v1.NewAPI(client)
		result, _, err := api.Query(ctx, "up", time.Now())
		if err != nil {
			log.Error(err, errQueryAPI)
			return ctrl.Result{}, nil
		}
		log.Info("Datasource successfully connected", "result", result)
	}

	if ds.Spec.Type == "cortex" {
		log.Info("Datasource Type is Cortex", "address", ds.Spec.ConnectionDetails.Address)
	}

	log.Info("Datasource reconciled")

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DatasourceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&openslov1.Datasource{}).
		Complete(r)
}
