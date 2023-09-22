# Teleport Kubernetes Operator

This package implements [an operator for Kubernetes](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/).
The Teleport Kubernetes Operator allows users deploying Teleport on Kubernetes to manage Teleport resources through
[Kubernetes custom resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/)

For more details, read the corresponding [RFD](https://github.com/gravitational/teleport-plugins/blob/master/rfd/0001-kubernetes-manager.md).

## Architecture
Teleport Operator is a Kubernetes (K8s) operator based on the `operator-sdk`.
The operator lives right beside the Teleport server, using a sidecar approach.
When multiple replicas are running, only the leader reconciles Kubernetes resources.

### Startup
When the operator starts it:
- connects to teleport using a local admin client (like `tctl` on an auth node)
- uses this local connection to ensure the operator role exist
- grabs the leader lock, to ensure only one operator is acting upon the modifications
- registers a Teleport bot ([see MachineID](https://goteleport.com/docs/machine-id/introduction/)) in charge of renewing certificates

After this point, the operator will listen to modifications in the K8S cluster, and act upon.
All the teleport resource changes are made using a gRPC client with certificates provided by `tBot`.

### Reconciliation
When something changes (either by a `kubectl apply` or from any other source), we start the reconciliation.

First, we try to identify the operation type: deletion or creation/modification.

If it's a deletion and we have our own finalizer, we remove the object in Teleport and remove the finalizer.
K8S will auto-remove the object when it gets 0 finalizers.

If it's a creation/modification:
- we add the finalizer, if it's not there already
- we lookup the object in Teleport side. If it's already here we validate it was created by the operator.
- we either create or update the resource in Teleport

### Diagram
```asciiflow
      POD
+--------------------------------------------------------+
|                                                        |         +------+
|                                                        |         |      |
|     teleport                                           |         |      |
| +---------------------------------+                    |         |      |
| |                                 |                    |         +-+----+
| |                                 |                    |           |
| |                            +----+                    |           | kubectl apply -f
| |  +-------------+           |gRPC|<--+                |           |
| |  |/etc/teleport|           +----+   |                |           |
| |  +^------------+                |   |                |           |
| |   |                             |   |                |           |
| |   |   +-----------------+       |   | Manage         |           |
| |   |   |/var/lib/teleport|       |   | Resources      |           |
| |   |   +^----------------+       |   |                |           |  kube-apiserver
| |   |    |                        |   |                |      +----v----------------+
| +---+----+------------------------+   |                |      |                     |
|     |    |                            |                |      |                     |
|     |    |                            |                |      |                     |
|     |    |   operator                 |                |      |                     |
| +---+----+----------------------------+--------+       |      |                     |
| |   |    |                            |        |       |      |                     |
| |  ++----+----+                 +-----v----+   |       |      |                     |
| |  |  tBot    |-----------------> teleport <---+-------+------>                     |
| |  +----------+ Get client      | operator |   |       |      |                     |
| |              & renew certs    +----------+   |       |      |                     |
| |                                              |       |      |                     |
| |                                              |       |      |                     |
| +----------------------------------------------+       |      +---------------------+
|                                                        |
|                                                        |
|                                                        |
+--------------------------------------------------------+
```

## Running

### Requirementes

#### K8S cluster
If you don't have a cluster yet, you can start one by using the [minikube](https://minikube.sigs.k8s.io/docs/start/) tool.

#### Operator's docker image
You can obtain the docker image by pulling from `public.ecr.aws/gravitational/teleport`

#### HELM chart from Teleport with the operator
The [teleport-cluster Helm chart](../examples/chart/teleport-cluster) supports deploying the operator alongside teleport.

#### Other tools
We also need the following tools: `helm`, `kubectl` and `docker`

### Running the operator

Install the helm chart:
```bash
# Run the command at the root of the teleport repo
helm upgrade --install --create-namespace -n teleport-cluster \
	--set clusterName=teleport-cluster.teleport-cluster.svc.cluster.local \
	--set teleportVersionOverride="11.0.0-dev" \
	--set operator.enabled=true \
	teleport-cluster ./examples/chart/teleport-cluster

kubectl config set-context --current --namespace teleport-cluster
```

Wait for the deployment to finish:
```bash
kubectl wait --for=condition=available deployment/teleport-cluster --timeout=2m
```

If it doesn't, check the errors.

Now, we want access to two configuration tools using a Web UI: K8S UI and Teleport UI.

If you are using `minikube`, you have to create a tunnel with: `minikube tunnel` (this command runs is foreground, open another terminal for the remaining commands).

Create a new Teleport User and login in the web UI:
```bash
PROXY_POD=$(kubectl get po -l app=teleport-cluster -o jsonpath='{.items[0].metadata.name}')
kubectl exec $PROXY_POD teleport -- tctl users add --roles=access,editor teleoperator
echo "open following url (replace the invite id) and configure the user"
TP_CLUSTER_IP=$(kubectl get service teleport-cluster -o jsonpath='{ .status.loadBalancer.ingress[0].ip }')
echo "https://${TP_CLUSTER_IP}/web/invite/<id>"
```

Open the Kubernetes Dashboard (`minikube dashboard` if your cluster was created by `minikube`) and switch to `teleport-cluster` namespace.
Your resources will appear under the Custom Resources menu.

You can manage users and roles using to usual kubernetes tools, for example, `kubectl`.

As an example, create the following file (`roles.yaml`) and then apply it:
```yaml
apiVersion: "resources.teleport.dev/v5"
kind: TeleportRole
metadata:
  name: myrole
spec:
  allow:
    logins: ["root"]
    kubernetes_groups: ["edit"]
    node_labels:
      dev: ["dev", "dev2"]
      type: [ "compute", "x" ]
```

```bash
kubcetl apply -f roles.yaml
```

And now check if the role was created in Teleport and K8S (`teleport-cluster` namespace).
```bash
PROXY_POD=$(kubectl get po -l app=teleport-cluster -o jsonpath='{.items[0].metadata.name}')
kubectl exec $PROXY_POD teleport -- tctl get roles/myrole
```

## Contributing

### Adding support for new Teleport types

The steps to add support for new Teleport types to the operator are as follows:

#### Make sure the existing CRDs are up to date

In a clean repo before making any changes:

1. Run `make manifests`.
2. Run `make -C crdgen update-protos`.
3. Run `make -C crdgen update-snapshot`.

Make sure everything looks sane and commit any changes (there will be some if
other .proto files used to generate the CRDs have changed).

#### Generate the new CRD

1. Add the type name to the `resources` list in `crdgen/main.go`.
2. Add the proto file to the `PROTOS` list in `Makefile` if it is not
   already present. Also add it to the `PROTOS` list in `crdgen/Makefile`.
3. Run `make manifests` to generate the CRD.
4. Run `make crdgen-test`. This will should fail if your new CRD is generated.
   Update the test snapshots with `make -C crdgen update-snapshots`

#### Create a "scheme" defining Go types to match the CRD

Add a type for your resource under `apis/resources/<version>`.
Follow the same patterns of existing types in those packages.
If there is no existing directory for the current version of your resource,
create one, and don't forget to in include a `groupversion_info.go` file copied
from one of the existing version directories (with the correct version).
Do not forget to include the //+kubebuilder comments in these files.

#### Create a reconciler for the new resource type

Create a reconciler under the `controllers/resources` path.
Follow the same patterns of existing reconcilers in those packages.
Use the generic TeleportResourceReconciler if possible, that way you only have
to implement CRUD methods for your resource.

Write unit tests for your reconciler. Use the generic `testResourceCreation`,
`testResourceDeletionDrift`, and `testResourceUpdate` helpers to get baseline
coverage.

#### Register your reconciler and scheme

In `main.go`, instantiate your reconciler and register it with the
controller-runtime manager.
Follow the pattern of existing resources which instantiate the reconciler and
call the `SetupWithManager(mgr)` method.

If you added a new version under `apis/resources/`, make sure the scheme for
your resource version is added to the root `scheme` with a call like
`resourcesvX.AddToScheme(scheme)` in the `init()` function.

#### Add RBAC permissions for the new resource type

Add Kubernetes RBAC permissions to allow the operator to work with the resources
on the Kubernetes side.
The cluster role spec is found in  `../../examples/chart/teleport-cluster/templates/auth/clusterrole.yaml`.

Add Teleport RBAC permissions for to allow the operator to work with the
resources on the Teleport side.
These should be added to the sidecar role in `sidecar/sidecar.go`.

### Debugging tips

Usually the best way to debug reconciler issues is with a unit test, rather than
trying to debug the operator live in a kubernetes cluster.
Try writing a short, temporary test to run your controller through a resource
single reconciliation.

You can set the `OPERATOR_TEST_TELEPORT_ADDR` environment variable to run the
controller tests against a live teleport cluster instead of a temporary one
created in the test. It will use your local `tsh` credentials to authenticate,
so make sure the role of your logged in user has sufficient permissions.
