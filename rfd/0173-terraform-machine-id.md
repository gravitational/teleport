---
author: hugoShaka (hugo.hervieux@goteleport.com)
state: implemented
---

# RFD 173 - Authenticating the Terraform provider with MachineID

## Required Approvers

* Engineering: @r0mant && (@Joerger || @strideynet)
* Product: (@xinding33 || @klizhentas)

## What

Make Terraform able to use MachineID natively to obtain and renew certificates.
Reduce the Terraform provider setup complexity and shorten the time-to-value for new provider users.
Improve IaC tooling security by encouraging short-lived certificate adoption.

## Why

We introduced two fundamental changes in Teleport since the birth of the TF provider:
- we added MachineID for machines to obtain credentials and interact with Teleport
- we started rolling out "MFA for admin" (MFA4A in the rest of the document) in v15 which makes Teleport require MFA
  verification for administrative actions

The previous recommendation was to create a TF user and sign a long-lived certificate with impersonation.
Such high-privilege long-lived credentials were an easy target for attackers and newer versions of Teleport
are actively discouraging users to rely on them
When MFA4A is enabled (by default on clusters with webauthn only since v15) this certificate does not allow Terraform to
perform administrative actions (i.e. creating users, roles, configuring SSO bindings).

The current workaround is to do a full MachineID/tbot setup and have Terraform use the generated certificates.
While this approach works, it has two main issues:

- setting up MachineID/tbot is complex and increases the time-to-value for new users wanting to test the provider
  locally. MachineID/tbot does not have a great story (yet) about using different secret tokens for the same bot (this
  will happen soon) so the complex setup must be done every time the user needs to run Terraform and the bot certs are
  expired.
- When running the TF provider on a dev laptop (as opposed as in the CI/on a dedicated master), the
  current workaround leaves certificates with administrative rights on the laptops. Any attacker with execution rights
  on the developer's laptop can use the local MachineID certs and bypass MFA4A.

## Details

### User stories and personas

With this RFD we want to address two distinct personas, each with their own story:

#### The existing advanced user

> I am an existing Teleport user who uses Terraform to manage my Teleport resources.
> I want to protect against IDP compromise with MFA4A while keeping my Terraform pipeline working.
> My Terraform code is committed in a git repo and applied in CI pipelines, either via a CI runner (GitHub
> Actions/GitLab CI) or via a dedicated service (Terraform Cloud/Spacelift).

The user is expected to know the Teleport basics and be able to perform advanced setup actions such as
creating roles and bot resources for MachineID. Some setup complexity is not an issue for this persona.

#### The "getting started" users

We have two personas:

- The prospect:
  > I am not yet a Teleport user and want to test what the Teleport IaC experience looks like before using/buying Teleport.
  > IaC will be mandatory for me because I am subject to various internal policies and I must check if Teleport is
  > compatible with my IaC policies.
  > I have prior Terraform experience.

- The existing manual user
  > I am an existing Teleport user that have been managing Teleport resources manually.
  > Today I am investigating how to make my setup more robust by leveraging an IaC tool.
  > I might not have prior Terraform experience.

Both personas don't have (yet) dedicated infrastructure such as CI runners to deploy the Terraform code.
They want to prototype and validate everything works using the local computer.

