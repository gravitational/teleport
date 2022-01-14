---
authors: Lisa Kim (lisa@goteleport.com)
state: draft
---

# RFD TBD - WebUI server-side paginating and filtering

## What

Provide server-side pagination and filtering capabilities for the web UI for select resources: nodes, apps, dbs, kubes, and desktops.

## Why

Currently, the web api calls upon non paginated endpoints that retrieves an entire list of resources which for higher ranges ~20k+, results in a ~30+ second load time for the UI. With pagination support, we can control the limit of resources to fetch, increasing load speed, and user experience. Filtering will then need to be done on the server-side to apply it to the entire list of resources.

## Details

### Pagination

There is already pagination support for resources: nodes, apps, dbs. Kubes is [pending](https://github.com/gravitational/teleport/pull/9096#issuecomment-989291135)

### Filter

There will be filters that will be applied in order of precedence: RBAC > label > search > sort. RBAC will not be discussed since this check existed before. 

### Filter: User Label Query Language

The query language for labels will be modeled after [kubernetes labels](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/). Kubernetes uses `set based` filtering which matches a label `key` against a set of `values` as shown in examples below.

The language will be the same across the web ui and cli tools.

Supported operators:
- `,`: Comma between labels acts as an `AND` operator (same interpretation as current `tsh ls <label1>,<label2>`)
- `|`: Pipe between label `values` denotes a set and acts as a `OR` operator
- `!=`: Acts as a `NOT` equal operator
- `=` or `==`: Both are used between a label key and value and can be used interchangeably

Additionally, an asterik on the right-hand side of the `equals` char means `any values`.

| Format                        | Operator | Example          | Description                                                      |
|-------------------------------|----------|------------------|------------------------------------------------------------------|
| `key=value`                   | in       | `env=prod`       | rows with label `env=prod`                                       |
| `key==value`                  | in       | `env==prod`      | same as prior, just using `==` instead of `=`, both will be okay |
| `key=value1\|value2\|value*`  | in       | `env=prod\|dev`  | rows with labels `env=prod` or `env=dev`                         |
| `key==value1\|value2\|value*` | in       | `env==prod\|dev` | same as prior, just using `==` instead of `=`, both will be okay |
| `key!=value`                  | notin    | `env!=prod`      | rows without label `env=prod`                                    |
| `key!=value1\|value2\|value*` | notin    | `env!=prod\|dev` | rows without labels `env=prod` or `env=dev`                      |

<br/>

| Example Combos                    | Description                                                              |
|-----------------------------------|--------------------------------------------------------------------------|
| `env=prod\|dev,os=mac\|linux`     | rows with labels (`env=prod` or `env=dev`) and (`os=mac` or `os=linux`)  |
| `env=prod\|dev,os=mac,country=us` | rows with labels (`env=prod` or `env=dev`) and `os=mac` and `country=us` |
| `env!=prod,os=mac`                | rows without label `env=prod` and with label `os=mac`                    |
| `foo=*`                           | rows with a label with key `foo`; values unchecked                       |
| `foo!=*`                          | rows without a label with key `foo`; values unchecked                    |
| `foo=*,foo!=bar`                  | rows with a label with key `foo` with any values except `bar`            |

<br/>

In a label query, every key and its set will be translated into a list of expression objects, which will then be iterated and matched against the resource labels:

```go
type Operator int

const (
	In Operator = iota
	NotIn
)

type Expr struct {
	Key    string
	Op     Operator
	Values []string
}
```

**Support for Grouping and OR operator**

Kubernetes does not support grouping (other than the basic shown in examples) and does not support OR operator between different labels ie `env=prod || os=mac`. On research, I could not find any user request or complaints about these features and concluded that it must not be too applicable enough to support.

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

`listResourcesWithSort` will:

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

The fields we allow for searching should be mostly the same as sorting. Search values will be stored as simple list of strings that will be iterated through select resource fields to look for a fuzzy match (contains), ignoring case and order. If a user wants to search a phrase, they must supply the phrase within paranthesis.

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

Tables below are listing of current pagination + filter support. Methods with `ListXXX` supports pagination and filter, while `GetXXX` does not (it retrieves entire list).

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

Currently, only `tsh ls` for listing nodes has support for filter. The command will be extended for advanced label querying and searching (sorting will be disabled as discussed earlier for performance reasons).

**Backwards Compatibility**

Because we keep the original meaning of `comma` the same, extending will not be a problem. Fallbacks will be used when we switch from `ListNodes` and `GetXXX` to `ListResources`.

**Proposed Flags**

`tsh` will keep the current behavior where it can define labels without any flags, in addition:

| Long flag | Description   | Example                                 |
|-----------|---------------|-----------------------------------------|
| --labels  | label query   | `tsh ls --labels=env=prod\|dev,os=mac`  |
| --search  | search values | `tsh ls --search=foo,bar,"some phrase"` |

<br/>

### Client: Web UI

#### Search Bar

Label querying and searching will be done in the search bar. There will be two states of our search bar:

1) `Simple Search` that does fuzzy `AND` searching only. Users can also look for words in a label ie: if we have a label `env=foo` user can search for words `env foo`. Anything more complex than this, the user will need to use our advanced search.
2) `Advanced Search` that allows users to perform the same query as done in the CLI, using the same long flags `--labels=<label query>` and `--search=<search words>`.

