package controller

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	openslov1 "github.com/oskoperator/osko/api/openslo/v1"
	oskov1alpha1 "github.com/oskoperator/osko/api/osko/v1alpha1"
)

func TestCreateOrUpdateInlineSLI(t *testing.T) {
	// Setup scheme
	s := runtime.NewScheme()
	_ = openslov1.AddToScheme(s)
	_ = oskov1alpha1.AddToScheme(s)

	tests := []struct {
		name        string
		slo         *openslov1.SLO
		expectedSLI *openslov1.SLI
		wantError   bool
	}{
		{
			name: "create new inline SLI with default name",
			slo: &openslov1.SLO{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slo",
					Namespace: "default",
					UID:       "test-uid",
				},
				Spec: openslov1.SLOSpec{
					Indicator: &openslov1.Indicator{
						Metadata: metav1.ObjectMeta{
							Name: "", // Empty, should generate default name
						},
						Spec: openslov1.SLISpec{
							Description: "Test SLI",
							RatioMetric: openslov1.RatioMetricSpec{
								Good: openslov1.MetricSpec{
									MetricSource: openslov1.MetricSource{
										MetricSourceRef: "test-ds",
										Type:            "Mimir",
										Spec: openslov1.MetricSourceSpec{
											Query: "sum(good)",
										},
									},
								},
								Total: openslov1.MetricSpec{
									MetricSource: openslov1.MetricSource{
										MetricSourceRef: "test-ds",
										Type:            "Mimir",
										Spec: openslov1.MetricSourceSpec{
											Query: "sum(total)",
										},
									},
								},
							},
						},
					},
				},
			},
			expectedSLI: &openslov1.SLI{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slo-sli",
					Namespace: "default",
				},
				Spec: openslov1.SLISpec{
					Description: "Test SLI",
					RatioMetric: openslov1.RatioMetricSpec{
						Good: openslov1.MetricSpec{
							MetricSource: openslov1.MetricSource{
								MetricSourceRef: "test-ds",
								Type:            "Mimir",
								Spec: openslov1.MetricSourceSpec{
									Query: "sum(good)",
								},
							},
						},
						Total: openslov1.MetricSpec{
							MetricSource: openslov1.MetricSource{
								MetricSourceRef: "test-ds",
								Type:            "Mimir",
								Spec: openslov1.MetricSourceSpec{
									Query: "sum(total)",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "create new inline SLI with custom name",
			slo: &openslov1.SLO{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slo",
					Namespace: "default",
					UID:       "test-uid",
				},
				Spec: openslov1.SLOSpec{
					Indicator: &openslov1.Indicator{
						Metadata: metav1.ObjectMeta{
							Name: "custom-sli-name",
						},
						Spec: openslov1.SLISpec{
							Description: "Custom SLI",
						},
					},
				},
			},
			expectedSLI: &openslov1.SLI{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "custom-sli-name",
					Namespace: "default",
				},
				Spec: openslov1.SLISpec{
					Description: "Custom SLI",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake client
			client := fake.NewClientBuilder().WithScheme(s).Build()

			// Create reconciler
			reconciler := &SLOReconciler{
				Client: client,
				Scheme: s,
			}

			// Test the function
			ctx := context.Background()
			sli, err := reconciler.createOrUpdateInlineSLI(ctx, tt.slo)

			if tt.wantError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, sli)

			// Verify SLI properties
			assert.Equal(t, tt.expectedSLI.Name, sli.Name)
			assert.Equal(t, tt.expectedSLI.Namespace, sli.Namespace)
			assert.Equal(t, tt.expectedSLI.Spec.Description, sli.Spec.Description)

			// Verify owner reference is set
			require.Len(t, sli.OwnerReferences, 1)
			ownerRef := sli.OwnerReferences[0]
			assert.Equal(t, "SLO", ownerRef.Kind)
			assert.Equal(t, tt.slo.Name, ownerRef.Name)
			assert.Equal(t, tt.slo.UID, ownerRef.UID)
			assert.True(t, *ownerRef.Controller)
			assert.True(t, *ownerRef.BlockOwnerDeletion)

			// Verify SLI was created in the fake client
			retrievedSLI := &openslov1.SLI{}
			err = client.Get(ctx, types.NamespacedName{
				Name:      sli.Name,
				Namespace: sli.Namespace,
			}, retrievedSLI)
			require.NoError(t, err)
			assert.Equal(t, sli.Name, retrievedSLI.Name)
		})
	}
}

func TestCreateAlertManagerConfig(t *testing.T) {
	// Setup scheme
	s := runtime.NewScheme()
	_ = openslov1.AddToScheme(s)
	_ = oskov1alpha1.AddToScheme(s)

	tests := []struct {
		name        string
		slo         *openslov1.SLO
		datasource  *openslov1.Datasource
		expectedAMC *oskov1alpha1.AlertManagerConfig
	}{
		{
			name: "create AlertManagerConfig with proper metadata",
			slo: &openslov1.SLO{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "payment-slo",
					Namespace: "monitoring",
					Annotations: map[string]string{
						"osko.dev/datasourceRef": "mimir-ds",
					},
				},
			},
			datasource: &openslov1.Datasource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mimir-ds",
					Namespace: "monitoring",
				},
			},
			expectedAMC: &oskov1alpha1.AlertManagerConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "payment-slo-alerting",
					Namespace: "monitoring",
					Labels: map[string]string{
						"app.kubernetes.io/name":       "osko",
						"app.kubernetes.io/managed-by": "osko-controller",
						"osko.dev/slo":                 "payment-slo",
					},
					Annotations: map[string]string{
						"osko.dev/datasourceRef": "mimir-ds",
					},
				},
				Spec: oskov1alpha1.AlertManagerConfigSpec{
					SecretRef: oskov1alpha1.SecretRef{
						Name:      "payment-slo-alerting-config",
						Namespace: "monitoring",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reconciler := &SLOReconciler{
				Scheme: s,
			}

			ctx := context.Background()
			amc, err := reconciler.createAlertManagerConfig(ctx, tt.slo, tt.datasource)

			require.NoError(t, err)
			assert.NotNil(t, amc)

			// Verify metadata
			assert.Equal(t, tt.expectedAMC.Name, amc.Name)
			assert.Equal(t, tt.expectedAMC.Namespace, amc.Namespace)
			assert.Equal(t, tt.expectedAMC.Labels, amc.Labels)
			assert.Equal(t, tt.expectedAMC.Annotations, amc.Annotations)

			// Verify spec
			assert.Equal(t, tt.expectedAMC.Spec.SecretRef.Name, amc.Spec.SecretRef.Name)
			assert.Equal(t, tt.expectedAMC.Spec.SecretRef.Namespace, amc.Spec.SecretRef.Namespace)
		})
	}
}

