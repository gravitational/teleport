---
author: hugoShaka (hugo.hervieux@goteleport.com)
state: draft
---

# RFD 173 - Authenticating the Terraform provider with MachineID

## Required Approvers

* Engineering: @r0mant && (@Joerger || @strideynet)
* Product: (@xinding33 || @klizhentas)

## What

Make Terraform able to use MachineID natively to obtain and renew certificates.
Reduce the Terraform provider setup complexity and shorten the time-to-value for new provider users.

## Why

We introduced two fundamental changes in Tlepeort since the birth of the TF provider:
- added MachineID for machines to obtain credentials and interact with Teleport
- started rolling out "MFA for admin" (MFA4A in the rest od the document) which makes Teleport require MFA certificates
  for administrative actions

The recommendation was to create a TF user and sign a long-lived certificate with impersonation.
When MFA4A is enabled (by default on clusters with webauthn only since v15) this certificate does not allow Terraform to
perform administrative actions (i.e. creating users, roles, configuring SSO bindings).

The current workaround is to do a full MachineID/tbot setup and have Terraform use the generated certificates.
While this approach works, it has two main issues:
- setting up MachineID/tbot is complex and increases the time to value for new users wanting to test the provider locally.
- MachineID/tbot does not have a great story (yet) about using different secret tokens for the same bot (this will
  happen soon). When running the TF provider on a dev laptop (as opposed as in the CI/on a dedicated master), the
  current workaround leaves certificates with administrative rights on the laptops.

## Details

### User stories

With this RFD we want to address two distinct user stories:

#### The existing advanced user

> I am an existing Teleport user who uses Terraform to manage my Teleport resources.
> I want to benefit from MFA4A while keeping my Terraform pipeline working.
> My Terraform code is commited in a git repo and applied in CI pipelines, either via a regular CI runner (GitHub
> Actions/GitLab CI) or via a dedicated service (Terraform Cloud/Spacelift).

For this story, the user is expected to know the Teleport basics and be able to perform advanced setup actions such as
creating roles and bot resources for MachineID.

#### The "getting started" user

> I am not yet a Teleport user (prospect) and want to test what IaC Teleport provides before adopting the tool.
> I can also be an existing Teleport user that have been managing Teleport resources manually and want to
> make my setup more robust by leveraging an IaC tool.
> I don't have dedicated infrastructure such as CI runners to deploy my Terraform code and want to do everything from my
> laptop.

