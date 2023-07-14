---
authors: Tiago Silva (tiago.silva@goteleport.com)
state: draft
---

# RFD 98 - Kubernetes Cluster Automatic discovery

## Required Approvers

- Engineering: `@r0mant`
- Product: `@klizhentas || @xinding33`

## What

Proposes the implementation of Kubernetes Auto-Discovery for EKS, GKE and AKS clusters.

### Related issues

- [#12048](https://github.com/gravitational/teleport/issues/12048)

## Why

Currently, when an operator wants to configure a new Kubernetes cluster in Teleport, he can opt for these two methods:

- Helm chart: when using this method, he has to install `helm` binary, configure Teleport Helm repo, and check all the configurable values (high availability, roles, apps, storage...). After that, he must create a Teleport invitation token using `tctl` and finally install the Helm chart with the desired configuration.

- `Kubeconfig`: when using the `kubeconfig` procedure, he has to connect to the cluster - using his credentials - and generate a new service account for Teleport. Besides that, he must assign the required RBAC permissions into the SA and extract its token. With it, he must create a `kubeconfig` file with the cluster CA and API server. If Teleport Kubernetes Service serves more than one cluster, he has to merge multiple `kubeconfig` files into a single file. Finally, he must configure the `kubeconfig` location in Teleport config under `kubernetes_service.kubeconfig_file` and restart the service.

Both processes are error-prone and can be tedious if the number of clusters to configure is high.

This document describes the required changes for Teleport to identify clusters based on their properties such as tags, location, and account. If the filtering criteria matches, Teleport will automatically enroll the cluster. On the other hand, whenever someone deletes an active Kubernetes cluster or the cluster no longer matches the discovery conditions, Teleport will automatically remove the cluster from its inventory.

## Scope

This RFD describes how Kubernetes cluster Auto-Discovery works for AWS, Azure and GCP clouds.

## Kubernetes Auto-Discovery

Teleport Kubernetes Auto-Discovery will search for Kubernetes clusters in the configured cloud providers and automatically enroll them in Teleport.

This process **will not install any agent or pods on the target clusters**. Instead, it will generate short-lived credentials to access the clusters' APIs and renew them when they are near expiration.

Kubernetes Auto-Discovery consists of two distinct services:  

### Polling cloud APIs

Teleport Discovery Service is responsible for scanning at regular intervals - 5 minutes -
the configured cloud providers and identifying if any Kubernetes cluster matches
the filtering labels. 

When the process identifies a new Kubernetes cluster, it creates a dynamic 
resource within Teleport. This resource includes information imported from the 
cloud such as name, tags, and account. By default, the cluster name in Teleport 
registry is the cluster's cloud name, however you can assign the tag 
`teleport.dev/kubernetes-name: custom_name` to your cluster to import it under a 
custom name in the Teleport registry.

In addition to detecting new Kubernetes clusters, the Discovery Service also removes - from Teleport's registry - the Kubernetes clusters that have been deleted or whose tags no longer match the filtering labels.


### Forwarding requests to the Kubernetes Cluster

Teleport Kubernetes Service is responsible for monitoring the dynamic resources created or 
updated by the Discovery Service and forwarding requests to them.

To turn on dynamic resource monitoring in Kubernetes Service you must configure 
the `kubernetes_service.resources` section as exemplified in the following snippet.

```yaml
## This section configures the Kubernetes Service
kubernetes_service:
    enabled: "yes"
    resources:
    - labels:
        "*": "*" # can be configured to limit the clusters to watched by this service.
```

When a Kubernetes cluster API is privately accessible, Teleport Kubernetes Service must be in the same network as the target cluster. You can configure the Kubernetes Service to filter only a subset of clusters to forward requests to.

When the Kubernetes Service detects a Kubernetes cluster originating from a cloud provider, it will generate short-lived credentials to access it. The service also registers the cluster in its inventory and advertises it using the heartbeat mechanism.

## AWS EKS discovery

The following subsections will describe the details required for implementing the EKS auto-discovery.

The discovery agent can auto-discover EKS-managed clusters in the AWS account it has credentials for by matching their resource tags and regions. For now, the discovery of connected clusters is out of the scope of this RFD.

Authentication on an EKS cluster happens using short-lived tokens associated with the IAM role that the Kubernetes Service is using. Authorization also occurs by matching the IAM role against a database of mappings between IAM roles/users and Kubernetes RBAC users/groups. The database is a Configmap - `configmap/aws-auth` in `kube-system` namespace - within the cluster. It means that before Teleport can access the cluster, its IAM role must be included in the Configmap. This situation limits the Teleport automatic Discovery because Teleport cannot configure the access it needs on its own and requires manual actions by the cluster administrator.

By default, the cluster creator and every member that shares his IAM role or federated user have immediate access to the cluster as `system:masters`. This rule is enforced by [AWS IAM authenticator][awsiamauthenticator] and cannot be seen or edited by manipulating `configmap/aws-auth` since it is hidden.

If Teleport Kubernetes Service shares the same IAM role as the cluster's creator, it immediately has full access to the cluster and no further action is necessary. But situations where EKS clusters are created by a different IAM roles, the Kubernetes Service won't be able to access them! For security purposes, it is not recommended running Teleport with the user's IAM role.

### IAM Permissions

Teleport requires access to [`eks:ListClusters`][listclusters] and [`eks:DescribeCluster`][descclusters] APIs to perform the cluster discovery and cluster details extraction.

The necessary IAM permissions required for calling the two endpoints are:

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
            "Resource": "*" // resource access can be limited
        }
    ]
}         
```

Besides the mentioned calls, Kubernetes Service also calls `sts:GetCallerIdentity`, but this operation does not require any permissions.

### UX

#### Configuration

The snippets below configure the Discovery Service to watch EKS resources with `tag:env=prod` in the `us-west-1` region and configures the Kubernetes Service to watch dynamic `kube_cluster` resources that include the label `env=prod`.

##### Discovery Service

The Teleport configuration for automatic EKS discovery will have the following structure:

```yaml
discovery_service:
  enabled: yes
  aws:
  - regions: ["us-west-1"]
    types: ["eks"]
    tags:
      "env": "prod"
