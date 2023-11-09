package controller

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/grafana/mimir/pkg/mimirtool/rules/rwrulefmt"
	openslov1 "github.com/oskoperator/osko/apis/openslo/v1"
	"github.com/oskoperator/osko/internal/mimirtool"
	"github.com/oskoperator/osko/internal/utils"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/prometheus/prometheus/model/rulefmt"
	"gopkg.in/yaml.v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	indicatorRef = ".spec.indicatorRef"
	errGetSLO    = "could not get SLO Object"
)

// SLOReconciler reconciles a SLO object
type SLOReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=openslo.com,resources=slos,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=openslo.com,resources=slos/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=openslo.com,resources=slos/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=monitoring.coreos.com,resources=prometheusrules,verbs=get;list;watch;create;update;patch;delete

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
	log := log.FromContext(ctx)

	sli := &openslov1.SLI{}
	slo := &openslov1.SLO{}

	err := r.Get(ctx, req.NamespacedName, slo)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("SLO resource not found. Object must have been deleted.")
			return ctrl.Result{}, nil
		}
		log.Error(err, errGetSLO)
		return ctrl.Result{}, nil
	}

	// Get SLI from SLO's ref
	if slo.Spec.IndicatorRef != nil {
		err = r.Get(ctx, client.ObjectKey{Name: *slo.Spec.IndicatorRef, Namespace: slo.Namespace}, sli)
		if err != nil {
			apierrors.IsNotFound(err)
			{
				log.Error(err, errGetSLI)
				err = utils.UpdateStatus(
					ctx,
					slo,
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
			}
		}
	} else if slo.Spec.Indicator != nil {
		log.Info("SLO has an inline SLI")
		sli.Name = slo.Spec.Indicator.Metadata.Name
		sli.Spec.Description = slo.Spec.Indicator.Spec.Description
		if slo.Spec.Indicator.Spec.RatioMetric != (openslov1.RatioMetricSpec{}) {
			sli.Spec.RatioMetric = slo.Spec.Indicator.Spec.RatioMetric
		}
		log.Info("SLI created", "SLI Name", sli.Name, "SLI Namespace", sli.Namespace)
		r.Recorder.Event(slo, "Normal", "SLICreated", fmt.Sprintf("SLI %s created", sli.Name))
	} else {
		err = utils.UpdateStatus(
			ctx,
			slo,
			r.Client,
			"Ready",
			metav1.ConditionFalse,
			"SLIObjectNotFound",
			"SLI Object not found",
		)
		if err != nil {
			log.Error(err, "Failed to update SLO status")
			r.Recorder.Event(slo, "Error", "SLIObjectNotFound", "SLI Object not found")
			return ctrl.Result{}, err
		}
		log.Error(err, "SLO has no SLI reference")
		return ctrl.Result{}, err
	}

	promRule := &monitoringv1.PrometheusRule{}
	err = r.Get(ctx, types.NamespacedName{
		Name:      slo.Name,
		Namespace: slo.Namespace,
	}, promRule)

	if apierrors.IsNotFound(err) {
		log.Info("PrometheusRule not found. Let's make one.")
		promRule, err = r.createPrometheusRule(slo, sli)
		if err != nil {
			err = utils.UpdateStatus(
				ctx,
				slo,
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
			log.Error(err, "Failed to create new PrometheusRule")
			r.Recorder.Event(slo, "Error", "FailedToCreatePrometheusRule", "Failed to create Prometheus Rule")
			return ctrl.Result{}, err
		}
		if err := r.Create(ctx, promRule); err != nil {
			if err := r.Status().Update(ctx, slo); err != nil {
				log.Error(err, "Failed to update SLO status")
				slo.Status.Ready = "Failed"
				if err := r.Status().Update(ctx, slo); err != nil {
					log.Error(err, "Failed to update SLO ready status")
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, err
			}
		}
	} else {
		// This is the main logic for the PrometheusRule update
		// Here we should take the existing PrometheusRule and update it with the new one
		log.Info("PrometheusRule already exists, we should update it")
		newPromRule, err := r.createPrometheusRule(slo, sli)
		if err != nil {
			log.Error(err, "Failed to create new PrometheusRule")
			r.Recorder.Event(slo, "Error", "FailedToCreatePrometheusRule", "Failed to create Prometheus Rule")
			return ctrl.Result{}, err
		}

		compareResult := reflect.DeepEqual(promRule, newPromRule)
		if compareResult {
			log.Info("PrometheusRule is already up to date")
			return ctrl.Result{}, nil
		}

		// has to be the same as for previous object, otherwise it will not be updated and throw an error
		newPromRule.ResourceVersion = promRule.ResourceVersion

		log.Info("Updating PrometheusRule", "PrometheusRule Name", newPromRule.Name, "PrometheusRule Namespace", newPromRule.Namespace)
		if err := r.Update(ctx, newPromRule); err != nil {
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
	}

	err = utils.UpdateStatus(
		ctx,
		slo,
		r.Client,
		"Ready",
		metav1.ConditionTrue,
		"PrometheusRuleCreated",
		"PrometheusRule created",
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	r.Recorder.Event(slo, "Normal", "PrometheusRuleCreated", "PrometheusRule created successfully")
	log.Info("Reconciling SLO")

	return ctrl.Result{}, nil

}

func (r *SLOReconciler) createPrometheusRule(slo *openslov1.SLO, sli *openslov1.SLI) (*monitoringv1.PrometheusRule, error) {
	var monitoringRules []monitoringv1.Rule
	var targetVector monitoringv1.Rule
	defaultRateWindow := "1m"
	//burnRateTimeWindows := []string{"1h", "6h", "3d"}
	sloTimeWindowDuration := string(slo.Spec.TimeWindow[0].Duration)
	m := utils.MetricLabelParams{Slo: slo, Sli: sli}

	targetVector.Record = "osko_slo_target"
	targetVector.Expr = intstr.Parse(fmt.Sprintf("vector(%s)", slo.Spec.Objectives[0].Value))
	m.TimeWindow = sloTimeWindowDuration
	targetVector.Labels = m.NewMetricLabelGenerator()

	// for now, total and good are required. bad is optional and is calculated as (total - good) if not provided
	// TODO: validate that the SLO budgeting method is Occurrences and that the SLIs are all ratio metrics in other case throw an error
	targetVectorConfig := utils.RuleConfig{
		Record:              "slo_target",
		Expr:                "",
		TimeWindow:          sloTimeWindowDuration,
		Slo:                 slo,
		Sli:                 sli,
		MetricLabelCompiler: &m,
	}

	totalRule28Config := utils.RuleConfig{
		RuleType:            "total",
		Record:              "sli_ratio_total",
		Expr:                "sum(increase(%s[%s]))",
		TimeWindow:          sloTimeWindowDuration,
		Slo:                 slo,
		Sli:                 sli,
		MetricLabelCompiler: &m,
	}

	goodRule28Config := utils.RuleConfig{
		RuleType:            "good",
		Record:              "sli_ratio_total",
		Expr:                "sum(increase(%s[%s]))",
		TimeWindow:          sloTimeWindowDuration,
		Slo:                 slo,
		Sli:                 sli,
		MetricLabelCompiler: &m,
	}

	badRule28Config := utils.RuleConfig{
		RuleType:            "bad",
		Record:              "sli_ratio_total",
		Expr:                "sum(increase(%s[%s]))",
		TimeWindow:          sloTimeWindowDuration,
		Slo:                 slo,
		Sli:                 sli,
		MetricLabelCompiler: &m,
	}

	totalRuleConfig := utils.RuleConfig{
		RuleType:            "total",
		Record:              "sli_ratio_total",
		Expr:                "sum(increase(%s[%s]))",
		TimeWindow:          defaultRateWindow,
		Slo:                 slo,
		Sli:                 sli,
		SupportiveRule:      &totalRule28Config,
		MetricLabelCompiler: &m,
	}

	goodRuleConfig := utils.RuleConfig{
		RuleType:            "good",
		Record:              "sli_ratio_good",
		Expr:                "sum(increase(%s[%s]))",
		TimeWindow:          defaultRateWindow,
		Slo:                 slo,
		Sli:                 sli,
		SupportiveRule:      &goodRule28Config,
		MetricLabelCompiler: &m,
	}

	badRuleConfig := utils.RuleConfig{
		RuleType:            "bad",
		Record:              "sli_ratio_bad",
		Expr:                "sum(increase(%s[%s]))",
		TimeWindow:          defaultRateWindow,
		Slo:                 slo,
		Sli:                 sli,
		SupportiveRule:      &badRule28Config,
		MetricLabelCompiler: &m,
	}

	errorBudgetRuleConfig := utils.BudgetRuleConfig{
		Record:           "error_budget_available",
		Slo:              slo,
		Sli:              sli,
		TargetRuleConfig: &targetVectorConfig,
		TotalRuleConfig:  &totalRuleConfig,
		BadRuleConfig:    &badRuleConfig,
	}

	configs := []utils.RuleConfig{
		totalRuleConfig,
		goodRuleConfig,
		badRuleConfig,
	}

	for _, config := range configs {
		rule, supportiveRule := config.NewRatioRule(config.TimeWindow)
		monitoringRules = append(monitoringRules, rule)
		monitoringRules = append(monitoringRules, supportiveRule)
	}

	monitoringRules = append(monitoringRules, targetVectorConfig.NewTargetRule())
	monitoringRules = append(monitoringRules, errorBudgetRuleConfig.NewBudgetRule())

	objectMeta := metav1.ObjectMeta{
		Name:            slo.Name,
		Namespace:       slo.Namespace,
		Labels:          slo.Labels,
		Annotations:     slo.Annotations,
		OwnerReferences: slo.OwnerReferences,
	}

	ruleGroup := []monitoringv1.RuleGroup{
		{
			Name:  slo.Name,
			Rules: monitoringRules,
		},
	}

	rule := &monitoringv1.PrometheusRule{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "monitoring.coreos.com/v1",
			Kind:       "PrometheusRule",
		},
		ObjectMeta: objectMeta,
		Spec: monitoringv1.PrometheusRuleSpec{
			Groups: ruleGroup,
		},
	}
	// Set SLO instance as the owner and controller.
	if err := ctrl.SetControllerReference(slo, rule, r.Scheme); err != nil {
		return nil, err
	}

	return rule, nil
}

func (r *SLOReconciler) createIndices(mgr ctrl.Manager) error {
	return mgr.GetFieldIndexer().IndexField(
		context.TODO(),
		&openslov1.SLO{},
		indicatorRef,
		func(object client.Object) []string {
			slo := object.(*openslov1.SLO)
			if slo.Spec.IndicatorRef == nil {
				return nil
			}
			return []string{*slo.Spec.IndicatorRef}
		})
}

func (r *SLOReconciler) findObjectsForSli() func(ctx context.Context, a client.Object) []reconcile.Request {
	return func(ctx context.Context, a client.Object) []reconcile.Request {
		attachedSLOs := &openslov1.SLOList{}
		listOpts := &client.ListOptions{
			FieldSelector: fields.OneTermEqualSelector(indicatorRef, a.GetName()),
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

func (r *SLOReconciler) createMimirRule(slo *openslov1.SLO, sli *openslov1.SLI, log logr.Logger) error {
	mimirRuleGroup := rwrulefmt.RuleGroup{
		RuleGroup: rulefmt.RuleGroup{
			Name: slo.Name,
			Rules: []rulefmt.RuleNode{
				{
					Record: yaml.Node{
						Kind:  8,
						Value: "osko_slo_target",
					},
					Expr: yaml.Node{
						Kind:  8,
						Value: "vector(0.999)",
					},
				},
			},
		},
		RWConfigs: []rwrulefmt.RemoteWriteConfig{},
	}

	mClient := mimirtool.MimirClientConfig{
		Address:  "https://localhost:8080",
		TenantId: "infra",
	}

	mimirClient, err := mClient.NewMimirClient()
	if err != nil {
		log.Error(err, "Failed to create Mimir client")
		return err
	}

	if err := mimirClient.CreateRuleGroup(context.Background(), "scratch_9", mimirRuleGroup); err != nil {
		log.Error(err, "Failed to create rule group")
		return err
	}

	rules, err := mimirClient.ListRules(context.Background(), "scratch_9")
	if err != nil {
		log.Error(err, "Failed to get rule group")
		return err
	}

	log.Info(fmt.Sprintf("%+v", rules))

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SLOReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := r.createIndices(mgr); err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&openslov1.SLO{}).
		Owns(&monitoringv1.PrometheusRule{}).
		Watches(
			&openslov1.SLI{},
			handler.EnqueueRequestsFromMapFunc(r.findObjectsForSli()),
		).
		Complete(r)
}
