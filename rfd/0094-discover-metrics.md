---
authors: Roman Tkachenko (roman@goteleport.com)
state: draft
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
  RESOURCE_SERVER = 0;
  RESOURCE_KUBERNETES = 1;
  RESOURCE_DATABASE_POSTGRES_SELF_HOSTED = 2;
  RESOURCE_DATABASE_MYSQL_SELF_HOSTED = 3;
  RESOURCE_DATABASE_MONGODB_SELF_HOSTED = 4;
  RESOURCE_DATABASE_POSTGRES_RDS = 5;
  RESOURCE_DATABASE_MYSQL_RDS = 6;
  RESOURCE_APPLICATION_HTTP = 7;
  RESOURCE_APPLICATION_TCP = 8;
  RESOURCE_WINDOWS_DESKTOP = 9;
}

// Contains common metadata identifying resource type being added.
message ResourceMetadata {
    // Resource type that is being added.
    DiscoverResource resource = 1;
}

// Represents a Discover step outcome.
enum DiscoverStatus {
  STATUS_SUCCESS = 0;
  STATUS_ERROR = 1;
  STATUS_ABORTED = 2;
}

// Contains fields that track a particular step outcome, for example connection
// test failed or succeeded, or user aborted the step.
message Status {
    // Indicates the step outcome. For example, "success" means user proceeded
    // to the next step, "error" if something failed (e.g. connection test),
    // "aborted" if user exited the wizard.
    DiscoverStatus status = 1;
    // Contains error details. We have to be careful to not include any
    // identifyable infomation like server addresses here.
    string error = 2 ;
}

// Emitted when user selected resource type to add and proceeded to the next
// step.
message DiscoverResourceSelectionEvent {
    DiscoverMetadata discover_metadata = 1;
    ResourceMetadata resource_metadata = 2;
    Status status = 3;
}

// Emitted on the "Configure Resource" screen when user has connected an SSH
// or Kubernetes agent and proceeded to the next step.
//
// For Database Access this step has sub-steps so will have individual events
// defined below, however this event will be emitted as well at the end of
// the last sub-step to be able to track it on a more high-level.
message DiscoverConfigureResourceEvent {
    DiscoverMetadata discover_metadata = 1;
    ResourceMetadata resource_metadata = 2;
    Status status = 3;
}

// Emitted when a user registered a database resource and proceeded to the next
// step.
message DiscoverRegisterDatabase {
    DiscoverMetadata discover_metadata = 1;
    ResourceMetadata resource_metadata = 2;
    Status status = 3;
}

// Emitted when a user deployed a database agent and proceeded to the next step.
message DiscoverDeployDatabaseAgent {
    DiscoverMetadata discover_metadata = 1;
    ResourceMetadata resource_metadata = 2;
    Status status = 3;
}

// Emitted when a user configured mutual TLS for self-hosted database and
// proceeded to the next step.
message DiscoverConfigureMTLS {
    DiscoverMetadata discover_metadata = 1;
    ResourceMetadata resource_metadata = 2;
    Status status = 3;
}

// Emitted when a user configured IAM for RDS database and proceeded to the
// next step.
message DiscoverConfigureIAM {
    DiscoverMetadata discover_metadata = 1;
    ResourceMetadata resource_metadata = 2;
    Status status = 3;
}

// Emitted on "Setup Access" screen when user has updated their principals
// and proceeded to the next step.
message DiscoverSetUpAccessEvent {
    DiscoverMetadata discover_metadata = 1;
    ResourceMetadata resource_metadata = 2;
    Status status = 3;
}

// Emitted on the "Test Connection" screen when user clicked tested connection
// to their resource.
message DiscoverTestConnectionEvent {
    DiscoverMetadata discover_metadata = 1;
    ResourceMetadata resource_metadata = 2;
    Status status = 3;
}

// Emitted when user completes the Discover wizard.
message DiscoverCompletedEvent {
    DiscoverMetadata discover_metadata = 1;
    ResourceMetadata resource_metadata = 2;
}
```

When converted to Posthog [events](https://posthog.com/docs/how-posthog-works/data-model#event)
these fields will become "properties" so that they can be filtered on. Funnels
in Posthog are built from a sequence of event types. The events can be filtered
based on their properties so using this data model we should be able to build
funnels for each separate Discover track like servers, self-hosted database,
RDS databases, etc.
