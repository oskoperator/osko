# Alert Tier Table Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make one `alertTier` table the single source of truth for burn-rate alerting, so the severity key is read in one place instead of switched on in three.

**Architecture:** `buildAlertRules` already declares the window pairing as data. Extend that table to carry the burn rate and the `for` duration as well, pass the resolved tier into `createMultiBurnRateAlert`, and delete the two switches that re-derive the same facts. Rename the four config knobs that are named for windows but hold per-tier burn rates.

**Tech Stack:** Go 1.23, prometheus-operator `monitoringv1` types, stdlib `testing`.

**Spec:** `docs/superpowers/specs/2026-09-15-alert-tier-table-design.md`

## Global Constraints

- Generated PromQL must be **byte-identical** before and after, for every tier. Expression, labels, annotations and alert names do not change.
- Burn-rate defaults stay `14.4`, `6`, `3`, `1`.
- Alert `for` durations stay `2m` for both page tiers and `15m` for both ticket tiers.
- **Every existing test must pass unmodified.** `prometheus_helper_test.go` references none of the renamed config fields, so the rename is invisible to it. If an existing test needs editing, the change is not a pure refactor — stop and report.
- Conventional Commits with a scope, signed off: `git commit -s`.
- `go.mod` and `go.sum` unchanged.
- `make test` green before any commit.

## The one non-obvious constraint

`config.Cfg` is populated by `config.NewConfig()` at runtime, **after** package initialisation. A package-level `var alertTiers = []alertTier{...}` reading `config.Cfg.AlertingBurnRates` would therefore capture zero values, and every alert would compare against `> 0.0` — which fires permanently.

`alertTiers` must be a **function** that reads config at call time. The plan below does that, and the comment explaining why must survive into the code.

---

### Task 1: Consolidate the tier table and rename the burn-rate config

**Files:**
- Modify: `internal/config/types.go:15-19`
- Modify: `internal/config/config.go:14-19`
- Modify: `internal/helpers/prometheus_helper.go:523-558` (`buildAlertRules`, `alertDurationFor`), `:592-623` (`createMultiBurnRateAlert` signature and switch)
- Modify: `adr/0004_configuration_management.md` (lines ~73-76 and ~98 name the old env vars)
- Test: `internal/helpers/prometheus_helper_test.go` (append one test)

**Interfaces:**
- Consumes: `config.SREAlertSeverity` constants `PageCritical`, `PageHigh`, `TicketHigh`, `TicketMedium` (`internal/config/types.go:30-33`); existing helpers `mrs.getBurnRateWindows`, `brw.get`, `brw.hasWindows`, `isValidRule`, `mapToColonSeparatedString`, `config.AlertSeveritiesByTool`.
- Produces: `type alertTier struct{ short, long string; severity config.SREAlertSeverity; burnRate float64; wait monitoringv1.Duration }`, `func alertTiers() []alertTier`, and `createMultiBurnRateAlert(brw *burnRateWindows, tier alertTier) monitoringv1.Rule`.
- Removes: `func alertDurationFor(config.SREAlertSeverity) monitoringv1.Duration`.

- [ ] **Step 1: Write the failing test**

Append to `internal/helpers/prometheus_helper_test.go`. **Add `"fmt"` to that file's import block** — it is not currently imported. `strings` already is.

```go
// TestAlertTiers_ExpressionMatchesTable pins every generated alert to its row in
// alertTiers(). This is the guard for the defect that motivated the refactor: the
// window pairing used to live both in buildAlertRules' table and in a switch inside
// createMultiBurnRateAlert, so the two could disagree silently.
func TestAlertTiers_ExpressionMatchesTable(t *testing.T) {
	groups := setupRules(t, createTestSLOWithAlerting("0.999"), createTestSLI(), "5m")
	alertGroup := groupByName(t, groups, alertGroupName)

	byAlert := map[string]monitoringv1.Rule{}
	for _, r := range alertRules(alertGroup) {
		byAlert[r.Alert] = r
	}

	tiers := alertTiers()
	if len(tiers) == 0 {
		t.Fatal("alertTiers() returned no tiers")
	}

	for _, tier := range tiers {
		t.Run(string(tier.severity), func(t *testing.T) {
			name := fmt.Sprintf("test-slo_alert_%s", tier.severity)
			rule, ok := byAlert[name]
			if !ok {
				t.Fatalf("no alert generated for tier %s", tier.severity)
			}

			if got := rule.Labels["short_window"]; got != tier.short {
				t.Errorf("short_window = %q, want %q", got, tier.short)
			}
			if got := rule.Labels["long_window"]; got != tier.long {
				t.Errorf("long_window = %q, want %q", got, tier.long)
			}

			// Both sides of the conjunction compare against the tier's single burn
			// rate. A tier wired to the wrong config field still produces a
			// valid-looking alert, so assert the number, not just its presence.
			threshold := fmt.Sprintf("> %.1f", tier.burnRate)
			expr := rule.Expr.String()
			if n := strings.Count(expr, threshold); n != 2 {
				t.Errorf("expr %q compares against %q %d times, want 2", expr, threshold, n)
			}

			if rule.For == nil {
				t.Fatalf("alert %s has no `for` duration", name)
			}
			if *rule.For != tier.wait {
				t.Errorf("for = %s, want %s", *rule.For, tier.wait)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/helpers/... -run TestAlertTiers_ExpressionMatchesTable`
