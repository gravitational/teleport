# Teleport Kubernetes Operator

This package implements [an operator for Kubernetes](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/).
The Teleport Kubernetes Operator allows users deploying Teleport on Kubernetes to manage Teleport resources through
[Kubernetes custom resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/)

Currently supported resources are `users` and `roles`.

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
You can obtain the docker image by pulling from `quay.io/gravitational/teleport`

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