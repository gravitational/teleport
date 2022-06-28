# Teleport Kubernetes Operator

This package implements [an operator for Kubernetes](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/).
This operator is useful to manage Teleport resources e.g. users and roles from [Kubernetes custom resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/)

For more details, read the corresponding [RFD](https://github.com/gravitational/teleport-plugins/blob/master/rfd/0001-kubernetes-manager.md).

## Architecture
Teleport Operator is a K8S operator based on `operator-sdk`.
The operator lives right beside the Teleport server, using a sidecar approach.

### Setup
When the operator starts it:
- grabs the leader lock, to ensure only one operator is acting upon the modifications
- ensures the exclusive role and user exist
- creates an identity file using that user
- starts a new client, using that identity file and manages the resources
- install the CRDs for the managed resources

After this point, the operator will listen to modifications in the K8S cluster, and act upon

### Reconciliation
When something changes (either by a `kubectl apply` or from any other source), we start the reconciliation.

First, we try to identify the operation type: deletion or creation/modification.

If it's a deletion and we have our own finalizer, we remove the object in Teleport and remove the finalizer.
K8S will auto-remove the object when it gets 0 finalizers.

If it's a creation/modification:
- we add the finalizer, if it's not there already
- we lookup the object in Teleport side, and if it's not there, we set the Origin label to `kubernetes`
- we either create or update the resource in Teleport

The first two steps above may change the object's state in K8S. If they do, we update and finish the cycle - we'll receive another reconciliation request with the new state.

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
| |  |sidecar   <-----------------> teleport <---+-------+------>                     |
| |  +----------+ Setup U/R       | operator |   |       |      |                     |
| |             Create Identity   +----------+   |       |      |                     |
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
You can obtain the docker image by pulling from `quay.io/gravitational/teleport`

#### HELM chart from Teleport with the operator
We are re-using the Teleport Cluster Helm chart but modifying to start the operator using the image above.

This change is not in master, so you'll need to checkout the HELM charts from this branch
`marco/plugins-teleport-operator-charts` (https://github.com/gravitational/teleport/pull/12144)

#### Other tools
We also need the following tools: `helm`, `kubectl` and `docker`

### Running the operator

Set the `TELEPORT_PROJECT` to the full path to your teleport's project checked out at `marco/plugins-teleport-operator-charts`.

Install the helm chart:
```bash
helm upgrade --install --create-namespace -n teleport-cluster \
	--set clusterName=teleport-cluster.teleport-cluster.svc.cluster.local \
	--set teleportVersionOverride="11.0.0-dev" \
	--set operator=true \
	teleport-cluster ${TELEPORT_PROJECT}/examples/chart/teleport-cluster

kubectl config set-context --current --namespace teleport-cluster

```

Now let's wait for the deployment to finish:
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
kind: Role
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