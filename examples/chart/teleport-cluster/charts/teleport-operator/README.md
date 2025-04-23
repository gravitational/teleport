# Teleport Operator

This chart deploys the Teleport Kubernetes Operator. The operator allows to manage
Teleport resources from inside Kubernetes.

## Important notice

The chart version follows the Teleport and Teleport Kube Operator version. e.g.
chart v15.0.1 runs the operator version 15.0.1 by default. To control which
operator version is deployed, use the `--version` Helm flag.

## Deployment

The chart can be deployed in two ways:
- in standalone mode by running
  ```code
  helm install teleport/teleport-operator teleport-operator --set authAddr=teleport.example.com:443 --set token=my-operator-token
  ```
  See [the standalone guide](https://goteleport.com/docs/admin-guides/infrastructure-as-code/teleport-operator/teleport-operator-standalone/) for more details.
- as a dependency of the `teleport-cluster` Helm chart by adding `--set operator.enabled=true`. See
  [the operator within teleport-cluster chart guide](https://goteleport.com/docs/admin-guides/infrastructure-as-code/teleport-operator/teleport-operator-helm/).

## Values and reference

The `values.yaml` is documented through comment or via
[the reference docs](https://goteleport.com/docs/reference/helm-reference/teleport-operator/).

Please make sure you are looking at the correct version when looking at the values reference.
