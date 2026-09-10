# Thanos Backend Support Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let OSKO drive Thanos as a metrics backend by delivering SLO rules as `PrometheusRule` objects and skipping the Mimir-specific push paths.

**Architecture:** A new `internal/backend` package owns per-backend capability knowledge. The Datasource controller learns to connect to Thanos and Prometheus; the SLO controller gates `MimirRule` and `AlertManagerConfig` creation on backend capability instead of creating them unconditionally. Generated `PrometheusRule` objects gain a stable marker label so a `ThanosRuler.ruleSelector` can find them.

**Tech Stack:** Go 1.x, controller-runtime, kubebuilder, testify, Ginkgo/Gomega + envtest, prometheus-operator API types.

**Spec:** `docs/superpowers/specs/2026-09-10-thanos-support-design.md`

## Global Constraints

- Backend type values are exactly `prometheus`, `mimir`, `cortex`, `thanos`, `victoriametrics` (lowercase).
- Thanos tenant header is exactly `THANOS-TENANT`; Mimir/Cortex is exactly `X-Scope-OrgID`.
- Mimir and Cortex serve the Prometheus API under `/prometheus`; Thanos and Prometheus serve it at the root.
- Marker labels are exactly `app.kubernetes.io/managed-by: osko` and `osko.dev/slo: <slo name>`.
- Commit style is Conventional Commits with a scope, signed off with `git commit -s` (repo DCO requirement).
- After any change to `api/`, run `make manifests generate` and then `make helm-crds`.
- Do not commit CRDs into the Helm subchart; `make helm-crds` output is a local-dev artifact only.
- Run `make test` before considering any task done.

## Deviation from the spec's testing section (read before Task 1)

The spec's testing section calls for envtest integration tests of the SLO reconciler. That is **not reachable** in this repository today, verified as follows:

- No `suite_test.go` starts a manager or registers a reconciler — `grep -rn "ctrl.NewManager\|SetupWithManager\|mgr.Start" internal/controller/ --include="*_test.go"` returns nothing.
- `CRDDirectoryPaths` in all three suites points only at `config/crd/bases`, which does not contain prometheus-operator CRDs, so a `PrometheusRule` cannot be created in envtest.
- `monitoringv1` is never added to the test scheme.
- Nothing in the `Makefile` fetches prometheus-operator CRDs for tests.

Making those tests work requires vendoring prometheus-operator CRDs, extending `CRDDirectoryPaths`, registering `monitoringv1`, and starting a manager in the suite. That is standalone test-infrastructure work unrelated to Thanos.

**Correction, made during Task 4 planning.** The paragraph above over-generalises. What is unreachable is running a *manager* under envtest — it needs the prometheus-operator CRDs. Calling `Reconcile` directly is a different matter: `sigs.k8s.io/controller-runtime/pkg/client/fake` needs only a scheme, and `github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.74.0` is already a direct dependency, so `monitoringv1` types register without any CRD YAML. Task 4 uses that to test the reconciler's gating behaviour for real.

**This plan therefore:**

- Puts all decision logic in pure functions in `internal/backend`, so every gating decision is unit-testable without a cluster (Task 1).
- Uses envtest for CRD schema validation, which **is** reachable with existing infrastructure (Task 2).
- Drives `connectDatasource` (Task 3) and `Reconcile` (Task 4) directly, against `httptest` and the fake client respectively, so the behaviour each task delivers is covered rather than merely inspected.
- Records the remaining gap — a real manager under envtest, exercising watches and the full controller runtime — as a follow-up in the ADR (Task 6).

---

### Task 1: The `internal/backend` capability package

Pure functions, no Kubernetes dependencies. Everything downstream consumes this.

**Files:**
- Create: `internal/backend/backend.go`
- Test: `internal/backend/backend_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `type Type string` with constants `Prometheus`, `Mimir`, `Cortex`, `Thanos`, `VictoriaMetrics` (values `"prometheus"`, `"mimir"`, `"cortex"`, `"thanos"`, `"victoriametrics"`)
  - `const TenantHeaderMimir = "X-Scope-OrgID"`
  - `const TenantHeaderThanos = "THANOS-TENANT"`
  - `func Parse(t string) (Type, error)`
  - `func (t Type) NeedsRemoteRulePush() bool`
  - `func (t Type) SupportsMagicAlerting() bool`
  - `func (t Type) QueryURL(address string) string`
  - `func (t Type) TenantHeader() string`

**Why the capabilities are methods on `Type` and not functions taking `string`:** `Parse` is the single validation boundary. Once a caller holds a `Type`, it has already dealt with the unknown-backend error. Had the capabilities taken a raw `string`, each would have had to swallow a parse error and return a zero value — turning a typo like `mimr` into a silently skipped MimirRule and a green reconcile. As methods, they are unreachable without handling the error first, so silent degradation is unrepresentable rather than merely discouraged.

`QueryURL` and `TenantHeader` return no error for the same reason: by the time you hold a `Type`, there is no error left to report.

- [ ] **Step 1: Write the failing test**

Create `internal/backend/backend_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/backend/... -v`

Expected: FAIL. If the package does not exist yet you will see `matched no packages`, which is not a test failure — create `backend.go` with only the `package backend` line first, then re-run so the failure is a real compile error naming the undefined symbols (`undefined: Parse`, `undefined: Mimir`, ...). Record that output; it is your RED evidence.

- [ ] **Step 3: Write minimal implementation**

Create `internal/backend/backend.go`:

```go
// Package backend describes the metrics backends OSKO can target and the
// capabilities each one provides. Controllers ask this package what a backend
// can do rather than comparing type strings.
//
// Note that this is not the only place backend names appear:
// internal/helpers.isPrometheusSource keeps a separate list for the SLI
// metric-source dialect, which is a different field with different values.
package backend

import (
	"fmt"
	"strings"
)

// Type identifies a metrics backend. The values match the enum accepted by
// Datasource.spec.type.
//
// Obtain a Type through Parse. The capability methods below are deliberately
// methods rather than functions over a raw string: Parse is the single
// validation boundary, so holding a Type means the unknown-backend error has
// already been handled and cannot be silently swallowed.
type Type string

const (
	Prometheus      Type = "prometheus"
	Mimir           Type = "mimir"
	Cortex          Type = "cortex"
	Thanos          Type = "thanos"
	VictoriaMetrics Type = "victoriametrics"
)

const (
	// TenantHeaderMimir scopes a request to a tenant in Mimir and Cortex.
	TenantHeaderMimir = "X-Scope-OrgID"

	// TenantHeaderThanos scopes a request to a tenant in Thanos Query. It
	// matches the default of Thanos' --query.tenant-header flag.
	TenantHeaderThanos = "THANOS-TENANT"

	// prometheusAPISubPath is where Mimir and Cortex expose the Prometheus
	// HTTP API. The others expose it at the root.
	prometheusAPISubPath = "/prometheus"
)

