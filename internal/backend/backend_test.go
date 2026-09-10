package backend

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Type
		wantErr bool
	}{
		{name: "mimir", input: "mimir", want: Mimir},
		{name: "cortex", input: "cortex", want: Cortex},
		{name: "thanos", input: "thanos", want: Thanos},
		{name: "prometheus", input: "prometheus", want: Prometheus},
		{name: "mixed case is accepted", input: "Thanos", want: Thanos},
		{name: "upper case is accepted", input: "MIMIR", want: Mimir},
		{name: "surrounding whitespace is trimmed", input: "  thanos  ", want: Thanos},
		{name: "unknown type is an error", input: "thanso", wantErr: true},
		{name: "empty type is an error", input: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "unsupported datasource type")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNeedsRemoteRulePush(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "mimir pushes to a ruler API", input: "mimir", want: true},
		{name: "cortex pushes to a ruler API", input: "cortex", want: true},
		{name: "thanos reads PrometheusRule objects", input: "thanos", want: false},
		{name: "prometheus reads PrometheusRule objects", input: "prometheus", want: false},
		{name: "unknown types never push", input: "thanso", want: false},
		{name: "empty type never pushes", input: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, NeedsRemoteRulePush(tt.input))
		})
	}
}

func TestSupportsMagicAlerting(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "mimir has an Alertmanager API", input: "mimir", want: true},
		{name: "cortex has an Alertmanager API", input: "cortex", want: true},
		{name: "thanos has no Alertmanager API", input: "thanos", want: false},
		{name: "prometheus has no Alertmanager API", input: "prometheus", want: false},
		{name: "unknown types do not", input: "thanso", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, SupportsMagicAlerting(tt.input))
		})
	}
}

func TestQueryURL(t *testing.T) {
	tests := []struct {
		name    string
		typ     string
		address string
		want    string
		wantErr bool
	}{
		{
			name:    "mimir gets the prometheus sub-path",
			typ:     "mimir",
			address: "http://mimir:9009",
			want:    "http://mimir:9009/prometheus",
		},
		{
			name:    "a trailing slash does not produce a double slash",
			typ:     "mimir",
			address: "http://mimir:9009/",
			want:    "http://mimir:9009/prometheus",
		},
		{
			name:    "cortex gets the prometheus sub-path",
			typ:     "cortex",
			address: "http://cortex:9009",
			want:    "http://cortex:9009/prometheus",
		},
		{
			name:    "thanos serves at the root",
			typ:     "thanos",
			address: "http://thanos-query:9090",
			want:    "http://thanos-query:9090",
		},
		{
			name:    "thanos trailing slash is trimmed",
			typ:     "thanos",
			address: "http://thanos-query:9090/",
			want:    "http://thanos-query:9090",
		},
		{
			name:    "prometheus serves at the root",
			typ:     "prometheus",
			address: "http://prometheus:9090",
			want:    "http://prometheus:9090",
		},
		{
			name:    "unknown type is an error",
			typ:     "thanso",
			address: "http://whatever:9090",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := QueryURL(tt.typ, tt.address)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTenantHeader(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "mimir uses the Cortex-style header", input: "mimir", want: "X-Scope-OrgID"},
		{name: "cortex uses the Cortex-style header", input: "cortex", want: "X-Scope-OrgID"},
		{name: "thanos uses its own header", input: "thanos", want: "THANOS-TENANT"},
		{name: "prometheus has no tenancy header", input: "prometheus", want: ""},
		{name: "unknown types have no tenancy header", input: "thanso", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, TenantHeader(tt.input))
		})
	}
}
