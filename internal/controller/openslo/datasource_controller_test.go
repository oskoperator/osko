package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
