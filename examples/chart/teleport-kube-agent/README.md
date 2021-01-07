# Teleport Kubernetes Agent

This chart is a minimal Teleport agent used to register a Kubernetes cluster
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
- choose a name for your Kubernetes cluster, distinct from other registered
  clusters (`$KUBERNETES_CLUSTER_NAME`)

To install the agent, run:

```sh
$ helm install teleport-kube-agent . \
  --create-namespace \
  --namespace teleport \
  --set proxyAddr=${PROXY_ENDPOINT?} \
  --set authToken=${JOIN_TOKEN?} \
  --set kubeClusterName=${KUBERNETES_CLUSTER_NAME?}
```

Set the values in the above command as appropriate for your setup.

After installing, the new cluster should show up in `tsh kube ls` after a few
minutes. If the new cluster doesn't show up, look into the agent logs with:

```sh
$ kubectl logs -n teleport deployment/teleport-kube-agent
```
