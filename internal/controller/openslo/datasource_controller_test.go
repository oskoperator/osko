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
	"k8s.io/client-go/tools/record"
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
