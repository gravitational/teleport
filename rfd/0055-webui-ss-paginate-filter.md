---
authors: Lisa Kim (lisa@goteleport.com)
state: draft
---

# RFD 55 - WebUI server-side paginating and filtering

## What

Provide server-side pagination and filtering capabilities for the web UI for select resources: nodes, apps, dbs, kubes, and desktops.

## Why

Currently, the web api calls upon non paginated endpoints that retrieves an entire list of resources which for higher ranges ~20k+, results in a ~30+ second load time for the UI. With pagination support, we can control the limit of resources to fetch, increasing load speed, and user experience. Filtering will then need to be done on the server-side to apply it to the entire list of resources.

## Details

### Pagination

There is already pagination support for resources: nodes, apps, dbs. Kubes is [pending](https://github.com/gravitational/teleport/pull/9096#issuecomment-989291135)

### Filter

There will be filters that will be applied in order of precedence: RBAC > label > search > sort. RBAC will not be discussed since this check existed before. 

### Filter: Query Language

The query language for both front and backend will use the `vulcand/predicate` language, the same language we use for `where` conditions for [roles](https://goteleport.com/docs/access-controls/guides/impersonation/#dynamic-impersonation).

| Supported Ops | Language     | Example                                                                             |
|---------------|--------------|-------------------------------------------------------------------------------------|
| EQ            | == or equals | `labels.env == "prod"` or `labels["env"] == "prod"` or `equals(labels.env, "prod")` |
| NEQ           | !=           | `labels.env != "prod"`                                                              |
| NOT           | !            | `!equals(labels.env, "prod")`                                                       |
| AND           | &&           | `labels.env == "prod" && labels.os == "mac"`                                        |
| OR            | \|\|         | `labels.env == "dev" \|\| labels.env == "qa"`                                       |

<br/>

We could also create custom function that does something like this:

| Functions                             | Description                                      |
|---------------------------------------|--------------------------------------------------|
| `exists(labels.env)`                  | rows with a label key `env`; values unchecked    |
| `!exists(labels.env)`                 | rows without a label key `env`; values unchecked |
| `search("foo", "bar", "some phrase")` | fuzzy match against select fields                |

<br/>

For common resource fields, we can define shortened fields:

| Short Field    | Actual Field                                            | Example             |
|----------------|---------------------------------------------------------|---------------------|
| labels.\<key\> | resource.metadata.labels + resource.spec.dynamic_labels | labels.env == "dev" |
| name           | resource.metadata.name                                  | name = "jenkins"    |

<br/>

If user wants to match by resource specific fields, the user can specify the json fields:

| Resource Type | Field to use              | Example                                         |
|---------------|---------------------------|-------------------------------------------------|
| Database      | resource.spec.protocol    | resource.spec.protocol == "postgres"            |
| Application   | resource.spec.public_addr | resource.spec.public_addr = "https://cloud.dev" |

<br/>

### Filter: Sort

Sorting can be accomplished by retrieving the entire list of resources and then applying filtering/pagination against it, but this works against how pagination + filtering currently works which is:

 - retrieve a subset from list (page)
 - apply matchers against this page
 - repeat until we reach the desired page limit

This technique does not support sorting, but provides faster performance and is used for [tsh](https://github.com/gravitational/teleport/blob/master/api/client/client.go#L1554) with nodes, where an entire list of nodes is retrieved upfront by getting chunks on a loop until `startKey` returns empty.

The web UI will not request for the entire list of resources upfront, but will provide a user with a `fetch more` button if a user desires to see the next page if any.

We can branch off into two functions with current `ListResources` based on if sorting was requested (orting will be disabled for `tsh`, so that `tsh` performance will not be affected):

  - `listResources` (keeps current behavior)
  - `listResourcesWithSort`

`listResourcesWithSort` will `fake` pagination by:

1. retrieve the entire list of resources ie: `GetNodes` (operating on cache)
1. apply filters (rbac > label + search)
1. sort
1. paginate

Similarly to how we did on the web UI, we have to decide which fields of a resource will be sortable. There will be a sort data type that stores the sort column (field const of a resource) and the sort direction (asc or desc). Custom sort function will be used for each resource type. The column name will be defined as const strings that will be used by clients and server.

```go
type Direction int

const (
	Asc Direction = iota
	Desc
)

type Sort struct {
	Column    string
	Dir     Direction
}

```

**Current sortable fields in the web ui:**

| Server Columns       | Const String |
|----------------------|--------------|
| Server.Spec.Hostname | "hostname"   |
| Server.Spec.Addr     | "address"    |

<br />

| App Server Columns       | Const String  |
|--------------------------|---------------|
| App.Metadata.Name        | "name"        |
| App.Metadata.Description | "description" |
| App.Spec.PublicAddr      | "address"     |

<br />

| Db Server Columns         | Const String  |
|---------------------------|---------------|
| DB.Metadata.Name          | "name"        |
| DB.Metadata.Description   | "description" |
| DB.Spec.(AWS,Azure...)... | "type"        |

<br />

| Kube Cluster Columns | Const String |
|----------------------|--------------|
| KubeCluster.Name     | "name"       |

<br/>

| Desktops Columns      | Const String |
|-----------------------|--------------|
| Desktop.Metadata.Name | "name"       |
| Desktop.Spec.Addr     | "address"    |

<br/>

### Filter: Fuzzy Search

The fields we allow for searching should be mostly the same as sorting. Search values will be stored as simple list of strings that will be iterated through select resource fields to look for a fuzzy match, ignoring case and order. If a user wants to search a phrase, they must supply the phrase within paranthesis.

| Search Examples | Interpretation     |
|-----------------|--------------------|
| foo bar         | ["foo", "bar"]     |
| "foo bar"       | ["foo bar"]        |
| "foo bar" baz   | ["foo bar", "baz"] |


The below are the current searchable fields in the web ui:

- `Server Fields`: [hostname, addr, tunnel, labels]
- `App Fields`: [name, publicAddr, description, labels]
- `DB Fields`: [name, description, protocol, type, labels]
- `Kube Fields`: [name, labels]
- `Desktop Fields`: [name, address, labels]

**Some things to consider:**

Sometimes the UI will format a field to be displayed differently ie: if a server does not have a addr, we display `tunnel` to the user, a user may search for `tunnel`, expecting to get all tunnels. Or for app types, we display `cloud sql` instead of `gcp`. To handle this, we could make custom search matching for these values ie: if a search value equals `tunnel`, we will also check if `nodeResource.UsesTunnel()`.

<br/>

### Client: tsh and tctl

Tables below are listing of current pagination + filter support. Methods with `ListXXX` supports pagination and filter, while `GetXXX` does not.

| tctl  | Method Used Currently | Current Query Language |
|-------|-----------------------|------------------------|
| nodes | GetNodes              |                        |
| apps  | GetApplicationServers |                        |
| dbs   | GetDatabaseServers    |                        |

<br/>

| tsh   | Method Used Currently | Current Query Language                                                                               |
|-------|-----------------------|------------------------------------------------------------------------------------------------------|
| nodes | ListNodes             | label `key=value` comma delimited ie. `tsh ls env=dev,role=admin`, to mean `env=dev` && `role=admin` |
| apps  | GetApplicationServers |                                                                                                      |
| dbs   | GetDatabaseServers    |                                                                                                      |
| kubes | GetKubernetesClusters |                                                                                                      |

<br/>

**Backwards Compatibility**

Fallbacks will be used when we switch from `ListNodes` and `GetXXX` to `ListResources`.

**Proposed Flags**

`tsh` will keep the current behavior where it can define simple label query without any flags, in addition:

| Long flag | Description                                                  | Example                                                       |
|-----------|--------------------------------------------------------------|---------------------------------------------------------------|
| --query   | resource query that has<br>to be wrapped in single<br>quotes | `tsh ls --query='labels.env == "prod" && labels.os == "mac"'` |
| --search  | fuzzy search                                                 | `tsh ls --search=foo,bar,"some phrase"`                       |

<br/>

### Client: Web UI

#### Search Bar

Resource querying and searching will be done in the search bar. There will be two states of our search bar:

1) `Simple Search` that does fuzzy `AND` searching only. Users can also look for words in a label ie: if we have a label `env=foo` user can search for words `env foo`. Anything more complex than this, the user will need to use our advanced search.
2) `Advanced Search (query)` that allows users to use the predicate language.

Similar to how it is done in the VS Code's search box, there will be two clickable buttons that alternates between these two states.

#### Clickable Labels from Table's

Label's on table for select resources will be clickable, which updates the URL param, and triggers a re-fetch.

Depending on what state the search bar is currently on, clicking on labels will have different behavior:


| Label Clicked             | Search Bar State | Displayed on Search Bar                      |
|---------------------------|------------------|----------------------------------------------|
| `env: prod`               | simple           | `env prod`                                   |
| `env: prod`               | advanced         | `labels.env == "prod"`                       |
| `env: prod` and `os: mac` | simple           | `env prod os mac`                            |
| `env: prod` and `os: mac` | advanced         | `labels.env == "prod" && labels.os == "mac"` |

<br/>

#### Bookmarkable URL 

The URL will be made bookmarkable and contains the search, sort, and query params.

**Allowed characters in the URL query param**

According to [rfc 3986](https://datatracker.ietf.org/doc/html/rfc3986#section-3.4), query parameters are allowed to have the following characters unencoded in the URL. We will keep delimiters unencoded for a cleaner looking url. Characters and delimiters were chosen following these rules:

| Category     | Description                                                      |
|--------------|------------------------------------------------------------------|
| `query`      | *( pchar / "/" / "?" )                                           |
| `pchar`      | unreserved / pct-encoded / sub-delims / ":" / "@"                |
| `sub-delims` | "!" / "$" / "&" / "'" / "(" / ")"  / "*" / "+" / "," / ";" / "=" |
| `unreserved` | ALPHA / DIGIT / "-" / "." / "_" / "~"                            |

<br/>

**Proposed Query Params and Delimiters**

| Query Params | Description                                                          |
|--------------|----------------------------------------------------------------------|
| `query`      | contains resource query in predicate language, entire string encoded |
| `search`     | search values, quotation marks encoded, values separated by `+`      |
| `sort`       | sort values in `<fieldName>:<direction>` format                      |

**Note:** The existence of `query` param will indicate that the user has used the advanced search.

<br/>

**Examples**

| Query                                        | URL                                                                                       | Description                                                |
|----------------------------------------------|-------------------------------------------------------------------------------------------|------------------------------------------------------------|
| `labels.env == "prod" && labels.os == "mac"` | ?query=labels.env%20%3D%3D%20%22prod<br>%22%20%26%26%20labels.os%20%3D%3D<br>%20%22mac%22 | rows with label `env=prod` and `os=mac`                    |
| `env prod "some phrase"`                     | ?search=env+prod+%22some%20phrase%22                                                      | rows contain search values `env`, `prod` and `some phrase` |
| User clicks on sort buttons on table         | ?sort=hostname:desc                                                                       | rows sorted by column `hostname`in `descending` order      |

<br/>

**Complete URL Example:**

```
https://cloud.dev/web/cluster/some-cluster-name/nodes?query=labels.env%20%3D%3D%20%22prod%22%20%26%26%20labels.os%20%3D%3D%20%22mac%22&sort=hostname:desc

// Makes request to an endpoint `/webapi/sites/:site/resources/:resourceType?limit=50&startKey=abc&query=labels.env%20%3D%3D%20%22prod%22%20%26%26%20labels.os%20%3D%3D%20%22mac%22&sort=hostname:desc`
// And extract the following from url query params and sent to auth server:

- Limit: 50
- StartKey: `abc`
- Query: `labels.env == "prod" && labels.os == "mac"` (unencoded)
- Sort: {col: `hostname`, dir: `desc`}
```

**Things to consider**

- We need to account for unexpected query changes when user's login with SSO. The redirect URL will get parsed in the back which will convert (as an example) unencoded `+` to blank space, so all unencoded delimiters should be tested.

<br/>

#### Pagination

Paginating and fetching for more rows will behave the same as how audit logs are currently working.

#### Sorting

Sort buttons on table columns will stay the same, but on click will update the query param with sort field, which will trigger a fetch.

## Phases of Work

- Phase 1: will focus on bringing server side pagination and filtering to nodes only in the web UI to match `tsh`.
  - create two state search bar for UI:
	 - fuzzy search
	 - advanced search
  - clickable labels from table
- Phase 2:
  - bring pagination + filter support to rest of resources for tsh, tctl and web ui

