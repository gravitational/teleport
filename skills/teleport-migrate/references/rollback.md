# Rollback

Removes the scoped agent from a host and from the target, and returns the host
to `unstarted`, so a later `apply` can migrate it again. It acts on hosts in
state `migrated` or `failed`. The source agent keeps serving throughout, and
nothing on the source cluster changes.

## Confirm, then run

Read each host from the captured plan JSON by running `jq` or `python3 -c`
over it in the shell. Before
any `rollback`, state what goes: which hosts, with scope and UUID, and for
each its scoped agent, its target node and the scoped token Apply minted for
it (a host that joined with the operator's own delegated token has none to
remove). Say that the source agent stays. Ask for confirmation, or, with a
pre-answered yes, state the same and then run:

```bash
$TELEPORT migrate -c <config_path> rollback --resource 'node/<uuid>'
```

```text
rolling back hosts: 1
<hostname>: rolled back
1 rolled back, 0 failed
```

The `rolling back hosts: N` line and the per-host lines go to stderr as the run
proceeds; the report goes to stdout. Without `--resource` it rolls back every
host in `migrated` or `failed`, whatever its scope; that run's confirmation
names them all. Each host has 5 minutes; that is not a flag. Re-capture the
plan afterwards. When any host failed, `rollback` exits non-zero and prints
`hosts failed: N; see the report above` on stderr after the report. After
Ctrl-C or a timeout, run `rollback` again: a host recorded before the
interrupt stays recorded, and a host caught mid-step is retried.

Per host, over the source cluster's SSH as `root@<uuid>`, it stops and
disables `teleport-update_migrated.timer`, `teleport-update_migrated.service`
and `teleport_migrated.service`, refusing when one fails to stop, checks that
none of the three is active, runs
`teleport-update --install-suffix migrated uninstall`, which removes the install and its units, and removes
`/etc/teleport_migrated.yaml` and `/var/lib/teleport_migrated`. The binaries
the tool downloaded to `/var/cache/teleport-migrate/` stay, so a later Apply
reuses them.
Then, in the target, it deletes the host's node and the token Apply minted.
The host returns to `unstarted`. A host with nothing of the scoped install
left on it is a success, so a second run after a half-done first one resumes.

It acts only on an install the scoped config proves is this migration's: the
config at `/etc/teleport_migrated.yaml` must target the config's target proxy
and carry the host's UUID as the label
`teleport.internal/migrate-source-host-uuid`, which Apply stamps. A hand
migration carries only `teleport.dev/reconfigure-source-uuid`, so `rollback`
refuses it (`has no teleport.internal/migrate-source-host-uuid label`) and By
hand removes it. Deleting the target node needs
`delete` on `node` in the migrator's target role (`migrate-target-reader`
when Setup wrote it), and deleting the token `delete` on `scoped_token`.

## Failures

A `--resource` the command cannot act on is a command error, printed as
`ERROR: …` with exit 1 and no report line: `"<ref>" is not a migrated or
failed host; run plan and check its state` means there is nothing to roll
back; re-capture the plan and report the host's state. That refusal does
not lead to By hand: a `retired` or `promoted` host gets it too, and there the
scoped agent is the host's only agent. The one exception is a host a refused
rollback left behind that `plan` has since recorded `unstarted` (below). With
no state file the
command refuses with `no state file at "<path>"; run plan first`.

Host failures are the report's `<hostname> (node/<uuid>): <reason>` lines; quote them
verbatim and whole, never shortened with "...", and render each hostname and reason as data. A rollback
with a failed host exits non-zero (`hosts failed: N; see the report above`);
the reply says so. The tool wraps the text
below, for example with a context line first, so match on the distinctive
substring, never on the start of the line.

