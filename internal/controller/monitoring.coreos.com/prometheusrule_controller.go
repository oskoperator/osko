package monitoringcoreoscom

import (
	"context"
	openslov1 "github.com/oskoperator/osko/api/openslo/v1"
	"github.com/oskoperator/osko/internal/helpers"
	"github.com/oskoperator/osko/internal/utils"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	objectiveRef = ".metaData.ownerReferences.name"
)

// PrometheusRuleReconciler reconciles a PrometheusRule object
type PrometheusRuleReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=prometheusrules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=prometheusrules/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=prometheusrules/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

func (r *PrometheusRuleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	slo := &openslov1.SLO{}
	sli := &openslov1.SLI{}
	prometheusRule := &monitoringv1.PrometheusRule{}
	newPrometheusRule := &monitoringv1.PrometheusRule{}

	err := r.Get(ctx, req.NamespacedName, prometheusRule)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("PrometheusRule resource not found. Ignoring since object mus be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get PrometheusRule")
		return ctrl.Result{}, err
	}

	for _, ref := range prometheusRule.ObjectMeta.OwnerReferences {
		if ref.Kind == "SLO" {
			sloNamespacedName := types.NamespacedName{
				Name:      ref.Name,
				Namespace: req.Namespace,
			}

			if err := r.Get(ctx, sloNamespacedName, slo); err != nil {
				log.Error(err, "Failed to get SLO")
				return ctrl.Result{}, err
			}
		}
	}

	if apierrors.IsNotFound(err) {
		log.Info("PrometheusRule not found. Let's make one.")
		prometheusRule, err = helpers.CreatePrometheusRule(slo, sli)
		if err != nil {
			err = utils.UpdateStatus(
				ctx,
				slo,
				r.Client,
				"Ready",
				metav1.ConditionFalse,
				"Failed to create Prometheus Rule",
			)
			if err != nil {
				log.Error(err, "Failed to update SLO status")
				return ctrl.Result{}, err
			}
			log.Error(err, "Failed to create new PrometheusRule")
			return ctrl.Result{}, err
		}
		if err := r.Create(ctx, prometheusRule); err != nil {
			r.Recorder.Event(slo, "Error", "FailedToCreatePrometheusRule", "Failed to create Prometheus Rule")
			if err := r.Status().Update(ctx, prometheusRule); err != nil {
				log.Error(err, "Failed to update SLO status")
				slo.Status.Ready = "Failed"
				if err := r.Status().Update(ctx, slo); err != nil {
					log.Error(err, "Failed to update SLO ready status")
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, err
			}
		} else {
			log.Info("PrometheusRule created successfully")
			r.Recorder.Event(slo, "Normal", "PrometheusRuleCreated", "PrometheusRule created successfully")
			slo.Status.Ready = "True"
			if err := r.Status().Update(ctx, slo); err != nil {
				log.Error(err, "Failed to update SLO ready status")
				return ctrl.Result{}, nil
			}
		}
	}

	// Update PrometheusRule
	// This is the main logic for the PrometheusRule update
	// Here we should take the existing PrometheusRule and update it with the new one
	log.Info("PrometheusRule already exists, we should update it")
	newPrometheusRule, err = helpers.CreatePrometheusRule(slo, sli)
	if err != nil {
		log.Error(err, "Failed to create new PrometheusRule")
		return ctrl.Result{}, err
	}

	compareResult := reflect.DeepEqual(prometheusRule, newPrometheusRule)
	if compareResult {
		log.Info("PrometheusRule is already up to date")
		return ctrl.Result{}, nil
	}

	// has to be the same as for previous object, otherwise it will not be updated and throw an error
	newPrometheusRule.ResourceVersion = prometheusRule.ResourceVersion

	log.Info("Updating PrometheusRule", "PrometheusRule Name", newPrometheusRule.Name, "PrometheusRule Namespace", newPrometheusRule.Namespace)
	if err := r.Update(ctx, newPrometheusRule); err != nil {
		log.Error(err, "Failed to update PrometheusRule")
		return ctrl.Result{}, err
	}
	if err := r.Status().Update(ctx, slo); err != nil {
		log.Error(err, "Failed to update SLO status")
		slo.Status.Ready = "Failed"
		if err := r.Status().Update(ctx, slo); err != nil {
			log.Error(err, "Failed to update SLO ready status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, err
	}

	log.Info("PrometheusRule reconciled")
	return ctrl.Result{}, nil
}

func (r *PrometheusRuleReconciler) createIndices(mgr ctrl.Manager) error {
	return mgr.GetFieldIndexer().IndexField(
		context.TODO(),
		&monitoringv1.PrometheusRule{},
		objectiveRef,
		func(object client.Object) []string {
			pr := object.(*monitoringv1.PrometheusRule)
			if pr.ObjectMeta.OwnerReferences == nil {
				return nil
			}
			return []string{pr.ObjectMeta.OwnerReferences[0].Name}
		})
}

func (r *PrometheusRuleReconciler) findObjectsForSlo() func(ctx context.Context, a client.Object) []reconcile.Request {
	return func(ctx context.Context, a client.Object) []reconcile.Request {
		attachedSLOs := &openslov1.SLOList{}
		listOpts := &client.ListOptions{
			FieldSelector: fields.OneTermEqualSelector(objectiveRef, a.GetName()),
			Namespace:     a.GetNamespace(),
		}
		err := r.List(ctx, attachedSLOs, listOpts)
		if err != nil {
			return []reconcile.Request{}
		}

		requests := make([]reconcile.Request, len(attachedSLOs.Items))
		for i, item := range attachedSLOs.Items {
			requests[i] = reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      item.Name,
					Namespace: item.Namespace,
				},
			}
		}
		return requests
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *PrometheusRuleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := r.createIndices(mgr); err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&monitoringv1.PrometheusRule{}).
		Watches(
			&openslov1.SLO{},
			handler.EnqueueRequestsFromMapFunc(r.findObjectsForSlo()),
		).
		Watches(
			&openslov1.Datasource{},
			handler.EnqueueRequestsFromMapFunc(r.findObjectsForSlo())).
		Complete(r)
}
