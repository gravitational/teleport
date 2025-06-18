---
authors: Noah Stride (noah@goteleport.com)
state: draft
---

# RFD 215 - MWI Terraform Provider 

## Required Approvals

- Engineering: (TimB (@timothyb89) || DanU (@boxofrad)) && Hugo (@hugoShaka)
- Product: DaveS (@thedevelopnik) 

## What

Introduce functionality to a Terraform provider to provide credentials for
other Terraform providers to access resources via Teleport Machine & Workload
Identity.

## Why

Teleport provides access to many kinds of resources (e.g Kubernetes clusters,
AWS accounts, GCP projects, etc.) that customers manage with Terraform. Today,
these customers may use long-lived credentials to access these resources and
have poor insight into what resources these credentials grant access to.

Some customers have explored leveraging Teleport's Machine and Workload Identity
to provide short-lived credentials to Terraform providers. However, this 
requires the ability to run the `tbot` binary within the environment that the
Terraform plan/apply runs (which excludes environments like Terraform Cloud) and
this implementation is generally cumbersome.

Providing the ability to generate short-lived credentials within a Terraform
provider for resources protected by Teleport will:

- Allow customers to eliminate the use of long-lived static credentials with
  high levels of privilege for their Terraform CI/CDs access to resources.
- Allow customers to better understand and control what resources these
  Terraform CI/CDs can access.
- Simplify existing Terraform CI/CDs where `tbot` runs outside the Terraform
  plan/apply itself.

## Details

Today, we already have basic support for embedding `tbot` functionality within
Go binaries. We leverage this already for allowing the Teleport terraform
provider to authenticate to a Teleport cluster using a join token.

Due to the distinct nature of the credentials and configuration required for
the various resources that we support, the Terraform provider will require
handwritten implementation for each resource type that we wish to support.

Based on design partner feedback, we'll initially focus on the following:

- Kubernetes
- AWS via Roles Anywhere

### Providing credentials within a Terraform plan/apply

Terraform Providers typically provide some method of providing credentials
directly within their arguments within the Terraform configuration.

For example, Kubernetes:

```hcl
provider "kubernetes" {
  host                   = "my-cluster.example.com"
  client_certificate     = file("~/.kube/client-cert.pem")
  client_key             = file("~/.kube/client-key.pem")
  cluster_ca_certificate = file("~/.kube/cluster-ca-cert.pem")
}
```

For example, AWS:

```hcl
provider "aws" {
  region     = "us-west-2"
  access_key = "my-access-key"
  secret_key = "my-secret-key"
}
```

The values provided to the provider in configuration do not have to be static
values and can be sourced dynamically (e.g from data sources, variables etc)

#### Using data sources

Terraform includes a kind of object known as a "Data Source". Data sources
are designed to surface read-only information from a provider. Typically, this 
is used to read information about an existing resource that is not managed
directly by that Terraform configuration.

Leveraging data sources would look like:

```hcl

data "mwi_aws_roles_anywhere" "account" {
  # Various configuration here needed to generate credentials 
}

provider "aws" {
  region     = "us-west-2"
  access_key = data.mwi_aws_roles_anywhere.account.access_key
  secret_key = data.mwi_aws_roles_anywhere.account.secret_key
}
```

Generally, it appears safe to use values from a data source as an input to
another provider:

- It would appear that this pattern is at least shown in examples in relation
  to the Vault and AWS providers:
  https://github.com/hashicorp-education/learn-terraform-inject-secrets-aws-vault/blob/main/operator-workspace/main.tf
- It would appear that historically there have been bugs related to this 
  pattern which have been resolved:
  https://github.com/hashicorp/terraform/issues/11264
- It would appear that data sources, were at least partially, introduced to
  solve for this use-case:
  https://github.com/hashicorp/terraform/issues/4169

However, one key limitation is that the value of a data source will only be
computed once - either during plan or during apply. The result of that
computation will be stored in the Terraform state. This means that if the 
apply runs long enough after the plan, the credentials may have expired.

Positives:

- Well-supported by a number of Terraform versions and forks like OpenTofu

Negatives:

- Data source outputs are stored in the Terraform state, which can be plaintext.
- Users would need to configure a TTL that is long enough for the entire
  plan/apply. It's possible for these credentials to expire between the plan and
  apply phases where they run separately.

#### Using ephemeral resources

Terraform also includes a kind of object known as an "ephemeral resource".
These are intended to be used as a source of sensitive, temporary values -
like credentials! In other regards, they are fairly similar to data sources in
that they are not intended to manage actual resources.

