---
name: tctl-investigate
description: Use when investigating identity or resource activity in a Teleport cluster. Covers `tctl investigate` filter semantics, fast facet-only exploration, custom Lucene query construction, and token-efficient parsing patterns.
---

# tctl investigate

`tctl investigate` searches Teleport's Identity Security activity log. Run it from a session that has a valid `tctl` login.

```sh
tctl investigate --user alice@example.com --from 24h --format json
```

Output (JSON/YAML) is `{total, facets, data}`:

- `total` — approximate match count across the window (from the stats endpoint; may drift a few percent from `len(data)` on long windows).
- `facets` — `[{name, values: [{value, count}]}]`. `name` is the **CLI flag name** (e.g. `resource`), not the underlying Athena column.
- `data` — array of events. Empty when `--facets-only` is set.

## Filter semantics

- **Multiple values on the same flag are ORed.** `--event-type session.start --event-type session.end` matches either.
- **Different flags are ANDed.** `--user alice --status error` matches events from Alice that errored.
- `--exclude-<flag>` adds a `NOT field:(values...)` clause (e.g. `--exclude-status success`).
- **Structured filter values are always quoted**, so wildcards (`*`, `?`) and regex (`/.../`) passed via `--user`, `--resource`, etc. are matched literally (`--user 'gh*'` looks for the literal string `gh*`). To use wildcards or regex, use `--query` instead (e.g. `--query 'identity_id:gh*'` or `--query 'identity_id:/gh.*an/'`).
- `--query` accepts a raw Lucene expression and is **mutually exclusive** with every structured filter and exclude flag; combining them errors out.
- Time window: `--from` / `--to` accept RFC3339, `YYYY-MM-DD`, relative durations like `24h`/`7d`, or the literal `now`. Default window is the last 1 day.
- `user_agent` is not populated on every Teleport event (the field only surfaces on event types whose payload carries it — login, cert issue, db session start, SCIM list/get). Filtering on `--user-agent` (or `--exclude-user-agent`) will silently drop events that simply don't have the field set, so use it deliberately — usually as a positive filter to find a specific client, not as a blanket `--exclude-user-agent` to "exclude bots". The same caveat applies to the `agent`/`user_agent` field in `--query`. AWS/GitHub/Okta events also generally lack `user_agent`.
- **Geo filters apply to events only.** `--latitude`/`--longitude`/`--radius` (all three required together) restrict the `data` array but **do not** narrow `total` or the facet counts — those come from a separate stats endpoint that doesn't accept geo. So with geo set, expect `total` to exceed `len(data)` even when no `--limit` truncation occurred. Text output prints a `Note:` line when geo is active as a reminder.

## Tips for efficient agent use

### 1. Use `--facets-only` to narrow scope before paying for events

Facets come from one cheap stats query; events come from a more expensive paginated logs query. When you don't yet know which value to filter on, **iterate on facets first**:

```sh
tctl investigate --from 7d --facets-only --format json
# Read facets, pick a value, refine:
tctl investigate --from 7d --user alice --facets-only --format json
# Repeat until the facet set is narrow enough, then drop --facets-only.
```

`--facets-only` implies `--all-facets` in text output so you see every value, not just the top 5.

### 2. Use `--print-query` to build complex Lucene expressions

Structured flags only emit AND-of-ORs. For OR-across-fields, parentheses, wildcards, or anything else Lucene supports, build a baseline and refine:

```sh
tctl investigate --user alice --exclude-status success --print-query
# prints: identity_id:"alice" AND NOT status:"success"

tctl investigate --query '(identity_id:"alice" OR identity_id:"bob") AND NOT status:"success"' --from 24h
```

`--print-query` exits without contacting the backend, so it's free.

#### `--query` syntax reference

The query string is parsed as Lucene and translated to SQL against Athena. Quote values containing spaces or special characters.

**Operators**

| Form                                             | Meaning                                                                  | Example                                             |
| ------------------------------------------------ | ------------------------------------------------------------------------ | --------------------------------------------------- |
| `field:value`                                    | Equality                                                                 | `status:error`                                      |
| `field:"quoted value"`                           | Equality with quoted literal                                             | `identity_id:"alice@example.com"`                   |
| `field:(a OR b OR c)`                            | IN-list (any of) — quote dotted values                                   | `event_type:("session.start" OR "session.end")`     |
| `a AND b`                                        | Boolean AND                                                              | `identity_id:alice AND status:error`                |
| `a OR b`                                         | Boolean OR                                                               | `source:aws OR source:okta`                         |
| `a b` (space-separated)                          | Implicit AND                                                             | `identity_id:alice status:error`                    |
| `NOT a` / `-a`                                   | Negation                                                                 | `NOT status:success`                                |
| `+a -b`                                          | Must / must-not shorthand                                                | `+identity_id:alice -status:success`                |
| `(…)`                                            | Term grouping                                                            | `(identity_id:a OR identity_id:b) AND status:error` |
| `field:val*` / `field:b?z`                       | Wildcards: `*` = any chars, `?` = single char (renders as Athena `LIKE`) | `identity_id:gh*`                                   |
| `field:/regex/`                                  | Regex (Athena `regexp_like`, Java regex syntax)                          | `target_resource:/prod-.*-db/`                      |
| `field:>v`, `field:>=v`, `field:<v`, `field:<=v` | Comparison (works on string fields)                                      | `identity_id:>m`                                    |
| `field:[a TO b]`                                 | Inclusive range                                                          | `identity_id:[a TO h]`                              |
| `field:{a TO b}`                                 | Exclusive range                                                          | `identity_id:{a TO h}`                              |
| `field:[* TO v]` / `field:[v TO *]`              | Unbounded range                                                          | `identity_id:[* TO h]`                              |

