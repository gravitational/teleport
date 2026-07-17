---
authors: Carlisia Campos (carlisia.campos@goteleport.com)
state: draft
---

# RFD 253 - Static snapshot DiscoveryConfigs

Related:

- RFD 125 - Dynamic Auto-Discovery Configuration
- Design discussion: https://github.com/gravitational/teleport/pull/68743

## Required Approvers

- Engineering: `@tener && @marcoandredinis`
- Security: `@reedloden || @jentfoo`

## What

Discovery Services periodically publish (heartbeat) their static (file-based) matcher inventory as read-only, TTL-bounded `DiscoveryConfig` resources with `sub_kind: static-snapshot`, one per service instance. This gives static matchers the same resource identity and status surface that dynamic matchers already have, so reporting and troubleshooting work uniformly regardless of how a matcher was configured.

Example: "this instance of discovery service handles this discovery group and has the following matcher configuration." A snapshot answers it in two reads: the record is per-instance, its spec carries the discovery group and the static inventory, and that group keys the dynamic configs the instance also consumes, so a group-filtered list of regular configs completes the picture.

## Details

Terms used:

- inventory: a Discovery Service's static matcher configuration, written to spec.
- heartbeat: one periodic owner publication of the inventory; each successful heartbeat replaces the stored inventory and renews the snapshot's expiry, so a snapshot stays alive exactly as long as its service keeps heartbeating.
- static snapshot: the read-only, TTL-bounded `DiscoveryConfig` record (`sub_kind: static-snapshot`) a service heartbeats its inventory into; a point-in-time observation, never configurable intent.
- reserved name: the exact canonical forms `static-snapshot-<uuid>` and `static-snapshot-hashed-<uuid>` derived from a server ID.
- grandfathered config: a regular `DiscoveryConfig` that held a reserved-shaped name before the reservation existed; it stays readable, deletable, and consumed by its discovery group, but its spec is frozen.
- owner: the Discovery Service instance whose authenticated server ID derives a snapshot's name; the only identity that can heartbeat or report into that snapshot.

### UX

No command or subcommand is new; the delta is entirely behavior on existing surfaces, plus one new UI section:

| Surface                            | Today                                     | Provided by this work                                                                                                |
| ---------------------------------- | ----------------------------------------- | -------------------------------------------------------------------------------------------------------------------- |
| `tctl get discovery_config`        | Exists; regular configs only              | Explicitly combines snapshots into the listing (presentation follow-up)                                              |
| `tctl get discovery_config/<name>` | Exists; regular configs only              | Resolves reserved names to snapshots on capable clients; no CLI change, the server fallback does the work            |
| `tctl discovery status`            | Exists; reads only `ListDiscoveryConfigs` | Merges snapshot reports via the instance-driven enumeration join (presentation follow-up, the non-trivial CLI piece) |
| `tctl get user_tasks`              | Exists; integration-sourced tasks only    | Renders source identity; ambient and static tasks appear (source-aware user-tasks follow-up)                         |
| Integration status page            | Exists; dynamic integration reports       | Static integration-keyed reports land in the data it reads (reporting follow-up); wiring the web UI is out of scope  |
| Discovery UI section               | Does not exist                            | The one genuinely new element: home for ambient and static reporting; out of scope here, later product work          |

### The parity gap this closes

The gap is in two independent axes:

- matcher source: dynamic (`DiscoveryConfig`) versus static (`teleport.yaml`);
- credentials: integration versus ambient (non-integration).

For every combination, users need discovery reports (sync timing, resource counts, errors; the "summary of discovered resources" from the design discussion) and user tasks.

| Source  | Credentials | Reports today | Tasks today | Tasks closed by |
| ------- | ----------- | ------------- | ----------- | --------------- |
| Dynamic | Integration | Yes           | Yes         | Complete today  |
| Dynamic | Ambient     | Partial (1)   | No          | Follow-up (2)   |
| Static  | Integration | No            | No          | Follow-up (2)   |
| Static  | Ambient     | No            | No          | Follow-up (2)   |

