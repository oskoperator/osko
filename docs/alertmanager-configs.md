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
