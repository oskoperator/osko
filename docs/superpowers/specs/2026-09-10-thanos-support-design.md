# Thanos backend support

- **Date:** 2026-09-10
- **Status:** Approved, not yet implemented
- **Affects:** `openslo.com/v1` Datasource, SLO controller, Datasource controller, rule generation helpers

## Problem

OSKO supports Grafana Mimir as a metrics backend and has a stub for Cortex. Users running
Thanos cannot use OSKO: a `Datasource` with `type: thanos` is silently ignored by the
Datasource controller, and the SLO controller unconditionally creates a `MimirRule` that
tries to push rule groups to a Mimir ruler API that does not exist in a Thanos stack.

We want first-class Thanos support with the smallest correct change.

## Background: how Thanos differs from Mimir

### Thanos Ruler has no rule-write API

This is the single fact that shapes the whole design. Mimir and Cortex expose a ruler
configuration API (`PUT /api/v1/rules/{namespace}`, tenant via `X-Scope-OrgID`) that OSKO
pushes to. Thanos Ruler has no equivalent. From the Thanos Ruler flag documentation:

> `--rule-file=rules/ ...` Rule files that should be used by rule manager. Can be in glob
> format (repeated). Note that rules are not automatically detected, use SIGHUP or do HTTP
> POST `/-/reload` to re-read them.

Thanos Ruler reads rules from files on disk. There is nothing to push to.

### prometheus-operator bridges the gap

The `ThanosRuler` CRD discovers rules from `PrometheusRule` objects. From
`prometheus-operator/pkg/apis/monitoring/v1/thanos_types.go` lines 287-297:

```go
// ruleSelector defines the PrometheusRule objects to be selected for rule evaluation. An empty
// label selector matches all objects. A null label selector matches no
// objects.
RuleSelector *metav1.LabelSelector `json:"ruleSelector,omitempty"`
// ruleNamespaceSelector defines the namespaces to be selected for Rules discovery. If unspecified, only
// the same namespace as the ThanosRuler object is in is used.
RuleNamespaceSelector *metav1.LabelSelector `json:"ruleNamespaceSelector,omitempty"`
```

The operator renders matching `PrometheusRule` objects into files and reloads the Ruler.

OSKO already creates a `PrometheusRule` for every SLO. That artifact is exactly what Thanos
needs. Thanos support is therefore mostly a matter of **not doing the Mimir-specific things**,
rather than doing new Thanos-specific things.

### Tenancy differs

Thanos Query's tenant header defaults to `THANOS-TENANT`, not `X-Scope-OrgID`
(`cmd/thanos/query.go:175`, flag `--query.tenant-header`). Tenancy is opt-in via
`--query.enforce-tenancy` and is enforced as a PromQL label matcher. Thanos has no
equivalent of Mimir's rule-group `source_tenants` field for federated rules.

### Query path differs

Thanos Query serves the Prometheus HTTP API at the root. Mimir serves it under `/prometheus`.

### Alertmanager differs

Mimir has a built-in Alertmanager with a configuration API. Thanos Ruler pushes alerts to an
external Alertmanager configured statically at deploy time via `--alertmanagers.url`. There is
no API for OSKO to write routing configuration to.

## Current state of the code

Verified against the repository at the time of writing.

