# Teleport Agent chart

This chart is a Teleport agent used to register any or all of the following services
with an existing Teleport cluster:
- Teleport Kubernetes access
- Teleport Application access
- Teleport Database access
- Teleport Kubernetes App Discovery
- Teleport Jamf service

To use it, you will need:
- an existing Teleport cluster (at least proxy and auth services)
- a reachable proxy endpoint (`$PROXY_ENDPOINT` e.g. `teleport.example.com:3080` or `teleport.example.com:443`)
- a reachable reverse tunnel port on the proxy (e.g. `teleport.example.com:3024`). The address is automatically
  retrieved from the Teleport proxy configuration.
- a join token for the Teleport Cluster. For this Teleport cluster (`$JOIN_TOKEN`) is used by default. See the [Join Methods and Token Reference](https://goteleport.com/docs/reference/join-methods/) for supported join methods and creating tokens.

## Combining roles

You can combine multiple roles as a comma-separated list: `--set roles=kube\,db\,app\,discovery`

Note that commas must be escaped if the values are provided on the command line. This is due to the way that
Helm parses arguments.

You must also provide the settings for each individual role which is enabled as detailed below.

## Backwards compatibility

To provide backwards compatibility with older versions of the `teleport-kube-agent` chart, if you do
not specify any value for `roles`, the chart will run with only the `kube` role enabled.

## Kubernetes access

To use Teleport Kubernetes access, you will also need:
- to choose a name for your Kubernetes cluster, distinct from other registered
  clusters (`$KUBERNETES_CLUSTER_NAME`)

To install the agent, run:

```sh
$ helm install teleport-kube-agent . \
  --create-namespace \
  --namespace teleport \
  --set roles=kube \
  --set proxyAddr=${PROXY_ENDPOINT?} \
  --set authToken=${JOIN_TOKEN?} \
  --set kubeClusterName=${KUBERNETES_CLUSTER_NAME?}
```

Set the values in the above command as appropriate for your setup.

You can also optionally set labels for your Kubernetes cluster using the
format `--set "labels.key=value"` - for example: `--set "labels.env=development,labels.region=us-west-1"`

To avoid specifying the auth token in plain text, it's possible to create a secret containing the token beforehand. To do so, run:

```sh
export TELEPORT_KUBE_TOKEN=`<auth token> | base64 -w0`
export TELEPORT_NAMESPACE=teleport

cat <<EOF > secrets.yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: teleport-kube-agent-join-token
  namespace: ${TELEPORT_NAMESPACE?}
type: Opaque
data:
  auth-token: ${TELEPORT_KUBE_TOKEN?}
EOF

$ kubectl apply -f secret.yaml

$ helm install teleport-kube-agent . \
  --create-namespace \
  --namespace ${TELEPORT_NAMESPACE?} \
  --set roles=kube \
  --set proxyAddr=${PROXY_ENDPOINT?} \
  --set kubeClusterName=${KUBERNETES_CLUSTER_NAME?}
```

Note that due to backwards compatibility, the `labels` value **only** applies to the Teleport
Kubernetes service. To set labels for applications or databases, use the different formats
detailed below.

## Application access

### Dynamic Registration mode

To use Teleport application access in [dynamic registration mode](https://goteleport.com/docs/enroll-resources/application-access/guides/dynamic-registration/),
you will need to know the application resource selector. (`$APP_RESOURCE_KEY` and `$APP_RESOURCE_VALUE`)

To listen for all application resources, set both variables to `*`.

To install the agent in dynamic application registration mode, run:
```sh
$ helm install teleport-kube-agent . \
--create-namespace \
--namespace teleport \
--set roles=app \
--set proxyAddr=${PROXY_ENDPOINT?} \
--set authToken=${JOIN_TOKEN?} \
--set "appResources[0].labels.${APP_RESOURCE_KEY?}=${APP_RESOURCE_VALUE?}"
```

### Manual configuration mode

To use Teleport Application access, you will also need:
- the name of an application that you would like to proxy (`$APP_NAME`)
- the URI to connect to the application from the node where this chart is deployed (`$APP_URI`)

To install the agent, run:

```sh
$ helm install teleport-kube-agent . \
  --create-namespace \
  --namespace teleport \
  --set roles=app \
  --set proxyAddr=${PROXY_ENDPOINT?} \
  --set authToken=${JOIN_TOKEN?} \
  --set "apps[0].name=${APP_NAME?}" \
  --set "apps[0].uri=${APP_URI?}"
```

Set the values in the above command as appropriate for your setup.

These are the supported values for the `apps` map:

| Key                    | Description                                                              | Example                                                    | Default | Required |
| ---------------------- | ------------------------------------------------------------------------ | ---------------------------------------------------------- | ------- | -------- |
| `name`                 | Name of the app to be accessed                                           | `apps[0].name=grafana`                                     |         | Yes      |
| `uri`                  | URI of the app to be accessed                                            | `apps[0].uri=http://localhost:3000`                        |         | Yes      |
| `public_addr`          | Public address used to access the app                                    | `apps[0].public_addr=grafana.teleport.example.com`         |         | No       |
| `labels.[name]`        | Key-value pairs to set against the app for grouping/RBAC                 | `apps[0].labels.env=local,apps[0].labels.region=us-west-1` |         | No       |
| `insecure_skip_verify` | Whether to skip validation of TLS certificates presented by backend apps | `apps[0].insecure_skip_verify=true`                        | `false` | No       |
| `rewrite.redirect`     | A list of URLs to rewrite to the public address of the app service       | `apps[0].rewrite.redirect[0]=https://192.168.1.1`          |         | No       |

You can add multiple apps using `apps[1].name`, `apps[1].uri`, `apps[2].name`, `apps[2].uri` etc.

After installing, the new application should show up in `tsh apps ls` after a few minutes.

## Database access

### Dynamic Registration mode

To use Teleport database access in [dynamic registration mode](https://goteleport.com/docs/enroll-resources/database-access/guides/dynamic-registration/),
you will need to know the database resource selector. (`$DB_RESOURCE_KEY` and `$DB_RESOURCE_VALUE`)

To listen for all database resources, set both variables to `*`.

To install the agent in dynamic database registration mode, run:
```sh
$ helm install teleport-kube-agent . \
--create-namespace \
--namespace teleport \
--set roles=db \
--set proxyAddr=${PROXY_ENDPOINT?} \
--set authToken=${JOIN_TOKEN?} \
--set "databaseResources[0].labels.${DB_RESOURCE_KEY?}=${DB_RESOURCE_VALUE?}"
```

### Auto-discovery mode (AWS)

To use Teleport database access in AWS database auto-discovery mode, you will also need:
- the database types you are attempting to auto-discover (`types`)
- the AWS region(s) you would like to run auto-discovery in (`regions`)
- the AWS resource tags if you want to target only certain databases (`tags`)

See the [AWS databases Helm chart reference](https://goteleport.com/docs/reference/helm-reference/teleport-kube-agent/#awsDatabases)
for an example of installing an agent with AWS database auto-discovery.

### Auto-discovery mode (Azure)

To use Teleport database access in Azure database auto-discovery mode, you will also need:
- the database types you are attempting to auto-discover (`types`)
- the Azure resource tags if you want to target only certain databases (`tags`)

You can optionally specify:
- the Azure subscription(s) to auto-discover in (`subscriptions`)
- the Azure region(s) to auto-discover in (`regions`)
- the Azure resource-group(s) to auto-discover in (`resource_groups`)

The default for each of these optional settings is `[*]`, which will auto-discover in all
subscriptions, regions, or resource groups accessible by the Teleport service
principal in Azure.

See the [Azure databases Helm chart reference](https://goteleport.com/docs/reference/helm-reference/teleport-kube-agent/#azureDatabases)
for an example of installing an agent with Azure database auto-discovery.

### Manual configuration mode

To use Teleport database access, you will also need:
- the name of an database that you would like to proxy (`$DB_NAME`)
- the URI to connect to the database from the node where this chart is deployed (`$DB_URI`)
- the database protocol used for the database (`$DB_PROTOCOL`)

To install the agent in manual database configuration mode, run:

```sh
$ helm install teleport-kube-agent . \
  --create-namespace \
  --namespace teleport \
  --set roles=db \
  --set proxyAddr=${PROXY_ENDPOINT?} \
  --set authToken=${JOIN_TOKEN?} \
  --set "databases[0].name=${DB_NAME?}" \
  --set "databases[0].uri=${DB_URI?}" \
  --set "databases[0].protocol=${DB_PROTOCOL?}"
```

Set the values in the above command as appropriate for your setup.

These are the supported values for the `databases` map:

| Key             | Description                                                   | Example                                                                            | Default | Required |
| --------------- | ------------------------------------------------------------- | ---------------------------------------------------------------------------------- | ------- | -------- |
| `name`          | Name of the database to be accessed                           | `databases[0].name=aurora`                                                         |         | Yes      |
| `uri`           | URI of the database to be accessed                            | `databases[0].uri=postgres-aurora-instance-1.xxx.us-east-1.rds.amazonaws.com:5432` |         | Yes      |
| `protocol`      | Database protocol                                             | `databases[0].protocol=postgres`                                                   |         | Yes      |
| `description`   | Free-form description of the database proxy instance          | `databases[0].description='AWS Aurora instance of PostgreSQL 13.0'`                |         | No       |
| `aws.region`    | AWS-specific region configuration (only used for RDS/Aurora)  | `databases[0].aws.region=us-east-1`                                                |         | No       |
| `labels.[name]` | Key-value pairs to set against the database for grouping/RBAC | `databases[0].labels.db=postgres-dev,apps[0].labels.region=us-east-1`              |         | No       |

You can add multiple databases using `databases[1].name`, `databases[1].uri`, `databases[1].protocol`,
`databases[2].name`, `databases[2].uri`, `databases[2].protocol` etc.

After installing, the new database should show up in `tsh db ls` after a few minutes.

## Kubernetes App Discovery

Teleport can be used to automatically discover apps based on services found in the Kubernetes cluster.
To run Teleport discovery you will need to enabled roles `discovery` and `app` and also provide token that allows access for these roles.

To install the agent, run:

```sh
$ helm install teleport-kube-agent . \
  --create-namespace \
  --namespace teleport \
  --set roles=kube,app,discovery \
  --set proxyAddr=${PROXY_ENDPOINT?} \
  --set authToken=${JOIN_TOKEN?}
```

With default settings Teleport will try to discovery all apps available in the cluster. To control what namespaces and what service labels
to use for discovery you can use `kubernetesDiscovery` property of the chart.

When discovery is running, `kubeClusterName` should be set in values, since it is used as a name for discovery field and as a target label
for the app service, so it can expose discovered apps.

## Jamf service

To use [Teleport Jamf service](https://goteleport.com/docs/access-controls/device-trust/jamf-integration/), 
you will also need:
- provide your Jamf Pro API endpoint
- provide your Jamf Pro API credentials

To install the agent with Jamf API credentials, run:

```sh
$ helm install teleport-kube-agent . \
  --create-namespace \
  --namespace teleport \
  --set roles=jamf \
  --set proxyAddr=${PROXY_ENDPOINT?} \
  --set authToken=${JOIN_TOKEN?} \
  --set jamfApiEndpoint=${JAMF_API_ENDPOINT?} \
  --set jamfClientId=${JAMF_CLIENT_ID?} \
  --set jamfClientSecret=${JAMF_CLIENT_SECRET?}
```

Set the values in the above command as appropriate for your setup.

The Helm chart will install Secrets by default. To avoid specifying the Jamf API credentials in plain text, it's possible to create a secret containing the password beforehand. To do so, run:

```sh
export JAMF_CLIENT_SECRET=`<jamf client secret> | base64 -w0`
export JAMF_SECRET_NAME=teleport-jamf-api-credentials
export TELEPORT_NAMESPACE=teleport

cat <<EOF > secrets.yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: ${JAMF_SECRET_NAME}
  namespace: ${TELEPORT_NAMESPACE?}
type: Opaque
data:
  jamfSecret: ${JAMF_CLIENT_SECRET?}
EOF

$ kubectl apply -f secret.yaml

$ helm install teleport-kube-agent . \
  --create-namespace \
  --namespace ${TELEPORT_NAMESPACE?} \
  --set roles=jamf \
  --set proxyAddr=${PROXY_ENDPOINT?} \
  --set authToken=${JOIN_TOKEN?} \
  --set jamfApiEndpoint=${JAMF_API_ENDPOINT?} \
  --set jamfClientId=${JAMF_CLIENT_ID?} \
  --set jamfCredentialsSecret.name=${JAMF_SECRET_NAME?} \
  --set jamfCredentialsSecret.create=false
```

## Troubleshooting

If the service for a given role doesn't show up, look into the agent logs with:

```sh
$ kubectl logs -n teleport deployment/teleport-kube-agent
```

## Contributing to the chart

Please read [CONTRIBUTING.md](../CONTRIBUTING.md) before raising a pull request to this chart.
