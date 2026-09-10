# Thanos Backend Support Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let OSKO drive Thanos as a metrics backend by delivering SLO rules as `PrometheusRule` objects and skipping the Mimir-specific push paths.

**Architecture:** A new `internal/backend` package owns per-backend capability knowledge. The Datasource controller learns to connect to Thanos and Prometheus; the SLO controller gates `MimirRule` and `AlertManagerConfig` creation on backend capability instead of creating them unconditionally. Generated `PrometheusRule` objects gain a stable marker label so a `ThanosRuler.ruleSelector` can find them.

**Tech Stack:** Go 1.x, controller-runtime, kubebuilder, testify, Ginkgo/Gomega + envtest, prometheus-operator API types.

**Spec:** `docs/superpowers/specs/2026-09-10-thanos-support-design.md`

## Global Constraints

- Backend type values are exactly `prometheus`, `mimir`, `cortex`, `thanos` (lowercase).
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

**This plan therefore:**

- Puts all decision logic in pure functions in `internal/backend`, so every gating decision is unit-testable without a cluster (Task 1).
- Uses envtest only for CRD schema validation, which **is** reachable with existing infrastructure (Task 2).
- Records the reconciler-level integration test gap as a follow-up in the ADR (Task 6).

---

### Task 1: The `internal/backend` capability package

Pure functions, no Kubernetes dependencies. Everything downstream consumes this.

**Files:**
- Create: `internal/backend/backend.go`
- Test: `internal/backend/backend_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `type Type string` with constants `Prometheus`, `Mimir`, `Cortex`, `Thanos` (values `"prometheus"`, `"mimir"`, `"cortex"`, `"thanos"`)
  - `const TenantHeaderMimir = "X-Scope-OrgID"`
  - `const TenantHeaderThanos = "THANOS-TENANT"`
  - `func Parse(t string) (Type, error)`
  - `func NeedsRemoteRulePush(t string) bool`
  - `func SupportsMagicAlerting(t string) bool`
  - `func QueryURL(t, address string) (string, error)`
  - `func TenantHeader(t string) string`

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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/backend/... -v`
Expected: FAIL — the package `internal/backend` does not exist yet, so the build fails with "no Go files" or "undefined: Parse".

- [ ] **Step 3: Write minimal implementation**

Create `internal/backend/backend.go`:

```go
// Package backend describes the metrics backends OSKO can target and the
// capabilities each one provides. Controllers ask this package what a backend
// can do rather than comparing type strings, so adding a new backend means
// changing one file.
package backend

import (
	"fmt"
	"strings"
)

// Type identifies a metrics backend. The values match the enum accepted by
// Datasource.spec.type.
type Type string

const (
	Prometheus Type = "prometheus"
	Mimir      Type = "mimir"
	Cortex     Type = "cortex"
	Thanos     Type = "thanos"
)

const (
	// TenantHeaderMimir scopes a request to a tenant in Mimir and Cortex.
	TenantHeaderMimir = "X-Scope-OrgID"

	// TenantHeaderThanos scopes a request to a tenant in Thanos Query. It
	// matches the default of Thanos' --query.tenant-header flag.
	TenantHeaderThanos = "THANOS-TENANT"

	// prometheusAPISubPath is where Mimir and Cortex expose the Prometheus
	// HTTP API. Thanos and Prometheus expose it at the root.
	prometheusAPISubPath = "/prometheus"
)

// Parse normalises a Datasource type string.
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
	default:
		return "", fmt.Errorf("unsupported datasource type: %q", t)
	}
}

// NeedsRemoteRulePush reports whether rule groups must be pushed to a remote
// ruler configuration API.
//
// Mimir and Cortex expose one. Thanos Ruler has no rule-write API and instead
// reads files rendered by prometheus-operator from PrometheusRule objects;
// Prometheus works the same way. Neither needs a push.
func NeedsRemoteRulePush(t string) bool {
	parsed, err := Parse(t)
	if err != nil {
		return false
	}
	return parsed == Mimir || parsed == Cortex
}

// SupportsMagicAlerting reports whether the backend exposes an Alertmanager
// configuration API that OSKO can write routing configuration to.
//
// Thanos Ruler sends alerts to an Alertmanager configured statically through
// --alertmanagers.url, which is outside OSKO's control.
func SupportsMagicAlerting(t string) bool {
	parsed, err := Parse(t)
	if err != nil {
		return false
	}
	return parsed == Mimir || parsed == Cortex
}

// QueryURL returns the base URL of the Prometheus-compatible query API for the
// backend reachable at address.
func QueryURL(t, address string) (string, error) {
	parsed, err := Parse(t)
	if err != nil {
		return "", err
	}

	trimmed := strings.TrimSuffix(address, "/")
	if parsed == Mimir || parsed == Cortex {
		return trimmed + prometheusAPISubPath, nil
	}
	return trimmed, nil
}

// TenantHeader returns the HTTP header carrying the tenant identifier for the
// backend, or an empty string when the backend has no tenancy header.
func TenantHeader(t string) string {
	parsed, err := Parse(t)
	if err != nil {
		return ""
	}

	switch parsed {
	case Mimir, Cortex:
		return TenantHeaderMimir
	case Thanos:
		return TenantHeaderThanos
	default:
		return ""
	}
}
```

