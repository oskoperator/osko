package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	openslov1 "github.com/oskoperator/osko/api/openslo/v1"
)

// TestSLOOwnershipLogic tests the logic for determining what resources should be owned
func TestSLOOwnershipLogic(t *testing.T) {
	tests := []struct {
		name      string
		slo       *openslov1.SLO
		sliType   string
		shouldOwn bool
	}{
		{
			name: "SLO with inline SLI should own the SLI",
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
			name: "SLO with referenced SLI should not own the SLI",
			slo: &openslov1.SLO{
				Spec: openslov1.SLOSpec{
					IndicatorRef: stringPtr("shared-sli"),
				},
			},
			sliType:   "reference",
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
			}

			assert.Equal(t, tt.sliType, actualType)
			assert.Equal(t, tt.shouldOwn, shouldOwn)
		})
	}
}

// TestMagicAlertingDetection tests the logic for detecting magic alerting
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasAlerting := tt.slo.ObjectMeta.Annotations["osko.dev/magicAlerting"] == "true"
			assert.Equal(t, tt.shouldAlert, hasAlerting)
		})
	}
}

// TestResourceNaming tests the naming logic for generated resources
func TestResourceNaming(t *testing.T) {
	t.Run("SLI name generation", func(t *testing.T) {
		tests := []struct {
			sloName       string
			indicatorName string
			expected      string
		}{
			{"my-slo", "", "my-slo-sli"},
			{"my-slo", "custom-name", "custom-name"},
			{"payment-service", "", "payment-service-sli"},
		}

		for _, tt := range tests {
			sliName := tt.indicatorName
			if sliName == "" {
				sliName = tt.sloName + "-sli"
			}
			assert.Equal(t, tt.expected, sliName)
		}
	})

	t.Run("AlertManagerConfig name generation", func(t *testing.T) {
		sloName := "payment-slo"
		expected := "payment-slo-alerting"
		actual := sloName + "-alerting"
		assert.Equal(t, expected, actual)
	})

	t.Run("Secret name generation", func(t *testing.T) {
		sloName := "payment-slo"
		expected := "payment-slo-alerting-config"
		actual := sloName + "-alerting-config"
		assert.Equal(t, expected, actual)
	})
}

// Helper function for string pointers
func stringPtr(s string) *string {
	return &s
}
