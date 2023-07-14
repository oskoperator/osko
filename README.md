# osko - the OpenSLO Kubernetes Operator

This operator aims to provide it's users with simple management and setting of SLIs, SLOs, alerting rules and alerts routing according to the
[OpenSLO](https://github.com/OpenSLO/OpenSLO) specification.

## Description
The current goals of `osko` are:

- [ ] To connect to and work with the following metrics datasources using [kind: Datasource](https://github.com/OpenSLO/OpenSLO#datasource)
    - [ ] Mimir
    - [ ] Cortex
- [ ] Understand required (baseline) of the [kind: SLI](https://github.com/OpenSLO/OpenSLO#sli) and [kind: SLO](https://github.com/OpenSLO/OpenSLO#slo)
        specs. There is not yet a clear definition of which ones.
- [ ] Be able to set up recording rules according to the data received through the [kind: SLI](https://github.com/OpenSLO/OpenSLO#sli) resources, either
        in the Mimir or Cortex ruler
- [ ] Set up alerts in Alertmanager based on kinds [kind: AlertPolicy](https://github.com/OpenSLO/OpenSLO#alertpolicy) and
        [kind: AlertCondition](https://github.com/OpenSLO/OpenSLO#alertcondition)

### Getting Started
You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

#### Running on the cluster
1. Install Instances of Custom Resources:

```sh
kubectl apply -f config/samples/
```

2. Build and push your image to the location specified by `IMG`:

```sh
make docker-build docker-push IMG=<some-registry>/osko:tag
```

3. Deploy the controller to the cluster with the image specified by `IMG`:

```sh
make deploy IMG=<some-registry>/osko:tag
```

#### Uninstall CRDs
To delete the CRDs from the cluster:

```sh
make uninstall
```

#### Undeploy controller
UnDeploy the controller from the cluster:

```sh
make undeploy
```

## Contributing
As of right now, we don't have any specific guidelines for contributors.

Feel free to open an issue or a pull (merge) request if
you would like to see a particular feature implemented.

The only thing that's required right now to get your code merged is signing your commits off with the `-s` flag during `git commit`
after reading the project's [DCO](DCO).

It would also be greatly appreciated if you tried using
[Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) for the commit style, but that's a detail :)

#### How it works
This project aims to follow the Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/).

It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/),
which provide a reconcile function responsible for synchronizing resources until the desired state is reached on the cluster.

#### Test It Out
1. Install the CRDs into the cluster:

```sh
make install
```

2. Run your controller (this will run in the foreground, so switch to a new terminal if you want to leave it running):

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

## License

For license, see the LICENSE file in the root of this repository.