| Location | Behaviour |
| --- | --- |
| `internal/helpers/prometheus_helper.go:298` | `isPrometheusSource()` lowercases its input and already accepts `thanos`. The PromQL dialect is already supported. |
| `internal/controller/openslo/datasource_controller.go:59` | `switch ds.Spec.Type`: `mimir` connects, `cortex` emits a NotImplemented warning, anything else falls through silently. |
| `internal/controller/openslo/datasource_controller.go:81` | `connectDatasource` rejects any type other than `mimir` and hardcodes `address + "/prometheus"`. |
| `internal/controller/openslo/slo_controller.go:211` | Creates a `MimirRule` unconditionally, with no datasource-type check. |
| `internal/controller/openslo/slo_controller.go:273` | Creates an `AlertManagerConfig` when `osko.dev/magicAlerting == "true"`. |
| `internal/controller/osko/alertmanagerconfig_controller.go:177` | Pushes that config via `MimirClient.CreateAlertmanagerConfig`. |
| `internal/helpers/prometheus_helper.go:677` | `CreatePrometheusRule` sets the object's `Labels` to `slo.Labels` verbatim. No OSKO-owned marker label. |
| `api/openslo/v1/datasource_types.go` | `Spec.Type` is a plain `string` with no kubebuilder enum and no CRD validation. |
| `api/osko/v1alpha1/connection_details.go:8` | `SyncPrometheusRules` is declared but read nowhere in the codebase. Dead field. |

The tail of the SLO reconcile (`slo_controller.go:321`) sets `Ready=True` with reason
`"PrometheusRule created"` and returns. Skipping the MimirRule block therefore falls through
cleanly with no dangling state.

## Decisions

### D1. Rules reach Thanos as PrometheusRule objects only

OSKO stops at the `PrometheusRule` it already creates. The user's `ThanosRuler` picks it up.

No `ThanosRule` CRD, no ConfigMap rendering, no `/-/reload` client.

*Rejected:* a `ThanosRule` CRD that renders rule files into a ConfigMap, which would support
Thanos Ruler deployed without prometheus-operator. Costs a new CRD, controller, RBAC set and
finalizer logic for a deployment style that is not the common case. Revisit if users ask.

### D2. Magic alerting is refused on Thanos, loudly

When an SLO sets `osko.dev/magicAlerting: "true"` and its datasource is `thanos`, OSKO skips
`AlertManagerConfig` creation, emits a Warning event, and records the reason in the SLO status.

The SLO **remains Ready**. The burn-rate alerting rules live in the `PrometheusRule` and do
fire; only the Alertmanager *routing* configuration is out of scope. Marking the SLO
not-Ready would misrepresent a working SLO as broken.

*Rejected:* silently skipping, which leaves the user with an annotation that does nothing and
no explanation. Also rejected for now: translating magic alerting into a
`monitoring.coreos.com/v1alpha1 AlertmanagerConfig`. That is the true Thanos analog but is a
materially larger feature (new CRD dependency, rendering logic, RBAC, tests) and is recorded
as a follow-up.

### D3. `targetTenant` maps to `THANOS-TENANT`; `sourceTenants` warns

On the Datasource connection check, if `targetTenant` is set it is sent as the
`THANOS-TENANT` header. If unset, no tenant header is sent. This is correct both for Thanos
deployments with `--query.enforce-tenancy` and for the common case where tenancy is off.

`sourceTenants` has no Thanos equivalent. If set on a Thanos datasource, OSKO emits a Warning
event stating that it is ignored. Non-fatal.

*Rejected:* adding a configurable `tenantHeader` field to `ConnectionDetails`. Thanos already
makes the header name configurable on its own side, and a new CRD field for this is API
surface we would have to keep forever.

`TenantHeader` is consumed only by the Datasource connection check. The Mimir rule push keeps
using the mimirtool client's own tenant handling, which sets `X-Scope-OrgID` internally from
the configured tenant ID; that path is not modified.

### D3a. `prometheus` is handled on the same path as `thanos`

The enum in D5 admits `prometheus`, so the controller must handle it rather than fall through
to the unsupported-type branch. From OSKO's point of view plain Prometheus behaves exactly as
Thanos does: query API at the root, no tenancy header, no remote rule push, no Alertmanager
configuration API. It therefore shares the `thanos` branch, and rule delivery is likewise the
`PrometheusRule` that prometheus-operator already consumes via `Prometheus.ruleSelector`.

Supporting it costs one extra `case` label and keeps the enum honest.

### D4. Backend types become typed constants with capability methods