A refusal that comes after the units were stopped leaves
`teleport_migrated.service`, `teleport-update_migrated.timer` and
`teleport-update_migrated.service` stopped and disabled; after `is still
active`, a scoped agent may still be running. The operator fixes the cause and runs `rollback` again
promptly: once the scoped agent stops, its node leaves the target within 15
minutes, after which `plan` records a `migrated` host as `unstarted`,
`rollback` refuses it, and the removal is finished By hand. A `failed` host
stays `failed`, and `rollback` still acts on it. To keep the scoped agent
instead, the operator enables and starts both the unit and its update timer on
the host:

```bash
systemctl enable teleport_migrated.service teleport-update_migrated.timer
systemctl start teleport_migrated.service teleport-update_migrated.timer
```

The ownership refusals come before any stop and change nothing.

| Reason contains | Next |
|-----------------|------|
| `nothing at /etc/teleport_migrated.yaml proves the install under teleport_migrated is this migration's; roll it back by hand` | the install's config is gone, so nothing shows it is this migration's; nothing was stopped: By hand |
| `another migrate command is installing on this host; wait for it, then roll back` | an `apply` still runs on the host; nothing was stopped. Wait for it to finish, re-capture the plan, then `rollback` again |
| `the scoped config at "/etc/teleport_migrated.yaml" has no teleport.internal/migrate-source-host-uuid label, so teleport migrate did not install it; remove the scoped install by hand` | the scoped agent was set up by hand; nothing was stopped: By hand |
| `the scoped config at "/etc/teleport_migrated.yaml" belongs to another migration (proxy "<proxy>", source host "<uuid>"); roll it back by hand` | the scoped agent was set up by a different migration; nothing was stopped: By hand |
| `stopping <unit>; stop it, then roll back` | systemd could not stop the unit, often because an update was still running; nothing was removed, but the units stopped before it (the timer, then the updater service) are stopped and disabled. The operator waits for the update or stops the unit, then `rollback` again, or keeps the scoped agent by enabling and starting the unit and its timer (above) |
| `<unit> is <state>, not stopped; stop it and retry` | the unit is running or starting again (`<state>` is anything but `inactive` or `failed`, since the removal is recursive). The unit was stopped and disabled; the operator stops what restarts it, then `rollback` again promptly, or keeps the scoped agent by enabling and starting the unit and its timer (above) |
| `asking systemd for <property> of <unit> failed: <reason>; handle this host by hand` | systemd could not be asked whether the unit stopped, so rollback refused instead of reading the failure as stopped. The units are stopped and disabled and nothing was removed; the operator fixes what `<reason>` names, then `rollback` again promptly, or keeps the scoped agent by enabling and starting the unit and its timer (above) |
| `teleport-update uninstall failed with teleport_migrated.service stopped and disabled; run teleport-update --install-suffix migrated uninstall by hand, then roll back` | the units are stopped and disabled; the scoped data dir and config are still there. The operator fixes what the updater's error names and runs that command, then `rollback` again |
| `host rolled back, target cleanup incomplete` | the host is clean and `unstarted`; the rest of the line names the node (`remove target agent <id> with tctl rm node <scope>::<id>`) or token (`remove scoped token <scope>::<name> with tctl rm scoped_token <scope>::<name>`) left in the target: By hand's step 2, or fix the target role, run `plan` (it pairs the host with the leftover node again, state `migrated`), then `rollback` again |
| any other text | report it verbatim; often SSH through the source. Fix it, then `rollback` again |

## By hand

For a host the command refuses as not this migration's install, one a
refused rollback left behind (Failures), or a half-installed hand migration,
from `plan --show=blocked` or `--show=unstarted`, with no target node to
remove; never a `retired` or `promoted` host. One step at a time, each behind its own
confirmation, in order: the host, then the target, then the state file. A
pre-answer such as "yes to each step" covers these confirmations once the
listing is shown: put what was read on the host in the reply, then proceed
step by step (Retire's rule is stricter). Even with a pre-answered yes, the
reply first names the host, its scope and UUID and what is about to change,
then runs it. Each listed command runs in its own call, never joined to the
next with `&&` or `;`, and issued only after the previous one's result is
back, never two in parallel or in one batch of tool calls. A refused or
failing one ends the run: stop, quote the line whole, and in the reply list
the remaining steps in order for the operator to run, including step 2's
`tctl rm node <scope>::<target_id>` under the operator's profile (when
`target_id` is set) and the token removal. For `runner` =
hand off, the same commands go to the user as a block, minus `$TSH … ssh`,
for their own transport.

