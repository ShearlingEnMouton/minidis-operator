# minidis-operator

A Kubernetes operator for [minidis](https://github.com/ShearlingEnMouton/minidis), a Redis-compatible server written in Rust. It adds a `Minidis` custom resource: creating one runs a minidis server with its persistent volume and Services, and the operator keeps them in line with the resource.

The operator is built with [Kubebuilder](https://book.kubebuilder.io/) and [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime). It is in early development: the API group is `minidis.shearlingenmouton.github.io/v1alpha1` and may still change.

## Requirements

- Go 1.26 or later
- Access to a Kubernetes cluster and `kubectl`
- Docker, to build the operator image

## Development

```sh
make manifests generate   # regenerate the CRD, RBAC and DeepCopy code after changing api/
make test                 # run the unit and envtest tests
make run                  # run the controller locally against the current kubeconfig context
```

`make install` installs the CRD into the cluster of the current kubeconfig context, and `make uninstall` removes it. `config/samples/` holds an example `Minidis` resource.

To run the operator inside the cluster:

```sh
make docker-build docker-push IMG=<registry>/minidis-operator:<tag>
make deploy IMG=<registry>/minidis-operator:<tag>
```

## License

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE).
