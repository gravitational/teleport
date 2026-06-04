# Agentic Experience: Teleport Investigate

## Task framing

A successful investigation answers the user's question — "who accessed X?",
"were there failed logins from country Y?", "what did bot Z do last week?" —
with the right events, surfaced concisely, without burning tokens on event
payloads the user didn't ask for.

## Happy paths

### 1. Narrow with facets before paying for events

Facets come from one cheap stats query; events come from a more expensive
paginated logs query. When you don't yet know which value to filter on, iterate
on facets first:

```sh
$TCTL investigate --from 7d --facets-only --format json
# Read facets, pick a value, refine:
$TCTL investigate --from 7d --user alice --facets-only --format json
# Repeat until the facet set is narrow, then drop --facets-only.
```

`--facets-only` implies `--all-facets` in text output so you see every value,
not just the top 5.

### 2. Build complex queries with --print-query

Structured flags only emit AND-of-ORs. For OR-across-fields, parentheses,
wildcards, or regex, build a baseline and refine. `--print-query` exits without
contacting the backend, so it's free:

```sh
$TCTL investigate --user alice --exclude-status success --print-query
# prints: identity_id:"alice" AND NOT status:"success"

$TCTL investigate --query '(identity_id:"alice" OR identity_id:"bob") AND NOT status:"success"' --from 24h
```

See [QUERY.md](QUERY.md) for the full Lucene syntax.

### 3. Keep --limit low and project with jq

Event payloads are large. While iterating, keep `--limit` low (1–20) and project
only the fields you need:

```sh
$TCTL investigate --user alice --limit 10 --format json \
  | jq '.data[] | {time, event_type, status, target_resource: .target.resource}'
```

When the filter is right, raise `--limit` (or pass `0` for unlimited) for the
full pull.

### 4. Let the facets map the concept to event types

A concept the user names — "logins", "database access", "access requests" —
sometimes spans several event types. Don't assume one from its name; sweep the
broad facets first and read the `event-type` values to see which types actually
carry the activity, then filter to all of them:

```sh
$TCTL investigate --status failure --from 7d --facets-only --format json \
  | jq '[.facets[] | select(.name=="event-type").values[]]'
# failed "logins" surface as BOTH user.login and auth — so filter on both.
```

## Reasoning through examples

Each example shows the reasoning from a question to the filter that answers it.
They mirror the invocations listed for this skill in the
[README](../../README.md#teleport-investigate). Read the facets first; only drop
`--facets-only` and pull events once the filter is narrow. Outputs below are
abbreviated.

### "Were there any failed authentications from India in the last 7 days?"

Failed logins span two event types — `user.login` and `auth`, both with
`status:failure` — so query both (see happy path 4). From India →
`--country India`. Facets answer it without pulling events:

```sh
$TCTL investigate --event-type auth --event-type user.login --status failure --country India \
  --from 7d --facets-only --format json \
  | jq '{total, who: [.facets[] | select(.name=="user").values[]]}'
```

```
{ "total": 1, "who": [{ "value": "alice", "count": 1 }] }
```

### "What did bot CI-deployer do yesterday?"

Bots are non-human identities; filter by the bot's name (optionally narrow with
`--user-kind bot`), then read the event-type and resource facets to see what it
did and what it touched:

```sh
$TCTL investigate --user CI-deployer --from 1d --facets-only --format json \
  | jq '{total, did:  [.facets[] | select(.name=="event-type").values[]],
         touched:     [.facets[] | select(.name=="resource").values[]]}'
```

```
{
  "total": 142,
  "did": [
    { "value": "cert.create", "count": 96 },
    { "value": "db.session.start", "count": 23 }
    …
  ],
  "touched": [{ "value": "production-database", "count": 23 }, …]
}
```

From this point you can drill further into specific events or resources,
eventually dropping the `--facets-only` flag to pull the actual events.

### "Show me who accessed the production-database resource this month"

Filter to the resource and to one event per session (`db.session.end`), then
read the user facet to see who reached it:

```sh
$TCTL investigate --resource production-database --event-type db.session.end \
  --from 30d --facets-only --format json \
  | jq '{total, who: [.facets[] | select(.name=="user").values[]]}'
```

```
{
  "total": 34,
  "who": [
    { "value": "alice", "count": 22 },
    { "value": "system", "count": 11 }
    …
  ]
}
```

### "Show me what activity was performed during the following access request `<uuid>`"

An access-request id is **not** a queryable column, and the events that make up
the elevated session carry it in _different_ `event_data` paths. So you can't
filter by the id server-side. To stay efficient, reason about how to surface it:
start with low-cost searches and refine until the result set is narrow.

