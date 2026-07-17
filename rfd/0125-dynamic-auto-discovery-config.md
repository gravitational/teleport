---
authors: Marco Dinis (marco.dinis@goteleport.com)
state: draft
---

# RFD 0125 - Dynamic Auto-Discovery Configuration

Related RFDs:

- RFD 57 - Automatic discovery and enrollment of AWS servers (`discovery_service` RFD)

## Required Approvers

- Engineering: `@r0mant && @smallinsky && @tigrato`
- Product: `@klizhentas || @xinding33`
- Security: `@reedloden || @jentfoo`

## What

Dynamically configure matchers for the Auto-Discovery service.

## Why

Teleport Discover will implement a flow where users are able to install a `discovery_service` and configure it without ever leaving the WebApp.

To do so, the `discovery_service` will be deployed with minimal configuration and let the user configure the matchers afterwards.

Although, this RFD is born from this necessity, there's nothing bounding this feature to using the Discover Wizard, and rather make this dynamic configuration available for any `discovery_service`.

Having an almost stateless configuration eases the configuration and deployment of `discovery_service`s, which is essential for adoption.
Users no longer need to log in to a machine that is running this service to change the matchers, and can do any sort of configuration from the clients (ie WebApp, `tctl`, Terraform Provider and Kubernetes Operator).

## Details

There will be a new resource - `DiscoveryConfig` - which will be used by the `discovery_service` to add extra matchers for that particular service.

`discovery_service` must support having only the `discovery_group` and still run, even if no `DiscoveryConfig` matches that group.

### New `DiscoveryConfig` resource

A new resource will be created to store the desired matchers.

The `spec` schema will contain a `discovery_group` and a list of matchers.

The `discovery_group` will be used by `discovery_service` to include matchers from `DiscoveryConfig` that have the same group.

The list of matchers is the same list that exists in `discovery_service` (for AWS, Azure and GCP).
Currently, that list is composed using the following matchers:

- `lib/config.AWSMatcher`
- `lib/config.AzureMatcher`
- `lib/config.GCPMatcher`

In order to use those types, we must first define them as protobuf types and then import them in `lib/config`.

This way, we have a single place where we define the matchers properties.
They are then used when loading the configuration from file or when defining `DiscoveryConfig` resources.

Example:

```yaml
kind: DiscoveryConfig
version: v1
metadata:
  name: production-resources
spec:
  discovery_group: prod-resources-all-clouds
  aws:
    - types: ["ec2"]
      regions: ["us-east-1", "us-west-1"]
      tags:
        "*": "*"
      install:
        join_params:
          token_name: "aws-discovery-iam-token"
        script_name: "default-installer"
      ssm:
        document_name: "TeleportDiscoveryInstaller"
  azure:
    - types: ["aks"]
      regions: ["eastus", "westus"]
      subscriptions: ["11111111-2222-3333-4444-555555555555"]
      resource_groups: ["group1", "group2"]
      tags:
        "*": "*"
  gcp:
    - types: ["gke"]
      locations: ["*"]
      tags:
        "*": "*"
      project_ids: ["myproject"]
```

### Discovery Group

The Discovery Group is used to match between `discovery_service` and `DiscoveryConfig`.

So, all `discovery_service`s with Group _X_ will include all the matchers from `DiscoveryConfig`s whose Group is also _X_.

### Hot-reloading `DiscoveryConfig` matchers

When the `discovery_service` starts, it loads all the static matchers plus the dynamic ones (using `DiscoveryConfig` resources).

When the user changes (either creating, deleting or updating) a `DiscoveryConfig`, the `discovery_service` must update its discovery matchers.

To achieve this, `discovery_service` must create a Watcher over the `DiscoveryConfig` resource and when a new change occurs update its internal list of matchers.

```go
	watcher, _ := clt.NewWatcher(ctx, types.Watch{
		Kinds: []types.WatchKind{{Kind: types.KindDiscoveryConfig}},
	})

	defer watcher.Close()

	for {
		select {
		case event := <-watcher.Events():
			// ...
			discoveryConfig := event.Resource
			switch event.Type {
			case types.OpDelete:
				discoveryWatchers.RemoveByDiscoveryConfigName(discoveryConfig.GetName())
			case types.OpPut:
				discoveryWatchers.UpsertDiscoveryConfig(discoveryConfig)
			}

			return nil

		case <-watcher.Done():
			if err := watcher.Error(); err != nil {
				return trace.Wrap(err)
			}
			return nil
		case <-ctx.Done():
			return nil
		}
	}
```

### Proto Specification for `DiscoveryConfig`

`DiscoveryConfig` will have the following protobuf specification

