# osko - OpenSLO Kubernetes Operator

This operator aims to provide it's users with simple management of SLIs, SLOs, alerting rules and alerts routing via Kubernetes CRDs according to the [OpenSLO](https://github.com/OpenSLO/OpenSLO) specification (currently `v1`).

See the [design document](DESIGN.md) for more details on what `osko` aims to do.

## Here be dragons!

`osko` is in very active development, hardly functional and definitely not stable. Until a `v1` release comes around, use at your own risk.

## Prerequisites

### Mimir ruler limits

Each SLO expands into a couple of dozen recording and alerting rules, which runs into two Mimir ruler limits. Both default to values that are low for SLO workloads, and exceeding either makes the ruler reject the whole rule group with `HTTP 400`. The rules never reach Mimir, so nothing evaluates:

```
per-user rules per rule group limit (limit: 20 actual: 21) exceeded
```

| Limit | Mimir default | What it constrains |
| --- | --- | --- |
| `ruler_max_rules_per_rule_group` | 20 | Rules in one group, so the number of windows per SLO |
| `ruler_max_rule_groups_per_tenant` | 70 | Total groups, so the number of SLOs per tenant |

An SLO with magic alerting enabled produces two groups, sized by the number of distinct windows `W`:

```
<slo>_recording   2 + W rules   (target, osko_sli_total, osko_sli_measurement)
<slo>_alert       4 + W rules   (osko_error_budget_burn_rate, burn-rate alerts)
```

`W` is the seven fixed alerting windows (`5m`, `30m`, `1h`, `2h`, `6h`, `24h`, `3d`) plus the SLO's base and reporting windows where those differ. The defaults give `W = 8`, so 11 and 12 rules. A group limit of 20 therefore allows `W ≤ 16`; a custom `osko.dev/baseWindow` or an unusual `timeWindow` adds at most two.

Without magic alerting the burn-rate rules stay in the recording group, giving one group of `3 + 2W` rules and a lower ceiling of `W ≤ 8`.

Raise both limits in the Mimir `limits` block:

```yaml
limits:
  ruler_max_rules_per_rule_group: 40
  ruler_max_rule_groups_per_tenant: 200
```

Or per tenant, via the runtime overrides file:

```yaml
overrides:
  my-tenant:
    ruler_max_rules_per_rule_group: 40
    ruler_max_rule_groups_per_tenant: 200
```

Divide `ruler_max_rule_groups_per_tenant` by two to get the number of SLOs a tenant can hold. Check what is actually in effect with `curl -H "X-Scope-OrgID: <tenant>" http://<mimir>/config | grep ruler_max`.

## Test It Out

1. You’ll need a Kubernetes cluster to run `osko`. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.
   - refer to the [Installation and usage](https://github.com/kubernetes-sigs/kind#installation-and-usage) section of the [KIND](https://sigs.k8s.io/kind) README to use KIND.

**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

2. Install the CRDs into the cluster:

```sh
make install
```

3. We also depend on Prometheus Operator CRDs (`monitoring.coreos.com` API group). Let's install that to our local cluster now:

```sh
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update
helm install prometheus-operator-crds prometheus-community/prometheus-operator-crds
```

4. Install sample CRDs into the cluster, so `osko` has resources to work with:

```sh
kubectl apply -k config/samples/
```

5. Run your controller (this will run in the foreground, so switch to a new terminal if you want to leave it running):

```sh
make run
```

**NOTE:** You can also run this in one step by running: `make install run`

#### Modifying the API definitions

If you are editing the API definitions, generate the manifests such as CRs or CRDs using:

```sh
make manifests
```

**NOTE:** Run `make --help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

### CRD Management

CRDs are generated from Go types via `controller-gen` and live in `config/crd/bases/`. The Helm chart's CRD subchart (`helm/osko/charts/crds/templates/`) is intentionally empty in the repository — CRDs are copied in automatically during the release CI pipeline.

For local development with Helm, sync CRDs manually:

```bash
make helm-crds
```

> **Note:** Do not commit the copied CRDs to the Helm subchart. They are generated artifacts managed by CI.

## Contributing

Feel free to open an issue or a pull (merge) request if
you would like to see a particular feature implemented after reading the below requirements:

- Please sign your commits off using the `-s` flag during `git commit` after reading the
  project's [DCO](DCO).
- It would be greatly appreciated if you tried using
  [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) for the commit style.

## License

For license, see the [LICENSE](LICENSE) file in the root of this repository.

## Community

If you have any questions or need general advice or help, feel free to join the
[#osko channel on the OpenSLO Slack](https://openslo.slack.com/archives/C06T64CP5DK)

## Sponsors

<img src="assets/HG Logo_Heureka Group Color.png" width="33%">
