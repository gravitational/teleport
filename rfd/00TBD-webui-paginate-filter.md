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
- `!`: Exclamation point before a label acts as a `NOT` operator
- `=` or `==`: Both are used between a label key and value and can be used interchangeably

Additionally, an empty right-hand side of the `equals` char means `any values`.

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

### Filter: Search

The fields we allow for searching should be mostly the same as sorting. Search values will be stored as simple list of strings that will be iterated through resource fields to look for match ignoring order. If a user wants to search a phrase, they must supply the phrase within paranthesis.

| Search values | Interpretation     |
|---------------|--------------------|
| foo bar       | ["foo", "bar"]     |
| "foo bar"     | ["foo bar"]        |
| "foo bar" baz | ["foo bar", "baz"] |


The below are the current searchable fields in the web ui, minus labels in which the label filtering should be used instead:

- `Server Fields`: [hostname, addr, tunnel]
- `App Fields`: [name, publicAddr, description]
- `DB Fields`: [name, description, protocol, type]
- `Kube Fields`: [name]
- `Desktop Fields`: [name, address]

**Some things to consider:**

Sometimes the UI will format a field to be displayed differently ie: if a server does not have a addr, we display `tunnel` to the user, a user may search for `tunnel`, expecting to get all tunnels. Or for app types, we display `cloud sql` instead of `gcp`. To handle this, we could extend search functionality for these special cases to check for these values.

<br/>

### List of Unique Labels

A user needs to be able to see a list of available labels as a dropdown, and so we need to create a new rpc and a new web api endpoint to retrieve them.

```
rpc GetResourceLabels(resourceType string) returns []Labels

GET /webabpi/sites/:sites/<nodes|apps|databases|kubernetes>/labels
```

The new rpc will return a list of unique labels by going through a list of resources that is first ran through rbac checks (so users don't see labels to a resource they don't have access to).

There is an edge case where each resource in a list, can have unique labels which can result in a large list, ie. with 30k items in list, one unique label will result in 30k+ labels, two unique labels will result in 60k+ labels and so on. I don't think listing one time labels are particularly useful to the user. We can only return labels that have at least 2 occurences by keeping track of how often labels appear as we process the list to dramatically cut down the length.

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

#### Bookmarkable URL 

The URL will be made bookmarkable and contain

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

| Query Params | Description                |
|--------------|----------------------------|
| `labels`     | Start of label query.      |
| `sort`       | Sort column and direction. |
| `search`     | Search values.             |

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
// which will then be sent to proxy server as POST request (with limit and startKey):
- Sort: {col: `hostname`, dir: `desc`}
- Search: ["foo", "bar"]
- LabelExpr: Expr[{Key: "os", Values: ["mac", "linux"], Op: In},   {Key: "env", Values: ["prod"], Op: NotIn}]
```

**Things to consider**

- We need to account for unexpected query changes when user's login with SSO. The redirect URL will get parsed in the back which will convert (as an example) unencoded `+` to blank space, so all unencoded delimiters should be tested.

- Since teleport does not put restrictions on label characters, the URL delimiters and label key and value can clash. To prevent this, we will encode labels. (`tsh` handles this by using quotation marks ie: if label contains a equal sign then `tsh ls "key=has=equal=sign"=foo`)

<br/>

#### Search

The behavior of the search input box will be different in that the user must be intentional and press the `enter` key to make a request (similar to github search behavior). 

We should also place an `x` button at the end of the search bar where if clicked, clears the search bar and updates URL query.

In future work, this search bar should also be extended to allow users to type the same label query as done in the CLI.

#### Label Filtering

There will be a button `Select Filter` that once clicked, makes a call to fetch the list of labels (if not already fetched), and then renders a drop down menu of the list of filters (paginated if the list exceeds 100).

In future work, this drop down will be extended to allow selecting sets, and `NOT`ing a label through use of shortcut keys (as done in github).

#### Pagination

Paginating and fetching for more rows will behave the same as how audit logs are currently working.

#### Sorting

Sort buttons on table columns will stay the same, but on click will update the query param with sort field, which will trigger a fetch.

## Phases of Work

- Phase 1: will focus on bringing server side pagination and filtering to nodes only in the web UI to match `tsh`.
  - filtering by labels (only `AND`), search values, and sort
  - label drop down menu and allow click on labels from table rows
  - search bar will be soley used for search values in this phase
- Phase 2: 
  - add support for label value set and `NOT` operator
  - extend web ui search bar to allow users to type label query
- Phase 3:
  - bring pagination + filter support to rest of resources for tsh, tctl and web ui
