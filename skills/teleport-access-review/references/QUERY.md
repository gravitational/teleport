# The `access_path` Query Language

`tctl access-review --query` takes a single SQL statement. It is **always** of
the form:

```sql
SELECT * FROM access_path [WHERE <predicate>]
```

- The `FROM` table **must** be `access_path`. Any other table is rejected with a
  `400`: `FROM must be access_path (this endpoint only accepts access_path as
  the entry point)`.
- `SELECT *` is the usual projection; the columns below are for the **`WHERE`**
  clause, which scopes the review.
- A query with no `WHERE` reviews every identity in the graph (page by page).
  This is expensive on a large cluster — scope it.

**Only `WHERE` is supported — no `LIMIT`, `OFFSET`, or `ORDER BY`.** The access
endpoint rejects them:

- `LIMIT` (and `OFFSET`) → `400`: `LIMIT is not supported on the access endpoint`
- `ORDER BY` → `400`: `ORDER BY is not supported`

To cap how many identities come back, use the **`--limit` flag**, not SQL
`LIMIT` — the endpoint paginates by identity itself, so a query-level `LIMIT`
would fight that and is disallowed. There is no result ordering to request;
client-side sort/search the JSON if you need an order.

## Queryable columns

The left column is what you type; each matches a node of a particular kind.
Unknown column names are rejected with a `400`: `column "<name>" not found`.

| Column                | Node matched     | Matches on                                                        | Notes                                                                 |
| --------------------- | ---------------- | ----------------------------------------------------------------- | --------------------------------------------------------------------- |
| `identity`            | identity         | name **or** alias                                                 | Users, bots. Prefer `identity`; the legacy alias `user` is a SQL keyword and must be quoted as `"user"` (bare `user` is a `400`). |
| `resource`            | resource         | name **or** alias                                                 | Servers, databases, apps, k8s, AWS resources, …                       |
| `sub_resource`        | sub_resource     | name **or** alias                                                 | A resource's sub-resource (e.g. a database name within a DB server).  |
| `identity_group`      | identity_group   | name **or** alias                                                 | Access lists, roles, access requests. Array-valued. Legacy: `user_group`. |
| `resource_group`      | resource_group   | name **or** alias                                                 | Array-valued.                                                         |
| `id`                  | any node         | the node's UUID                                                   | Exact match on the `id` shown in output. Useful for an unambiguous handle. |
| `source`              | identity         | the identity's source (e.g. `TELEPORT`, `OKTA`)                   | Case-insensitive. Scope to one identity provider.                     |
| `action` / `action_type` / `kind` | action | action name / sub-kind / action `type`                           | For advanced action-based scoping.                                    |
| `resource_labels`     | resource         | the resource's labels (JSON)                                      | Use JSON operators (below).                                           |
| `standing_privileges` | identity         | the identity's standing-privilege count (number)                  | **Filter-only**: the value is not returned in output and is sparsely populated; verify against a broad query before relying on a threshold. |

## Operators

Allow-listed operators (anything else is a `400`
`unsupported operator in the WHERE clause`):

| Operator                              | Use                                                                 |
| ------------------------------------- | ------------------------------------------------------------------- |
| `=`, `!=`                             | Exact match / negation on a single value.                           |
| `<`, `>`, `<=`, `>=`                  | Numeric comparison (e.g. `standing_privileges > 50`).               |
| `IN (…)`, `NOT IN (…)`                | Match any of a list — the workhorse for reviewing a set of identities or resources. |
| `LIKE` / `ILIKE` (and `NOT …`)        | Wildcard match with `%` / `_`. `ILIKE` is case-insensitive.         |
| JSON: `?`, `?&`, `?\|`, `@>`, `<@`    | Existence / containment against `resource_labels`.                  |
| `AND`, `OR`                           | Combine predicates.                                                 |

**There is no regex operator** (`~`, `~*`, `SIMILAR TO` are not allowed). Use
`ILIKE` for fuzzy matching.

## Name resolution — the most common pitfall

`identity`, `resource`, and `identity_group` match a node's **exact stored name
or alias**. `=` and `IN` are exact, case-sensitive matches — a label that
differs in case, spacing, or form (a short name vs. a fully-qualified one, a UID
vs. a username) returns nothing. Use `ILIKE` for case-insensitive or partial
matching:

```sql
-- Exact, case-sensitive match on the stored name or alias:
WHERE resource = 'prod-db'

-- Case-insensitive / prefix match — robust when unsure of the exact name:
WHERE resource ILIKE 'prod-db%'
```

**Workflow:** when you don't already know exact names, run a broad query first
(e.g. `SELECT * FROM access_path WHERE resource ILIKE '%db%'`), read the `name`
values from the output, then scope precisely with `=` / `IN` or keep using
`ILIKE`. An empty result is far more often a name mismatch than a true "no
access" — treat it as a prompt to widen and re-check the name. (A first query by
the wrong identifier — e.g. a UID when the graph indexes identities by username —
silently returns nothing.)

## Query is the scope — and why that matters

Results are derived **only from the graph your query produced**, so a single
query can show you a *subset* of reality — paths are scoped to your query. This
is the single most important semantic to get right:

> A query scoped through an **access list** exercises only the paths that flow
> *through that list*. It does **not** show every path each member has to every
> resource.

So `SELECT * FROM access_path WHERE identity_group IN ('Prod Admins')` answers
"what does membership in *Prod Admins* grant, and to whom" — not "what is the
full access of every Prod Admins member." A member may also reach a resource
through another role, another list, or an access request, and those paths are
invisible to that query.

**Follow-up pattern.** To see a user's (or set of users') *complete* access to a
resource regardless of how it is granted, scope by the identity and resource
directly. Use the access-list query to define both sets, then re-query:

```sql
-- 1. Who's in the list and what does the list reach:
SELECT * FROM access_path WHERE identity_group IN ('Prod Admins')

-- 2. The full picture for those identities against those resources,
--    via ANY path (this is also Access Graph's "View in Graph" query):
SELECT * FROM access_path
WHERE resource IN (<resources from step 1>) AND identity IN (<identities from step 1>)
```

To isolate *why* one row is granted, narrow `identity_group` to just the grantor
returned for that row:

```sql
SELECT * FROM access_path
WHERE identity = '<identity>' AND resource = '<resource>'
  AND identity_group IN ('<grantor for this row>')
```

## Examples

```sql
-- Everyone who can reach a database, by name prefix:
SELECT * FROM access_path WHERE resource ILIKE 'prod-db%'

-- One user's access across the cluster:
SELECT * FROM access_path WHERE identity = 'alice@example.com'

-- A set of users against a set of resources (complete picture):
SELECT * FROM access_path
WHERE identity IN ('alice@example.com', 'bob@example.com') AND resource ILIKE 'prod-%'

-- What an access list grants, and to whom (scoped to the list's paths):
SELECT * FROM access_path WHERE identity_group IN ('Prod Admins')

-- Only identities sourced from Okta:
SELECT * FROM access_path WHERE source = 'OKTA'

-- Resources carrying a specific label (JSON containment):
SELECT * FROM access_path WHERE resource_labels @> '{"env":"prod"}'
```