Notes:

1. Azure and AWS report for ambient dynamic matchers today; the remaining families are reporting follow-up work (RDS filters ambient credentials during report setup; EKS models integration enrollment only). Ambient reports use the empty integration key in the same status schema, keeping the two axes distinguishable.
2. Source-aware user-tasks follow-up, one coordinated change: remove the agent-side integration guards (`lib/srv/discovery/status.go`), relax server-side validation (`api/types/usertasks` requires an integration for every Discover task type), and add the originating config, a dynamic config name or the snapshot name, to the deterministic task identity (the integration name is a task-name component today); any subset alone is ineffective or unsafe.

### Chosen approach: reuse DiscoveryConfig with a static-snapshot subkind

A static snapshot is an ordinary `DiscoveryConfig` in every schema sense, distinguished by subkind and by where and how it is written. As stored and served:

```yaml
kind: discovery_config
sub_kind: static-snapshot
version: v1
metadata:
  # Derived from the owning service's server ID; reserved.
  name: static-snapshot-9f6d8b2e-4d3f-4c1a-9b7e-2f1a3c5d7e9b
  labels:
    teleport.internal/origin: config-file
  # Auth-assigned, renewed only by a successful heartbeat.
  expires: "2026-07-17T18:40:00Z"
spec:
  # The inventory: mirrors the service's file configuration;
  # may be empty. Publication policy is visible here: every
  # field shown is "published"; "omitted" fields never appear.
  discovery_group: prod-resources
  aws:
    - types: ["ec2"]
      regions: ["us-east-1", "us-west-2"]
      tags:
        env: ["prod"]
      assume_role:
        role_arn: "arn:aws:iam::123456789012:role/discovery"
        # external_id: omitted
      # install (installer parameters, join tokens, proxy
      # settings): omitted
      ssm:
        document_name: "TeleportDiscoveryInstaller"
status:
  # Written by status reporting, preserved by heartbeats; the
  # existing DiscoveryConfig report schema, unchanged.
  state: DISCOVERY_CONFIG_STATE_SYNCING
  discovered_resources: 12
  last_sync_time: "2026-07-17T18:31:12Z"
```

- **Fail-closed inventory.** A heartbeat still carrying installer parameters is rejected rather than silently stripped: sanitizing is the publisher's job, and an Auth-stripped record would misrepresent the configuration the service actually runs. Every service publishes, even with zero static matchers, so an empty snapshot (live service, none configured) stays distinguishable from an absent one (not publishing).
- **Owner-only writes.** Publication routes before generic RBAC for builtin Discovery identities carrying the subkind, and Auth takes only the inventory, rebuilding the whole envelope: this keeps the Discovery role free of generic write verbs and leaves no caller-controlled field that could forge another instance's snapshot or extend a lifetime.
- **No unconditional writes.** Every storage write merges only the writer's owned fields under a revision check, never replacing the whole record: with two independent writers on one record, an unconditional replace would silently restore stale inventory or status over the other's newer data.

#### Enumeration and presentation read path

- `ListDiscoveryConfigs` never returns snapshots; fleet views are a join: list `Instance` records filtered to the Discovery role, derive each snapshot name with `StaticSnapshotName`, then bounded-concurrency named gets.
- `NotFound` is an expected row state, not an error: live service, no accepted snapshot yet (pre-feature Auth, blocked reserved name, or no heartbeat landed).
- A failed get degrades one row, never the whole view; consumers page through instances, never materialize the fleet.
- This path needs `instance` list/read in addition to `DiscoveryConfig` read.
- Instance and snapshot TTLs are independent; both skews are bounded: an instance without a snapshot renders as "no snapshot," and a snapshot whose publisher vanished drops from enumeration up to one snapshot TTL early, harmlessly, since it was about to expire.
- The new-kind alternative was not rejected for needing a union read; this design needs one too. The difference: here the union lives only in fleet-presentation code, while producers, schemas, and per-record reads stay single.

