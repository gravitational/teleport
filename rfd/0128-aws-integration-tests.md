---
authors: Tiago Silva (tiago.silva@goteleport.com)
state: implemented (14.0)
---

# RFD 128 - AWS End To End Tests

## Required Approvers

- Engineering: `@r0mant && @smallinsky`
- Security: `@reedloden || @jentfoo`

## What

As part of the increase of the reliability of Teleport, we want to add end-to-end (E2E)
tests for AWS. This will allow us to test the integration with AWS without having
to rely on manual testing.

## Why

We want to increase the reliability of Teleport by adding E2E tests for
AWS. This will allow us to test the integration with AWS without having to rely
on manual testing. This will also allow us to test the integration with AWS in
a more reliable process to ensure we don't introduce regressions in the future
or if we do, we can catch them early.

Teleport integrates deeply with AWS to provide a seamless experience for users
when using auto-discovery of nodes, databases and Kubernetes clusters. This
integration is critical for the success of Teleport and we want to ensure that
we can test it reliably.

Each E2E test will use the minimum required AWS API permissions to test the integration.
This will ensure that we don't introduce regressions by changing the permissions
required by Teleport but those changes are not detected because another test
requires the same permissions.

We are describing the E2E tests for AWS in this RFD but the same process will
be used to add E2E tests for other cloud providers. With that in mind, we
should ensure that the process is generic enough to be used for other cloud
providers.

## When

AWS E2E tests will be added to the Teleport CI pipeline as part of the
tests that run on each PR. This will ensure that we can catch regressions
early and fix them before they are merged into the master branch.

For Kubernetes access, tests are easily parallelizable since each test can run
in isolation and does not interfere with the cluster state.

## How

This section describes how the end-to-end tests will authenticate with AWS API
and how they will be authorized to perform the required actions.

Teleport AWS account will be configured to allow GitHub OIDC provider to assume
a set of roles. These roles will be assumed by the GitHub actions pipeline to
interact with AWS API.

