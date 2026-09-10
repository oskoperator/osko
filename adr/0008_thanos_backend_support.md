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
* A `cortex` Datasource now reports `Ready=False` with `Cortex support is not implemented yet`, instead of warning and then reporting `Ready=True` for a backend it never contacted.

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