A new `internal/backend` package owns the knowledge of what each backend can do. Call sites
express intent (`NeedsRemoteRulePush`) rather than identity (`type == "mimir"`).

The capabilities are **methods on a `Type` obtained only through `Parse`**, not functions
taking a raw `string`. Raw-string predicates were reviewed and rejected during
implementation: each would have to swallow the parse error and return a zero value, so a
typo like `mimr` would make `NeedsRemoteRulePush` return `false` and silently skip the
MimirRule, producing a green reconcile with no rules. Requiring a parsed `Type` makes that
state unrepresentable rather than merely discouraged.

*Rejected:* adding `thanos` cases to the existing switches inline. Adding Thanos introduces
two new branch sites on top of the two that exist; four scattered string comparisons across
three files is what the next backend turns into five.

### D5. `Datasource.Spec.Type` gets a CRD enum

`+kubebuilder:validation:Enum=prometheus;mimir;cortex;thanos;victoriametrics`.

`victoriametrics` is included because `internal/helpers.isPrometheusSource` already accepts
it for the SLI metric-source dialect. Excluding it would have hard-rejected a value the
codebase already acknowledges. It is handled exactly like Thanos and Prometheus: query API at
the root, no tenancy header, no remote rule push, no magic alerting. A typo such as `thanso` is
rejected at `kubectl apply` with a clear message instead of silently producing no rules.

The field stays `omitempty`; it does not become newly required. An empty type is handled by
the controller's default branch.

**Accepted upgrade caveat:** after the new CRD is applied, an existing `Datasource` whose
`type` is outside the enum becomes invalid and cannot be updated until corrected. Reads
continue to work. This is acceptable for a pre-`v1` project and belongs in the release notes.

### D6. Generated PrometheusRule objects carry a stable OSKO marker label

`CreatePrometheusRule` currently copies `slo.Labels` verbatim onto the generated object. For
Mimir this is irrelevant, because the MimirRule controller reads `.Spec.Groups` and ignores
object labels. For Thanos, object labels are the **entire** discovery mechanism.

Combined with "a null label selector matches no objects", the default outcome without this
change is: an SLO with no labels produces a `PrometheusRule` with no labels, which a
`ThanosRuler` with a `matchLabels` selector never selects, and nothing anywhere reports an
error.

OSKO therefore stamps a stable marker, merged over the inherited SLO labels:

```go
Labels: mergeLabels(slo.Labels, map[string]string{
    "app.kubernetes.io/managed-by": "osko",
    "osko.dev/slo":                 slo.Name,
}),
```

This gives users a selector that is knowable in advance instead of a convention they have to
invent and keep in sync. `mergeLabels` already exists at `prometheus_helper.go:101`.

*Blast radius:* existing Mimir users' `PrometheusRule` objects gain two labels on the next
reconcile, and those labels propagate one step further than is immediately obvious:

- The payload pushed to Mimir is unchanged. `NewMimirRuleGroups` builds rule groups from
  `rule.Spec.Groups` and `connectionDetails.SourceTenants` only; it never reads object
  metadata.
- `NewMimirRule` does copy `rule.Labels` and `rule.Annotations` onto the `MimirRule` object,
  so `MimirRule` objects inherit the same two markers.

Both are inert: the codebase contains no label selectors at all, and every `MimirRule` access
is a name/namespace `Get` or an owner-based `Owns` watch. The visible effect of the change is
therefore one update per `PrometheusRule` and per `MimirRule` at upgrade time, and nothing
else.

*Rejected:* documentation only, telling Thanos users to label their SLOs. Zero blast radius,
but the silent-no-op failure mode stays live and undetectable.

## Architecture

Before, every SLO gets a MimirRule regardless of backend:

```
SLO ─┬─> PrometheusRule       (always)
     ├─> MimirRule            (always)            ──> Mimir ruler API
     └─> AlertManagerConfig   (if magicAlerting)  ──> Mimir Alertmanager API
```

