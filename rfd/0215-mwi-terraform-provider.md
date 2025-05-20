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

### Providing credentials within a Terraform plan/apply

Terraform Providers typically provide some method of providing credentials
directly within their configuration within the Terraform plan.

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

With this approach, we must consider:

- Whether it is safe to pass the output of a data source as a value in the
  configuration of a provider (e.g. does this introduce problems with execution
  order of a Terraform plan/apply.)?
  - It would appear that this pattern is at least shown in examples in relation
    to the Vault and AWS providers:
    https://github.com/hashicorp-education/learn-terraform-inject-secrets-aws-vault/blob/main/operator-workspace/main.tf
  - It would appear that historically there have been bugs related to this 
    pattern which have been resolved:
    https://github.com/hashicorp/terraform/issues/11264
  - It would appear that data sources, were at least partially, introduced to
    solve for this use-case:
    https://github.com/hashicorp/terraform/issues/4169
- Whether the data source is computed on both plan/apply or if the data source
  outputs are cached and reused from state?
  - It could be problematic if the data source is only computed during plan,
    as the credentials issued could expire before the apply is run.
  - TODO: Determine this...

Positives:

- Well-supported by a number of Terraform versions and forks like OpenTofu

Negatives:

- Data source outputs are stored in the Terraform state, which can be plaintext.
- Data source outputs are computed once, and provide no mechanism for refreshing
  credentials. Users would need to configure a TTL that is long enough for the
  entire plan/apply.

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
      this functionality will be largely hand-written. Mixing generated and 
      hand-written code can make enforcing standards more difficult.
    - Existing Terraform provider uses a very old version of the Terraform SDK
      which makes compatability with other tools like Pulumi more challenging.
    - Existing Terraform provider has a large number of configuration
      parameters which would not be compatible with this functionality, this
      poses a risk of creating a poor configuration UX.
- Build all variants of this functionality into a new Terraform provider
  - Positives:
    - No inherited tech debt from existing provider & can leverage latest
      versions of the Terraform SDK.
    - Separation of hand-written and generated code.
  - Negatives:
    - Additional build and release infrastructure to maintain.
    - Additional codebase to maintain.
- Build each variant of this functionality as its own Terraform provider
  - Positives:
    - No inherited tech debt from existing provider & can leverage latest
      versions of the Terraform SDK.
    - Separation of hand-written and generated code.
    - Separation of variants would allow for smaller binary sizes.
  - Negatives:
    - Much more build and release infrastructure to maintain. This would need 
      to be handled each time we added a new variant.

Overall, the most appropriate approach seems to be introducing a single new
Terraform provider for this functionality.



## UX