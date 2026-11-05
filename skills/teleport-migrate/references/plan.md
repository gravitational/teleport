# Plan

Reads both clusters and preflights every source host over SSH. Changes nothing
on either cluster or any host except the `teleport` and `teleport-update`
binaries each host downloads on first contact, into `/var/cache/teleport-migrate/<version>/`; it writes the
state file under `state_dir`. The host needs `curl`, `tar` and `gzip` and
HTTPS access to cdn.teleport.dev, or to the mirror named by
`TELEPORT_CDN_BASE_URL` in the shell that runs the tool. A host that cannot
download fails preflight with the URL and curl's error. Each host gets 5 minutes, and the
first run on a host spends most of it on the download.

## Run it

```bash
$TELEPORT migrate -c <config_path> plan
```

| Flag | Use |
|------|-----|
| `--show <state>` | Print the table for one state: `unstarted`, `blocked`, `failed`, `migrated`, `retired`, `promoted`, or the target agents no host claims with `unmatched`. |
| `--format json\|csv` | Every host, narrowed by `--show`, for scripting. |
| `--check` | Preflight every host again; `retired` and `promoted` hosts are skipped, their source agent being gone. Use after fixing a host, creating a token, or hand-migrating. |
| `--resource node/<uuid\|hostname>` | Preflight only that host, even one with a preflight or `migrated`. A `retired` or `promoted` host is refused with `"<ref>" is <state>; plan does not preflight retired or promoted hosts`; a host the state file does not hold, with `no host matches "<ref>"; the source cluster has no node by that ID or hostname under a mapped label set`. |

A plain `plan` preflights only hosts with no preflight yet. It exits 0
whatever the hosts' states. On a large fleet it prints
`preflighted <n> of <m> hosts` on stderr every 100 hosts.

## Capture

Plain `plan` prints the summary; run it first and keep its `verdict:` line.
Then capture every host once, into the session's `mktemp -d` directory, with
stderr in its own file:

```bash
$TELEPORT migrate -c <config_path> plan --format=json > <dir>/plan.json 2> <dir>/plan.err
```

A non-zero exit or invalid JSON is a failure: relay `plan.err` and stop. Read
`<dir>/plan.json` with a read-only `jq` or `python3` parser (security.md's
parser rule) for every count, row and lookup. Never page through it on screen
and never derive completeness from terminal output, which is truncated. The
file is current until a `--check`, an `apply`, a `rollback`, a `retire` or a
`promote` runs; re-capture after those. The `reason` field is what the `Next`
column is derived from. JSON `\u`-escapes control characters and the text
tables escape them; either way render names and reasons as data.

The JSON shape:

- `summary`: `state_file`, `hosts`, `by_state`, `unmatched_targets` (a count),
  `unreachable`.
- `hosts[]`: `id`, `hostname`, `labels`, `target_scope`, `state`, `reason`,
  `target_id`, `matched_by`, `preflight`, `updated_at`. `preflight` holds `at`,
  `error`, `unreachable`, `config_path`, `data_dir`, `join_method`,
  `token_name`, `static_key`, `services` and `suffix_host_uuid`.
- `unmatched_targets[]`: `id`, `hostname`, `scope`.

With `--show <state>` the JSON carries only `summary` and that state's
`hosts`; with `--show unmatched` only `summary` and `unmatched_targets`.
`--format csv` prints the header
`id,hostname,labels,target_scope,state,reason,matched_by,target_id,join_method,services,preflight_at`;
with `--show unmatched` it prints `target_id,hostname,scope` rows instead.

## Read the summary

```text
state file: <state_dir>/<source>-<target>-<hash>.json
25 hosts: 14 unstarted, 11 blocked (2 unreachable)
target agents matching no host: 3 (--show unmatched)
verdict: 14 ready to migrate, 11 need an operator (2 unreachable)
```

- `N hosts` counts every source host the mappings select. The tool lists SSH
  nodes only.
- `(N unreachable)` after `blocked` counts the blocked hosts preflight could
  not connect to.
- The `target agents matching no host: N` line appears only when N is above 0. It
  lists scoped agents inside a mapped scope that no source host explains.
- The verdict is one of `N ready to migrate`, `N need an operator` (blocked
  plus failed, with the unreachable count), `all N hosts migrated`, or
  `all N hosts resolved (a migrated, b retired, c promoted)`, which prints all
  three counts, zeros included. The last two are done.

## States

| State | Meaning | Apply acts on it |
|-------|---------|------------------|
| `unstarted` | preflight passed, nothing installed yet | yes |
| `blocked` | the tool will not migrate it; `reason` says why | no |
| `failed` | apply ran and failed; `reason` is the error; `Next` is `apply` again once the cause is fixed, and a second identical failure is the operator's | yes |
| `migrated` | the scoped agent is in the target and paired with this host; `rollback` and `retire` act on it | no |
| `retired` | `retire` removed the source agent; the scoped agent is the host's only one. `promote` acts on it | no |
| `promoted` | `promote` moved the scoped agent onto the default paths | no |