After, the two Mimir-specific branches become conditional on backend capability:

```
SLO ─┬─> PrometheusRule       (always, unchanged)
     ├─> MimirRule            (only if backend needs remote rule push)
     └─> AlertManagerConfig   (only if magicAlerting AND backend has an Alertmanager API)
```

Thanos path end to end:

```
SLO ──> PrometheusRule ──> [ThanosRuler.ruleSelector] ──> Thanos Ruler
                                                             │
                                       evaluates against Thanos Query
                                                             │
                                   alerts ──> Alertmanager (--alertmanagers.url)
```

No new controller, no new CRD, no new HTTP client.

## The `internal/backend` package

```go
package backend

type Type string

const (
    Prometheus      Type = "prometheus"
    Mimir           Type = "mimir"
    Cortex          Type = "cortex"
    Thanos          Type = "thanos"
    VictoriaMetrics Type = "victoriametrics"
)

// NeedsRemoteRulePush reports whether rule groups must be pushed to a remote ruler
// configuration API. Mimir and Cortex return true; backends that read rules from
// PrometheusRule objects return false.
func (t Type) NeedsRemoteRulePush() bool

// SupportsMagicAlerting reports whether the backend exposes an Alertmanager configuration
// API that OSKO can write routing configuration to. Mimir and Cortex return true.
func (t Type) SupportsMagicAlerting() bool

// QueryURL returns the base URL of the Prometheus-compatible query API for the backend.
// Mimir and Cortex serve it under /prometheus; the others serve it at the root.
func (t Type) QueryURL(address string) string

// TenantHeader returns the HTTP header carrying the tenant identifier, or "" when the
// backend has no tenancy header. Mimir and Cortex return X-Scope-OrgID; Thanos returns
// THANOS-TENANT.
func (t Type) TenantHeader() string
```

`Parse` lowercases and trims its input before matching. This is defensive rather than
decorative: the CRD enum only validates on write, so a `Datasource` stored before the upgrade
can still be read back with `Mimir`.

## Changes by file

| File | Change |
| --- | --- |
| `api/openslo/v1/datasource_types.go` | Add `+kubebuilder:validation:Enum=prometheus;mimir;cortex;thanos;victoriametrics` to `DatasourceSpec.Type`, and add `Conditions`/`Ready` to `DatasourceStatus` (required: `utils.UpdateStatus` locates them by reflection and is otherwise a no-op). |
| `internal/backend/backend.go` | New package as specified above. |
| `internal/backend/backend_test.go` | New table tests. |
| `internal/controller/openslo/datasource_controller.go:59` | Parse the type once, then add `thanos`, `prometheus` and `victoriametrics` cases that connect, sharing one branch. An unparseable type emits a Warning and sets Datasource status not-ready. |
| `internal/controller/openslo/datasource_controller.go:81` | Replace the `!= "mimir"` rejection and hardcoded `/prometheus` suffix with `backend.QueryURL`. Set the tenant header from `backend.TenantHeader` when `TargetTenant` is non-empty. Emit a Warning when `SourceTenants` is set on a Thanos datasource. |
| `internal/controller/openslo/slo_controller.go:211` | Wrap the MimirRule get-and-create block in `if backend.NeedsRemoteRulePush(ds.Spec.Type)`. |
| `internal/controller/openslo/slo_controller.go:273` | Gate magic alerting on `backend.SupportsMagicAlerting(ds.Spec.Type)`; emit a Warning event and record the status reason otherwise. |
| `internal/helpers/prometheus_helper.go:677` | Merge the OSKO marker labels into the generated `PrometheusRule` object metadata per D6. |
| `config/crd/bases/openslo.com_datasources.yaml` | Regenerated by `make manifests`, then synced with `make helm-crds`. |
| `config/samples/openslo_v1_datasource_thanos.yaml` | New Thanos datasource sample. |
| `config/samples/` | A Thanos SLO sample referencing the above. |
| `README.md` | Document supported backends and the ThanosRuler wiring. |
| `docs/labels-and-annotations.md` | Document that `osko.dev/magicAlerting` is unsupported on Thanos, and document the new marker labels. |
| `adr/0008_thanos_backend_support.md` | Decision record, following the format of ADRs 0001-0007. |