// Parse normalises a Datasource type string and is the only way to obtain a
// Type from user input.
//
// Matching is case-insensitive on purpose: the CRD enum only validates on
// write, so a Datasource stored before the enum was introduced can still be
// read back with mixed case.
func Parse(t string) (Type, error) {
	switch Type(strings.ToLower(strings.TrimSpace(t))) {
	case Prometheus:
		return Prometheus, nil
	case Mimir:
		return Mimir, nil
	case Cortex:
		return Cortex, nil
	case Thanos:
		return Thanos, nil
	case VictoriaMetrics:
		return VictoriaMetrics, nil
	default:
		// Report the raw input, not the normalised form, so the operator sees
		// exactly what they typed.
		return "", fmt.Errorf("unsupported datasource type: %q", t)
	}
}

// NeedsRemoteRulePush reports whether rule groups must be pushed to a remote
// ruler configuration API.
//
// Mimir and Cortex expose one. Thanos Ruler has no rule-write API and instead
// reads files rendered by prometheus-operator from PrometheusRule objects;
// Prometheus and VictoriaMetrics work the same way. Neither needs a push.
func (t Type) NeedsRemoteRulePush() bool {
	switch t {
	case Mimir, Cortex:
		return true
	default:
		return false
	}
}

// SupportsMagicAlerting reports whether the backend exposes an Alertmanager
// configuration API that OSKO can write routing configuration to.
//
// Thanos Ruler sends alerts to an Alertmanager configured statically through
// --alertmanagers.url, which is outside OSKO's control.
func (t Type) SupportsMagicAlerting() bool {
	switch t {
	case Mimir, Cortex:
		return true
	default:
		return false
	}
}

// QueryURL returns the base URL of the Prometheus-compatible query API for a
// backend reachable at address.
//
// This is kept separate from NeedsRemoteRulePush even though the two agree on
// today's backend set: serving the query API under a sub-path and exposing a
// ruler write API are unrelated properties that coincide by accident.
func (t Type) QueryURL(address string) string {
	trimmed := strings.TrimRight(address, "/")
	switch t {
	case Mimir, Cortex:
		return trimmed + prometheusAPISubPath
	default:
		return trimmed
	}
}

// TenantHeader returns the HTTP header carrying the tenant identifier for the
// backend, or an empty string when the backend has no tenancy header.
func (t Type) TenantHeader() string {
	switch t {
	case Mimir, Cortex:
		return TenantHeaderMimir
	case Thanos:
		return TenantHeaderThanos
	default:
		return ""
	}
}
```

`QueryURL` uses `TrimRight`, not `TrimSuffix`, so repeated trailing slashes are all removed. The current code appends `/prometheus` without trimming at all, so the shipped sample address `http://localhost:9009/` produces `http://localhost:9009//prometheus`. This is a drive-by fix in code being rewritten anyway.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/backend/... -v`
Expected: PASS, all subtests green, output pristine with no warnings.

Then run the full suite once: `make test`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/backend/backend.go internal/backend/backend_test.go
git commit -s -m "feat(backend): add metrics backend capability package

Introduce a Type with capability methods so controllers ask what a backend
can do instead of comparing type strings. Parse is the single validation
boundary; capabilities are methods on Type so a caller cannot reach them
without first handling the unknown-backend error."
```

---

### Task 2: Datasource API — type enum and status conditions

Two API changes to the same file, sharing one manifest regeneration.

The status fields are a **prerequisite discovered during planning**, not decoration: `utils.UpdateStatus` locates `Status.Conditions` and `Status.Ready` by reflection and silently does nothing when neither exists. `DatasourceStatus` is currently empty, so the spec's "set Datasource status not-ready" behaviour is a no-op without this.

**Files:**
- Modify: `api/openslo/v1/datasource_types.go:16-27`
- Create: `internal/controller/openslo/datasource_validation_test.go`
- Regenerate: `config/crd/bases/openslo.com_datasources.yaml`

**Interfaces:**
- Consumes: nothing.
- Produces: `DatasourceStatus{Conditions []metav1.Condition, Ready string}`, and a `spec.type` enum restricted to `prometheus;mimir;cortex;thanos;victoriametrics`.

- [ ] **Step 1: Write the failing test**

Create `internal/controller/openslo/datasource_validation_test.go`. This is a Ginkgo spec so it runs inside the existing `TestControllers` suite, which is what starts envtest.

```go
package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	openslov1 "github.com/oskoperator/osko/api/openslo/v1"
	oskov1alpha1 "github.com/oskoperator/osko/api/osko/v1alpha1"
)

var _ = Describe("Datasource spec.type validation", func() {
	newDatasource := func(name, dsType string) *openslov1.Datasource {
		return &openslov1.Datasource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "default",
			},
			Spec: openslov1.DatasourceSpec{
				Type: dsType,
				ConnectionDetails: oskov1alpha1.ConnectionDetails{
					Address: "http://example:9090",
				},
			},
		}
	}

	It("rejects a misspelled datasource type", func() {
		err := k8sClient.Create(context.Background(), newDatasource("bad-type", "thanso"))
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("spec.type"))
	})

	It("accepts thanos", func() {
		ds := newDatasource("good-thanos", "thanos")
		Expect(k8sClient.Create(context.Background(), ds)).To(Succeed())
		Expect(k8sClient.Delete(context.Background(), ds)).To(Succeed())
	})

	It("accepts prometheus", func() {
		ds := newDatasource("good-prometheus", "prometheus")
		Expect(k8sClient.Create(context.Background(), ds)).To(Succeed())
		Expect(k8sClient.Delete(context.Background(), ds)).To(Succeed())
	})

	It("accepts mimir", func() {
		ds := newDatasource("good-mimir", "mimir")
		Expect(k8sClient.Create(context.Background(), ds)).To(Succeed())
		Expect(k8sClient.Delete(context.Background(), ds)).To(Succeed())
	})

	It("accepts victoriametrics", func() {
		ds := newDatasource("good-vm", "victoriametrics")
		Expect(k8sClient.Create(context.Background(), ds)).To(Succeed())
		Expect(k8sClient.Delete(context.Background(), ds)).To(Succeed())
	})
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `make test`
Expected: the "rejects a misspelled datasource type" spec FAILS, because `spec.type` currently has no enum and the API server accepts `thanso`.

- [ ] **Step 3: Write minimal implementation**

In `api/openslo/v1/datasource_types.go`, replace the `DatasourceSpec` and `DatasourceStatus` blocks (lines 16-27) with:

