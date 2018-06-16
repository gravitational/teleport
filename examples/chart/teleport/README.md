# Teleport

[Gravitational Teleport](https://github.com/gravitational/teleport) is a modern SSH/Kubernetes API proxy server for remotely accessing clusters of Linux containers and servers via SSH, HTTPS, or Kubernetes API.

## Introduction

This chart deploys Teleport components to your cluster via a Kubernetes `Deployment`.

By default this chart is configured as follows:

- 1 replica
- Record ssh/k8s exec and attach session to the `emptyDir` of the Teleport pod
- The assumed externally accessible hostname of Teleport is `teleport.example.com`
- Teleport is accessible only from within your cluster. You need `kubectl port-forward` for external access.
- TLS is enabled by default on the Proxy

See the comments in the default `values.yaml` and also the Teleport documentation for more options.

## Prerequisites

- Kubernetes 1.10+
- A Teleport license file stored as a Kubernetes Secret object(See below)

### Prepare the license file

Download the `license.pem` from the Teleport dashboard, and then rename it to the filename that this chart expects:

```
cp ~/Downloads/license.pem license-enterprise.pem
```

Store it as a Kubernetes secret:

```console
kubectl create secret generic license --from-file=license-enterprise.pem
```

## Installing the chart

To install the chart with the release name `teleport`, run:

```
$ helm install --name teleport ./
```

Teleport proxy generates a TLS key and a cert of its own by default.
In order to instruct the proxy to use the TLS assets brought by you, prepare the following files:

- Your CA cert named `ca.pem`
- Your proxy server cert named `proxy-server.pem`
- Your proxy server key named `proxy-server-key.pem`

Then run:

```
$ kubectl create secret tls tls-web --cert=proxy-server.pem --key=proxy-server-key.pem
$ kubectl create configmap ca-certs --from-file=ca.pem
$ helm upgrade --install --name teleport ./
```

## Running locally on minikube

Grab the test setup from the community project [teleport-on-minikube](http://github.com/mumoshu/teleport-on-minikube) and run:

```
path/to/teleport-on-minikube//scripts/install-on-minikube
```

Type your desired password, capture the barcode with your MFA device like Google Authenticator, type the OTP.

Now, you can run various `tsh` commands against your local Teleport installation via `teleport.example.com`:

```
$ tsh login --auth=local --user=$USER login
```

## Contributing

### Building the cli yourself

```console
$ git clone git@github.com:gravitational/teleport.git ~/go/src/github.com/gravitational/teleport
cd $_

$ make full

$ build/tsh --proxy=teleport.example.com --auth=local --user=admin login
```
