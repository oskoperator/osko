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
		{name: "victoriametrics", input: "victoriametrics", want: VictoriaMetrics},
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
				assert.Empty(t, got, "no Type may be returned alongside an error")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestParseErrorPreservesRawInput pins the diagnostic value of the error: the
// operator sees exactly what they typed, whitespace and casing included.
func TestParseErrorPreservesRawInput(t *testing.T) {
	_, err := Parse("  Thanso  ")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"  Thanso  "`)
}

func TestTypeNeedsRemoteRulePush(t *testing.T) {
	tests := []struct {
		name string
		typ  Type
		want bool
	}{
		{name: "mimir pushes to a ruler API", typ: Mimir, want: true},
		{name: "cortex pushes to a ruler API", typ: Cortex, want: true},
		{name: "thanos reads PrometheusRule objects", typ: Thanos, want: false},
		{name: "prometheus reads PrometheusRule objects", typ: Prometheus, want: false},
		{name: "victoriametrics reads PrometheusRule objects", typ: VictoriaMetrics, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.typ.NeedsRemoteRulePush())
		})
	}
}

func TestTypeSupportsMagicAlerting(t *testing.T) {
	tests := []struct {
		name string
		typ  Type
		want bool
	}{
		{name: "mimir has an Alertmanager API", typ: Mimir, want: true},
		{name: "cortex has an Alertmanager API", typ: Cortex, want: true},
		{name: "thanos has no Alertmanager API", typ: Thanos, want: false},
		{name: "prometheus has no Alertmanager API", typ: Prometheus, want: false},
		{name: "victoriametrics has no Alertmanager API", typ: VictoriaMetrics, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.typ.SupportsMagicAlerting())
		})
	}
}

func TestTypeQueryURL(t *testing.T) {
	tests := []struct {
		name    string
		typ     Type
		address string
		want    string
	}{
		{
			name:    "mimir gets the prometheus sub-path",
			typ:     Mimir,
			address: "http://mimir:9009",
			want:    "http://mimir:9009/prometheus",
		},
		{
			name:    "a trailing slash does not produce a double slash",
			typ:     Mimir,
			address: "http://mimir:9009/",
			want:    "http://mimir:9009/prometheus",
		},
		{
			name:    "repeated trailing slashes are all trimmed",
			typ:     Mimir,
			address: "http://mimir:9009///",
			want:    "http://mimir:9009/prometheus",
		},
		{
			name:    "cortex gets the prometheus sub-path",
			typ:     Cortex,
			address: "http://cortex:9009",
			want:    "http://cortex:9009/prometheus",
		},
		{
			name:    "thanos serves at the root",
			typ:     Thanos,
			address: "http://thanos-query:9090",
			want:    "http://thanos-query:9090",
		},
		{
			name:    "thanos trailing slash is trimmed",
			typ:     Thanos,
			address: "http://thanos-query:9090/",
			want:    "http://thanos-query:9090",
		},
		{
			name:    "prometheus serves at the root",
			typ:     Prometheus,
			address: "http://prometheus:9090",
			want:    "http://prometheus:9090",
		},
		{
			name:    "victoriametrics serves at the root",
			typ:     VictoriaMetrics,
			address: "http://victoria:8428",
			want:    "http://victoria:8428",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.typ.QueryURL(tt.address))
		})
	}
}

func TestTypeTenantHeader(t *testing.T) {
	tests := []struct {
		name string
		typ  Type
		want string
	}{
		{name: "mimir uses the Cortex-style header", typ: Mimir, want: "X-Scope-OrgID"},
		{name: "cortex uses the Cortex-style header", typ: Cortex, want: "X-Scope-OrgID"},
		{name: "thanos uses its own header", typ: Thanos, want: "THANOS-TENANT"},
		{name: "prometheus has no tenancy header", typ: Prometheus, want: ""},
		{name: "victoriametrics has no tenancy header", typ: VictoriaMetrics, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.typ.TenantHeader())
		})
	}
}
