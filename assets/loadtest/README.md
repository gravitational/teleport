# loadtest

Automation for the `loadtest` kuberentes cluster and performing Teleport load tests.

## About

This automation sets up a kubernetes cluster named `loadtest` in the configured GCP project.  
This cluster is used for, among other things, the `1k`/`10k` scaling tests that are performed as part of
Teleports manual release test plan.

## Setup

### Prerequisites
- Make sure you have the following tools installed:
    - `terraform`
    - `gcloud`
    - `kubectl`
- Make sure that you have a GCP service account key with `Compute Admin`, `Compute Network Admin`,
      `Kubernetes Engine Admin`, `Kubernetes Engine Cluster Admin`, and `Service Account User`
  - To authenticate as the service account follow these [instructions](https://cloud.google.com/docs/authentication/production)
- Make sure you have reserved static ip addresses for the proxy
  - This only needs to be done once per GCP project, see the [network docs](./network/README.md) for details

### Creating the Cluster

First create a cluster, if you are running this automation for the first time, you may be asked to run
`terraform init` from the cluster directory before continuing. To resize the cluster, edit [`terraform.tfvars`](cluster/terraform.tfvars) as needed.

```bash
$ make create-cluster
```

### DNS Entries
Before deploying anything to the cluster you first need to set `PROXY_HOST`. These variables should
be the DNS names to be used for the [`proxy`](./k8s/proxy.yaml). When everything is successfully deployed you should be able
to navigate to `https://PROXY_HOST:3080` in your browser.

```bash
$ export PROXY_HOST=proxy.loadtest.com 
```

### TLS Certificates

Certificates can be provisioned automatically via [cert-manager](https://cert-manager.io/) or by hand.

#### cert-manager
If you would like to use cert-manager to automatically retrieve TLS certificates for you, create 
[`cetificate.yaml`](./k8s/certificate.yaml) with your `cert-manager.io/v1/ClusterIssuer`, `cert-manager.io/v1/Certificate` and any
secrets required for your solver.

#### Kubernetes secret

To manual supply TLS certificates create a tls secret, run the following:

```bash
$ kubectl create secret tls teleport-tls -n loadtest \
    --cert=path/to/cert/file \
    --key=path/to/key/file
```

You **must** also provide `USE_CERT_MANAGER=no` to all make commands below.

### Teleport Configuration
You must supply an [OIDC Connector](https://goteleport.com/docs/enterprise/sso/oidc/) that will be used for authentication. Create [`oidc.yaml`](./teleport/oidc.yaml)
before attempting to deploy Teleport to the cluster.


## Deploy Teleport

### etcd Backend

```bash
$ make deploy-etcd-cluster
```

### Firestore Backend

To use the firestore backend you must have a GCP service account key with `Cloud Datastore User`, `Cloud Datastore Index Admin`
`Storage Object Creator`, `Storage Object Viewer`. Set `GCP_CREDS_LOCATION` to the location that you saved the service account key.

```bash
$ export GCP_CREDS_LOCATION=/path/to/service/account/key
$ make deploy-firestore-cluster
```

## Running Tests

To run soak tests:

```bash
$ make run-soak-tests
```


**Note:** You must have enough nodes in the cluster to run the following tests. Ensure your `node_count` in [`terraform.tfvars`](cluster/terraform.tfvars) is correctly set.

To run the 10k node scaling tests:

```bash
$ make run-scaling-test
```

To run the trusted cluster scaling test:
```bash
$ make run-tc-scaling-test
```

## Cleanup

To delete the loadtest deployment:
```bash
$ make delete-deploy
```


To delete the entire cluster:
```bash
$ make delete-cluster
```