```proto
// DiscoveryConfigV1 represents a DiscoveryConfig to be used by discovery_services.
message DiscoveryConfigV1 {
  // Header is the resource header.
  ResourceHeader Header = 1;

  // Spec is an DiscoveryConfig specification.
  DiscoveryConfigSpecV1 Spec = 2 ;
}

// DiscoveryConfigSpecV1 contains properties required to create matchers to be used by discovery_service.
// Those matchers are used by discovery_service to watch for cloud resources and create them in Teleport.
message DiscoveryConfigSpecV1 {
  // DiscoveryGroup is used by discovery_service to add extra matchers.
  // All the discovery_services that have the same discovery_group, will load the matchers of this resource.
  string DiscoveryGroup = 1;

  // AWS is a list of AWSMatchers.
  repeated AWSMatcher aws = 2;

  // Azure is a list of AzureMatchers.
  repeated AzureMatcher azure = 3;

  // GCP is a list of GCPMatchers.
  repeated GCPMatcher gcp = 4;
}

// AWSMatcher matches AWS EC2 instances and AWS Databases
message AWSMatcher {
  // Types are AWS database types to match, "ec2", "rds", "redshift", "elasticache",
	// or "memorydb".
  repeated string Types = 1;
  // Regions are AWS regions to query for databases.
	repeated string Regions = 2;
	// AssumeRoleARN is the AWS role to assume for database discovery.
	string AssumeRoleARN = 3;
	// ExternalID is the AWS external ID to use when assuming a role for
	// database discovery in an external AWS account.
	string ExternalID = 4;
	// Tags are AWS tags to match.
  wrappers.LabelValues Tags = 5;
	// InstallParams sets the join method when installing on
	// discovered EC2 nodes
	MatcherInstallParams InstallParams = 6;
	// SSM provides options to use when sending a document command to
	// an EC2 node
	AWSSSM SSM = 7;
}

// MatcherInstallParams sets join method to use on discovered nodes
message MatcherInstallParams {
	// JoinParams sets the token and method to use when generating
	// config on cloud instances
	DiscoveryJoinParams JoinParams = 1;
	// ScriptName is the name of the teleport installer script
	// resource for the cloud instance to execute
	string ScriptName = 2;
	// InstallTeleport disables agentless discovery
	string InstallTeleport = 3;
	// SSHDConfig provides the path to write sshd configuration changes
	string SSHDConfig = 4;
	// PublicProxyAddr is the address of the proxy the discovered node should use
	// to connect to the cluster. Used only in Azure.
	string PublicProxyAddr = 5;
}

// AWSSSM provides options to use when executing SSM documents
message AWSSSM {
	// DocumentName is the name of the document to use when executing an
	// SSM command
	string DocumentName = 1;
}

// JoinParams configures the parameters for Simplified Node Joining.
message DiscoveryJoinParams {
  string TokenName = 1;
	string Method = 2;
	AzureJoinParams Azure = 3;
}

// AzureJoinParams configures the parameters specific to the Azure join method.
message AzureJoinParams {
  string ClientID = 1;
}

// AzureMatcher matches Azure resources.
// It defines which resource types, filters and some configuration params.
message AzureMatcher {
	// Subscriptions are Azure subscriptions to query for resources.
	repeated string Subscriptions = 1;
	// ResourceGroups are Azure resource groups to query for resources.
	repeated string ResourceGroups = 2;
	// Types are Azure types to match: "mysql", "postgres", "aks", "vm"
	repeated string Types = 3;
	// Regions are Azure locations to match for databases.
	repeated string Regions = 4;
	// ResourceTags are Azure tags on resources to match.
  wrappers.LabelValues ResourceTags = 5;
	// InstallParams sets the join method when installing on
	// discovered Azure nodes.
	MatcherInstallParams InstallParams = 6;
}

// GCPMatcher matches GCP resources.
message GCPMatcher {
	// Types are GKE resource types to match: "gke".
	repeated string Types = 1;
	// Locations are GKE locations to search resources for.
	repeated string Locations = 2;
	// Tags are GCP labels to match.
  wrappers.LabelValues Tags = 3;
	// ProjectIDs are the GCP project ID where the resources are deployed.
	repeated string ProjectIDs = 4;
}

```

### Backward Compatibility

#### Watcher on `DiscoveryConfig`

If the `discovery_service` tries to lookup `DiscoveryConfig` resources, but Teleport Auth/Proxy is not yet aware of this resource, then `discovery_service` will log a warning but continue with the static configuration.

### UX

#### `discovery_service` new service configuration properties

Users need to ensure the `discovery_group` (which is optional) is set, in order for it to include extra matchers from `DiscoveryConfig`.

Example:

```yaml
discovery_service:
  enabled: "yes"
  discovery_group: "my-production-resources"
```

#### gRPC: manage `DiscoveryConfig` resource

The following methods will be created in the gRPC server to allow `DiscoveryConfig` resource management:

