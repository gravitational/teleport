## Teleport Kubernetes Agent Updater (`teleport-kube-agent-updater`)

The Teleport kubernetes updater is a controller in charge of updating Teleport
Kubernetes agents. This alleviates the cost of updating all agents on
large-scale deployments.

Note: Teleport Kubernetes agents are not limited to
[Kubernetes Access](https://goteleport.com/docs/kubernetes-access/introduction/).
The term applies to every Teleport instance running in a Kubernetes cluster and
not running the Proxy nor Auth Service. Agents are typically deployed by the
[teleport-kube-agent](https://goteleport.com/docs/reference/helm-reference/teleport-kube-agent/)
chart.

### Design

This updater was designed first for cloud customers but can be adapter to run for on-prem users as well.

See [the cloud update RFD](../../rfd/XXXX-change-me-once-merged.md) for more context.

If an update goes wrong, a temporary downtime is acceptable until a correct
version is pushed (this risk is mitigated by multi-replica deployments).
However, the failure mode in which the deployment is stuck and the user has to
take manual action must not happen.

The updater validates the image provenance to protect against registry
compromise.

The updater logic is the following:
- check if maintenance is allowed
- check if a new version is available and version change is valid
- check if the new image can be validated
