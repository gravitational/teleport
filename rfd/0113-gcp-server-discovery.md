---
authors: Andrew Burke (andrew.burke@goteleport.com)
state: draft
---

# RFD 113 - GCP auto-discovery

## Required Approvers

Engineering: @strideynet && @r0mant
Product: @klizhentas && @xinding33
Security: @reedloden

## What

Teleport discovery services will be able to automatically discover and enroll GCP virtual machine
instances.

## Why

This change will bring GCP auto-joining capabilities in line with EC2 and Azure.

## Details

### Discovery

Support for auto-discovering GCP VMs will be added to the Discovery Service.

```yaml
discovery_service:
  enabled: 'yes'
  gcp:
    - types: ['vm']
      project_ids: ['<project-id>']
      locations: ['<location>']
      tags:
        'teleport': 'yes'
      install:
        join_params:
          token_name: 'gcp-discovery-token' # default value
        nodename:
        script_name: 'default-installer' # default value
```

New GCP nodes will be discovered periodically on a 5 minute timer, as new
nodes are found they will be added to the teleport cluster.

In order to avoid attempting to reinstall Teleport on top of an instance where it is
already present, the generated Teleport config will match against the node name using
the project ID, zone, and instance ID by default. This can be overridden
by specifying a node name in the join params.

```json
{
  "kind": "node",
  "version": "v2",
  "metadata": {
    "name": "<project-id>-<zone>-<instance-id>",
    "labels": {
      "env": "example",
      "teleport.internal/discovered-node": "yes",
      "teleport.internal/discovered-by": "<discovered-node-uuid>",
      "teleport.internal/origin": "cloud",
      "teleport.internal/zone": "<zone>",
      "teleport.internal/projectId": "<project-id>",
      "teleport.internal/instanceId": "<instance-id>"
    }
  },
  "spec": {
    "public_addr": "...",
    "hostname": "gcpxyz"
  }
}
```

### Agent installation

Teleport agents will be installed on GCP virtual machines using the installer
scripts that are already served at `/webapi/scripts/{installer-resource-name}`,
just like EC2 and Azure auto-discovery.

```yaml
kind: installer
metadata:
  name: 'installer' # default value
spec:
  # shell script that will be downloaded and run by the virtual machine
  script: |
    #!/bin/sh
    curl https://.../teleport-pubkey.asc ...
    echo "deb [signed-by=... stable main" | tee ... > /dev/null
    apt-get update
    apt-get install teleport
    teleport node configure --auth-agent=... --join-method=gcp --token-name=gcp-token
  # Any resource in Teleport can automatically expire.
  expires: 0001-01-01T00:00:00Z
```

The default installer will be extended to support GCP.

To run commands on a VM, the Discovery Service will create a short-lived
ssh key pair and add the public key to the VM via its metadata. Then it will
run the installer on the VM over SSH.

