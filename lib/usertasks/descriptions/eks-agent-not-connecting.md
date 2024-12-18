The process of automatically enrolling EKS Clusters into Teleport, starts by installing the [`teleport-kube-agent`](https://goteleport.com/docs/reference/helm-reference/teleport-kube-agent/) to the cluster.

If the installation is successful, the EKS Cluster will appear in your Resources list.

However, the following EKS Clusters did not automatically enrolled.
This usually happens when the installation is taking too long or there was an error preventing the HELM chart installation.

Navigate to the Teleport Agent to get more information.

[//]: <> (UI must include a column on clusters list:)
[//]: <> (Open Teleport Agent: https://<region>.console.aws.amazon.com/eks/home?region=<region>#/clusters/<cluster-name>/statefulsets/teleport-kube-agent?namespace=teleport-agent)