```go
// DatasourceSpec defines the desired state of Datasource
type DatasourceSpec struct {
	Description Description `json:"description,omitempty"`

	// Type selects the metrics backend this Datasource points at.
	// +kubebuilder:validation:Enum=prometheus;mimir;cortex;thanos;victoriametrics
	Type string `json:"type,omitempty"`

	ConnectionDetails osko.ConnectionDetails `json:"connectionDetails,omitempty"`
}

// DatasourceStatus defines the observed state of Datasource
type DatasourceStatus struct {
	// Conditions holds the latest observations of the Datasource state.
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Ready mirrors the status of the Ready condition so it can be surfaced
	// as a printer column.
	Ready string `json:"ready,omitempty"`
}
```

Then add the printer column marker to the existing marker block above `type Datasource struct` (currently lines 29-31), matching the pattern used by SLO:

```go
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Namespaced
//+kubebuilder:printcolumn:name="Type",type=string,JSONPath=.spec.type,description="The metrics backend this Datasource points at"
//+kubebuilder:printcolumn:name="Ready",type=string,JSONPath=.status.ready,description="The reason for the current status of the Datasource resource"
```

`metav1` is already imported in this file.

- [ ] **Step 4: Regenerate manifests and run tests**

Run: `make manifests generate && make helm-crds && make test`
Expected: `config/crd/bases/openslo.com_datasources.yaml` now contains an `enum:` list under `spec.properties.type` and a `status` schema with `conditions` and `ready`. All specs PASS.

Confirm the enum landed:

```bash
grep -A6 "type:" config/crd/bases/openslo.com_datasources.yaml | grep -A5 enum
```

- [ ] **Step 5: Commit**

```bash
git add api/openslo/v1/datasource_types.go api/openslo/v1/zz_generated.deepcopy.go \
        config/crd/bases/openslo.com_datasources.yaml \
        internal/controller/openslo/datasource_validation_test.go
git commit -s -m "feat(api)!: validate Datasource type and add status conditions

Restrict spec.type to the backends OSKO understands so a typo fails
at apply time instead of silently producing no rules.

Add Conditions and Ready to DatasourceStatus. utils.UpdateStatus finds
these fields by reflection and was previously a no-op for Datasource,
which left connection failures invisible in the resource.

BREAKING CHANGE: an existing Datasource whose type is outside the enum
becomes invalid and cannot be updated until corrected."
```

---

### Task 3: Datasource controller supports Thanos and Prometheus

**Files:**
- Modify: `internal/controller/openslo/datasource_controller.go:34-37` (CustomRoundTripper), `:59-76` (Reconcile switch), `:78-114` (connectDatasource and RoundTrip)
- Test: `internal/controller/openslo/datasource_controller_test.go` (create)

**Interfaces:**
- Consumes: `backend.Parse` and the `Type` methods `QueryURL` / `TenantHeader`, plus the constants `backend.Thanos`, `backend.Mimir`, `backend.Cortex`, `backend.Prometheus`, `backend.VictoriaMetrics` from Task 1. `utils.UpdateStatus` and the status fields from Task 2.
- Produces: `CustomRoundTripper{Transport http.RoundTripper, TenantHeader string, TenantID string}`, and `connectDatasource(ctx, ds, backendType)` which now takes the parsed type.

- [ ] **Step 1: Write the failing test**

Create `internal/controller/openslo/datasource_controller_test.go`:

```go
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
```

The test above covers the `RoundTrip` guard in isolation. It does not exercise the wiring
this task exists to deliver, so add a second test that drives `connectDatasource` against a
real HTTP server. Without it, reverting the query URL to a hardcoded `/prometheus` and the
header to a hardcoded `X-Scope-OrgID` would leave the suite green.

```go
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
```

This needs these imports beyond the first test's: `context`, `github.com/oskoperator/osko/internal/backend`,
`openslov1 "github.com/oskoperator/osko/api/openslo/v1"`,
`oskov1alpha1 "github.com/oskoperator/osko/api/osko/v1alpha1"`,
`metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"`, and `k8s.io/client-go/tools/record`.

`record.NewFakeRecorder(10)` is required because `connectDatasource` emits events on both the
success and failure paths; a nil Recorder panics. The buffer of 10 is ample for one call.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/controller/openslo/... -run TestCustomRoundTripperTenantHeader -v`
Expected: FAIL to compile — `CustomRoundTripper` has no `TenantHeader` field.

- [ ] **Step 3: Write minimal implementation**

3a. Add imports to `internal/controller/openslo/datasource_controller.go`:

```go
	"github.com/oskoperator/osko/internal/backend"
	"github.com/oskoperator/osko/internal/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
```

3b. Replace the `CustomRoundTripper` struct (lines 34-37):

```go
type CustomRoundTripper struct {
	Transport http.RoundTripper
	// TenantHeader is the HTTP header carrying the tenant identifier for the
	// backend, or empty when the backend has no tenancy header.
	TenantHeader string
	TenantID     string
}
```

3c. Replace the `switch ds.Spec.Type` block (lines 59-75) with:

```go
	backendType, err := backend.Parse(ds.Spec.Type)
	if err != nil {
		log.Error(err, "unsupported datasource type", "type", ds.Spec.Type)
		r.Recorder.Event(ds, "Warning", "UnsupportedDatasourceType", err.Error())
		if statusErr := utils.UpdateStatus(ctx, ds, r.Client, "Ready", metav1.ConditionFalse, err.Error()); statusErr != nil {
			log.Error(statusErr, "Failed to update Datasource status")
			return ctrl.Result{}, errors.Transient(statusErr, 5*time.Second)
		}
		return ctrl.Result{}, errors.Permanent(err)
	}

	switch backendType {
	case backend.Mimir, backend.Thanos, backend.Prometheus, backend.VictoriaMetrics:
		log.Info("Connecting Datasource", "type", string(backendType), "address", ds.Spec.ConnectionDetails.Address)
		if err := r.connectDatasource(ctx, ds, backendType); err != nil {
			log.Error(err, errConnectDS)
			if statusErr := utils.UpdateStatus(ctx, ds, r.Client, "Ready", metav1.ConditionFalse, errConnectDS); statusErr != nil {
				log.Error(statusErr, "Failed to update Datasource status")
			}
			return ctrl.Result{}, errors.Transient(err, 5*time.Second)
		}
	case backend.Cortex:
		log.Info("Datasource Type is Cortex", "address", ds.Spec.ConnectionDetails.Address)
		r.Recorder.Event(ds, "Warning", "NotImplemented", "Cortex support is not implemented yet")
		if err := utils.UpdateStatus(ctx, ds, r.Client, "Ready", metav1.ConditionFalse,
			"Cortex support is not implemented yet"); err != nil {
			log.Error(err, "Failed to update Datasource status")
			return ctrl.Result{}, errors.Transient(err, 5*time.Second)
		}
		return ctrl.Result{}, nil
	}

	if backendType == backend.Thanos && len(ds.Spec.ConnectionDetails.SourceTenants) > 0 {
		r.Recorder.Event(ds, "Warning", "SourceTenantsIgnored",
			"sourceTenants has no Thanos equivalent and is ignored")
	}

	if err := utils.UpdateStatus(ctx, ds, r.Client, "Ready", metav1.ConditionTrue, "Datasource reconciled"); err != nil {
		log.Error(err, "Failed to update Datasource status")
		return ctrl.Result{}, errors.Transient(err, 5*time.Second)
	}

	log.V(1).Info("Datasource reconciled")
	r.Recorder.Event(ds, "Normal", "DatasourceReconciled", "Datasource reconciled")

	return ctrl.Result{}, nil