```

##### Kubernetes Service

In the Kubernetes Service, we have to configure the `resources` monitoring section with the cluster tags of the dynamic resources that this agent is able to forward requests for.

```yaml
## This section configures the Kubernetes Service
kubernetes_service:
    enabled: "yes"
    resources:
    - labels:
        "env": "prod" # can be configured to limit the clusters to watched by this service.
```

#### `teleport discovery bootstrap`

Teleport will provide a simple CLI program to simplify the EKS Auto-Discovery process and cluster permissions management. 

When `teleport discovery bootstrap` detects that it has AWS discovery enabled and `eks` is defined in types, it will create the required RBAC permissions and assign them to Teleport's IAM Role ARN.

`teleport discovery bootstrap` must run with an IAM identity that is mapped into `system:masters` for every cluster it is expected to discover and will have the following behavior:

```shell
$ teleport discovery bootstrap --aws-arn=arn:aws:iam::222222222222:role/teleport-role
[1] Connecting to the AWS environment...
[2] Checking your user permissions....
[3] Validating Teleport IAM Role....
[4] Attaching IAM permissions....
[5] Discovering EKS clusters based on tags provided....
[6] Installing RBAC resources for cluster %cluster[0].name%
[7] Mapping `teleport` RBAC Group to Teleport Agent IAM Role for cluster %cluster[0].name%
...
[..] Installing RBAC resources for cluster %cluster[n].name%  
[..] Mapping `teleport` RBAC Group to Teleport Agent IAM Role for cluster %cluster[n].name%
```

Where `n` is the number of clusters discovered based on rules provided. 

After the command finishes, the Discovery and Kubernetes Services can start enrolling the affected EKS clusters. 

In situations where you do not want to run this CLI or you create new clusters, you can follow the manual guide referenced in the next section.

#### Manual guide

In this section, we describe the manual process for granting to Kubernetes Service the necessary permissions when it runs with a different IAM role.

First, you must create the `ClusterRole` resource defined in Appendix A using `kubectl`. In the second step, you must create the `ClusterRoleBinding` resource defined in Appendix B, which maps the cluster role into the `teleport` Kubernetes RBAC group. It is possible, although not recommended, to assign an existent group such as `system:masters` with Impersonation permissions instead of creating a new group.

At this point, the `teleport` group has the minimum permissions, and the only piece left is assigning it to the Teleport IAM role. The mapping between the IAM role and `teleport` is created by appending into the `configmap/aws-auth` - located at the `kube-system` namespace - the following content:

```yaml
apiVersion: v1
data:
  mapRoles: |
    - groups:
      - teleport
      rolearn: arn:aws:iam::222222222222:role/teleport-role
      username: teleport
...
```

`eksctl` has a simpler way to manage IAM Role mappings:

```bash
$  eksctl create iamidentitymapping --cluster  <clusterName> --region=<region> \
      --arn arn:aws:iam::222222222222:role/teleport-role --group teleport --username teleport
```

The cluster is now ready to be discovered in the next iteration.

### Resource watcher

AWS does not provide a method that returns the available EKS clusters and their details. Instead, Teleport will check for available EKS clusters by calling the [`eks:ListClusters`][listClusters] API at regular intervals. This endpoint returns the list of EKS cluster names that the IAM identity has access to but not their details. To extract the details such as name and tags, the Discovery Service will call [`eks:DescribeCluster`][descclusters] method for each cluster returned by the previous call. These calls will be made concurrently with a limit of 5 simultaneous calls to speed up the process.

### Authentication & Authorization

This section defines the EKS token generation (authentication) and the authorization required for Teleport to work with the cluster.

#### Authentication

Access to the EKS cluster is granted by sending a pre-signed token as an authorization header. To generate this token, Teleport will pre-sign a `sts:GetCallerIdentity` request with an extra `x-k8s-aws-id` header whose value is the name of the target EKS cluster. The token contains the prefix `k8s-aws-v1.` followed by the base64 encoded version of the pre-sign result as shown below.

```go
request, _ := stsClient.GetCallerIdentityRequest(&sts.GetCallerIdentityInput{})
request.HTTPRequest.Header.Add(`x-k8s-aws-id`, clusterName)
presignedURLString, _ := request.Presign(requestPresignParam)

token = "k8s-aws-v1."+ base64.RawURLEncoding.EncodeToString([]byte(presignedURLString))
```

For each Kubernetes API request, the Kubernetes client sends the token as an `Authorization` header, and the [AWS IAM authenticator][awsiamauthenticator] - which is installed by default on every EKS control plane - manages the access control. This component is responsible for checking if the token is valid and for translating the IAM roles/users into Kubernetes RBAC roles or users. The [AWS IAM authenticator][awsiamauthenticator] searches for any correspondence between the IAM user or role associated with the token and the Kubernetes role. If a match is found, it impersonates the Kubernetes RBAC group/user, otherwise, it impersonates the `system:anonymous` group that does not have access to cluster resources.

#### Authorization

AWS IAM does not allow setting access permissions to any Kubernetes cluster. This means that it is not possible through the AWS API to guarantee access to one or several Kubernetes clusters. Instead, access control is managed by the [AWS IAM authenticator][awsiamauthenticator] project, which transforms the IAM identities into Kubernetes roles or users using local rules per cluster.

As mentioned before, the [AWS IAM authenticator][awsiamauthenticator] has a database with mappings between IAM roles and Kubernetes RBAC principals. This database is a simple `configmap` whose name is `aws-auth`, and it's stored in the `kube-system` namespace. To edit it you must have writing permissions to Configmaps at the `kube-system` namespace, which can only happen if you previously had access to the cluster.

By default, the cluster creator and every member that shares his IAM role or federated user have immediate access to the cluster as `system:masters`. This rule is enforced by [AWS IAM authenticator][awsiamauthenticator] and cannot be seen or edited by manipulating `configmap/aws-auth`.

If Teleport discovery shares the same IAM role as the cluster's creator, it immediately has full access to the cluster and no further action is necessary. This creates a limitation because EKS clusters must be created by users with the same IAM role as Teleport otherwise if an EKS cluster is created by a different IAM role/federated user, Teleport does not have access to that cluster! For security purposes, it is not recommended running Teleport with the user's role.

If the Teleport agent is running with a different IAM role, it is required that its IAM role maps into a Kubernetes RBAC group. This can be configured by appending an extra entry into `configmap/aws-auth`.

The `configmap` has the following format:

```yaml
apiVersion: v1
data:
  mapRoles: |
    - groups:
      - {kube group}
      rolearn: {IAM role}
      username: {user name}
      ...
  mapUsers: |
    - groups:
      - {kube group}
      userARN: {IAM user}
      username: {user name}
    ...
