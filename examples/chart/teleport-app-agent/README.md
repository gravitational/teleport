# Teleport App Service Agent

This chart is a minimal Teleport agent used to register a Teleport app service
with an existing Teleport cluster.

To use it, you will need:
- an existing Teleport cluster (at least proxy and auth services)
- a reachable proxy endpoint (`$PROXY_ENDPOINT`)
- a [static join
  token](https://goteleport.com/teleport/docs/admin-guide/#adding-nodes-to-the-cluster)
  for this Teleport cluster (`$JOIN_TOKEN`)
  - this chart does not currently support dynamic join tokens; please [file an
    issue](https://github.com/gravitational/teleport/issues/new?labels=type%3A+feature+request&template=feature_request.md)
    if you require support for dynamic tokens
- the name of an application that you would like to proxy (`$APP_NAME`)
- the URI to connect to the application from the node where this chart is deployed (`$APP_URI`)

To install the agent, run:

```sh
$ helm install teleport-app-agent . \
  --create-namespace \
  --namespace teleport \
  --set proxyAddr=${PROXY_ENDPOINT?} \
  --set authToken=${JOIN_TOKEN?} \
  --set "apps[0].name=${APP_NAME?}" \
  --set "apps[0].uri=${APP_URI?}"
```

Set the values in the above command as appropriate for your setup.

These are the supported values for the `apps` map:

| Key | Description | Example | Default | Required |
| --- | --- | --- | --- | --- |
| `name` | Name of the app to be accessed | `apps[0].name=grafana` | | Yes |
| `uri` | URI of the app to be accessed | `apps[0].uri=http://localhost:3000` | | Yes |
| `public_addr` | Public address used to access the app | `apps[0].public_addr=grafana.teleport.example.com` | | No |
| `labels.[name]` | Key-value pairs to set against the app for grouping/RBAC | `apps[0].labels.env=local,apps[0].labels.region=us-west-1` | | No |
| `insecure_skip_verify` | Whether to skip validation of TLS certificates presented by backend apps | `apps[0].insecure_skip_verify=true` | `false` | No |
| `rewrite.redirect` | A list of URLs to rewrite to the public address of the app service | `apps[0].rewrite.redirect[0]=https://192.168.1.1` | | No

You can add multiple apps using `apps[1].name`, `apps[1].uri`, `apps[2].name`, `apps[2].uri` etc.

After installing, the new application should show up in `tsh apps ls` after a few
minutes. If the new cluster doesn't show up, look into the agent logs with:

```sh
$ kubectl logs -n teleport deployment/teleport-app-agent
```