For this story, the user has very little knowledge about Teleport and does not know what MachineID is and how to set it
up. We want those users to succeed as fast as possible without having to read all the MachineID docs. When they will go
to production, they will hopefully turn into [existing advanced users](#the-existing-advanced-user) and setup CI/CD
pipelines for their deployment.

### Implementation

We will introduce two new mechanisms in the Terraform provider:
- MachineID joining
- Resource bootstrapping

#### MachineID joining

The Terraform provider will be able to take a MachineID token and join method to join the Teleport cluster.
The Terraform provider will run an in-process `tbot` instance and generate a client with automatically-rotated certificates.
The `tbot` instance will not persist its certificates on the disk.

A typical GitHub actions provider configuration would look like:

```hcl
provider "teleport" {
  addr               = "mytenant.teleport.sh:443"
  onboarding = {
    token       = "gha-runner"
    join_method = "github"
  }
}
```

As Teleport does not support yet reusable bot tokens so the `token` join method will not be supported.

Implementation-wise, this would reuse the same in-process tbot library we used for the Teleport operator.

#### Resource bootstrapping

When configured to do so, the Terraform provider will use the user's local profile (from `~/.tsh`) to create resources
it needs to interact with the Teleport cluster. Such resources include:
- the `terraform-provider` role (this is an upsert so we can add new rules after a provider update). This resource does not expire.
- the `terraform-provider-<user>-<random hash>` bot allowed to issue certificates for the `terraform-provider` role. This resource expires.
- the `terraform-provider-<user>-<random hash>` secret random provision token allowing to join as the `terraform-provider-<user>-<random hash>` bot. This resource expires but will be consumed automatically on join.

Each local Terraform invocation will create a new bot and join token. To avoid resource accumulation those resource will
have an expiry of 1 hour by default.

A typical "getting started" provider configuration would look like:

```hcl
provider "teleport" {
  addr = "mytenant.teleport.sh:443"
  bootstrap = {
    enabled = true
    resource_prefix = "terraform-provider"
    
    # optional
    expires_after = "1h"
  }
}
```

When running with this configuration, the provider will:
- get the local `~/.tsh` profile associated with this `addr`
- ping the cluster to validate the user client and store the user name
- generate a random secret token (16 bytes of hex-encoded random, the Teleport default)
- create the 3 bootstrap resources
  - if the call fails because of MFA4A, perform the MFA challenge once, save the MFa certs valid for a minute and use
    them to create the 3 resources
  - if the call fails because of missing permissions, output a user-friendly message:
    ```
    Failed to create bootstrap resources using your local credentials (user "hugo.hervieux@goteleport.com", address "mytenant.telpeort.sh:443").
    Please chech you have the rights to create role, bot and token resources. If you got recently granted those rights you might need to re-log in.
    (tsh logout --proxy="mytenant.teleport.sh:443" --user="hugo.hervieux@goteleport.com")
    ```
- continue the Provider flow with the following onboarding config
  ```hcl
  onboarding = {
    token = "<secret token that got generated earlier>"
    join_method = "token"
  }
  ```

#### Backward compatibility

By default, when neither `bootstrap` nor `onboarding` values are set the provided uses the existing `identity_*` values.
This ensures compatibility with existing setups.

`onboarding`, `bootstrap` and the `identity_` settings are mutually exclusive and the Provider will refuse to start if
more than one is set. This will avoid any un-tested hybrid configuration.

```
Invalid provider configration. `bootstrap`, `onboarding` and `identity_*`/`key_*`/`profile_*` values are mutually exclusive.
You must set only one value.

- `bootstrap` is used when running Terraform on a developper laptop
- `onboarding` is used when running Terraform in CI/CD pipelines such as GitHub Actions or GitLab CI.
- passing certificates directly with `identity_*`, `key_*` or `profile_*` is used when you already have Teleport credentials for the provider.
```

### Security

#### Benefits

This approach makes using MachineID and adopting short-lived certificates easier, especially in CI/CD pipelines.
Switching to short-lived certificates and delegated join methods improves security as there's less material to
exfiltrate and users can fine-tune the token permissions (allow joining based on the service account/github
project/workload location). Adopting "MFA for admin" will be easier for existing users.

This approach also improves security by not writing to disk the MachineID-generated certs. In case of
misconfigured/broad ACLs, an attacker already present on the host will not be able to obtain certs from reading the
filesystem. Exfiltrating the MachineID certs requires dumping the process memory.


#### Risks

The main risk is the amount of resources created by the `bootstrap` configuration. Each terraform invocation will
create a bot resource. Too many resource could create noise opr affect Teleport's performance. This risk is mitigated
by:
- the fact `bootstrap` is only usable on local laptop. It requires a valid `~/.tsh` profile and the ability to pass an
  MFA challenge (when MFA4A is enabled, which we are pushing everywhere and is Cloud's default). Intensive CI usage will
  use existing bot resources and the `onboarding` configuration.
- the bootstrap resource expiry, by default 1 hour.

Reusing the same resource (delete/re-create) proved to be very harmful when we did this in the operator.
This caused a lot of instability/consistency issues and it took a full operator rewrite to solve them.

If needed we can list how many terraform bot resources are living in Teleport and warn the user if it goes above a
certain threshold, but this should not be necessary.

The issue caused by the number of resources will very likely be addressed in the future by the work done on
the `BotInstance` resource. This will allow tokens (even secret) and bots to be reused by multiple instances.

### Privacy

The fact a user ran a Terraform command with `bootstrap` is disclosed via the bot resource.
Once could infer when Terraform commands are executed from this. This information is already available to admins via the
audit log.

### UX

This improves the UX for the "existing advanced user" persona as they don't need to install and configure tbot anymore.
This also unblocks support for runtimes where running tbot was not possible: e.g. Terraform Cloud.

This greatly improves the UX of the "getting started" persona as the provider will hide all the complexity and shorten
the time to value for IaC adoption. The
whole [setup Terraform provider page](https://github.com/gravitational/teleport/blob/master/docs/pages/management/dynamic-resources/terraform-provider.mdx)
becomes a 2-step guide: run tsh login and create the `main.tf`.

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

In this approach, telemetry would be opt-in and reuse the existing tbot start event and adding new fields:

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
  string helper = 4;
  string helper_version = 5;
  int32 destinations_other = 6;
  int32 destinations_database = 7;
  int32 destinations_kubernetes = 8;
  int32 destinations_application = 9;
  // process is the name of the process running tbot when run mode is "in process".
  // The only value will be "terraform" but we can reuse this in kube with "operator".
  string process = 10;
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
- `onboarding` with an existing bot
- `onboarding` with mocking MFA (not sure about the feasibility)
- `bootstrap` enabled and checking resources are created

Manual test (in the test plan) for:
- running the provider in GitHub actions with `onboarding`
- running the provider locally with `onboarding` against a cluster with MFA4A

### Future work

Add Terraform Cloud support via a dedicated join method.