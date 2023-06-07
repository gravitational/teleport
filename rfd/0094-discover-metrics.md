---
authors: Roman Tkachenko (roman@goteleport.com)
state: implemented
---

# RFD 0094 - Teleport Discover Metrics

## Required Approvers

Product: @klizhentas && @xinding33

## What

This RFD proposes additional events/metrics for Teleport Discover flows to be
added to our Posthog-based reporting system.

## Why

Existing metrics don't give us enough insight into Teleport Discover performance
on its main KPIs such as "time to add a resource", or sufficient granularity to
be able to understand what causes most friction.

Currently, we only have information about account creation time (in Cloud) and
aggregated resource counts. This allows us to answer the question like:

* How much time has passed since the user signed up and registered first server?

This is a useful metric but alone doesn't tell us much since the user may have
signed up and only added the first server next day.

More granular events would allow us to better understand how long it takes
users to register resources, where they get stuck, and make more informed
decisions about whether changes we make to the Discover flows actually improve
things.

## Scope

* Initially we'll focus on metrics that cover Day One experience which will be
  an indicator of how easy/hard it is to add a single resource. This RFD will
  evolve over time to add more metrics once we implement more protocols and
  Day Two which covers auto-discovery.
* We'll mostly be targeting Cloud deployments but same approach should extend
  to self-hosted deployments once our reporting machinery supports it.

## Requirements

* The new events should use the same Posthog-based reporting machinery we
  use to instrument other parts of Teleport and website.
* The events should not include any customer identifiable information.

## Details

We want to be able to answer the following question to analyze the full Discover
flow. Below, by "resource" we mean a server, Kubernetes cluster, database,
application or Windows desktop.

* _Once the user signed up, how long did it take them to add a first resource?_
  It's the question we can answer today and, as discussed above, not very useful
  by itself but still needed for a full picture.

* _How long did the actual process of adding a first resource took?_ Meaning,
  from the moment they started going through the Discover Wizard (a datapoint
  we lack today) to the successful completion.

* _How long does adding a resource take?_ It's a more generic version of the
  above, which is a general indicator of how easy it is to add new resource.

* _Which steps of adding a new resource take most time?_ Discover is a multi-
  step wizard and this will help understand where users get stuck the most.

* _Which types of resources/protocols take most time?_ To be able to pinpoint,
  for example, which databases users have most issues with.

* _Where do users experience failures when adding resources?_ For example,
  agent does not join in time or connection tester fails.

