---
authors: Lisa Kim (lisa@goteleport.com)
state: draft
---

# RFD 47 - WebUI server-side paginating, searching, and filtering for servers.

## What

Provide server-side pagination and filtering capabilities for the web UI for select resources: nodes, apps, dbs, and kubes.

## Why

Currently, the web api calls upon non paginated endpoints that retrieves an entire list of servers which for higher ranges ~20k+, results in a ~30+ second load time for the UI. With pagination support, we can control the limit of servers to fetch, increasing load speed, and user experience. Filtering will then need to be done on the server-side to apply it to the entire list of servers.

## Details

### Pagination

There is already pagination support for resources: nodes, apps, dbs. Kubes is [pending](https://github.com/gravitational/teleport/pull/9096#issuecomment-989291135)

### Filter

There will be filters that will be applied to each row in order of precedence: RBAC, sort, label, and search.

### Filter: Sort

Sorting will neeed to be implemented. There will be a sort data type that stores the sort column (field name of a resource) and the sort direction (asc or desc). Custom sort function will be used for each resource type.

The column name will be defined as const strings that will be used by both client and server side.

| Server Columns       | Const String  |
|----------------------|---------------|
| Server.Spec.Hostname | "colHostname" |
| Server.Spec.Addr     | "colAddr"  |


| App Server Columns       | Const String     |
|--------------------------|------------------|
| App.Metadata.Name        | "colName"        |
| App.Metadata.Description | "colDescription" |
| App.Spec.PublicAddr      | "colPublicAddr"  |


| Db Server Columns         | Const String     |
|---------------------------|------------------|
| DB.Metadata.Name          | "colName"        |
| DB.Metadata.Description   | "colDescription" |
| DB.Spec.(AWS,Azure...)... | "colType"        |


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

Currently, the client side searches by one string `phrase`, ie: if a search value was `foo bar`, it will match against strings containing the string `foo bar`. I propose we do what github does which is to search by a list of `keywords`, ie: if a search value was `foo bar`, the search will match against strings containing `foo` and `bar`.

Similarly to how we did on the client side, we have to decide which fields of a resource will be searchable which will be different for each type of resource, so each resource type will use its own search function.

- `Server Searcheable Fields`: [hostname, addr, tunnel]
- `App Searcheable Fields`: [name, publicAddr, description]
- `DB Searcheable Fields`: [name, description, protocol, type]
- `Kube Searcheable Fields`: [name]

**Some things to consider:**

Sometimes the UI will format a field to be displayed differently ie: if a server does not have a addr, we display `tunnel` to the user, a user may search for `tunnel`, expecting to get all tunnels. Or for app types, we display `cloud sql` instead of `gcp`. To handle this, we could extend search functionality for these special cases to check for these values.


### List of Unique Labels

To allow users to filter by resource labels, a new rpc `GetResourceLabels` will need to be created, and then a new web api endpoint to retrieve the list of labels for the UI:

```
GET /webabpi/sites/:sites/<nodes|apps|databases|kubernetes>/labels
```

The new rpc will build and return a map of unique labels by going through a list of resources that was first ran through rbac checks (so users don't see labels to a resource they don't have access to).

TODO: handling of possible huge numbers of unique labels

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

Some `dynamic` labels are time sensitive, as shown in the example. Such `dyanmic` labels with non zero `duration` will not be part of the list. If users really want to filter by time, we may need to make a time filter option where a user enters time manually. 


### Query Parameter

Query parameters will be appended to our existing web api endpoints:
- `/webapi/sites/:site/nodes`
- `/webapi/sites/:site/apps`
- `/webapi/sites/:site/databases`
- `/webapi/sites/:site/kubernetes`

The query will be parsed server side for the following values: 

```go
// Only showing relevant and new fields to the request struct.
type ListResourcesRequest struct {
	Limit int32
	StartKey string

  // New fields.
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

| Query Params | Description                                                                                   |
|--------------|-----------------------------------------------------------------------------------------------|
| `limit`      | Maximum number of rows to return on each fetch.                                               |
| `startKey`   | Resume row search from the last row received. Empty string means start search from beginning. |
| `query`      | Start of filter query.                                                                        |


| Filter Identifier | Description                                                      |
|-------------------|------------------------------------------------------------------|
| `l=<key>:<value>` | Identifies `<key>:<value>` as a label filter.                    |
| `s=<col>:<dir>`   | Identifies `<col>:<dir>` as a sort filter.                       |
| `<value>`         | Values with no identifiers will be interpreted as a search value.|


| Delimiters | Description                                                                               |
|------------|-------------------------------------------------------------------------------------------|
| `:`        | Delimiter in a filter identifier to mean separate values.                                 |
| `+`        | A filter delimiter `AND` to mean inclusion of filter values.                              |
| `-`        | A filter delimiter `NOT` to mean exclusion of filter values. Only applicable with labels. |
| `.`        | A filter delimiter `OR` to mean include any filter values. Only applicable with labels.   |


| Query Examples                         | Description                                                                    |
|----------------------------------------|--------------------------------------------------------------------------------|
| `query=l=env:prod+l=country:US`        | rows must contain labels `dev:prod` and `country:US`                           |
| `query=l=os:mac,os:linux+-l=env:prod`  | rows can contain labels `os:mac` or `os:linux` but not `env:prod`              |
| `query=-l=os:mac+banana`               | rows cannot contain label `os:mac` but contain search value `banana`           |
| `query=l=os:mac+s=colName:desc+banana` | rows must contain `os:mac` and `banana` and sorted by column `name` descending |


**Complete Example Request:**

```
https://cloud.dev/v1/webapi/sites/clusterName/nodes?limit=10&startKey=abc&query=l=os:windows+s=colName:asc+foo+bar

- limit: 10 per page
- startKey: abc
- filter by labels: `os:window` and `dev:prod` 
- filter by search values: `foo` and `bar`
- results should be sorted by column `name` and ascending

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

There will be a button `Select Filter` that once clicked renders a drop down menu of the list of filters (paginated if the list exceeds 100).

### Search

The behavior of the search input box will be different in that the user must be intentional and press the `enter` key to make a request. 

We should also place an `x` button at the end of the search bar where if clicked, clears the search box and fetches new rows without the search values.

