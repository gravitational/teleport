# Teleport `tctl access-review` Output Schema

The output is **identity-centric**: a list of identities, each with the
resources it can reach and how. `--format json` (or `yaml`) returns a single
object:

| Field        | Type             | Description                                                                 |
| ------------ | ---------------- | --------------------------------------------------------------------------- |
| `identities` | IdentityAccess[] | One entry per identity in the query's scope. Empty (`[]`) when nothing matched. |
| `warnings`   | string[]         | Non-fatal notices (truncation, activity-lookup failure). Omitted when none. |

## IdentityAccess

| Field       | Type             | Description                                                                                   |
| ----------- | ---------------- | --------------------------------------------------------------------------------------------- |
| `identity`  | Node             | The identity (user or bot).                                                                   |
| `resources` | ResourceAccess[] | What it can reach **within the query's scope**. **May be empty** — see _Empty rows_ below.    |

## ResourceAccess

| Field       | Type      | Description                                                                                           |
| ----------- | --------- | ---------------------------------------------------------------------------------------------------- |
| `resource`  | Node      | The resource reached.                                                                                 |
| `level`     | string    | Resolved access level: `standing`, `impersonate`, `request`, or `denied` (see _Levels_).             |
| `temporary` | bool      | `true` when the access is self-expiring (granted by an access request). Rendered as a `*` in text.   |
| `grantor_counts` | GrantorCounts | How many grantors back this access at each level (`standing`/`impersonate`/`request`). Denied grantors are listed in `grantors` but **not** counted here. |
| `grantors`  | Grantor[] | The identity-group node(s) (access list / role / access request) that grant or deny the access.      |
| `activity`  | Activity  | Present only with a time window (`--from`/`--to`). Access count and last-access time over the window. |

### GrantorCounts

`{ standing, impersonate, request }` — the number of grantors backing the access
at each level (denied grantors excluded; they appear in `grantors`). More than
one at a level means multiple grants back it, so removing a single grantor won't
revoke the access. In text, shown under the `Grantor Counts` column as e.g.
`3 standing, 1 request`.

### Grantor

`{ node: Node, level: string }` — the attributing identity-group node and the
level **it** contributes. The row's resolved `level` is the strongest across all
its grantors (priority `denied > standing > impersonate > request`); a grantor's
own `level` can differ (this is what `--detailed` exposes).

### Activity

`{ count: number, last_access?: string (RFC3339) }`. Absent or null
`last_access` renders as `never`; a zero count renders as `0`. Requires Identity
Activity Center; if the activity lookup fails, `activity` is omitted and a
`warnings` entry (`activity unavailable: …`) is added — the access decision is
still returned.

## Node

Used for identities, resources, and grantors.

| Field       | Type   | Description                                                                       |
| ----------- | ------ | --------------------------------------------------------------------------------- |
| `id`        | string | Node UUID. Filterable via the `id` column (see [QUERY.md](QUERY.md)).             |
| `name`      | string | The node's name **as stored** — what `=`/`IN` match against (exactly, case-sensitively). |
| `alias`     | string | Optional friendly alias; also matched by name filters.                            |
| `kind`      | string | `identity`, `resource`, `identity_group`, `resource_group`, `sub_resource`, …     |
| `sub_kind`  | string | e.g. identity → `user`/`bot`; resource → `ssh`/`db`/`app`/`kube`/`s3`; group → `role`/`access_list`/`access_request`. The text `Kind` columns show this. |
| `source`    | string | Origin system, e.g. `TELEPORT`, `OKTA`.                                           |
| `origin`    | string | Finer origin, e.g. `teleport_user`.                                               |
| `temporary` | bool   | For a grantor, `true` if created by an access request (self-expiring).            |

## Levels

| Level         | Meaning                                                                                              |
| ------------- | ---------------------------------------------------------------------------------------------------- |
| `standing`    | The identity holds the access directly, no action required.                                          |
| `impersonate` | Reachable only by minting a certificate to impersonate another identity (no approval needed).        |
| `request`     | Reachable only after an approved access request.                                                     |
| `denied`      | A deny rule blocks the access. Any denied path wins over everything else.                            |

A trailing `*` on the level in text output marks `temporary` (self-expiring)
access — distinguish it from standing membership so you don't trim access that
will lapse on its own.

## Empty rows

An identity in the query's scope with **no qualifying access** is still
returned, with `resources: []` (text: the identity on a row with blank resource
cells). This is intentional and meaningful: it surfaces an identity that is
present in the graph but is **missing a requirement**, has an **expired/inactive
membership**, or has only **non-access edges** (e.g. review-only). Read it as "no
access **within this query's scope**", not "no access anywhere" — widen the
query to see the rest.

## Text output

**Summary (default)** — one row per `(identity, resource)`; the identity cell is
shown once then blanked:

```
Identity   Kind  Resource    Resource Kind  Access Level  Grantor        Grantor Counts  [Accesses  Last Access]
```

**Detailed (`--detailed`)** — keeps the `Grantor` column and replaces `Grantor
Counts` with `Grantor Level`, showing one row per grantor (each grantor's own level). A resource with multiple grantors gets a
summary row carrying the resolved level and the activity, then one indented
(`↳`) row per grantor; a temporary grantor is marked `*`. A sole grantor is
folded onto the resource's row.

```
Identity   Kind  Resource    Resource Kind  Access Level  Grantor        Grantor Level  [Accesses  Last Access]
```

With a window, a `Period: <from> → <to>` header precedes the table and the
`Accesses` / `Last Access` columns appear.