#### Mixed-version behavior

A denied response is evidence about the single endpoint that served the RPC, never about the cluster: rolling Auth upgrades alternate acceptance and denial, so no response ever slows or suppresses heartbeats, and snapshots cannot flap or expire mid-upgrade.

| Discovery Service | Auth endpoint | Behavior                                                                                                   |
| ----------------- | ------------- | ---------------------------------------------------------------------------------------------------------- |
| Old               | Old           | Existing behavior, unchanged                                                                               |
| Old               | New           | Existing behavior, unchanged (no snapshot RPCs are made)                                                   |
| New               | Old           | Discovery unaffected; heartbeat denied, retried next interval; status `NotFound`, retained for next cycle  |
| New               | New           | Heartbeats and snapshot status reports work                                                                |
| New               | Mixed cluster | Converges: each attempt succeeds or fails per endpoint; no TTL-length suppression from any single response |

- The reporter always attempts its status update and keeps no state about past attempts. When the snapshot does not exist yet (old endpoint, or no heartbeat accepted yet), the update returns `NotFound`; the reporter keeps its accumulated status and tries again next cycle. Nothing is ever dropped.
- `AccessDenied` is ambiguous (old Auth, malformed identity, authorization regression); the per-endpoint rule makes that harmless because every cause gets the same handling: keep heartbeating, warn slowly, never block discovery.
- A dedicated capability signal would be cleaner, but reusing the existing RPC constrains the vocabulary, and this rule removes the need for feature negotiation.

#### Quantitative choices

- Ten-minute TTL: survives a couple of missed heartbeats; a stopped or blocked service disappears within minutes.
- Heartbeat interval at most one third of the TTL, jittered: two missed heartbeats still leave the snapshot alive; jitter de-synchronizes fleet writes.
- 256 KiB cap on the whole stored record, spec and status combined. When a write, heartbeat or status alike, would exceed it:
  - At the write: rejected whole with `LimitExceeded`, stored record intact; a partial or truncated record would look authoritative while silently omitting configuration.
  - What the publisher does next: retries at a slow cadence; faster retries cannot help an oversized record.
  - How it self-heals: a fresh heartbeat carries no status, so expiry clears a record crowded by a large stored status within one TTL.
- Four CAS attempts, jittered linear backoff: two writers converge in a few retries; exhaustion returns `CompareFailed`, and the next periodic cycle is the outer retry.
- One snapshot per instance: N services means N records and N writes per interval, independent of matcher count (the cap bounds that).
- Publisher cadences: 30 s bound per publish RPC, 15 s retry after `CompareFailed`, one minute after unexpected errors, warnings throttled to 15 minutes. Only `LimitExceeded` slows the loop.

#### Semantics introduced

```text
regular config:  spec = user intent         status = observed discovery results
static snapshot: spec = observed inventory  status = observed discovery results
```

The status column is identical, so report consumers need no new vocabulary; the spec inversion is what the subkind enforces:

1. Spec as observation: the record must never be consumable as configuration.
2. Ownership by identity-derived name: a regular config accepts status from any authorized service processing it; a snapshot accepts only its owner.
3. Presence as liveness: an unexpired snapshot proves Auth accepted an inventory from that identity within the TTL; it does not prove the service is currently live.
4. Invisibility of a watched kind: the storage range, not the kind, determines visibility; snapshots are structurally excluded from watchers, caches, and listings, reachable only by named get.

Supporting rules: the reserved-name grandfather freeze, the error taxonomy as publisher protocol (`AlreadyExists` means occupancy by a grandfathered confi - again, very unlikely - and is non-retryable; within its own range a heartbeat never fails on existence), and group-required validation relaxed for snapshot specs only.

#### Status semantics

Terminology, precisely: the design discussion's "summary of discovered resources" is the whole status. The only place "summary" names an actual field is inside it: `status.integration_discovered_resources` maps integration names to typed `IntegrationDiscoveredSummary` per-resource-type counts.

