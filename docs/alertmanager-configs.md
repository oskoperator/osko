# Alertmanager configs

When an SLO carries `osko.dev/magicAlerting: "true"`, OSKO creates an
`AlertManagerConfig` resource for it. That resource does **not** contain the
Alertmanager configuration — it points at a Secret you supply. OSKO reads the
Secret and pushes its contents to the datasource's Alertmanager.

Without that Secret the `AlertManagerConfig` stays `Ready=False` with
`Secret from secretRef not found`, and no routing is ever configured.

## The naming is not symmetric

For an SLO named `my-slo`, OSKO creates:

| Resource | Name |
| --- | --- |
| `AlertManagerConfig` | `my-slo-alerting` |
| Secret it expects | `my-slo-alerting-config` |

The Secret must contain a single key, **`alertmanager.yaml`**, holding a
complete Alertmanager configuration.

```bash
kubectl create secret generic my-slo-alerting-config \
  --from-file=alertmanager.yaml=./alertmanager.yaml
```

Both live in the SLO's namespace unless the `AlertManagerConfig`'s
`spec.secretRef.namespace` says otherwise.

## One configuration per tenant, not per SLO

This is the part that surprises people.

OSKO pushes through Mimir's `POST /api/v1/alerts`, which **replaces the entire
tenant's** Alertmanager configuration. It is not merged, and it is not scoped to
the SLO that triggered it.

So if five SLOs in one tenant each have an `AlertManagerConfig`, all five push to
the same place and the last writer wins. If their Secrets differ, the tenant's
routing flips every time any one of them reconciles.

Two ways to live with this:

1. **Give every Secret in a tenant identical contents.** Whichever SLO reconciles
   last pushes the same bytes, so the result is stable. Route per SLO inside that
   one configuration using the labels OSKO puts on the alerts — `slo_name`,
   `service`, `team`, `severity`.
2. **Enable magic alerting on exactly one SLO per tenant** and treat its Secret as
   the tenant's routing configuration.

The first is usually what you want, and it is what the example below does.

## Labels available for routing

Every burn-rate alert OSKO generates carries:

| Label | Example | Notes |
| --- | --- | --- |
| `slo_name` | `mimir-api-latency` | |
| `sli_name` | `mimir-api-latency-5ms` | |
| `service` | `mimir-api` | from `spec.service` |
| `severity` | `P1` | mapped from the SRE tier via the alerting tool |
| `short_window` | `5m` | the fast leg of the burn-rate pair |
| `long_window` | `1h` | the slow leg |
| `alertname` | `mimir-api-latency_alert_page_critical` | |

Any label you add to the SLO as `label.osko.dev/team: infra` appears on the alert
as `team: infra`, so arbitrary routing keys are available.

## Escalation: one problem, one page

A burning SLO trips **several tiers at once**. That is not a bug — it is what
multi-window multi-burn-rate is for. The fast tier catches a sharp burn quickly,
the slow tiers confirm a sustained one, and a genuinely broken service satisfies
all four conditions simultaneously.

Without inhibition that means four notifications for one problem. On ntfy that is
merely noisy. On PagerDuty it is four incidents and four pages at 03:00.

`inhibit_rules` fixes it. Each tier suppresses every quieter tier for the **same
`slo_name`**:

```yaml
inhibit_rules:
  - source_matchers: ['severity="P1"']
    target_matchers: ['severity=~"P2|P3|P4"']
    equal: ['slo_name']
  - source_matchers: ['severity="P2"']
    target_matchers: ['severity=~"P3|P4"']
    equal: ['slo_name']
  - source_matchers: ['severity="P3"']
    target_matchers: ['severity="P4"']
    equal: ['slo_name']
```

`equal: ['slo_name']` is what keeps this safe. A P1 on one SLO must never silence
a P4 on a different one.

Both shipped configurations already include these rules.

### What escalation looks like

A slow burn trips P4 first and you get one low-priority notification. If the burn
accelerates, P1 starts firing, P4 is immediately suppressed, and you get one page
at the highest severity. As the incident recovers the tiers drop away in reverse.
At every moment exactly one notification stream is live per SLO.

You can watch the suppression directly:

```bash
curl -s -H "X-Scope-OrgID: <tenant>" \
  '<mimir>/alertmanager/api/v2/alerts?filter=slo_name%3D"my-slo"' \
  | jq -r '.[] | "\(.labels.severity) \(.status.state) \(.status.inhibitedBy)"'
```

