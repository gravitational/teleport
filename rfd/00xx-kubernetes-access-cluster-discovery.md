---
authors: Tiago Silva (tiago.silva@goteleport.com)
state: draft
---

# RFD XX - AWS Kubernetes Cluster Automatic discovery

## Required Approvers

- Engineering: `@r0mant`
- Product: `@klizhentas || @xinding33`

## What

Proposes the implementation for Teleport's Kubernetes service to automatically discover and enroll EKS clusters.

### Related issues

- [#12048](https://github.com/gravitational/teleport/issues/12048)

## Why

Currently, when an operator wants to configure a new Kubernetes cluster in the Teleport, he can opt for these two methods:

- Helm chart: when using this method, the operator has to install `helm` binary, configure the Teleport Helm repo, and check all the configurable values (high availability, roles, apps, storage...). After that, he must create a Teleport invitation token using `tctl` and finally do the Helm chart installation with the desired configuration.

- `Kubeconfig`: when using the `kubeconfig` procedure, the operator has to connect to the cluster with his credentials, generate a new service account for Teleport with the desired RBAC permissions and extract the service account token. With the token, he must create a `kubeconfig` file with the cluster CA and API server. If multiple clusters are expected to be added,  the operator has to merge multiple `kubeconfig` files into a single [kubeconfig][kubeconfig]. Finally, he must configure the `kubeconfig` location in Teleport config under `kubernetes_service.kubeconfig_file`.

Both processes described above are error-prone and can be tedious if the number of clusters to add to Teleport is high.

This document describes the changes required for Teleport to identify the clusters based on regions and desired tags. If the clusters matched the filtering criteria, they will automatically enrolled in Teleport. Once the Kubernetes cluster is deleted or no longer satisfies the discovery conditions, Teleport will automatically remove it from its lists.

### Scope

This RFD focuses only on AWS EKS clusters. Similar ideas will be explored in the future for GCP's GKE and Azure's AKS.

## Details

### Part 1: Deprecate `ServerV2` usage for Kubernetes Servers

The initial work proposed by this RFD is to remove the usage of `ServerV2` objects for representing Kubernetes Servers in Teleport API. Currently, [`ServerV2`][serverv2] has a field `KubernetesClusters` which holds the list of Kubernetes clusters proxied by the Kubernetes Server.

To remove usage of `ServerV2` for servers other than the SSH server, we must introduce the `KubernetesServerV3`, which holds the information about the clusters proxied by it as well as the server status.

```protobuf
// KubernetesServerV3 represents a Kubernetes server.
message KubernetesServerV3 {
    option (gogoproto.goproto_stringer) = false;
    option (gogoproto.stringer) = false;

    // Kind is the Kubernetes server resource kind. Always "kube_server".
    string Kind = 1 [ (gogoproto.jsontag) = "kind" ];
    // SubKind is an optional resource subkind.
    string SubKind = 2 [ (gogoproto.jsontag) = "sub_kind,omitempty" ];
    // Version is the resource version.
    string Version = 3 [ (gogoproto.jsontag) = "version" ];
    // Metadata is the Kubernetes server metadata.
    Metadata Metadata = 4 [ (gogoproto.nullable) = false, (gogoproto.jsontag) = "metadata" ];
    // Spec is the Kubernetes server spec.
    KubernetesServerSpecV3 Spec = 5 [ (gogoproto.nullable) = false, (gogoproto.jsontag) = "spec" ];
}

// KubernetesServerSpecV3 is the Kubernetes server spec.
message KubernetesServerSpecV3 {
    // Version is the Teleport version that the server is running.
    string Version = 1 [ (gogoproto.jsontag) = "version" ];
    // Hostname is the Kubernetes server hostname.
    string Hostname = 2 [ (gogoproto.jsontag) = "hostname" ];
    // HostID is the Kubernetes server host uuid.
    string HostID = 3 [ (gogoproto.jsontag) = "host_id" ];
    // Rotation contains the Kubernetes server CA rotation information.
    Rotation Rotation = 4
        [ (gogoproto.nullable) = false, (gogoproto.jsontag) = "rotation,omitempty" ];
    // Clusters is the list of Kubernetes Clusters proxied by this Kubernetes server.
    repeated KubernetesClusterV3 Clusters = 5 [ (gogoproto.jsontag) = "cluster" ];
}

```

`KubernetesServerV3` message will be added as a new field to the [`PaginatedResource`][paginatedresource] message, and the current `KubeService` field will be deprecated.

## Part 2: Kubernetes EKS auto-discovery

### AWS EKS discovery and IAM

AWS API has a method that allows listing every EKS cluster by calling [`eks:ListClusters`][listclusters] endpoint. This endpoint returns all EKS cluster names to which the user has access. The response has no other details besides the cluster name. So, for Teleport to extract the cluster details such as CA, API endpoint, and labels, it has to make an extra call, per cluster, to [`eks:DescribeCluster`][descclusters]. 

The necessary IAM permissions required for calling those two methods are:

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "eks:DescribeCluster",
                "eks:ListClusters"
            ],
            "Resource": "*" # can be limited
        }
    ]
}         
```

Besides the previous IAM policies required for the discovery service, the discovery service also calls `GetCallerIdentity` API endpoint to generate an access token to Kubernetes API, but no special permissions are required.

With cluster details such as API endpoint and CA, Teleport creates an access token to gain access to the cluster. The access is given by a token generated by [AWS IAM authenticator][awsiamauthenticator] project. This project generates a short-lived user access token by mapping the user IAM username/role into [Kubernetes RBAC credentials][iamroleatch] ([eksctl IAM Mappings][iameksctl]).

AWS IAM does not allow setting access permissions to any Kubernetes cluster. This means that it is not possible through the AWS API to guarantee access to one or several Kubernetes clusters. This is achieved by an extra project, [AWS IAM authenticator][awsiamauthenticator], that is installed, by default, in the EKS control plane which translates the IAM roles and users into Kubernetes roles or users. [AWS IAM authenticator][awsiamauthenticator] component receives a request when any user generates an access token for a cluster and, it searches in its database for any match between IAM user or role and the Kubernetes role. If the match is found, it returns a short-lived access token for the impersonated Kubernetes user or role, otherwise, it returns a token for `system:anonymous` group which has no access to resources.

The [AWS IAM authenticator][awsiamauthenticator] database is a simple `configmap/aws-auth` stored in `kube-system` namespace. This configmap cannot be changed unless Teleport has write permissions to Kubernetes Configmaps, which can only happen if the Teleport role is already in the configmap.

 The configmap has the following format:

```yaml
apiVersion: v1
data:
  mapRoles: |
    - groups:
      - {kube group}
      rolearn: {IAM role}
      username: {user name}
  mapUsers: |
    - groups:
      - {kube group}
      userARN: {IAM user}
      username: {user name}