```

The appended entry must have the following format:

```yaml
apiVersion: v1
data:
  mapRoles: |
    - groups:
      - system:teleport
      rolearn: arn:aws:iam::222222222222:role/teleport-role
      username: system:teleport
```

Where `teleport` RBAC group defines the minimum required permissions and can be found at Appendix A.

Without this entry in `configmap/aws-auth`, Teleport does not have access to the cluster. 

Summarizing, it is impossible for Teleport to escalate its privileges and grant access to the cluster from a no-access situation. It requires a manual action to link Teleport IAM role into Kubernetes RBAC principals otherwise Teleport cannot forward requests to the cluster.

### Limitations

The IAM mapping between Teleport IAM Role and Kubernetes roles is a complex and tedious process that must happen per cluster. Without it, Teleport cannot forward requests to clusters. The AWS EKS team has a feature request to add an external API that allows configuring access to the cluster without manually editing the `configmap` ([aws/containers-roadmap#185](https://github.com/aws/containers-roadmap/issues/185)). Hopefully, once the feature is available, Teleport can leverage it to automatically configure its access to the cluster.

## Azure AKS discovery

The following subsections will describe the necessary details for implementing AKS Auto-Discovery.

### AKS Authentication & Authorization

This section briefly describes the different authentication and authorization modes
available on AKS clusters. 

Azure AKS clusters have three different configuration modes for managing cluster 
access. Each cluster has one of these modes configured, and depending on the mode, 
the authentication and authorization process is different.

- **Kubernetes Local Accounts** (*default*)

When a cluster uses this mode, the access happens via Kubernetes local accounts. During the cluster provisioning phase, these accounts are created, and the access credentials are available on different API endpoints depending on the desired access level.

- `aks:ListClusterUserCredentials`: returns credentials for the cluster user account.
- `aks:ListClusterAdminCredentials`: returns credentials for the cluster admin account.

The returned credentials can be used to authenticate directly into the Kubernetes API and are shared across all users that have access to `ListCluster*Credentials` endpoints. Thus, the credentials returned are non-auditable.

When a cluster uses this mode, Teleport can automatically discover and forward requests into it without any modification of the target cluster.

- **Active Directory with Kubernetes RBAC**

When a cluster uses this mode, Azure allows the user's identity to be the authentication principal. The process happens via an OpenID Connect layer that validates the user credentials and returns the user principals such as `user_id` and `group_ids` used by Kubernetes RBAC for authorization. OpenID layer only guarantees the authentication but does not handle authorization. Authorization handled by Kubernetes RBAC.

Teleport can automatically forward requests into AKS clusters in the following cases:

- Teleport's AD identity has permissions that allow access to the static [cluster administrator credentials](https://learn.microsoft.com/en-us/rest/api/aks/managed-clusters/list-cluster-admin-credentials) (local accounts).
- Teleport's AD identity belongs to the cluster's administrator group.
- Teleport's AD identity has permissions to create `ClusterRole` and `ClusterRoleBinding` on the 
cluster and permissions to execute [remote commands](https://learn.microsoft.com/en-us/rest/api/aks/managed-clusters/run-command).

- **Active Directory with Azure RBAC** (*recommended*)

Under this mode, Azure transfers the authorization of AD users and groups to Azure RBAC. While Kubernetes RBAC validates regular Kubernetes users and service accounts.
Azure RBAC, associated with the groups to which the user belongs, defines the authorization rules, and therefore it is possible to grant access to multiple AKS clusters without having to manage Kubernetes RBAC policies on each of them.

An example of Azure RBAC policy that allows a user to get pods is the following:

```json
{
    "Name": "Read Pods",
    "Description": "Read Pods.",
    "Actions": [

],
    "NotActions": [],
    "DataActions": [
      "Microsoft.ContainerService/managedClusters/pods/read",
    ],
    "NotDataActions": [],
    "assignableScopes": [
        "/subscriptions/{subscription_id}" // allows access to any cluster within the subscription.
        // "/subscriptions/{subscription_id}/resourceGroups/{resource_group}/providers/Microsoft.ContainerService/managedClusters/" // limits access to a certain resource group
        // "/subscriptions/{subscription_id}/resourceGroups/{resource_group}/providers/Microsoft.ContainerService/managedClusters/{cluster_name}" // limits access to a certain cluster
    ]
}
```

When a cluster uses this mode, Teleport can automatically discover and forward requests into it without any modification of the target cluster.

### Permissions

In order for the Discovery Service to be able to list the AKS clusters, the Azure identity which the Teleport Discovery Service is using must include the following permission:

```json

    "permissions": [
      {
        "actions": [
          "Microsoft.ContainerService/managedClusters/read"
        ],
      }
    ]      
