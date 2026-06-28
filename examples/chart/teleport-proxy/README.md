# Teleport Proxy

This chart deploys Teleport Proxy instances that join an existing Teleport
cluster.

Use this chart to add stand-alone Teleport Proxies without also deploying the
Teleport Auth on the same Kubernetes clusters, such as for regional proxies
closer to users. To deploy the Teleport Proxy and Teleport Auth together, use
[teleport-cluster](../teleport-cluster/) instead.

## Prerequisites

- An existing Teleport cluster with Auth Service endpoint reachable from where
  you're deploying this proxy.
- A Teleport provision token that allows Proxy joining.
  - [Terraform](https://goteleport.com/docs/reference/infrastructure-as-code/terraform-provider/resources/provision_token/)
  - [tctl](https://goteleport.com/docs/reference/cli/tctl/#tctl-tokens-add)
    (`tctl tokens add --type=proxy ...`)

## Example Usage

Create a token

```bash
$ kubectl \
  --context my-k8s-cluster-with-teleport-auth \
  -n my-teleport-cluster-namespace \
  get --raw /.well-known/openid-configuration | yq .issuer

https://oidc.my-k8s-cluster.my-cloud-provider.com/not-real/dont-copy

tctl create -f - <<EOT
kind: token
version: v2
metadata:
  name: my-proxy-token
spec:
  roles: [Proxy]
  join_method: kubernetes
  kubernetes:
    type: oidc
    oidc:
      issuer: https://oidc.my-k8s-cluster.my-cloud-provider.com/not-real/dont-copy
    allow:
      - service_account: "my-proxy-only-ns:my-sa"
EOT
```

Install the teleport-proxy chart, matching the values to those of the token you
created.

```bash
helm install teleport-proxy ./teleport-proxy \
  --kube-context my-k8s-proxy-only-cluster \
  --create-namespace \
  --namespace my-proxy-only-ns \
  --set clusterName=teleport.example.com \
  --set serviceAccout.name=my-sa \
  --set auth_server=teleport-auth.example.com:3025 \
  --set join_params.method=kubernetes \
  --set join_params.token_name=my-proxy-token \
```

## Configuration Notes

- See [values.yaml](values.yaml) for field-by-field instructions

## Documentation

See the Teleport documentation for deployment and configuration details:

- Helm deployment guides:
  https://goteleport.com/docs/zero-trust-access/deploy-a-cluster/helm-deployments/
- Teleport configuration reference:
  https://goteleport.com/docs/reference/config/
- Teleport join methods:
  https://goteleport.com/docs/reference/deployment/join-methods/

## Contributing

Please read [CONTRIBUTING.md](../CONTRIBUTING.md) before raising a pull request
to this chart.
