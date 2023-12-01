---
authors:  Anton Miniailo (anton@goteleport.com)
state: Draft
---

# RFD 0157 - AWS EKS Discover Integration

## Required Approvals

* Engineering: @r0mant && (@tigrato || @marcoandredinis)
* Product: @xinding33 || @klizhentas
* Security: @reedloden || @jentfoo

## What

Add a guided workflow to the Discover UI for enrolling AWS Elastic Kubernetes Service (EKS) clusters.

## Why

At present, the process for adding an EKS cluster to Teleport is somewhat technical. Users need to either manually install the Teleport agent on the cluster or manually set up and run discovery and kube services for EKS.
We aim to lower the friction of adding EKS clusters to Teleport by providing a seamless experience. Creating a guided UI workflow will help users onboard
their EKS clusters more quickly and with less effort.

## Scope

This RFD focuses on Amazon EKS clusters. A similar approach can be taken later for other specialized Kubernetes providers, such as
Azure AKS and Google GKE.

## Details

### UX

We will build a workflow similar to the ones we already have for AWS RDS and EC2 enrollment.

![EKS screen](assets/0157-eks-screen.png)

General steps of the workflow:

1. AWS Integration setup
2. Clusters Enrollment
3. Setup access
4. Test connection

AWS Integration for EKS will require permissions to list/describe EKS clusters and associate an identity provider
to an EKS cluster (details [below](#eks-identity-provider-association)). AWS permissions config:

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "eks:DescribeCluster",
                "eks:ListClusters",
                "eks:AssociateIdentityProviderConfig"
            ],
            "Resource": "*"
        }
    ]
}
```

We will reuse existing code for AWS Integration setup using these new permissions.

After setting up AWS integration, users will see a list of the EKS clusters in the next step.
There, they will be able to use checkboxes in the table to select the cluster(s) they want to enroll.
We will determine which clusters are already enrolled and present in the Teleport inventory; those clusters
will be greyed out in the table.

Users will also have an option to enable Kubernetes App Discovery in the enrolled clusters, this option
will be enabled by default.

Users will have two main options for enrolling EKS clusters:
 - Enroll specific selected clusters from the list of available ones.
 - Set up automatic discovery and enrollment of the clusters using OIDC integration.

### Enrolling Selected Clusters

For the enrollment of just the selected clusters, users will be further given two choices:
- Manually run a script in the AWS CloudShell.
- Use EKS OIDC identity provider association to enroll a cluster with just the API.

Cluster enrollment will be done through the installation of the Teleport agent using the Helm chart
`teleport-kube-agent`. 

For the AWS CloudShell, the command we generate will download a script
that will run the Helm installation of the chart on the clusters selected by the user.

For completely automated cluster enrollment through the API, the Helm Go SDK will be used -
allowing us to avoid the need for an additional step to run Helm itself elsewhere. It would require
running a job inside the target EKS cluster with a special image that has Helm on it, and this job
would install the Teleport kube agent. By using the Helm Go SDK, we can run the installation directly from the Teleport
process and don't need to maintain a new special image and EKS installation job.

### Automatic Discovery and Enrollment

Users will also be able to set up automatic discovery and enrollment of EKS clusters in the Discover UI.
We will reuse the existing EKS discovery capability, although a new mode will be added to it - discovery through
the AWS integration. EKS discovery in that mode will rely on the AWS integration, selected in the Discovery UI,
to find clusters and then install the Helm chart on them. The algorithm will be as follows:
- List EKS clusters through the AWS integration.
- Cross-check to see which EKS clusters are already registered in the system.
- Once a new EKS cluster is found, associate our OIDC provider with it.
- Using the associated provider, generate an access token and install the kube agent through the Helm chart.

Users will be able to filter EKS clusters for discovery by using labels.

A dynamic discovery configuration will be created as a result of users setting up automatic EKS discovery in the Discover UI.
This configuration can be picked up by any discovery service. In the Teleport cloud, the Discovery service will be available by default,
meaning users will not need to perform any additional actions.

### EKS Identity Provider Association

AWS EKS supports a method of authentication that involves associating an OIDC identity provider with the cluster.
Such an associated identity provider can issue access tokens (JWT) that grant access to the EKS cluster groups and users.
Since Teleport already supports OIDC providers, we can easily use this approach. It will require slight
modifications to the AWS OIDC integration code to allow it to issue JWTs for an EKS cluster. To associate an OIDC provider with an
EKS cluster, the selected AWS integration needs to issue an API call `eks:AssociateIdentityProviderConfig`
with the following configuration (includes example parameters):

```yaml
---
apiVersion: eksctl.io/v1alpha5
kind: ClusterConfig

metadata:
  name: eks-cluster-name
  region: us-east-1

identityProviders:
  - name: TeleportProvider
    type: oidc
    issuerUrl: https://teleport.example.com
    clientId: kubernetes
    usernameClaim: email
    groupsClaim: sub
```

To access the target EKS cluster in API mode, we will generate an access token using the associated OIDC provider.
That token will grant access to the `system:masters` group, so the Helm chart installation will be able to set up
all the required infrastructure for the Teleport kube agent, including service accounts.

## Product Usage

We will add a new PostHog event to track the usage of the Discover flow for EKS.

```protobuf
// UIDiscoverEKSClusterEnrollEvent is emitted when a user is finished with
// the step that asks user to select from a list of EKS clusters.
message UIDiscoverEKSClusterEnrollEvent {
  DiscoverMetadata metadata = 1;
  DiscoverResourceMetadata resource = 2;
  DiscoverStepStatus status = 3;
  int64 selected_resources_count = 4;
}
```

## Security

New permissions will be required for the AWS OIDC integration to perform the necessary tasks. It will need
to be able to list EKS clusters as well as associate an OIDC provider with them. An associated OIDC provider can
generate authentication tokens that give full control over the cluster. We will only use those tokens to
perform the initial installation of the `teleport-kube-agent` Helm chart. The agent itself will be configured with
the standard service account permissions it requires. After the Helm chart installation succeeds, Teleport doesn't need
to have an OIDC provider associated with the enrolled EKS cluster, and clients can remove the association or even put
a block on OIDC provider association for the cluster if they want.

## Future considerations

At the moment, the association of an OIDC provider with an EKS cluster allows for the best automation regarding cluster
enrollment. This approach completely sidesteps the IAM method of EKS authentication, which currently lacks the flexibility
we would require. Currently, IAM authentication in EKS is controlled by the `aws-auth` config map, which needs to be
manually edited to allow IAM roles to have access to the cluster, and there's no API for that. Amazon is working on
implementing an API for controlling `aws-auth`, but it's still work in progress (https://github.com/aws/containers-roadmap/issues/185).
Once the aforementioned API is implemented, we might review the access pattern for the AWS EKS integration and compare the OIDC provider
solution with the new API.

## Alternative

We could enroll clusters not by installing the kube agent through the Helm chart, but by proxying requests through the
Kube service running outside of the EKS cluster. However, this would require clients to run a dedicated Kube service
and would make permissions management more complicated due to the aforementioned issue with the EKS `aws-auth` config map.
We would also lose the capability of auto-discovering apps inside EKS clusters, since the current Kubernetes App Auto-Discovery
feature works only with the Teleport agent deployed inside the cluster.