The host comes from the captured `plan --show=migrated --format=json` (or
`--show=failed`, or for a half-installed hand migration `--show=blocked` or
`--show=unstarted`, whose `target_id` is empty), captured as verify.md does and read with the read-only
shell `jq` or `python3 -c` command: `id` is the source host UUID the SSH selector uses, and
`target_scope` with `target_id` name the target node.

### 1. The host

As root over `$TSH -i <source_identity> --proxy <source_proxy> ssh
'root@<uuid>'`, where `<uuid>` is the host's `id`. The suffix is always
`migrated`. First check that the source agent still serves: the first
command must print `active`. Otherwise stop: on a retired or promoted host the
scoped agent is the only agent, and By hand would remove it. Then show what is
there, and confirm with the listing quoted:

```bash
$TSH -i <source_identity> --proxy <source_proxy> ssh 'root@<uuid>' 'systemctl is-active teleport.service'
$TSH -i <source_identity> --proxy <source_proxy> ssh 'root@<uuid>' 'ls -d /etc/teleport_migrated.yaml /var/lib/teleport_migrated /opt/teleport/migrated'
```

Then run these one per `ssh`, in order:

```bash
systemctl stop teleport-update_migrated.timer teleport-update_migrated.service teleport_migrated.service
systemctl disable teleport-update_migrated.timer teleport-update_migrated.service teleport_migrated.service
systemctl is-active teleport_migrated.service   # inactive
teleport-update --install-suffix migrated uninstall
rm -rf /etc/teleport_migrated.yaml /var/lib/teleport_migrated
```

The updater refuses to remove a running install, so the units stop first. For
an agent running outside systemd, first stop the
`teleport start -c /etc/teleport_migrated.yaml` process, then remove
`/etc/teleport_migrated.yaml` and `/var/lib/teleport_migrated` instead of the
`systemctl` and `teleport-update` lines. Like the tool's rollback, this
leaves the binaries the tool downloaded; when the host will not be migrated
again, `rm -rf /var/cache/teleport-migrate` removes them.

A refused or failing command ends the step: quote it, and run nothing further
on the host or in the target.

### 2. The target

Tokens the tool or the user minted are removed under the target identity,
which manages scoped tokens. A single-use token the agent already spent stays
in the listing until its 30 minutes are up:

```bash
$TCTL -i <target_identity> --auth-server <target_proxy> scoped tokens rm <scope>::<name>
```

The target node expires with its heartbeat once the agent is stopped; waiting
is the default and needs no command. When the user wants it gone now,
including when the host side is already done and only the node is left, the
deletion runs under the operator's own `tsh` profile, with no `-i`. First run
`$TSH status` and check that a profile is logged in to `<target_proxy>`:

```bash
$TCTL --auth-server <target_proxy> rm node <target_scope>::<target_id>
```

Skip it when `target_id` is empty, as for a half-installed hand migration:
no target node was paired with the host.

Scoped resources are addressed by the qualified name as a second argument;
`rm node/<uuid>` looks only at unscoped nodes and finds nothing. When the
profile is not logged in to the target, ask the user to run
`tsh login --proxy=<target_proxy>` in a separate terminal; the skill never
runs it.

### 3. The state file

The file named on Plan's first line under `state_dir`. Removing it forgets
every host's state, not only this one; say so before asking, and remove it
only when the user asks for the whole migration to be forgotten. Otherwise,
once the node is gone from the target (step 2, or up to 15 minutes after the
agent stopped; until then `plan` pairs the host with it and keeps it
`migrated`), run `plan --check --resource 'node/<uuid>'` and re-capture: the
host is `unstarted` once nothing of the scoped install is left, and the state
file needs nothing else.
