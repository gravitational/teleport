---
authors: Yassine Bounekhla (yassine@goteleport.com)
state: draft
---

# RFD 0229 - Instances Inventory UI

## Required Approvals

- Engineering: @zmb3 @fspmarshall
- Product: @roraback

## What

A page in the Web UI that lists connected instances along with their details, with options to search and filter through them.

## Why

Currently, the only way to list agents is via the `tctl inventory list` and `tctl bots instances ls` (RFD 222) CLI commands, this can
be quite cumbersome for users and as a result limits the visibility cluster administrators have into their infrastructure. Exposing
this information in the Web UI would greatly improve the user experience and make it easier for cluster administrators to do
things like identify instances still running old versions of Teleport and need to be updated.

## Details

### Implementation

The first version of this feature will primarily focus on adding the existing `tctl inventory` and `tctl bots instances` functionality
to the Web UI and use most of the same underlying backend API. Both agents and bot instances will be fetched via streams and
combined into a single paginated response ordered alphabetically by name by leveraging the `MergeStreams` utility.

#### Web Endpoint

A new `/webapi/sites/:site/instances` will be used by the Web UI to fetch a paginated list of instances, this endpoint will support
various query parameters for pagination and filtering, including:

- `search`: Filter by hostname.
- `query`: Filter by a predicate language expression.
- `services`: Filter to only return agents running the specified service(s).
- `upgraders`: Filter to only return agents using the specified external upgrader(s).
- `updaterGroup`: Filter to only return agents in the specified updater group.
- `type`: Filters to return only agents, bot instances, or both. Default/undefined is both.

The response will be a JSON object containing the page of agents requested.

##### Example response:

```json
{
  "instances": [
    {
      "id": "1",
      "type": "agent",
      "agent": {
        "name": "server1",
        "version": "16.1.3",
        "services": ["ssh", "db", "desktop"]
      },
      "upgrader": {
        "type": "unit",
        "version": "v2",
        "group": "group1"
      }
    },
    {
      "id": "2",
      "type": "agent",
      "agent": {
        "name": "kube1",
        "version": "16.5.4",
        "services": ["kube"],
        "upgrader": {
          "type": "kube",
          "version": "v2",
          "group": "group1"
        }
      }
    },
    {
      "id": "3",
      "type": "bot",
      "bot": {
        "name": "bot1",
        "version": "16.1.3",
        "service": "kubernetes",
        "health": "Unknown"
      }
    },
    {
      "id": "4",
      "type": "agent",
      "agent": {
        "name": "agent1",
        "version": "16.1.3",
        "services": ["kube"],
        "upgrader": {
          "type": "kube",
          "version": "v2",
          "group": "group2"
        }
      }
    }
  ],
  "totalCount": 4,
  "startKey": ""
}
```

#### RPC Service and Proto messages

```protobuf
service InventoryService {
  // ListInstances returns a page of Agents and Bot Instances.
  rpc ListInstances(ListInstancesRequest) returns (ListInstancesResponse);
}

// ListInstancesRequest is the request for listing instances.
message ListInstancesRequest {
  // page_size is the size of the page to return.
  int32 page_size = 1;
  // page_token is the next_page_token value returned from a previous ListInstances request, if any.
  string page_token = 2;
  // filters specify optional search criteria to limit which instances should be returned.
  ListInstancesFilter filter = 3;
}

// ListInstancesResponse is the response from listing instances.
message ListInstancesResponse {
  // instances is the instances returned.
  repeated Instance instances = 1;
  // next_page_token contains the next page token to use as the start key for the next page of instances.
  string next_page_token = 2;
  // total_count is the total number of agents and bot instances in the cache.
  int32 total_count = 3;
}

// ListInstancesFilter provides a mechanism to refine ListInstances results.
message ListInstancesFilter {
  // search is a basic string search query which will filter results by `Instance.Spec.Name`.
  string search = 1;
  // advanced_search is an advanced search query using predicate language.
  string advanced_search = 2;
  // types is the types of instances to return (`instance` and/or `bot_instance`). If ommitted, both types will be returned.
  repeated string types = 3;
  // services is the list of services to filter agents by. If ommitted, agents with any services will be returned.
  repeated string types = 4;
  // updater_groups is the list of updater groups to filter instances by.
  repeated string updater_groups = 5;
  // upgraders is the list of upgraders to filter instances by.
  repeated string upgraders = 6;
}

// Instance represents either an agent or a bot instance item with the necessary information for the client.
message Instance {
  // id is the is the id of the instance, for agents this represents the `serverId`.
  string id = 1;
  // sub_kind represents the kind of instance this is, either `instance` or `bot_instance`.
  string sub_kind = 2;
  // metadata is common metadata that all resources share.
  teleport.header.v1.Metadata metadata = 3;
  // spec is the instance specification.
  InstanceSpec spec = 4;
}

// InstanceSpec is the instance specification.
message InstanceSpec {
  // name is the name this instance most recently advertised. `hostname` for agents and `bot_name` for bot instances.
  string name = 1;
  // version is the version of teleport this instance most recently advertised.
  string version = 2;
  // services is the list of active services this instance most recently advertised, if applicable.
  repeated string services = 3;
  // upgrader_info contains information about the external upgrader and update configuration for this instance, if applicable.
  InstanceUpgraderInfo upgrader_info = 4;
}

// InstanceUpgraderInfo contains information about the external upgrader configuration for an instance.
message InstanceUpgraderInfo {
  // upgrader identifies the external upgrader type (e.g. 'kube', 'unit'). Empty if no upgrader is defined.
  string upgrader = 1;
  // upgrader_version identifies the external upgrader version. Empty if no upgrader is defined.
  string upgrader_version = 2;
  // update_group is the update group for the teleport installation on this instance.
  string update_group = 3;
}
```

