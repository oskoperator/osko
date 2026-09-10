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

Not supported on `thanos` or `prometheus` datasources. Setting it there emits a
`MagicAlertingUnsupported` warning event and the SLO remains Ready, because the
burn-rate alerting rules are generated regardless. Only the Alertmanager routing
configuration is skipped.

## Labels applied by OSKO

Generated `PrometheusRule` objects, and the `MimirRule` objects derived from them, carry:

| Label | Value | Purpose |
| --- | --- | --- |
| `app.kubernetes.io/managed-by` | `osko` | Stable selector for `ThanosRuler.ruleSelector` and `Prometheus.ruleSelector` |
| `osko.dev/slo` | The SLO's name | Traceability back to the owning SLO |

Labels on the SLO are inherited by the generated `PrometheusRule`. The two labels above are
applied on top and win on conflict.