```

Azure built-in "Reader" role already has these permissions assigned, but it is encouraged to use the minimal version provided above.

Depending on the AKS Authentication & Authorization mode, Teleport Kubernetes Service requires a different set of permissions.

- **Kubernetes Local Accounts**

```json
{
    "Name": "AKS Teleport Discovery",
    "Description": "Required permissions for Teleport auto-discovery.",
    "Actions": [
      "Microsoft.ContainerService/managedClusters/read",
      "Microsoft.ContainerService/managedClusters/listClusterUserCredential/action",
    ],
    "NotActions": [],
    "DataActions": [],
    "NotDataActions": [],
    "assignableScopes": [
        "/subscriptions/{subscription_id}"
    ]
}
```

- **Active Directory with Kubernetes RBAC**

The optional permissions can be included to allow Teleport to automatically configure its own access whenever it is possible.

```json
{
    "Name": "AKS Teleport Discovery",
    "Description": "Required permissions for Teleport auto-discovery.",
    "Actions": [
        "Microsoft.ContainerService/managedClusters/read",
        "Microsoft.ContainerService/managedClusters/listClusterUserCredential/action",
        
        # optional - useful if Teleport belongs to the admin groups or the cluster has static admin credentials
        "Microsoft.ContainerService/managedClusters/listClusterAdminCredential/action", 

        # optional - Usefull if Teleport has the ability to create ClusterRole and ClusterRoleBindings in the target cluster.
        "Microsoft.ContainerService/managedClusters/runcommand/action",
        "Microsoft.ContainerService/managedclusters/commandResults/read"
    ],
    "NotActions": [],
    "DataActions": [],
    "NotDataActions": [],
    "assignableScopes": [
        "/subscriptions/{subscription_id}"
    ]
}
```

- **Active Directory with Azure RBAC**

```json
{
    "Name": "AKS Teleport Discovery",
    "Description": "Required permissions for Teleport auto-discovery.",
    "Actions": [
      "Microsoft.ContainerService/managedClusters/read"
    ],
    "NotActions": [],
    "DataActions": [
      "Microsoft.ContainerService/managedClusters/groups/impersonate/action",
      "Microsoft.ContainerService/managedClusters/users/impersonate/action",
      "Microsoft.ContainerService/managedClusters/serviceaccounts/impersonate/action",
      "Microsoft.ContainerService/managedClusters/pods/read",
      "Microsoft.ContainerService/managedClusters/authorization.k8s.io/selfsubjectaccessreviews/write",
      "Microsoft.ContainerService/managedClusters/authorization.k8s.io/selfsubjectrulesreviews/write",
    ],
    "NotDataActions": [],
    "assignableScopes": [
        "/subscriptions/{subscription_id}"
    ]
}
```

### UX

#### Configuration

The snippets below configure the Discovery Service to watch AKS resources with `tag:env=prod` on `eastus` and `centralus` regions and configures Kubernetes Service to watch Teleport `kube_cluster` resources that include the label `env=prod`.

##### Discovery Service

The Azure configuration for the Discovery Service will have the following structure:

```yaml
discovery_service:
  enabled: yes
  azure:
      # subscriptions defines the Azure subscription IDs the discovery agent will use for AKS cluster search
      # default: ["*"]
    - subscriptions: ["sub1", "sub2"]
      # resource_groups defines the Azure resource group names to filter AKS clusters.
      # default: ["*"]
      resource_groups: ["group1", "group2"]
      # regions defines the Azure regions of AKS clusters
      # default: ["*"]
      regions: ["eastus", "centralus"]
      # types defines the discovery types.
      # For AKS the value must be "aks"
      # mandatory field
      types: ["aks"] 
      # tags defines the filtering tags for AKS clusters
      tags: # default: "*":"*"
        "env": "prod" 
```

##### Kubernetes Service

In the Kubernetes Service we have to configure the `resources` monitoring section with the cluster tags that this agent is able to serve.

```yaml
kubernetes_service:
  enabled: yes
  # resources section lists the label selectors you'd like this agent to monitor
  resources:
#  - labels:
#        "*": "*"  # catches any `kube_cluster` resource
  - labels:
        "env": "prod"  # catches `kube_cluster` resource with a label = `env:prod`.
```

#### Access control

This section describes how users will configure the minimum required access levels for Teleport services depending on cluster authentication and authorization mode. A more detailed description is outside the scope of this section but is satisfied by the sections that follow.

Teleport will provide a simple CLI program to simplify the AKS Auto-Discovery setup.

When `teleport discovery bootstrap` detects that it has Azure discovery enabled and `aks` is defined in types, it allows a guided experience for assigning the necessary Azure RBAC permissions.

The available flags are:

- `-c`, `--config`: Path to a configuration file [/etc/teleport.yaml].
- `--manual`: When executed in "manual" mode, it will print the instructions to complete the configuration instead of applying them directly.
- `--policy-name`: Name of the Teleport agents policy. Default: "AKS Teleport Discovery"
- `--confirm`: Do not prompt user and auto-confirm all actions.
- `--attach-to-role`: Role name to attach policy to. Mutually exclusive with --attach-to-user. If none of the attach-to flags is provided, the command will try to attach the policy to the current user/role based on the credentials.
- `--attach-to-user`: User id to attach policy to. Mutually exclusive with --attach-to-role. If none of the attach-to flags is provided, the command will try to attach the policy to the current user/role based on the credentials.

The Teleport CLI will describe the clusters available in the AKS to extract their authorization modes and guide the user based on these values.

In the following subsections, we will present the process for each mode. However, the CLI will support clusters with different modes. The final guide will merge the permissions required to operate correctly.

##### Active Directory and Azure RBAC enabled

If any cluster has AD integration and Azure RBAC enabled, the CLI guide will be the following:

```bash
$ teleport discovery bootstrap \
    -c teleport.yaml \ 
    --manual \
    --attach-to-user {identityID} 

1. Create Azure Policy "AKS Teleport Discovery":
{
    "Name": "AKS Teleport Discovery",
    "Description": "Required permissions for Teleport auto-discovery.",
    "Actions": [
      "Microsoft.ContainerService/managedClusters/read"
    ],
    "NotActions": [],
    "DataActions": [
      "Microsoft.ContainerService/managedClusters/groups/impersonate/action",
      "Microsoft.ContainerService/managedClusters/users/impersonate/action",
      "Microsoft.ContainerService/managedClusters/serviceaccounts/impersonate/action",
      "Microsoft.ContainerService/managedClusters/pods/read",
      "Microsoft.ContainerService/managedClusters/authorization.k8s.io/selfsubjectaccessreviews/write",
      "Microsoft.ContainerService/managedClusters/authorization.k8s.io/selfsubjectrulesreviews/write",
    ],
    "NotDataActions": [],
    "assignableScopes": [
        "/subscriptions/{subscription_id}"
    ]
}

