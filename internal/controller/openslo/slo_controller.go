package controller

import (
	"context"
	"fmt"
	openslov1 "github.com/oskoperator/osko/apis/openslo/v1"
	"github.com/oskoperator/osko/internal/utils"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// SLOReconciler reconciles a SLO object
type SLOReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=openslo.com,resources=slos,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=openslo.com,resources=slos/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=openslo.com,resources=slos/finalizers,verbs=update

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
	_ = log.FromContext(ctx)
	log := log.FromContext(ctx)

	var slo openslov1.SLO
	var sli openslov1.SLI

	err := r.Get(ctx, req.NamespacedName, &slo)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("SLO deleted")
			return ctrl.Result{}, nil
		}

		log.Error(err, errGetDS)
		return ctrl.Result{}, nil
	}

	// Get SLI from SLO's ref
	if slo.Spec.IndicatorRef != nil {
		err = r.Get(ctx, client.ObjectKey{Name: *slo.Spec.IndicatorRef, Namespace: slo.Namespace}, &sli)
	}
	if err != nil && apierrors.IsNotFound(err) {
		log.Error(err, errGetSLI)
		err = utils.UpdateStatus(
			ctx,
			&slo,
			r.Client,
			"Ready",
			metav1.ConditionFalse,
			"SLIObjectNotFound",
			"SLI Object not found",
		)
		if err != nil {
			log.Error(err, "Failed to update SLO status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, err
	} else {
		// Set SLI instance as the owner and controller.
		err = ctrl.SetControllerReference(&slo, &sli, r.Scheme)
		if err != nil {
			err = utils.UpdateStatus(
				ctx,
				&slo,
				r.Client,
				"Ready",
				metav1.ConditionFalse,
				"FailedToSetSLIOwner",
				"Failed to set SLI owner reference",
			)
			if err != nil {
				log.Error(err, "Failed to update SLO status")
				return ctrl.Result{}, err
			}
			log.Error(err, "Failed to set owner reference for SLI")
			return ctrl.Result{}, err
		}
	}

	// Check if this PrometheusRule already exists
	promRule := &monitoringv1.PrometheusRule{}
	err = r.Get(ctx, types.NamespacedName{
		Name:      slo.Name,
		Namespace: slo.Namespace,
	}, promRule)

	if err != nil && apierrors.IsNotFound(err) {
		log.Info("PrometheusRule not found. Let's make some.")
		promRule, err = createPrometheusRule(slo, sli)
		if err != nil {
			err = utils.UpdateStatus(
				ctx,
				&slo,
				r.Client,
				"Ready",
				metav1.ConditionFalse,
				"FailedToCreatePrometheusRule",
				"Failed to create Prometheus Rule",
			)
			if err != nil {
				log.Error(err, "Failed to update SLO status")
				return ctrl.Result{}, err
			}
			log.Error(err, "Failed to create new PrometheusRule", "PrometheusRule.Namespace", promRule.Namespace, "PrometheusRule.Name", promRule.Name)
			return ctrl.Result{}, err
		}
	}

	for _, rule := range promRule.Spec.Groups[0].Rules {
		if rule.Expr != intstr.Parse(fmt.Sprintf("sum(rate(%s[%s])) / sum(rate(%s[%s]))",
			sli.Spec.RatioMetric.Good.MetricSource.Spec,
			slo.Spec.TimeWindow[0].Duration,
			sli.Spec.RatioMetric.Total.MetricSource.Spec,
			slo.Spec.TimeWindow[0].Duration,
		)) {
			promRule.Spec.Groups[0].Rules[0].Expr = intstr.Parse(fmt.Sprintf("sum(rate(%s[%s])) / sum(rate(%s[%s]))",
				sli.Spec.RatioMetric.Good.MetricSource.Spec,
				slo.Spec.TimeWindow[0].Duration,
				sli.Spec.RatioMetric.Total.MetricSource.Spec,
				slo.Spec.TimeWindow[0].Duration,
			))
		}
	}

	// Set SLO instance as the owner and controller.
	if err := ctrl.SetControllerReference(&slo, promRule, r.Scheme); err != nil {
		log.Error(err, "Failed to set owner reference for PrometheusRule")
		return ctrl.Result{}, err
	}

	if err := r.Update(ctx, promRule); err != nil {
		if apierrors.IsNotFound(err) {
			if err := r.Create(ctx, promRule); err != nil {
				if err := r.Status().Update(ctx, &slo); err != nil {
					log.Error(err, "Failed to update SLO status")
					slo.Status.Ready = "Failed"
					if err := r.Status().Update(ctx, &slo); err != nil {
						log.Error(err, "Failed to update SLO ready status")
						return ctrl.Result{}, err
					}
					return ctrl.Result{}, err
				}
			}
		} else {
			if err := r.Status().Update(ctx, &slo); err != nil {
				log.Error(err, "Failed to update SLO status")
				slo.Status.Ready = "Failed"
				if err := r.Status().Update(ctx, &slo); err != nil {
					log.Error(err, "Failed to update SLO ready status")
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, err
			}
		}
	}

	err = utils.UpdateStatus(
		ctx,
		&slo,
		r.Client,
		"Ready",
		metav1.ConditionTrue,
		"PrometheusRuleCreated",
		"PrometheusRule created",
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func createPrometheusRule(slo openslov1.SLO, sli openslov1.SLI) (*monitoringv1.PrometheusRule, error) {
	rule := &monitoringv1.PrometheusRule{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "monitoring.coreos.com/v1",
			Kind:       "PrometheusRule",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            slo.Name,
			Namespace:       slo.Namespace,
			Labels:          slo.Labels,
			OwnerReferences: slo.OwnerReferences,
		},
		Spec: monitoringv1.PrometheusRuleSpec{
			Groups: []monitoringv1.RuleGroup{{
				Name: slo.Name,
				Rules: []monitoringv1.Rule{{
					Expr: intstr.Parse(fmt.Sprintf("sum(rate(%s[%s])) / sum(rate(%s[%s]))",
						sli.Spec.RatioMetric.Good.MetricSource.Spec,
						slo.Spec.TimeWindow[0].Duration,
						sli.Spec.RatioMetric.Total.MetricSource.Spec,
						slo.Spec.TimeWindow[0].Duration,
					)),
				}}}},
		},
	}
	return rule, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SLOReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&openslov1.SLO{}).
		Owns(&monitoringv1.PrometheusRule{}).
		Watches(
			&openslov1.SLI{},
			&handler.EnqueueRequestForObject{},
		).
		Complete(r)
}