`isPrometheusSource()` needs no change: it already lowercases and already lists `thanos`, so
`SLI.metricSource.type: Thanos` works today.

`SetupWithManager`'s `Owns(&MimirRule{})` stays as-is. It is inert when no MimirRule exists.

## Canonical ThanosRuler wiring to document

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ThanosRuler
metadata:
  name: thanos-ruler
spec:
  ruleSelector:
    matchLabels:
      app.kubernetes.io/managed-by: osko
  ruleNamespaceSelector: {}   # omit and discovery is limited to this namespace only
  queryConfig:
    name: thanos-ruler
    key: query.yaml
```

## Error handling

| Condition | Behaviour |
| --- | --- |
| Unknown or empty `Datasource.Spec.Type` | Warning event, Datasource status not-ready with `unsupported datasource type: %q`. Permanent error, so no hot requeue loop. |
| `thanos` datasource with `magicAlerting: "true"` | Warning event `MagicAlertingUnsupported`. No AlertManagerConfig created. SLO stays Ready. |
| `thanos` datasource with `sourceTenants` set | Warning event `SourceTenantsIgnored`. Non-fatal. |
| Thanos query endpoint unreachable | Existing transient-retry path, unchanged. |

## Testing

Unit:

- `internal/backend` table tests covering all four backend types against all four functions,
  plus mixed-case input and unknown or empty values.

Integration (envtest):

- A `thanos` Datasource is connected against its address as given, with no `/prometheus`
  suffix appended, and with a `THANOS-TENANT` header only when `targetTenant` is set.
- A `prometheus` Datasource follows the same path and is not reported as unsupported.
- SLO with a `thanos` datasource creates a `PrometheusRule` and does **not** create a `MimirRule`.
- Regression: SLO with a `mimir` datasource still creates both.
- SLO with a `thanos` datasource and `magicAlerting: "true"` creates no `AlertManagerConfig`,
  emits the warning event, and leaves the SLO Ready.
- Generated `PrometheusRule` objects carry `app.kubernetes.io/managed-by: osko` and
  `osko.dev/slo: <name>`, and still carry any labels inherited from the SLO.
- Ownership and cascade-delete behaviour is unchanged on the Thanos path.

CRD validation:

- Applying a `Datasource` with `type: bogus` is rejected by the API server.

## Release notes

- New supported datasource types: `thanos` and `victoriametrics`. Rules are delivered as `PrometheusRule` objects;
  point your `ThanosRuler.ruleSelector` at `app.kubernetes.io/managed-by: osko`.
- `Datasource.spec.type` is now validated against `prometheus|mimir|cortex|thanos|victoriametrics`. Existing
  Datasources with any other value must be corrected before they can be updated.
- Generated `PrometheusRule` objects now carry `app.kubernetes.io/managed-by: osko` and
  `osko.dev/slo` labels, and `MimirRule` objects inherit them. Existing objects of both kinds
  are updated once on upgrade. The rule payload pushed to Mimir is unchanged.
- `osko.dev/magicAlerting` is not supported on `thanos` datasources; configure Alertmanager
  through Thanos Ruler's `--alertmanagers.url`.

## Out of scope, possible follow-ups

- A `ThanosRule` CRD rendering rule files into ConfigMaps, for Thanos Ruler deployed without
  prometheus-operator.
- Translating magic alerting into `monitoring.coreos.com/v1alpha1 AlertmanagerConfig`.
- Completing the Cortex implementation, which is still a NotImplemented warning.
- Removing the dead `ConnectionDetails.SyncPrometheusRules` field, which belongs with the
  cleanup work described in ADR 0006.
