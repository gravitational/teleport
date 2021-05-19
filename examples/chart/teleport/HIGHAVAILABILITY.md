## Configuring High Availability

Running multiple instances of the Authentication Services requires using a High Availability storage configuration.  The [documentation](https://gravitational.com/teleport/docs/admin-guide/#high-availability) provides detailed examples using AWS DynamoDB/S3, GCP Firestore/Google storage or an `etcd` cluster. Here we provide detailed steps for an AWS example configuration.

### Prerequisites
 - Available AWS credentials (/home/<user>/.aws/credentials)
 - Have AWS credentials that can create and access DyanmoDB details as documented here.
 - Bucket for storing Sessions as documented [here](https://gravitational.com/teleport/docs/admin-guide/#using-amazon-s3).

### Example High Availability Storage
1. First add the credentials file as a secret
```bash
kubectl create secret generic awscredentials --from-file=~/.aws/credentials
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

3. In the `values.yaml` set the volume and volume mount for AWS credentials only available to the auth service.
```yaml
extraAuthVolumes:
  - name: awscredentials
    secret:
      secretName: awscredentials


  # - name: ca-certs
  #   configMap:
  #     name: ca-certs
extraAuthVolumeMounts:
  - mountPath: /root/.aws
    name: awscredentials
```



### Configuring Multiple Instances of Teleport

A High Availability deployment of Teleport will typically have at least 2 proxy and 2 auth service instances.  SSH service is typically not enabled on these instances.  To enable separate deployments of the auth and auth services follow these steps.

1. In the configuration section set the `highAvailability` to true.  Also confirm the auth public address and Service Type.
```yaml
  highAvailability: true
  # High availability configuration with proxy and auth servers. No SSH configured service.
  proxyCount: 2
  authCount: 2
  authService:
    type: ClusterIP
  auth_public_address: auth.example.com
```
2. Set the connection for the proxies to connect to the auth service in the config section. The auth service is available at the Kubernetes service name and the public address setting.  So if you deploy an app named `myexample` then the auth service will be available in the Cluster at `myexampleauth` in addition to the public address.

```yaml
  auth_service_connection:
    auth_token: dogs-are-much-nicer-than-cats
    auth_servers:
    - teleportauth:3025
    - teleport.example.com:3025
```
### Confirming

After configuring both of these options run the install.  In the example below you will see two teleport pods that are the Proxy instances (`teleport-`) and two teleport pods that that are the Auth instances (`teleportauth-`).

``` bash
$ helm install teleport ./

$ kubectl get pods
NAME                            READY   STATUS    RESTARTS   AGE
teleport-d67584df8-8vfls        1/1     Running   0          62m
teleport-d67584df8-p9l2g        1/1     Running   0          62m
teleportauth-66455f85ff-7x7g4   1/1     Running   0          62m
teleportauth-66455f85ff-hgsdj   1/1     Running   0          62m
```
