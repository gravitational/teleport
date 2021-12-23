---
authors: Lisa Kim (lisa@goteleport.com)
state: draft
---

# RFD TBD - WebUI server-side paginating and filtering

## What

Provide server-side pagination and filtering capabilities for the web UI for select resources: nodes, apps, dbs, and kubes.

## Why

Currently, the web api calls upon non paginated endpoints that retrieves an entire list of resources which for higher ranges ~20k+, results in a ~30+ second load time for the UI. With pagination support, we can control the limit of resources to fetch, increasing load speed, and user experience. Filtering will then need to be done on the server-side to apply it to the entire list of resources.

## Details

### Pagination

There is already pagination support for resources: nodes, apps, dbs. Kubes is [pending](https://github.com/gravitational/teleport/pull/9096#issuecomment-989291135)

### Filter

There will be filters that will be applied to each row in order of precedence: RBAC > sort > label > > search. RBAC will not be discuessed since this existed before. 

### Filter: Sort

Sorting will neeed to be implemented. There will be a sort data type that stores the sort column (field name of a resource) and the sort direction (asc or desc). Custom sort function will be used for each resource type.

The column name will be defined as const strings that will be used by both client and server side.

**Example:**

| Server Columns       | Const String  |
|----------------------|---------------|
| Server.Spec.Hostname | "colHostname" |
| Server.Spec.Addr     | "colAddr"  |

<br />

| App Server Columns       | Const String     |
|--------------------------|------------------|
| App.Metadata.Name        | "colName"        |
| App.Metadata.Description | "colDescription" |
| App.Spec.PublicAddr      | "colPublicAddr"  |

<br />

| Db Server Columns         | Const String     |
|---------------------------|------------------|
| DB.Metadata.Name          | "colName"        |
| DB.Metadata.Description   | "colDescription" |
| DB.Spec.(AWS,Azure...)... | "colType"        |

<br />

| Kube Cluster Columns | Const String |
|----------------------|--------------|
| KubeCluster.Name     | "colName"    |


### Filter: Label Matching

Currently, only node resource has the capability to be matched against labels (other resources are [pending](https://github.com/gravitational/teleport/pull/9096#issuecomment-989291135)), and it only supports inclusive match. This is okay for the first phase, but we would eventually need to support for `exclude (NOT)` and `loose (OR)` label matching.

| Label Map (in order of precedence) | Description                                            |
|------------------------------------|--------------------------------------------------------|
| excludeLabels map[string]string    | excludes rows with these labels (`NOT`)                |
| includeLabels map[string]string    | includes rows that matches all these labels (`AND`)    |
| looseLabels   map[string]string    | includes rows that matches any of these labels (`OR`) |


### Filter: Search

Matching against search values needs to be implemented.

Currently, the client side searches by one string `phrase`, ie: if a search value was `foo bar`, it will match against field strings containing the string `foo bar`. I propose we do what github does which is to search by a list of `keywords`, ie: if a search value was `foo bar`, the search will match against strings containing `foo` and `bar`.

Similarly to how we did on the client side, we have to decide which fields of a resource will be searchable which will be different for each type of resource, so each resource type will use its own search function.

- `Server Searcheable Fields`: [hostname, addr, tunnel]
- `App Searcheable Fields`: [name, publicAddr, description]
- `DB Searcheable Fields`: [name, description, protocol, type]
- `Kube Searcheable Fields`: [name]

**Some things to consider:**

Sometimes the UI will format a field to be displayed differently ie: if a server does not have a addr, we display `tunnel` to the user, a user may search for `tunnel`, expecting to get all tunnels. Or for app types, we display `cloud sql` instead of `gcp`. To handle this, we could extend search functionality for these special cases to check for these values.


### List of Unique Labels

A user needs to be able to see a list of available labels as a dropdown, and so we need to create a new rpc and a new web api endpoint to retrieve them.

```
rpc GetResourceLabels(resourceType string) returns []Labels

GET /webabpi/sites/:sites/<nodes|apps|databases|kubernetes>/labels
```

The new rpc will return a list of unique labels by going through a list of resources that is first ran through rbac checks (so users don't see labels to a resource they don't have access to).

There is an edge case where each resource in a list, can have unique labels which can result in a large list, ie. with 30k items in list, one unique label will result in 30k+ labels, two unique labels will result in 60k+ labels and so on. I don't think listing one time labels are particularly useful to the user. We can only return labels that have at least 2 occurences by keeping track of how often labels appear as we process the list to dramatically cut down the length.

There are two types of labels: `static` and `dynamic` ie:

```
ssh_service:
  enabled: yes

  # static
  labels:
    env: dev

  # dynamic
  commands:
  - name: uptime
    command: ["uptime", "-p"]
    period: 1m0s
```

Some `dynamic` labels are time sensitive, as shown in the example. Such `dyanmic` labels with non zero `duration` will not be part of the list.


### Query Parameter

Query parameters will be appended to our existing web api endpoints:

```
GET /webabpi/sites/:sites/<nodes|apps|databases|kubernetes>?...
```

The query parameters will be parsed server side for the following values: 

```go
// Only showing relevant and new fields to the request struct.
type ListResourcesRequest struct {
	Limit int32
	StartKey string

  // New fields:
	SearchValues []string
	Labels map[string]string
	ExcludeLabels map[string]string
	LooseLabels map[string]string
	Sort { col, dir string }
}
```

These web api endpoints will return a new response:
```go
type Response struct {
	List []UIResourceType
	StartKey string
	SearchValues []string
	Labels []{ name, value string }
	ExcludeLabels []{ name, value string }
	LooseLabels []{ name, value string }
	Sort { col, dir string }
}
```

<br/>

| Query Params | Description                                                                                   |
|--------------|-----------------------------------------------------------------------------------------------|
| `limit`      | Maximum number of rows to return on each fetch.                                               |
| `startKey`   | Resume row search from the last row received. Empty string means start search from beginning. |
| `query`      | Start of filter query.                                                                        |

<br/>

| Filter Identifier | Description                                                      |
|-------------------|------------------------------------------------------------------|
| `l=<key>:<value>` | Identifies `<key>:<value>` as a label filter.                    |
| `s=<col>:<dir>`   | Identifies `<col>:<dir>` as a sort filter.                       |
| `<value>`         | Values with no identifiers will be interpreted as a search value.|

<br/>

| Delimiters  | Description                                                                               |
|-------------|-------------------------------------------------------------------------------------------|
| `:` (colon) | Delimiter in a filter identifier to mean separate values.                                 |
| `+` (plus)  | A filter delimiter `AND` to mean inclusion of filter values.                              |
| `-` (minus) | A filter delimiter `NOT` to mean exclusion of filter values. Only applicable with labels. |
| `.` (comma) | A filter delimiter `OR` to mean include any filter values. Only applicable with labels.   |

<br/>

| Query Examples                         | Description                                                                    |
|----------------------------------------|--------------------------------------------------------------------------------|
| `query=l=env:prod+l=country:US`        | rows must contain labels `env:prod` and `country:US`                           |
| `query=l=os:mac,os:linux+-l=env:prod`  | rows can contain labels `os:mac` or `os:linux` but not `env:prod`              |
| `query=-l=os:mac+banana`               | rows cannot contain label `os:mac` but contain search value `banana`           |
| `query=l=os:mac+s=colName:desc+banana` | rows must contain `os:mac` and `banana` and sorted by column `name` descending |

<br/>

**Complete Example:**

```
https://cloud.dev/v1/webapi/sites/clusterName/nodes?limit=10&startKey=abc&query=l=os:windows+-l=env:prod+s=colName:asc+foo+bar

- Limit: 10 per page
- StartKey: abc
- SearchValues: `foo`, `bar`
- Labels: `os:window`, `dev:prod` 
- ExcludeLabels: `env:prod`
- Sort: {col: `name`, dir: `asc`}
```

## Allowed characters in the query param

According to [rfc 3986](https://datatracker.ietf.org/doc/html/rfc3986#section-3.4), query parameters are allowed to have the following characters unencoded. The characters we've chosen as our delimiters are all valid.

| Category     | Description                                                      |
|--------------|------------------------------------------------------------------|
| `query`      | *( pchar / "/" / "?" )                                           |
| `pchar`      | unreserved / pct-encoded / sub-delims / ":" / "@"                |
| `sub-delims` | "!" / "$" / "&" / "'" / "(" / ")"  / "*" / "+" / "," / ";" / "=" |
| `unreserved` | ALPHA / DIGIT / "-" / "." / "_" / "~"                            |

**Things to consider when parsing query**

When golang parses a query, it will replace unencoded characters like `+` into a space.

## Web UI Changes

### Pagination

Paginating and fetching for more rows will behave the same as how audit logs are currently working.

### Sorting

Sort buttons on table columns will stay the same.

### Label Filtering

There will be a button `Select Filter` that once clicked, makes a call to fetch the list of labels (if not already fetched), and then renders a drop down menu of the list of filters (paginated if the list exceeds 100).

### Search

The behavior of the search input box will be different in that the user must be intentional and press the `enter` key to make a request (similar to github search behavior). 

We should also place an `x` button at the end of the search bar where if clicked, clears the search bar and fetches new rows without the search values.

## Phases of Work

- Phase 1: will only implement inclusive labels
- Phase 2: will implement excluding or loosely matching labels