...
```

For Teleport to be able to connect and operate the Kubernetes cluster, it is required that the `configmap/aws-auth` maps the AWS IAM role which Teleport Kubernetes Auto-Discovery agent is running into a Kubernetes group.

```yaml
apiVersion: v1
data:
  mapRoles: |
    - groups:
      - system:teleport
      rolearn: arn:aws:iam::222222222222:role/teleport-role
      username: system:teleport
...
```

This means that this mapping has to exist for each cluster that the operator wants to be discovered and to do that, the operator has to change the `configmap/aws-auth` manually. `eksctl` has a simpler method for appending identities into the `configmap/aws-auth`, but it still requires executing it for every cluster that is expected to be enrolled by Teleport.

```bash
$  eksctl create iamidentitymapping --cluster  <clusterName> --region=<region> \
      --arn arn:aws:iam::222222222222:role/teleport-role --group system:teleport --username system:teleport
```

When a user creates an Amazon EKS cluster, the AWS Identity and Access Management (IAM) entity user or role, such as a federated user that creates the cluster, is automatically granted `system:masters` permissions in the cluster's role-based access control (RBAC) configuration. If Teleport can use the same role as the user that created the cluster, then it is not required to change the `configmap`, but this solution requires that every cluster is created by users with the same IAM Role and has several security implications.

### UX

The Teleport configuration for automatic AWS EKS discovery will have the following structure:

```yaml
kubernetes_service:
  enabled: yes
  aws:
  - regions: ["us-west-1"]
    tags:
      "env": "prod"

  - regions: ["us-east-1", "us-east-2"]
    tags:
      "env": "stage"

```

To identify auto-discovered clusters, Teleport config will include a static label indicating it was created via auto-discovery.

```json
{
    "kind": "kube_cluster",
    "version": "v3",
    
    "name": "{CLUSTER_NAME}",
    "id": "{id}",
    "expires": "{never placeholder}",
    "description": "",
    "labels": {
      // forced label to indicate that the cluster was dynamically discovered
      "teleport.dev/origin": "cloud",
      // imported eks cluster tags
      "{eks_cluster_tag_key_1}":"{eks_cluster_tag_val_1}",
      "{eks_cluster_tag_key_2}":"{eks_cluster_tag_val_2}"
    },
    
    "spec": {
          "dynamic_labels": null,
    }

}
```

### Cluster enroll process

Teleport supports loading clusters from Kubernetes Kubeconfig during the startup procedure and the file has 3 main sections, `clusters`, `contexts` and `users`:

```yaml