func TestFinalizerManagement(t *testing.T) {
	tests := []struct {
		name            string
		initialSLO      *openslov1.SLO
		expectFinalizer bool
	}{
		{
			name: "SLO without finalizer should get one",
			initialSLO: &openslov1.SLO{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-slo",
					Namespace:  "default",
					Finalizers: []string{},
				},
			},
			expectFinalizer: true,
		},
		{
			name: "SLO with finalizer should keep it",
			initialSLO: &openslov1.SLO{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slo",
					Namespace: "default",
					Finalizers: []string{
						sloFinalizer,
						"some.other/finalizer",
					},
				},
			},
			expectFinalizer: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slo := tt.initialSLO.DeepCopy()

			// Test adding finalizer
			if !controllerutil.ContainsFinalizer(slo, sloFinalizer) {
				controllerutil.AddFinalizer(slo, sloFinalizer)
			}

			if tt.expectFinalizer {
				assert.Contains(t, slo.Finalizers, sloFinalizer)
			}

			// Test removing finalizer
			if controllerutil.ContainsFinalizer(slo, sloFinalizer) {
				controllerutil.RemoveFinalizer(slo, sloFinalizer)
			}

			assert.NotContains(t, slo.Finalizers, sloFinalizer)
		})
	}
}

func TestOwnershipPatterns(t *testing.T) {
	// Setup scheme
	s := runtime.NewScheme()
	_ = openslov1.AddToScheme(s)

	tests := []struct {
		name      string
		slo       *openslov1.SLO
		resource  metav1.Object
		shouldOwn bool
		ownerKind string
	}{
		{
			name: "SLO should own inline SLI",
			slo: &openslov1.SLO{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slo",
					Namespace: "default",
					UID:       "slo-uid",
				},
			},
			resource: &openslov1.SLI{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sli",
					Namespace: "default",
				},
			},
			shouldOwn: true,
			ownerKind: "SLO",
		},
		{
			name: "SLO should own PrometheusRule",
			slo: &openslov1.SLO{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slo",
					Namespace: "default",
					UID:       "slo-uid",
				},
			},
			resource: &metav1.ObjectMeta{
				Name:      "test-prometheus-rule",
				Namespace: "default",
			},
			shouldOwn: true,
			ownerKind: "SLO",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.shouldOwn {
				err := controllerutil.SetOwnerReference(tt.slo, tt.resource, s)
				require.NoError(t, err)

				ownerRefs := tt.resource.GetOwnerReferences()
				require.Len(t, ownerRefs, 1)

				ownerRef := ownerRefs[0]
				assert.Equal(t, tt.ownerKind, ownerRef.Kind)
				assert.Equal(t, tt.slo.Name, ownerRef.Name)
				assert.Equal(t, tt.slo.UID, ownerRef.UID)
				assert.True(t, *ownerRef.Controller)
				assert.True(t, *ownerRef.BlockOwnerDeletion)
			}
		})
	}
}