With $ az role definition create --role-definition @config.json

2. Assign the Policy into Teleport Identity


$ az role assignment create \
    --assignee {identityID} \ # can be app id or any user id.
    --role "AKS Teleport Discovery" \
    --scope "/subscriptions/{subscription_id}" # the scope can be limited

```

At this point, Teleport identity can list and access any cluster within the subscription.

##### Kubernetes Local Accounts

If any cluster runs with Local Accounts, the guide will be the following:

```bash
$ teleport discovery bootstrap \
    -c teleport.yaml \ 
    --manual \
    --attach-to-user {identityID} 

1. Create Azure Policy "AKS Teleport Discovery":
{
    "Name": "AKS Teleport Discovery",
    "Description": "Required permissions for Teleport auto-discovery.",
    "Actions": [
      "Microsoft.ContainerService/managedClusters/read",
      "Microsoft.ContainerService/managedClusters/listClusterUserCredential/action",
    ],
    "NotActions": [],
    "DataActions": [],
    "NotDataActions": [],
    "assignableScopes": [
        "/subscriptions/{subscription_id}"
    ]
}

With $ az role definition create --role-definition @config.json

2. Assign the Policy into Teleport Identity


$ az role assignment create \
    --assignee {identityID} \ # can be app id or any user id.
    --role "AKS Teleport Discovery" \
    --scope "/subscriptions/{subscription_id}" # the scope can be limited

```

At this point, Teleport identity can list and access the user credentials endpoint for any cluster in the subscription.

##### Active Directory with Kubernetes RBAC

If any cluster has AD with Kubernetes RBAC mode, the user experience depends on whether it's possible to allow Teleport to access  `aks:ListClusterAdminCredentials` or `aks:RunCommand` APIs.
If the answer is no, then the setup must follow the manual guide below.

##### Automatic cluster Authorization 

Teleport can automatically create the `ClusterRole` and `ClusterRoleBinding` resources 
in the following cases:
- Teleport's AD identity has permissions that allow access to the static [cluster administrator credentials](https://learn.microsoft.com/en-us/rest/api/aks/managed-clusters/list-cluster-admin-credentials) (local accounts).
- Teleport's AD identity belongs to the cluster's administrator group.
- Teleport's AD identity has permissions to create `ClusterRole` and `ClusterRoleBinding` on the 
cluster and permissions to execute [remote commands](https://learn.microsoft.com/en-us/rest/api/aks/managed-clusters/run-command).

###### Cluster with local admin account

The following command must be executed to configure the Teleport permissions.

```bash
$ teleport discovery bootstrap \
    -c teleport.yaml \ 
    --manual \
    --attach-to-user {identityID} 

1. Create Azure Policy "AKS Teleport Discovery":
{
    "Name": "AKS Teleport Discovery",
    "Description": "Required permissions for Teleport auto-discovery.",
    "Actions": [
        "Microsoft.ContainerService/managedClusters/read",
        "Microsoft.ContainerService/managedClusters/listClusterUserCredential/action",
        "Microsoft.ContainerService/managedClusters/listClusterAdminCredential/action"
    ],
    "NotActions": [],
    "DataActions": [],
    "NotDataActions": [],
    "assignableScopes": [
        "/subscriptions/{subscription_id}"
    ]
}

With $ az role definition create --role-definition @config.json

2. Assign the Policy into Teleport Identity


$ az role assignment create \
    --assignee {identityID} \ # can be app id or any user id.
    --role "AKS Teleport Discovery" \
    --scope "/subscriptions/{subscription_id}" # the scope can be limited

```

###### Cluster without local admin account

In this case, the CLI will configure permissions to execute remote commands on clusters, 
but to work correctly, it requires that the AD group to which the Teleport belongs has permissions to 
create `ClusterRole` and `ClusterRoleBinding` resources.

```bash
$ teleport discovery bootstrap \
    -c teleport.yaml \ 
    --manual \
    --attach-to-user {identityID} 

1. Create Azure Policy "AKS Teleport Discovery":
{
    "Name": "AKS Teleport Discovery",
    "Description": "Required permissions for Teleport auto-discovery.",
    "Actions": [
      "Microsoft.ContainerService/managedClusters/read",
      "Microsoft.ContainerService/managedClusters/listClusterUserCredential/action",
      "Microsoft.ContainerService/managedClusters/runcommand/action",
      "Microsoft.ContainerService/managedclusters/commandResults/read"
    ],
    "NotActions": [],
    "DataActions": [],
    "NotDataActions": [],
    "assignableScopes": [
        "/subscriptions/{subscription_id}"
    ]
}

With $ az role definition create --role-definition @config.json

2. Assign the Policy into Teleport Identity


$ az role assignment create \
    --assignee {identityID} \ # can be app id or any user id.
    --role "AKS Teleport Discovery" \
    --scope "/subscriptions/{subscription_id}" # the scope can be limited

```

The final step is to start Teleport Discovery agent which, using the `aks:RunCommand` API, will create the `ClusterRole` and `ClusterRoleBinding`.

###### Manual guide 

This manual guide is only applicable for clusters with Active Directory and Kubernetes RBAC.

```bash
az role definition create --role-definition @config.json
```

where `config.json` is:

```json
{
    "Name": "AKS Teleport Discovery",
    "Description": "Required permissions for Teleport auto-discovery.",
    "Actions": [
      "Microsoft.ContainerService/managedClusters/read",
      "Microsoft.ContainerService/managedClusters/listClusterUserCredential/action"
    ],
    "NotActions": [],
    "DataActions": [],
    "NotDataActions": [],
    "assignableScopes": [
        "/subscriptions/{subscription_id}"
    ]
}
```

To create the role assignment, execute the following command:

```bash
az role assignment create \
    --assignee {identity_id} \ # can be app id or any user id.
    --role "AKS Teleport Discovery" \
    --scope "/subscriptions/{subscription_id}" # the scope can be limited