Expected: FAIL to compile — `undefined: alertTiers`, and `tier.burnRate` / `tier.wait` undefined.

- [ ] **Step 3: Rename the config fields**

`internal/config/types.go`, replacing the `AlertingBurnRates` struct:

```go
// AlertingBurnRates holds the error-budget burn rate each alert tier compares
// against. The SRE Workbook defines one rate per tier, shared by both windows in
// that tier — not one rate per window, which is what the previous field names
// (PageShortWindow and friends) wrongly implied.
type AlertingBurnRates struct {
	PageCriticalBurnRate float64
	PageHighBurnRate     float64
	TicketHighBurnRate   float64
	TicketMediumBurnRate float64
}
```

`internal/config/config.go`, replacing the `AlertingBurnRates` literal:

```go
		AlertingBurnRates: AlertingBurnRates{
			PageCriticalBurnRate: GetEnvAsFloat64("ABR_PAGE_CRITICAL_BURN_RATE", 14.4),
			PageHighBurnRate:     GetEnvAsFloat64("ABR_PAGE_HIGH_BURN_RATE", 6),
			TicketHighBurnRate:   GetEnvAsFloat64("ABR_TICKET_HIGH_BURN_RATE", 3),
			TicketMediumBurnRate: GetEnvAsFloat64("ABR_TICKET_MEDIUM_BURN_RATE", 1),
		},
```

Defaults are unchanged. Only the field names and env var names move.

- [ ] **Step 4: Add the tier type and table**

In `internal/helpers/prometheus_helper.go`, immediately above `buildAlertRules`:

```go
// alertTier is one row of the SRE Workbook's multiwindow, multi-burn-rate table.
// https://sre.google/workbook/alerting-on-slos/#6-multiwindow-multi-burn-rate-alerts
//
// Both windows in a tier compare against the same burn rate: the rate is a
// property of the tier, not of a window.
type alertTier struct {
	short    string
	long     string
	severity config.SREAlertSeverity
	burnRate float64
	wait     monitoringv1.Duration
}

// alertTiers is a function, not a package-level var, because the burn rates come
// from config.Cfg, which main() populates through config.NewConfig() after package
// initialisation. A var would capture zeroes and every alert would compare against
// `> 0.0`, firing permanently.
//
// The windows are the SRE Workbook's and are deliberately not user-tunable.
func alertTiers() []alertTier {
	rates := config.Cfg.AlertingBurnRates
	return []alertTier{
		{short: "5m", long: "1h", severity: config.PageCritical, burnRate: rates.PageCriticalBurnRate, wait: "2m"},
		{short: "30m", long: "6h", severity: config.PageHigh, burnRate: rates.PageHighBurnRate, wait: "2m"},
		{short: "2h", long: "24h", severity: config.TicketHigh, burnRate: rates.TicketHighBurnRate, wait: "15m"},
		{short: "6h", long: "3d", severity: config.TicketMedium, burnRate: rates.TicketMediumBurnRate, wait: "15m"},
	}
}
```

The `wait` values must match what `alertDurationFor` returns today: `2m` for `PageCritical` and `PageHigh`, `15m` for `TicketHigh` and `TicketMedium`. `TestSetupRules_AlertForDurations` pins these by alert name and will catch a mismatch.

- [ ] **Step 5: Rewrite `buildAlertRules` against the table**

Replace the whole function body, dropping its local `tiers` literal:

```go
func (mrs *MonitoringRuleSet) buildAlertRules(alertingBurnRates []monitoringv1.Rule) []monitoringv1.Rule {
	burnRateWindows := mrs.getBurnRateWindows(alertingBurnRates)

	var alertRules []monitoringv1.Rule
	for _, tier := range alertTiers() {
		if !burnRateWindows.hasWindows(tier.short, tier.long) {
			continue
		}
		alertRules = append(alertRules, mrs.createMultiBurnRateAlert(burnRateWindows, tier))
	}

	return alertRules
}
```

