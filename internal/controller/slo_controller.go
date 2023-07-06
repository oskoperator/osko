package controller

import (
	"context"
	"errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	openslov1 "github.com/SLO-Kubernetes-Operator/slo-kubernetes-operator/api/v1"
)

// SLOReconciler reconciles a SLO object
type SLOReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=openslo.openslo,resources=slos,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=openslo.openslo,resources=slos/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=openslo.openslo,resources=slos/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the SLO object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *SLOReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var slo openslov1.SLO

	err := r.Get(ctx, req.NamespacedName, &slo)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// This is what happens when manifest is deleted??
			logger.Info("SLO deleted")
			return ctrl.Result{}, nil
		}

		logger.Error(err, errGetDS)
		return ctrl.Result{}, nil
	}

	var sli openslov1.SLI
	var sliSpec openslov1.SLISpec
	var ns = slo.ObjectMeta.Namespace

	if slo.Spec.IndicatorRef != "" {
		var sliRef = types.NamespacedName{Namespace: ns, Name: slo.Spec.IndicatorRef}
		err = r.Get(ctx, sliRef, &sli)
		if err != nil {
			// TODO patch status of SLO
			logger.Error(err, "Referenced SLI not found")
			return ctrl.Result{}, err
		}
		sliSpec = sli.Spec
	} else {
		sliSpec = slo.Spec.Indicator
	}

	var dsRef types.NamespacedName
	var ds openslov1.Datasource
	var ratioSecondaryDsRef types.NamespacedName
	var secondaryDs openslov1.Datasource

	if sliSpec.RatioMetric.Raw != (openslov1.MetricSpec{}) {
		dsRef = types.NamespacedName{Namespace: ns, Name: sliSpec.RatioMetric.Raw.MetricSource.MetricSourceRef}
	} else if sliSpec.RatioMetric.Total != (openslov1.MetricSpec{}) {
		dsRef = types.NamespacedName{Namespace: ns, Name: sliSpec.RatioMetric.Total.MetricSource.MetricSourceRef}
		if sliSpec.RatioMetric.Good != (openslov1.MetricSpec{}) {
			ratioSecondaryDsRef = types.NamespacedName{Namespace: ns, Name: sliSpec.RatioMetric.Good.MetricSource.MetricSourceRef}
		} else if sliSpec.RatioMetric.Bad != (openslov1.MetricSpec{}) {
			ratioSecondaryDsRef = types.NamespacedName{Namespace: ns, Name: sliSpec.RatioMetric.Bad.MetricSource.MetricSourceRef}
		} else {
			// TODO patch something
			err = errors.New("RatioMetric must have either Good or Bad present")
			logger.Error(err, "Neither Good or Bad found in RatioMetric")
			return ctrl.Result{}, err
		}
	} else if sliSpec.ThresholdMetric != (openslov1.MetricSpec{}) {
		dsRef = types.NamespacedName{Namespace: ns, Name: sliSpec.ThresholdMetric.MetricSource.MetricSourceRef}
	}

	err = r.Get(ctx, dsRef, &ds)
	if err != nil {
		// TODO patch something
		logger.Error(err, "Referenced DataSource not found")
		return ctrl.Result{}, err
	}
	if (ratioSecondaryDsRef != (types.NamespacedName{})) && (ratioSecondaryDsRef != dsRef) {
		err = r.Get(ctx, ratioSecondaryDsRef, &secondaryDs)
		if err != nil {
			// TODO patch something
			logger.Error(err, "Referenced DataSource not found")
			return ctrl.Result{}, err
		}
		// Check that we can handle Total with Good-Bad within the same rules
		if (ds.Spec.Type != secondaryDs.Spec.Type) || (ds.Spec.ConnectionDetails.Address != secondaryDs.Spec.ConnectionDetails.Address) {
			// TODO patch something
			err = errors.New("Good/Bad metrics source type and address must match that of Total")
			return ctrl.Result{}, err
		}
	}

	// TODO Alerts

	/*
		Everything should be resolved here, we should have most of the information
		fetched from the cluster, so we can start doing actual things based on the datasource type
	*/
	if ds.Spec.Type == "mimir" {
		// TODO create CRD MimirRule or something with the finished thing

	}

	logger.Info("SLO reconciled")

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SLOReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&openslov1.SLO{}).
		Complete(r)
}
