# osko - the OpenSLO Kubernetes Operator

This operator aims to provide it's users with simple management and setting of SLIs, SLOs, alerting rules and alerts routing according to the
[OpenSLO](https://github.com/OpenSLO/OpenSLO) specification.

## Goals

- [ ] To connect to and work with the following metrics datasources using [kind: Datasource](https://github.com/OpenSLO/OpenSLO#datasource)
    - [ ] Mimir
    - [ ] Cortex
- [ ] Understand required (baseline) of the [kind: SLI](https://github.com/OpenSLO/OpenSLO#sli) and [kind: SLO](https://github.com/OpenSLO/OpenSLO#slo)
        specs. There is not yet a clear definition of which ones.
- [ ] Be able to set up recording rules according to the data received through the [kind: SLI](https://github.com/OpenSLO/OpenSLO#sli) resources, either
        in the Mimir or Cortex ruler
- [ ] Set up alerts in Alertmanager based on kinds [kind: AlertPolicy](https://github.com/OpenSLO/OpenSLO#alertpolicy) and
        [kind: AlertCondition](https://github.com/OpenSLO/OpenSLO#alertcondition)


## Test It Out

1. You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

2. Install the CRDs into the cluster:

```sh
make install
```

3. Run your controller (this will run in the foreground, so switch to a new terminal if you want to leave it running):

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
As of right now, we don't have any specific guidelines for contributors.

Feel free to open an issue or a pull (merge) request if
you would like to see a particular feature implemented.

The only thing that's required right now to get your code merged is signing your commits off with the `-s` flag during `git commit`
after reading the project's [DCO](DCO).

It would also be greatly appreciated if you tried using
[Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) for the commit style, but that's a detail :)


## License

For license, see the LICENSE file in the root of this repository.