Similar to how it is done in the VS Code's search box, there will be two clickable buttons that alternates between these two states.

#### Clickable Labels from Table's

Label's on table for select resources will be clickable, which updates the URL param, and triggers a re-fetch.

Depending on what state the search bar is currently on, clicking on labels will have different behavior:


| Label Clicked           | Search Bar State | Displayed on Search Bar    | What it means                                                          |
|-------------------------|------------------|----------------------------|------------------------------------------------------------------------|
| `env=prod`              | simple           | `env prod`                 | resources with fields containing string `env` and `prod`               |
| `env=prod`              | advanced         | `--labels=env=prod`        | resources with labels `env=prod`                                       |
| `env=prod` and `os=mac` | simple           | `env prod os mac`          | resources with fields containing string `env`, `prod`, `os`, and `mac` |
| `env=prod` and `os=mac` | advanced         | `--labels=env=prod,os=mac` | resources with labels `env=prod` and `os=mac`                          |

<br/>

#### Bookmarkable URL 

The URL will be made bookmarkable and contains the search and label queries.

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

| Query Params | Description    |
|--------------|----------------|
| `labels`     | Labels         |
| `search`     | Search values. |

**Note:** The existence of `labels` query param will indicate that the user has used the advanced search.

<br/>

| URL Delimiters | Description                                                          |
|----------------|----------------------------------------------------------------------|
| `=` (equal)    | Delimiter to mean separate values, `key=value` or `column=direction` |
| `-` (minus)    | Acts as a `NOT` operator specifically used to mean excluding a label |
| `+` (plus)     | Acts as a `AND` operator between values.                             |
| `,` (comma)    | Denotes a label `values` as a set, and acts as a `OR` operator.      |

<br/>

**Examples**

| Bookmarkable Query Examples      | Description                                                       |
|----------------------------------|-------------------------------------------------------------------|
| `?labels=env=prod+country=US`    | rows must contain labels `env:prod` and `country:US`              |
| `?labels=os=mac,linux+-env=prod` | rows can contain labels `os:mac` or `os:linux` but not `env:prod` |
| `?search=foo+bar`                | rows contain search values `foo` and `bar`                        |
| `?sort=hostname=desc`            | rows sorted by column `hostname`in `descending` order             |

<br/>

**Complete URL Example:**

```
https://cloud.dev/v1/webapi/sites/clusterName/nodes?labels=os=mac,linux+-env=prod&sort=hostname=desc&search=foo+bar

// Parsed in web client for the following data,
// which will then be sent to proxy server as 
// POST request (with limit and startKey) to: `/webapi/sites/:site/resources/:resourceType`
// resourceType can be: app, node, db, kube, windesktop

- Sort: {col: `hostname`, dir: `desc`}
- Search: ["foo", "bar"]
- LabelExpr: Expr[{Key: "os", Values: ["mac", "linux"], Op: In},   {Key: "env", Values: ["prod"], Op: NotIn}]
```

**Things to consider**

- We need to account for unexpected query changes when user's login with SSO. The redirect URL will get parsed in the back which will convert (as an example) unencoded `+` to blank space, so all unencoded delimiters should be tested.

- Since teleport does not put restrictions on label characters, the URL delimiters and label key and value can clash. To prevent this, we will encode labels. (`tsh` handles this by using quotation marks ie: if label contains a equal sign then `tsh ls "key=has=equal=sign"=foo`)

<br/>

#### Pagination

Paginating and fetching for more rows will behave the same as how audit logs are currently working.

#### Sorting

Sort buttons on table columns will stay the same, but on click will update the query param with sort field, which will trigger a fetch.

## Phases of Work

- Phase 1: will focus on bringing server side pagination and filtering to nodes only in the web UI to match `tsh`.
  - create two state search bar for UI:
	 - fuzzy search
	 - advanced search that supports "AND"ing labels (and search)
  - clickable labels from table
- Phase 2: 
  - add full label query support to our advanced search bar
  - bring pagination + filter support to rest of resources for tsh, tctl and web ui

