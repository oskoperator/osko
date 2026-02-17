# Error Handling Strategy in Reconcile Loops

* Status: accepted
* Date: 2026-02-17
* Implementation Date: 2026-02-17

## Context and Problem Statement

Multiple controllers in OSKO swallow errors by returning `ctrl.Result{}, nil` instead of propagating errors. This prevents:
1. Proper retry mechanisms via controller-runtime's exponential backoff
2. Error visibility in operator metrics and logs
3. Correct status updates to reflect failures

### Current Problematic Code

```go
// slo_controller.go:62-64
if err != nil {
    log.Error(err, errGetSLO)
    return ctrl.Result{}, nil  // Error is swallowed!
}

// mimirrule_controller.go:87-91
rgs, err := helpers.NewMimirRuleGroups(prometheusRule, &mimirRule.Spec.ConnectionDetails)
if err != nil {
    log.Error(err, "Failed to convert MimirRuleGroup")
    // Execution continues with empty rgs - no return!
}
```

## Considered Options

* **Option A**: Return errors directly (`return ctrl.Result{}, err`)
* **Option B**: Return errors with requeue delay (`return ctrl.Result{RequeueAfter: duration}, err`)
* **Option C**: Use error wrapping with sentinel errors for classification
* **Option D**: Implement custom error types with retry semantics

## Decision Outcome

Chosen option: **Option C with aspects of Option D** - Use error wrapping with sentinel errors for classification, combined with helper functions for consistent error handling.

### Implementation

```go
// internal/errors/errors.go
package errors

import (
    "errors"
    "time"
)

var (
    ErrTransient      = errors.New("transient error")
    ErrPermanent      = errors.New("permanent error")
    ErrDependencyNotReady = errors.New("dependency not ready")
)

type ReconcileError struct {
    Err        error
    Type       error
    RequeueAfter time.Duration
}

func (e *ReconcileError) Error() string { return e.Err.Error() }
func (e *ReconcileError) Unwrap() error { return e.Err }

func Transient(err error, requeueAfter time.Duration) *ReconcileError {
    return &ReconcileError{Err: err, Type: ErrTransient, RequeueAfter: requeueAfter}
}

func Permanent(err error) *ReconcileError {
    return &ReconcileError{Err: err, Type: ErrPermanent}
}

func DependencyNotReady(err error) *ReconcileError {
    return &ReconcileError{Err: err, Type: ErrDependencyNotReady, RequeueAfter: 10 * time.Second}
}
```

```go
// Controller usage
func (r *SLOReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    slo := &openslov1.SLO{}
    if err := r.Get(ctx, req.NamespacedName, slo); err != nil {
        if apierrors.IsNotFound(err) {
            return ctrl.Result{}, nil
        }
        return ctrl.Result{}, errors.Transient(err, 5*time.Second)
    }
    // ...
}
```

### Positive Consequences

* Errors are properly propagated to controller-runtime for exponential backoff
* Status conditions accurately reflect failures
* Better debugging through error classification
* Consistent error handling across all controllers

### Negative Consequences

* Requires refactoring all controllers
* May increase reconciliation frequency for transient errors
* Need to carefully classify errors to avoid infinite retry loops

## Pros and Cons of the Options

### Option A: Return errors directly

* Good, because it's simple and enables controller-runtime's built-in retry
* Bad, because all errors are treated equally (no classification)
* Bad, because no control over requeue timing

### Option B: Return errors with requeue delay

* Good, because provides control over retry timing
* Bad, because inconsistent across controllers
* Bad, because no error classification

### Option C: Use error wrapping with sentinel errors

* Good, because enables error classification
* Good, because allows different retry strategies per error type
* Good, because provides consistent pattern across codebase

### Option D: Implement custom error types with retry semantics

* Good, because most flexible
* Good, because self-documenting error handling
* Bad, because more complex to implement

## Implementation Status

### Completed
- [x] Created `internal/errors/errors.go` with `ReconcileError` type and helper functions
- [x] Updated `slo_controller.go` to use `errors.Transient()`, `errors.Permanent()`, `errors.DependencyNotReady()`
- [x] Updated `mimirrule_controller.go` to use `errors.Transient()`
- [x] Updated `alertmanagerconfig_controller.go` to use `errors.Transient()`
- [x] Updated `sli_controller.go` to use `errors.Transient()`
- [x] Updated `datasource_controller.go` to use `errors.Transient()`

### Files Modified
- `internal/errors/errors.go` - New error types package
- `internal/controller/openslo/slo_controller.go`
- `internal/controller/openslo/sli_controller.go`
- `internal/controller/openslo/datasource_controller.go`
- `internal/controller/osko/mimirrule_controller.go`
- `internal/controller/osko/alertmanagerconfig_controller.go`

## Links

* [Controller Runtime Error Handling](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/reconcile)
* [Kubernetes API Conventions - Errors](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#errors)
