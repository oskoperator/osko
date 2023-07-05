package controller

import (
	"context"
	"fmt"
	"net/http"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	openslov1 "github.com/SLO-Kubernetes-Operator/slo-kubernetes-operator/api/v1"
)

const (
	errGetDS = "could not get Datasource"
)

// DatasourceReconciler reconciles a Datasource object
type DatasourceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=openslo.openslo,resources=datasources,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=openslo.openslo,resources=datasources/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=openslo.openslo,resources=datasources/finalizers,verbs=update

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

	// check if we can connect to the ruler if it's enabled
	if ds.Spec.ConnectionDetails.Ruler.Enabled {
		client := &http.Client{}
		req, err := http.NewRequest("GET", fmt.Sprintf("%v%v", ds.Spec.ConnectionDetails.Address, ds.Spec.ConnectionDetails.Ruler.Subpath), nil)
		if ds.Spec.ConnectionDetails.Multitenancy.Enabled {
			req.Header.Add("X-Org-ID", ds.Spec.ConnectionDetails.Multitenancy.SourceTenants[0])
		}

		resp, err := client.Do(req)
		if err != nil {
			log.Error(err, "Couldn't connect to the ruler")
			return ctrl.Result{}, nil
		}
		if resp.StatusCode > 299 {
			log.Info("There was an error connecting to the Ruler", "Status code", resp.StatusCode)
			return ctrl.Result{}, nil
		}
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