```
P1   active      []
P3   suppressed  ["8a71efff422051f9"]
```

The fingerprint in `inhibitedBy` is the alert doing the suppressing.

### A caveat if you page PagerDuty

Alertmanager's `pagerduty_config` has no `dedup_key` field — PagerDuty dedupes on
the Alertmanager **group key**. Because `group_by` includes `severity`, each tier
is a separate group and therefore a separate PagerDuty incident.

So an escalation from P3 to P1 resolves the P3 incident and opens a new P1 one,
rather than raising the urgency of a single incident in place. One incident is
open at a time either way, which is usually what you want. If you would rather
have a single long-lived incident, drop `severity` from `group_by` — but then one
receiver serves every tier and the PagerDuty severity can no longer be a constant
per receiver.

## Example: routing to ntfy.sh

`devel/mimir/alertmanager-default-config.yaml` ships a working configuration that
routes each severity to [ntfy.sh](https://ntfy.sh) with a distinct priority. Copy
it, replace `NTFY_TOPIC`, and load it into a Secret.

> An ntfy topic is effectively a password. Anyone who knows it can read your
> alerts and publish to them. Pick something unguessable and do not commit it.

```bash
sed 's/NTFY_TOPIC/your-unguessable-topic/' \
  <(kubectl get cm mimir-alertmanager-config -o jsonpath='{.data.alertmanager-fallback-config\.yaml}') \
  > /tmp/alertmanager.yaml

for slo in $(kubectl get slo -o name | cut -d/ -f2); do
  kubectl create secret generic "${slo}-alerting-config" \
    --from-file=alertmanager.yaml=/tmp/alertmanager.yaml \
    --dry-run=client -o yaml | kubectl apply -f -
done
```

### Why the message needs templating on ntfy's side

Alertmanager's webhook payload is a fixed JSON document and is **not**
templatable. Point a webhook receiver straight at ntfy and the notification body
is the raw JSON — receiver, status, every label, the full `generatorURL`.

ntfy can render it instead. Setting `X-Template: 1` makes ntfy treat `X-Title` and
`X-Message` as templates evaluated against the incoming JSON, and `\n` in
`X-Message` becomes a real newline. Alertmanager sends `http_headers` values
verbatim, so the `{{ }}` passes through untouched.

```yaml
http_headers:
  X-Template:
    values: ['1']
  X-Title:
    values: ['{{if eq .status "firing"}}🔥{{else}}✅{{end}} {{(first .alerts).labels.severity}} · {{(first .alerts).labels.slo_name}}']
```

Note the shape: `http_headers` values are objects with a `values` list, not plain
strings. `Tags: bell` is rejected with
`cannot unmarshal !!str 'bell' into config.Header`.

ntfy also ships a built-in template that needs no configuration —
`https://ntfy.sh/<topic>?template=alertmanager`. It is a reasonable starting
point, but it renders `Instance: <no value>` for OSKO alerts, which carry no
`instance` label, and it has no access to the SLO-specific labels above.

## Example: routing to PagerDuty

Select the tool on the SLO, which changes the `severity` label OSKO stamps on the
generated alerts:

```yaml
metadata:
  annotations:
    osko.dev/magicAlerting: "true"
    osko.dev/alertingTool: pagerduty
```

With `pagerduty` the four tiers become `SEV_1`, `SEV_2`, `SEV_3`, `SEV_4`
(`opsgenie`, the default, produces `P1`–`P4`). Set it globally instead with
`OSKO_ALERTING_TOOL=pagerduty` on the operator.

> **`SEV_1` is not a PagerDuty severity.** PagerDuty's Events API v2 accepts only
> `critical`, `error`, `warning` and `info`. `SEV_*` is an Alertmanager *routing*
> label; passing it through as `severity: '{{ .CommonLabels.severity }}'` makes
> PagerDuty reject the event. Each receiver must state a valid severity itself.

```yaml
route:
  group_by: ['slo_name', 'severity']
  group_wait: 30s
  group_interval: 5m
  repeat_interval: 4h
  receiver: pd-low
  routes:
    - matchers: ['severity="SEV_1"']
      receiver: pd-critical
    - matchers: ['severity="SEV_2"']
      receiver: pd-error
    - matchers: ['severity="SEV_3"']
      receiver: pd-warning
    - matchers: ['severity="SEV_4"']
      receiver: pd-low

inhibit_rules:
  - source_matchers: ['severity="SEV_1"']
    target_matchers: ['severity=~"SEV_2|SEV_3|SEV_4"']
    equal: ['slo_name']
  - source_matchers: ['severity="SEV_2"']
    target_matchers: ['severity=~"SEV_3|SEV_4"']
    equal: ['slo_name']
  - source_matchers: ['severity="SEV_3"']
    target_matchers: ['severity="SEV_4"']
    equal: ['slo_name']

receivers:
  - name: pd-critical
    pagerduty_configs:
      - &pd
        routing_key: PAGERDUTY_INTEGRATION_KEY
        send_resolved: true
        description: '{{ .CommonLabels.slo_name }} burning error budget ({{ .CommonLabels.severity }})'
        client: osko
        details:
          slo: '{{ .CommonLabels.slo_name }}'
          sli: '{{ .CommonLabels.sli_name }}'
          service: '{{ .CommonLabels.service }}'
          windows: '{{ .CommonLabels.short_window }} and {{ .CommonLabels.long_window }}'
          firing: '{{ .Alerts.Firing | len }}'
        severity: critical
  - name: pd-error
    pagerduty_configs:
      - <<: *pd
        severity: error
  - name: pd-warning
    pagerduty_configs:
      - <<: *pd
        severity: warning
  - name: pd-low
    pagerduty_configs:
      - <<: *pd
        severity: info
```

`routing_key` is an **Events API v2** integration key, created on the PagerDuty
service under *Integrations → Add integration → Events API v2*. The whole document
lives in a Secret, so the key is not in plain sight — but it is not encrypted
either, so treat the Secret accordingly.

Unlike the ntfy webhook payload, `pagerduty_config` fields **are** templatable,
so no receiver-side rendering is needed.

## Troubleshooting

**`Ready=False`, `Secret from secretRef not found`.** The Secret does not exist,
is in the wrong namespace, or is missing the `alertmanager.yaml` key.

**Secret exists but the status never changes.** Reconcile it by hand:

```bash
kubectl annotate alertmanagerconfigs.osko.dev --all \
  osko.dev/reconcile-nudge="$(date +%s)" --overwrite
```

Note that `kubectl get alertmanagerconfig` is ambiguous when prometheus-operator
is installed — it resolves to `alertmanagerconfigs.monitoring.coreos.com`. Always
spell out `alertmanagerconfigs.osko.dev`.

**All alerting stopped, yet every `AlertManagerConfig` still says `Ready=True`.**
Someone deleted an `AlertManagerConfig`. Its finalizer deletes the tenant's
**entire** Alertmanager configuration, not just that SLO's contribution — see
[#154](https://github.com/oskoperator/osko/issues/154). The survivors keep
reporting `Ready=True` because that reflects their last successful push, not what
Mimir currently holds.

Confirm it:

```bash
curl -s -H "X-Scope-OrgID: <tenant>" http://<mimir>/api/v1/alerts
# alertmanager_config: ""
```

Recover by making a surviving `AlertManagerConfig` push again:

```bash
kubectl annotate alertmanagerconfigs.osko.dev --all \
  osko.dev/restore-nudge="$(date +%s)" --overwrite
```

This only works if the Secrets still hold the configuration, which is another
reason to keep every Secret in a tenant identical.

**`Ready=True` but nothing arrives.** The config reached Mimir but the ruler
cannot reach Alertmanager. Check the ruler's own metrics:

```bash
curl -s -H "X-Scope-OrgID: <tenant>" http://<mimir>/metrics \
  | grep -E 'ruler_notifications_(sent|errors|dropped)_total'
```

`errors_total` climbing in step with `dropped_total` while `sent_total` stays at
zero means `ruler.alertmanager_url` points somewhere that does not resolve.

**Check what Mimir actually holds** for a tenant:

```bash
curl -s -H "X-Scope-OrgID: <tenant>" http://<mimir>/api/v1/alerts
```

**Send a test alert** without waiting for a real burn, which also bypasses
`repeat_interval` because the fingerprint is new:

```bash
curl -X POST -H "X-Scope-OrgID: <tenant>" -H "Content-Type: application/json" \
  -d '[{"labels":{"alertname":"test","severity":"P1","slo_name":"test-slo"}}]' \
  http://<mimir>/alertmanager/api/v2/alerts
```