```proto
// DiscoveryConfigService provides methods to manage DiscoveryConfig resources.
// These resources are used by `discovery_service` to set up the matchers.
service DiscoveryConfigService {
  // ListDiscoveryConfig returns a paginated list of DiscoveryConfig resources.
  rpc ListDiscoveryConfig(ListDiscoveryConfigRequest) returns (ListDiscoveryConfigResponse);

  // GetDiscoveryConfig returns the specified DiscoveryConfig resource.
  rpc GetDiscoveryConfig(GetDiscoveryConfigRequest) returns (types.DiscoveryConfigV1);

  // CreateDiscoveryConfig creates a new DiscoveryConfig resource.
  rpc CreateDiscoveryConfig(CreateDiscoveryConfigRequest) returns (types.DiscoveryConfigV1);

  // UpdateDiscoveryConfig updates an existing DiscoveryConfig resource.
  rpc UpdateDiscoveryConfig(UpdateDiscoveryConfigRequest) returns (types.DiscoveryConfigV1);

  // DeleteDiscoveryConfig removes the specified DiscoveryConfig resource.
  rpc DeleteDiscoveryConfig(DeleteDiscoveryConfigRequest) returns (google.protobuf.Empty);

  // DeleteAllDiscoveryConfigs removes all DiscoveryConfigs.
  rpc DeleteAllDiscoveryConfigs(DeleteAllDiscoveryConfigsRequest) returns (google.protobuf.Empty);
}
```

#### WebAPI: manage `DiscoveryConfig` resource

The following endpoints must be create to be used by WebAPI:

```
Methods:
GET .../discoveryconfig
GET .../discoveryconfig/:id
POST .../discoveryconfig
PUT .../discoveryconfig/:id
DELETE .../discoveryconfig/:id
```

JSON representation:

```json
{
  "discovery_group": "prod-resources-all-clouds",
  "aws": [
    {
      "types": ["ec2"],
      "regions": ["us-east-1", "us-west-1"],
      "tags": {
        "*": "*"
      },
      "install": {
        "join_params": {
          "token_name": "aws-discovery-iam-token"
        },
        "script_name": "default-installer"
      },
      "ssm": {
        "document_name": "TeleportDiscoveryInstaller"
      }
    }
  ],
  "azure": [
    {
      "types": ["aks"],
      "regions": ["eastus", "westus"],
      "subscriptions": ["11111111-2222-3333-4444-555555555555"],
      "resource_groups": ["group1", "group2"],
      "tags": {
        "*": "*"
      }
    }
  ],
  "gcp": [
    {
      "types": ["gke"],
      "locations": ["*"],
      "tags": {
        "*": "*"
      },
      "project_ids": ["myproject"]
    }
  ]
}
```

#### `tctl`: manage `DiscoveryConfig` resource

Users must be able to create, list, read, update and delete `DiscoveryConfig`s using `tctl`.

Example of `$ tctl get discovery_config`:

```yaml
kind: DiscoveryConfig
version: v1
metadata:
  name: production-resources
spec:
  discovery_group: my-ec2
  aws:
    - types: ["ec2"]
      regions: ["us-east-1", "us-west-1"]
      tags:
        "*": "*"
```

#### IaC - Terraform

A new resource `DiscoveryConfig` must be created to allow its management from the Terraform provider.

#### IaC - Kube Operator

A new resource `DiscoveryConfig` must be created to allow its management from the Kube Operator.

#### IaC - Helm Charts

Helm charts must support the new property when setting up a `discovery_service`.

### Security

#### RBAC for `DiscoveryConfig` resource

The `editor` preset role will include read and write access to this new resource.

The `discovery` system role (used by `discovery_service`) must be able to list and read `DiscoveryConfig` resources to be able to update the matchers of its service.

### Static snapshot publication

Discovery Services self-publish their static (file-based) matcher configuration as owner-managed "static snapshot" `DiscoveryConfig` resources so the backend can expose each service's effective configuration. The snapshot reuses the existing resource schema and RPC surface; no new protobuf messages or RPCs:

- The resource carries `sub_kind: static-snapshot`, origin `config-file`, and the observed discovery group and sanitized matchers in the ordinary `spec`. Installer parameters are excluded from the inventory in their entirety; publication is fail-closed (an unsanitized spec is rejected), and storage reads re-sanitize loaded records so the invariant holds even for stored bytes that predate it. Validation never enriches the stored inventory: the snapshot spec is validated against a throwaway copy, so values that ordinary matcher defaulting derives (installer parameters, SSM document names, wildcard scoping such as Azure regions or GCP locations) are never persisted as if the publisher had sent them, and the stored record stays exactly the publisher's effective configuration. Inventory validation runs at the RPC boundary and again in the storage layer, which validates on every write and read like any other resource; the snapshot marshal additionally enforces the byte-level invariants, namely the absence of installer params and the stored-size cap. A service configured without a discovery group publishes an empty group: only the static-snapshot subkind relaxes the group-required validation.