Both personas have very little knowledge about Teleport and don't know what MachineID is and how to configure it.
We want those users to succeed as fast as possible without having to read all the MachineID docs. When they will go
to production, they will hopefully turn into [existing advanced users](#the-existing-advanced-user) and setup CI/CD
pipelines for their deployment.

### Implementation

We will introduce two new mechanisms:
- MachineID joining in the Terraform provider
- Terraform Resource bootstrapping in `tctl`

#### MachineID joining

The Terraform provider will be able to natively join a Teleport cluster using MachineID when provided with a token and join method.
The users will not need to install or configure `tbot`.

The Terraform provider will run an in-process `tbot` instance and generate a client with automatic cert-renewal.
The `tbot` instance will not persist its certificates on the disk.

A typical GitHub actions provider configuration would look like:

```hcl
provider "teleport" {
  addr               = "mytenant.teleport.sh:443"
  joining = {
    token  = "gha-runner"
    method = "github"
  }
}
```

As Teleport does not support yet reusable bot tokens so the `token` join method will not be supported.
_Note: this might change in the future with the `BotInstance` resource decoupling the bot from the
secret token._

Implementation-wise, this would reuse the
same [in-process tbot library](https://github.com/gravitational/teleport/tree/rfd/173-terraform-machine-id/integrations/operator/embeddedtbot)
we used for the Teleport operator.

#### Resource bootstrapping

Teleport will now ship with a preset Terraform provider role: `terraform-provider` automatically updated using
our existing preset logic. This will allow users to benefit from the new resources supported by the Terraform provider
without having to update their preset role. If a role with this name already exists, Teleport will not modify it.

We will automatically create the resources required by the Terraform provider before executing Terraform:
- the `tctl-terraform-env-<random hash>` bot allowed to issue certificates for the `terraform-provider` role.
  This resource expires by default after 1h.
- the random secret token allowing to join as
  the `tctl-terraform-env-<random hash>` bot. This resource expires by default after 1h but will be consumed
  automatically on join.

Every bootstrapped resource will be annotated with `teleport.dev/created-by: tctl-terraform-env`

##### Discarded approaches

Creating those resources requires passing an MFA challenge, this means being built with libfido2 and CGO.
This would make the provider unable to run on Hashicorp Cloud Platform, which is requested by many Teleport users.

Two alternatives were considered but discarded:

- Have Terraform re-exec `tctl` or `tsh` to pass the MFA challenge and create the resources. The issue with this
  approach is that we don't have access to stdin/stdout/stderr to prompt the user for MFA.
- Have `tsh`/`tctl` create the resources first, then exec into Terraform by passing the token via environment variables.
  This would force users to invoke `tctl` each time instead of terraform which is cumbersome. Handling arguments
  properly would also be a challenge.

##### Proposed approach

We will add a `tctl terraform env` command which will
- upsert the terraform role
- create a temporary bot (1hour) and token
- run a oneshot tbot to retrieve certificates
- set certificates in the shell's environment variables using the already supported `TF_TELEPORT_IDENTITY_FILE_BASE64`

The full flow will look like

```
$ eval $(tctl terraform env)
- Creating/Updating the "terraform-provider-helper” role
- Creating a temporary bot "terraform-provider-hugo.hervieux@goteleport.com-b9a853e0”
- Obtaining certificates for the bot
- You can now invoke terraform commands in this shell for 1 hour 🚀
$ terraform plan ...
$ terraform apply ...
```

`tctl terraform env` will:
- get a client from the current-profile
- ping the cluster to validate the user client and recover the user's name
- generate a random secret token (16 bytes of hex-encoded random, the Teleport default)
- Check if MFA4A is required, if so:
  - Create an MFA challenge that can be reused for all 3 API calls (reuse challenge extension)
  - Prompt the user to answer the MFA challenge
  - Attach the MFA challenge response in the ctx for each API call (as described in #37121)
- create the 3 bootstrap resources
  - if the call fails because of missing permissions, output a user-friendly error such as:
    ```
    Failed to create bootstrap resources using your local credentials (user "hugo.hervieux@goteleport.com", address "mytenant.teleport.sh:443").
    Please check if you have the rights to create role, bot and token resources. You might need to re-log in for new rights to take effect.
    (tsh logout --proxy="mytenant.teleport.sh:443" --user="hugo.hervieux@goteleport.com")
    ```
- run a one-shot tbot to retrieve certificates via the bot for the terraform role
- set the environment variable `TF_TELEPORT_IDENTITY_FILE_BASE64`
- echo a user-friendly message containing the bot name and the certificate validity

#### Backward compatibility

By default, when `joining` values are not set, the provider uses the existing `identity_*` values.
This ensures compatibility with existing setups.

`joining`, and the `identity_` settings are mutually exclusive and the provider will refuse to start if
both are set. This will avoid any un-tested hybrid configuration.

The error message would look like:
```
Invalid provider configuration. `joining` and `identity_*`/`key_*`/`profile_*` values are mutually exclusive.
You must set only one.

- `joining` is used when running Terraform in CI/CD pipelines such as GitHub Actions or GitLab CI.
- passing certificates directly with `identity_*`, `key_*` or `profile_*` is used when you already have Teleport credentials for the provider.
```

The `tctl terraform env` command uses the `TF_TELEPORT_IDENTITY_FILE_BASE64` environment variable which is
already supported by the Terraform Provider. This helper can be backported to v15 and v14.

### Security

#### Benefits

This approach makes using MachineID and adopting short-lived certificates easier, especially in CI/CD pipelines.
Switching to short-lived certificates and delegated join methods improves security as there's less material to
exfiltrate and users can fine-tune the token permissions (allow joining based on the service account/github
project/workload location). Adopting "MFA for admin" will also be easier for existing users.

This approach also improves security by not writing to disk the MachineID-generated certs. In case of
misconfigured/permissive ACLs, an attacker already present on the host will not be able to obtain certs from reading the
filesystem. Exfiltrating the MachineID certs requires dumping the process memory.

#### Risks

The main risk is the amount of resources created by `tctl terraform env`. Each invocation will
create a bot resource. Too many resource could create noise or affect Teleport's performance. This risk is mitigated
by:
- the fact the helper is only usable on local laptop. It requires a valid `~/.tsh` profile and the ability to pass an
  MFA challenge (when MFA4A is enabled, which we are pushing everywhere and is Cloud's default). Intensive CI usage will
  use existing bot resources and the `joining` configuration.
- the certificate validity: a single invocation is needed every hour
- the bootstrap resource expiry, by default 1 hour.

Reusing the same resource (delete/re-create) proved to be very harmful when we did this in the Kubernetes operator.
This caused a lot of instability/consistency issues, it took a full operator rewrite to solve them.

If needed we can list how many terraform bot resources are living in Teleport and warn the user if it goes above a
certain threshold, but this should not be necessary.

The issue caused by the number of resources will very likely be addressed in the future by the work done on
[the `BotInstance` resource](https://github.com/gravitational/teleport/pull/36510).
This will allow tokens (even secret ones) and bots to be shared by multiple instances.

### Privacy

The fact a user ran a `tctl terraform env` is disclosed via the bot resource.
Someone able to read roles, users or bots could infer when Terraform commands are executed.
This information is already available to admins via the audit log.

### UX

This improves the UX for the "existing advanced user" persona as they don't need to install and configure `tbot`
anymore. This also unblocks support for runtimes where running `tbot` was not possible:
e.g. [Terraform Cloud](https://github.com/gravitational/teleport/issues/26345).

This greatly improves the UX of the "getting started" personas as the provider will hide all the complexity and shorten
the time to value for IaC adoption. The
whole [setup Terraform provider page](https://github.com/gravitational/teleport/blob/master/docs/pages/management/dynamic-resources/terraform-provider.mdx)
becomes a 3-step guide: run `tsh login`, `eval $(tctl terraform env)` and create the `main.tf`.

Two documentation guides will be published:

- Getting started with the Terraform Provider, explaining users how to run the provider locally
- Running the Terraform Provider in CI/CD pipelines, explaining users how to run the provider in GitHub Actions, GitLab
  CI, Circle CI, Spacelift, (and Terraform Cloud when we'll add support).

The Getting started guide should provide instruction on how to connect to a local cluster with self-signed certificates
(recover and trust the cluster cert with `SSL_CERT_FILE` instead of running with --insecure).

### Observability

The new Provider joining mechanisms will reuse existing Teleport APIs which are already emitting audit events and
reporting Prometheus metrics.

By its short-lived nature, the terraform process does not expose metrics.

### Product usage

To validate product adoption and join method usage we need some Telemetry, there are two possible approaches:

#### Anonymous opt-in Telemetry client-side

In this approach, telemetry would be opt-in and reuse the existing tbot start event and its `helper` field:

```protobuf
syntax = "proto3";

message TbotStartEvent {
  enum RunMode {
    RUN_MODE_UNSPECIFIED = 0;
    RUN_MODE_ONE_SHOT = 1;
    RUN_MODE_DAEMON = 2;
    RUN_MODE_IN_PROCESS = 3;
  }
  RunMode run_mode = 1;
  string version = 2;
  string join_type = 3;
  string helper = 4;  // helper would be `terraform`
  string helper_version = 5; // helper_version would be the TF provider version
  int32 destinations_other = 6;
  int32 destinations_database = 7;
  int32 destinations_kubernetes = 8;
  int32 destinations_application = 9;
}
```

Telemetry would be gated by the `TELEPORT_ANONYMOUS_TELEMETRY=1` environment variable.

#### Auth-based Telemetry for Teleport customers

Depending on their license, Teleport customers can have clusters dialing home and reporting usage.
We can leverage this to report metrics from the auth. An option would be to add an annotation on the bot resource such as
`teleport.dev/integration: terraform` or `teleport.dev/usage: terraform`. Potential values could be expanded as we add
native tbot support to other integrations (e.g. `operator|access/slack|access/pagerduty|event-handler`).

In this case we cannot trust what the client requests so we'd add the annotation on the bot resource.
This usage would be reflected in the `BotJoin` audit event. To avoid reporting directly identifiable information, the
Anonymization step would check the usage/integration value against a hardcoded list of known Teleport integrations.

### Test plan

Write integration tests for:
- `joining` with an existing bot
- `tctl terraform env` running against a Teleport cluster

Manual test (in the test plan) for:
- running the provider in GitHub actions with `joining`
- running `tctl terraform env` against a cluster with MFA4A

### Future work

Add Hashicorp Cloud Platform Terraform support via a dedicated join method.