Most of the questions we're interested in can be answered using PostHog
[funnels](https://posthog.com/manual/funnels) as they are designed to help
understand how users progress through the product flows and where the
bottlenecks and friction are.

To be able to build a Discover funnel, each step the user goes through needs
to be defined as an event. Below is a Protobuf spec containing proposed events
metadata. Note that each event will include anonymized cluster name and timestamp
as shown in our [reporting spec](https://github.com/gravitational/prehog/blob/main/proto/prehog/v1alpha/teleport.proto)
so below we only include event metadata.

```protobuf
// Contains common metadata for Discover related events.
message DiscoverMetadata {
  // Uniquely identifies Discover wizard "session". Will allow to correlate
  // events within the same Discover wizard run. Can be UUIDv4.
  string id = 1;
}

// Represents a resource type that is being added.
enum DiscoverResource {
  UNSPECIFIED = 0;
  SERVER = 1;
  KUBERNETES = 2;
  DATABASE_POSTGRES_SELF_HOSTED = 3;
  DATABASE_MYSQL_SELF_HOSTED = 4;
  DATABASE_MONGODB_SELF_HOSTED = 5;
  DATABASE_POSTGRES_RDS = 6;
  DATABASE_MYSQL_RDS = 7;
  APPLICATION_HTTP = 8;
  APPLICATION_TCP = 9;
  WINDOWS_DESKTOP = 10;
}

// Contains common metadata identifying resource type being added.
message DiscoverResourceMetadata {
  // Resource type that is being added.
  DiscoverResource resource = 1;
}

// Represents a Discover step outcome.
enum DiscoverStatus {
  UNSPECIFIED = 0;
  // The user tried to complete the action and it succeeded.
  SUCCESS = 1;
  // The system skipped the step.
  // For example:
  // When setting up a Database and there's already a Database Service proxying the DB.
  // In this case the Database Agent installation is skipped.
  SKIPPED = 2;
  // The user tried to complete the action and it failed.
  ERROR = 3;
  // The user did not complete the action and left the wizard.
  ABORTED = 4;
}

// Contains fields that track a particular step outcome, for example connection
// test failed or succeeded, or user aborted the step.
message DiscoverStepStatus {
  // Indicates the step outcome.
  DiscoverStatus status = 1;
  // Contains error details in case of Error Status.
  // We have to be careful to not include any identifyable information like server addresses here.
  string error = 2;
}

// Emitted when the wizard opens.
message UIDiscoverStartedEvent {
  DiscoverMetadata metadata = 1;
  DiscoverStepStatus status = 2;
}

// Emitted when user selected resource type to add and proceeded to the next
// step.
message UIDiscoverResourceSelectionEvent {
  DiscoverMetadata metadata = 1;
  DiscoverResourceMetadata resource = 2;
  DiscoverStepStatus status = 3;
}


// Emitted when user is offered to install a Teleport Agent.
// For SSH this is the Teleport 'install-node' script.
//
// For Kubernetes this is the teleport-agent helm chart installation.
//
// For Database Access this step is the installation of the teleport 'install-db' script.
// It can be skipped if the cluster already has a Database Service capable of proxying the database.
message UIDiscoverDeployServiceEvent {
  DiscoverMetadata metadata = 1;
  DiscoverResourceMetadata resource = 2;
  DiscoverStepStatus status = 3;
}

// Emitted when a user registered a database resource and proceeded to the next
// step.
message UIDiscoverConfigureRegisterDatabaseEvent {
  DiscoverMetadata metadata = 1;
  DiscoverResourceMetadata resource = 2;
  DiscoverStepStatus status = 3;
}

// Emitted when a user configured mutual TLS for self-hosted database and
// proceeded to the next step.
message UIDiscoverConfigureDatabaseMTLSEvent {
  DiscoverMetadata metadata = 1;
  DiscoverResourceMetadata resource = 2;
  DiscoverStepStatus status = 3;
}

// Emitted when a user configured IAM for RDS database and proceeded to the
// next step.
message UIDiscoverConfigureDatabaseIAMPolicyEvent {
  DiscoverMetadata metadata = 1;
  DiscoverResourceMetadata resource = 2;
  DiscoverStepStatus status = 3;
}

// Emitted on "Setup Access" screen when user has updated their principals
// and proceeded to the next step.
message UIDiscoverSetUpAccessEvent {
  DiscoverMetadata metadata = 1;
  DiscoverResourceMetadata resource = 2;
  DiscoverStepStatus status = 3;
}

// Emitted on the "Test Connection" screen when user clicked tested connection
// to their resource.
message UIDiscoverTestConnectionEvent {
  DiscoverMetadata metadata = 1;
  DiscoverResourceMetadata resource = 2;
  DiscoverStepStatus status = 3;
}

// Emitted when user completes the Discover wizard.
message UIDiscoverCompletedEvent {
  DiscoverMetadata metadata = 1;
  DiscoverResourceMetadata resource = 2;
  DiscoverStepStatus status = 3;
}
```

When converted to Posthog [events](https://posthog.com/docs/how-posthog-works/data-model#event)
these fields will become "properties" so that they can be filtered on. Funnels
in Posthog are built from a sequence of event types. The events can be filtered
based on their properties so using this data model we should be able to build
funnels for each separate Discover track like servers, self-hosted database,
RDS databases, etc.


## Events reference

### Event properties definition
#### `tp.discover.metadata.id`
Unique ID that identifies the discover session.
Created as soon as the user enters the Discover Wizard screen.

#### `tp.discover.resource.name`
Name that identifies the resource kind the user is trying to add.

One of the following names:
```
DISCOVER_RESOURCE_SERVER
DISCOVER_RESOURCE_KUBERNETES
DISCOVER_RESOURCE_DATABASE_POSTGRES_SELF_HOSTED
DISCOVER_RESOURCE_DATABASE_MYSQL_SELF_HOSTED
DISCOVER_RESOURCE_DATABASE_MONGODB_SELF_HOSTED
DISCOVER_RESOURCE_DATABASE_POSTGRES_RDS
DISCOVER_RESOURCE_DATABASE_MYSQL_RDS
DISCOVER_RESOURCE_APPLICATION_HTTP
DISCOVER_RESOURCE_APPLICATION_TCP
DISCOVER_RESOURCE_WINDOWS_DESKTOP
DISCOVER_RESOURCE_DATABASE_SQLSERVER_RDS
DISCOVER_RESOURCE_DATABASE_POSTGRES_REDSHIFT
DISCOVER_RESOURCE_DATABASE_SQLSERVER_SELF_HOSTED
DISCOVER_RESOURCE_DATABASE_REDIS_SELF_HOSTED
DISCOVER_RESOURCE_DATABASE_POSTGRES_GCP
DISCOVER_RESOURCE_DATABASE_MYSQL_GCP
DISCOVER_RESOURCE_DATABASE_SQLSERVER_GCP
```

#### `tp.discover.step.status`
Each event is considered a step.
The outcome of that step is reflected in this field.

The following status are available:
```
DISCOVER_STATUS_SUCCESS: the step was completed successfully
DISCOVER_STATUS_SKIPPED: the step was skipped by the user
DISCOVER_STATUS_ERROR: there was an error when the user tried to complete the step
DISCOVER_STATUS_ABORTED: the user left the step by closing the browser tab/window or leaving the wizard
```

#### `tp.discover.step.error`
When the previous property has the `DISCOVER_STATUS_ERROR` value, then an error message exists.

### Events
#### `tp.ui.discover.started`
Emitted when the wizard starts.

Event properties:
- `tp.discover.metadata.id`
- `tp.discover.step.status`
- `tp.discover.step.error`


#### `tp.ui.discover.resourceSelection`
Emitted when the user selects a resource and proceeds to the next step.

Event properties:
- `tp.discover.metadata.id`
- `tp.discover.resource.name`
- `tp.discover.step.status`
- `tp.discover.step.error`


#### `tp.ui.discover.deployService`
Emitted when the UI detects a new service (either an SSH Node or a DB service) in the cluster.
This happens after the user is asked to run a script on the target host.

Event properties:
- `tp.discover.metadata.id`
- `tp.discover.resource.name`
- `tp.discover.step.status`
- `tp.discover.step.error`


#### `tp.ui.discover.database.register`
Emitted when the user configures a new Database resource (database name and its endpoint).

Event properties:
- `tp.discover.metadata.id`
- `tp.discover.resource.name`
- `tp.discover.step.status`
- `tp.discover.step.error`


#### `tp.ui.discover.database.configure.mtls`
Emitted when the user is asked to configure the mTLS settings for Database Self Hosted flow.

Event properties:
- `tp.discover.metadata.id`
- `tp.discover.resource.name`
- `tp.discover.step.status`
- `tp.discover.step.error`


#### `tp.ui.discover.database.configure.iampolicy`
Emitted when the user is asked to configure the IAM Policy to allow access during the Database MySQL/Postgres RDS flow.

Event properties:
- `tp.discover.metadata.id`
- `tp.discover.resource.name`
- `tp.discover.step.status`
- `tp.discover.step.error`


#### `tp.ui.discover.desktop.activeDirectory.tools.install`
Emitted when the user is asked to configure Active Directory for during the Windows Desktop flow.

Event properties:
- `tp.discover.metadata.id`
- `tp.discover.resource.name`
- `tp.discover.step.status`
- `tp.discover.step.error`


#### `tp.ui.discover.desktop.activeDirectory.configure`
Emitted when the user is asked to configure Active Directory for during the Windows Desktop flow.

Event properties:
- `tp.discover.metadata.id`
- `tp.discover.resource.name`
- `tp.discover.step.status`
- `tp.discover.step.error`


#### `tp.ui.discover.autoDiscoveredResources`
After configuring the ActiveDirectory and Windows Desktop Service, the user will receive a list of auto discovered Windows machines.

This event might be used for other purposes as we add more "bulk" import features to the wizard.

Event properties:
- `tp.discover.metadata.id`
- `tp.discover.resource.name`
- `tp.discover.step.status`
- `tp.discover.step.error`
- `tp.discover.auto_discovered.count` : how many resources were auto-discovered


#### `tp.ui.discover.principals.configure`
This step is about adding traits to the current user and for the current resource.
This can be SSH user names, DB users and logical database names, Kube users and groups and so on.

Event properties:
- `tp.discover.metadata.id`
- `tp.discover.resource.name`
- `tp.discover.step.status`
- `tp.discover.step.error`


#### `tp.ui.discover.testConnection`
Emitted when the user tests the connection.

Event properties:
- `tp.discover.metadata.id`
- `tp.discover.resource.name`
- `tp.discover.step.status`
- `tp.discover.step.error`


#### `tp.ui.discover.completed`
Emitted when the user reaches the final screen and closes the wizard.

Event properties:
- `tp.discover.metadata.id`
- `tp.discover.resource.name`
- `tp.discover.step.status`
- `tp.discover.step.error`
