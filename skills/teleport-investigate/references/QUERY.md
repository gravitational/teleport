# `tctl investigate --query` Syntax Reference

`--query` takes a raw Lucene expression and is **mutually exclusive** with every
structured filter and `--exclude-<flag>`; combining them errors out. The string
is parsed as Lucene and translated to SQL against Athena. Quote values
containing spaces or special characters. For the list of queryable field names,
see [SCHEMA.md](SCHEMA.md).

## Operators

| Form                                             | Meaning                                                                  | Example                                               |
| ------------------------------------------------ | ------------------------------------------------------------------------ | ----------------------------------------------------- |
| `field:value`                                    | Equality                                                                 | `status:failure`                                      |
| `field:"quoted value"`                           | Equality with quoted literal                                             | `identity_id:"alice@example.com"`                     |
| `field:(a OR b OR c)`                            | IN-list (any of) — quote dotted values                                   | `event_type:("session.start" OR "session.end")`       |
| `a AND b`                                        | Boolean AND                                                              | `identity_id:alice AND status:failure`                |
| `a OR b`                                         | Boolean OR                                                               | `source:aws OR source:okta`                           |
| `a b` (space-separated)                          | Implicit AND                                                             | `identity_id:alice status:failure`                    |
| `NOT a` / `-a`                                   | Negation                                                                 | `NOT status:success`                                  |
| `+a -b`                                          | Must / must-not shorthand                                                | `+identity_id:alice -status:success`                  |
| `(…)`                                            | Term grouping                                                            | `(identity_id:a OR identity_id:b) AND status:failure` |
| `field:val*` / `field:b?z`                       | Wildcards: `*` = any chars, `?` = single char (renders as Athena `LIKE`) | `identity_id:gh*`                                     |
| `field:/regex/`                                  | Regex (Athena `regexp_like`, Java regex syntax)                          | `target_resource:/prod-.*-db/`                        |
| `field:>v`, `field:>=v`, `field:<v`, `field:<=v` | Comparison (works on string fields)                                      | `identity_id:>m`                                      |
| `field:[a TO b]`                                 | Inclusive range                                                          | `identity_id:[a TO h]`                                |
| `field:{a TO b}`                                 | Exclusive range                                                          | `identity_id:{a TO h}`                                |
| `field:[* TO v]` / `field:[v TO *]`              | Unbounded range                                                          | `identity_id:[* TO h]`                                |

- Wildcards may appear anywhere in the value, including both ends for a
  substring match: `resource:*iac*` matches any resource containing `iac`.
- Boolean keywords are case-insensitive: `and`, `AND`, `AnD`, `Or`, `not` all parse.
- An empty `--query ''` is a valid match-all (equivalent to omitting `--query`).
- A `--query` value that **begins with `-`** (e.g. a leading `-status:success`
  negation) is misread by the CLI as a flag — `expected argument for flag
'--query'`. Use the equals form `--query=-status:success`, or write the
  negation as `NOT status:success`. A leading `+` is unaffected.
- Backslash-escape Lucene specials in values: `:`, `\`, `(`, `)`, `[`, `]`, `{`, `}`, `!`, `^`, `~`, `*`, `?`, `/`, `"`. `+` and `-` are operators only at the **start** of a term; a hyphen *inside* a value (e.g. `ghassan-iac-storage`) is literal — don't escape it. Escaping hyphens around a wildcard (`ghassan\-iac\-*\-storage`) makes the match return nothing.
- IN-lists with unquoted dotted values may not match — quote them: `event_type:("session.start" OR "user.login")`.
- Regex uses Java regular-expression syntax (Athena `regexp_like`), not POSIX. Anchors (`^`, `$`), `.`, `.*`, character classes, alternation, and quantifiers all work. Match is anywhere-in-string by default.
- **No full-text column.** Every term needs a field, so to find a value anywhere,
  OR a `*foo*` wildcard across the text fields, or `--facets-only`-sweep to see
  which facet reports it. `event_data` isn't queryable — grep it client-side.

## Not supported

The parser or backend rejects these — don't suggest them:

| Form                        | Behavior                                                               |
| --------------------------- | ---------------------------------------------------------------------- |
| `val~` / `"a b"~5`          | Fuzzy / proximity — `unable to render operator [FUZZY]`.               |
| `val^2`                     | Boost — `unable to render operator [BOOST]`.                           |
| `a && b` / `a \|\| b`       | C-style booleans — parse error.                                        |
| `_exists_:field`            | Not in field allowlist.                                                |
| Bare term (no `field:`)     | Rejected by the backend; every term must be qualified with a `field:`. |
| `NOT NOT a`                 | Parse error.                                                           |
| `event_data.*` / inner JSON | `event_data` JSON columns are not queryable as fields.                 |

**Invalid queries fail opaquely.** An unqualified term, unknown field, or
unsupported operator returns a generic `got unexpected state: FAILED` / HTTP 400,
not a syntax error — it resembles the transient credentials error but is
deterministic, so retrying won't help. Fix the query and check it with
`--print-query` (free) before spending a real call.