The status is the same message for both sources; anything that renders a dynamic config's status renders a snapshot's unchanged. Three semantics are stricter for snapshots, all enforced by Auth:

- One reporter: a snapshot's `server_status` may contain only its owner, and foreign keys are rejected; a dynamic config legitimately accumulates entries from every service in its group.
- No life extension: a status write never renews the snapshot's expiry.
- Bounded together: the size cap is measured on the merged record, stored inventory plus incoming status. An overflowing status write is rejected whole; the stored record, including its previous status, is untouched. Nothing is trimmed or partially applied.

Investigating this surfaced a pre-existing dynamic-side wart, out of scope here: with several services reporting one dynamic config, each status update replaces the whole status, so concurrent reporters are last-writer-wins. Snapshots structurally avoid it by having exactly one reporter.

### Alternative considered: a new resource kind

A dedicated per-instance resource (for example `discovery_service`) carrying the observed inventory and a discovery summary. The summary is the same payload in both designs, the existing `DiscoveryConfigStatus` message; the designs differ only in which envelope carries it. But that envelope is the cost: a new kind brings the full apparatus of one (proto service, client, RBAC noun, `tctl` plumbing, a permanent second concept), detailed under the weakest case below.

#### Strongest case

##### General

Not advantages over the chosen design, but costs this alternative does not actually carry:

- Needs no cache and no watch: RFD 153 makes caching optional for infrequently accessed resources.
- Needs no second report schema: it can embed the existing `DiscoveryConfigStatus` message.
- Needs no verbatim YAML matchers: that was one discussion variant, not intrinsic; it can use the typed matcher schema.

##### Versus the chosen design

- Read-only and machine-only by construction: no subkind or reserved-name carve-outs inside a user-managed kind, and `spec` keeps a single meaning everywhere.
- Hosts the problem-inherent semantics (observed inventory, per-instance ownership, liveness TTL, split write ownership) natively instead of as subkind exceptions.
- The chosen approach is non-uniform (one noun with special cases, not one uniform surface), which is a plus for this option since with this approach the semantics would te more clear:
  - lists are intentionally incomplete for the kind;
  - named gets depend on name shape and client version;
  - spec meaning inverts by subkind;
  - reserved names change write-verb behavior;
  - owner reads are inventory-stripped;
  - fleet presentation still needs the enumeration join.

#### Weakest case

- A second noun and its API surface: a new proto service, client, RBAC kind, and `tctl` plumbing for one machine writer and a handful of readers, plus a permanent second concept in docs, RBAC policy, and support.
- An explicit union in every consumer: both designs need the fleet-presentation join, but with the subkind only fleet presentation merges; with a new kind, every consumer of "discovery state," including ones that never enumerate the fleet, must know both nouns.
- Duplicated write-path machinery: its own CRUD, validation, size cap, CAS, and TTL handling, even when sharing the status message; the subkind reuses all of it.
- If the shared status message is not adopted, a second report vocabulary that every producer targets and every consumer translates; if the YAML variant is adopted, an opaque payload whose parsing and validation move to each consumer.

The trade: the subkind buys maximal reuse at the price of special cases inside one noun, enforced structurally at one write boundary and documented here; the new kind buys semantic clarity at the price of a second noun that every consumer, operator, and document re-learns forever. Measured against the parity matrix, the subkind reaches every row with a source-name change; the new kind reaches the same rows only after building its own write path, even in its strongest form.

### Security

No new attack surface: heartbeats ride an existing authenticated RPC, and a compromised Discovery Service gains nothing beyond describing its own configuration. Snapshots never grant access, and reserved-name responses never act as an existence oracle for callers lacking the read verb.

Publication policy: a fixed, per-field decision made when a field is introduced, no runtime state. Each field is either published or omitted; anything else is an explicit future amendment. Dynamic configs need no such policy because their spec is user-authored cluster data, exposure chosen by whoever wrote it; a snapshot's spec is machine-copied from a host-local file, so the policy is the consent rule for what the machine may copy off the host, and without it a new schema field would be copied fleet-wide the moment it exists, with nobody choosing.

