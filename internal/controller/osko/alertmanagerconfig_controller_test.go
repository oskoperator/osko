package osko

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	oskov1alpha1 "github.com/oskoperator/osko/api/osko/v1alpha1"
)

func amcTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(s))
	require.NoError(t, oskov1alpha1.AddToScheme(s))
	return s
}

func amc(name, namespace, secretName, secretNamespace string) *oskov1alpha1.AlertManagerConfig {
	return &oskov1alpha1.AlertManagerConfig{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: oskov1alpha1.AlertManagerConfigSpec{
			SecretRef: oskov1alpha1.SecretRef{Name: secretName, Namespace: secretNamespace},
		},
	}
}

func secretNamed(name, namespace string) *corev1.Secret {
	return &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
}

// TestFindObjectsForSecret covers the mapping that makes the Secret watch work.
// The SLO controller names the Secret "<slo>-alerting-config" while the
// AlertManagerConfig it creates is "<slo>-alerting", so a mapping keyed on the
// Secret's own name never matches: the AlertManagerConfig stays NotReady until
// something else happens to trigger a reconcile.
func TestFindObjectsForSecret(t *testing.T) {
	tests := []struct {
		name    string
		objects []*oskov1alpha1.AlertManagerConfig
		secret  *corev1.Secret
		want    []string
	}{
		{
			name: "osko naming: secret name differs from the AlertManagerConfig name",
			objects: []*oskov1alpha1.AlertManagerConfig{
				amc("my-slo-alerting", "default", "my-slo-alerting-config", "default"),
			},
			secret: secretNamed("my-slo-alerting-config", "default"),
			want:   []string{"my-slo-alerting"},
		},
		{
			name: "secretRef with an empty namespace defaults to the AlertManagerConfig's own",
			objects: []*oskov1alpha1.AlertManagerConfig{
				amc("my-slo-alerting", "default", "my-slo-alerting-config", ""),
			},
			secret: secretNamed("my-slo-alerting-config", "default"),
			want:   []string{"my-slo-alerting"},
		},
		{
			name: "only the referencing AlertManagerConfig is enqueued",
			objects: []*oskov1alpha1.AlertManagerConfig{
				amc("a-alerting", "default", "a-alerting-config", "default"),
				amc("b-alerting", "default", "b-alerting-config", "default"),
			},
			secret: secretNamed("b-alerting-config", "default"),
			want:   []string{"b-alerting"},
		},
		{
			name: "several AlertManagerConfigs may share one Secret",
			objects: []*oskov1alpha1.AlertManagerConfig{
				amc("a-alerting", "default", "shared-config", "default"),
				amc("b-alerting", "default", "shared-config", "default"),
			},
			secret: secretNamed("shared-config", "default"),
			want:   []string{"a-alerting", "b-alerting"},
		},
		{
			name: "a Secret nothing references enqueues nothing",
			objects: []*oskov1alpha1.AlertManagerConfig{
				amc("a-alerting", "default", "a-alerting-config", "default"),
			},
			secret: secretNamed("unrelated", "default"),
			want:   nil,
		},
		{
			name: "same Secret name in another namespace does not match",
			objects: []*oskov1alpha1.AlertManagerConfig{
				amc("a-alerting", "default", "a-alerting-config", "default"),
			},
			secret: secretNamed("a-alerting-config", "other"),
			want:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := fake.NewClientBuilder().WithScheme(amcTestScheme(t))
			for _, o := range tt.objects {
				builder = builder.WithObjects(o)
			}
			r := &AlertManagerConfigReconciler{Client: builder.Build()}

			got := r.findObjectsForSecret()(context.Background(), tt.secret)

			names := make([]string, 0, len(got))
			for _, req := range got {
				assert.Equal(t, "default", req.Namespace,
					"the request must be keyed by the AlertManagerConfig, not the Secret")
				names = append(names, req.Name)
			}
			assert.ElementsMatch(t, tt.want, names)
		})
	}
}
