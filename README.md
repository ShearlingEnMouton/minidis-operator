# minidis-operator

A Kubernetes operator for [minidis](https://github.com/ShearlingEnMouton/minidis), a Redis-compatible server written in Rust. It adds a `Minidis` custom resource: creating one runs a minidis server with its persistent volume and Services, and the operator keeps them in line with the resource.

The operator is built with [Kubebuilder](https://book.kubebuilder.io/) and [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime). It is in early development: the API group is `minidis.shearlingenmouton.github.io/v1alpha1` and may still change.

## Installation

Each release publishes the operator image to `ghcr.io/shearlingenmouton/minidis-operator` and attaches an `install.yaml` manifest to the GitHub release. The manifest contains the CRD, the RBAC rules and the controller Deployment:

```sh
kubectl apply -f https://github.com/ShearlingEnMouton/minidis-operator/releases/download/v0.1.0/install.yaml
```

The controller runs in the `minidis-operator-system` namespace. Once it is running, create a `Minidis` resource as shown in [Usage](#usage).

Releases are created by pushing a `v*` tag: the release workflow builds and pushes the image, then builds `install.yaml` with `make build-installer` and attaches it to the release.

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

## Usage

A `Minidis` resource describes one minidis server:

```yaml
apiVersion: minidis.shearlingenmouton.github.io/v1alpha1
kind: Minidis
metadata:
  name: cache
spec:
  version: "1.0.0"
  storage:
    size: 2Gi
  resources:
    requests:
      cpu: 100m
      memory: 128Mi
  auth:
    name: cache-auth
    key: password
  fsync: everysec
```

| Field | Default | Description |
| --- | --- | --- |
| `version` | `1.0.0` | Tag of the `ghcr.io/shearlingenmouton/minidis` image to run. |
| `storage.size` | `2Gi` | Size of the persistent volume that holds the AOF. |
| `storage.storageClassName` | cluster default | StorageClass of the persistent volume. |
| `resources` | none | CPU and memory requests and limits of the minidis container. |
| `auth` | none | Secret key that holds the password clients must send with `AUTH`. Authentication is disabled when it is not set. |
| `fsync` | `everysec` | AOF fsync policy: `always`, `everysec` or `no`. |

`storage` cannot be changed after the resource is created, because the volume claim template of a StatefulSet is immutable. The other fields can be updated; the operator applies the change to the StatefulSet, which rolls the pod.

For a `Minidis` named `<name>`, the operator creates the following objects in the same namespace:

- a StatefulSet `<name>` with one replica, which mounts a persistent volume claim `minidis-data` at `/data`
- a Service `<name>` that exposes port 3131 inside the cluster
- a headless Service `<name>-headless`, which governs the StatefulSet and gives the pod a stable DNS name

These objects are owned by the `Minidis` resource and are deleted with it. The persistent volume claim is not owned by it, so the data is kept after the resource is deleted.

The status reports the number of ready replicas in `readyReplicas` and a `Ready` condition, which is `True` once the minidis pod is ready. `observedGeneration` is the generation of the spec that the status reflects.

`kubectl get minidis`, or the short name `kubectl get md`, lists the version, the readiness and the storage size of each resource:

```text
NAME             VERSION   READY   STORAGE   AGE
minidis-sample   1.0.0     True    2Gi       5m
```

## License

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE).