- Writes reuse `UpsertDiscoveryConfig`, routed before generic RBAC when the caller is a built-in Discovery Service identity and the payload carries the static-snapshot subkind. The server derives the name from the caller's server ID and rebuilds every trusted envelope field; only the spec is taken from the request. All other callers keep the regular path, which discards subkinds, so a user cannot self-declare a snapshot. On Auth servers that predate the feature, publication fails with `AccessDenied` (the Discovery role has no generic write verbs), which publishers must treat as "unsupported version."

- Snapshots persist in an isolated backend range that no `DiscoveryConfig` event parser watches and generic listings never touch, so they cannot surface through caches, watchers, or `ListDiscoveryConfigs`, the channels Discovery Services use to load dynamic matchers. This isolation is what makes spec-carried matchers safe for already-released services.

- Reads reuse `GetDiscoveryConfig`: regular resources first, snapshots second for reserved names. No channel carries snapshot matchers back to a Discovery Service. A service may fetch its own snapshot inventory-stripped (envelope and status only), enabling read-modify-write status reporting to merge against stored history. Foreign snapshot names return the unoccupied `NotFound`, and publication and status-update responses echo the same inventory-stripped shape.

  Clients that report no version or a version older than v19.0.0 also receive `NotFound` because their unconditional validation cannot decode a group-less snapshot. All v19 prereleases count as too old: multiple builds can report the same prerelease version while straddling the feature's introduction. A client version that does not parse is rejected with `BadParameter`, consistent with the regular-config downgrade path, rather than folded into `NotFound`. Served snapshots pass through the same client-version downgrade pipeline as regular configs (a no-op today, since every current downgrade rule targets clients older than the gate). The gate itself is transition code, marked for deletion once v19 is the oldest supported client version.

  The initial implementation ships named snapshot reads only, and there is intentionally no snapshot List API: generic listings, including `tctl get discovery_config`, return regular configs without combining snapshots.

  Instance-driven enumeration and its presentation are follow-up work. A consumer will list the existing TTL-backed `Instance` inventory filtered to `RoleDiscovery`, derive each snapshot name from the instance's server ID, and perform bounded-concurrency named gets, treating `NotFound` as expected when an instance heartbeat precedes the first successful snapshot publication. Consumers of that follow-up will need `instance` list/read access in addition to `DiscoveryConfig` permissions.

- Renewal replaces the spec and preserves the stored status; status reporting replaces the status and preserves the spec and expiry. A single stored-size cap covers the merged record. A previously accepted status can block a larger inventory renewal only until the record TTL expires: the fresh publication carries no status, after which the oversized report no longer fits and fails loudly instead of being reaccepted.

  Publication and reporting run independently, so both use bounded read/merge/revision-CAS updates (a small fixed number of attempts with jittered linear backoff) rather than unconditional full-resource writes. This reuses the existing dynamic `DiscoveryConfig` reporting path. Unlike a service heartbeat with one full-resource writer, either unconditional writer could otherwise restore stale inventory or status over the other. The two failure categories stay distinct: exceeding the stored-size cap returns `LimitExceeded`, while exhausting the CAS attempts returns `CompareFailed`, so a publisher can tell an oversized record (shrink it, or wait for the TTL to clear a blocking status) from write contention (retry soon).

## Namespace compatibility

Names of the form `static-snapshot-<canonical-UUID>` and the equivalent hashed form (for historical non-UUID server IDs) are reserved for snapshots.

Create, Update, and Upsert reject every reserved name with `BadParameter`, with guidance to recreate the config under a different name. The check is a pure name-shape test: it reads nothing from the backend, so the response is identical whether the name is occupied by a grandfathered regular config, a snapshot, or nothing; these three verbs disclose no occupancy information to any caller.

A regular `DiscoveryConfig` that used a reserved name before the namespace was reserved remains readable and deletable, and continues to be served to its discovery group and to accept status reports for as long as it exists. Its spec is frozen: the migration path is to read the config, delete it, and recreate it under a non-reserved name. While such a config exists, the owner's snapshot publication remains blocked.

Whether a snapshot occupies a reserved name is disclosed (via the owner-managed `AccessDenied` on delete attempts) only to callers holding the read verb, who could learn it through Get anyway. All other callers, including foreign Discovery Services using the status RPC, receive exactly the unoccupied-name responses, so no RPC works as a snapshot-existence oracle.

Freezing grandfathered specs rather than supporting in-place management is a policy choice. Allowing updates would keep dual occupancy alive indefinitely and require update-only upsert semantics with deletion-race handling; the freeze makes every write verb's reserved-name behavior a stateless check and guarantees the namespace converges to its reserved owner through exactly one operator action, which the rejection error spells out.