Note: `QueryURL` trims a trailing slash before appending. The current code does not, so a Datasource with `address: http://localhost:9009/` (as in `config/samples/openslo_v1_datasource.yaml`) produces `http://localhost:9009//prometheus`. This is a drive-by fix in code being modified anyway.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/backend/... -v`
Expected: PASS, all subtests green.

- [ ] **Step 5: Commit**

```bash
git add internal/backend/backend.go internal/backend/backend_test.go
git commit -s -m "feat(backend): add metrics backend capability package

Introduce typed backend constants and capability predicates so controllers
ask what a backend can do instead of comparing type strings. Adding a
backend now means changing one file."
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
- Produces: `DatasourceStatus{Conditions []metav1.Condition, Ready string}`, and a `spec.type` enum restricted to `prometheus;mimir;cortex;thanos`.

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
	// +kubebuilder:validation:Enum=prometheus;mimir;cortex;thanos
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

Restrict spec.type to prometheus, mimir, cortex and thanos so a typo fails
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
- Consumes: `backend.Parse`, `backend.QueryURL`, `backend.TenantHeader`, `backend.Thanos`, `backend.Mimir`, `backend.Cortex`, `backend.Prometheus` from Task 1. `utils.UpdateStatus` and the status fields from Task 2.
- Produces: `CustomRoundTripper{Transport http.RoundTripper, TenantHeader string, TenantID string}`.

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
	case backend.Mimir, backend.Thanos, backend.Prometheus:
		log.Info("Connecting Datasource", "type", string(backendType), "address", ds.Spec.ConnectionDetails.Address)
		if err := r.connectDatasource(ctx, ds); err != nil {
			log.Error(err, errConnectDS)
			if statusErr := utils.UpdateStatus(ctx, ds, r.Client, "Ready", metav1.ConditionFalse, errConnectDS); statusErr != nil {
				log.Error(statusErr, "Failed to update Datasource status")
			}
			return ctrl.Result{}, errors.Transient(err, 5*time.Second)
		}
	case backend.Cortex:
		log.Info("Datasource Type is Cortex", "address", ds.Spec.ConnectionDetails.Address)
		r.Recorder.Event(ds, "Warning", "NotImplemented", "Cortex support is not implemented yet")
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

3d. Replace the address-resolution block at the top of `connectDatasource` (lines 79-85) with:

```go
	datasourceAddress, err := backend.QueryURL(ds.Spec.Type, ds.Spec.ConnectionDetails.Address)
	if err != nil {
		return err
	}
```

3e. Update the round tripper construction in `connectDatasource` (lines 87-90):

```go
	customRoundtripper := &CustomRoundTripper{
		Transport:    api.DefaultRoundTripper,
		TenantHeader: backend.TenantHeader(ds.Spec.Type),
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
- Test: `internal/controller/openslo/slo_controller_test.go` (append)

**Interfaces:**
- Consumes: `backend.NeedsRemoteRulePush`, `backend.SupportsMagicAlerting` from Task 1.
- Produces: no new exported symbols.

- [ ] **Step 1: Write the failing test**

Append to `internal/controller/openslo/slo_controller_test.go`:

```go
// TestBackendGating documents which owned resources each datasource type
// should produce. The reconciler cannot be exercised without envtest wiring
// that does not exist yet, so this asserts the decision predicates the
// reconciler branches on.
func TestBackendGating(t *testing.T) {
	tests := []struct {
		name                string
		datasourceType      string
		wantMimirRule       bool
		wantMagicAlerting   bool
	}{
		{
			name:              "mimir gets a MimirRule and magic alerting",
			datasourceType:    "mimir",
			wantMimirRule:     true,
			wantMagicAlerting: true,
		},
		{
			name:              "cortex gets a MimirRule and magic alerting",
			datasourceType:    "cortex",
			wantMimirRule:     true,
			wantMagicAlerting: true,
		},
		{
			name:              "thanos gets neither",
			datasourceType:    "thanos",
			wantMimirRule:     false,
			wantMagicAlerting: false,
		},
		{
			name:              "prometheus gets neither",
			datasourceType:    "prometheus",
			wantMimirRule:     false,
			wantMagicAlerting: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantMimirRule, backend.NeedsRemoteRulePush(tt.datasourceType))
			assert.Equal(t, tt.wantMagicAlerting, backend.SupportsMagicAlerting(tt.datasourceType))
		})
	}
}
```

Add the import `"github.com/oskoperator/osko/internal/backend"` to that file.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/controller/openslo/... -run TestBackendGating -v`
Expected: FAIL to compile until the import is added; then PASS immediately, because it tests Task 1's predicates. Its purpose is to pin the intended reconciler behaviour next to the reconciler. Proceed to Step 3 regardless.