```hcl
ephemeral "mwi_aws_roles_anywhere" "account" {
  # Various configuration here needed to generate credentials 
}

provider "aws" {
  region     = "us-west-2"
  access_key = ephemeral.mwi_aws_roles_anywhere.account.access_key
  secret_key = ephemeral.mwi_aws_roles_anywhere.account.secret_key
}
```

Values produced by ephemeral resources can only be used
[within certain contexts](https://developer.hashicorp.com/terraform/language/resources/ephemeral/reference#reference-ephemeral-resources).
This list includes "Configuring providers in the provider block" which is our
intended use-case.

Positives:

- Ephemeral resources are not stored in the Terraform state, avoiding the risk
  of exposing sensitive information.
- As ephemeral resources are invoked both during plan and apply, it's possible
  to grant the "plan" phase a lower set of privileges than the "apply" phase.
- Ephemeral resource kinds can expose a `refresh` method, which can be used to
  refresh the credentials if they expire during the plan/apply. This avoids the
  need for the end user to manually configure an appropriate TTL.
- Ephemeral resource kinds can expose a `close` method, which can be used to
  clean up any resources that are created during the credential generation
  process. This could be useful if we needed to open long-lived tunnels to allow
  access to resources.

Negatives:

- Ephemeral resources are only available from Terraform v1.10 (December 2024) 
  and are not yet available in OpenTofu.
  - OpenTofu does plan to add support:
    https://github.com/opentofu/opentofu/pull/2793

#### Decision

It's clear that the ephemeral resource kind is designed for this precise
use-case. However, the recency of its introduction and the lack of support in
OpenTofu means that if this were the only option, uptake by our customers may
be limited. Our design partner indicates that whilst some teams are able to 
choose their own version, other teams are leveraging OpenTofu or platforms
which currently use older versions of Terraform.

Therefore, we should support both data sources and ephemeral resources at least
until support is more common-place. This will enable customers with more recent
versions of Terraform to use the more secure option. In 6-12 months, we can 
revisit this decision and consider deprecating support for data sources.

### Which provider to use

One key decision is where to build this functionality. Largely, we have three
choices:

- Build this functionality within the existing Teleport Terraform provider.
  - Positives:
    - No additional build/release pipelines to maintain.
    - Customers leveraging this functionality and managing a Teleport cluster 
      within the same plan/apply only need to configure a single provider.
  - Negatives:
    - Existing Teleport terraform provider has a significant degree of tech
      debt, and due to the different nature of this functionality, we're likely
      to run into unforeseen edge-cases within the core implementation of the 
      Terraform provider.
    - Existing Terraform provider consists largely of generated code, whereas
      this functionality will be largely handwritten. Mixing generated and 
      handwritten code can make enforcing standards more difficult.
    - Existing Terraform provider uses a very old version of the Terraform SDK
      which makes compatibility with other tools like Pulumi more challenging.
    - Existing Terraform provider has a large number of configuration
      parameters which would not be compatible with this functionality, this
      poses a risk of creating a poor configuration UX.
- Build all variants of this functionality into a new Terraform provider
  - Positives:
    - No inherited tech debt from existing provider & can leverage latest
      versions of the Terraform SDK.
    - Separation of handwritten and generated code.
  - Negatives:
    - Additional build and release infrastructure to maintain.
    - Additional codebase to maintain.
- Build each variant of this functionality as its own Terraform provider
  - Positives:
    - No inherited tech debt from existing provider & can leverage latest
      versions of the Terraform SDK.
    - Separation of handwritten and generated code.
    - Separation of variants would allow for smaller binary sizes.
  - Negatives:
    - Much more build and release infrastructure to maintain. This would need 
      to be handled each time we added a new variant.

Overall, the most appropriate approach seems to be introducing a single new
Terraform provider for this functionality.

### Building and Publishing

As we have an existing Terraform provider and registry, we can leverage the
existing build and release infrastructure.

A new go module will be created for the new provider to avoid polluting the
dependencies of the main module or the module of the existing provider.

## UX

Before using the data sources or ephemeral resources, the user must configure
the provider itself. To do so, they will provide the details required to
connect and authenticate to the Teleport cluster.

Configuring the MWI provider:

```hcl
provider "mwi" {
  proxy_server  = "example.teleport.sh:443"
  join_method = "terraform_cloud"
  join_token  = "my-join-token"
}
```

Provider arguments:

- `proxy_server`: the address of the Teleport Proxy. Required. 
- `join_method`: the method to use to join the Teleport cluster. Required.
- `join_token`: the join token to use to join the Teleport cluster. Required.

### Kubernetes Access

```hcl
provider "mwi" {
  proxy_server  = "example.teleport.sh:443"
  join_method = "terraform_cloud"
  join_token  = "my-join-token"
}

ephemeral "mwi_kubernetes" "my_cluster" {
  selector {
    name = "my-kubernetes-cluster"
  }
  credential_ttl = "1h" 
}

// https://registry.terraform.io/providers/hashicorp/kubernetes/latest/docs
provider "kubernetes" {
  host                   = ephemeral.mwi_kubernetes.my_cluster.host
  tls_server_name        = ephemeral.mwi_kubernetes.my_cluster.tls_server_name
  client_certificate     = ephemeral.mwi_kubernetes.my_cluster.client_certificate
  client_key             = ephemeral.mwi_kubernetes.my_cluster.client_key 
  cluster_ca_certificate = ephemeral.mwi_kubernetes.my_cluster.cluster_ca_certificate 
}
```

Ephemeral resource arguments:

- `selector`: this object mirrors the `selector` object within the
  `kubernetes/v2` output. This identifies the Teleport resource that we want to
   connect to.
  - `name`: the name of the resource to connect to. Required.
- `credential_ttl`: how long the generated credentials should be valid for. 
  This is a string supporting the 's', 'm', 'h', 'd' suffixes. Required.

Ephemeral resource outputs:

- `host`: the address of the Teleport Proxy.
- `tls_server_name`: the TLS server name to use when connecting to the
  Teleport Proxy. 
- `client_certificate`: the client certificate to use when connecting to the
  Teleport Proxy.
- `client_key`: the client key to use when connecting to the Teleport Proxy.
- `cluster_ca_certificate`: the CA certificate to use when connecting to the
  Teleport Proxy.

### AWS via Roles Anywhere

```hcl
provider "mwi" {
  proxy_server  = "example.teleport.sh:443"
  join_method = "terraform_cloud"
  join_token  = "my-join-token"
}

ephemeral "mwi_aws_roles_anywhere" "my_account" {
  selector {
    name = "my-workload-identity"
  }
  role_arn         = "arn:aws:iam::123456789012:role/my-role"
  profile_arn      = "arn:aws:rolesanywhere:us-east-1:123456789012:profile/0000000-0000-0000-0000-00000000000"
  trust_anchor_arn = "arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/0000000-0000-0000-0000-000000000000" 
  region           = "us-west-2"
  session_duration = "1h"
}

// https://registry.terraform.io/providers/hashicorp/aws/latest/docs
provider "aws" {
  region     = "us-west-2"
  access_key = ephemeral.mwi_aws_roles_anywhere.account.access_key
  secret_key = ephemeral.mwi_aws_roles_anywhere.account.secret_key
  token      = ephemeral.mwi_aws_roles_anywhere.account.token
}
```

Ephemeral resource arguments:

- `selector`: this object mirrors the `selector` object within the
  `workload-identity-aws-roles-anywhere` output. It identifies the Workload 
  Identity resource to use when issuing credentials.
  - `name`: the name of the Workload Identity resource. Required.
- `role_arn`: the ARN of the IAM role to assume. Required.
- `profile_arn`: the ARN of the Roles Anywhere profile to use. Required.
- `trust_anchor_arn`: the ARN of the Roles Anywhere trust anchor to use.
  Required.

Ephemeral resource outputs:

- `access_key`: the access key to use when connecting to AWS.
- `secret_key`: the secret key to use when connecting to AWS.
- `token`: the session token to use when connecting to AWS. Sometimes known as
  the "session token".

### Documentation

We can leverage the existing documentation generation tools to produce a
reference for the new Terraform provider.

## Security Considerations

### Sensitive data in Terraform state

When using data sources, the output values are stored in the Terraform state. 
This allows them to be generated in the plan stage and reused for the apply 
stage.

Unfortunately, this means that sensitive data (e.g credentials) will be stored
in the Terraform state.

For the most part, security-savvy customers will leverage some form of 
encryption-at-rest for their Terraform state. However, this is not always the
case, especially in simpler environments.

We will already be offering a more secure variant in the form of ephemeral
resources (which are not persisted into the state), so we should document
clearly the potential risks of leveraging the data source variant and if using
the data source variant is absolutely necessary, we should recommend that the
customer encrypt their Terraform state.