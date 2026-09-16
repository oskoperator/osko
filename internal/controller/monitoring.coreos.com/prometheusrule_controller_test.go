package monitoringcoreoscom

import (
	"reflect"
	"testing"

	openslov1 "github.com/oskoperator/osko/api/openslo/v1"
	"github.com/oskoperator/osko/internal/config"
	"github.com/oskoperator/osko/internal/helpers"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func init() {
	config.NewConfig()
}

func testSLO() *openslov1.SLO {
	return &openslov1.SLO{
		ObjectMeta: metav1.ObjectMeta{Name: "test-slo", Namespace: "default"},
		Spec: openslov1.SLOSpec{
			Service:         "test-service",
			BudgetingMethod: "Occurrences",
			Objectives:      []openslov1.ObjectivesSpec{{Target: "0.999"}},
			TimeWindow:      []openslov1.TimeWindowSpec{{Duration: "28d", IsRolling: true}},
		},
	}
}

func testSLI() *openslov1.SLI {
	return &openslov1.SLI{
		ObjectMeta: metav1.ObjectMeta{Name: "test-sli", Namespace: "default"},
		Spec: openslov1.SLISpec{
			RatioMetric: openslov1.RatioMetricSpec{
				Counter: true,
				Good: openslov1.MetricSpec{MetricSource: openslov1.MetricSource{
					Type: "Mimir",
					Spec: openslov1.MetricSourceSpec{Query: "sum(rate(good_total[5m]))"},
				}},
				Total: openslov1.MetricSpec{MetricSource: openslov1.MetricSource{
					Type: "Mimir",
					Spec: openslov1.MetricSourceSpec{Query: "sum(rate(requests_total[5m]))"},
				}},
			},
		},
	}
}

// asStoredByAPIServer returns the object the API server would hand back: the
// same desired state, plus the server-assigned metadata a freshly generated
// object cannot have.
func asStoredByAPIServer(rule *monitoringv1.PrometheusRule) *monitoringv1.PrometheusRule {
	live := rule.DeepCopy()
	live.ResourceVersion = "874512"
	live.UID = types.UID("6f1d0b2a-0f3e-4a1c-9a6f-2b7c8d9e0f11")
	live.CreationTimestamp = metav1.Now()
	live.Generation = 3
	return live
}

// TestPrometheusRuleNeedsUpdate_NoDriftDoesNotUpdate is the regression guard for
// the hot reconcile loop. Comparing whole objects always reports a difference,
// because the stored object carries resourceVersion, uid and creationTimestamp
// that the generated one lacks. Every reconcile then issues an Update, the
// Update bumps resourceVersion, the watch fires, and the loop never settles.
func TestPrometheusRuleNeedsUpdate_NoDriftDoesNotUpdate(t *testing.T) {
	desired, err := helpers.CreatePrometheusRule(testSLO(), testSLI())
	if err != nil {
		t.Fatalf("CreatePrometheusRule() error = %v", err)
	}
	live := asStoredByAPIServer(desired)

	if reflect.DeepEqual(live, desired) {
		t.Fatal("precondition failed: stored and generated objects should differ by server metadata, " +
			"otherwise this test cannot detect the bug it guards")
	}

	if prometheusRuleNeedsUpdate(live, desired) {
		t.Error("an unchanged PrometheusRule was reported as needing an update; " +
			"this is the hot reconcile loop - every pass writes, every write wakes the watch")
	}
}

func TestPrometheusRuleNeedsUpdate_DetectsRealDrift(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*monitoringv1.PrometheusRule)
	}{
		{
			name: "rule expression changed outside OSKO",
			mutate: func(r *monitoringv1.PrometheusRule) {
				r.Spec.Groups[0].Rules[0].Expr.StrVal = "vector(0)"
			},
		},
		{
			name: "a rule group was deleted",
			mutate: func(r *monitoringv1.PrometheusRule) {
				r.Spec.Groups = r.Spec.Groups[:len(r.Spec.Groups)-1]
			},
		},
		{
			name: "the managed-by marker was stripped, so ThanosRuler would stop selecting it",
			mutate: func(r *monitoringv1.PrometheusRule) {
				delete(r.Labels, helpers.LabelManagedBy)
			},
		},
		{
			name: "an annotation was changed",
			mutate: func(r *monitoringv1.PrometheusRule) {
				if r.Annotations == nil {
					r.Annotations = map[string]string{}
				}
				r.Annotations["osko.dev/baseWindow"] = "17m"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			desired, err := helpers.CreatePrometheusRule(testSLO(), testSLI())
			if err != nil {
				t.Fatalf("CreatePrometheusRule() error = %v", err)
			}
			live := asStoredByAPIServer(desired)
			tt.mutate(live)

			if !prometheusRuleNeedsUpdate(live, desired) {
				t.Error("real drift was not detected, so OSKO would leave the cluster wrong")
			}
		})
	}
}
