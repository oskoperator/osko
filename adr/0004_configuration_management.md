# Configuration Management Pattern

* Status: proposed
* Date: 2026-02-17

## Context and Problem Statement

OSKO uses a global mutable variable for configuration (`var Cfg Config` in `internal/config/config.go`). This creates several issues:

### Current Problems

```go
// internal/config/config.go
var Cfg Config  // Global mutable variable

func NewConfig() {
    Cfg = Config{
        MimirRuleRequeuePeriod: GetEnvAsDuration("MIMIR_RULE_REQUEUE_PERIOD", 60*time.Second),
        // ...
    }
}
```

1. **Thread Safety**: Global mutable variable is not thread-safe
2. **Testability**: Tests cannot easily isolate configuration
3. **No Validation**: Configuration values are not validated at startup
4. **No Hot-Reload**: Configuration requires restart to change
5. **Hidden Dependencies**: Functions access `config.Cfg` directly, making dependencies implicit

## Considered Options

* **Option A**: Dependency injection via controller constructors
* **Option B**: Configuration mounted as ConfigMap with watches
* **Option C**: Immutable config loaded at startup with validation
* **Option D**: viper/cobra with validated configuration struct

## Decision Outcome

Chosen option: **Option A + Option C** - Dependency injection via controller constructors with immutable validated config loaded at startup.

### Implementation

```go
// internal/config/config.go
package config

import (
    "fmt"
    "time"
)

type Config struct {
    MimirRuleRequeuePeriod time.Duration
    AlertingBurnRates      AlertingBurnRates
    DefaultBaseWindow      time.Duration
    AlertingTool           string
}

type AlertingBurnRates struct {
    PageShortWindow   float64
    PageLongWindow    float64
    TicketShortWindow float64
    TicketLongWindow  float64
}

// Load creates and validates configuration - returns error if invalid
func Load() (*Config, error) {
    cfg := &Config{
        MimirRuleRequeuePeriod: getEnvAsDuration("MIMIR_RULE_REQUEUE_PERIOD", 60*time.Second),
        DefaultBaseWindow:      getEnvAsDuration("DEFAULT_BASE_WINDOW", 5*time.Minute),
        AlertingTool:           getEnv("OSKO_ALERTING_TOOL", "opsgenie"),
        AlertingBurnRates: AlertingBurnRates{
            PageShortWindow:   getEnvAsFloat64("ABR_PAGE_SHORT_WINDOW", 14.4),
            PageLongWindow:    getEnvAsFloat64("ABR_PAGE_LONG_WINDOW", 6),
            TicketShortWindow: getEnvAsFloat64("ABR_TICKET_SHORT_WINDOW", 3),
            TicketLongWindow:  getEnvAsFloat64("ABR_TICKET_LONG_WINDOW", 1),
        },
    }

    if err := cfg.Validate(); err != nil {
        return nil, fmt.Errorf("invalid configuration: %w", err)
    }

    return cfg, nil
}

func (c *Config) Validate() error {
    if c.MimirRuleRequeuePeriod < 0 {
        return fmt.Errorf("MIMIR_RULE_REQUEUE_PERIOD must be non-negative")
    }
    if c.DefaultBaseWindow < time.Minute {
        return fmt.Errorf("DEFAULT_BASE_WINDOW must be at least 1 minute")
    }
    if c.AlertingTool == "" {
        return fmt.Errorf("OSKO_ALERTING_TOOL cannot be empty")
    }
    if c.AlertingBurnRates.PageShortWindow <= 0 {
        return fmt.Errorf("ABR_PAGE_SHORT_WINDOW must be positive")
    }
    return nil
}
```

```go
// cmd/main.go
func main() {
    cfg, err := config.Load()
    if err != nil {
        setupLog.Error(err, "failed to load configuration")
        os.Exit(1)
    }

    // Pass config to controllers via constructor
    if err = (&openslocontroller.SLOReconciler{
        Client:   mgr.GetClient(),
        Scheme:   mgr.GetScheme(),
        Config:   cfg,
        Recorder: mgr.GetEventRecorderFor("slo-controller"),
    }).SetupWithManager(mgr); err != nil {
        setupLog.Error(err, "unable to create controller", "controller", "SLO")
        os.Exit(1)
    }
}
```

```go
// internal/controller/openslo/slo_controller.go
type SLOReconciler struct {
    client.Client
    Scheme   *runtime.Scheme
    Config   *config.Config  // Injected, not global
    Recorder record.EventRecorder
}

func (r *SLOReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    // Use r.Config instead of config.Cfg
    requeuePeriod := r.Config.MimirRuleRequeuePeriod
    // ...
}
```

### Testing Example

```go
// internal/controller/openslo/slo_controller_test.go
func TestSLOReconciler_WithCustomConfig(t *testing.T) {
    cfg := &config.Config{
        MimirRuleRequeuePeriod: 30 * time.Second,
        DefaultBaseWindow:      10 * time.Minute,
    }

    r := &SLOReconciler{
        Client: fake.NewClientBuilder().Build(),
        Config: cfg,
    }

    // Test with isolated configuration
}
```

### Positive Consequences

* Thread-safe immutable configuration
* Easy to test with isolated configurations
* Fail-fast on invalid configuration
* Explicit dependencies via constructor injection
* No hidden global state

### Negative Consequences

* Requires refactoring all controllers to accept config
* Slightly more verbose initialization
* Configuration changes still require restart (but this is acceptable for operators)

## Pros and Cons of the Options

### Option A: Dependency injection

* Good, because explicit dependencies
* Good, because testable
* Good, because follows Go best practices
* Bad, because requires refactoring

### Option B: ConfigMap with watches

* Good, because enables hot-reload
* Good, because Kubernetes-native
* Bad, because adds complexity
* Bad, because config changes during reconciliation can cause issues

### Option C: Immutable config with validation

* Good, because simple
* Good, because fail-fast
* Good, because no runtime surprises
* Bad, because requires restart for changes

### Option D: viper/cobra

* Good, because feature-rich
* Good, because standard in Go ecosystem
* Bad, because adds dependency for limited benefit
* Bad, because overkill for current needs

## Links

* [Dependency Injection in Go](https://dave.cheney.net/2014/06/07/five-things-that-make-go-fast)
* [Kubernetes Operator Best Practices](https://sdk.operatorframework.io/docs/best-practices/)