1. **Confirm the id and see who's involved.** A request surfaces as a target
   resource of kind `access_request`, so it _is_ reachable via `--resource`:

   ```sh
   $TCTL investigate --resource <uuid> --from 30d --facets-only --format json
   ```

   ```
   { "total": 3,
     "facets": [{ "name": "event-type",
                  "values": [{ "value": "access_request.create" }, … ] },
                { "name": "user", "values": ["alice", "bob", "system"] }] }
   ```

2. **Read the request itself.** You only need the `access_request.create` event,
   so filter to it; its payload hands you the requester, the requested
   roles/resources, and the window to search:

   ```sh
   $TCTL investigate --resource <uuid> --event-type access_request.create \
     --from 30d --limit 0 --format json \
     | jq '.data[].event_data | {user, roles, resource_names, time, expires}'
   ```

   ```
   {
     "user": "bob",
     "roles": ["access"],
     "resource_names": ["/acme.example.com/app/internal-dashboard"],
     "time": "2026-06-02T18:47:26Z",
     "expires": "2026-06-03T06:47:14Z"
   }
   ```

3. **Gauge the requester's footprint in the grant window** before pulling, so
   you know how big step 4 is:

   Scoping by the requested resource alone (e.g. `--resource internal-dashboard`) would only
   return events directly tied to that resource, not everything done while the
   grant was active. So pull all of the user's activity in the window, then
   narrow it client-side.

   ```sh
   $TCTL investigate --user bob --from 2026-06-02T18:47:26Z --to 2026-06-03T06:47:14Z \
     --facets-only --format json \
     | jq '{total, types: [.facets[] | select(.name=="event-type").values[]]}'
   ```

   ```
   { "total": 71,
     "types": [{ "value": "cert.create", "count": 54 },
               { "value": "app.session.start", "count": 4 },
               { "value": "mfa_auth_challenge.create", "count": 4 }, … ] }
   ```

4. **Pull that window and keep every event whose payload references the id**
   anywhere — a plain `tostring | contains` catches both `event_data` paths:

   ```sh
   $TCTL investigate --user bob --from 2026-06-02T18:47:26Z --to 2026-06-03T06:47:14Z \
     --limit 0 --format json \
     | jq -r --arg req <uuid> '
         ["time","event_type","resource"],
         (.data[] | select(.event_data | tostring | contains($req))
                  | [.time, .event_type, .target.resource]) | @csv'
   ```

   ```
   "time","event_type","resource"
   "2026-06-03T00:41:50Z","cert.create","bob"
   "2026-06-02T19:09:42Z","app.session.start","internal-dashboard"
   …
   ```

   For a narrative answer rather than a row count, a common pattern is to expose
   the **full** matching events — every column including `event_data` — and hand
   them to an LLM to summarise what the user actually did with the access:

   ```sh
   $TCTL investigate --user bob --from 2026-06-02T18:47:26Z --to 2026-06-03T06:47:14Z \
     --limit 0 --format json \
     | jq --arg req <uuid> '[.data[] | select(.event_data | tostring | contains($req))]'
   ```

## Edge cases and fallbacks

- **`truncated: true`** — more events matched than were returned. Raise `--limit`
  or pass `0`; don't present a truncated set as complete.
- **Empty `data`** — broaden the time window, or add `--show-unmatched` to a
`--facets-only` run to discover which filter values actually exist in the
window, then adjust the filter.
<!-- TODO: Remove Geo filter restriction after stats endpoint adds support -->
- **Geo filters apply to events only.** `--latitude`/`--longitude`/`--radius`
  (all three required together) restrict the `data` array but **not** `total` or
  the facet counts — those come from a stats endpoint that ignores geo. Expect
  `total` to exceed `len(data)` with geo set even without `--limit` truncation;
  explain this, don't report it as an inconsistency.
- **Negative coordinates need the equals form.** A negative `--latitude` or
  `--longitude` is misread as a flag (`expected argument for flag '--latitude'`);
  pass it as `--latitude=-26.2309 --longitude=28.0583`.
- **`user_agent` is sparse.** For Teleport events it's only populated on some
  event types (login, cert issue, db session start, SCIM list/get); treat a
  missing `user_agent` as absent data, not a signal.
- **Wildcards/regex in structured flags don't expand** — switch to `--query` for
  patterns. See [QUERY.md](QUERY.md).
- **Unsupported Lucene constructs are rejected** — don't suggest them. See the
  _Not supported_ list in [QUERY.md](QUERY.md).

## Interaction patterns

- State the assembled query or filters before or alongside the results, so the
  user can trust and refine them.
- Summarise findings (counts, notable values) rather than dumping raw events;
  offer to pull more detail on request.
- Before an unbounded pull (`--limit 0`) over a wide window, confirm with the
  user — it can be large and slow.
- Render timestamps clearly and note the time window you searched.
