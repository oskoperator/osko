# Labels and annotations

OSKO resources often use labels and annotations for configuring different behaviors for different
OpenSLO types. This is the documentation of the available labels and annotations that you can use
and what they are used for.

## Labels

### `label.osko.dev/<key>`

Enables labeling of Prometheus recording and alerting rules, for example for alert routing purposes.

```yaml
label.osko.dev/team: "infrastructure"
```

## Annotations

### `osko.dev/datasourceRef`

Configures which Datasource to use in an SLO definition.

Accepts a name of the Datasource as string.

```yaml
osko.dev/datasourceRef: "mimir-infra-ds"
```

### `osko.dev/baseWindow`

Configures the base window for an individual SLO (instead of the default of "5m" specified in the config).

Accepts a string in the [time.Duration](https://pkg.go.dev/time#Duration) format.

```yaml
osko.dev/baseWindow: "30m"
```

### `osko.dev/magicAlerting`

Configures whether OSKO creates multiwindow, multi-burn-rate alerts for the SLO, automagically.

Accepts the string "true" as the only valid input.

```yaml
osko.dev/magicAlerting: "true"
```

Supported only on `mimir` and `cortex` datasources — the two backends that expose an
Alertmanager configuration API. On `thanos`, `prometheus` and `victoriametrics` it emits
a `MagicAlertingUnsupported` warning event and the SLO remains Ready, because the
burn-rate alerting rules are generated regardless. Only the Alertmanager routing
configuration is skipped.

The `AlertManagerConfig` this creates holds no configuration itself — it points at a
Secret named `<slo>-alerting-config` that you supply, and stays `Ready=False` until that
Secret exists. Note also that each push replaces the **whole tenant's** Alertmanager
configuration, not just this SLO's. See [Alertmanager configs](alertmanager-configs.md).

### `osko.dev/alertingTool`

Chooses the vocabulary of the `severity` label OSKO stamps on the generated burn-rate
alerts. It changes nothing else — routing remains entirely yours to write.

```yaml
osko.dev/alertingTool: pagerduty
```

| Value | `severity` becomes |
| --- | --- |
| `opsgenie` (default) | `P1`, `P2`, `P3`, `P4` |
| `pagerduty` | `SEV_1`, `SEV_2`, `SEV_3`, `SEV_4` |
| `custom` | `OSKO_ALERTING_SEVERITY_CRITICAL` / `_HIGH` / `_MEDIUM` / `_LOW`, defaulting to `critical`, `high`, `medium`, `low` |

An unrecognised value falls back to `custom` rather than failing, so a typo yields the
custom defaults instead of an error.

Set the default for every SLO with `OSKO_ALERTING_TOOL` on the operator; the annotation
overrides it per SLO.

These are Alertmanager **routing labels**, not the target tool's own severity values.
`SEV_1` in particular is not a valid PagerDuty severity — PagerDuty accepts only
`critical`, `error`, `warning` and `info`, so each receiver must state one itself. See
[routing to PagerDuty](alertmanager-configs.md#example-routing-to-pagerduty).

## Labels applied by OSKO

Generated `PrometheusRule` objects, and the `MimirRule` objects derived from them, carry:

| Label | Value | Purpose |
| --- | --- | --- |
| `app.kubernetes.io/managed-by` | `osko` | Stable selector for `ThanosRuler.ruleSelector` and `Prometheus.ruleSelector` |
| `osko.dev/slo` | The SLO's name | Traceability back to the owning SLO |

Labels on the SLO are inherited by the generated `PrometheusRule`. The two labels above are
applied on top and win on conflict.