> Note: GCP VMs using [OS Login](https://cloud.google.com/compute/docs/oslogin) do not support SSH keys in instance metadata.

The Discovery Service's service account will require the following permissions:

- `compute.instances.setMetadata`
- `compute.instances.get`
- `compute.instances.list`

### GCP join method

In order to register GCP virtual machines, a new `gcp` join method will be
created. The `gcp` join method will be
an oidc-based join method like `github`, `circleci`, etc. The token will be fetched from the VM's
instance metadata at the internal URL
`http://metadata/computeMetadata/v1/instance/service-accounts/default/identity?audience=AUDIENCE&format=FORMAT&licenses=LICENSES`,
with the audience claim set to the name of the Teleport cluster. The issuer
will always be `https://accounts.google.com`. The rest of the registration
process will be identical to that of the
[other oidc join methods](https://github.com/gravitational/teleport/blob/master/rfd/0079-oidc-joining.md#auth-server-support).

The joining VM will need a [service account](https://cloud.google.com/compute/docs/access/create-enable-service-accounts-for-instances)
assigned to it to be able to generate id tokens. No permissions on the account
are needed.

#### Teleport Configuration

The existing provision token type can be extended to support GCP
authentication, using new GCP-specific fields in the token rules section.

```yaml
kind: token
version: v2
metadata:
  name: example_gcp_token
spec:
  roles: [Node, Kube, Db]
  gcp:
    allow:
      # IDs of projects from which nodes can join. At least one required.
      - project_ids: ['p1', 'p2']
        # Regions and/or zones from which nodes can join. If empty or omitted,
        # nodes from any location are allowed.
        locations: ['l1', 'l2']
        # Emails of service accounts associated with service accounts assigned
        # to joining nodes. If empty or omitted, nodes with any email are
        # allowed.
        service_accounts: ['e1@example.com', 'e2@example.com']
```

The `google.compute_engine.project_id` and `google.compute_engine.zone` JWT
claims will be mapped to configuration values.

teleport.yaml on the nodes should be configured so that they will use the GCP
join token:

```yaml
teleport:
  join_params:
    token_name: 'example_gcp_token'
    method: gcp
```

### teleport.yaml generation

The `teleport node configure` subcommand will be used to generate a
new /etc/teleport.yaml file:

```sh
teleport node configure
    --auth-server=auth-server.example.com [auth server that is being connected to]
    --token="$1" # name of the join token, passed via parameter from run-command
    --labels="teleport.internal/discovered-node=yes,\
    teleport.internal/discovered-by=$2,\
    teleport.internal/origin=cloud" # sourced from instance metadata
```

This will generate a file with the following contents:

```yaml
teleport:
  nodename: '<project-id>-<zone>-<instance-id>'
  auth_servers:
    - 'auth-server.example.com:3025'
  join_params:
    token_name: token
  # ...
ssh_service:
  enabled: 'yes'
  labels:
    teleport.internal/projectId: '<project-id>'
    teleport.internal/zone: '<zone>'
```

GCP-specific labels (`teleport.internal/project-id`, `teleport.internal/zone`)
will be added by Teleport using the process outlined in
[RFD 22033](https://github.com/gravitational/teleport/pull/22033).

## UX

### User has 1 account to discover servers on

#### Teleport config

Discovery server:

```yaml
teleport: ...
auth_service:
  enabled: 'yes'
discovery_service:
  enabled: 'yes'
  gcp:
    - types: ['vm']
      project_ids: ['<project_id>']
      locations: ['westcentralus']
      tags:
        'teleport': 'yes'
      install:
        # Use default values
```

Permissions required for the Discovery Service:

```yaml
title: teleport_vm_discovery
description: Role for discovering VMs and adding them to a Teleport cluster.
stage: ALPHA
includedPermissions:
  - compute.instances.setMetadata
  - compute.instances.get
  - compute.instances.list
```

Teleport will retrieve GCP credentials in the same way that it already does for
[GKE auto-discovery](https://goteleport.com/docs/kubernetes-access/discovery/google-cloud/#retrieve-credentials-for-your-teleport-services).

## Security considerations

Automatic EC2 joining uses SSM to separate permission to create/update commands
from permission to call them. GCP does not have an SSM-like service, so the
discovery service will require permission to both create and execute commands.

GCP does not have a dedicated "Run command" API; commands on VMs are instead
executed over SSH, with access managed with keys in the VM's
metadata. There is no permission specifically for running commands; the most
limiting one is `compute.instances.setMetadata`.

## IaC

### Terraform

The `gcp` join method extents `ProvisionTokenV2`, so no extra work will be
needed for the Terraform provider.

### Helm charts

Helm charts will need to be extended to support the GCP discovery service.
However, since the Helm charts don't currently support any discovery services,
that work is better saved for when we add discovery support in general.

### Kube operator

Provision tokens are not currently supported by the Kube operator. If we add
support for GCP joining, we should do so as part of general token support.

## Links

- [Verifying GCP VM Identity](https://cloud.google.com/compute/docs/instances/verifying-instance-identity)
- [SSH on GCP VMs](https://cloud.google.com/compute/docs/instances/ssh#third-party-tools)

## Appendix I - Example ID token payload

```json
{
  "aud": "teleport.example.com",
  "azp": "<...>",
  "email": "<...>-compute@developer.gserviceaccount.com",
  "email_verified": true,
  "exp": 1680121810,
  "google": {
    "compute_engine": {
      "instance_creation_timestamp": 1680117502,
      "instance_id": "<...>",
      "instance_name": "<...>",
      "project_id": "<...>",
      "project_number": 12345678,
      "zone": "<...>"
    }
  },
  "iat": 1680118210,
  "iss": "https://accounts.google.com",
  "sub": "<...>"
}
```
