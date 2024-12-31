# osko - OpenSLO Kubernetes Operator

This operator aims to provide it's users with simple management of SLIs, SLOs, alerting rules and alerts routing via Kubernetes CRDs according to the [OpenSLO](https://github.com/OpenSLO/OpenSLO) specification (currently `v1`).

See the [design document](DESIGN.md) for more details on what `osko` aims to do.

## Here be dragons!

`osko` is in very active development, hardly functional and definitely not stable. Until a `v1` release comes around, use at your own risk.

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