```

Now, for each AKS cluster you want to enroll, you must create the `ClusterRole` and `ClusterRoleBinding` resources.

`ClusterRole` is available at Appendix A and the `ClusterRoleBinding` at Appendix B.

Once you define the permissions and the `ClusterRole` and `ClusterRoleBinding` resources exist, Teleport Discovery and Kubernetes services will discover and forward requests to the discovered clusters.

#### Resources watch

The discovery of new resources and watcher mechanism is built on top of [`Microsoft.ContainerService/managedClusters`][listclustersaks] API endpoint. The discovery mechanism is similar to the one described for AWS, and consists on calling the endpoint at regular intervals while managing the differences between iterations. 

The API endpoint returns the complete cluster configuration, including authentication options, `TenantID` and `resourceID` fields that are required for authentication. 

### Authentication

Azure has several options to authenticate against an AKS cluster depending on the Kubernetes cluster version and whether the Azure Active Directory integration is enabled or not.

The Kubernetes Service will adapt the login method based on the authentication configuration of the cluster. This data is available in the cluster config returned by [`Microsoft.ContainerService/managedClusters`][listclustersaks] and the Kubernetes Service adapt the authorization method accordingly.

#### With AD integration enabled

For Kubernetes clusters after version `v1.22` with Active directory integration enabled, Azure forces the authentication to happen via a short-lived token. In this case, Teleport grants its access to the Kubernetes API by a Bearer token generated by calling [`AAD/Token`][aadtoken] endpoint with the cluster's `TenantID` and a fixed `Scope` with a value equal to [`6dae42f8-4368-4678-94ff-3960e28e3630`](https://github.com/Azure/kubelogin#exec-plugin-format). 

#### With AD integration disabled

For clusters without Active directory integration, Teleport will use local accounts. The credentials used will be those returned by the [`aks:ListClusterUserCredentials`][listclusterusercredentials] endpoint. This endpoint returns a `kubeconfig` that, after parsing, contains the user credentials to access the cluster API. 

### Authorization

In this section we will describe how Teleport will access clusters with different authentication and authorization options. It will also describe scenarios where Teleport may grant access to itself for clusters running with AD with Kubernetes RBAC. 

#### Active Directory and Azure RBAC enabled

If the Azure AKS cluster has Azure AD and Azure RBAC enabled, the Azure Role definition grants access one or more Kubernetes clusters. The list of valid RBAC rules can be found [here](https://docs.microsoft.com/en-us/azure/role-based-access-control/resource-provider-operations#microsoftcontainerservice). 

Since the authorization is specified at the Role level and must be attached to the identity that the Kubernetes and Discovery agents are running with. Users must manually create the Role and attach it to the Teleport Identity in advance, otherwise Teleport will not be able to gain access. 

This operation will be performed only once, and therefore we can provide a CLI that guides the user in setting the necessary identity permissions. 

The Azure RBAC representation of the minimum access level that agents require is as follows:

```json
{
    "Name": "AKS Teleport Discovery Permissions",
    "Description": "Required permissions for Teleport auto-discovery.",
    "Actions": [],
    "NotActions": [],
    "DataActions": [
      "Microsoft.ContainerService/managedClusters/pods/read",
      "Microsoft.ContainerService/managedClusters/users/impersonate/action",
      "Microsoft.ContainerService/managedClusters/groups/impersonate/action",
      "Microsoft.ContainerService/managedClusters/serviceaccounts/impersonate/action",
      "Microsoft.ContainerService/managedClusters/authorization.k8s.io/selfsubjectaccessreviews/write",
      "Microsoft.ContainerService/managedClusters/authorization.k8s.io/selfsubjectrulesreviews/write",
    ],
    "NotDataActions": [],
    "assignableScopes": [
        "/subscriptions/{subscription_id}"
    ]
}
```

If the Azure RBAC grants the access required by the agents, the cluster is created and proxied by the Kubernetes agent.

#### Kubernetes Local Accounts

For clusters without AD enabled, Teleport will use the credentials provided by [`aks:ListClusterUserCredentials`][listclusterusercredentials]. The call returns a `kubeconfig` file populated with Cluster CA and authentication details.

To work in this mode, the agents need the `Microsoft.ContainerService/managedClusters/listClusterUserCredential/action` permission. This permission must be included in the Teleport identity role beforehand.

The required role must have the following configuration:

```json

    "permissions": [
      {
        "actions": [
          "Microsoft.ContainerService/managedClusters/read",
          "Microsoft.ContainerService/managedClusters/listClusterUserCredential/action",
        ],
        "dataActions": [],
        "notActions": [],
        "notDataActions": []
      }
    ]      
```

If Teleport services cannot access [`aks:ListClusterUserCredentials`][listclusterusercredentials], they are unable to gain access to the cluster and cannot enroll it. If it happens, an error will be printed mentioning that access to [`aks:ListClusterUserCredentials`] is required.

#### Active Directory enabled without Azure RBAC

For clusters with AD enabled but without Azure RBAC integration, operators must manually create the RBAC policies and bind them into the user/app principal. A detailed guide is available [here](https://docs.microsoft.com/en-us/azure/aks/azure-ad-integration-cli#create-kubernetes-rbac-binding).

To simplify the process, Teleport can create RBAC policies using less secure APIs, but the process depends on whether the cluster has local accounts enabled.

The first step, which is independent of local account existence, is to check if the Teleport already has access to the cluster. The access may exist because it was manually created or because the Teleport has configured it in the past. If authorized, the agent enrolls the cluster, otherwise it can take the following actions.

##### Enabled Local Accounts

If local accounts option is enabled, Azure created admin credentials during the cluster provision. If the agent has access to [`aks:ListClusterAdminCredentials`][listclusteradmincredentials] then it could use the returned credentials to create the Teleport RBAC `ClusterRole` and create a `ClusterRoleBinding` that binds the cluster role into Teleport's `group_id`.

In order to access [`aks:ListClusterAdminCredentials`][listclusteradmincredentials], the agent's identity must include `Microsoft.ContainerService/managedClusters/listClusterAdminCredential/action` permission.
The role must have the following configuration:

```json

    "permissions": [
      {
        "actions": [
          "Microsoft.ContainerService/managedClusters/read",
          "Microsoft.ContainerService/managedClusters/listClusterAdminCredential/action",
        ],
        "dataActions": [],
        "notActions": [],
        "notDataActions": []
      }
    ]      