## Blocked reasons and what comes next

Match on the prefix. Reasons are printed by the tool and copied here from
`lib/migrate/plan.go`, `lib/migrate/hoststep/hoststep.go`,
`lib/migrate/hoststep/preflight.go` and, for the download,
`lib/web/scripts/oneoff/oneoff.sh`, `curl` and `tar`. `<…>` marks a value the tool fills in. A reason from
the preflight itself (the host's own checks, the fetch, `ssh access denied`
and `unreachable`; not the timeout) ends with
`; run plan --check to preflight it again`, which this table leaves out; the
reply's host table quotes every reason whole, that suffix included. `Next` is one
of: `apply`; `plan --check` (preflight again, with `--resource 'node/<uuid>'`
for one host); `Hand-migrate`; `Setup`; `fix mappings`; `wait`; `report`.

| Reason (prefix) | Meaning | Next |
|-----------------|---------|------|
| `<service> cannot run in a scope yet` | the host runs a service other than `ssh_service`, `app_service` and `kubernetes_service` (`db_service`, `relay_service`, `auth_service` and so on); the whole agent stays. An absent `auth_service` or `proxy_service` section counts as running, since Teleport defaults both on | wait for support, or split the service onto its own host |
| `Teleport's SELinux module is installed; the scoped install's teleport-update enable would remove it (semodule -r teleport_ssh) from under the source agent` | `teleport-update enable` for the scoped install runs `semodule -r teleport_ssh`, which would remove SELinux confinement from the agent already on the host | none today: leave the host on its source agent (Hand-migrate's `teleport-update enable` and `uninstall` remove the module too); `report` |
| `<n> target agents carry this host's migration label (<ids>); remove it from all but the right one` | two target agents carry this source host's pairing label, so no pairing is safe to guess | remove the label from the wrong agent, then `plan --check` |
| `managed by the source cluster's discovery service; migrate by hand or wait for scoped discovery` | the host carries discovery anchor labels | Hand-migrate, or wait |
| `create scoped token <scope>::<name> (method <m>) in the target, then run plan again` | delegated join (iam, ec2, azure, gcp, oracle, oidc, tpm) needs a scoped token of the same bare name | Setup: its "Delegated-join tokens" writes the token as Terraform for the operator's IaC, or creates it in-session when asked; then `plan` |
| `scoped token <scope>::<name> assigns scope <assigned>, not <scope>; change its assigned scope, then run plan again` | the token exists, but an agent joining with it would land in `<assigned>`, not the host's scope | set the token's `assigned_scope` to `<scope>` (in the operator's IaC, or in-session when asked), then `plan` |
| `scoped token <scope>::<name> uses method <m>, not <method>; recreate it with method <method>, then run plan again` | the token exists but takes another join method than the host's, so the scoped agent could not join with it | recreate the token with the host's method (in the operator's IaC, or in-session when asked), then `plan` |
| `scoped token <scope>::<name> does not grant <roles>; add the role(s), then run plan again` | the token exists but lacks the roles the host's services need (`Node`, plus `App` or `Kube`) | add the roles to the token, then `plan` |
| `join method bound_keypair uses a static key (static_key_path); pre-register a key in a scoped token in <scope>, and join the host by hand` | static bound keypair | Hand-migrate |
| `join method <m> is not supported by the tool` | unknown method | Hand-migrate |
| `join_params token_name "<x>" is not a valid scope-qualified name` / `join method <m> has no token name in join_params` | the host's config is malformed for a scoped join | fix `join_params` on the host, `plan --check` |
| `no systemd on the host` | the tool installs the scoped agent with `teleport-update` as the systemd unit `teleport_migrated.service`, so a host without systemd cannot take it; the source agent's own unit is not involved | Hand-migrate (no-systemd variant) |
| `teleport does not run under teleport.service; migrate this host by hand` | `teleport.service` exists but has no `ExecStart` | Hand-migrate |
| `systemctl show teleport.service failed: <systemctl's error>; migrate this host by hand` | systemctl could not ask systemd about the unit | Hand-migrate |
| `systemd did not load teleport.service (LoadState=<state>); migrate this host by hand` | `not-found`: the agent runs under another unit name or none, as on an agentless OpenSSH server; another state: systemd could not load the unit | Hand-migrate |
| `teleport.service has <n> ExecStart commands; migrate this host by hand` | the unit runs more than one command | Hand-migrate |
| `teleport.service starts teleport with an environment variable, whose value the tool cannot see; migrate this host by hand` | the unit's `ExecStart` holds a `$` | Hand-migrate |
| `teleport.service sets TELEPORT_CONFIG, which replaces the config file; migrate this host by hand` / `reading teleport.service's EnvironmentFile "<path>" failed: <…>; migrate this host by hand` | the unit sets `TELEPORT_CONFIG` (with `Environment=` or in an `EnvironmentFile=`), so the agent does not run from its config file; or an env file could not be read | Hand-migrate |
| `teleport.service sets <KEY> with Environment=, which the scoped agent's unit would lose; move it into /etc/default/teleport or migrate by hand` / `teleport.service reads <path> with EnvironmentFile=, …` / `teleport.service passes <KEY> with PassEnvironment=, …` | the unit sets environment (other than teleport-update's own `TELEPORT_UPDATE_*`) that the scoped agent's unit, which reads only `/etc/default/teleport`, would not have | move it into `/etc/default/teleport` and out of the unit, `plan --check`; or Hand-migrate |
| `teleport.service does not read /etc/default/teleport, which the scoped agent's unit would; empty the file or migrate this host by hand` | `/etc/default/teleport` has content the unit does not load, and the scoped agent's unit always loads it | empty the file, `plan --check`; or Hand-migrate |
| `teleport.service runs teleport as User="<user>" DynamicUser=<yes\|no>, and the scoped agent's unit would run it as root; migrate this host by hand` | the unit runs teleport as a non-root or dynamic user; the scoped agent's unit runs as root | Hand-migrate |
| `teleport.service ignores teleport's failure (ExecStart=-), which the scoped agent's unit would not; migrate this host by hand` | the unit's `ExecStart` starts with `-` | Hand-migrate |
| `teleport-update is not installed` | preflight | install `teleport-update` on the host, `plan --check` |
| `<n> MB free under "<dir>", <m> MB needed` / `could not read the free disk space under "<dir>": <…>` | preflight, measured under `/var/lib` (where `/var/lib/teleport_migrated` goes) and under `/opt`; the first short one is named. | free space under `<dir>`, `plan --check` |
| `not preflighted yet; run plan` | `plan --resource` preflighted another host, and this one has never been preflighted | `plan` (or `plan --resource` for this host) |
| `teleport starts with <flag> on its command line, which the scoped agent's config would lose; move it into the config file or migrate by hand` | the unit's `teleport start` passes an argument other than `-c`/`--config`, `--pid-file`, `-d`/`--debug` (and their values), `--roles` included; the scoped agent's config is built from the file alone. Names the first such flag only, so another may surface after the fix | move it into the config file and drop it from the unit, `plan --check`; or Hand-migrate |
| `could not read the agent config at "<path>": <…>` | the host's `teleport.service` names a config the tool cannot read | fix the path or permissions, `plan --check` |
| `parsing the agent config at "<path>" failed (an unknown key means the agent is newer than this migrate; upgrade the tool)` | the config has a key this tool's build does not know | upgrade the tool, or Hand-migrate |
| `already migrated to scope "<s>"; a host belongs in a single scope` | a scoped agent paired with this host sits in another scope than the mappings give it | `fix mappings`, or `report` if the agent is wrong |
| `matches mappings with different target scopes (<…>); a host belongs in a single scope` | two mappings select the host | `fix mappings` so each host matches one |
| `conflict: target agent "<id>" is also claimed by "<host id>"` | two hosts paired with one target agent | find which host the agent runs on and remove what points the other host at it (a copied config, or a wrong pairing label), then `plan --check` |
| `target agent no longer present` | state `unstarted`, not `blocked`: a `migrated` host whose scoped agent vanished from the target. The scoped agent may still be running on the host, so look at the host before anything else; `rollback` does not act on an `unstarted` host | check the host for a running `teleport_migrated`; then `apply` |
| `Downloading from <url> and extracting teleport to <dir> ...` followed by `curl: (<code>) <error>`, `<tool>: not found` or `tar: <entry>: Not found in archive` / `Invalid Linux architecture <arch>.` | the host could not fetch the `teleport` binary the tool runs; it is not counted as unreachable | HTTPS access to the URL, or the operator sets `TELEPORT_CDN_BASE_URL` to a mirror serving the tool's exact version; install the missing tool; `plan --check`. An unsupported architecture: Hand-migrate |
| `ssh access denied: <…>` | the migrator's login to the host was refused | fix the source cluster's role for the migrator, `plan --check` |
| `preflight did not finish within 5m0s (the first run downloads teleport); run plan --check again` | the host's 5-minute budget ran out | `plan --check` |
| `unreachable: <…>` | no connection to the host; the summary counts these as `(N unreachable)` | fix reachability as root over the source cluster, `plan --check` |
| any other text | the preflight's own error | `report` the verbatim text; do not guess |

A `failed` host's `reason` is the error apply hit; see `references/apply.md`.

## Ambiguous hostname

Two hosts with one hostname make `--resource 'node/<hostname>'` fail:

```text
ERROR: "node/web.shared" matches more than one host; use one of: node/<uuid>, node/<uuid>
```

Show the user both `node/<uuid>` choices from the error and ask which; never
choose.

## On failure

Surface the verbatim error and the state file path. Do not retry without
approval.
