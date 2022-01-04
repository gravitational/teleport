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

There will be filters that will be applied in order of precedence: RBAC > label + search > sort. RBAC will not be discussed since this check existed before. 

We will need to account for the query language used for other client tools. Currently, only `tsh ls` for listing nodes has support for filter. The language used for `tsh` will need to be deprecated to support for other filters like `search values` and to keep the user query language the same throughout the web UI and CLI (discussed later).

Listing of current pagination + filter support. Methods with `ListXXX` supports pagination and filter, while `GetXXX` does not (it retrieves entire list).

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

### Filter: Query Language

There will be three types of query language:
1. **User query language**: language the user uses to convey a query, applicable to `UI search bar` and `CLI tools`
1. **URL query language**: just a slightly modified version of the user query language to make it URL friendly and bookmarkable (mainly replacing spaces with plus symbol)
1. **Server side query language**: `vulcand/predicate` verbs like `contains` and `equals` and logical operators: `&&`, `||`, and `!`. Paranthesis can be used for grouping statements. The `user/url query language` will be converted to this language in the server.

**Allowed characters in the query param**

According to [rfc 3986](https://datatracker.ietf.org/doc/html/rfc3986#section-3.4), query parameters are allowed to have the following characters unencoded in the URL. Delimiters were chosen following these rules:

| Category     | Description                                                      |
|--------------|------------------------------------------------------------------|
| `query`      | *( pchar / "/" / "?" )                                           |
| `pchar`      | unreserved / pct-encoded / sub-delims / ":" / "@"                |
| `sub-delims` | "!" / "$" / "&" / "'" / "(" / ")"  / "*" / "+" / "," / ";" / "=" |
| `unreserved` | ALPHA / DIGIT / "-" / "." / "_" / "~"                            |

<br/>

**Proposed User/URL Query language (inspired by github)**

| Query Params (URL) | Description                                                                                   |
|--------------------|-----------------------------------------------------------------------------------------------|
| `limit`            | Maximum number of rows to return on each fetch.                                               |
| `startKey`         | Resume row search from the last row received. Empty string means start search from beginning. |
| `q`                | Start of filter query.                                                                        |

<br/>

| Filter Identifier | Description                                                       |
|-------------------|-------------------------------------------------------------------|
| `l=<key>:<value>` | Identifies `<key>:<value>` as a label filter.                     |
| `s=<col>:<dir>`   | Identifies `<col>:<dir>` as a sort filter.                        |
| `<value>`         | Values with no identifiers will be interpreted as a search value. |

<br/>

| Delimiters  | Description                                                                                 |
|-------------|---------------------------------------------------------------------------------------------|
| `:` (colon) | Delimiter in a filter identifier to mean separate values.                                   |
| `+` (plus)  | A filter delimiter `AND` to mean inclusion of filter values. Only used in URL.              |
| ` ` (space) | A filter delimiter `AND` to mean inclusion of filter values. Only used in CLI or search bar |
| `-` (minus) | A filter delimiter `NOT` to mean exclusion of filter value.                                 |
| `,` (comma) | A filter delimiter `OR` to mean match any filter values.                                    |

<br/>

**Examples**

| URL Query Examples                  | Description                                                                    |
|-------------------------------------|--------------------------------------------------------------------------------|
| `?q=l=env:prod+l=country:US`        | rows must contain labels `env:prod` and `country:US`                           |
| `?q=l=os:mac,os:linux+-l=env:prod`  | rows can contain labels `os:mac` or `os:linux` but not `env:prod`              |
| `?q=-l=os:mac+banana`               | rows cannot contain label `os:mac` but contain search value `banana`           |
| `?q=l=os:mac+s=colName:desc+banana` | rows must contain `os:mac` and `banana` and sorted by column `name` descending |

<br/>

| User Query Equivalent                    | Description                                                                    |
|------------------------------------------|--------------------------------------------------------------------------------|
| `tsh ls l=env:prod l=country:US`         | rows must contain labels `env:prod` and `country:US`                           |
| `tsh ls l=os:mac,l=os:linux -l=env:prod` | rows can contain labels `os:mac` or `os:linux` but not `env:prod`              |
| `tsh ls -l=os:mac banana`                | rows cannot contain label `os:mac` but contain search value `banana`           |
| `l=os:mac s=colName:desc banana` *       | rows must contain `os:mac` and `banana` and sorted by column `name` descending |

\* sorting will be disabled for `tsh` for performance reasons (discussed later)

<br/>

| Server Query Equivalent                                                | Description                                                          |
|------------------------------------------------------------------------|----------------------------------------------------------------------|
| `equals(env, "prod") && equals(country, "US")`                         | rows must contain labels `env:prod` and `country:US`                 |
| `(equals(os, "mac") \|\| equals(os, "linux")) && !equals(env, "prod")` | rows can contain labels `os:mac` or `os:linux` but not `env:prod`    |
| `!equals(os, "mac") && (equals(<searchField>, "banana"))`              | rows cannot contain label `os:mac` but contain search value `banana` |

<br/>

**Complex Query Support**

We can support very basic grouping (similar to github):

| Complex Query Examples                     | Description                                                                          |
|--------------------------------------------|--------------------------------------------------------------------------------------|
| `l=env:prod,l=env:dev l=os:mac,os:windows` | rows must contain labels `env:prod` or `env:dev` AND labels `os:mac` or `os:windows` |

If we want to support flexible grouping, we could define another filter identifier `g=(<values>)` meaning whatever is contained in this identifier should be grouped, contrived example: `-l=env:dev,g=(l=os:mac l=os:windows)` which is saying `!a || (b && c)`

<br/>

**Complete URL Example:**

```
https://cloud.dev/v1/webapi/sites/clusterName/nodes?limit=10&startKey=abc&q=l=os:windows+-l=env:prod+s=colName:asc+foo+bar

// Parsed in proxy for the following data, which will be sent to auth server:
- Limit: 10 per page
- StartKey: abc
- Sort: {col: `colName`, dir: `asc`}
- Query: equals(os, "windows") && !equals(env, "prod") && (contains(<searcheableField>, "foo") && contains(<searcheableField>, "bar"))
```

### Filter: Sort

Sorting can be accomplished by retrieving the entire list of resources and then applying filtering/pagination against it, but this works against how pagination + filtering currently works which is:

 - retrieve a subset from list (page)
 - apply matchers against this page
 - repeat until we reach the desired page limit

This technique does not support sorting, but provides faster performance and is used for [tsh](https://github.com/gravitational/teleport/blob/master/api/client/client.go#L1554) with nodes, where an entire list of nodes is retrieved upfront by getting chunks on a loop until `startKey` returns empty.

The web UI will not request for the entire list of resources upfront, but will provide a user with a `fetch more` button if a user desires to see the next page if any.

We can branch off into two functions with current `ListResources` based on if sorting was requested. Sorting will be disabled for `tsh`, so that `tsh` performance will not be affected:

  - `listResources` (keeps current behavior)
  - `listResourcesWithSort`

`listResourcesWithSort` will:

1. retrieve the entire list of resources ie: `GetNodes` (operating on cache)
1. apply filters (rbac > label + search)
1. sort
1. paginate

Similarly to how we did on the web UI, we have to decide which fields of a resource will be sortable. There will be a sort data type that stores the sort column (field const of a resource) and the sort direction (asc or desc). Custom sort function will be used for each resource type. The column name will be defined as const strings that will be used by both client and server side:

**Example:**

| Server Columns       | Const String  |
|----------------------|---------------|
| Server.Spec.Hostname | "colHostname" |
| Server.Spec.Addr     | "colAddr"     |

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

<br/>

### Filter: Label + Search

Currently, only node resource has the capability to be matched against labels (other resources are [pending](https://github.com/gravitational/teleport/pull/9096#issuecomment-989291135)). The matching needs to be extended to also support filtering by search values and support for `NOT` and `OR` label matching.

In order to expand filtering to support NOT and OR, we can use the `vulcand/predicate` library.

Assuming the query string is in the form the `vulcand/predicate` library expects:
1. Create a custom predicate parser that on parse `query string` ouputs an expression tree (similar to [newParserForIdentifierSubcondition](https://github.com/gravitational/teleport/blob/master/lib/services/parser.go#L416))
1. Feed this expression tree to [ToFieldsCondition](https://github.com/gravitational/teleport/blob/master/lib/utils/fields.go#L100) that returns boolean function to apply on fields.
1. For each row, create a `Fields` map that contains all the field values we want to test against the conditions created from previous step. [(example of a similar usage done in teleport)](https://github.com/gravitational/teleport/blob/master/lib/utils/fields_test.go#L52)

**Example:**

Given these searcheable columns for an app (same as sort fields):

| App Server Columns       | Const String     |
|--------------------------|------------------|
| App.Metadata.Name        | "colName"        |
| App.Metadata.Description | "colDescription" |
| App.Spec.PublicAddr      | "colPublicAddr"  |

Given a query string, build an expression tree to create field conditions:
```go
// row must contain labels `env:prod` and `os:mac` and search value `searchValue`
equals(env, "prod") && equals(os, "mac") && (contains(colName, "searchValue") || contains(colDescription, "searchValue") || contains(colPublicAddr, "searchValue"))
```

For each row, construct a `map[string]string` that contains all fields and values we want to test against our field conditions:
```go
Fields{
  // Static Labels, already in map[string]string format
  ...App.Metadata.labels,
  // Dynamic labels, already in map[string]string format
  ...App.Spec.CmdLabels
  // Searcheable fields
  "colName": App.Metadata.Name,
  "colDescription": App.Metadata.Description,
  "colPublicAddr": App.Spec.PublicAddr
}
```

**Some things to consider:**

Sometimes the UI will format a field to be displayed differently ie: if a server does not have a addr, we display `tunnel` to the user, a user may search for `tunnel`, expecting to get all tunnels. Or for app types, we display `cloud sql` instead of the actual value `gcp`.

I'm not entirely sure how to handle this, other than to display actual values to user (tunnel column and display `gcp` instead of `cloud sql`). Or if filtering by tunnels is important, we can make `isTunnel` to be a type of filter a user can specify, and if specified returns servers that uses tunnels.

<br/>

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

<br/>

## Web UI Changes

### Pagination

Paginating and fetching for more rows will behave the same as how audit logs are currently working.

### Sorting

Sort buttons on table columns will stay the same.

### Label Filtering

There will be a button `Select Filter` that once clicked, makes a call to fetch the list of labels (if not already fetched), and then renders a drop down menu of the list of filters (paginated if the list exceeds 100).

There are some edge cases that neesd to be considered for this filter:

- disallow user selecting more than one type of label for `AND` operator. Label keys are unique, and can't appear twice in a row, so the following won't make sense: `env:prod && env:staging`

### Search

The behavior of the search input box will be different in that the user must be intentional and press the `enter` key to make a request (similar to github search behavior). 

We should also place an `x` button at the end of the search bar where if clicked, clears the search bar and updates URL query.

In future work, this search bar should also allow users to type user query language (similar to github). 

### URL 

URL query param will be updated per user changes filter, which will trigger a fetch.

**Note**

We need to account for unexpected query changes when user's login with SSO. The redirect URL will get parsed in the back which will convert unencoded `+` to blank space (other delimiters should also be tested).

<br/>

## Phases of Work

- Phase 1: 
  - pagination
  - filtering by AND labels, search values, and sort
  - label drop down and allow click on labels from table rows
  - search bar will be soley used for search values in this phase
- Phase 2: 
  - add support for OR and NOT labels
  - extend our search bar to allow users to type query
