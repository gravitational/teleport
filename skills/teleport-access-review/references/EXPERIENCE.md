# Agentic Experience: Teleport Access Review

## Task framing

A successful review answers an access-governance question — "who can reach
prod-db, and is that access used?", "what does the Prod Admins list grant?",
"what can Alice access and does she still need it?", "which standing grants are
dormant?" — with the right `(identity, resource)` rows, the grantor responsible
for each, and (when asked about usage) the activity. The reviewer's next step is
almost always a remediation they run themselves: trim a list, fix a role, revoke
a grant. Give them the rows and the grantor to act on; don't dump the whole
graph.

## Mental model: standing topology, not a timeline

`access-review` answers **who *can* reach what, right now, and how** — the
standing-privilege graph. It is *not* a session timeline. When activity is
requested it annotates each *current* access path with how often it was used,
but it only ever reports pairs that **have a path today**. Keep this distinct
from `tctl investigate`, which is the temporal "who *did* what, when" log. The
two are complementary — see _Combining with `tctl investigate`_.

## Happy paths

### 1. Discover exact names, then scope

Filters match a node's exact stored name/alias, and `=`/`IN` are case-sensitive;
a label in the wrong case or form silently returns nothing. When you don't
already know exact names, open with `ILIKE` and read the names back, then tighten:

```sh
# Find the resources, read their `name`s:
$TCTL access-review --query "SELECT * FROM access_path WHERE resource ILIKE '%prod-db%'" \
  --limit 50 --format json | jq -r '[.identities[].resources[]? | (.resource.alias // .resource.name)] | unique[]'
# Then scope precisely with = / IN, or keep using ILIKE.
```

An empty result is more often a name mismatch than a true "no access" — widen
and re-check the name before concluding (this is the UID-vs-username dead end).

### 2. Add a window only when the question is about usage

Without `--from`, the review is pure topology and runs against the graph alone.
Adding `--from` (and optionally `--to`) turns on the activity columns and the
audit-log lookup — that's what powers "used / not used". Use a window that
matches the policy (commonly 90 days):

```sh
$TCTL access-review --from 90d \
  --query "SELECT * FROM access_path WHERE resource ILIKE 'prod-db%'" --format json
```

### 3. Use `--detailed` when "how" has more than one answer

The summary shows the primary grantor. When a pair is granted by several
grantors and you need to see each one and its level (e.g. standing via a role
*and* request via a break-glass list), switch to `--detailed`.

### 4. Project with jq while iterating

JSON is nested. Keep `--limit` modest and project just what the question needs;
save a pull to a file and re-`jq` it per view rather than re-running the query:

```sh
$TCTL access-review --from 90d --query "…" --format json \
  | jq -r '.identities[] | (.identity.alias // .identity.name) as $i
           | .resources[] | [$i, (.resource.alias // .resource.name), .level,
               (.activity.count // 0), (.activity.last_access // "never")] | @tsv'
```

## Reasoning through examples

### "Review the Prod Admins access list — who's on it, and who hasn't used it?"

This is the ACL recertification flow. Scope to the list and add the policy
window so you can see usage:

```sh
$TCTL access-review --from 90d \
  --query "SELECT * FROM access_path WHERE identity_group IN ('Prod Admins')" --format json
```

Each row is a member's access *granted through this list*, with the access count
and last-access. **Activity is path-agnostic:** the count is that member's
*total* usage of the resource over the window — not usage *via this list* — and a
pair is returned only when at least one path exists. So a `0` / `never` here is a
trustworthy "this member has not used this resource" (subject to the
activity-source caveats below), no matter how many paths grant it.

**The trap is access, not activity — the list view is not the member's full
picture, in two ways that matter for recertification.** (1) **It omits resources
the member reaches only by other paths:** the `WHERE identity_group` filter
returns only `(member, resource)` pairs with a path *through this list*, so a
resource the member reaches solely via another role or list never appears here.
(2) **"Unused" does not mean "safe to de-list":** a resource shown here may also
be granted by other paths, so removing the member from the list drops only *this*
grant — if the `access` role (or another list) also grants it, their access
persists. `grantor_counts` is the quick tell: more than one grant at the resolved
level means a single de-listing won't revoke — confirm which grantors with
`--detailed` or the cross-path follow-up below.

