# Controller Responsibility Boundaries

* Status: proposed
* Date: 2026-02-17

## Context and Problem Statement

OSKO has overlapping controller responsibilities that create architectural confusion and potential for reconciliation conflicts:

### Current Issues

1. **Finalizer Overlap**: MimirRule controller adds `finalizer.osko.dev/prometheusrule` to PrometheusRule resources it doesn't own
2. **Watch Overlap**: Multiple controllers watch the same resources:
   - MimirRule controller watches both MimirRule and PrometheusRule
   - PrometheusRule controller watches both PrometheusRule and SLO
3. **Creation vs Management Split**: SLO controller creates MimirRule, but MimirRule controller manages its lifecycle

```go
// mimirrule_controller.go - Adds finalizer to PrometheusRule
if !controllerutil.ContainsFinalizer(prometheusRule, prometheusRuleFinalizer) {
    controllerutil.AddFinalizer(prometheusRule, prometheusRuleFinalizer)
}

// prometheusrule_controller.go - Also watches PrometheusRule
func (r *PrometheusRuleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    // Watches PrometheusRule but also has SLO watches
}
```

This creates:
- Race conditions during deletion
- Unclear ownership semantics
- Difficulty reasoning about reconciliation flow

## Considered Options

* **Option A**: Single SLO controller handles all downstream resources
* **Option B**: Event-driven architecture with clear ownership
* **Option C**: Owner-based cascading with no cross-controller finalizers
* **Option D**: Keep current structure but remove finalizer conflicts

## Decision Outcome

Chosen option: **Option C** - Owner-based cascading with no cross-controller finalizers.

### Principles

1. **One Owner, One Finalizer**: Only the controller that owns a resource adds finalizers
2. **No Cross-Controller Finalizers**: Controllers cannot add finalizers to resources they don't own
3. **Explicit Ownership Chain**: SLO → MimirRule → Mimir API (via MimirRule controller)

### Proposed Architecture

```
SLO Controller
├── Owns: PrometheusRule (finalizer: slo.osko.dev/prometheusrule)
├── Owns: MimirRule (finalizer: slo.osko.dev/mimirrule)
├── Owns: AlertManagerConfig (finalizer: slo.osko.dev/alertmanagerconfig)
└── Owns: inline SLI (no finalizer, uses OwnerReference GC)

MimirRule Controller
├── Watches: MimirRule (owned by SLO)
├── Manages: Mimir API rules (via finalizer on MimirRule)
└── Does NOT add finalizers to PrometheusRule

AlertManagerConfig Controller
├── Watches: AlertManagerConfig (owned by SLO)
└── Manages: AlertManager API config (via finalizer on AlertManagerConfig)
```

### Implementation Changes

```go
// MimirRule controller - REMOVE this:
// if !controllerutil.ContainsFinalizer(prometheusRule, prometheusRuleFinalizer) {
//     controllerutil.AddFinalizer(prometheusRule, prometheusRuleFinalizer)
// }

// MimirRule controller - Only manage MimirRule's finalizer:
const mimirRuleFinalizer = "finalizer.mimirrule.osko.dev"

func (r *MimirRuleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    mimirRule := &oskov1alpha1.MimirRule{}
    if err := r.Get(ctx, req.NamespacedName, mimirRule); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }

    // Only manage finalizer on MimirRule, not PrometheusRule
    if mimirRule.DeletionTimestamp != nil {
        if controllerutil.ContainsFinalizer(mimirRule, mimirRuleFinalizer) {
            // Cleanup Mimir API rules
            if err := r.cleanupMimirRules(ctx, mimirRule); err != nil {
                return ctrl.Result{}, err
            }
            controllerutil.RemoveFinalizer(mimirRule, mimirRuleFinalizer)
            return ctrl.Result{}, r.Update(ctx, mimirRule)
        }
        return ctrl.Result{}, nil
    }

    if !controllerutil.ContainsFinalizer(mimirRule, mimirRuleFinalizer) {
        controllerutil.AddFinalizer(mimirRule, mimirRuleFinalizer)
        return ctrl.Result{}, r.Update(ctx, mimirRule)
    }

    // Sync rules to Mimir
    return r.syncToMimir(ctx, mimirRule)
}
```

### Positive Consequences

* Clear ownership boundaries
* No race conditions from conflicting finalizers
* Easier to understand reconciliation flow
* Simpler debugging

### Negative Consequences

* Requires removing existing finalizer logic
* May require migration for existing resources
* SLO controller becomes more complex (owns more resources)

## Pros and Cons of the Options

### Option A: Single SLO controller handles all

* Good, because eliminates all overlap
* Bad, because creates a monolithic controller
* Bad, because harder to test and maintain

### Option B: Event-driven architecture

* Good, because clean separation
* Good, because testable in isolation
* Bad, because adds complexity (events, queues)
* Bad, because significant refactor required

### Option C: Owner-based cascading

* Good, because minimal changes required
* Good, because follows Kubernetes patterns
* Good, because clear ownership
* Bad, because requires careful finalizer cleanup

### Option D: Keep current structure

* Good, because no changes needed
* Bad, because doesn't solve the problem
* Bad, because ongoing technical debt

## Links

* [Kubernetes Owner References](https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/)
* [Controller Runtime Finalizers](https://book.kubebuilder.io/reference/using-finalizers)
