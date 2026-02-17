# Test Coverage Strategy for Core Logic

* Status: proposed
* Date: 2026-02-17

## Context and Problem Statement

OSKO has significant test coverage gaps, particularly in core business logic:

### Current Test Coverage

| Component | Lines | Test Status |
|-----------|-------|-------------|
| `prometheus_helper.go` | 546 | **No tests** |
| `mimirtool_helper.go` | ~100 | **No tests** |
| `mimirrule_controller.go` | ~200 | **No tests** |
| `alertmanagerconfig_controller.go` | ~150 | Empty scaffold |
| `slo_controller.go` | 496 | Ownership tests only |
| `common_utils.go` | ~100 | **No tests** |

### Critical Untested Logic

```go
// prometheus_helper.go - Complex rule generation with NO tests
func NewMimirRuleGroups(prometheusRule *monitoringv1.PrometheusRule, cd *ConnectionDetails) (MimirRuleGroups, error) {
    // 100+ lines of transformation logic
}

func CreateAlertingRules(slo *openslov1.SLO, sli *openslov1.SLI, ruleLabels map[string]string) ([]Rule, error) {
    // Complex multi-window, multi-burn-rate alert generation
    // Multiple edge cases around ratio vs threshold metrics
}

func CreateRecordingRules(slo *openslov1.SLO, sli *openslov1.SLI) ([]Rule, error) {
    // Generates recording rules for SLI measurements
    // Edge cases for good/bad event counting
}
```

This untested code is the core value proposition of OSKO - generating correct SLO recording and alerting rules.

## Considered Options

* **Option A**: Table-driven unit tests for helper functions
* **Option B**: Integration tests with envtest for full reconciliation
* **Option C**: Contract tests with golden files
* **Option D**: All of the above (layered approach)

## Decision Outcome

Chosen option: **Option D** - Layered testing approach with table-driven unit tests, golden file tests, and integration tests.

### Testing Layers

```
┌─────────────────────────────────────────────────────────────┐
│ Layer 3: Integration Tests (envtest)                        │
│ - Full reconciliation loop                                   │
│ - Resource creation and deletion                             │
│ - Status updates                                             │
│ - External API mocking                                       │
├─────────────────────────────────────────────────────────────┤
│ Layer 2: Golden File Tests                                   │
│ - Snapshot testing for rule generation                       │
│ - Easy to review expected output                             │
│ - Catches unintended changes                                 │
├─────────────────────────────────────────────────────────────┤
│ Layer 1: Unit Tests (table-driven)                           │
│ - Helper function logic                                      │
│ - Edge cases                                                 │
│ - Error conditions                                           │
└─────────────────────────────────────────────────────────────┘
```

### Implementation

#### Layer 1: Unit Tests

```go
// internal/helpers/prometheus_helper_test.go
package helpers

import (
    "testing"
    "github.com/stretchr/testify/assert"
    openslov1 "github.com/patrik/osko/api/openslo/v1"
)

func TestCreateRecordingRules(t *testing.T) {
    tests := []struct {
        name      string
        slo       *openslov1.SLO
        sli       *openslov1.SLI
        wantRules int
        wantErr   bool
    }{
        {
            name: "ratio metric with good/bad events",
            slo: &openslov1.SLO{
                ObjectMeta: metav1.ObjectMeta{Name: "test-slo"},
                Spec: openslov1.SLOSpec{
                    Objectives: []openslov1.Objective{{Target: 0.99}},
                },
            },
            sli: &openslov1.SLI{
                Spec: openslov1.SLISpec{
                    Indicator: &openslov1.Indicator{
                        RatioMetric: &openslov1.RatioMetric{
                            Good: openslov1.MetricSource{Prometheus: &openslov1.PrometheusQuery{Query: "good_events"}},
                            Total: openslov1.MetricSource{Prometheus: &openslov1.PrometheusQuery{Query: "total_events"}},
                        },
                    },
                },
            },
            wantRules: 4, // good, bad, total, measurement
            wantErr: false,
        },
        {
            name: "threshold metric",
            sli: &openslov1.SLI{
                Spec: openslov1.SLISpec{
                    Indicator: &openslov1.Indicator{
                        ThresholdMetric: &openslov1.MetricSource{
                            Prometheus: &openslov1.PrometheusQuery{Query: "latency_seconds"},
                        },
                    },
                },
            },
            wantRules: 2, // total, measurement
            wantErr: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            rules, err := CreateRecordingRules(tt.slo, tt.sli)
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
                assert.Len(t, rules, tt.wantRules)
            }
        })
    }
}
```