- [ ] **Step 6: Delete `alertDurationFor` and rewrite `createMultiBurnRateAlert`**

Delete `func alertDurationFor(...)` and its doc comment entirely. Replace `createMultiBurnRateAlert` with:

```go
func (mrs *MonitoringRuleSet) createMultiBurnRateAlert(
	brw *burnRateWindows,
	tier alertTier,
) monitoringv1.Rule {
	log := ctrllog.FromContext(context.Background())

	shortWindow := brw.get(tier.short)
	longWindow := brw.get(tier.long)

	if !isValidRule(shortWindow) || !isValidRule(longWindow) {
		log.V(1).Info("Missing or invalid burn rate windows for alert",
			"severity", tier.severity,
			"shortWindowValid", isValidRule(shortWindow),
			"longWindowValid", isValidRule(longWindow))
		return monitoringv1.Rule{}
	}

	shortLabels := mapToColonSeparatedString(shortWindow.Labels)
	longLabels := mapToColonSeparatedString(longWindow.Labels)

	// ignoring(window) is load-bearing: `and` matches on the full label set, and
	// the two sides differ in `window`, so a plain conjunction never intersects
	// and the alert can never fire.
	alertExpression := fmt.Sprintf(
		"(%s{%s} > %.1f and ignoring(window) %s{%s} > %.1f)",
		shortWindow.Record, shortLabels, tier.burnRate,
		longWindow.Record, longLabels, tier.burnRate,
	)

	alertingTool := mrs.Slo.ObjectMeta.Annotations[annotationAlertingTool]
	if alertingTool == "" {
		alertingTool = config.Cfg.AlertingTool
	}

	severities := config.AlertSeveritiesByTool(alertingTool)
	toolSeverity := severities.GetSeverity(tier.severity)

	log.V(1).Info("Alerting rule", "sreSeverity", tier.severity, "toolSeverity", toolSeverity)

	// Copy to a local before taking its address: For is a *Duration, and the
	// tier is a loop variable in the caller.
	wait := tier.wait

	return monitoringv1.Rule{
		Alert: fmt.Sprintf("%s_alert_%s", mrs.Slo.Name, tier.severity),
		Expr:  intstr.FromString(alertExpression),
		For:   &wait,
		Labels: map[string]string{
			"severity":     toolSeverity,
			"slo_name":     mrs.Slo.Name,
			"sli_name":     mrs.Sli.Name,
			"short_window": shortWindow.Labels["window"],
			"long_window":  longWindow.Labels["window"],
		},
		Annotations: map[string]string{
			"summary":     "SLO Burn Rate Alert",
			"description": fmt.Sprintf("The burn rate of SLO %s is consuming error budget faster than acceptable. Short window: %s, Long window: %s", mrs.Slo.Name, shortWindow.Labels["window"], longWindow.Labels["window"]),
		},
	}
}
```

Both `%.1f` verbs now take `tier.burnRate`. That is the same value the old code put in both slots, so the rendered expression is unchanged.

- [ ] **Step 7: Update ADR 0004**

`adr/0004_configuration_management.md` carries the old names in **three** places. All three must change:

- **Lines 60-63** — the `AlertingBurnRates` struct fields in its example type definition.
- **Lines 73-76** — the `getEnvAsFloat64("ABR_PAGE_SHORT_WINDOW", 14.4)` constructor example.
- **Lines 97-98** — a validation example: `if c.AlertingBurnRates.PageShortWindow <= 0 { return fmt.Errorf("ABR_PAGE_SHORT_WINDOW must be positive") }`.

Update every occurrence to the new names so the ADR does not document an interface that no longer exists. Do not otherwise rewrite the ADR. Verify with:

```bash
grep -n "ABR_\|PageShortWindow\|PageLongWindow\|TicketShortWindow\|TicketLongWindow" adr/0004_configuration_management.md
```

Expected after the edit: only new names appear.

- [ ] **Step 8: Run the new test, then the whole suite**

Run: `go test ./internal/helpers/... -run TestAlertTiers_ExpressionMatchesTable -v`
Expected: PASS, one subtest per tier.

Run: `make test`
Expected: all packages `ok`. **No existing test file may have been edited** apart from the append in Step 1 and its `fmt` import. Confirm with:

```bash
git diff --stat internal/helpers/prometheus_helper_test.go
```

Expected: insertions only, no deletions beyond the import line change.

- [ ] **Step 9: Prove the new test has teeth**

