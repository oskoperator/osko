package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	openslov1 "github.com/oskoperator/osko/api/openslo/v1"
	oskov1alpha1 "github.com/oskoperator/osko/api/osko/v1alpha1"
	"github.com/oskoperator/osko/internal/backend"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestCustomRoundTripperTenantHeader(t *testing.T) {
	tests := []struct {
		name       string
		header     string
		tenantID   string
		wantHeader string
		wantValue  string
		wantAbsent []string
	}{
		{
			name:       "mimir tenant is sent as X-Scope-OrgID",
			header:     "X-Scope-OrgID",
			tenantID:   "team-a",
			wantHeader: "X-Scope-OrgID",
			wantValue:  "team-a",
		},
		{
			name:       "thanos tenant is sent as THANOS-TENANT",
			header:     "THANOS-TENANT",
			tenantID:   "team-b",
			wantHeader: "THANOS-TENANT",
			wantValue:  "team-b",
		},
		{
			name:       "no header is sent when the tenant is empty",
			header:     "X-Scope-OrgID",
			tenantID:   "",
			wantAbsent: []string{"X-Scope-OrgID", "THANOS-TENANT"},
		},
		{
			name:       "no header is sent when the backend has no tenancy header",
			header:     "",
			tenantID:   "team-c",
			wantAbsent: []string{"X-Scope-OrgID", "THANOS-TENANT"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got http.Header
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = r.Header.Clone()
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			rt := &CustomRoundTripper{
				Transport:    http.DefaultTransport,
				TenantHeader: tt.header,
				TenantID:     tt.tenantID,
			}

			req, err := http.NewRequest(http.MethodGet, server.URL, nil)
			require.NoError(t, err)

			resp, err := rt.RoundTrip(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			if tt.wantHeader != "" {
				assert.Equal(t, tt.wantValue, got.Get(tt.wantHeader))
			}
			for _, absent := range tt.wantAbsent {
				assert.Empty(t, got.Get(absent), "expected %s to be absent", absent)
			}
		})
	}
}

func TestConnectDatasourceWiring(t *testing.T) {
	tests := []struct {
		name         string
		datasource   string
		targetTenant string
		wantPath     string
		wantHeader   string
		wantValue    string
		wantAbsent   []string
	}{
		{
			name:         "mimir queries under the prometheus sub-path with X-Scope-OrgID",
			datasource:   "mimir",
			targetTenant: "team-a",
			wantPath:     "/prometheus/api/v1/query",
			wantHeader:   "X-Scope-OrgID",
			wantValue:    "team-a",
			wantAbsent:   []string{"THANOS-TENANT"},
		},
		{
			name:         "cortex queries under the prometheus sub-path with X-Scope-OrgID",
			datasource:   "cortex",
			targetTenant: "team-b",
			wantPath:     "/prometheus/api/v1/query",
			wantHeader:   "X-Scope-OrgID",
			wantValue:    "team-b",
			wantAbsent:   []string{"THANOS-TENANT"},
		},
		{
			name:         "thanos queries at the root with THANOS-TENANT",
			datasource:   "thanos",
			targetTenant: "team-c",
			wantPath:     "/api/v1/query",
			wantHeader:   "THANOS-TENANT",
			wantValue:    "team-c",
			wantAbsent:   []string{"X-Scope-OrgID"},
		},
		{
			name:       "prometheus queries at the root with no tenant header",
			datasource: "prometheus",
			wantPath:   "/api/v1/query",
			wantAbsent: []string{"X-Scope-OrgID", "THANOS-TENANT"},
		},
		{
			name:       "victoriametrics queries at the root with no tenant header",
			datasource: "victoriametrics",
			wantPath:   "/api/v1/query",
			wantAbsent: []string{"X-Scope-OrgID", "THANOS-TENANT"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPath string
			var gotHeader http.Header

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				gotHeader = r.Header.Clone()
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
			}))
			defer server.Close()

			backendType, err := backend.Parse(tt.datasource)
			require.NoError(t, err)

			ds := &openslov1.Datasource{
				ObjectMeta: metav1.ObjectMeta{Name: "test-ds", Namespace: "default"},
				Spec: openslov1.DatasourceSpec{
					Type: tt.datasource,
					ConnectionDetails: oskov1alpha1.ConnectionDetails{
						Address:      server.URL,
						TargetTenant: tt.targetTenant,
					},
				},
			}

			r := &DatasourceReconciler{Recorder: record.NewFakeRecorder(10)}
			require.NoError(t, r.connectDatasource(context.Background(), ds, backendType))

			assert.Equal(t, tt.wantPath, gotPath,
				"query path must reflect the backend's API layout")
			if tt.wantHeader != "" {
				assert.Equal(t, tt.wantValue, gotHeader.Get(tt.wantHeader))
			}
			for _, absent := range tt.wantAbsent {
				assert.Empty(t, gotHeader.Get(absent), "expected %s to be absent", absent)
			}
		})
	}
}

// TestDatasourceReconcileHandlesEveryBackend drives the full reconcile, not just
// connectDatasource, so the backend switch is actually exercised. The default
// arm must stay unreachable for every backend Parse accepts: reaching it would
// mean a real backend had stopped being connected to.
func TestDatasourceReconcileHandlesEveryBackend(t *testing.T) {
	tests := []struct {
		name        string
		datasource  string
		wantReady   string
		wantMessage string
	}{
		{name: "mimir", datasource: "mimir", wantReady: "True", wantMessage: "Datasource reconciled"},
		{name: "thanos", datasource: "thanos", wantReady: "True", wantMessage: "Datasource reconciled"},
		{name: "prometheus", datasource: "prometheus", wantReady: "True", wantMessage: "Datasource reconciled"},
		{name: "victoriametrics", datasource: "victoriametrics", wantReady: "True", wantMessage: "Datasource reconciled"},
		{name: "cortex", datasource: "cortex", wantReady: "False", wantMessage: "Cortex support is not implemented yet"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
			}))
			defer server.Close()

			ds := &openslov1.Datasource{
				ObjectMeta: metav1.ObjectMeta{Name: "test-ds", Namespace: "default"},
				Spec: openslov1.DatasourceSpec{
					Type: tt.datasource,
					ConnectionDetails: oskov1alpha1.ConnectionDetails{
						Address: server.URL,
					},
				},
			}

			s := gatingTestScheme(t)
			c := gatingClient(s, ds)
			r := &DatasourceReconciler{Client: c, Scheme: s, Recorder: record.NewFakeRecorder(20)}

			key := types.NamespacedName{Name: "test-ds", Namespace: "default"}
			_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: key})
			require.NoError(t, err, "%s is a real backend and must reconcile cleanly", tt.datasource)

			got := &openslov1.Datasource{}
			require.NoError(t, c.Get(context.Background(), key, got))
			assert.Equal(t, tt.wantReady, got.Status.Ready)

			require.NotEmpty(t, got.Status.Conditions)
			msg := got.Status.Conditions[len(got.Status.Conditions)-1].Message
			assert.Equal(t, tt.wantMessage, msg,
				"%s must not fall through to the unhandled-type arm", tt.datasource)
			assert.NotContains(t, msg, "not handled by the Datasource controller")
		})
	}
}
