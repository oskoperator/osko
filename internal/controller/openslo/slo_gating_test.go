package controller

import (
	"context"
	"strings"
	"testing"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	openslov1 "github.com/oskoperator/osko/api/openslo/v1"
	oskov1alpha1 "github.com/oskoperator/osko/api/osko/v1alpha1"
	"github.com/oskoperator/osko/internal/config"
)

func gatingTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	require.NoError(t, openslov1.AddToScheme(s))
	require.NoError(t, oskov1alpha1.AddToScheme(s))
	require.NoError(t, monitoringv1.AddToScheme(s))
	return s
}

// TestSLOGatesOwnedResourcesByBackend asserts which owned resources the SLO
// reconciler creates for each backend. Mimir and Cortex expose a ruler
// configuration API, so they need a MimirRule; Thanos, Prometheus and
// VictoriaMetrics consume the PrometheusRule instead and must not get one.
func TestSLOGatesOwnedResourcesByBackend(t *testing.T) {
	config.NewConfig()

	tests := []struct {
		name           string
		datasourceType string
		magicAlerting  bool
		wantMimirRule  bool
		wantAMC        bool
	}{
		{name: "mimir pushes rules to a remote ruler", datasourceType: "mimir", wantMimirRule: true},
		{name: "cortex pushes rules to a remote ruler", datasourceType: "cortex", wantMimirRule: true},
		{name: "thanos consumes the PrometheusRule instead", datasourceType: "thanos", wantMimirRule: false},
		{name: "prometheus consumes the PrometheusRule instead", datasourceType: "prometheus", wantMimirRule: false},
		{name: "victoriametrics consumes the PrometheusRule instead", datasourceType: "victoriametrics", wantMimirRule: false},
		{name: "magic alerting on mimir creates an AlertManagerConfig", datasourceType: "mimir", magicAlerting: true, wantMimirRule: true, wantAMC: true},
		{name: "magic alerting on thanos creates nothing", datasourceType: "thanos", magicAlerting: true, wantMimirRule: false, wantAMC: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			annotations := map[string]string{"osko.dev/datasourceRef": "test-ds"}
			if tt.magicAlerting {
				annotations["osko.dev/magicAlerting"] = "true"
			}

			ds := &openslov1.Datasource{
				ObjectMeta: metav1.ObjectMeta{Name: "test-ds", Namespace: "default"},
				Spec: openslov1.DatasourceSpec{
					Type: tt.datasourceType,
					ConnectionDetails: oskov1alpha1.ConnectionDetails{
						Address:      "http://example:9090",
						TargetTenant: "test-tenant",
					},
				},
			}

			metricSource := func() openslov1.MetricSource {
				return openslov1.MetricSource{
					MetricSourceRef: "test-ds",
					Type:            tt.datasourceType,
					Spec:            openslov1.MetricSourceSpec{Query: "sum(rate(requests_total[5m]))"},
				}
			}

			slo := &openslov1.SLO{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-slo",
					Namespace:   "default",
					Annotations: annotations,
				},
				Spec: openslov1.SLOSpec{
					Service:         "test-service",
					BudgetingMethod: "Occurrences",
					Objectives:      []openslov1.ObjectivesSpec{{Target: "0.99"}},
					TimeWindow:      []openslov1.TimeWindowSpec{{Duration: "28d", IsRolling: true}},
					Indicator: &openslov1.Indicator{
						Metadata: metav1.ObjectMeta{Name: "test-sli"},
						Spec: openslov1.SLISpec{
							RatioMetric: openslov1.RatioMetricSpec{
								Counter: true,
								Good:    openslov1.MetricSpec{MetricSource: metricSource()},
								Total:   openslov1.MetricSpec{MetricSource: metricSource()},
							},
						},
					},
				},
			}

			s := gatingTestScheme(t)
			c := fake.NewClientBuilder().
				WithScheme(s).
				WithObjects(ds, slo).
				WithStatusSubresource(
					&openslov1.SLO{},
					&openslov1.SLI{},
					&openslov1.Datasource{},
					&oskov1alpha1.MimirRule{},
					&oskov1alpha1.AlertManagerConfig{},
				).
				Build()

			r := &SLOReconciler{Client: c, Scheme: s, Recorder: record.NewFakeRecorder(50)}
			ctx := context.Background()
			key := types.NamespacedName{Name: "test-slo", Namespace: "default"}

			// Reconcile is not single-pass: pass 1 adds the finalizer and
			// requeues, pass 2 creates the PrometheusRule and returns, and only
			// pass 3 reaches the MimirRule and magic-alerting gates. A fourth
			// pass is harmless and guards against an extra early return.
			for i := 1; i <= 4; i++ {
				_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: key})
				require.NoError(t, err, "reconcile pass %d", i)
			}

			pr := &monitoringv1.PrometheusRule{}
			require.NoError(t, c.Get(ctx, key, pr),
				"every backend gets a PrometheusRule")

			mr := &oskov1alpha1.MimirRule{}
			mrErr := c.Get(ctx, key, mr)
			if tt.wantMimirRule {
				assert.NoError(t, mrErr,
					"%s exposes a ruler API, so a MimirRule is required", tt.datasourceType)
			} else {
				assert.True(t, apierrors.IsNotFound(mrErr),
					"%s has no rule-write API, so no MimirRule may be created; got err=%v",
					tt.datasourceType, mrErr)
			}

			amc := &oskov1alpha1.AlertManagerConfig{}
			amcErr := c.Get(ctx, types.NamespacedName{Name: "test-slo-alerting", Namespace: "default"}, amc)
			if tt.wantAMC {
				assert.NoError(t, amcErr,
					"magic alerting on %s must create an AlertManagerConfig", tt.datasourceType)
			} else {
				assert.True(t, apierrors.IsNotFound(amcErr),
					"no AlertManagerConfig expected for %s (magicAlerting=%v); got err=%v",
					tt.datasourceType, tt.magicAlerting, amcErr)
			}

			got := &openslov1.SLO{}
			require.NoError(t, c.Get(ctx, key, got))
			assert.Equal(t, "True", got.Status.Ready,
				"burn-rate rules live in the PrometheusRule and still fire, so the SLO stays Ready")

			if tt.magicAlerting && !tt.wantAMC {
				rec := r.Recorder.(*record.FakeRecorder)
				var warned bool
				for len(rec.Events) > 0 {
					if strings.Contains(<-rec.Events, "MagicAlertingUnsupported") {
						warned = true
					}
				}
				assert.True(t, warned, "the operator must be told why no routing config was written")
			}
		})
	}
}
