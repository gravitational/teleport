# Teleport Cluster

This chart sets up a Teleport cluster composed of at least 1 Proxy instance
and 1 Auth instance. When applicable, the chart will default to 2 pods to
provide high-availability.

## Important Notices

- The chart version follows the Teleport version. e.g. chart v10.x can run Teleport v10.x and v11.x, but is not compatible with Teleport 9.x
- Teleport does mutual TLS to authenticate clients. Establishing mTLS through a L7
  LoadBalancer, like a Kubernetes `Ingress` [requires ALPN support](https://goteleport.com/docs/architecture/tls-routing/#working-with-layer-7-load-balancers-or-reverse-proxies).
  Exposing Teleport through a `Service` with type `LoadBalancer` is still recommended
  because its the most flexible and least complex setup.

## Getting Started

### Single-node example

To install Teleport in a separate namespace and provision a web certificate using Let's Encrypt, run:

```bash
$ helm install teleport/teleport-cluster \
    --set acme=true \
    --set acmeEmail=alice@example.com \
    --set clusterName=teleport.example.com\
    --create-namespace \
    --namespace=teleport-cluster \
    ./teleport-cluster/
```

Finally, configure the DNS for `teleport.example.com` to point to the newly created LoadBalancer.

Note: this guide uses the built-in ACME client to get certificates.
In this setup, Teleport nodes cannot be replicated. If you want to run multiple
Teleport replicas, you must provide a certificate through `tls.existingSecretName`
or by installing [cert-manager](https://cert-manager.io/docs/) and setting the `highAvailability.certManager.*` values.

### Replicated setup guides

- [Running an HA Teleport cluster in Kubernetes using an AWS EKS Cluster](https://goteleport.com/docs/admin-guides/deploy-a-cluster/helm-deployments/aws/)
- [Running an HA Teleport cluster in Kubernetes using an Google Cloud GKE cluster](https://goteleport.com/docs/admin-guides/deploy-a-cluster/helm-deployments/gcp/)
- [Running an HA Teleport cluster in Kubernetes using an Azure AKS cluster](https://goteleport.com/docs/admin-guides/deploy-a-cluster/helm-deployments/azure/)
- [Running a Teleport cluster in Kubernetes with a custom Teleport config](https://goteleport.com/docs/admin-guides/deploy-a-cluster/helm-deployments/custom/)

### Creating first user

The first user can be created by executing a command in one of the auth pods.

```code
kubectl exec it -n teleport-cluster statefulset/teleport-cluster-auth -- tctl users add my-username --roles=editor,auditor,access
```

The command should output a registration link to finalize the user creation.

## Uninstalling

```bash
helm uninstall --namespace teleport-cluster teleport-cluster
```

## Documentation

See https://goteleport.com/docs/admin-guides/deploy-a-cluster/helm-deployments/ for guides on setting up HA Teleport clusters
in EKS or GKE, plus a comprehensive chart reference.

## Contributing to the chart

Please read [CONTRIBUTING.md](../CONTRIBUTING.md) before raising a pull request to this chart.