apiVersion: v1
clusters:
- cluster:
    certificate-authority: ca.crt
    server: https://127.0.0.1:52181
  name: minikube
contexts:
- context:
    cluster: minikube
    namespace: default
    user: minikube
  name: minikube
kind: Config
users:
- name: minikube
  user:
    client-certificate: client.crt
    client-key: client.key
```

- `clusters`: defines the `cluster` API and certificate authority.
- `users`: describes `user` credentials.
- `contexts`: links a `cluster` and `user` definition.

In Teleport, the `cluster` details are used to create the cluster config and the `users` credentials are stored in credentials map. This map links the cluster name into the credentials. The discovery mechanism will leverage this behavior and dynamically enroll clusters using the same philosophy but without the necessity of writing or editing the kubeconfig file.

For each cluster to enroll, due to limitations described in the [EKS Discovery and IAM](#aws-eks-discovery-and-iam) section, Teleport cannot create access to itself, so the operator has to create the following resources for each cluster he pretends that Teleport enrolls.

The first resource to be created is the `ClusterRole` that grants minimum RBAC permissions to get pods and impersonate groups, users and service accounts. It is possible to run map Teleport role into `system:masters` but for security reasons that is discouraged.

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: teleport-role
rules:
- apiGroups:
  - ""
  resources:
  - users
  - groups
  - serviceaccounts
  verbs:
  - impersonate
- apiGroups:
  - ""
  resources:
  - pods
  verbs:
  - get
- apiGroups:
  - "authorization.k8s.io"
  resources:
  - selfsubjectaccessreviews
  - selfsubjectrulesreviews
  verbs:
  - create
```

The second step is to create the cluster role binding to map the cluster role into the Kubernetes Teleport group.

```yaml
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: teleport-role-binding
subjects:
- kind: Group
  name: system:teleport
  apiGroup: rbac.authorization.k8s.io
```

Finally, the operator has to map the Teleport IAM role into the Kubernetes group in the `configmap/aws-auth`:

```yaml
apiVersion: v1
data:
  mapRoles: |
    - groups:
      - system:teleport
      rolearn: arn:aws:iam::222222222222:role/teleport-role
      username: system:teleport
...
```

At this point, Teleport discovery can run the cluster discovery mechanism every minute and monitor if new clusters appeared or if any was deleted. When a new cluster is found, Teleport checks if it has access to it by generating short-lived credentials and doing requests to the API. If the request is successful, it registers the cluster by creating the cluster config and stores the short-lived credentials in the credentials map. Teleport will also import, for each discovered cluster, the EKS cluster tags as labels.

If the request fails due to forbidden access, Teleports logs an error mentioning that the IAM role map process is not working correctly.

For security reasons, instead of creating a new service account that provides a static token, like the one used by the Kubeconfig method, which can be stolen, the credentials used are the ones provided by [AWS IAM authenticator][awsiamauthenticator] since they are only valid for short periods, the period can be controlled by the operator at IAM role level, and Teleport can rotate them once they are near expiration.

If a cluster is deleted or no longer matches the discovery criteria, the credentials are wiped, and the cluster entry is removed.

With this method, Teleport does not install any agent in the cluster, but it requires that the Kubernetes API is accessible to the Teleport discovery. This means that if the operator wants to enroll clusters with private API endpoints, he must configure the Teleport discovery in a machine that has the ability to access the endpoint.


### Limitations

Teleport will only provide access to API and will not enroll, automatically, databases or applications like Prometheus or Grafana that easily configured in a situation where Teleport Agent is installed into the cluster.

The IAM mapping between Teleport IAM Role and Kubernetes roles is a complex and tedious process that must be done by the operator. Without it, Teleport cannot enroll the cluster.

## Security

In terms of security, the `eks:DescribeCluster` and `eks:ListClusters` methods do not give direct access to the cluster and only allow to grab the endpoint and CA certificate.

The credentials that grant access into the cluster are generated by [AWS IAM authenticator][awsiamauthenticator] and are short-lived. The TTL can be defined by the operator when he creates the Teleport Role in AWS IAM console. After the expiration date, the token can no longer be used to access the cluster and is invalid.

In order to Teleport to be able to connect to the cluster, its role must be manually added into `aws-auth` configmap  by the operator with the desired group permissions. As described above, the permissions required for Teleport to operate in this mode do not give him the possibility to create or delete resources, it can only happen via impersonation.

## Future work

