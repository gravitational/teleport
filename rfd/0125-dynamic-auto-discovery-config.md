---
authors: Marco Dinis (marco.dinis@goteleport.com)
state: draft
---

# RFD 0125 - Dynamic Auto-Discovery Configuration

Related RFDs:
- RFD 57 - Automatic discovery and enrollment of AWS servers (`discovery_service` RFD)

## Required Approvers

- Engineering: `@r0mant`
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
There will be a new resource - `DiscoveryConfig` - which will be used by the `discovery_service` to create a set of matchers for that particular service.

We'll create relations between those two entities using labels and label matchers.
This is common for other type of services, eg `db_service` has the `resources.labels` labels matcher which allows users to define which Databases to proxy based on their labels.

### New `DiscoveryConfig` resource
A new resource will be created to store the desired matchers.

The `spec` schema of the `DiscoveryConfig` contains all the required fields for matching on resources on supported clouds.

Currently, the schema is compose using the following matchers:
- `lib/config.AWSMatcher`
- `lib/config.AzureMatcher`
- `lib/config.GCPMatcher`

In order to use those types, we must first define them as protobuf types and then import them in `lib/config`.

This way, we have a single place where we define the matchers properties.
They are then used for file reading or for defining `DiscoveryConfig` resources.

Example:
```yaml
kind: DiscoveryConfig
version: v1
metadata:
  name: production-resources
  labels:
    cloud: aws
    env: prod
spec:
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
The Discovery Group property will not be ported to `DiscoveryConfig` because it exists to ensure that multiple `discovery_services` don't step on each other.

Running `discovery_service`s in high availability (multiple instances and same config) should yield the same result as before.

### New `discovery_service` property: `selector`
The configuration of the `discovery_service` has a new field: `selector`.

This field works as a label matcher for `DiscoveryConfig` resources.
Standard label matcher rules apply here (eg, Role's `node_labels` or Database Service's `resources.labels`).

For each matching `DiscoveryConfig`, the list of matchers is appended to the `discovery_service` matchers.

Example:
```yaml
discovery_service:
  enabled: "yes"
  discovery_group: my_group
  selector:
    cloud: aws
    env: [qa, prod]
  aws:
  - types: ["ec2"]
    regions: ["us-east-1","us-west-1"]
    # ...
  azure:
  - types: ["aks"]
    regions: ["eastus", "westus"]
    # ...
  gcp:
  - types: ["gke"]
    # ...
```

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
  // AWS is a list of AWSMatchers.
  repeated AWSMatcher aws = 1;

  // Azure is a list of AzureMatchers.
  repeated AzureMatcher azure = 2;

  // GCP is a list of GCPMatchers.
  repeated GCPMatcher gcp = 3; 
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
	// to connect to the cluster. Used ony in Azure.
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

#### `discovery_service.selector`
If the config has this field but Teleport version is no aware of that, the user must remove it before they can start the Discovery Service.

#### Watcher on `DiscoveryConfig`
If the `discovery_service` tries to set up a watcher over `DiscoveryConfig` but Teleport Auth/Proxy is not yet aware of this resource, then `discovery_service` will log a warning but continue with the static configuration.

### UX

#### `discovery_service` new service configuration properties
Users need to add the `selector` property and then a list of label matchers.

Example:
```yaml
discovery_service:
  enabled: "yes"
  selector:
    labelA: value1 # matches against a single value
    labelB: [value2, value3] # matches against multiple values
    labelC: "*" # matches on all values
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

Example of `tctl get discovery_config`:
```
$ tctl get discovery_config
kind: DiscoveryConfig
version: v1
metadata:
  name: production-resources
  labels:
    cloud: aws
    env: prod
spec:
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