#### Performance

The current implementation of `tctl inventory ls` bypasses the cache entirely and reads the instances directly from the backend.
This was done intentionally because caching instances in the traditional way would nearly double the number of backend reads done
on startup while populating the cache, with insufficient read capacity this could easily lead to issues like degraded performance
throughout the entire cluster, or possibly even break the cluster entirely due to a self-propagating loop of errors where
the cache never gets healthy.

Instances still need to be cached if exposed to the Web UI to maintain performance and prevent constant backend reads, so to
prevent the aforementioned issues, instances will be cached in a separate "lazy" cache with rate-limited reads that is
initialized after the primary cache has. This `InstancesCache` will wait for the primary cache to be ready and healthy, and only
then begin populating itself with the instances from the backend. Its reads from the backend will be rate-limited variably,
based on the size of the cluster which will be derived from the size of the primary cache. Larger clusters (which can be assumed
to have more bandwidth) will have more lenient rate-limits in order to prevent it from taking too long to populate.

Aggregate counters such as the total number of up-to-date agents, total number of each service, etc. will be their own separate
fields in the cache and tallied up during initialization, and updated as needed during create/edit/delete operations.

The cache will hook into the backend events watcher and watch for `Instances` events and update the cache accordingly whenever
an update is detected (such as an instance no longer being connected). This cache will live in auth and the data will be exposed to
the proxy via a new RPC service with a `ListInstances()` method which will build the page of instances by streaming from the
instances cache and bot instances cache (which is already implemented). To achieve this, the `MergeStreams` utility will be used
to iterate through both caches simultaneously and use a compare function that compares the names of the instances to ensure that
the returned page is based on alphabetical order. The `next_key` will consist of the next keys in each cache separated by a comma,
eg. `"server3,bot2"`, similar to the existing implementation for dual notifications streaming.

Any requests made for instances will be rejected until the `InstancesCache` is initialized and healthy.

#### UX

![](assets/0229-agent-inventory-mockup.png)
\*The above mockup is not finalized and may differ slightly from the implemented feature.

The new `Agent Inventory` page will be available in the Web UI under the `Zero Trust Access` navigation section.

The top of the page will contain aggregate metrics including the total number of up-to-date and out-of-date agents, number of
agents using externally managed upgrades and their upgraders, and total active services.

The list of instances will be a paginated list with infinite scroll support, and contain columns for hostname,
version, services, and external upgrader (if any). A search bar will be available to use in either basic mode (default),
which searches by hostname, or advanced mode using predicate language to perform queries. The `versions` filter controls will
merely populate the advanced search bar with a predicate query to filter for the desired range of version(s). Filters for
services, upgrader, updater group, and types will be dropdowns containing checkbox lists that allow the user to select one
or more options. Where applicable, the predicate language query functions will be kept consistent with those used in the Bot Instances
dashboard.

Bot instances in the list will also contain a deep link to their dedicated bot instance page in the Bot Instances dashboard (RFD 0222),
this will work by building the URL to the Bot Instances dashboard with a predicate language advacned search query that filters solely
for the specific bot instance's name.

If the page is loaded while the `InstancesCache` is still being initialized, the page will be empty and only show a banner message
with the text "The agent inventory is not yet ready to be displayed, please check back in a few minutes." Users who don't
have `list` or `read` permissions for the `instance` resource kind will see a banner informing them that they need permissions
for `instance.list` and `instance.list`. If a user has `list` and `read` permissions for the `instance` resource, but not
for `bot_instance`, the list will only show agents and not bot instances and the type filter control will be disabled, additionally,
a disclaimer at the top of the page will inform the user of this and why.

#### Security

The proposed changes don't introduce any new potential vulnerabilities, and cluster administrators should be sure to only
allow intended users to have `read` and `list` permissions for `instance` and `bot_instance` resources.