GitHub action [configure-aws-credentials](https://github.com/aws-actions/configure-aws-credentials)
will be used to handle the authentication with AWS API. This action configures
AWS credentials and region environment variables for use in subsequent steps.
The action implements the AWS SDK credential resolution chain and exports the
following environment variables:

- `AWS_ACCESS_KEY_ID`
- `AWS_SECRET_ACCESS_KEY`
- `AWS_SESSION_TOKEN`

Once the environment variables are set, the AWS SDK that Teleport uses will
automatically use them to authenticate with the AWS API and does not require
any additional configuration.

One of the requirements for the E2E tests is that they should use the
minimum required permissions to perform the required actions. This will ensure
that we don't introduce regressions by changing the permissions required by
Teleport. To achieve this, GitHub action will be allowed to assume a set of
roles that will simulate the minimum required permissions that each Teleport
service requires to perform the required actions. Teleport services will be
configured to assume these roles when running the E2E tests - requires
changes to the Teleport configuration - so we can test them in isolation.

The AWS account will be configured with a set of roles that will be assumed by
the GitHub actions pipeline. These roles will be configured with the minimum
required permissions to run the E2E tests. This will ensure that we
don't introduce regressions by changing the permissions required by Teleport
but those changes are not detected because another test requires the same
permissions.

AWS account configuration will be handled by IAC and will be stored in the
cloud-terraform repository. This will allow us to track changes to the configuration
and ensure that we can revert them if needed while also allowing us to review
the changes before they are applied. Each End-to-End test will run in a separate
Job so we can configure the assumed role for each test. End-to-End action
requires `id-token: write` permission to the GitHub OIDC provider to generate
the OIDC token that will be used to authenticate with AWS API.

## Tests

This section describes the E2E tests that will be added to the Teleport
and what they will test.

### Kubernetes Access

Teleport supports the automatic discovery of Kubernetes clusters running on AWS.
This is done by using the AWS API to discover the Kubernetes clusters running on
the account that matches the configured labels and then using the Kubernetes API
to forward the requests to the cluster.

The E2E tests for Kubernetes access will spin up the following Teleport
services:

1. Auth Server
2. Proxy
3. Discovery service
4. Kubernetes Service

Teleport Auth and Proxy configurations are similar to the configurations we
use for other tests. The discovery service will be configured to discover the healthy EKS
clusters with `env: ci` tag. The Kubernetes service will be configured to
watch clusters discovered by the discovery service with `env: ci` label.

#### Discovery service config

E2E tests will configure the Discovery service to discover EKS clusters with
`env: ci` tag. The snippet below shows the configuration that will be used
by the E2E tests.

```yaml

discovery_service:
    enabled: "yes"
    # discovery_group is used to group discovered resources into different
    # sets. This is useful when you have multiple Teleport Discovery services
    # running in the same cluster but polling different cloud providers or cloud
    # accounts. It prevents discovered services from colliding in Teleport when
    # managing discovered resources.
    discovery_group: "ci"
    aws:
     - types: ["eks"]
       # AWS regions to search for resources from
       regions: ["us-east-1", "us-west-1"]
       tags:
         "env": "ci"

```

#### Kubernetes service config

E2E tests will configure the Kubernetes service to watch `kube_cluster` objects
with label `env:ci` and serve them. The snippet bellow shows the configuration that
will be used.

```yaml
kubernetes_service:
  enabled: "yes"
  resources:
  - tags:
      "env": "ci"
```

Once the Teleport components are running, the test expects that the Kubernetes
service starts heartbeat the clusters discovered by the discovery service.
`tsh kube ls` will be used to monitor the clusters being served by the Kubernetes
service. Once the cluster shows up in the list, we consider it ready.

After the initial discovery phase, E2E tests will use `tsh` to simulate client connections
to the cluster and run a simple command to ensure that the connection works and is forwarded as expected.
`tsh kube login <kube_cluster>` will be used to generate the kubeconfig, and
`tsh kubectl get service` will be used to test the connection to the cluster.

#### Requirements

This section describes the requirements for the E2E tests for Kubernetes
access.

- One or more EKS control plane clusters running on AWS. We don't need that the
  clusters are running any workloads, we only need that their control plane is
  running and accessible.
- Discovery service configured to discover the EKS clusters and with List and Describe
  permissions to the EKS API. [Permissions](https://goteleport.com/docs/kubernetes-access/discovery/aws/#step-13-set-up-aws-iam-credentials)
- Kubernetes service IAM role configured with the minimum required permissions
  to generate the short-lived token and forward the requests to the Kubernetes
  API. [IAM Mapping](https://goteleport.com/docs/kubernetes-access/discovery/aws/#iam-mapping)

Spin up the EKS clusters takes a long time and it's not feasible to do it for
each test. To speed up the process, we should use an existing EKS cluster that
is already running, accessible and configured to allow Teleport Kubernetes service
to access it. This will allow us to run the integration tests without having to
wait for the EKS cluster to be created. Since we can interact with the EKS API
and we do not need to run any workloads on the cluster, the existing EKS cluster
does not need to have dedicated nodes to run workloads. A single control plane
deployed on a single availability zone is enough. To ensure that the EKS cluster
is up to date and we don't have to deal with the cluster upgrades having
major impacts on the development, we will run the Terraform script to destroy
and recreate the cluster during the weekend. This ensures that the EKS cluster is
available for the E2E tests during the week (with minor downtime on weekends)
and that we don't have to deal with cluster upgrades/security patches. The
recreation of the cluster will be triggered by Spacelift scheduled jobs and will
run every Sunday at 12:00 AM PST.

## Security

Several security considerations must be to be taken into account
when implementing the E2E tests for AWS.

### GitHub OIDC Provider

GitHub OIDC provider will be used to authenticate with AWS API. This will allow
us to use GitHub actions to run the E2E tests without having to manage
AWS credentials. GitHub actions will be allowed to assume a set of roles that
will simulate the minimum required permissions that each Teleport service
requires to perform the required actions. An important consideration is that
all other OIDC tokens from GitHub that don't belong to the Teleport Enterprise
Account actions or originated from other repositories besides `gravitational/teleport`
must be rejected. This includes tokens from other GitHub
organizations and tokens from the Teleport Enterprise Account that don't belong
to the GitHub actions pipeline.

This is achieved by configuring the AWS federation to only allow JWT tokens with:

1. The issuer - `iss` - set to `token.actions.githubusercontent.com`
2. The audience - `aud` - set to `sts.amazonaws.com`
3. The subject - `sub` - set to `repo:gravitational/teleport:*`. The last `*` matches
    any branch in the `gravitational/teleport` repository and we can further
    restrict it to only allow the `master` branch.

### Credential Lifetime

The default session duration is 1 hour when using the OIDC provider to directly
assume an IAM Role.

We can reduce the session duration with the `role-duration-seconds` parameter
to reduce the risk of credentials being leaked. This parameter can be set to
the timeout of the GitHub action to ensure that the credentials are not valid
after the action finishes.

### Least Privilege

The GitHub actions pipeline will be allowed to assume a set of roles that will
simulate the minimum required permissions that each Teleport service requires
to perform the required actions. We need to protect these roles from being
too permissive or being used to escalate privileges to other resources in the
AWS account.

### AWS Account

The AWS account used to run the E2E tests must be isolated from the other AWS
accounts used by Teleport. This will prevent the E2E tests from affecting the
other AWS accounts and will allow us to easily clean up the resources created
by the E2E tests. It will also prevent devs from mistakenly deleting resources used
by E2E tests because they won't have admin privileges into `ci` account as
opposed to `dev` accounts. The AWS account reserved for CI is `teleport-integration-test-prod`.

## Future Work

This RFD focuses on the E2E tests for AWS but the same approach will be
pursued for other cloud providers. The next step will be to implement the same
E2E tests for GCP and Azure.