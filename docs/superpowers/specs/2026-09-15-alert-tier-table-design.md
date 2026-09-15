# Alert tier table: one source of truth for burn-rate alerting

- **Date:** 2026-09-15
- **Status:** Proposed
- **Issue:** [#135](https://github.com/oskoperator/osko/issues/135)
- **Affects:** `internal/helpers/prometheus_helper.go`, `internal/config`

## Problem

Issue #135 was filed against positional indexing in the burn-rate alert builder:

```go
alertingPageWindowsOrder := []string{"1h", "5m", "6h", "30m", "24h", "2h", "3d"}
...
alertingPageWindows[alertingPageWindowsOrder[2]].Record,
mapToColonSeparatedString(burnRates[2].Labels),
```

The window came from a hand-scrambled order slice by index, and its labels came from
`burnRates[N]` by index, assuming rule generation order matched. That indexing is **already
gone** — `8190963` replaced it with `burnRateWindows`, a map keyed by the `window` label, and
`e6adc9f` fixed the correctness bugs on top, including the missing `ignoring(window)` that
made burn-rate alerts unable to fire at all.

What remains is the residue: the coupling stopped being positional and became **duplicated**.

### The severity key is switched on three times

`buildAlertRules:526` declares the pairing as data:

```go
tiers := []struct{ short, long string; severity config.SREAlertSeverity }{
    {"5m", "1h", config.PageCritical},
    {"30m", "6h", config.PageHigh},
    {"2h", "24h", config.TicketHigh},
    {"6h", "3d", config.TicketMedium},
}
```

It uses `tier.short`/`tier.long` for the `hasWindows` guard, then discards them and passes only
`severity`. `createMultiBurnRateAlert:602` re-derives the identical pairing from a switch, and
`alertDurationFor:552` switches on severity a third time for the `for` duration.

Editing a pair in one place and not the other is silent: the guard would validate one window
pair while the expression used another. Nothing in the type system or the tests prevents it.

### The burn-rate knobs describe the wrong quantity

```go
case config.PageCritical:
    shortThreshold = config.Cfg.AlertingBurnRates.PageShortWindow
    longThreshold  = config.Cfg.AlertingBurnRates.PageShortWindow   // same field
```

Both windows in a tier compare against the same value in every branch. That is correct — the
SRE Workbook defines one burn rate per tier (14.4, 6, 3, 1), not one per window. But the fields
are named `PageShortWindow` / `PageLongWindow` / `TicketShortWindow` / `TicketLongWindow`, and
the environment variables are `ABR_PAGE_SHORT_WINDOW=14.4`. Those read as durations. They are
per-tier burn-rate multipliers.

`shortThreshold` and `longThreshold` are therefore two variables that always hold the same
value.

## Decisions

### D1. One tier table carries every severity-keyed fact

```go
// alertTier is one row of the SRE Workbook's multiwindow, multi-burn-rate table.
// https://sre.google/workbook/alerting-on-slos/#6-multiwindow-multi-burn-rate-alerts
type alertTier struct {
	short    string                   // fast window, e.g. "5m"
	long     string                   // slow window, e.g. "1h"
	severity config.SREAlertSeverity
	burnRate float64                  // one per tier, both windows compare against it
	wait     monitoringv1.Duration    // the alert's `for`
}
```

`createMultiBurnRateAlert` takes the resolved tier instead of a bare severity and contains no
switch. `alertDurationFor` is deleted; its two values move into the table. `shortThreshold` and
`longThreshold` collapse into the tier's single `burnRate`.

The table stays a package-level `var` in `prometheus_helper.go` rather than moving to config:
the windows are the SRE Workbook's and are not user-tunable, which is a deliberate limit on
scope.

*Rejected:* keeping the switch and merely asserting the two agree in a test. That documents the
duplication instead of removing it.

### D2. The burn-rate config fields are renamed to say what they are

| Was | Becomes |
| --- | --- |
| `PageShortWindow` / `ABR_PAGE_SHORT_WINDOW` | `PageCriticalBurnRate` / `ABR_PAGE_CRITICAL_BURN_RATE` |
| `PageLongWindow` / `ABR_PAGE_LONG_WINDOW` | `PageHighBurnRate` / `ABR_PAGE_HIGH_BURN_RATE` |
| `TicketShortWindow` / `ABR_TICKET_SHORT_WINDOW` | `TicketHighBurnRate` / `ABR_TICKET_HIGH_BURN_RATE` |
| `TicketLongWindow` / `ABR_TICKET_LONG_WINDOW` | `TicketMediumBurnRate` / `ABR_TICKET_MEDIUM_BURN_RATE` |

Defaults are unchanged: 14.4, 6, 3, 1. The new names key off the severity they configure, which
is what the code actually does with them.

Old names stop working, with no compatibility shim. Justification: the variables appear nowhere
in `helm/` and nowhere in `README.md` — only in `internal/config/config.go` and
`adr/0004_configuration_management.md` — so they cannot be set through the chart and are
undocumented for users. A name that describes the wrong kind of quantity is a worse defect than
a break nobody can hit. `adr/0004` is updated in the same change.

### D3. Rule-type consolidation is explicitly out of scope

Issue #135 also asks to fold the rule *types* into the same structure. That should not be done,
and the issue's list of them is stale — `goodRule`, `badRule` and `errorBudgetValue` no longer
exist (inlined by `e6adc9f`), and a raw/derived split appeared that did not exist in 2024.

The six that remain take different arguments:

```go
createTargetRule(reportingWindow)
createSliTotalRule(w)
createDerivedSliTotalRule(w, baseWindow)
createRawSliMeasurementRule(w)
createDerivedSliMeasurementRule(w, baseWindow)
createBurnRateRule(w, errorBudgetTarget)
```

A single table over these needs a union struct with mostly-empty fields, or an interface plus a
context object. Worse, `SetupRules` emits them in dependency order deliberately — its own
comment reads *"strict dependency order inside a single group so Prometheus resolves the whole
chain within one evaluation interval."* A table would express that ordering as iteration order,
hiding exactly the kind of implicit positional dependency #135 was filed about.

The windows half of the ask is already solved: `resolveWindows` parses, dedupes and sorts
ascending by duration, and the raw/derived decision is a duration comparison, not a position.

### D4. Drive-by: delete `CreateAlertingRule`

`prometheus_helper.go:673` is `func CreateAlertingRule() (*monitoringv1.PrometheusRule, error) {
return nil, nil }`. A repo-wide grep finds one occurrence: its own declaration. It is in the
file being changed and belongs with the ADR 0006 dead-code cleanup.

## Non-goals

- No change to generated PromQL. The expression string, labels, annotations and alert names are
  byte-identical before and after, for every tier.
- No change to which alerts are produced, or when.
- No new configuration surface.

## Testing

The existing suite is the safety net. Verified: `prometheus_helper_test.go` references none of
`PageShortWindow`, `PageLongWindow`, `TicketShortWindow`, `TicketLongWindow`,
`AlertingBurnRates` or `alertDurationFor`, so a correct consolidation leaves it green with
**zero test edits** — including the config rename, which is invisible to it.

That makes the acceptance criterion unusually crisp: **if any existing test needs changing,
the change is not a pure refactor and something is wrong.** The relevant guards are:

- `TestSetupRules_AlertExpressionLiteral`
- `TestSetupRules_AlertForDurations`
- `TestSetupRules_MagicAlerting_WindowPairs`
- `TestSetupRules_MagicAlerting_CorrectWindowPairs`
- `TestSetupRules_AlertFiresOnlyWhenBothWindowsBreach`
- `TestBurnRateWindows_HasWindows`

`TestSetupRules_AlertForDurations` is worth calling out: it pins `page_critical` and
`page_high` to `2m` and both ticket tiers to `15m` by alert name. Those are exactly the values
moving out of `alertDurationFor` and into the table's `wait` field, so it is a direct guard on
D1's riskiest mechanical step.

That property — the suite passing unmodified — is the primary evidence that this is a
refactor and not a behaviour change, so it is worth stating in the commit.

One test is added: a table-driven check that for every `alertTier`, the generated alert uses
that tier's `short`/`long` windows and its `burnRate` on both sides of the conjunction. That is
the assertion that would have failed under the old duplication if the two sources disagreed,
and it is the regression guard for the defect being fixed.

## Release notes

- The four burn-rate environment variables are renamed to describe the tier they configure
  rather than a window: `ABR_PAGE_SHORT_WINDOW` becomes `ABR_PAGE_CRITICAL_BURN_RATE`,
  `ABR_PAGE_LONG_WINDOW` becomes `ABR_PAGE_HIGH_BURN_RATE`, `ABR_TICKET_SHORT_WINDOW` becomes
  `ABR_TICKET_HIGH_BURN_RATE`, and `ABR_TICKET_LONG_WINDOW` becomes
  `ABR_TICKET_MEDIUM_BURN_RATE`. Defaults are unchanged. The old names are no longer read; they
  were never exposed through the Helm chart.
- No change to generated recording or alerting rules.

## Risks

- **Silent default regression.** If a renamed field is wired to the wrong severity, alerts keep
  generating but at the wrong burn rate. Mitigated by the new per-tier test asserting the
  numeric threshold in the expression, not merely that an expression exists.
- **Loop-variable aliasing.** `for` takes `*monitoringv1.Duration`. Taking the address of a
  field on the loop variable is safe under this module's `go 1.23.0` per-iteration semantics,
  but the plan should copy into a local anyway rather than rely on the language version.