```

If the [`aks:ListClusterAdminCredentials`][listclusteradmincredentials] returned the admin credentials, the Kubernetes service creates, in the AKS cluster, the `ClusterRole` and `ClusterRoleBinding`. Then, Teleport will use [`AAD/Token`][aadtoken] method to generate a Bearer token that allows accessing the cluster.

If access to [`aks:ListClusterAdminCredentials`][listclusteradmincredentials] is denied by lack of policy, the Disabled Local accounts method can be used as a fallback.

##### Disabled Local accounts

Teleport, under these conditions, has no way to grant access to the cluster because [`aks:ListClusterAdminCredentials`][listclusteradmincredentials] and [`aks:ListClusterUserCredentials`][listclusterusercredentials] both return `exec` kubeconfigs and the agent's role mapping does not exist yet.

Since direct access is unavailable, Teleport can delegate in Azure the responsibility of creating the `ClusterRole` and `ClusterRoleBinding`. This operation can happen if the agent has access to the `aks:Command` API. It allows you to run indiscriminate commands on the cluster and, as such, would allow the creation of `ClusterRole` and `ClusterRoleBinding`.

Once the agent creates the command request, AKS provisions a new POD with admin permissions and executes the specified command. The pod already has `kustomize`, `helm`, and `kubectl` binaries installed.

The usage of this API should only happen in the last resource because the command scope cannot be limited and virtually anything is liable to be executed.

Access to this API requires that the agent's identity must include `Microsoft.ContainerService/managedClusters/runcommand/action` and `Microsoft.ContainerService/managedclusters/commandResults/read` permissions.

```json

    "permissions": [
      {
        "actions": [
          "Microsoft.ContainerService/managedClusters/read",
          "Microsoft.ContainerService/managedClusters/runcommand/action",
          "Microsoft.ContainerService/managedclusters/commandResults/read"
        ],
        "dataActions": [],
        "notActions": [],
        "notDataActions": []
      }
    ]      
```

If the operation was successful, the discovery service creates the dynamic `kube_cluster` in the Auth Server. Then, The Kubernetes service will use [`AAD/Token`][aadtoken] method to generate a Bearer token that allows accessing the cluster.

If the agent identity does not grant access to the `aks:RunCommand` API, it's not possible to enroll the cluster and an error will be returned.

### Limitations

In cases where Active Directory or Azure RBAC options are disabled and the Teleport RBAC permissions (if AD is enabled) does not exist, the agent uses insecure APIs to create access. It requires the usage of APIs that expose long-lived admin credentials or methods that allow running commands in the cluster as an administrator and where command scope limits do not exist. To extract the full potential of AKS auto-discovery, we recommend that AD and AZ RBAC are enabled. 

To extract the full potential of AKS auto-discovery, we recommend that AD and AZ RBAC are enabled.

## GCP GKE discovery

The following subsections will describe the details required for implementing GKE auto-discovery. 

The Discovery service can auto-discover GKE clusters in the GCP account it has credentials for by matching their resource tags. 

Authentication on an GKE cluster happens using the same OAuth2 token used to access any other GCP service and authorization is associated with the roles attached to the GCP Service Account the Discovery and Kubernetes Services are using to access the cluster. 

#### IAM Permissions 

Create a role with the following spec and assign it to the GCP Service Account.

```yaml
description: 'Kubernetes GCP Auto-Discovery'
includedPermissions:
# allow getting and listing GKE clusters
- container.clusters.get
- container.clusters.list
# Kubernetes permissions for impersonation + get pods
- container.clusters.impersonate
- container.pods.get
- container.selfSubjectAccessReviews.create
- container.selfSubjectRulesReviews.create
name: projects/{projectID}/roles/KubeDiscovery
stage: GA
title: KubeDiscovery
```

Given a role with the above permissions, Teleport can access any cluster without the need for manual configuration of permissions and thus can discover and forward requests to every cluster available.

### UX


#### Configuration

The snippets below configure the Discovery Service to watch GKE resources with `tag:env=prod` on any location that belong to project `p1` or `p2`. It also configures Kubernetes Service to watch Teleport `kube_cluster` resources that include the label `env=prod`.

##### Discovery Service

The Teleport configuration for automatic GKE discovery will have the following structure

```yaml
discovery_service:
  enabled: yes
  gcp:
  - locations: ["*"]
    types: ["gke"]
    project_ids: ["p1","p2"]
    tags:
      "env": "prod"
```

##### Kubernetes Service

In the Kubernetes Service we have to configure the `resources` monitoring section with the cluster tags that this agent is able to serve.

```yaml
## This section configures the Kubernetes Service
kubernetes_service:
    enabled: "yes"
    resources:
    - labels:
        "env": "prod" # can be configured to limit the clusters to watched by this service.
```

#### `teleport discovery bootstrap`

Teleport will provide a simple CLI program to simplify the GCP Auto-Discovery process and cluster permissions management. 

When `teleport discovery bootstrap` detects that it has GCP discovery enabled and `gke` is defined in types, it will create the required IAM role and assign it to the Teleport GCP Service Account.

```shell
$ teleport discovery bootstrap --gcp-sa=sa@gcp...
[1] Connecting to the GCP environment...
[2] Checking your user permissions....
[3] Validating Teleport Service Account....
[4] Attaching IAM permissions....
```

After the command finishes, the Discovery and Kubernetes Services can start.

### Resources watcher

The discovery of new resources and watcher mechanism is built on top of [`container.clusters.list`](https://cloud.google.com/sdk/gcloud/reference/container/clusters/list) API endpoint. The discovery mechanism is similar to the one described for AWS, and consists on calling the endpoint at regular intervals while managing the differences between iterations.

The API endpoint returns the complete cluster configuration, including labels and clusters' status.

### Authentication & Authorization

This section defines the GKE token generation (authentication) and the authorization required for Teleport to work with the cluster. 

#### Authentication

Access to the GKE cluster is granted by sending a Google OAuth2 token as Authorization header. To generate this token, Teleport will delegate the token creation into [google/oauth2](https://pkg.go.dev/golang.org/x/oauth2/google) library. To generate a GCP Auth Token, Teleport must specify the Kubernetes Engine scope - `https://www.googleapis.com/auth/cloud-platform`.

