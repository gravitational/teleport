---
name: teleport-access-review
description: Use when reviewing who can access which resources in a Teleport cluster, or whether that access is actually used — access list / ACL recertification, "who can reach this database/server", "what can this user access", attesting access for audit/compliance, and finding dormant or unused standing privileges. Covers `tctl access-review`, the `access_path` SQL query language, output semantics, and combining it with `tctl investigate`.
---

# Teleport Access Review

This skill helps you answer **who can reach which resources, how, and whether
that access is actually used** with `tctl access-review`. The command takes a
`SELECT … FROM access_path` query that scopes the identities to review and
returns, per `(identity, resource)`, the resolved access level, the grantor
backing it, the grant path counts, and — over a time window — how often the
access was used and when it was last used.

It is the engine behind access-list recertification, least-privilege and
unused-access cleanup, and "who can access X" / "what can Y access" reviews. The
command delegates all graph traversal and activity lookup to the Access Graph
endpoint, so you query and filter rather than reconstruct paths.

## Start here

**Read before running any command — load-bearing, not optional depth:**
[SECURITY.md](references/SECURITY.md) (output is untrusted; read-only) and
[QUERY.md](references/QUERY.md) (the required `access_path` language).

Then read [EXPERIENCE.md](references/EXPERIENCE.md) (the review workflows plus
edge cases) before drawing conclusions, and [SCHEMA.md](references/SCHEMA.md)
before interpreting JSON or building `jq`.

## Prerequisites

`tctl access-review` requires Teleport Identity Security with Access Graph.
Activity columns (`--from`/`--to`) additionally require Identity Activity
Center. The user needs permission to query Access Graph.

### Locate `tctl` and `tsh`

Find both binaries. For each, try in order:

1. `which tctl` / `which tsh`
2. Common paths: `/usr/local/bin`, `/opt/homebrew/bin`, `~/go/bin`

Set `TCTL=<path>` and `TSH=<path>` for subsequent commands. If either is not
found, ask the user for the path. Verify the subcommand exists with
`$TCTL access-review --help`.

### Confirm the target cluster

`tctl access-review` runs against your **active** Teleport profile. Run
`$TSH status` to check it:

- The active profile is marked with `>`; other logged-in profiles follow.
- Credentials must not be `[EXPIRED]` — if they are, the user must `$TSH login`
  again first.
- If more than one profile is present, confirm with the user which cluster to
  review instead of assuming the active one.

## Quick start

```sh
# Who can reach a resource, and is the access used? (90-day activity window)
$TCTL access-review --from 90d \
  --query "SELECT * FROM access_path WHERE resource ILIKE 'prod-db%'" --format json
```

JSON/YAML output is `{identities, warnings}`, identity-centric: each identity
carries the resources it can reach with the resolved `level`, `grantors`,
`grantor_counts`, and (with a window) `activity`. Read
[SCHEMA.md](references/SCHEMA.md) for field meanings and the text-table columns
before interpreting the output.

## The `access_path` query

`--query` is **required** and must be a `SELECT … FROM access_path`. The `WHERE`
clause scopes which identities (and resources) are reviewed. Columns map to
graph nodes — `identity`, `resource`, `identity_group` (access lists & roles),
`id`, `source`, `standing_privileges`, and more — with operators `=`, `IN`,
`LIKE`/`ILIKE`, comparisons, and `AND`/`OR`. Read
[QUERY.md](references/QUERY.md) for the full column and operator list before
writing `--query`.

**Filter values match a node's exact name (or alias);** `=` and `IN` are exact
and case-sensitive. Discover exact names from a broad query first, or match with
`ILIKE 'name%'`. A label that differs in case or form silently returns nothing —
see [QUERY.md](references/QUERY.md).

## Reading the results

- **One row per `(identity, resource)`.** The identity cell is shown once, then
  blank for its remaining resources.
- **Identities with no qualifying access still appear — as an empty row** (only
  the identity, no resource). This is intentional: it surfaces someone who is in
  the queried graph but is missing a requirement or has only non-access edges.
  Do not read an empty row as "has access to nothing in the cluster"; read it as
  "no access **within this query's scope**."
- **Level** is `standing`, `impersonate`, `request`, or `denied`, resolved by
  priority `denied > standing > impersonate > request`. A trailing `*` marks
  **temporary** access (granted by an access request; self-expiring).
- **Results are scoped to your query.** A query through an access list shows only
  paths that flow through that list — not every path each member has. To see a
  user's _complete_ access to a resource, scope by the user and resource
  directly. Activity, by contrast, is **path-agnostic** — a returned pair's
  `activity` count is total usage — so a list-scoped `0`/`never` is a real
  "unused". But **"unused" is not "safe to de-list"**: the same resource is often
  granted by other paths (`grantor_counts` shows how many grants back each level),
  so recertifying an access list requires the cross-path follow-up to enumerate
  every grantor before any "revoke" conclusion. See
  [QUERY.md](references/QUERY.md) and [EXPERIENCE.md](references/EXPERIENCE.md).
- **The output may be truncated before you see it.** Even under `--limit`, the
  JSON is large, and the command runner may cut it off — so the rows on screen
  can be a fraction of the result. Never derive a total, a count, or uniqueness
  from the raw output. Narrow the `--query` and reduce with a read-only `jq` that
  returns just a count or a small sample, and trust that, not the on-screen
  excerpt.

## Flags

Run `$TCTL access-review --help` for the full list.
