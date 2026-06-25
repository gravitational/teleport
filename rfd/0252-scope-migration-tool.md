---
authors: Randy Modowski (randal.modowski@goteleport.com)
state: draft
---

# RFD 0252 - Cluster and scope migration tooling (`tmig`)

## Required Approvers

* Engineering: @r0mant || @hugoShaka
* Security: @rosstimothy
* Product: @klizhentas

## What

`tmig` migrates running Teleport agents from one cluster (or one scope) into another. It stands up a
validated second agent on each host and retires the original only after the operator promotes the final
stage; cutover is never automatic. It migrates only agents, never backend state, and it reports
IaC-managed resources rather than mutating them.

## Why

[RFD 0229 (Scopes)](./0229-scopes.md) lets one cluster host many independently administered subtrees.
That is the isolation organizations run separate clusters for today, so once scopes exist they will want
to collapse a fleet of clusters into one. There is no migration path for that, and a cluster is not a
unit you can simply copy.

`tmig` owns the agent-fleet half of that consolidation. It does not retire a SOURCE cluster on its own:
a cluster's users, roles, audit history, recordings, and trust relationships are out-of-band and out of
scope (see [Non-goals](#non-goals)). What it delivers is the safe re-enrollment of every agent into the
right place, plus a report of what it could not automate.

The driver, and the whole of v1, is consolidating unscoped clusters into the scopes of one scoped
cluster (`SOURCE` into a `scope` in `TARGET`). The stage model isn't inherently tied to that one
direction: the same flow, classifier, config rewrite, and marker would also serve scope-to-scope and
unscoped-to-unscoped re-enrollment (version upgrades, Cloud-to-self-hosted). Those modes are out of
scope for v1 and proposed as future work (see [Future work](#future-work)). v1 does the one thing, and
it depends on scopes reaching GA on TARGET (see
[MVP boundary and dependencies](#mvp-boundary-and-dependencies)).

Migrating a single agent is straightforward: re-point its config and let a second install join. The
difficulty is scale. The fleet is large (thousands of hosts), mixed (token and delegated joins; systemd,
Kubernetes, and hand-rolled installs; multi-service agents), and partly offline at any moment. By hand
there is no dry run, no rollback, no signal for when retiring the originals is safe, and a standing risk
of disturbing IaC-owned resources. `tmig` makes it a reviewable, resumable operation with easy rollback
before cutover.

One framing point up front: in the MVP `tmig` is a central-admin tool that scope admins can partly drive. A
scope admin runs `inventory`/`enroll` scope-pinned on their own, but `verify`, `drain`, and `decommission`
rely on reads that aren't scope-aware yet, so a central admin (or a narrow interim unscoped grant) owns the
back of the lifecycle. Full self-service opens once those reads gain scope awareness;
[Permissions](#permissions-and-the-migrator-role) is the canonical account.

Recommended reading: [RFD 0229 (Scopes)](./0229-scopes.md),
[RFD 0229a (Scoped Join Tokens)](./0229a-scoped-join-tokens.md),
[RFD 0243 (Scoped Roles in Access Lists)](./0243-scoped-roles-in-access-lists.md),
[Agent Inventory UI (`./0229-agent-inventory-ui.md`)](./0229-agent-inventory-ui.md).

### Non-goals

* Backend-to-backend state copy. Users, sessions and recordings, audit history, cluster auth settings,
  and trusted/leaf-cluster relationships are not migrated.
* Retiring a SOURCE *cluster*. `tmig` disables migrated *agents*, host by host; the control plane,
  identities, audit history, and recordings are out of scope, so a cluster shutdown is additional
  out-of-band work.
* Migrating control-plane hosts. The migrated config is always an agent that dials TARGET, so
  `auth_service` and `proxy_service` are disabled in it.
* Automatic role or access-list migration. Teams author roles in the new scope and own them via IaC. An
  opt-in report-only resource mode exists (see [Optional resource migration](#optional-resource-migration));
  it never creates policy, and v1 does not emit applyable policy (emitting is proposed for future/v2).
* Unattended decommissioning. Turning off the old agents is always an operator-promoted, gated step.

## Details

### Terminology

* **SOURCE** / **TARGET**: the cluster an agent migrates from / into. When the target is scoped, agents
  land in a specific scope. Always written uppercase.
* **scope**: a path-like administrative boundary carried as a resource or credential *attribute* (for
  example `/org/team-a`), not a created object. A scope "exists" insofar as the cluster accepts scoped
  operations at that path.
* **resource group**: an example of a resource label customers use to group hosts. `tmig` filters on
  whatever labels you choose and points the selected hosts at a scope; a mapping can also select a whole
  cluster (no selector). Selector trust matters; see
  [Selector trust and AUTO eligibility](#selector-trust-and-auto-eligibility).
* **suffixed install**: a parallel agent managed by `teleport-update --install-suffix <s>`, with config
  `/etc/teleport_<s>.yaml`, data dir `/var/lib/teleport_<s>`, and unit `teleport_<s>.service`. It is the
  supported way to run two agents on one host and the basis of the side-by-side validation window. The
  suffix is a run-config field (see [Run state and the suffix contract](#run-state-and-the-suffix-contract)).
* **migration marker**: reserved-prefix labels that correlate a SOURCE host with its migrated TARGET node.
  Verify, idempotency, and decommission key on it, subject to the caveats in
  [The migration marker](#the-migration-marker).
* **stages**: `inventory → preflight → enroll → verify → drain → decommission`. `validate` is the sign-off
  gate between `verify` and `drain`; it is a gate, not a pipeline stage (see
  [Commands, stages, and gates](#commands-stages-and-gates)).
* **verdicts**: `AUTO`, `PREREQ`, `PIPELINE`, `MANUAL` (per-host classifier outputs).
* **IaC**: Terraform or the Kubernetes operator. It owns roles, access lists, and long-lived tokens.
  `tmig` reports IaC-managed resources; it never mutates them.

(`tmig` is a working name; see [Future work](#future-work).)

### UX

`tmig` runs as a sequence of stages. The read-only stages, `inventory` and `preflight`, make no cluster
or host mutations: preflight reads `/webapi/ping` and the local capability table and changes nothing on
either cluster. They do write local run state and the readiness report, which are part of the safety
model (see [Run state and the suffix contract](#run-state-and-the-suffix-contract)), not durable fleet
changes. Each mutating stage runs only after an operator reviews the previous stage's output and promotes
the run forward. Every stage is resumable and converges across the fleet: offline or failing hosts stay
pending and are retried without holding up the rest. Idempotency is server-proven where a server-enforced
marker exists and best-effort (static label plus an on-host proof at decommission) otherwise; see
[The migration marker](#the-migration-marker).

```
inventory → preflight → enroll → verify →[validate gate]→ drain → decommission
(read-only) (read-only)  (──────────── operator-promoted ─────────────)
```

The mutation boundary keeps the blast radius small. `enroll` only adds a second agent and leaves the
original running, recoverable by stopping the new install. Only `drain` and `decommission` carry
irreversible risk, so they are gated, fail closed on active-SOURCE signals, and never touch IaC-managed
resources.

#### Commands, stages, and gates

`tmig` is one subcommand per stage, plus `validate`. Read-only commands need no flag; commands that mutate
require `--execute`. This table is the source of truth for the contract.

| Command | Role | Mutates a cluster/host | `--execute` | Persists to run state |
|---|---|---|---|---|
| `inventory` | stage (read-only) | no | no | host facts, probe results |
| `preflight` | stage (read-only) | no | no | verdicts, identity pins, config/report hashes |
| `enroll` | stage | TARGET tokens + SOURCE second agent | yes | per-host enroll state, planted nonce |
| `verify` | stage (read-only) | no | no | per-host presence + marker trust |
| `validate` | **gate** (not a pipeline stage) | records sign-off only | uses `--confirm` | per-mapping sign-off |
| `drain` | stage (read-only by default) | deletes only with opt-in | yes (deletes/waivers) | drain findings, waivers |
| `decommission` | stage | disables SOURCE units | yes | per-host decommission record |

`validate` is a gate, not a stage: recording sign-off gates `decommission` without advancing hosts through
the pipeline. The read-only stages still write local run state and the report; only
`enroll`/`drain`/`decommission` change anything on a cluster or host.

#### User story 1 — consolidating a team into a scope

An org runs a dozen unscoped clusters and is moving to one scoped cluster (`root → org → team`). Dana
administers the `team-a` scope (`/org/team-a`) and wants to move the `team-a` resource group out of the
unscoped `legacy-1` cluster into her scope. She holds a migrator assignment scoped to `/org/team-a`, and a
central admin finishes `verify`/`drain`/`decommission` with her (see
[Permissions](#permissions-and-the-migrator-role) for why). Her run config maps a label selector to a scope:

```yaml
# run.yaml
apiVersion: tmig/v1
target:
  proxy: scoped.example.com:443
  identity: ./team-a-identity          # scope-pinned to /org/team-a
suffix: team-a-mig                     # names the parallel install
migrations:
  - source: { proxy: legacy-1.example.com:443, identity: ./legacy-1-identity }
    mappings:
      - selector: { resource_group: team-a }
        selector_trust: iac_inventory  # how the selector is trusted; see below
        scope: /org/team-a
    # hosts matching no mapping are reported as orphans, never migrated
```

1. **Inventory (read-only).** `tmig` connects to SOURCE read-only, enumerates agents from the cluster
   inventory, probes each host over SSH (binary and `teleport-update` presence, systemd, config
   readability, join method), and resolves each against the mappings. A host matching two mappings is an
   error, not a race.

   ```console
   $ tmig inventory -c run.yaml
   legacy-1: 1,812 agents; 240 match resource_group=team-a; 0 mapping overlaps
   ```

2. **Preflight / dry run.** Preflight runs the cross-cluster and capability checks (see
   [Preflight checks](#preflight-checks)), classifies every host (see [Classification](#classification)),
   renders the per-host config change with secrets redacted, and writes the readiness report Dana reviews
   before promoting.

   ```console
   $ tmig preflight -c run.yaml -o report/
   SOURCE legacy-1 (v17.3.1) → TARGET scoped.example.com (v17.3.1); scopes enabled ✓
   capability table: v17.3 known ✓   selector trust: iac_inventory ✓
   roles scopable on TARGET: Node ✓ Kube ✓ App ✓ Db ✗
   verdicts: AUTO 201 · PREREQ 22 (3 blocked) · PIPELINE 12 · MANUAL 5
   report: report/readiness.html
   ```

3. **Fix prerequisites.** Dana lands the emitted Terraform for the three blocked IAM hosts, so her
   pipeline owns the token. Re-running `preflight` shows `PREREQ 22 (0 blocked)`.

4. **Enroll (promoted).** Enroll first runs a single throwaway token round-trip (the
   [RBAC canary](#preflight-checks)) to prove create/read/delete work in the scope. Then for each AUTO host
   it mints a single-use scoped token, produces the suffixed config on the node (see
   [Producing the migrated config](#producing-the-migrated-config)), plants a decommission nonce, enables
   the suffixed unit, and starts it. The originals keep serving SOURCE. The day-two re-run is a no-op
   because each migrated host carries its [marker](#the-migration-marker).

   ```console
   $ tmig enroll -c run.yaml --execute
   canary: scoped_token create→read→delete in /org/team-a ✓
   enrolling 223 (parallelism 32) … 219 ✓, 4 pending (unreachable, will retry)
   ```

5. **Verify.** Verification lists nodes in the mapped scope using Dana's scope-pinned identity and matches
   them to SOURCE hosts by marker. Where it needs the cluster *instance* inventory it depends on a
   not-yet-scope-aware read and falls back to a central admin or interim unscoped identity (see
   [Verification and the instance-inventory dependency](#verification-and-the-instance-inventory-dependency)).
   The five MANUAL hosts run their two rendered commands during a window, and `verify` picks them up.

6. **Validate and sign off (gate).** Team A logs into the new scope and confirms access; Dana records it.
   The gate is recorded sign-off plus verified presence; the usage line is advisory and shows only when
   the audit read is available.

   ```console
   $ tmig validate -c run.yaml --mapping /org/team-a --confirm
   sign-off recorded for /org/team-a
   usage since enroll (advisory): 31 users, 1,408 sessions · 6 hosts zero-usage (flagged)
   ```

7. **Drain, then decommission.** Drain reports the source token surface and recent joins; `decommission`
   fails closed if SOURCE is still in use (see [Drain and decommission](#drain-and-decommission)).

   ```console
   $ tmig decommission -c run.yaml --execute
   gate: /org/team-a signed off ✓ · no active-SOURCE blockers ✓
   eligible 209 (verify-green + marker match + on-host proof) · skipped 14 (db_service)
   disabled original units on 209 hosts
   ```

**Failure modes Dana hits.** An unreachable host at enroll stays `pending`; she re-runs later. A
misconfigured suffixed agent is recoverable: the original is still serving, so she stops the unit and runs
`teleport-update --install-suffix team-a-mig uninstall`. A `decommission` before sign-off is refused and
names the mapping. A missing delegated token halts that host at preflight with exact remediation, never a
silent skip.

#### User story 2 — an automated/CI run

`tmig` is built to be driven non-interactively. An operator (or an AI agent) runs
`tmig preflight --format=json`, parses verdicts and blockers, and surfaces emitted remediation for a human
to approve; the tool never applies IaC. It re-runs preflight until `blocked: 0`, then
`tmig enroll --execute --format=json`, then polls `tmig verify --format=json` until pending reaches zero.
It stops at the validation gate, because sign-off is a human decision. The CLI choices that make this work
(structured output, no prompts, actionable errors, stable exit codes) are under [CLI UX](#cli-ux).

### The readiness report

`preflight -o report/` writes the report (HTML, plus JSON via `--format=json`) an operator reads before
promoting. It lists each host's verdict and reason, marker trust, selector trust, the redacted config
change, copy-pasteable remediation for blocked hosts, unmapped hosts, and which services would be disabled.
A trimmed excerpt:

```
Run 7f3c  ·  SOURCE legacy-1 → TARGET /org/team-a  ·  240 hosts  ·  2026-06-22

  verdict    hosts   marker trust              selector trust
  AUTO         201    server-enforced (SSH/Node) iac_inventory
  PREREQ        22    static-label  (3 blocked)  iac_inventory
  PIPELINE      12    n/a (pipeline)             iac_inventory
  MANUAL         5    static-label               iac_inventory

blocked (showing 2 of 3):
  ip-10-2-9-3   PREREQ   iam: no scoped token in /org/team-a covers account 8839… (role Node)
                         → apply emitted Terraform (scoped_token "team-a-iam")
  gpu-7         MANUAL   no teleport-update present → render the two-command runbook

would-change  (ip-10-2-4-7, AUTO; secrets redacted):
  proxy_server → scoped.example.com:443 · auth_token → join_params (token) · +data_dir
  ca_pin → TARGET pin(s) · db_service disabled (no scoped support); stays on original, host partial

orphans (match no mapping): 4 hosts        services disabled on 14 hosts (db_service)
```

### MVP boundary and dependencies

v1 is the unscoped-into-scoped consolidation (User Story 1). It is design-complete on existing machinery
but depends on scopes reaching GA on TARGET (the "Need, and no workaround" group below); until then it
runs only behind `TELEPORT_UNSTABLE_SCOPES` and is not for production. The dependency table below is what
stands between the design and that GA-gated v1.

`tmig` ships out-of-tree on the scopes-enabled `/webapi/ping` check, scoped join tokens for the PREREQ
methods per [0229a](./0229a-scoped-join-tokens.md), and scoped node listing via
`ListUnifiedResources`/`KindNode`. The remaining gaps fall into three groups.

**Want, and have a workaround** (the MVP ships on these):

* Cluster capability (scopable roles/methods) → versioned local table, fail-closed.
* Scope-aware instance inventory → interim unscoped reader, or central admin runs verify.
* Scope-aware audit/session reads → unscoped read or central admin; usage advisory.
* Team-scoped SOURCE drain reads → a central drain reader.
* Node-local `teleport reconfigure` → pushed helper, then opt-in operator-side rewrite.
* Scope-pinned Discovery → per-host PREREQ migration plus drain monitoring.
* Migrator role template → hand-authored per-scope roles, or `assignable_scopes: ["/**"]`.

**Want, but no workaround** (absent in the MVP, non-blocking):

* `scoped_token.*` audit events ([0229a](./0229a-scoped-join-tokens.md), proposed) → scope-narrowed token attribution, no substitute.
* Server-enforced marker for non-SSH agents → only the weaker static-label + on-host-SSH-proof substitute.

**Need, and no workaround** (blocks v1):

* Scopes production-ready on TARGET, currently behind `TELEPORT_UNSTABLE_SCOPES` and not production-ready.
  v1 is the unscoped-into-scoped consolidation, so it depends on scopes reaching GA.

### Preflight checks

Before classifying hosts, preflight confirms the run targets the intended clusters and that TARGET can do
what the run needs.

* **Cross-cluster identity.** For both ends, record and display the cluster name and stable cluster ID, the
  proxy address and CA fingerprint the operator profile trusts, the authenticated user, and whether the
  credential is scope-pinned. These go into the run-state log, so `enroll` and `decommission` act against the
  identities preflight approved. Preflight hard-fails if SOURCE and TARGET resolve to the same cluster, since
  a profile mix-up or proxy typo could otherwise mint tokens in, or disable units on, the wrong cluster. In v1
  this hard-fail is absolute: SOURCE (unscoped) and TARGET (scoped) are always different clusters. A
  same-cluster scope-to-scope run would need a guarded exception to this invariant, but that mode is future
  work (see [Future work](#future-work)).
* **Gating and version.** Scopes sit behind `TELEPORT_UNSTABLE_SCOPES=yes` and are marked not-production-ready,
  both enforced in code. Preflight records SOURCE/TARGET versions and the agent versions found at inventory,
  confirms the scopes gate, and flags any host whose binary or flags would make a scoped suffixed install
  incompatible. Version *parity* is rollout guidance, not a code-enforced rule, and relaxes at GA; a run during
  an upgrade window must set its version target explicitly.
* **Capability data is fail-closed.** Preflight resolves the scopable role/method set from the versioned local
  table (see [Sourcing scope capability](#sourcing-scope-capability)). If the TARGET version is newer than the
  table knows, preflight fails closed: affected hosts aren't classified AUTO and enroll is blocked for them. An
  operator can override with `--capability-override`, which records the TARGET version, the table version, the
  assumed support set, and a reviewer sign-off into run state.

The next check is scope-specific; every v1 mapping targets a scope, so it always runs.

* **Scopes enabled and scopable set (cluster, read-only).** Read `GET /webapi/ping` to confirm
  `auth.scopes: "enabled"` and `proxy.scopes_enabled: true`, validate the mapped scope path, and resolve which
  roles and methods are scopable (see [Sourcing scope capability](#sourcing-scope-capability)). All read-only.

One check happens at `enroll`, not preflight, because preflight makes no cluster writes:

* **Identity RBAC, proven by canary.** Whether the *cluster* supports scoped tokens is separate from whether
  the *operator's identity* may create them in the target scope (RBAC: `scoped_token: create,read,delete`; see
  [Permissions](#permissions-and-the-migrator-role)). Enroll's first action is a single throwaway token
  round-trip in the scope (create, read, delete): the *RBAC canary*. If any leg fails, enroll stops before
  touching the fleet, so a missing `delete` permission can't surface only after thousands of tokens exist.

### Selector trust and AUTO eligibility

A mapping decides which TARGET scope a host joins from a SOURCE label selector. On a flat legacy cluster, node
labels can be node-supplied, team-supplied, stale, or spoofed, so trusting one blindly could let a compromised
host select itself into another team's scope. The blast radius is bounded (a host can only land in a scope the
operator's identity may mint tokens in, and the operator reviews the matched set), but it remains a trust
decision the tool must make explicit. Each mapping declares a `selector_trust` source, shown per host:

* `iac_inventory`: reconciled against an authoritative external inventory (Terraform state, a CMDB, cloud tags
  resolved by the cloud API). Trusted for AUTO.
* `source_immutable`: keys only on SOURCE-side labels that are server-enforced, e.g. immutable labels on the
  SOURCE join token. Trusted for AUTO. These exist only for `Node`/SSH hosts minted by a scoped token, so on
  v1's flat *unscoped* SOURCE there are none to key on; it would only become useful for a scoped SOURCE (the
  future scope-to-scope mode).
* `attested`: a central admin has attested the mapping for the matched set. Trusted for AUTO, attestation
  recorded in run state.
* `node_reported`: node-supplied labels with no external corroboration. Not trusted for AUTO scoped enrollment:
  matched hosts are downgraded to MANUAL so an untrusted label can't silently place a host in a scope.

### Per-host mechanics

For one AUTO host, `tmig` mints a single-use scoped token on TARGET, writes its secret to a 0600 file on the
node, plants a decommission nonce, runs the node-local config transform, enables and starts the suffixed unit,
polls TARGET for the marker with its scope-pinned identity, then deletes the token. The original agent keeps
serving SOURCE side-by-side until decommission.

The migrated config is the original with only the minimal changes needed to join TARGET: a new endpoint and
`join_params`, a new `data_dir`, TARGET CA pins in place of SOURCE pins, `auth_service`/`proxy_service`
disabled, and unsupported services disabled (see [Scope-support gap](#scope-support-gap)). Everything else,
including comments and key order, is preserved. The token secret passes by file reference, so it never appears
in argv, logs, or the config body. The decommission nonce planted alongside it lets static-label decommission
prove, over SOURCE SSH, that this exact host was enrolled by this run (see
[The migration marker](#the-migration-marker)).

#### The migration marker

**In one line:** the marker is how a SOURCE host is correlated with its migrated TARGET node; only a
server-enforced marker authorizes decommission on its own, while a static label is a correlation hint that needs an
extra on-host proof.

A suffixed install gets a new host UUID, so SOURCE and TARGET nodes can never be matched by UUID equality. The
marker is the only thing that correlates them; verify, idempotency, and decommission all key on it. Its content
is labels under a reserved prefix (for example `tmig.teleport.dev/`): source cluster, source server ID, scope,
install suffix, and run ID. Where it lives, and how far it can be trusted, depends on the host.

AUTO SSH hosts (Node role) carry the marker as immutable labels on the tool-minted scoped token
([RFD 0229a](./0229a-scoped-join-tokens.md)): certificate-bound and server-enforced, so they can't be edited on
the node and are safe as decommission proof on their own. 0229a immutable labels are SSH/`Node` only, so a
token-joined systemd host running just `kube_service` is legitimately AUTO but gets no immutable marker and
falls to the static-label path. Everything else (non-SSH AUTO hosts, and PREREQ and MANUAL hosts with an
IaC-owned token) carries the marker as static labels in the migrated config. These are node-supplied, so a
wrong or spoofed label could make `verify` associate a TARGET node with the wrong SOURCE server ID. The
static label is a correlation hint, not a security boundary.

What this means for decommission. A TARGET node matching only by hostname is always treated as unknown, never
eligible. A server-enforced marker authorizes decommission on its own. A static-label marker does not, and the
extra step it requires is *not* an independent authorization control: it is a correlation-plus-liveness check
whose security rests entirely on the integrity of the SSH path to an uncompromised SOURCE host. Concretely:

* At enroll, `tmig` plants a per-host nonce (a 0600 file with the run ID and a random value) on the SOURCE host
  over the same SSH session that installs the suffixed agent.
* At decommission, before disabling a static-label host's original unit, `tmig` opens SOURCE SSH to that exact
  host and confirms both the planted nonce and a healthy suffixed install carrying this run's marker. This binds
  the action to the host `tmig` actually enrolled, not to a spoofable TARGET-side label; but the nonce, the
  suffixed install, and the unit all live on that one host over that one SSH path, so whoever can forge the proof
  already controls the host.
* The operator must additionally pass `tmig decommission --allow-static-marker`. Run state records, per host,
  which path retired it, and the report shows marker trust per host.

So the check defends against label spoofing and stale state, not a compromised SOURCE host: an attacker would
have to control the very SOURCE host whose agent would be disabled. A server-enforced non-SSH marker would make
this an actual authorization boundary ([future work](#future-work)).

### Classification

`tmig` gives every SOURCE host exactly one verdict: the contract for the readiness report and for the per-host
account of what can't be automated.

First, three senses of "Kubernetes" this section keeps separate: the join method `kubernetes` (PREREQ,
scopable); the service/role `Kube` (scopable, like `App`, unlike `Db`); and Kubernetes as a deployment platform
(helm/operator agents, MANUAL today). One host can involve any combination.

| Verdict | Which hosts | What happens |
|---|---|---|
| **AUTO** | Token-joined, systemd, `teleport-update` present, config at `/etc/teleport.yaml`, root/sudo via the SSH login, trusted selector, known capability | Fully automatic: mint a single-use scoped token, drive the parallel install, verify, delete the token. |
| **PREREQ** | Delegated joins scoped tokens accept: `iam`, `ec2`, `gcp`, `azure`, `azure_devops`, `oracle`, `kubernetes` | Automatic once a matching scoped token exists in the target scope, created by the scope admin (normally via IaC). If missing, the report emits the exact Terraform/YAML remediation. `tmig` never creates long-lived tokens. |
| **PIPELINE** | Discovery-enrolled hosts | Excluded from per-host SSH. Migrate the *pipeline*: a Discovery Service joined to TARGET in the mapped scope, same matchers, with an install-suffix, so future instances land correctly (pending scope-pinned discovery; see [Future work](#future-work)). Agentless/EICE hosts wait for scoped agentless SSH. |
| **MANUAL** | No systemd, container/supervisor installs, no `teleport-update`, non-standard config, non-Linux, no root path, a join method scoped tokens reject, `bound_keypair`, an untrusted (`node_reported`) selector, a host carrying `Bot`, or a host also running a control-plane service | The report renders the two commands for that host to run out-of-band (except `Bot`; see below); `verify` picks it up. Kubernetes-as-platform agents are MANUAL today. |

**`bound_keypair` is MANUAL in v1**, because it is not a pure config rewrite and its concurrent-second-join
semantics aren't confirmed; see [`bound_keypair` (MANUAL in v1)](#bound_keypair-manual-in-v1).

**`Bot` is out of scope for v1, with no runbook.** A scoped Bot token *can* be minted (`rolesSupportingScopes`
includes `Bot`), but Bot/MWI migration has its own semantics ([RFD 0229b](./0229b-mwi-scopes.md)): a pre-created
scoped Bot, SRA-bound, `usage_mode: bot`, with the `token` method rejected for bot tokens. The "render two
commands, `verify` picks it up" mechanism can't apply, since `verify` keys on a node marker and a Bot isn't an
SSH node. So any host carrying `Bot` routes to MANUAL/out-of-scope with reason "Bot migration is out of scope
(RFD 0229b); no runbook," regardless of join method.

**Mixed control-plane + agent hosts.** A SOURCE host running `auth_service` or `proxy_service` alongside agent
services (common in small self-hosted clusters) routes to MANUAL: the migrated config disables
the control-plane services, so automating it would silently drop that role. The report names this rather than
producing a non-equivalent config.

**Join methods with no scoped equivalent.** Scoped tokens accept only the PREREQ-row methods plus `bound_keypair`;
everything else is rejected by the auth server (`join method "tpm" does not support scoping`). `tmig` doesn't
hand-maintain this list: preflight classifies from the same versioned capability table that drives the
scopable-role set (see [Sourcing scope capability](#sourcing-scope-capability)), with the auth-server rejection at
enroll as the backstop for staleness. Today the rejected set includes `tpm`, `env0`, and the CI methods (`github`,
`gitlab`, `circleci`, `spacelift`, `terraform_cloud`, `bitbucket`). A host on one of these can't migrate
automatically into a scope, even a standard Linux host, so it routes to MANUAL with reason `join method not
scopable`.

Classification runs before anything is touched because some fraction of any real fleet can't be automated, and
operators need that count up front. Non-Linux hosts, env-driven configs, hosts with no root path, and unscopable
join methods always need a human; knowing how many before committing is what makes a fleet migration plannable.

#### Delegated joins are migratable today

Scoped tokens already accept the delegated methods in the PREREQ row, so an IAM-joined fleet is mechanically
migratable now. The source of truth for accepted methods is the auth server's scoped-token join-method allow-list,
not [RFD 0229a](./0229a-scoped-join-tokens.md), which predates current behavior both ways: it deferred
`bound_keypair` (now accepted, though `tmig` keeps it MANUAL in v1) and listed `github` and `tpm` (not accepted).

The remaining question is token ownership. The scoped delegated token must already exist in the target scope,
created by whoever owns that scope's IaC. `tmig` reads the host's join params, searches the target scope for a
scoped token of the same method whose allow-rules and role set cover the host, and either proceeds (rewriting
`join_params` to reference the scoped token name, no secret crossing the wire) or halts the host and emits
remediation. It emits, never applies; IaC stays the owner. (Today `tctl scoped tokens add` only creates
`token`-method tokens, so delegated scoped tokens are defined via resource YAML or Terraform, which is what the
remediation emits.)

#### `bound_keypair` (MANUAL in v1)

`bound_keypair` is accepted by scoped tokens, but unlike a stateless delegated join it is not a pure config
rewrite, and the behavior an automated path would depend on isn't confirmed. So in v1 it is MANUAL: the report
renders the runbook and `verify` picks the host up like any other MANUAL node.

The open behavior is whether provisioning a fresh scoped `bound_keypair` token for the suffixed install cleanly
supports a second concurrent agent with an independent recovery counter. The design would mint a *separate* TARGET
token (it can't share the original's, since the two installs have separate data dirs), which sidesteps the "one
token backing two joins" reading. But the resource carries a single registration secret and a single recovery
mode/limit, and it isn't confirmed that a fresh TARGET token leaves the original's recovery accounting undisturbed.
Rather than ship on an unconfirmed assumption, v1 keeps these hosts MANUAL. To promote it to PREREQ later (tracked
in [Future work](#future-work)): the suffixed install provisions its own TARGET token (pre-registered key or
one-time secret, by file reference); the recovery limit is surfaced in the report so retries don't exhaust it; and
rollback stays stop-and-`uninstall`, leaving the original keypair untouched. This covers agent joins only; Bot/MWI
(`tbot`) semantics differ ([RFD 0229b](./0229b-mwi-scopes.md)).

### Producing the migrated config

Every path above needs the same primitive: produce a modified copy of a node's `teleport.yaml`, on the node,
pointing at TARGET, changing as little as possible and leaving the original in place. It must run locally because
the source config can hold a join secret we shouldn't read back to the operator host. It validates output with
Teleport's own config loader before writing, redacts secrets from anything it prints, and detects listener and log
conflicts between the two side-by-side agents.

Teleport has no transform command today (`teleport configure` only generates or validates), so `tmig` needs one:
the proposed `teleport reconfigure`. Illustratively, over SSH (comments on their own lines so the block is
copy-pasteable):

```console
# default output path is /etc/teleport_<suffix>.yaml
# --token is the token NAME (non-secret); delegated joins omit --token-secret-file
$ teleport reconfigure \
    --input  /etc/teleport.yaml \
    --output /etc/teleport_team-a-mig.yaml \
    --proxy  scoped.example.com:443 \
    --token  scope-migrate-ip-10-2-4-17 \
    --token-secret-file /var/run/tmig-secret \
    --data-dir /var/lib/teleport_team-a-mig \
    --disable-service db,app
```

Most fleet binaries predate any such command, so `tmig` needs a fallback. The preferred one is a pushed node-local
helper: `tmig` copies a small standalone transform helper to the node over SSH and runs it there, so the edit
happens on the host, the source config is never read back, and it works regardless of installed `teleport` version.
The last resort is an operator-side rewrite, only with explicit `--allow-operator-side-rewrite`, recorded as a
separate per-host acceptance; even then `tmig` redacts secrets in memory before anything reaches the report or run
state, never persists raw configs, and flags every host that took this path.

Four edits this primitive must get right, because each can otherwise produce a non-startable or unsafe config:

* **Endpoint exclusivity.** Drop the conflicting `auth_server`/`auth_servers`/`proxy_server`. The v3 config loader
  (`config.ApplyFileConfig`) rejects more than one endpoint, so the rewrite relies on that validation; a test
  asserts the loader rejects the multi-endpoint case the rewrite avoids.
* **Join method present.** A `join_params` block needs a `method`.
* **No mixed token styles.** Introducing `join_params` drops any top-level `auth_token`.
* **CA pin handling.** CA pins validate the Auth Server during registration, so a SOURCE pin left in place fails
  every TARGET join, and clearing pins blindly drops MITM protection on a direct Auth join. Preflight records
  TARGET's CA pin(s); the rewrite replaces SOURCE `ca_pin` values with TARGET's, and clears pins only on a
  documented proxy/TLS-safe join path. Tests cover stale SOURCE pins, TARGET-pin insertion, multiple pin values,
  and redaction in the report.

The edit preserves comments and key order, so the output reviews as a minimal diff.

### Join token handling

| Token class | Who creates it | `tmig` behavior |
|---|---|---|
| Scoped single-use (`token`, scoped target) | `tmig` | Mint a single-use scoped token per host, deterministic name, marker plus source labels (immutable labels for SSH/`Node` hosts). Once consumed, reusable by the same host (matched on public-key fingerprint) until `reusable_until` (~30 min after first use), refused for any other host ([RFD 0229a](./0229a-scoped-join-tokens.md)). Deleted after join. |
| Delegated, long-lived (`iam`/`ec2`/`gcp`/`azure`/`azure_devops`/`oracle`/`kubernetes`) | Scope admin via IaC | Verify existence, allow-rule, and role coverage at preflight; halt and emit remediation if absent. Never created or deleted. (`bound_keypair` is also delegated and scopable but is MANUAL in v1.) |
| Source-cluster tokens (drain) | — | List with detected origin. `teleport.dev/origin=kubernetes` reliably marks operator-created tokens; Terraform vs. manual aren't distinguishable (both `origin=dynamic`), so unknown-origin is possibly-IaC and report-only. Deleting confirmed-manual tokens is opt-in. |

Two timers are easy to confuse on the scoped path: the token's TTL bounds first use, while 0229a's `reusable_until`
(~30 min after first use) bounds same-host retry. Neither gives day-scale idempotency; the
[marker](#the-migration-marker) does, on every path.

### Sourcing scope capability

When a mapping targets a scope, preflight needs two read-only facts about the TARGET *cluster*: are scopes enabled
(`GET /webapi/ping` returns `auth.scopes: "enabled"` and `proxy.scopes_enabled: true`), and which roles and join
methods support scoping. Today the second is a cluster-wide property of the build, not per-scope
(`rolesSupportingScopes` and the join-method allow-list are global), and there is no read-only API. Since preflight
makes no cluster writes and never probes by minting a token, it classifies from a versioned local table keyed by
the TARGET version it sees; the auth-server rejection at enroll backstops staleness, and a TARGET newer than the
table [fails closed](#preflight-checks).

The set is small today: `rolesSupportingScopes` is `{Node, Kube, App, Bot}` and the accepted join methods are a single
allow-list, so the table is two short lists, not a sprawling matrix. Maintaining it still costs something, since
`tmig` versions on its own cadence and must track which versions made which roles and methods scopable; whether
that justifies a dedicated API depends on how often the set actually changes. The long-term option is a read-only,
cluster-wide capability query (`tctl scopes capabilities`, or a `/webapi/ping` extension) returning the supported
set directly. Whether the operator's identity may *create* scoped tokens is a separate RBAC question, proven by the
canary at enroll.

### Scope-support gap

Not every Teleport service is scope-aware yet. Databases and Windows desktops have no scoped story today;
agentless SSH, the Kubernetes operator, and access lists are arriving incrementally. For a
multi-service agent, `tmig` migrates the services TARGET supports with a token whose roles match, and disables the
unsupported ones in the migrated config, leaving the original agent running to serve them. Such a host is reported
as partially migrated and excluded from decommission until those services gain a scoped path. Because the scopable
set changes over time, `tmig` reads it from the versioned capability table rather than hardcoding a list.

### Verification and the instance-inventory dependency

Verification answers one question: did this host land healthy in the right scope? The preferred design answers it
with the operator's scope-pinned identity, listing nodes within the mapped scope and matching them to SOURCE hosts
by marker, which needs no cluster-wide privilege. For SSH nodes that listing uses the scope-aware path backing
`tsh ls`: `ListUnifiedResources` with `KindNode`. It must not use `ListResources` (which rejects node kind for
scoped identities), `GetNodes`, or `tctl inventory ls` / `ListUnifiedInstances`. The last is the agent *instance*
inventory, which is not scope-aware (Agent Inventory UI RFD; `ListUnifiedInstances` authorizes only on unscoped
`KindInstance`/`KindBotInstance` and its filter has no scope field), and scoped audit and session reads aren't
available either. Where `tmig` needs the instance inventory or audit data, it falls back to an unscoped read on
TARGET, which a pure scope admin won't hold.

This is a dependency, not a permanent property (see [Permissions](#permissions-and-the-migrator-role)). Until those
reads are scope-aware, an org either grants the migrator a narrow interim unscoped read-only identity on TARGET, or
has a central admin run `verify`/`decommission` while scope admins run `inventory`/`enroll`.
The post-enroll usage signal needs the same audit read, so it is advisory and may be unavailable to a pure scope
admin; the decommission gate never depends on it.

### Run state and the suffix contract

Sign-off, the `--allow-static-marker` and operator-side-rewrite acceptances, identity pins, planted nonces, per-host
verification, and decommission history all live in local `tmig` run state, and the gates read from it. Because it is
a safety boundary, its properties are pinned down here.

* **Keying.** A run is keyed by SOURCE cluster ID, TARGET cluster ID, mapping, suffix, and run ID; the same tuple
  identifies the state directory, so two runs against different targets or suffixes can't collide.
* **Durability.** State is written 0600 with atomic write-temp-then-rename. Each mutating stage records the hash of
  the effective run config and the readiness report it acted on.
* **Locking.** A stage takes an exclusive lock on the run-state directory for the tuple; a second concurrent `tmig`
  against the same tuple is refused with the holder's identity.
* **Revalidation.** Every mutating stage revalidates the identity pins and config/report hashes before acting,
  refusing on a mismatch. Decommission re-checks verify-green and, for static-label hosts, the on-host proof at the
  moment of action.
* **Loss and tampering.** Read-only stages rebuild their state and `enroll` stays idempotent via the marker, so
  inventory/preflight/enroll survive state loss. Static-label *decommission* authorization does not: it depends on
  the planted-nonce record in durable local state. After state loss a nonce read back from the host is no longer
  proof that *this* run planted it, so static-label hosts can be re-verified for presence but not retired until
  proof is re-established (a re-enroll/replant pass, or a future cluster-side attestation). Server-enforced-marker
  hosts are unaffected, since their authorization lives on the cluster. Hand-edited state is detected by the
  recorded hashes; on a mismatch `tmig` refuses the mutating stage.

The suffix sets the config path, data dir, systemd unit, and marker content, so it is a run-config field. Preflight
inspects each host for an existing suffixed install and hard-blocks unless its marker and run metadata match the
current run exactly; the report shows unit/config/data-dir ownership for any pre-existing suffix. Enroll refuses to
overwrite a suffix it doesn't own, so a re-used or colliding suffix can never clobber another migration's agent.

### Drain and decommission

**In one line:** observe SOURCE first, then cut over only behind the `validate` sign-off; the convergence loop
never hard-blocks on unreachable hosts, but the destructive cutover fails closed on any signal that SOURCE is
still in use, and a blocker clears only with an explicit, recorded waiver.

1. **Monitor.** Watch SOURCE audit events for new joins and token creation, so teams find automation still pointed
   at the old cluster before the plug is pulled.
2. **Report.** List the source token surface with detected origin, and the roles that grant `token:create`. Removing
   that permission via IaC is usually cleaner than deleting tokens IaC will recreate. These drain reads are a
   central-admin path on an unscoped SOURCE (see [Permissions](#permissions-and-the-migrator-role)).
3. **Act (opt-in).** Delete only confirmed-manual tokens, enumerating bots/automation first.
4. **Gate decommission (the `validate` gate).** The requirement is a per-mapping sign-off plus verified presence in
   the target scope. The post-enroll usage signal is advisory, not a blocker. None of this proves access equivalence
   (that needs role diffing, out of scope since teams author roles fresh).
5. **Fail closed on active-SOURCE signals.** `decommission` refuses, by default, on: recent new joins to SOURCE for
   the mapping's hosts, live migration-relevant tokens for the mapping, active sessions on hosts about to be
   disabled, unmigrated services still on the original agent, or an unknown drain-read status. Each blocker is named.
   An operator can waive a specific blocker, but a waiver is explicit and recorded with owner, reason, expiry, and the
   exact blocker waived. Only the destructive step fails closed; the convergence guarantee is unchanged.
6. **Decommission.** Disable original units host by host, only where verify is green, nothing is left on the original
   agent, the mapping passed the gate, and no active-SOURCE blocker remains. A server-enforced marker keys
   decommission directly; a static-label marker additionally needs the on-host proof and `--allow-static-marker` per
   [The migration marker](#the-migration-marker), else those hosts are skipped and reported. To avoid tearing down
   the SSH session mid-command, `tmig` doesn't stop the unit inline: it installs a one-shot detached local job (a
   transient systemd unit) that disables and stops the original unit after the session returns, then reconnects to
   confirm the unit is inactive. A host stays `pending` until that confirmation, so "disabled" in the report means
   observed-inactive. Rollback before this point is always the same: stop the suffixed unit and run
   `teleport-update --install-suffix <s> uninstall`. The original was never touched. After decommission, recovery is
   the inverse: `tmig` only *disables* the original unit and leaves its config and data dir intact, so a cut-over
   host is restored by re-enabling and starting the original unit; `tmig` never deletes a SOURCE install.

**Residual exposure (not closed by the gate).** The gate proves SOURCE was idle at decommission, not that it will
stay idle: recent-join absence is no proof future joins are impossible. On an unscoped SOURCE this is sharper, since
origin detection can't distinguish Terraform-created from manual `dynamic` tokens and the drain read isn't
label-bounded, so "migration-relevant token" is a heuristic (tokens whose roles/labels overlap the mapping, plus
non-expired `token`/static credentials). An autoscaling group, Discovery pipeline, or Terraform apply still pointed
at SOURCE can re-enroll agents after `tmig` disables the current hosts. The durable fix is out-of-band: stop the
SOURCE-side automation, usually by removing `token:create` roles via IaC, before decommission. `tmig` reports the
surface and fails closed on observed signals but doesn't by itself guarantee SOURCE silence; hardening the gate to
require explicit SOURCE-automation attestation is an [open question](#decisions-and-open-questions).

### Optional resource migration

Not the default, and unneeded when teams own roles in IaC. In v1 it is report-only: a read-only listing of what
exists in SOURCE and what has no scoped equivalent, with origin detection. It does not create scoped policy, and
(unlike earlier drafts) it does not emit applyable Terraform/YAML either.

Emitting applyable policy was cut from v1 deliberately. Classic-to-scoped RBAC isn't lossless: scoped roles are a
subset, evaluate differently, and lack classic deny semantics ([RFD 0229](./0229-scopes.md) states scoped roles
don't support classic deny rules). An emitted artifact is designed to be applied by CI, so emitting a translation
without the full conversion matrix, unsupported-construct failures, semantic diffing, and access-broadening analysis
would hand users a dangerous policy migration while preserving a false sense of safety. Report-only can't broaden
access, because it produces nothing applyable. A future emit or direct-create mode, with the complete conversion and
broadening analysis, is deferred to its own RFD (see [Future work](#future-work)).

Access lists are a special case, and the path is blocked on unbuilt work. There is no scoped Access List resource.
[RFD 0243](./0243-scoped-roles-in-access-lists.md) covers *unscoped* Access Lists that grant scoped roles,
materialized into scoped role assignments, and requires those lists to grant roles defined in the root scope `/`. But
defining scoped roles in the root scope is, per 0243 itself, "currently not allowed, we will need to update/allow
this," and [RFD 0229](./0229-scopes.md) reserves root and defers it. So "express access through 0243's model"
describes a path that depends on an unbuilt 0229 change; v1 reports the access-list surface but does not present this
translation as currently available.

### Permissions and the migrator role

`tmig` uses scope-pinned credentials wherever it can. The places it currently can't are explicit dependencies, not
designed-in global grants, and they cluster at the back of the lifecycle, which is why the full migration is a
central-admin operation at both ends in the MVP: TARGET instance-inventory and audit reads (for `verify` and the
advisory usage signal) and SOURCE drain reads (for `drain`) are not scope-aware, so a pure scope admin runs
`inventory`/`enroll` but needs a central admin or interim unscoped grant for the rest.

| Identity | Cluster / scope | Needs | Notes |
|---|---|---|---|
| SOURCE team migrator | SOURCE | `node: list,read`; SSH as `--login` | Label-constrained via `node_labels`; bounds node access and SSH targets only. |
| SOURCE drain reader | SOURCE | `token`/`role`/`event`/`session: list,read` | Not bounded by `node_labels`. On an unscoped legacy SOURCE these reads can see other teams' tokens, roles, and audit/session metadata, so this is a central-admin path unless a server-side label/time-filtered API exists. |
| TARGET token + verify identity | TARGET, **scope-pinned** to the mapping's scope | `scoped_token: create,read,delete` in scope (create exercised by the enroll canary); `node: list,read` within scope (via `ListUnifiedResources`); `event/session: list,read` within scope once scoped audit lands | Acting on scoped tokens and scoped node listing are themselves scoped. |
| *(interim)* TARGET inventory/audit reader | TARGET, unscoped read-only | `node/instance: list,read`, `event/session: list,read` | Only where verification or usage needs the not-yet-scope-aware instance inventory or audit reads. Narrow and time-bounded; drops away once those are scope-aware. Alternative: a central admin runs `verify`/`decommission`. |

Both unscoped reads (TARGET usage and SOURCE drain) can see data beyond the migrated team; `tmig` keeps them narrow
and treats them as central-admin paths. See [Privacy](#privacy).

Teleport should ship a scope-parameterized migrator role template. The MVP doesn't depend on it: scoped-role
templating on a scope variable isn't confirmed (today scoped roles carry a static `assignable_scopes`), so v1 falls
back to hand-authored per-scope roles, or a single `assignable_scopes: ["/**"]` role where a broad migrator is
acceptable. Role *definitions* at `/` are permitted; what's constrained is using `/` as an assignable scope or
scope-of-effect. Both gaps are tracked in the [dependency table](#mvp-boundary-and-dependencies).

### Agent-driven operation

AI agents are first-class users of `tmig`. The whole tool is built to be driven non-interactively
(see [User story 2](#user-story-2--an-automatedci-run)); the CLI contract and the reference skill below
are what let an agent drive a migration end to end while staying inside the safety model.

#### CLI UX

There are two surfaces. The node-local config-transform command ([above](#producing-the-migrated-config)) is a
generic, hidden building block. `tmig` itself is one subcommand per stage plus the `validate` gate (see
[Commands, stages, and gates](#commands-stages-and-gates)); read-only commands need no flag and mutating ones require
`--execute`. Following RFD 0, stdout is the agent's API:

* Every command, including mutating ones, supports `--format=json`, so one call returns the whole picture (verdicts,
  counts, blockers, remediation).
* No interactive prompts; all input comes from the run config or flags.
* Errors are actionable. A blocked PREREQ host names the missing token, the scope, the uncovered account, and the
  remediation.
* Risk acceptance gets its own flag, recorded independently in run state: `validate --confirm`,
  `decommission --allow-static-marker` (the [static-marker risk](#the-migration-marker)), and per-blocker drain
  waivers.
* `--print-config` dumps the effective merged run config; `preflight -o report/` writes the readiness report
  alongside the JSON.

##### Run config and machine-readable output

Because the tool is driven by scripts and agents, the machine contract is part of the spec. The `run.yaml` schema has
a versioned `apiVersion` (shown in both user-story examples), a `target` (proxy, identity, optional scope), a
top-level `suffix`, and a list of `migrations`, each with a `source` and `mappings` (`selector`, `selector_trust`,
optional `scope`); it ships as a JSON Schema so configs validate before a run. JSON output is a versioned API:
`--format=json` emits a top-level `schema` version; per-host objects carry stable identifiers (run ID, SOURCE server
ID, TARGET node ID once known), verdict, marker trust, selector trust, blockers, and remediation; fields are added
compatibly. Exit codes: `0` success; `1` unexpected error; `2` usage/config error; `3` blockers remain, so a CI loop
can branch on "work remains" vs. "broken." The run-state location is reported in every JSON payload, and the
redaction guarantee is part of the output contract, asserted by test.

#### Agent Skills

We'll ship a reference Agent Skill that drives a migration end to end, so an AI agent inherits the safety posture
instead of rediscovering it: read-only stages are always safe; `--execute` only after a clean preflight; emitted
remediation is surfaced for human approval and never applied by the agent; and the run stops at the `validate` gate.
The skill encodes the run-config schema and how to read the `--format=json` verdict output, following RFD 0's
guidance to describe a reference skill for moderately complex workflows.

### Security

SSH-as-root is not a new grant: rendered shell runs as root/sudo on thousands of hosts, but `tmig` reuses the
operator's existing config-management/Teleport access. The control is injection safety: quoting discipline,
shellcheck, and stub-binary tests over every rendered script.

Token secrets never appear in argv, logs, the config body, diffs, or the report; redaction lives in the
config-transform primitive and a regression test asserts no fixture secret survives into output. The operator-side
fallback redacts in memory, never persists raw configs, and is flagged per-host. Token exposure differs by path:
scoped tokens are single-use, short-TTL, scope-pinned, delivered over the SOURCE SSH session, deleted after join, and
post-first-use restricted to the same public key until `reusable_until`; unscoped provision tokens have no fingerprint
binding, so only the short TTL and SSH-session delivery bound misuse. Neither is bound to the source host before first
use.

The rest of the posture is covered in place: trusted-selector gating with `node_reported` downgraded to MANUAL
([Selector trust](#selector-trust-and-auto-eligibility)); marker-based decommission proof and its SSH-path-integrity
caveat, where only server-enforced markers authorize on their own and a compromised SOURCE host can defeat the
static-label check but only for its own agent ([The migration marker](#the-migration-marker)); and the SOURCE≠TARGET
hard-fail ([Preflight checks](#preflight-checks)). Capability data fails closed; gates
read from 0600, atomically-written, per-tuple-locked, hash-validated run state, revalidated on every mutating stage.
All mutation is operator-promoted, `enroll`'s worst case leaves the original untouched, and the SSH identity stays off
TARGET.

### Privacy

`tmig` works with infrastructure metadata (hostnames, UUIDs, labels, join methods, cloud account IDs/ARNs), not
personal data. The PII-adjacent cases are SSH logins and the advisory usage signal in `validate` (aggregate counts
plus a zero-usage flag, local to run state). Two reads can expose data beyond the migrated team (the TARGET usage read
and the SOURCE drain read), so `tmig` time-bounds and filters them where the API allows and treats them as
central-admin paths until scoped audit and filtered reads narrow them. Report redaction is mandatory and asserted by
test. The tool does not phone home.

### Proto Specification

`tmig` needs no proto to ship. Two proposed companions would add proto; this RFD advocates the first. The
`scoped_token.*` audit events (`created`, `used`, `deleted`) are proposed in [0229a](./0229a-scoped-join-tokens.md) but
not implemented (today create/delete emit nothing; joins emit generic `InstanceJoin`). They materially improve
migration auditability and adoption measurement, so this RFD advocates landing them as a near-term server companion.
The second is the read-only scope-capability query (see [Sourcing scope capability](#sourcing-scope-capability)); as an
API it adds a small request/response for the supported roles and methods. A future server-enforced non-SSH marker would
also touch proto (a certificate extension or inventory attestation field), but that is out of scope here.

### Scale

The fleet is thousands of hosts with some always offline, so the design converges rather than looping sequentially:
bounded parallel workers each hold their own control-plane client, each stage attempts all pending hosts, tolerates
unreachability, persists per-host state, and resumes on re-run. The numbers below are initial guesses to be tuned by the
load test, not derived limits. `enroll.parallelism` defaults to 32 workers, gated by per-cluster semaphores (SOURCE SSH
fan-out, TARGET control-plane calls). TARGET token create/delete is rate-limited (default 16/s), deletes batched after
join. Failed hosts retry with exponential backoff plus jitter (base 1s, cap 5m). All list calls paginate (default page
500); audit/usage reads are time-windowed. An unhealthy TARGET cache makes verification provisional and doesn't advance
the gate; Ctrl-C or a deadline drains in-flight workers and persists state for resume. Write load lands at enroll
(preflight makes no TARGET writes); the pressure points are TARGET token churn and SOURCE SSH fan-out, both bounded. The
load-test target is 10,000 hosts with 20% offline, asserting convergence, bounded QPS at both clusters, and clean resume
after a mid-run restart.

### Backward Compatibility

`tmig` is out-of-tree and changes no server schemas or caches; it only creates/deletes ephemeral tokens through existing
APIs. The compatibility surface is the node-local config transform: hosts whose version lacks the command route to the
pushed helper or opt-in fallback (see [Producing the migrated config](#producing-the-migrated-config)), and the rewrite
keeps both v2 and v3 configs loadable after editing (e.g. dropping legacy `auth_servers` for a v3 endpoint). Teleport has
not dropped a config field, so newer binaries parse older configs. Scopes are gated behind `TELEPORT_UNSTABLE_SCOPES` and
not production-ready, making mixed-version fleets a preflight concern that relaxes at GA. Unknown/newer TARGET capability
data fails closed. Trusted/leaf-cluster relationships are out of scope.

### Audit Events

The MVP relies on events that exist today: generic agent join (`InstanceJoin`) on both clusters, scoped-token *resource
state*, and `tmig`'s own local run-state log (per-host stage transitions, promoter, sign-off,
`--allow-static-marker`/drain-waiver records, and server-enforced-marker vs. operator-attestation+on-host-proof
retirements). It does not assume `scoped_token.*` audit events (proposed, not implemented), so drain monitoring uses
`InstanceJoin`, resource state, and the local log until they land. This RFD advocates adding them (see
[Proto Specification](#proto-specification)); once present, drain monitoring and adoption metrics should consume them for
scope-narrowed attribution.

### Observability

Observability is the readiness report plus per-stage counters (`AUTO/PREREQ/PIPELINE/MANUAL`, `migrated/pending/manual`,
blocked PREREQ) emitted to stdout and the report: the numbers an operator reads to decide whether to promote. Per-host
verdicts, marker/selector trust, diffs, and failure reasons are persisted and rendered; nothing fails silently (a dropped
invalid label key gets a per-host report entry). A future long-running controller mode turns the same counters into
metrics and gates promotion on them.

### Product Usage

No phone-home telemetry (see [Privacy](#privacy)). Cluster-side markers (`tmig.teleport.dev/…`) plus generic
`InstanceJoin` events approximate migrated-host counts, but generic joins don't record verdict, validation, or
decommission status; those live in `tmig`'s local run state until a durable cluster-side artifact or `scoped_token.*` /
migration audit events land. A high AUTO:MANUAL ratio (from run state) points at gaps in `teleport-update` coverage or
unscopable join methods, and is the most useful investment signal.

### Test Plan

This is out-of-tree tooling, so there's no in-repo `testplan.md` entry to extend
(`.github/ISSUE_TEMPLATE/testplan.md` covers server features); coverage is the automated suite below, noted explicitly so
the omission is intentional rather than overlooked.

* Unit and fuzz tests over the config-transform edit logic: assert the edit set against fixture v2/v3 configs (endpoint
  exclusivity, empty join method, `auth_token` vs. `join_params`, legacy `auth_servers`, CA-pin replacement), and run
  every rewrite's output back through Teleport's config loader.
* CA-pin handling: stale SOURCE pins replaced with TARGET pins, TARGET-pin insertion where SOURCE had none, multiple pin
  values, the documented pin-clearing path, and pins redacted in the report.
* Redaction regression: no fixture secret appears in any rendered output or report, including the operator-side rewrite
  path.
* shellcheck + stub-binary over every rendered script (MANUAL runbooks, enroll commands, pushed-helper invocation).
* Preflight is read-only on clusters: assert no SOURCE/TARGET writes, reads scopes-enabled from `/webapi/ping`, resolves
  the scopable set from the local table, fails closed on unknown TARGET version, and runs the cross-cluster (hard-fail
  when SOURCE == TARGET) and version/flag checks. The RBAC triad surfaces via the enroll canary; a missing `delete` stops
  enroll before fleet writes.
* Selector trust: a `node_reported` selector is downgraded to MANUAL and never AUTO; trusted sources classify normally.
* Verification uses `ListUnifiedResources` with `KindNode` for scoped callers and does not fall through to
  `ListResources` / `GetNodes` / `tctl inventory ls`.
* Marker trust and on-host proof: a static-label-only host is refused at `decommission` without a `validate` sign-off,
  `--allow-static-marker`, and the on-host nonce/suffixed-install proof; a server-enforced-marker host needs none of the
  extras; and after run-state loss a static-label host is refused retirement until proof is re-established.
* Run state and suffix: atomic write/lock behavior, stale-verification refusal, hash-mismatch refusal, concurrent-run
  refusal, and enroll refusing to overwrite a foreign suffix.
* Drain fail-closed: decommission refuses on recent SOURCE joins/token-creates, active sessions, unmigrated services, and
  unknown drain-read status; a recorded waiver releases exactly the named blocker; and "disabled" is recorded only after
  the detached job confirms the unit inactive.
* End-to-end (scoped): AUTO idempotency, convergence (inject unreachable hosts), PREREQ halt-and-resume, `join method not
  scopable` MANUAL routing, `bound_keypair` MANUAL routing, service disable, the static-marker plus on-host-proof
  decommission path for non-SSH/PREREQ hosts, scope-pinned verify/decommission, and rollback.

## Decisions and open questions

A few choices here are deliberate trade-offs or rest on things not yet confirmed. Collecting them so reviewers can push
on the weak points directly.

1. **Self-service is partial in the MVP.** A scope admin can run `inventory`/`enroll` scope-pinned, but
   `verify`/`drain`/`decommission` need reads that aren't scope-aware yet, so the full lifecycle is a central-admin
   operation at both ends until those reads land. Decided: ship the partial model now. Open: when those reads arrive.
2. **Static-label decommission is correlation + liveness, not authorization.** The on-host nonce proof binds the action
   to the enrolled host but lives on that host over the same SSH path, so a compromised SOURCE host can defeat it (only
   for its own agent). Decided: accept it with `--allow-static-marker`, since the alternative is no automated
   decommission for non-SSH hosts. A server-enforced non-SSH marker closes it.
3. **`bound_keypair` is MANUAL in v1.** Whether a fresh TARGET token cleanly backs a concurrent second join with
   independent recovery accounting isn't confirmed. Decided: keep it MANUAL rather than ship on an unconfirmed
   assumption; promote to PREREQ once confirmed.
4. **The 0243 access-list path is blocked on unbuilt 0229 work** (root-scope role *definitions*). v1 reports the surface
   but doesn't present the translation as available.
5. **The drain gate proves SOURCE idle, not future-silent.** IaC/autoscaling/Discovery still pointed at SOURCE can
   re-enroll after decommission. Decided for v1: report the surface, fail closed on observed signals, and document that
   stopping SOURCE-side automation is out-of-band. Open: whether to harden the gate to require explicit attestation, and
   how to define "migration-relevant token" on an unscoped SOURCE.

## Alternatives considered

* Swapping the config in place instead of standing up a side-by-side suffixed install is simpler, but a
  single restart cuts over with no validation window and no rollback. The
  [suffixed install](#per-host-mechanics) keeps the original serving SOURCE until decommission, so
  `enroll`'s worst case is recoverable by stopping the new unit; the added complexity (second data dir,
  marker correlation, the suffix contract) buys the safety boundary the whole design rests on.
* Cloning the cluster by copying backend state (users, roles, audit history, recordings, trust) instead
  of re-enrolling agents would make `tmig` a full cluster migrator. A cluster is not a unit you can copy,
  and scoped RBAC isn't a lossless target for classic roles (see
  [Optional resource migration](#optional-resource-migration)), so `tmig` deliberately owns only the
  agent-fleet half and reports the rest as out-of-band (see [Non-goals](#non-goals)).
* Probing scope capability by minting a throwaway token at preflight, instead of reading a versioned
  local table, would be authoritative, but preflight must make no cluster writes and probing per host
  doesn't scale. Preflight classifies from the [versioned table](#sourcing-scope-capability) (fail-closed
  on an unknown TARGET version), and the single [RBAC canary](#preflight-checks) at enroll proves
  create/read/delete once, with the auth-server rejection as the staleness backstop.
* Blocking v1 on scope-aware reads and a read-only capability API, instead of shipping on workarounds,
  would stall the tool on unbuilt server work. v1 ships instead on a versioned table plus an interim
  unscoped reader (or central admin) for the back of the lifecycle, with every gap and its removal
  tracked in the [dependency table](#mvp-boundary-and-dependencies).
* Emitting applyable RBAC/Terraform, instead of a report-only resource listing, was in an earlier draft
  and was cut: a classic-to-scoped translation applied by CI without the full conversion matrix and
  broadening analysis is a dangerous policy migration with a false sense of safety (see
  [Optional resource migration](#optional-resource-migration)). v1 is report-only; emit/direct-create is
  deferred to its own RFD.
* Folding the stages into a `tctl` subcommand instead of a standalone `tmig` binary is a reasonable
  alternative; `tmig` is a working name and the stage model carries over unchanged, so the choice is left
  to implementation time (see [Future work](#future-work)).

## Future work

* **Additional migration modes** — scope-to-scope and unscoped-to-unscoped re-enrollment (version upgrades,
  Cloud-to-self-hosted) on the same stage model. An unscoped TARGET turns off the scope-specific behavior (scope
  checks skipped, ordinary provision tokens, static-label-only marker); scope-to-scope adds the guarded
  same-cluster exception to the SOURCE≠TARGET hard-fail. Out of scope for v1.
* **`scoped_token.*` audit events** — the cleanest source for drain attribution and adoption metrics.
* **Read-only scope-capability query** (`tctl scopes capabilities`, cluster-wide), so preflight needs no local table and
  unknown-version runs need no override.
* **Scope-aware instance inventory and audit/session reads** — removes the interim unscoped TARGET reader and lets
  verification and the usage signal run fully scope-pinned, closing the self-service gap.
* **Server-side filtered SOURCE drain reads** — a label/time-filtered token/role/event API, so team-scoped SOURCE drain
  doesn't need a central reader, plus a stronger SOURCE-automation attestation for the gate.
* **Server-enforced marker for non-SSH agents** — a certificate extension, inventory attestation field, or extension of
  0229a immutable labels beyond SSH, so decommission keys on a trustworthy marker without the on-host-proof fallback (and
  survives local state loss).
* **`bound_keypair` second-cluster lifecycle** — confirm the registration/recovery semantics for a concurrent second
  join, so it can move from MANUAL to PREREQ.
* **Discovery pipeline migration** — depends on the Discovery Service running scope-pinned end to end.
* **Resource migration (emit / direct-create)** — a fuller classic-to-scoped conversion with a complete conversion matrix
  and broadening analysis, in its own RFD if demand grows.
* **Tool naming.** `tmig` is the working name. If a `tctl` subcommand is preferred at implementation time, the stage model
  carries over unchanged.