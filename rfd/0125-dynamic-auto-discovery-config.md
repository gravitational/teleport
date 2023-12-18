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
       regions: ["us-east-1","us-west-1"]
       tags:
         "*": "*"
       install:
         join_params:
           token_name:  "aws-discovery-iam-token"
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
   "aws":[
      {
         "types":[
            "ec2"
         ],
         "regions":[
            "us-east-1",
            "us-west-1"
         ],
         "tags":{
            "*":"*"
         },
         "install":{
            "join_params":{
               "token_name":"aws-discovery-iam-token"
            },
            "script_name":"default-installer"
         },
         "ssm":{
            "document_name":"TeleportDiscoveryInstaller"
         }
      }
   ],
   "azure":[
      {
         "types":[
            "aks"
         ],
         "regions":[
            "eastus",
            "westus"
         ],
         "subscriptions":[
            "11111111-2222-3333-4444-555555555555"
         ],
         "resource_groups":[
            "group1",
            "group2"
         ],
         "tags":{
            "*":"*"
         }
      }
   ],
   "gcp":[
      {
         "types":[
            "gke"
         ],
         "locations":[
            "*"
         ],
         "tags":{
            "*":"*"
         },
         "project_ids":[
            "myproject"
         ]
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
       regions: ["us-east-1","us-west-1"]
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