Temporarily change the `PageCritical` row's `burnRate` to `rates.PageHighBurnRate` (a plausible copy-paste error). Run `go test ./internal/helpers/... -run TestAlertTiers_ExpressionMatchesTable`.
Expected: FAIL on the `page_critical` subtest, reporting the threshold appears 0 times instead of 2.

Then temporarily swap the `TicketHigh` row's `short` and `long`. Re-run.
Expected: FAIL on `ticket_high` for `short_window`/`long_window`.

Revert both, confirm `git diff` on the production file is empty, and re-run to confirm GREEN. Paste all of it in the commit-ready evidence.

- [ ] **Step 10: Commit**

```bash
git add internal/config/types.go internal/config/config.go \
        internal/helpers/prometheus_helper.go internal/helpers/prometheus_helper_test.go \
        adr/0004_configuration_management.md
git commit -s -m "refactor(alerting)!: make the alert tier table the single source of truth

The severity key was switched on in three places: buildAlertRules' local
tiers table, a switch in createMultiBurnRateAlert that re-derived the same
window pairing, and alertDurationFor. Editing one and not the others was
silent - the hasWindows guard could validate a different pair than the
expression used.

Fold the windows, burn rate and \`for\` duration into one alertTier row.
createMultiBurnRateAlert now takes the resolved tier and has no switch;
alertDurationFor is gone; shortThreshold and longThreshold collapse into
the tier's single burnRate, which is what the SRE Workbook defines.

Rename the four burn-rate knobs, which were named for windows but hold
per-tier multipliers: both windows in a tier always compared against the
same value. They are absent from the Helm chart and the README.

Generated PromQL is unchanged, and the existing alerting tests pass
without modification.

BREAKING CHANGE: ABR_PAGE_SHORT_WINDOW, ABR_PAGE_LONG_WINDOW,
ABR_TICKET_SHORT_WINDOW and ABR_TICKET_LONG_WINDOW are renamed to
ABR_PAGE_CRITICAL_BURN_RATE, ABR_PAGE_HIGH_BURN_RATE,
ABR_TICKET_HIGH_BURN_RATE and ABR_TICKET_MEDIUM_BURN_RATE. Defaults are
unchanged."
```

---

### Task 2: Delete the dead `CreateAlertingRule`

**Files:**
- Modify: `internal/helpers/prometheus_helper.go:673-675`

**Interfaces:**
- Consumes: nothing.
- Produces: nothing. Removes `func CreateAlertingRule() (*monitoringv1.PrometheusRule, error)`.

- [ ] **Step 1: Confirm it is unreferenced**

Run: `grep -rn "CreateAlertingRule" --include="*.go" .`
Expected: exactly one line, its own declaration in `internal/helpers/prometheus_helper.go`. If anything else appears, stop — do not delete, and report.

- [ ] **Step 2: Delete it**

Remove these three lines:

```go
func CreateAlertingRule() (*monitoringv1.PrometheusRule, error) {
	return nil, nil
}
```

- [ ] **Step 3: Verify the build and suite**

Run: `go build ./... && go vet ./... && make test`
Expected: all clean, all packages `ok`. An exported function with no callers cannot break anything, so any failure here means something else is wrong — report rather than work around it.

- [ ] **Step 4: Commit**

```bash
git add internal/helpers/prometheus_helper.go
git commit -s -m "refactor(helpers): drop the unused CreateAlertingRule stub

CreateAlertingRule returned (nil, nil) and had no callers - a repo-wide
grep found only its own declaration. Belongs with the dead-code cleanup
in ADR 0006."
```

---

## Final verification

- [ ] `make test` green
- [ ] `git diff --stat origin/main..HEAD` touches only: the two config files, `prometheus_helper.go`, `prometheus_helper_test.go`, `adr/0004_configuration_management.md`, and the two `docs/superpowers/` files
- [ ] `git diff origin/main..HEAD -- go.mod go.sum` is empty
- [ ] `grep -rn "PageShortWindow\|PageLongWindow\|TicketShortWindow\|TicketLongWindow\|ABR_PAGE_SHORT\|ABR_PAGE_LONG\|ABR_TICKET_SHORT\|ABR_TICKET_LONG" --include="*.go" --include="*.md" .` returns nothing
- [ ] `grep -rn "alertDurationFor\|CreateAlertingRule" --include="*.go" .` returns nothing

## Out of scope

Recorded in the spec as D3: consolidating the six rule-generation types into the same structure. They take different arguments and `SetupRules` orders them by dependency on purpose, so a table would hide exactly the kind of implicit positional dependency issue #135 was filed about.
