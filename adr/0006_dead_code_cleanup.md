# Dead Code and Incomplete Implementation Cleanup

* Status: proposed
* Date: 2026-02-17

## Context and Problem Statement

OSKO contains significant dead code and incomplete implementations that create maintenance burden and confuse users:

### Dead Code

```go
// prometheus_helper.go:494-496
func CreateAlertingRule(slo *openslov1.SLO, sli *openslov1.SLI, ruleLabels map[string]string) (*Rule, error) {
    return nil, nil  // Always returns nil, never used
}

// common_utils.go - Unused struct types
type Rule struct { ... }          // Duplicates types in prometheus_helper.go
type BudgetRule struct { ... }    // Never instantiated
type MetricLabel struct { ... }   // Never used
```

### Incomplete Implementations

```go
// datasource_controller.go - Cortex not implemented
if ds.Spec.Type == openslov1.DatasourceTypeCortex {
    r.Recorder.Event(ds, "Warning", "NotImplemented", "Cortex support is not implemented yet")
    // Controller continues and may cause issues
}

// Empty controller implementations
// alertpolicy_controller.go - No reconciliation logic
// alertcondition_controller.go - No reconciliation logic
// alertnotificationtarget_controller.go - No reconciliation logic
```

### Hard-coded Values

```go
// prometheus_helper.go
const mimirRuleNamespace = "osko"  // Hard-coded, not configurable
extendedWindow := "28d"             // Hard-coded, ignores config.DefaultBaseWindow

// slo_controller.go
if slo.ObjectMeta.Annotations["osko.dev/magicAlerting"] == "true" {
    // No validation, silent failure if annotation malformed
}
```

## Considered Options

* **Option A**: Remove all dead code and incomplete implementations
* **Option B**: Implement missing features
* **Option C**: Mark as experimental/deprecated with clear documentation
* **Option D**: Phased cleanup with deprecation notices

## Decision Outcome

Chosen option: **Option D** - Phased cleanup with deprecation notices.

### Phase 1: Immediate Removal (Dead Code)

```go
// DELETE: prometheus_helper.go:494-496
// func CreateAlertingRule(...) (*Rule, error) { return nil, nil }

// DELETE: common_utils.go unused types
// type Rule struct { ... }
// type BudgetRule struct { ... }
// type MetricLabel struct { ... }
```

### Phase 2: Deprecation Warnings (Incomplete Controllers)

```go
// datasource_controller.go
func (r *DatasourceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    ds := &openslov1.Datasource{}
    if err := r.Get(ctx, req.NamespacedName, ds); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }

    if ds.Spec.Type == openslov1.DatasourceTypeCortex {
        r.Recorder.Event(ds, "Warning", "Deprecated", 
            "Cortex support is deprecated and will be removed in v2.0. Use Mimir instead.")
        utils.UpdateStatus(ctx, ds, r.Client, "Ready", metav1.ConditionFalse, 
            "CortexDatasourceDeprecated", "Use Mimir datasource instead")
        return ctrl.Result{}, nil
    }
    // ...
}
```

### Phase 3: Remove or Implement (Next Major Version)

For incomplete controllers, choose one:

**Option 3a: Remove CRDs and Controllers**
```bash
# Remove from api/
rm -rf api/openslo/v1/alertpolicy_types.go
rm -rf api/openslo/v1/alertcondition_types.go
rm -rf api/openslo/v1/alertnotificationtarget_types.go

# Remove controllers
rm -rf internal/controller/openslo/alertpolicy_controller.go
rm -rf internal/controller/openslo/alertcondition_controller.go
rm -rf internal/controller/openslo/alertnotificationtarget_controller.go
```

**Option 3b: Implement with Clear Scope**
```go
// alertpolicy_controller.go - Implement if part of core OpenSLO spec
func (r *AlertPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    // Implementation based on OpenSLO spec
    // Reference SLOs that use this policy
    // Create AlertManagerConfig based on notification targets
}
```

### Phase 4: Configuration for Hard-coded Values

```go
// internal/config/config.go
type Config struct {
    // ...
    MimirRuleNamespace string
    DefaultExtendedWindow time.Duration
}

// prometheus_helper.go
func CreateRecordingRules(slo *openslov1.SLO, sli *openslov1.SLI, cfg *config.Config) ([]Rule, error) {
    extendedWindow := cfg.DefaultExtendedWindow.String()  // Use config
    // ...
}
```

### Documentation Updates

```markdown
# docs/deprecated-features.md

## Deprecated Features

| Feature | Deprecated | Removal | Migration |
|---------|------------|---------|-----------|
| Cortex Datasource | v1.5.0 | v2.0.0 | Use Mimir datasource |
| AlertPolicy CRD | v1.5.0 | v2.0.0 | Use AlertManagerConfig |
| AlertCondition CRD | v1.5.0 | v2.0.0 | Use SLO burn rate alerts |

## Removed Features

| Feature | Removed In | Replacement |
|---------|------------|-------------|
| CreateAlertingRule helper | v1.5.0 | Use CreateAlertingRules |
```

### Positive Consequences

* Cleaner codebase
* No confusing dead ends for users
* Clear migration path
* Reduced maintenance burden

### Negative Consequences

* Breaking changes for users using deprecated features
* Requires versioning strategy
* Documentation effort

## Pros and Cons of the Options

### Option A: Remove all dead code immediately

* Good, because cleanest codebase
* Good, because no confusion
* Bad, because may break users relying on undefined behavior
* Bad, because no migration time

### Option B: Implement missing features

* Good, because full OpenSLO spec support
* Bad, because significant effort
* Bad, because may not be needed

### Option C: Mark as experimental/deprecated

* Good, because gives users time
* Good, because documents current state
* Bad, because still has dead code

### Option D: Phased cleanup

* Good, because balances cleanup and stability
* Good, because clear timeline
* Good, because documented migration
* Bad, because takes time

## Links

* [Go Dead Code Elimination](https://golang.org/doc/faq#unused_variables_and_imports)
* [Kubernetes Deprecation Policy](https://kubernetes.io/docs/reference/using-api/deprecation-policy/)
* [OpenSLO Spec](https://github.com/OpenSLO/OpenSLO)