func TestMagicAlertingDetection(t *testing.T) {
	tests := []struct {
		name        string
		slo         *openslov1.SLO
		shouldAlert bool
	}{
		{
			name: "SLO with magic alerting enabled",
			slo: &openslov1.SLO{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"osko.dev/magicAlerting": "true",
					},
				},
			},
			shouldAlert: true,
		},
		{
			name: "SLO with magic alerting disabled",
			slo: &openslov1.SLO{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"osko.dev/magicAlerting": "false",
					},
				},
			},
			shouldAlert: false,
		},
		{
			name: "SLO without magic alerting annotation",
			slo: &openslov1.SLO{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
			shouldAlert: false,
		},
		{
			name:        "SLO with no annotations",
			slo:         &openslov1.SLO{},
			shouldAlert: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasAlerting := tt.slo.ObjectMeta.Annotations["osko.dev/magicAlerting"] == "true"
			assert.Equal(t, tt.shouldAlert, hasAlerting)
		})
	}
}

func TestSLITypeDetection(t *testing.T) {
	tests := []struct {
		name      string
		slo       *openslov1.SLO
		sliType   string
		shouldOwn bool
	}{
		{
			name: "SLO with inline SLI",
			slo: &openslov1.SLO{
				Spec: openslov1.SLOSpec{
					Indicator: &openslov1.Indicator{
						Metadata: metav1.ObjectMeta{
							Name: "inline-sli",
						},
					},
				},
			},
			sliType:   "inline",
			shouldOwn: true,
		},
		{
			name: "SLO with referenced SLI",
			slo: &openslov1.SLO{
				Spec: openslov1.SLOSpec{
					IndicatorRef: stringPtr("shared-sli"),
				},
			},
			sliType:   "reference",
			shouldOwn: false,
		},
		{
			name: "SLO with no SLI (invalid)",
			slo: &openslov1.SLO{
				Spec: openslov1.SLOSpec{},
			},
			sliType:   "none",
			shouldOwn: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var actualType string
			var shouldOwn bool

			if tt.slo.Spec.Indicator != nil {
				actualType = "inline"
				shouldOwn = true
			} else if tt.slo.Spec.IndicatorRef != nil {
				actualType = "reference"
				shouldOwn = false
			} else {
				actualType = "none"
				shouldOwn = false
			}

			assert.Equal(t, tt.sliType, actualType)
			assert.Equal(t, tt.shouldOwn, shouldOwn)
		})
	}
}

// Helper function for string pointers
func stringPtr(s string) *string {
	return &s
}
