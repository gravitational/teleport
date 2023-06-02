---
authors: Gavin Frazar (gavin.frazar@goteleport.com)
state: draft
---

# RFD 0129 - Discovery Resource Name Templates

Related RFDs:
- [RFD 0125 - Dynamic Auto-Discovery Configuration](./0125-dynamic-auto-discovery-config.md)

## Required Approvers

- Engineering: `@r0mant && @smallinsky && @tigrato`
- Product: `@klizhentas || @xinding33`
- Security: `@reedloden || @jentfoo`

## What

Auto-Discovery service can be configured to dynamically name discovered
resources using a template string that can reference a subset of resource
metadata. The choice of what metadata to make available is the main motivation
for this RFD.

## Why

Multiple discovery agents can discover resources with identical names.
For example, this happened when customers had databases in different AWS
regions or accounts with the same name. When a name collision occurs, only one
of the databases can be accessed by users.

Resource name templates can be used to avoid naming collisions by renaming
discovered resources with a prefix/suffix specific to a discovery agent, or by
using other available metadata such as AWS region or account ID.

See:
- https://github.com/gravitational/teleport/issues/22438

## Details

There will be a new optional configuration string in Discovery service matchers,
`resource_name_template`, that will be parsed as a
[Go Template](https://pkg.go.dev/text/template) to rename resources discovered
using that matcher.

This setting will be made available in both static config and the upcoming
dynamic discovery config from RFD 125.

### `resource_name_template`

This template string is just a Go text/template, so the template syntax is the
same.

Using Go templates is not uncommon, so this UX should not be surprising.
A few examples of software that supports using Go templates: `helm`, `gh`
(GitHub CLI), `docker`.

### Example config:
```yaml
discovery_service:
  enabled: true
  aws:
    - types: ["rds"]
      regions: ["us-west-1", "us-west-2"]
      resource_name_template: "teleport-dev-{{.Name}}-{{.AWS.AccountID}}-{{.AWS.Region}}"
      tags:
        "*": "*"
```

In this example, the discovery agent will discover AWS RDS databases in the
us-west-1 and us-west-2 AWS regions.

Each database discovered by the matcher will have the prefix "teleport-dev-"
followed by the discovered database's name, AWS account ID, and AWS region.

For instance, if a database named `foo` is discovered in AWS account ID
`0123456789012` in region `us-west-1`, the rewritten name will be
`teleport-dev-foo-012345689012-us-west-1`.
This would disambiguate the database name from other databases in other regions
and other AWS accounts.

Since the full template syntax is available, a user could modify this to use
available template builtin functions to shorten the account ID portion:
`resource_name_template: 'teleport-dev-{{printf "%s-%.4s-%s" .Name .AWS.AccountID .AWS.Region}}'`
results in: `teleport-dev-foo-0123-us-west-1`

### Available template variables

We must decide which variables to make available for templating.

The supported variables must be sufficient to disambiguate cloud resources to
avoid name collisions, therefore I think, at a minimum, the supported variables
should include:

- the original discovered resource name: `.Name`
- cloud metadata: `.AWS`, `.GCP`, `.Azure`
  - each of these should only be available for the corresponding matcher
    type: `aws:`, `gcp:`, `azure:`.
- the region/location of the resource: `.Region` or `.Location`
  - this is already included in AWS metadata, but not Azure nor GCP.
  - rather than a protobuf update, we can just expose this template data
    variable separately: `.Region` for AWS/Azure and `.Location` for GCP, for
    consistency with the naming conventions chosen for our discovery matcher
    config - AWS/Azure matchers use `regions:` and GCP matchers use
    `locations:`.

For each cloud metadata type, we can make available the corresponding
`api/types` protobufs:

- `api/types.AWS`
- `api/types.GCPCloudSQL`
- `api/types.Azure`

Doing this would couple our cloud metadata protobufs to the supported template
variables, however it would simplify the implementation and ensure that any new
cloud metadata is available for templating.

For whatever choice of supported variables, we must document the supported
template variables in our docs reference.

We should also print a friendly user message that lists the supported
variables if a user provides a template string that references an unsupported
variable.

We can check the user provided template when file config is applied,
or when a dynamic discovery config is created, by executing the template with
stub data input, or by walking the template parse tree.

### Security

This configuration setting will only be available to Teleport cluster admins,
which limits the potential for intentional abuse. I'm not aware of any
unintentional security concerns, since we control the available supported
template variables and none of them are secrets.

I was concerned about the potential for resource exhaustion if a
self-referencing template makes it into a running discovery agent:
`{{template "ResourceNameTemplate" .}}` - this is self-referencing and looks
like it will evaluate infinitely.
What will actually happen is text/template terminates the recursion at 100,000
depth with an error (not a panic).

We could alternatively catch the invocation of `{{template}} "name" .` actions 
by writing a visitor to walk the template parse tree. We could use this parse
tree walking for further template restrictions if we need to.

We can check for invocations of the template action on agent startup from static
config, or server-side when a dynamic discovery config is created.

### UX

#### `discovery_service` new service configuration properties

Users need to ensure the `resource_name_template` option is set to a template
string for each discovery matcher to enable resource name rewriting.

Example:
```yaml
discovery_service:
  enabled: true
  aws:
    - types: ["ec2", "rds"]
      regions: ["us-west-1"]
      resource_name_template: "{{.Name}}-{{.Region}}-{{.AWS.AccountID}}"
      tags:
        "*": "*"
  azure:
    - types: ["aks"]
      resource_name_template: "{{.Name}}-{{.Region}}"
      regions: ["eastus", "westus"]
      subscriptions: ["11111111-2222-3333-4444-555555555555"]
      resource_groups: ["group1", "group2"]
      tags:
        "*": "*"
  gcp:
    - types: ["gke"]
      resource_name_template: "{{.Name}}-{{.GCP.ProjectID}}-{{.Location}}"
      locations: ["*"]
      tags:
        "*": "*"
       project_ids: ["myproject"]
```

Likewise, use `resource_name_template` in matchers for `DiscoveryConfig`
from RFD 125 using `tctl`.

#### `teleport` and `tctl` template errors

Print a user-friendly message that lists supported template variables
when a user tries to use an unsupported template variable:

config file:
```yaml
discovery_service:
  enabled: true
  aws:
    - types: ["ec2", "rds"]
      regions: ["us-west-1"]
      resource_name_template: "{{.Thing}}-{{.Region}}-{{.AWS.AccountID}}"
      tags:
        "*": "*"
```

usage:
```shell
$ teleport start
ERROR: failed to parse Teleport configuration: discovery service AWS resource_name_template variable ".Thing" is not supported, supported template variables types are:
.Name
.Region
.AWS
  .AccountID
  ...
```

And a similar message specific to the matcher type: AWS/Azure/GCP.

### Proto Specification

When RFD 125 is implemented, we will need to update the proto messages for
`DiscoveryConfig` to add a string `resource_name_template` field to each of
AWS/Azure/GCP matcher messages.

### Backward Compatibility

No concerns I can think of.

### Audit Events

N/A

### Test Plan

We should test discovering multiple resources with identical names do not suffer
a name collision when using templates to disambiguate them.

For instance, setup identically named RDS databases in different AWS regions
and a discovery agent to discover them with this template:
`resource_name_template: '{{.Name}}-{{.Region}}'`

Check that the databases are both present and differentiated by region in their
name using `tsh db ls`.

