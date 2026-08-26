## Generating a ServiceAccount to use for Teleport integration tests

This should be done on a 'clean' k8s cluster i.e. one that doesn't already have Teleport installed for
Kubernetes forwarding (and doesn't require it), as we delete the default Teleport `ClusterRole` and
`ClusterRoleBinding` for security.

```
# Check out the Teleport repo and change dir to it
git clone https://github.com/gravitational/teleport
cd teleport

# generate a ServiceAccount using the get-kubeconfig script
TELEPORT_NAMESPACE="ci-teleport" examples/k8s-auth/get-kubeconfig.sh

# copy the generated kubeconfig, then add it to CI as a secret (out of band)
mv kubeconfig INTEGRATION_CI_KUBECONFIG

# add the additional required RBAC fixtures
kubectl create -f fixtures/ci-teleport-rbac/ci-teleport.yaml

# remove the additional teleport permissions that were added by the get-kubeconfig script
# (as these are not needed for CI, we can remove them for greater security)
kubectl delete clusterrole/teleport-role
kubectl delete clusterrolebinding/teleport-crb
```