The token's TTL is 1 hour and Teleport will revalidate it once it's near expiration.  

#### Authorization

Similarly to the Azure AD with Azure RBAC access mode, authorization in GKE clusters is associated with the IAM roles assigned to the GCP Service Account used.

That said, the Teleport Kubernetes Service requires the following permissions:

- `container.clusters.list`
- `container.clusters.get`
- `container.pods.get`
- `container.selfSubjectAccessReviews.create`
- `container.selfSubjectRulesReviews.create`
- `container.clusters.impersonate` 

Although, `container.clusters.impersonate` is hidden from GCP permissions listings, the GKE cluster forces it to be present when performing Impersonation requests.

## Security

### AWS

From a security perspective, the `eks:DescribeCluster` and `eks:ListClusters` methods do not give direct access to the cluster and only allow the callee to grab the endpoint and CA certificate.

The credentials that grant access into the cluster are pre-signed, independent for each cluster and are short-lived - 15 minutes.

In order to Teleport to be able to connect to the cluster, its role must be present in `aws-auth` configmap with the desired group permissions. As described above, the permissions required for Teleport to operate in this mode do not give him the possibility to create or delete resources, that it can only happen via impersonation.

### Azure

[`Microsoft.ContainerService/managedClusters`][listclustersaks] does not immediately grant access to AKS clusters, and depending on the cluster version and authentication and authorization mode, the access modes are different.

If AD is enabled, the authentication details are short-lived and must be revalidated each time their TTL is about to expire. The call to [`AAD/Token`][aadtoken] returns the expiration time of the credentials. After their expiration time reaches, the token no longer grants access to the cluster. Each cluster has a different authentication token.

In clusters without AD or Azure RBAC enabled, the access requires either permanent local Kubernetes accounts whose credentials are long-lived, the usage of insecure APIs like `aks:Command` or extracting the admin credentials from [`aks:ListClusterAdminCredentials`][listclusteradmincredentials]. 
The only method to revoke access to the certificate key pair and the automatic token returned by `aks:ListCluster*Credentials` is to rotate cluster CA authority. On the other hand, `aks:Command` allows the execution of arbitrary commands in the cluster with the administrator role. It is a security concern because any attacker that escalates privileges to the agent's role can execute destructive commands without limits because `aks:Command` does not allow you to validate the commands executed.

From a security perspective, it is highly recommended use Azure Active Directory and Azure RBAC enabled for any cluster.

### GCP

The credentials that grant access to the cluster are generated by [google/oauth2](https://pkg.go.dev/golang.org/x/oauth2/google) and have a maximum associated TTL of 1 hour. After the expiration date, the token can no longer be used to access the cluster and is invalid.

## Requirements

Requirements section describes required actions to be able to introduce dynamic registration of new clusters in Teleport cluster.

### Introduce `KubernetesServerV3` and deprecate `ServerV2` usage for Kubernetes Servers

This RFD proposes the deprecation of `ServerV2` objects for representing Kubernetes Servers in Teleport API. Currently, [`ServerV2`][serverv2] has a field `KubernetesClusters` which holds the list of Kubernetes clusters proxied by the Kubernetes Service.

To remove usage of `ServerV2` for servers other than the SSH server, we must introduce the `KubernetesServerV3`, which holds the information about the cluster proxied as well as the server status. Each Kubernetes cluster is represented by a different `KubernetesServerV3` object.

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
    // Cluster is a Kubernetes Cluster proxied by this Kubernetes server.
    KubernetesClusterV3 Cluster = 5 [ (gogoproto.jsontag) = "cluster" ];
    // ProxyIDs is a list of proxy IDs this server is expected to be connected to.
    repeated string ProxyIDs = 6 [ (gogoproto.jsontag) = "proxy_ids,omitempty" ];
}

```

`KubernetesServerV3` message will be added as a new field to the [`PaginatedResource`][paginatedresource] message, and the current `KubeService` field will be deprecated.

With `KubernetesServerV3`, Teleport can create a different Kubernetes Server for each Kubernetes cluster available, and it's possible to create and stop Heartbeats independently without interrupting access to other Kubernetes clusters served by the same Kubernetes service.

## Future work

Understand how to simplify the cluster enroll process, in particular how to simplify the Kubernetes permissions mapping from a IAM role.

## Links

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
[listclustersaks]:https://docs.microsoft.com/en-us/azure/templates/microsoft.containerservice/managedclusters?pivots=deployment-language-bicep
[aadtoken]:https://docs.microsoft.com/en-us/azure/databricks/dev-tools/api/latest/aad/app-aad-token
[listclusterusercredentials]:https://docs.microsoft.com/en-us/rest/api/aks/managed-clusters/list-cluster-user-credentials?tabs=HTTP
[listclusteradmincredentials]:https://docs.microsoft.com/en-us/rest/api/aks/managed-clusters/list-cluster-admin-credentials?tabs=HTTP

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

## Appendix

### A: Teleport RBAC `ClusterRole`

Appendix A lists the minimum required RBAC roles that Teleport Kube Service requires to be able to correctly forward and impersonate requests on user's behalf.

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: teleport
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

#### B: Teleport RBAC `ClusterRoleBinding`

Appendix B represents the cluster `ClusterRoleBinding` that connects Teleport Identity and the RBAC policy specified. The placeholder `{group_name}` is replaced with the group value associated with the Teleport identity. For AKS clusters it is the Kubernetes group while on Azure if the AD integration is enabled it is the AD group ID.

```yaml
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: teleport
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: teleport
subjects:
- kind: Group
  name: {group_name}
  apiGroup: rbac.authorization.k8s.io
```

