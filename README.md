# capi-secretsyncer

**capi-secretsyncer** is a lightweight Kubernetes controller that automatically synchronises Secrets from a Cluster API (CAPI) management cluster into tenant (workload) clusters.

## Overview

In a Cluster API setup, the management cluster holds credentials, certificates, and other sensitive configuration as Kubernetes Secrets. These Secrets often need to be available inside the tenant clusters as well — for example to configure cluster add-ons, storage backends, or monitoring agents.

`capi-secretsyncer` watches Secrets on the management cluster and, whenever a new Secret appears, pushes a copy of it into the corresponding tenant cluster. It locates the tenant cluster by reading a well-known Secret called `tenant-kubeconfig` (key: `kubeconfig`) that must exist in the same namespace as the source Secret.

## How it works

1. The controller starts an informer on the management cluster and listens for `Secret` add events.
2. For each new Secret it looks up `tenant-kubeconfig` in the same namespace to obtain the tenant cluster's kubeconfig.
3. It creates an identical Secret (same name, namespace, type, and data) inside the tenant cluster.

```
Management Cluster                        Tenant Cluster
┌─────────────────────────────┐           ┌──────────────────────────┐
│  Namespace: my-cluster      │           │  Namespace: my-cluster   │
│  ┌───────────────────────┐  │  syncs    │  ┌────────────────────┐  │
│  │ Secret: my-secret     │──┼──────────▶│  │ Secret: my-secret  │  │
│  └───────────────────────┘  │           │  └────────────────────┘  │
│  ┌───────────────────────┐  │           └──────────────────────────┘
│  │ Secret:               │  │
│  │  tenant-kubeconfig    │  │
│  └───────────────────────┘  │
└─────────────────────────────┘
```

## Prerequisites

- A running Kubernetes management cluster with Cluster API installed.
- A `tenant-kubeconfig` Secret in each namespace that should be synced, containing the key `kubeconfig` with a valid kubeconfig for the respective tenant cluster.
- The controller must run with sufficient RBAC permissions to read and list Secrets in the management cluster.

## Getting started

### Run in-cluster (recommended)

Deploy the controller into your management cluster. It uses the in-cluster configuration automatically:

```bash
kubectl apply -f deploy/
```

### Build from source

```bash
git clone https://github.com/eumel8/capi-secretsyncer.git
cd capi-secretsyncer
go build -o capi-secretsyncer .
```

### Container image

Pre-built images are published to the GitHub Container Registry:

```bash
docker pull ghcr.io/eumel8/capi-secretsyncer/capi-secretsyncer:<tag>
```

## Configuration

| Secret name         | Key          | Description                                              |
|---------------------|--------------|----------------------------------------------------------|
| `tenant-kubeconfig` | `kubeconfig` | Kubeconfig for the tenant cluster in the same namespace  |

The controller resync period is set to **10 minutes** by default.

## Development

```bash
# Run unit tests
go test ./...

# Build binary
go build -o capi-secretsyncer .
```

## License

Apache 2.0
