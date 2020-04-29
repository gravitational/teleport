# Teleport

[Gravitational Teleport](https://github.com/gravitational/teleport) is a modern SSH/Kubernetes API proxy server for
remotely accessing clusters of Linux containers and servers via SSH, HTTPS, or Kubernetes API.

## Introduction

This chart deploys Teleport components to your cluster via a Kubernetes `Deployment`.

By default this chart is configured as follows:

- 1 replica
- Record ssh/k8s exec and attach session to the `emptyDir` of the Teleport pod
  - These sessions will also be stored on the root cluster, if used to access this Helm-configured cluster remotely
- The assumed externally accessible hostname of Teleport is `teleport.example.com`
- Teleport will be deployed using a Kubernetes LoadBalancer
  - This is a requirement for a trusted cluster setup
- TLS is enabled by default on the Proxy

See the comments in the default `values.yaml` and also the Teleport documentation for more options.

### Setting up your trusted cluster configuration

This version of the chart has been modified to automatically connect back to a "root" Teleport cluster when started. This
enables remote access to and management of this Teleport cluster (the "leaf" cluster) when deployed on a customer site.

You will need to edit the values in the `trustedCluster.extraVars` section of `values.yaml` as appropriate for your cluster.
There are comments in the file describing what the values need to be set to.

## Prerequisites

- Helm v2.16+
- Kubernetes 1.10+
- A Teleport license file stored as a Kubernetes Secret object - see below
- Valid TLS private key and certificate chain which validates against a known root CA, stored in the `tls-web` secret.
  - Private key should be called `privkey.pem`
  - Certificate chain should be called `fullchain.pem`
  - Providers like Let's Encrypt are good for providing these certificates.

### Prepare the license file

Download the `license.pem` from the Teleport dashboard, and then rename it to the filename that this chart expects:

```
cp ~/Downloads/license.pem license-enterprise.pem
```

Store it as a Kubernetes secret:

```console
kubectl create secret generic license --from-file=license-enterprise.pem
```

### Prepare the tls-web secret

You will need to  issue valid TLS certificates for the public-facing name of this cluster.

You can do this with a provider like Let's Encrypt. This is an example of how to do this using the `certbot` client.

Install `certbot`:

```console
# RHEL/CentOS
$ yum -y install certbot

# Debian/Ubuntu
$ apt-get -y install certbot

# Generic installation via Python PIP
$ pip3 install certbot
```

Issue the certificate using certbot in manual mode:

```console
$ certbot -d teleport.example.com --manual --logs-dir . --config-dir . --work-dir . --preferred-challenges dns certonly
```

Add the certificate and private key as a Kubernetes secret:

```console
$ kubectl create secret tls tls-web --cert=fullchain.pem --key=privkey.pem
```
#### Important information

In manual mode, you will need to configure a DNS TXT record on the DNS provider for `teleport.example.com` so that the
checks will validate. Certificates issued by LetsEncrypt expire every 90 days, so the renewal process for these certificates
will need to be automated.

**WARNING**: If your certificates expire, connections from the root cluster to this leaf cluster will stop working.

Another good option for automatically issuing certificates inside a cluster is [cert-manager](https://github.com/jetstack/cert-manager).
Teleport needs access to the generated private key and certificate chain itself. It can't easily be used with a Kubernetes `Ingress`.

## Installing the chart

To install the chart with the release name `teleport`, run:

```
$ helm install --name teleport ./
```