- [ ] **Step 3: Write minimal implementation**

3a. Add the import to `internal/controller/openslo/slo_controller.go`:

```go
	"github.com/oskoperator/osko/internal/backend"
```

3b. Wrap the MimirRule block. Find line 211 `mimirRule := &oskov1alpha1.MimirRule{}` and the `log.V(1).Info("MimirRule found", ...)` line that closes the block at line 270. Wrap the whole span:

```go
	if backend.NeedsRemoteRulePush(ds.Spec.Type) {
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
		if !backend.SupportsMagicAlerting(ds.Spec.Type) {
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

- [ ] **Step 4: Run tests to verify they pass**

Run: `make test`
Expected: PASS. Confirm nothing else regressed, particularly `TestSLOOwnershipLogic` and `TestMagicAlertingDetection`.

- [ ] **Step 5: Commit**

```bash
git add internal/controller/openslo/slo_controller.go \
        internal/controller/openslo/slo_controller_test.go
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
| `cortex` | Not implemented yet | `X-Scope-OrgID` | Not implemented yet |
| `thanos` | `PrometheusRule` consumed by `ThanosRuler` | `THANOS-TENANT` | Not supported |
| `prometheus` | `PrometheusRule` consumed by `Prometheus` | none | Not supported |

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

Not supported on `thanos` or `prometheus` datasources. Setting it there emits a
`MagicAlertingUnsupported` warning event and the SLO remains Ready, because the
burn-rate alerting rules are generated regardless. Only the Alertmanager routing
configuration is skipped.
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
* `Datasource.spec.type` is validated by a CRD enum of `prometheus;mimir;cortex;thanos`.
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

* `osko.dev/magicAlerting` is unsupported on Thanos. Thanos Ruler sends alerts to an
  Alertmanager configured through `--alertmanagers.url`, outside OSKO's control. The SLO
  stays Ready because its burn-rate alerting rules still fire; only routing is skipped.
* The `spec.type` enum is a breaking change: an existing `Datasource` with an out-of-enum
  value cannot be updated until corrected.
* Thanos Ruler deployed without prometheus-operator is not supported.
* Generated `PrometheusRule` and the `MimirRule` objects derived from them gain two labels,
  causing one update per object at upgrade time. The rule payload pushed to Mimir is
  unchanged.

### Known gap: reconciler integration tests

Reconciler-level integration tests are not possible in this repository yet. No `suite_test.go`
starts a manager or registers a reconciler, `CRDDirectoryPaths` covers only
`config/crd/bases` so prometheus-operator CRDs are absent, and `monitoringv1` is not in the
test scheme.

Backend decisions are therefore unit-tested as pure functions in `internal/backend`, and
envtest is used only for CRD schema validation, which existing infrastructure supports.
Closing this gap means wiring a manager, vendoring prometheus-operator CRDs and registering
`monitoringv1` — standalone work that would complete Layer 3 of ADR 0005.

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

- [ ] **Step 6: Verify the samples apply**

Run:

```bash
make install
kubectl apply -f config/samples/openslo_v1_datasource_thanos.yaml
kubectl get datasource thanos-ds -o jsonpath='{.spec.type}{"\n"}'
```

Expected: `thanos`

Then confirm the enum rejects a typo:

```bash
kubectl apply --dry-run=server -f - <<'EOF'
apiVersion: openslo.com/v1
kind: Datasource
metadata:
  name: bad-type
spec:
  type: thanso
  connectionDetails:
    address: http://example:9090
EOF
```

Expected: rejected with a message naming `spec.type` and listing the supported values.

Clean up: `kubectl delete -f config/samples/openslo_v1_datasource_thanos.yaml`

- [ ] **Step 7: Commit**

```bash
git add config/samples/openslo_v1_datasource_thanos.yaml \
        config/samples/openslo_v1_slo_thanos.yaml \
        adr/0008_thanos_backend_support.md \
        README.md docs/labels-and-annotations.md
git commit -s -m "docs(thanos): document Thanos backend support

Add Thanos datasource and SLO samples, a supported-backends table with
the ThanosRuler wiring, the magicAlerting caveat, and ADR 0008 recording
the decision and the integration-test gap."
```

---

## Final verification

- [ ] Run the full suite: `make test`
- [ ] Confirm generated manifests are current: `make manifests generate` produces no diff
- [ ] Confirm the Helm subchart was not committed: `git status --short helm/` shows no staged CRDs
- [ ] Review the whole diff: `git diff main...HEAD`

## Out of scope

Recorded in the spec and ADR 0008 as follow-ups:

- Wiring envtest with a manager and prometheus-operator CRDs to enable reconciler integration tests.
- A `ThanosRule` CRD rendering rule files into ConfigMaps for Thanos Ruler deployed without prometheus-operator.
- Translating magic alerting into `monitoring.coreos.com/v1alpha1 AlertmanagerConfig`.
- Completing the Cortex implementation.
- Removing the dead `ConnectionDetails.SyncPrometheusRules` field.
