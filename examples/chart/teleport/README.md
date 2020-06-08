# Teleport

[Gravitational Teleport](https://github.com/gravitational/teleport) is a modern SSH/Kubernetes API proxy server for remotely accessing clusters of Linux containers and servers via SSH, HTTPS, or Kubernetes API.  Community and Enterprise versions of Teleport are available.  You can start with the Community edition with this Chart and in the future update to an Enterprise version for the same deployment.

## Introduction

This chart deploys Teleport Community or Enterprise components to your cluster via a Kubernetes `Deployment`.

By default this chart is configured as follows:

- Enterprise Edition of Teleport
- 1 instance (replica) of Teleport
- Directory Storage with no persistent storage.  
- Record ssh/k8s exec and attach session to the `emptyDir` of the Teleport pod
- The assumed externally accessible hostname of Teleport is `teleport.example.com` 
- Teleport is accessible only from within your cluster. You need `kubectl port-forward` for external access. Change the Service type in `values.yaml` to options such as LoadBalancer to make it externally accessible.
- TLS is enabled by default on the Proxy 


The `values.yaml` is configurable for multiple options including:
- Using the Community edition of Teleport (Set license.enabled to false)
- Using self-signed TLS certificates (Set proxy.tls.usetlssecret to false)
- Using a specific version of Teleport (See image.tag)
- Using persistent or high availability storage (See below example).  Persistent or high availability storage is recommended for production usage. 
- Increasing the replica count for multiple instances (Using High Availability configuration)

See the comments in the default `values.yaml` and also the [Teleport documentation](https://gravitational.com/teleport/docs/) for more options.   

## Prerequisites

- Kubernetes 1.10+
- Teleport license for Enterprise deployments
- TLS Certificates or optionally use self-signed certificates

### Prepare a Teleport Enterprise license file


If you are deploying the Enterprise version you will require the license file as a secret available to Teleport. To use the community edition of Teleport simply set `enabled: false` under the `license:` settings in `values.yaml`.

Download the `license.pem` from the Teleport dashboard, and then <b>rename it to the filename</b> that this chart expects:

```
cp ~/Downloads/license.pem license-enterprise.pem
```

Store it as a Kubernetes secret:

```console
kubectl create secret generic license --from-file=license-enterprise.pem
```

## TLS Certificates

### Certificate Usage Configuration
Teleport can generate self-signed certificates that is useful for first time or non-production deployments. You can set Teleport to use self-signed certificates by setting `usetlssecret: false` under the `proxy.tls settings` in `values.yaml`. You will need to add `--insecure` to some interactions such as `tsh` and browser interaction will require confirming interaction.  Please see our [article](https://gravitational.com/blog/letsencrypt-teleport-ssh/) on generating certificates via Let's Encrypt.

If you plan to have TLS terminate at a seperate load balancer  you should set both `proxy.tls.enabled` and `proxy.usetlssecret` to false. 


### Adding TLS Certificates
You can provide the signed TLS certificates and optionally the TLS Certificate Authority (CA) that signed these certificates.
In order to instruct the proxy to use the TLS assets brought by you, prepare the following files:

- Your proxy server cert named `proxy-server.pem`
- Your proxy server key named `proxy-server-key.pem`
- Your TLS CA cert named `ca.pem`  (Optional. Update the value.yaml extraVars, extraVolumes and extraVolumeMounts to use this)

Then run:

```
$ kubectl create secret tls tls-web --cert=proxy-server.pem --key=proxy-server-key.pem

# Run this command if you are providing your own TLS CA
$ kubectl create configmap ca-certs --from-file=ca.pem
```
## Installing the chart

To install the chart with the release name `teleport`, run:

```
$ helm install --name teleport ./
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


## Configuring High Availability and Multiple Replicas

Running multiple instances of the Authentication Services requires using a high availability storage configuration.  The [documentation](https://gravitational.com/teleport/docs/admin-guide/#high-availability) provides detailed examples on AWS, ETCD and GCP options. Here we provide detailed steps for a AWS example configuration.

### Prerequisites
 - Available AWS credentials (/home/<user>/.aws/credentials)
 - Have AWS credentials that can create and access DyanmoDB details as documented here.
 - Bucket for storing Sessions as documented [here](https://gravitational.com/teleport/docs/admin-guide/#using-amazon-s3).

### Configuring AWS Credentials and Storage
1. First add the credentials file as a secret
```bash
kubectl create secret generic awscredentials --from-file=credentials
```

2. Set the DynamoDB and Sessions storage within `values.yaml`

```yaml
    storage:
      type: dynamodb
      region: us-east-1
      table_name: teleport
      audit_events_uri: 'dynamodb://teleport_events'
      audit_sessions_uri: 's3://teleportexample/sessions?region=us-east-1'
```

3. With the `values.yaml` set the volume and volume mount for AWS credentials
```yaml
extraVolumes:
  - name: awscredentials
    secret:
      secretName: awscredentials


  # - name: ca-certs
  #   configMap:
  #     name: ca-certs
extraVolumeMounts:
  - mountPath: /root/.aws
    name: awscredentials
```



### Configuring Multiple Instances of Teleport



## Troubleshooting

### Teleport Pods are not starting

If you the Teleport pods are not starting the most common issue is lack of required volumes (license, TLS certificates).  If you run `kubectl get pods` and the <chart-name>-hostid pod shows as not running that could be the issue.  Run a describe on the pod to see if there are any missing secrets or configurations.
 Example:
   `kubectl describe pod teleport-5f5f989b96-9khzq`

  
### Teleport Pods keep restarting with Error
The issue may be due to a malformed Teleport configuration file or other configuration issue.  Use the kubectl logs command to see the logs output Example: `kubectl logs -f teleport-5f5f989b96-9khzq` .


## Contributing

### Building the cli yourself

```console
$ git clone git@github.com:gravitational/teleport.git ~/go/src/github.com/gravitational/teleport
cd $_

$ make full

$ build/tsh --proxy=teleport.example.com --auth=local --user=admin login
```