- omitted: installer-parameter subtrees (join tokens, script and proxy settings) and AWS assume-role external IDs (confused-deputy material).
- published: resource-selection metadata (types, regions, locations, projects, subscriptions, tags, labels, integration names, role ARNs and role names, namespaces, SSM document names). Same audience as the identical fields on dynamic configs; the new exposure is file-only configuration leaving the host, which is the point.

Note: Adding a field to a matcher requires deciding, in the same change, whether snapshots publish it. This rule is new; today a new field flows into snapshots without anyone deciding. Enforcement: a reflection test walks every matcher field and fails when one has no decision, so forgetting is a build failure, not a leak. This is test-and-review protection, not an architectural guarantee; if it ever proves too weak, the fallback is to publish only an explicit allowlist of fields instead of everything except the omitted list.

### Privacy

The published inventory is configuration metadata (matcher types, regions, tags, project and subscription selectors, integration names); it contains no user data or PII. Retention is bounded by the ten-minute TTL: records vanish shortly after their publisher stops heartbeating them.

### Proto Specification

None. The subkind rides the existing resource header field, and the spec and status reuse the existing `DiscoveryConfig` messages and RPCs unchanged.

### Schema extensibility

The existing fields serve both sources unchanged.

If presentation needs more, fields are added to the shared schema (additive, ignored by old clients, one addition serving dynamic configs and snapshots).

This is the complete status the dynamic schema has today, used identically for snapshots:

```yaml
status:
  state: DISCOVERY_CONFIG_STATE_RUNNING # RUNNING | ERROR | SYNCING
  error_message: "" # set when state is ERROR
  discovered_resources: 12 # count from the previous iteration
  last_sync_time: "2026-07-17T18:31:12Z"
  # Per-integration rollup; each resource type (aws_ec2, aws_rds,
  # aws_eks, azure_vms) carries found/enrolled/failed counts plus
  # sync start and end timestamps.
  integration_discovered_resources:
    my-aws-integration:
      aws_ec2: { found: 12, enrolled: 10, failed: 2 }
  # Per-reporting-service detail; for a snapshot this map holds
  # exactly one entry, the owner.
  server_status:
    9f6d8b2e-4d3f-4c1a-9b7e-2f1a3c5d7e9b:
      last_update: "2026-07-17T18:31:12Z"
      poll_interval: 5m
      # The empty integration key means ambient credentials.
      integration_summaries:
        "":
          aws_ec2:
            { current: {}, previous: { found: 12, enrolled: 10, failed: 2 } }
```

These are the only proposed additions, used exactly as shown when an inventory outgrows the size cap (per-family counts and a flag replace matcher detail; heartbeat-owned, publication-policy classified). When the inventory fits, neither field is present and the spec is exactly the payload example above; absence means full detail, never unknown (proto3 does not serialize a false bool, so an explicit `matchers_truncated: false` is the same wire state, and no tri-state exists because the heartbeat writes the flag and the matcher detail atomically):

```yaml
spec:
  # Publication policy: published.
  matcher_counts:
    aws: 2
    azure: 1
    gcp: 0
    kube: 0
  matchers_truncated: true
```

### Test Plan

This is a cross-version machine protocol, so the implementation must include integration coverage for: mixed-version Auth (new publisher against old Auth and against an alternating mix); old clients performing named gets; reserved-name collision migration end to end (clone, verify, delete, heartbeat unblocks); snapshot expiry while status writes continue; size pressure from spec and status independently; concurrent heartbeat and status writes exercising the CAS bound; no snapshot events reaching `DiscoveryConfig` watchers and no snapshots entering caches or dynamic matcher loading; per-family sensitive-field sanitization against the publication policy; and enumeration plus CLI behavior with partial named-get failures. When the CLI presentation lands, `tctl discovery status` and `tctl get discovery_config` gain manual test plan entries covering static inventory display, including the empty-versus-absent snapshot distinction.