**Gate — before recommending you trim the list, run the cross-path follow-up.**
Use the list query to define the identities and resources, then re-query by
identity and resource directly (optionally `--detailed`) to see *every* path and
grantor — both the resources the list view omitted and the other grantors of the
ones it showed:

```sh
# identities = the list's members; resources = what the list reaches (from step 1)
$TCTL access-review --from 90d \
  --query "SELECT * FROM access_path
           WHERE identity IN ('alice@example.com','bob@example.com') AND resource IN ('prod-db','prod-web')"
```

The activity counts won't change — they're path-agnostic — but the grantors
will. If Prod Admins is the *only* grantor of a resource, de-listing genuinely
revokes that access; if the `access` role also grants it, de-listing changes
nothing. That's the difference between "trim the list" and "revoke the access".
The direct query may also surface resources the member reaches by other paths
that never appeared in the list view at all. Only access that is unused **and**
granted solely by this list is a clean de-listing candidate.

### "Who can access prod-db?"

Scope by the resource; no window needed if you only care about *who* and *how*:

```sh
$TCTL access-review --query "SELECT * FROM access_path WHERE resource ILIKE 'prod-db%'" --format json \
  | jq -r '.identities[] | select(.resources|length>0) | .resources[0] as $r
           | [(.identity.alias // .identity.name), $r.level, ($r.grantors[0].node | .alias // .name)] | @tsv'
```

Report identities, their level, and the grantor. Remember empty rows: an
identity listed with no resource is *in scope but without a qualifying path* (a
missing requirement or only non-access edges) — not an accessor. Note `denied`
rows explicitly: a deny means they are blocked, not allowed.

### "Who has actually accessed prod-db recently?"

Add the window and read the activity, but mind the boundary with `investigate`:

```sh
$TCTL access-review --from 90d --query "SELECT * FROM access_path WHERE resource ILIKE 'prod-db%'" --format json \
  | jq -r '.identities[] | (.identity.alias // .identity.name) as $i | .resources[]
           | select((.activity.count // 0) > 0) | [$i, .activity.count, .activity.last_access] | @tsv'
```

**`access-review` only counts usage on access paths that still exist.** If
someone accessed prod-db and their access was *since removed*, the pair has no
path today, so it won't appear here at all — the count isn't zero, the row is
absent. For a definitive "who ever accessed this resource", including identities
whose access was later revoked, use `tctl investigate` against the audit log
(see below). Use `access-review` activity to answer "of those who *can* access
it, who is *using* it".

### "Does Alice still need all her standing access?" (unused / least-privilege)

The unused-access cleanup flow (a real, recurring ask — e.g. "tell me if Ben has
any unused permissions"). Scope to the one identity, add the policy window, and
split used from unused. Avoid hedging: enumerate what *is* known.

```sh
$TCTL access-review --from 90d --query "SELECT * FROM access_path WHERE identity = 'alice@example.com'" \
  --format json > /tmp/alice.json
# Standing access never used in the window — candidates to revoke, with the grantor to act on:
jq -r '.identities[].resources[]
       | select(.level=="standing" and (.activity.count // 0)==0 and (.temporary|not))
       | [(.resource.alias // .resource.name), .resource.sub_kind, (.grantors[0].node | .alias // .name // "?")] | @tsv' /tmp/alice.json
# Used standing access — keep:
jq -r '.identities[].resources[] | select(.level=="standing" and (.activity.count // 0)>0)
       | [(.resource.alias // .resource.name), .activity.count, .activity.last_access] | @tsv' /tmp/alice.json
```

Present it as a concrete table: resource, kind, grantor, used/last-used — so the
reviewer can prioritise. Skip `temporary` (`*`) access: it self-expires, so
there's no point trimming it. If you can tell sensitive resources (crown jewels,
specific prod accounts) from low-risk ones, surface those first.

### "What over-privileged access exists via the junior-dev role?"

Scope to the grantor and look for access that looks wrong for it:

```sh
$TCTL access-review --query "SELECT * FROM access_path WHERE identity_group IN ('junior-dev')" --format json
```