```

3d. Change the `connectDatasource` signature so it receives the already-parsed type instead of re-deriving it, and replace the address-resolution block at the top of the function (lines 78-85).

The signature becomes:

```go
func (r *DatasourceReconciler) connectDatasource(ctx context.Context, ds *openslov1.Datasource, backendType backend.Type) error {
```

and the first statements become:

```go
	datasourceAddress := backendType.QueryURL(ds.Spec.ConnectionDetails.Address)
```

3e. Update the round tripper construction in `connectDatasource` (lines 87-90):

```go
	customRoundtripper := &CustomRoundTripper{
		Transport:    api.DefaultRoundTripper,
		TenantHeader: backendType.TenantHeader(),
		TenantID:     ds.Spec.ConnectionDetails.TargetTenant,
	}
```

3f. Replace `RoundTrip` (lines 111-114):

```go
func (c *CustomRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if c.TenantHeader != "" && c.TenantID != "" {
		req.Header.Set(c.TenantHeader, c.TenantID)
	}
	return c.Transport.RoundTrip(req)
}
```

This also fixes existing behaviour: the header was previously added even when `TenantID` was empty.

3g. The `fmt` import may now be unused by the removed error path. Check and leave it if `connectDatasource` still uses `fmt.Sprintf` for its event messages — it does, on lines 104 and 107, so keep it.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/controller/openslo/... -run TestCustomRoundTripperTenantHeader -v`
Expected: PASS

Run: `make test`
Expected: PASS overall.

- [ ] **Step 5: Commit**

```bash
git add internal/controller/openslo/datasource_controller.go \
        internal/controller/openslo/datasource_controller_test.go
git commit -s -m "feat(datasource): connect Thanos and Prometheus datasources

Resolve the query URL and tenant header through internal/backend instead
of hardcoding Mimir's /prometheus sub-path and X-Scope-OrgID. Thanos and
Prometheus serve the query API at the root; Thanos uses THANOS-TENANT.

Unknown types now emit a warning and mark the Datasource not ready
instead of falling through the switch silently. The tenant header is no
longer sent when no tenant is configured. Warn that sourceTenants has no
Thanos equivalent."
```

---

### Task 4: Gate MimirRule and magic alerting on backend capability

**Files:**
- Modify: `internal/controller/openslo/slo_controller.go:211-270` (MimirRule block), `:272-319` (magic alerting block)

**Interfaces:**
- Consumes: `backend.Parse` and the `Type` methods `NeedsRemoteRulePush` / `SupportsMagicAlerting` from Task 1.
- Produces: no new exported symbols.

**On testing this task.** A test that merely re-asserted Task 1's predicates would pass before the change, pass after a *wrong* change, and give false confidence — that shape was considered and rejected. But the reconciler itself *can* be driven directly with `sigs.k8s.io/controller-runtime/pkg/client/fake`: the fake client needs only a scheme, not CRD YAML, and `github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.74.0` is already a direct dependency, so `monitoringv1` types register fine. The envtest limitation recorded at the top of this plan applies to running a *manager*, not to calling `Reconcile` directly.

So this task asserts the real thing: given a Datasource of each backend type, does the reconciler create a MimirRule or not, and does magic alerting produce an AlertManagerConfig or not.

- [ ] **Step 1: Confirm the baseline is green before changing anything**

Run: `make test`
Expected: PASS. If anything is already failing, stop and report.

- [ ] **Step 2: Write the failing test**

Create `internal/controller/openslo/slo_gating_test.go`:

```go
package controller

import (
	"context"
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
		})
	}
}
```

Notes for whoever implements this:

- `config.NewConfig()` is required because `CreatePrometheusRule` reads `config.Cfg.DefaultBaseWindow`; without it the base window is zero and rule generation misbehaves.
- Build a fresh `runtime.NewScheme()` rather than using the global `scheme.Scheme`, which `suite_test.go` already mutates.
- `WithStatusSubresource` is required in controller-runtime 0.18 for `r.Status().Update()` to work against the fake client. If a status update still errors, add the offending type to that list rather than removing the status call from production code.
- If a reconcile pass returns an unexpected error, do not paper over it by lowering the pass count — read the error and report what the reconciler actually did.

- [ ] **Step 3: Write the implementation**

3a. Add the import to `internal/controller/openslo/slo_controller.go`:

```go
	"github.com/oskoperator/osko/internal/backend"
```

3a-bis. Parse the datasource type once, immediately after the Datasource `ds` has been fetched and before the PrometheusRule block. An unparseable type is a permanent error: the SLO cannot be reconciled correctly against a backend OSKO does not understand, and failing here is what stops the gates below from degrading silently.

```go
	backendType, err := backend.Parse(ds.Spec.Type)
	if err != nil {
		log.Error(err, "unsupported datasource type", "type", ds.Spec.Type)
		if r.Recorder != nil {
			r.Recorder.Event(slo, "Warning", "UnsupportedDatasourceType", err.Error())
		}
		if statusErr := utils.UpdateStatus(ctx, slo, r.Client, "Ready", metav1.ConditionFalse, err.Error()); statusErr != nil {
			log.Error(statusErr, "Failed to update SLO status")
			return ctrl.Result{}, errors.Transient(statusErr, 5*time.Second)
		}
		return ctrl.Result{}, errors.Permanent(err)
	}
```

`utils`, `metav1`, `errors` and `time` are already imported in this file.

3b. Wrap the MimirRule block. Find line 211 `mimirRule := &oskov1alpha1.MimirRule{}` and the `log.V(1).Info("MimirRule found", ...)` line that closes the block at line 270. Wrap the whole span:

```go
	if backendType.NeedsRemoteRulePush() {
		mimirRule := &oskov1alpha1.MimirRule{}
		err = r.Get(ctx, types.NamespacedName{
			Name:      slo.Name,
			Namespace: slo.Namespace,
		}, mimirRule)

		// ... the existing body is unchanged, only indented one level ...

		log.V(1).Info("MimirRule found", "Name", mimirRule.Name, "Namespace", mimirRule.Namespace)
	}
```

Do not change any logic inside the block. The `return ctrl.Result{}, nil` statements inside it stay as they are.

3c. Gate magic alerting. Replace the opening of the block at line 273:

```go
	// Create AlertManagerConfig if magic alerting is enabled and the backend
	// exposes an Alertmanager configuration API to write it to.
	if slo.ObjectMeta.Annotations["osko.dev/magicAlerting"] == "true" {
		if !backendType.SupportsMagicAlerting() {
			log.V(1).Info("magicAlerting is not supported for this datasource type",
				"type", ds.Spec.Type)
			if r.Recorder != nil {
				r.Recorder.Event(slo, "Warning", "MagicAlertingUnsupported", fmt.Sprintf(
					"magicAlerting is not supported for %q datasources; configure Alertmanager through Thanos Ruler --alertmanagers.url",
					ds.Spec.Type))
			}
		} else {
			alertManagerConfig := &oskov1alpha1.AlertManagerConfig{}
			// ... the existing body is unchanged, only indented one level ...
		}
	}
```

The SLO is deliberately left Ready: the burn-rate alerting rules live in the PrometheusRule and do fire. Only the routing configuration is out of scope.

- [ ] **Step 4: Run tests to verify nothing regressed**

Run: `make test`
Expected: PASS. Confirm nothing else regressed, particularly `TestSLOOwnershipLogic` and `TestMagicAlertingDetection`.

- [ ] **Step 5: Commit**

```bash
git add internal/controller/openslo/slo_controller.go
git commit -s -m "feat(slo): create MimirRule and AlertManagerConfig only when supported

The SLO reconciler created a MimirRule for every SLO regardless of
datasource type, which is meaningless for Thanos: Thanos Ruler has no
rule-write API and consumes the PrometheusRule instead.

Gate both the MimirRule and the magic-alerting AlertManagerConfig on
backend capability. An SLO on Thanos with magicAlerting enabled now emits
a warning and stays Ready, because its alerting rules still fire."
```

---

### Task 5: Stamp marker labels on generated PrometheusRule objects

Without this, a `ThanosRuler` whose `ruleSelector` uses `matchLabels` selects nothing, because generated `PrometheusRule` objects inherit only whatever labels the SLO happens to carry — and a null selector matches no objects.

**Files:**
- Modify: `internal/helpers/prometheus_helper.go:677` (`CreatePrometheusRule`)
- Test: `internal/helpers/prometheus_helper_test.go` (append)

**Interfaces:**
- Consumes: the existing unexported `mergeLabels(ms ...map[string]string) map[string]string` at `prometheus_helper.go:101`. Later maps win.
- Produces: exported constants `LabelManagedBy`, `LabelManagedByValue`, `LabelSLOName`.

- [ ] **Step 1: Write the failing test**

Append to `internal/helpers/prometheus_helper_test.go`:

This file uses the standard library testing style, not testify. Match it.

```go
func TestCreatePrometheusRuleMarkerLabels(t *testing.T) {
	slo := &openslov1.SLO{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "checkout-availability",
			Namespace: "default",
			Labels: map[string]string{
				"team": "payments",
			},
		},
		Spec: openslov1.SLOSpec{
			Service:         "checkout",
			BudgetingMethod: "Occurrences",
			Objectives:      []openslov1.ObjectivesSpec{{Target: "0.99"}},
			TimeWindow:      []openslov1.TimeWindowSpec{{Duration: "28d", IsRolling: true}},
		},
	}

	sli := &openslov1.SLI{
		ObjectMeta: metav1.ObjectMeta{Name: "checkout-sli", Namespace: "default"},
		Spec: openslov1.SLISpec{
			RatioMetric: openslov1.RatioMetricSpec{
				Counter: true,
				Good: openslov1.MetricSpec{
					MetricSource: openslov1.MetricSource{
						MetricSourceRef: "thanos-ds",
						Type:            "Thanos",
						Spec:            openslov1.MetricSourceSpec{Query: "sum(rate(good_total[5m]))"},
					},
				},
				Total: openslov1.MetricSpec{
					MetricSource: openslov1.MetricSource{
						MetricSourceRef: "thanos-ds",
						Type:            "Thanos",
						Spec:            openslov1.MetricSourceSpec{Query: "sum(rate(requests_total[5m]))"},
					},
				},
			},
		},
	}

	rule, err := CreatePrometheusRule(slo, sli)
	if err != nil {
		t.Fatalf("CreatePrometheusRule() error = %v", err)
	}

	if got := rule.Labels[LabelManagedBy]; got != LabelManagedByValue {
		t.Errorf("rule.Labels[%q] = %q, want %q; ThanosRuler.ruleSelector needs a stable marker to select on",
			LabelManagedBy, got, LabelManagedByValue)
	}
	if got := rule.Labels[LabelSLOName]; got != "checkout-availability" {
		t.Errorf("rule.Labels[%q] = %q, want %q", LabelSLOName, got, "checkout-availability")
	}
	if got := rule.Labels["team"]; got != "payments" {
		t.Errorf("rule.Labels[\"team\"] = %q, want %q; labels inherited from the SLO must be preserved", got, "payments")
	}
}
```

Type notes, verified against the current API: objectives are `[]openslov1.ObjectivesSpec` (not `Objective`), time windows are `[]openslov1.TimeWindowSpec` (not `TimeWindow`), `Duration` is a named string type, `SLISpec.RatioMetric` is a value not a pointer, and `MetricSource` has fields `MetricSourceRef`, `Type` and `Spec`. The file's `init()` already calls `config.NewConfig()`, which `CreatePrometheusRule` needs for `DefaultBaseWindow`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/helpers/... -run TestCreatePrometheusRuleMarkerLabels -v`
Expected: FAIL — `LabelManagedBy` is undefined.

- [ ] **Step 3: Write minimal implementation**

3a. Add the constants near the top of `internal/helpers/prometheus_helper.go`, alongside the existing constant declarations:

```go
const (
	// LabelManagedBy marks resources generated by OSKO. A ThanosRuler
	// ruleSelector can match on it to discover generated PrometheusRule
	// objects, because a null label selector matches no objects.
	LabelManagedBy = "app.kubernetes.io/managed-by"

	// LabelManagedByValue is the value LabelManagedBy is set to.
	LabelManagedByValue = "osko"

	// LabelSLOName records which SLO a generated resource belongs to.
	LabelSLOName = "osko.dev/slo"
)
```

3b. In `CreatePrometheusRule`, replace the `objectMeta` literal:

```go
	objectMeta := metav1.ObjectMeta{
		Name:      slo.Name,
		Namespace: slo.Namespace,
		Labels: mergeLabels(slo.Labels, map[string]string{
			LabelManagedBy: LabelManagedByValue,
			LabelSLOName:   slo.Name,
		}),
		Annotations:     slo.Annotations,
		OwnerReferences: ownerRef,
	}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/helpers/... -v`
Expected: PASS

Run: `make test`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/helpers/prometheus_helper.go internal/helpers/prometheus_helper_test.go
git commit -s -m "feat(helpers): mark generated PrometheusRule objects as OSKO-managed

Generated PrometheusRule objects inherited only the SLO's own labels. A
ThanosRuler ruleSelector using matchLabels therefore selected nothing for
an unlabelled SLO, with no error reported anywhere.

Stamp app.kubernetes.io/managed-by=osko and osko.dev/slo so users have a
stable selector. MimirRule objects inherit these labels through
NewMimirRule; nothing selects on labels, so they are inert there. The
rule payload pushed to Mimir is unchanged."
```

---

### Task 6: Samples, documentation and ADR

**Files:**
- Create: `config/samples/openslo_v1_datasource_thanos.yaml`
- Create: `config/samples/openslo_v1_slo_thanos.yaml`
- Create: `adr/0008_thanos_backend_support.md`
- Modify: `README.md` (add a "Supported backends" section after "Prerequisites")
- Modify: `docs/labels-and-annotations.md`
- Modify: `config/samples/kustomization.yaml` if it enumerates sample files

**Interfaces:**
- Consumes: everything from Tasks 1-5.
- Produces: no code.

- [ ] **Step 1: Create the Thanos datasource sample**

`config/samples/openslo_v1_datasource_thanos.yaml`:

```yaml
apiVersion: openslo.com/v1
kind: Datasource
metadata:
  labels:
    app.kubernetes.io/name: thanos-ds
  name: thanos-ds
spec:
  description: Thanos Query datasource
  type: thanos
  # Thanos Query serves the Prometheus HTTP API at the root, so no
  # /prometheus sub-path is appended.
  connectionDetails:
    address: http://thanos-query.monitoring.svc:9090
    # targetTenant is sent as the THANOS-TENANT header and is only meaningful
    # when Thanos runs with --query.enforce-tenancy. Omit it otherwise.
    # targetTenant: monitoring
    #
    # sourceTenants has no Thanos equivalent and is ignored with a warning.
```

- [ ] **Step 2: Create the Thanos SLO sample**

`config/samples/openslo_v1_slo_thanos.yaml`:

```yaml
apiVersion: openslo.com/v1
kind: SLO
metadata:
  name: thanos-query-availability
  annotations:
    osko.dev/datasourceRef: "thanos-ds"
    # magicAlerting is NOT supported on thanos datasources. Setting it emits a
    # MagicAlertingUnsupported warning; the burn-rate alerting rules are still
    # generated and still fire. Route them via Thanos Ruler --alertmanagers.url.
  labels:
    # Inherited by the generated PrometheusRule alongside the OSKO markers.
    team: observability
spec:
  budgetingMethod: Occurrences
  description: 99% of Thanos Query requests should succeed
  service: thanos
  indicator:
    metadata:
      name: thanos-query-success
    spec:
      ratioMetric:
        counter: true
        good:
          metricSource:
            metricSourceRef: thanos-ds
            type: Thanos
            spec:
              query: sum(rate(http_requests_total{job="thanos-query",code!~"5.."}[5m]))
        total:
          metricSource:
            metricSourceRef: thanos-ds
            type: Thanos
            spec:
              query: sum(rate(http_requests_total{job="thanos-query"}[5m]))
  objectives:
    - target: "0.99"
  timeWindow:
    - duration: 28d
      isRolling: true
```

- [ ] **Step 3: Add the README section**

Insert after the "Prerequisites" section in `README.md`:

````markdown
## Supported backends

Set `spec.type` on a `Datasource` to one of:

| Type | Rule delivery | Tenancy header | Magic alerting |
| --- | --- | --- | --- |
| `mimir` | Pushed to the Mimir ruler API | `X-Scope-OrgID` | Supported |
| `cortex` | Pushed to the ruler API via `MimirRule` (untested against Cortex) | `X-Scope-OrgID` | Supported (untested against Cortex) |
| `thanos` | `PrometheusRule` consumed by `ThanosRuler` | `THANOS-TENANT` | Not supported |
| `prometheus` | `PrometheusRule` consumed by `Prometheus` | none | Not supported |
| `victoriametrics` | `PrometheusRule` consumed by your ruler | none | Not supported |

### Thanos

Thanos Ruler has no rule-write API, so OSKO does not push rules to it. Instead it relies on
the `PrometheusRule` it already generates, which prometheus-operator renders into files for
Thanos Ruler. Every generated `PrometheusRule` carries
`app.kubernetes.io/managed-by: osko`, so point your ruler at it:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ThanosRuler
metadata:
  name: thanos-ruler
spec:
  ruleSelector:
    matchLabels:
      app.kubernetes.io/managed-by: osko
  # Without this, rule discovery is limited to the ThanosRuler's own namespace.
  ruleNamespaceSelector: {}
  queryConfig:
    name: thanos-ruler
    key: query.yaml
```

A **null** `ruleSelector` matches no objects, so it must be set.

`osko.dev/magicAlerting` is not supported on Thanos. The burn-rate alerting rules are still
generated and still fire; route them by configuring Thanos Ruler's `--alertmanagers.url`.
````

- [ ] **Step 4: Update docs/labels-and-annotations.md**

Add to the annotations section:

```markdown
### `osko.dev/magicAlerting`

Supported only on `mimir` and `cortex` datasources — the two backends that expose an
Alertmanager configuration API. On `thanos`, `prometheus` and `victoriametrics` it emits a
`MagicAlertingUnsupported` warning event and the SLO remains Ready, because the burn-rate
alerting rules are generated regardless. Only the Alertmanager routing configuration is
skipped.
```

Add a labels section:

```markdown
## Labels applied by OSKO

Generated `PrometheusRule` objects, and the `MimirRule` objects derived from them, carry:

| Label | Value | Purpose |
| --- | --- | --- |
| `app.kubernetes.io/managed-by` | `osko` | Stable selector for `ThanosRuler.ruleSelector` and `Prometheus.ruleSelector` |
| `osko.dev/slo` | The SLO's name | Traceability back to the owning SLO |

Labels on the SLO are inherited by the generated `PrometheusRule`. The two labels above are
applied on top and win on conflict.
```

- [ ] **Step 5: Write the ADR**

Create `adr/0008_thanos_backend_support.md` with exactly this content:

````markdown
# Thanos Backend Support

* Status: accepted
* Date: 2026-09-10

## Context and Problem Statement

OSKO supported Grafana Mimir and carried a stub for Cortex. A `Datasource` with
`type: thanos` was silently ignored by the Datasource controller, and the SLO controller
created a `MimirRule` for every SLO regardless of backend, attempting to push rule groups to
a Mimir ruler API that does not exist in a Thanos stack.

The decisive constraint is that **Thanos Ruler has no rule-write API**. Mimir and Cortex
expose a ruler configuration API (`PUT /api/v1/rules/{namespace}`, tenant via
`X-Scope-OrgID`). Thanos Ruler reads rules from files:

> `--rule-file=rules/ ...` Rule files that should be used by rule manager. Can be in glob
> format (repeated). Note that rules are not automatically detected, use SIGHUP or do HTTP
> POST `/-/reload` to re-read them.

There is nothing to push to. However, prometheus-operator's `ThanosRuler` CRD discovers
`PrometheusRule` objects through `ruleSelector` and renders them into files for the Ruler.
OSKO already creates a `PrometheusRule` for every SLO.

## Considered Options

* **Option A**: Deliver rules as `PrometheusRule` objects only, consumed by `ThanosRuler`
* **Option B**: Add a `ThanosRule` CRD that renders rule files into ConfigMaps and calls `/-/reload`
* **Option C**: Render ConfigMaps for all backends, ignoring the generated `PrometheusRule`

## Decision Outcome

Chosen option: **Option A**.

OSKO already produces exactly the artifact `ThanosRuler` consumes. Support is therefore
mostly a matter of *not* doing the Mimir-specific work rather than doing new Thanos-specific
work: no new CRD, no new controller, no new HTTP client.

### Supporting decisions

* Backend capabilities live in `internal/backend`. Controllers ask `NeedsRemoteRulePush` and
  `SupportsMagicAlerting` rather than comparing type strings, so the next backend is a
  one-file change.
* `Datasource.spec.type` is validated by a CRD enum of
  `prometheus;mimir;cortex;thanos;victoriametrics`.
* Backend capabilities are methods on a `backend.Type` obtained only through `backend.Parse`,
  so a caller cannot consult a capability without first handling the unknown-backend error.
  Raw-string predicates were rejected because they force each call site to swallow a parse
  error and return a zero value, turning a typo into a silently skipped MimirRule.
* `targetTenant` maps to the `THANOS-TENANT` header for Thanos, matching the default of
  Thanos' `--query.tenant-header`. `sourceTenants` has no Thanos equivalent and is ignored
  with a warning.
* Thanos and Prometheus serve the Prometheus HTTP API at the root; the `/prometheus`
  sub-path is Mimir- and Cortex-specific.
* Generated `PrometheusRule` objects carry `app.kubernetes.io/managed-by: osko` and
  `osko.dev/slo`, so a `ThanosRuler.ruleSelector` has a stable target. Without this, an SLO
  with no labels produces a `PrometheusRule` with no labels, which a `matchLabels` selector
  never selects, and nothing reports an error.

### Positive Consequences

* Thanos users get SLO rule generation with no new moving parts.
* Backend branching is centralised instead of scattered across controllers.
* A misspelled datasource type now fails at `kubectl apply` rather than silently producing
  no rules.
* Datasource connection failures are visible in the resource, because `DatasourceStatus`
  gained the `Conditions` and `Ready` fields `utils.UpdateStatus` looks for by reflection.

### Negative Consequences

* `victoriametrics` is accepted by the enum and handled on the same path as Thanos and
  Prometheus. It was already named in `internal/helpers.isPrometheusSource` for the SLI
  metric-source dialect, so excluding it from the Datasource enum would have hard-rejected a
  value the codebase already acknowledged.
* `osko.dev/magicAlerting` is unsupported on Thanos. Thanos Ruler sends alerts to an
  Alertmanager configured through `--alertmanagers.url`, outside OSKO's control. The SLO
  stays Ready because its burn-rate alerting rules still fire; only routing is skipped.
* The `spec.type` enum is a breaking change: an existing `Datasource` with an out-of-enum
  value cannot be updated until corrected.
* Thanos Ruler deployed without prometheus-operator is not supported.
* Generated `PrometheusRule` and the `MimirRule` objects derived from them gain two labels,
  causing one update per object at upgrade time. The rule payload pushed to Mimir is
  unchanged.

### Testing approach, and the gap that remains

Three layers cover this change:

* **Pure unit tests** for the capability decisions in `internal/backend` (100% statement
  coverage), and for the generated `PrometheusRule` labels in `internal/helpers`.
* **Reconciler tests** in `internal/controller/openslo/slo_gating_test.go`, which call
  `Reconcile` directly against `sigs.k8s.io/controller-runtime/pkg/client/fake` and assert
  which owned resources each of the five backends produces. `monitoringv1` and
  `oskov1alpha1` register into a scheme as Go types, so no prometheus-operator CRD YAML is
  needed — only a *manager* would require that. `internal/controller/openslo` coverage went
  from 1.0% to 33.6% as a result.
* **API-server tests** under envtest for the `spec.type` enum and for the shipped sample
  manifests, using the real generated CRDs from `config/crd/bases`.

The gap that remains is manager-level: no suite calls `mgr.Start`, so the watch
configuration, owner-reference garbage collection and requeue behaviour declared in
`SetupWithManager` are exercised by no test. Closing it means starting a manager and adding
prometheus-operator CRDs to `CRDDirectoryPaths` so a real `PrometheusRule` can be created
through the API server. That is standalone work which would complete Layer 3 of ADR 0005.

## Pros and Cons of the Options

### Option A: PrometheusRule only

* Good, because OSKO already generates the artifact
* Good, because no new CRD, controller, RBAC or client
* Good, because it matches how the prometheus-operator ecosystem expects rules to flow
* Bad, because it requires prometheus-operator and a correctly configured `ThanosRuler`

### Option B: ThanosRule CRD rendering ConfigMaps

* Good, because it supports Thanos Ruler without prometheus-operator
* Bad, because it adds a CRD, controller, RBAC set and finalizer logic
* Bad, because it duplicates rule content already present in the `PrometheusRule`

### Option C: ConfigMap rendering for all backends

* Good, because it is a single consistent path
* Bad, because it ignores the generated `PrometheusRule`
* Bad, because it forces prometheus-operator users onto a worse path

## Links

* [Thanos Ruler documentation](https://thanos.io/tip/components/rule.md/)
* [prometheus-operator ThanosRuler API](https://github.com/prometheus-operator/prometheus-operator/blob/main/pkg/apis/monitoring/v1/thanos_types.go)
* [Design spec](../docs/superpowers/specs/2026-09-10-thanos-support-design.md)
* [ADR 0005: Test Coverage Strategy](0005_test_coverage_strategy.md)
````

- [ ] **Step 6: Verify the samples apply against a real API server**

Do **not** run `make install` against a live cluster. Validate the sample files under
envtest instead, which loads the same generated CRDs from `config/crd/bases`.

The samples are the copy-paste starting point for anyone adopting Thanos, so a sample the
API server rejects is a broken onboarding path. Nothing currently reads these files in a
test, which means a typo in a field name ships silently.

Create `internal/controller/openslo/samples_test.go`:

```go
package controller

import (
	"context"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	openslov1 "github.com/oskoperator/osko/api/openslo/v1"
)

// The Thanos samples are what users copy to adopt the backend. Decoding them
// strictly catches a misspelled field, and applying them against the real
// generated CRDs catches an out-of-enum value. Strictness is load-bearing:
// a non-strict decoder drops an unknown field client-side, so the API server
// never sees it and the spec would stay green while `kubectl apply` — which
// has validated fields strictly since 1.25 — rejects the sample.
var _ = Describe("Thanos sample manifests", func() {
	decodeSample := func(name string, obj client.Object) {
		data, err := os.ReadFile(filepath.Join("..", "..", "..", "config", "samples", name))
		Expect(err).NotTo(HaveOccurred(), "sample file must exist")

		Expect(yaml.UnmarshalStrict(data, obj)).To(Succeed(),
			"sample must decode into its typed object with no unknown fields")
		obj.SetNamespace("default")
	}

	It("applies the Thanos datasource sample", func() {
		ds := &openslov1.Datasource{}
		decodeSample("openslo_v1_datasource_thanos.yaml", ds)

		Expect(ds.Spec.Type).To(Equal("thanos"),
			"the sample must actually exercise the thanos path")

		Expect(k8sClient.Create(context.Background(), ds)).To(Succeed())
		Expect(k8sClient.Delete(context.Background(), ds)).To(Succeed())
	})

	It("applies the Thanos SLO sample", func() {
		slo := &openslov1.SLO{}
		decodeSample("openslo_v1_slo_thanos.yaml", slo)

		Expect(slo.ObjectMeta.Annotations).To(HaveKeyWithValue("osko.dev/datasourceRef", "thanos-ds"),
			"the SLO sample must point at the datasource sample")

		Expect(k8sClient.Create(context.Background(), slo)).To(Succeed())
		Expect(k8sClient.Delete(context.Background(), slo)).To(Succeed())
	})
})
```

Use `k8s.io/apimachinery/pkg/util/yaml`, not `gopkg.in/yaml.v3`. Kubernetes types carry
`json:` tags, and `yaml.v3` ignores those, so it would silently produce a zero-valued
object. `apimachinery` is already a direct dependency, so `go.mod` must not change.

Run: `make test`
Expected: both new specs pass, joining the existing 6 in the `TestControllers` suite.

To prove the specs have teeth, temporarily change `type: thanos` to `type: thanso` in the
datasource sample and re-run — the create must be rejected by the enum. Revert afterwards
and confirm `git diff` on `config/samples/` is empty.

- [ ] **Step 7: Commit**

```bash
git add config/samples/openslo_v1_datasource_thanos.yaml \
        config/samples/openslo_v1_slo_thanos.yaml \
        internal/controller/openslo/samples_test.go \
        adr/0008_thanos_backend_support.md \
        README.md docs/labels-and-annotations.md
git commit -s -m "docs(thanos): document Thanos backend support

Add Thanos datasource and SLO samples, a supported-backends table with
the ThanosRuler wiring, the magicAlerting caveat, and ADR 0008 recording
the decision and the remaining manager-level test gap.

Cover both samples with envtest specs so a misspelled field or an
out-of-enum type in the copy-paste starting point fails the suite."
```

If `config/samples/kustomization.yaml` enumerates its resources, add the two new files to it
and include it in the `git add` above.

---

## Final verification

- [ ] Run the full suite: `make test`
- [ ] Confirm generated manifests are current: `make manifests generate` produces no diff
- [ ] Confirm the Helm subchart was not committed: `git status --short helm/` shows no staged CRDs
- [ ] Review the whole diff: `git diff main...HEAD`

## Post-review fix wave

Tasks 1-6 are complete. The final whole-branch review over `44d688d..3b4ee04` returned
"with fixes". These are the rulings, to be delivered as **one** fix wave. Where a fix
contradicts an executed task's step text, this section governs.

**F1 (was Critical).** Add `+kubebuilder:default=mimir` to `DatasourceSpec.Type` in
`api/openslo/v1/datasource_types.go`, alongside the existing enum marker. Regenerate.

The enum does not validate an absent field, so a `Datasource` with no `spec.type` is
admissible and yields `Type == ""`. `backend.Parse("")` errors, and the SLO reconciler parses
at `slo_controller.go:115` — *above* `PrometheusRule` creation — so such an SLO currently
produces nothing at all and reports `Ready=False`. That configuration is legal and working in
the released version. Defaulting restores it with no user action. See spec D5.

**F2 (was Important).** In `slo_controller.go`, delete owned resources the backend can no
longer justify: when `!backendType.NeedsRemoteRulePush()`, delete any `MimirRule` owned by
this SLO; when magic alerting is requested but `!backendType.SupportsMagicAlerting()`, delete
any `AlertManagerConfig` owned by this SLO. Normal event on each deletion; treat `IsNotFound`
as success, since most SLOs never had one. See spec D7.

**F3 (was Important).** The `MagicAlertingUnsupported` event message hardcodes Thanos Ruler
advice but fires for `prometheus` and `victoriametrics` too. Make the remedy backend-specific
or drop it and point at `docs/labels-and-annotations.md`, which was already corrected to name
all three backends.

**F4 (was Important).** Add a `default:` arm to the backend switch in
`datasource_controller.go`. Today a `backend.Type` not listed falls through to the shared
success write and reports `Ready=True` without ever connecting.

**F5 (was Important, test gap).** Add gating-test rows for the empty-type case — the hole F1
shipped through — and for the F2 transition.

**F6 (was Minor, promoted by triage).** `README.md`'s `ThanosRuler` example references a
`queryConfig` Secret it never defines, so the ruler will not start if copy-pasted. This is the
canonical wiring block for the whole feature.

**Deferred by ruling:** replacing `internal/helpers.isPrometheusSource`'s duplicate backend
switch with `backend.Parse`. It reads a different field on a different CRD and is untouched by
this branch; folding an unrelated refactor into a fix wave carrying an upgrade-critical change
would make the diff harder to review and revert. Follow-up issue.

## Out of scope

Recorded in the spec and ADR 0008 as follow-ups:

- Wiring envtest with a manager and prometheus-operator CRDs to enable reconciler integration tests.
- A `ThanosRule` CRD rendering rule files into ConfigMaps for Thanos Ruler deployed without prometheus-operator.
- Translating magic alerting into `monitoring.coreos.com/v1alpha1 AlertmanagerConfig`.
- Completing the Cortex implementation.
- Removing the dead `ConnectionDetails.SyncPrometheusRules` field.