#### Layer 2: Golden File Tests

```go
// internal/helpers/prometheus_helper_golden_test.go
package helpers

import (
    "testing"
    "github.com/stretchr/testify/require"
)

func TestGolden_CreateAlertingRules(t *testing.T) {
    slo := &openslov1.SLO{
        ObjectMeta: metav1.ObjectMeta{
            Name: "api-latency",
            Namespace: "default",
        },
        Spec: openslov1.SLOSpec{
            Service: "api",
            Objectives: []openslov1.Objective{{Target: 0.999}},
        },
    }

    sli := &openslov1.SLI{
        Spec: openslov1.SLISpec{
            Indicator: &openslov1.Indicator{
                ThresholdMetric: &openslov1.MetricSource{
                    Prometheus: &openslov1.PrometheusQuery{
                        Query: `histogram_quantile(0.99, rate(http_request_duration_seconds_bucket{service="api"}[5m]))`,
                    },
                },
            },
        },
    }

    rules, err := CreateAlertingRules(slo, sli, map[string]string{"slo": "api-latency"})
    require.NoError(t, err)

    // Compare against golden file
    golden.Assert(t, "testdata/alerting_rules_api_latency.golden.yaml", rules)
}
```

```yaml
# testdata/alerting_rules_api_latency.golden.yaml
- alert: SLOAPILatencyPageShortTerm
  expr: |
    (
      osko_error_budget_burn_rate{slo_name="api-latency",window="1h"} > 14.4
    and
      osko_error_budget_burn_rate{slo_name="api-latency",window="5m"} > 14.4
    )
  for: 2m
  labels:
    severity: page
    slo: api-latency
```

#### Layer 3: Integration Tests

```go
// internal/controller/openslo/slo_controller_integration_test.go
func TestSLOReconciler_CreatesPrometheusRule(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    // Setup envtest
    cfg, k8sClient := setupEnvtest(t)
    defer teardownEnvtest(t, cfg)

    // Create datasource
    ds := &openslov1.Datasource{
        ObjectMeta: metav1.ObjectMeta{Name: "mimir", Namespace: "default"},
        Spec: openslov1.DatasourceSpec{
            Type: openslov1.DatasourceTypeMimir,
            ConnectionDetails: openslov1.ConnectionDetails{
                Address: "http://mimir:8080",
            },
        },
    }
    require.NoError(t, k8sClient.Create(ctx, ds))

    // Create SLO with inline SLI
    slo := &openslov1.SLO{
        ObjectMeta: metav1.ObjectMeta{
            Name: "test-slo",
            Namespace: "default",
            Annotations: map[string]string{"osko.dev/datasourceRef": "mimir"},
        },
        Spec: openslov1.SLOSpec{
            Indicator: &openslov1.Indicator{ /* ... */ },
        },
    }
    require.NoError(t, k8sClient.Create(ctx, slo))

    // Wait for PrometheusRule to be created
    Eventually(func() bool {
        pr := &monitoringv1.PrometheusRule{}
        err := k8sClient.Get(ctx, client.ObjectKey{Namespace: "default", Name: "test-slo"}, pr)
        return err == nil
    }, 10*time.Second, 1*time.Second).Should(BeTrue())
}
```

### Positive Consequences

* High confidence in core logic correctness
* Easy to detect regressions
* Documentation through examples
* Faster development cycle with unit tests

### Negative Consequences

* Initial time investment to write tests
* Golden files require maintenance
* Integration tests are slower

## Pros and Cons of the Options

### Option A: Table-driven unit tests

* Good, because fast and focused
* Good, because easy to write
* Bad, because doesn't test integration

### Option B: Integration tests only

* Good, because tests real behavior
* Bad, because slow
* Bad, because harder to debug

### Option C: Golden file tests

* Good, because easy to review
* Good, because catches unintended changes
* Bad, because can be brittle
* Bad, because requires manual golden updates

### Option D: All of the above

* Good, because comprehensive
* Good, because catches bugs at multiple levels
* Bad, because more maintenance
* Bad, because initial setup cost

## Links

* [Testing Kubernetes Controllers](https://book.kubebuilder.io/cronjob-tutorial/writing-tests)
* [Golden File Testing in Go](https://github.com/sebdah/goldie)
* [Table Driven Tests](https://dave.cheney.net/2019/05/07/prefer-table-driven-tests)