Understand how to simplify the cluster enroll process, in particular how to simplify the Kubernetes permissions mapping from a IAM role.

# Links

[kubeconfig]: https://goteleport.com/docs/kubernetes-access/guides/standalone-teleport/#step-12-generate-a-kubeconfig
[listclusters]: https://docs.aws.amazon.com/eks/latest/APIReference/API_ListClusters.html
[descclusters]: https://docs.aws.amazon.com/eks/latest/APIReference/API_DescribeCluster.html
[awsiamauthenticator]: https://github.com/kubernetes-sigs/aws-iam-authenticator
[telepportiamrole]: https://goteleport.com/docs/setup/guides/joining-nodes-aws-iam/
[iamroleatch]: https://docs.aws.amazon.com/eks/latest/userguide/add-user-role.html
[iameksctl]: https://eksctl.io/usage/iam-identity-mappings/
[helmlib]: https://github.com/helm/helm/tree/main/pkg
[serverv2]: https://github.com/gravitational/teleport/blob/7d5b73eda3caf13717a647264032ef983f997c39/api/types/types.proto#L468
[paginatedresource]:https://github.com/gravitational/teleport/blob/d3e33465380de070dd3ec8347c9967c1b1257582/api/client/proto/authservice.proto#L1539
[blockpodaccess]: https://aws.github.io/aws-eks-best-practices/security/docs/iam/
#restrict-access-to-the-instance-profile-assigned-to-the-worker-node

1. https://goteleport.com/docs/kubernetes-access/guides/standalone-teleport/#step-12-generate-a-kubeconfig
2. https://docs.aws.amazon.com/eks/latest/APIReference/API_ListClusters.html
3. https://docs.aws.amazon.com/eks/latest/APIReference/API_DescribeCluster.html
4. https://github.com/kubernetes-sigs/aws-iam-authenticator
5. https://goteleport.com/docs/setup/guides/joining-nodes-aws-iam/
6. https://docs.aws.amazon.com/eks/latest/userguide/add-user-role.html
7. https://eksctl.io/usage/iam-identity-mappings/
8. https://github.com/helm/helm/tree/main/pkg
9. https://aws.github.io/aws-eks-best-practices/security/docs/iam/#restrict-access-to-the-instance-profile-assigned-to-the-worker-node
10. https://github.com/gravitational/teleport/blob/7d5b73eda3caf13717a647264032ef983f997c39/api/types/types.proto#L468
11. https://github.com/gravitational/teleport/blob/d3e33465380de070dd3ec8347c9967c1b1257582/api/client/proto/authservice.proto#L1539
---

## Alternative enroll method: Helm chart

Teleport has a join method, IAM join, that allows Teleport agents and Proxies to join a Teleport cluster without sharing any secrets when they are running in AWS.

The IAM join method is available to any Teleport agent running anywhere with access to IAM credentials, such as an EC2 instance that is part of an EKS cluster. This method allows any resource that fulfills the defined criteria to be able to join automatically into the Teleport cluster.

To use this method, each agent requires access to `sts:GetCallerIdentity` to use the IAM method. If the operator didn't [block the pod access to IMDS][blockpodaccess], this is already true since the pods inherit the node IAM role, otherwise, it can be granted by IAM OICD Provider.

To configure the IAM joining token method, the operator has to define the IAM token spec.

```yaml

kind: token
version: v2
metadata:
  # the token name is not a secret because instances must prove that they are
  # running in your AWS account to use this token
  name: kube-iam-token
  # set a long expiry time, the default for tokens is only 30 minutes
  expires: "3000-01-01T00:00:00Z"
spec:
  # use the minimal set of roles required
  roles: [Kube,[App]]

  # set the join method allowed for this token
  join_method: iam

  allow:
  # specify the AWS account which nodes may join from
  - aws_account: "111111111111"
  # multiple allow rules are supported
  - aws_account: "222222222222"
  # aws_arn is optional and allows you to restrict the IAM role of joining nodes
  - aws_account: "333333333333"
    aws_arn: "arn:aws:sts::333333333333:assumed-role/teleport-node-role/i-*"
```

Once Teleport has discovered a cluster and granted access to its API, it installs the Helm Agent chart via [Helm library][helmlib]. The Teleport Helm chart has to be updated to support IAM joining token.

Teleport discovery has to define the correct values for the Helm chart installation and execute them into the cluster. After that, the deployment is done.

##### Limitations

- If helm chart values are updated the helm installation code must also be updated accordingly
- Requires Kube Secrets backend storage.
- Requires one or more teleport agents to run in the cluster
- Requires constant upgrades of Teleport Agent