Standing access to production from a role that shouldn't grant it is the signal
to fix the role spec, then re-review to confirm the path is gone.

## Combining with `tctl investigate`

The two skills answer different halves of an access question:

| Question                                                   | Command                                  |
| ---------------------------------------------------------- | ---------------------------------------- |
| Who *can* reach X, how, and is that standing access used?  | `tctl access-review`                     |
| Who *did* reach X, when, and what did they do there?       | `tctl investigate`                       |
| Who accessed X even though their access was since removed? | `tctl investigate` (audit log is durable) |
| What are this user's standing privileges to clean up?      | `tctl access-review`                     |

A natural pairing: `access-review` to find dormant standing access, then
`investigate` to confirm the identity truly did nothing relevant in the window
before recommending revocation; or `investigate` to find who *touched* a
resource, then `access-review` to show *how* each of them is granted it.

## Edge cases and fallbacks

- **Separate "unused" from "safe to revoke," and trust activity only as far as
  its source.** Activity is path-agnostic, so a `0` / `never` on a returned pair
  is a real "not used" — *provided the activity source is sound*. If *every*
  identity in scope shows `0` / `never`, suspect the activity lookup (`iac_error`)
  or synthetic/test data before calling anything dormant, and say so rather than
  asserting "unused" as fact. Separately, "unused" never by itself means "safe to
  remove": confirm via the cross-path follow-up that no other grant keeps the
  access alive before recommending a revocation. Honest scoping, not hedging —
  still enumerate everything you *can* assert.
- **Empty rows are signal, not noise.** An identity with no resource row is in
  scope but has no qualifying path here — missing requirement, inactive
  membership, or only non-access edges. Don't drop these silently if the user
  asked "who's in scope"; do exclude them when the user asked "who has access".
- **Empty result (`identities: []` / `No access found.`)** — most often a name
  mismatch (case, alias, wrong identifier) rather than true zero access. Widen
  with `ILIKE` and re-check the name before reporting "nobody".
- **Endpoint unavailable (`access-review is unavailable on this cluster …`)** —
  the access endpoint isn't served: Identity Security isn't enabled, or the
  endpoint isn't yet available on this cluster. This is a hard failure, **not** an
  empty result — do not report "no access". Tell the user to enable Identity
  Security or upgrade the cluster. (A transport error like `connection refused`
  instead means the proxy or Access Graph is down or unreachable — check the proxy
  address and that the services are running.)
- **Truncation warning** (`results truncated at N identities; narrow --query`) —
  more identities matched than `--limit`. Raise `--limit` or narrow the query;
  don't present a truncated set as the complete answer.
- **`activity unavailable: …` warning** (`iac_error`) — the audit-log lookup
  failed (or Identity Activity Center isn't enabled). The access decisions are
  still valid; the usage columns are not. Say so rather than reporting `0`/`never`
  as fact.
- **Activity requires a window.** No `--from` → no activity columns at all.
  `--to` without `--from` is rejected; `--from` must be before `--to`, which
  defaults to now — so a future `--from` with no `--to` is rejected.
- **`standing_privileges` is filter-only and sparse.** You can filter on it but
  it isn't returned, and it isn't populated for every identity — verify against a
  broad query before trusting a threshold; prefer counting standing rows from the
  output for "how much standing access".
- **No regex.** Use `ILIKE` for fuzzy matching; `~` and `SIMILAR TO` are rejected.
- **A malformed query fails with a `400`** naming the problem (`FROM must be
  access_path`, `column "x" not found`, `unsupported operator …`). Fix the query;
  these don't recover on retry.

## Interaction patterns

- State the query (and window) alongside the results, so the user can trust and
  refine the scope — especially the scope caveat when you reviewed by access list.
- Summarise: counts, the dormant/over-privileged candidates, and the grantor to
  act on — don't dump every row. Offer the full table on request.
- When a finding implies remediation (trim a list, fix a role, revoke a grant),
  recommend it and name the exact grantor — but do not run any write command
  yourself (see [SECURITY.md](SECURITY.md)).
- Before an unscoped or very broad review (`WHERE`-less `SELECT *` on a large
  cluster), confirm with the user — it can be slow and large.
- Render timestamps clearly and state the window you reviewed.