- Boolean keywords are case-insensitive: `and`, `AND`, `AnD`, `Or`, `not` all parse.
- An empty `--query ''` is a valid match-all (equivalent to omitting `--query`).
- Backslash-escape Lucene specials in values: `:`, `\`, `(`, `)`, `[`, `]`, `{`, `}`, `+`, `-`, `!`, `^`, `~`, `*`, `?`, `/`, `"`.
- IN-lists with unquoted dotted values may not match — quote them: `event_type:("session.start" OR "user.login")`.
- Regex uses Java regular-expression syntax (Athena `regexp_like`), not POSIX. Anchors (`^`, `$`), `.`, `.*`, character classes, alternation, and quantifiers all work. Match is anywhere-in-string by default.

**Not supported (the parser or backend will reject these):**

| Form                        | Behavior                                                 |
| --------------------------- | -------------------------------------------------------- |
| `val~` / `"a b"~5`          | Fuzzy / proximity — `unable to render operator [FUZZY]`. |
| `val^2`                     | Boost — `unable to render operator [BOOST]`.             |
| `a && b` / `a \|\| b`       | C-style booleans — parse error.                          |
| `_exists_:field`            | Not in field allowlist.                                  |
| Bare term (no `field:`)     | Sent to Athena without a column; query fails.            |
| `NOT NOT a`                 | Parse error.                                             |
| `event_data.*` / inner JSON | `event_data` JSON columns are not queryable as fields.   |

**Queryable fields.** The query layer accepts these names (left column is what you type; right is the underlying column — both forms work):

| Alias                 | Canonical column      | Notes                                                                                                               |
| --------------------- | --------------------- | ------------------------------------------------------------------------------------------------------------------- |
| `source`              | `event_source`        |                                                                                                                     |
| `identity`            | `identity`            |                                                                                                                     |
| `identity_kind`       | `identity_kind`       | Used by the structured `--user-kind` flag.                                                                          |
| `identity_id`         | `identity_id`         | Used by the structured `--user` flag.                                                                               |
| `token`               | `token`               |                                                                                                                     |
| `action`              | `action`              |                                                                                                                     |
| `ip`                  | `ip`                  |                                                                                                                     |
| `city`                | `city`                |                                                                                                                     |
| `country`             | `country`             |                                                                                                                     |
| `region`              | `region`              |                                                                                                                     |
| `resource`            | `target_resource`     |                                                                                                                     |
| `kind`                | `target_kind`         |                                                                                                                     |
| `agent`               | `user_agent`          | Used by the structured `--user-agent` flag. Not populated on every event — see the caveat under "Filter semantics". |
| `type`                | `event_type`          |                                                                                                                     |
| `status`              | `status`              |                                                                                                                     |
| `aws_account_id`      | `aws_account_id`      |                                                                                                                     |
| `aws_service`         | `aws_service`         |                                                                                                                     |
| `github_organization` | `github_organization` |                                                                                                                     |
| `github_repo`         | `github_repo`         |                                                                                                                     |
| `okta_org`            | `okta_org`            |                                                                                                                     |
| `teleport_cluster`    | `teleport_cluster`    |                                                                                                                     |

Unknown field names are rejected with `unknown field "<name>"`.

### 3. Reduce token cost with jq/yq + low `--limit`

Event payloads are large. While iterating, **keep `--limit` low (1-20)** and project just the fields you need:

```sh
tctl investigate --user alice --limit 10 --format json \
  | jq '.data[] | {time, event_type, status, target_resource: .target.resource}'
```

When the filter is right, raise `--limit` (or pass `0` for unlimited) for the full pull.

## Useful flags

| Flag                                  | Effect                                                                                |
| ------------------------------------- | ------------------------------------------------------------------------------------- |
| `--from`/`--to`                       | Time window. RFC3339, `YYYY-MM-DD`, `24h`/`7d`, or `now`. Default last 1d.            |
| `--limit N`                           | Cap events returned. `0` = unlimited. Default 100.                                    |
| `--format text\|json\|yaml`           | Output format. Default text.                                                          |
| `--facets-only`                       | Skip events; return only facets.                                                      |
| `--all-facets`                        | Show every facet value in text (no top-5 cap).                                        |
| `--show-unmatched`                    | Include facet values present in the window but absent from the filter (`count = -1`). |
| `--print-query`                       | Print the constructed Lucene query and exit.                                          |
| `--order asc\|desc`                   | Time order. Default `desc`.                                                           |
| `--latitude`/`--longitude`/`--radius` | Geo-bounded search. All three required together; applies to events only.              |

Structured filters (each `--<flag>` has a matching `--exclude-<flag>`): `--user`, `--user-kind`, `--resource`, `--resource-kind`, `--event-type`, `--source`, `--status`, `--ip`, `--city`, `--country`, `--region`, `--aws-account-id`, `--aws-service`, `--github-org`, `--github-repo`, `--okta-org`, `--teleport-cluster`, `--token`, `--user-agent`.
