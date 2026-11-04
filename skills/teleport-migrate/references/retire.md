# Retire

Removes the source agent from each migrated host and its node from the source
cluster, leaving the scoped agent as the host's only agent. It cannot be
undone: the source agent's identity goes with its data dir. The host becomes
`retired`, which plan counts as resolved and Rollback does not act on.

Retire reaches each host as root through the target cluster, through the
scoped agent itself: a session through the source would ride the agent it
removes. Reaching the host that way is the first proof that the scoped agent
stands alone. It needs `delete` on `node` in the migrator's source role and
`logins: [root]` in its target role (`migrate-source-reader` and
`migrate-target-reader` when Setup wrote them).

## Confirm, then one host

Read each host from the captured plan JSON (`plan --show=migrated
--format=json`) by running `jq` or `python3 -c` over it in the shell. Retire refuses a host with no
preflight facts, and it takes the services from them; when the host changed
since its last preflight, run `plan --check --resource 'node/<uuid>'` and
re-capture first.

The question, and the message before a run the request already said yes to,
contains every item:

1. Each host the run retires: hostname, UUID, scope and target agent.
2. Retire cannot be undone.
3. What goes: `teleport.service`, the config it starts with (the host's
   `preflight.config_path`), the data dir that config names
   (`preflight.data_dir`; the source identity goes with it), and the source
   node.
4. Config management (Ansible, Puppet, Chef) must already manage the scoped
   agent's files, `/etc/teleport_migrated.yaml` and `teleport_migrated.service`,
   and no longer the source's; a run that still writes `/etc/teleport.yaml` or
   starts `teleport.service` brings a source agent back.
5. A teleport package installed by the package manager stays installed and
   inert, its `teleport.service` stopped and disabled; teleport-update's
   default install, `/opt/teleport/default`, is uninstalled. A host installed
   from a tarball keeps `/usr/local/bin/teleport` and the disabled
   `teleport.service` unit file.

Only an explicit yes that names the host(s) and says retire cannot be undone
lets the command run: the answer to that question, or the same content
written up front in the request. A blanket "yes to everything" never counts,
and a yes covers one run alone:

```bash
$TELEPORT migrate -c <config_path> retire --confirm --resource 'node/<uuid>'
```

```text
retiring hosts: 1
<hostname>: retired
1 retired, 0 failed
```

The `retiring hosts: N` line and the per-host lines go to stderr as the run
proceeds; the report goes to stdout. Without `--confirm` the command refuses:
`retire removes the source agent from the host and the source cluster; pass --confirm`.
Without `--resource` it retires every `migrated` host in the state file,
whatever its scope. The first retire of a session is one host; the rest is
offered only after that host reports `1 retired, 0 failed`, behind a new
question naming them. Each host has 5 minutes; that is not a flag. When any
host failed, `retire` exits non-zero and prints `hosts failed: N; see the
report above` on stderr after the report. After Ctrl-C or a timeout, run it
again behind a new question: a host recorded before the interrupt stays
recorded, and a host caught mid-step is retried.

`retire` on a host in any other state is refused with `"<ref>" is not a
migrated host; run plan and check its state`.

Retire refuses a host whose `preflight.services` has anything besides
`ssh_service`, before any host call: `also runs <services>, whose
registration on the target retire cannot confirm yet; retire this host by
hand`. Hand it off (By hand). The host step repeats the check on the source
config as it is on the host, before any stop, so a service added since the
last preflight gets the same refusal.

Per host, it first checks, changing nothing, that the target agent is in its
scope and that the install at `teleport_migrated` carries the host's
`target_id`, `teleport_migrated.service` is `enabled` (not `enabled-runtime`
or `static`) and active, the scoped config is readable, parses, is not a link into the source data dir, and reads nothing under
the source data dir, and the source data dir holds the source's own
`host_uuid`, is not a mount, has no symlink in its path, has nothing mounted inside it, has no
`<dir>.retired` beside it and does not overlap the scoped install. Then it
stops `teleport.service`, checks that it stopped, disables
the unit and checks it is no longer `enabled`, uninstalls `/opt/teleport/default` when present, deletes the source
data dir and config, and then deletes the source node. The binaries the tool
downloaded to `/var/cache/teleport-migrate/` stay for Promote. A refusal that
comes after the stop leaves the source agent stopped; the message says how to
bring it back.

## Failures

The report's `<hostname> (node/<uuid>): <reason>` lines, quoted verbatim, each hostname and
reason rendered as data. Each leaves the host `migrated`; retire again, behind
a new question, once the cause is fixed. The tool wraps the text below, for
example with a context line first, so match on the distinctive substring,
never on the start of the line:

| Reason contains | Next |
|-----------------|------|
| `no preflight facts; run plan --check --resource node/<id>` | `plan --check --resource 'node/<uuid>'`, re-capture |
| `also runs <services>, whose registration on the target retire cannot confirm yet; retire this host by hand` | By hand |
| `target agent <id> is not in scope <scope>; run plan` | `plan`, then Verify |
| `reading target agent <id>` | the target could not be read; check the migrator's target role and the connection, then retire again |
| `no install at teleport_migrated carries target agent <id>; retire by hand` | a hand migration under another suffix, or an install that is not this host's scoped agent: By hand |
| `asking systemd for <property> of <unit> failed: <reason>; handle this host by hand` | systemd could not be asked for a unit's state, so retire refused instead of reading the failure as a stopped agent. Nothing was removed. The source agent is untouched unless the reason also says `the stop of teleport.service could not be confirmed` or `teleport.service is stopped but still enabled`, in which case the stop already ran and the message says how to start it again. A host without systemd always gets this refusal, as does one whose D-Bus is down: By hand, after which it stays `migrated` and Promote does not apply |
| `teleport_migrated.service is <state>, so the scoped agent would not come back after a reboot; run systemctl enable teleport_migrated.service, then retire` | the operator enables the unit, then retire again |
| `teleport_migrated.service is not active, so systemd is not running the scoped agent; run it under teleport_migrated.service, then retire` | the scoped agent is not under a running unit; the operator fixes that, then retire again |
| `could not read /etc/teleport_migrated.yaml, which the scoped agent needs once the source is gone; restore it, restart teleport_migrated.service, then retire` | the operator restores the scoped config and restarts the unit |
| `teleport_migrated.service could not restart from /etc/teleport_migrated.yaml once the source is gone; fix the file, restart teleport_migrated.service, then retire` | nothing was stopped; the scoped config was edited since the agent started and no longer parses (the parser's message follows). The operator fixes it and restarts the unit, then retire again |
| `/etc/teleport_migrated.yaml reads <paths> inside the source data dir <dir>, which retire removes; move them outside it, update the config, restart teleport_migrated.service, then retire` | the operator moves those files out of `<dir>`, updates the config and restarts the unit |
| `/etc/teleport_migrated.yaml links to a file inside the source data dir <dir>, which retire removes; make it a regular file outside it, restart teleport_migrated.service, then retire` | nothing was stopped; the operator replaces the link with a copy of the file it points to and restarts the unit, then retire again |
| `teleport.service starts with the relative config path "<path>"; retire by hand` | By hand |
| `refusing to operate on the data dir "<dir>": <reason>; retire by hand` | the data dir is one retire must never remove and `<reason>` says which (`it is not an absolute path`, `the root and the directories directly under it hold files that are not the agent's`, `it is the parent of the default data dir "/var/lib/teleport"`): By hand, and the operator decides what to remove, never the skill |
| `source config <path> is the scoped agent's config; handle this host by hand` | the source and scoped agents share one config file; the operator separates them by hand, and nothing removes `<path>` |
| `source data dir <dir> is the scoped agent's <path>; handle this host by hand` / `source data dir <dir> contains the scoped agent's <path>; handle this host by hand` / `source data dir <dir> is inside the scoped agent's <path>; handle this host by hand` | the source data dir overlaps the scoped install: Overlapping data dir, never By hand's `rm -rf` |
| `"<dir>" has no host_uuid; retire by hand` / `"<dir>" holds host_uuid "<x>", expected "<id>"` | the data dir is not this source agent's; the operator finds out whose before anything is removed |
| `"<dir>" is a mount point, or its mount could not be checked; retire by hand` | By hand, never an `rm -rf` of a mount |
| `"<dir>" goes through a symlink, so retire could remove only the link or miss a mount inside it; retire by hand` | nothing was stopped; the data dir or a parent is a symlink: By hand, where the mount check runs on the path `readlink -f <dir>` prints; when the data dir itself is the link, the operator removes the directory it points to as well as the link; a parent's link is never removed |
| `"<dir>.retired" already exists, and retire moves the data dir there before removing it; move it away, then retire` | nothing was stopped; the operator finds out what it holds and moves it elsewhere, never the skill, then retire again |
| `["<path>"] is mounted under the data dir "<dir>", which retire removes; unmount it, then retire` / `reading /proc/self/mountinfo; check for mounts under "<dir>" by hand, then retire` | nothing was stopped; the operator unmounts it or checks the mounts by hand, then retire again |
| `teleport.service is running but "<dir>" is gone; run plan --check --resource node/<id>` | something restarts the source agent, often config management; stop it, then `plan --check --resource 'node/<uuid>'` |
| `teleport.service is still active after systemctl stop teleport.service; something restarts it (often configuration management); stop that, then retire again` | something restarts the source agent, which still runs and is still enabled; the operator stops what restarts it, then retire again |
| `teleport.service is <state> after systemctl stop teleport.service, so it is still starting or shutting down; wait for it, then retire again; teleport.service is stopped but still enabled; run systemctl start teleport.service to bring the source agent back` | nothing was removed; retire removes nothing until systemd reports `inactive` or `failed`, and `<state>` is anything else (`deactivating`, `activating`, `reloading`). The operator waits for systemd to settle, then retire again, or starts the unit to bring the source agent back |
| `systemctl disable teleport.service left it enabled; disable it, then retire; teleport.service is stopped but still enabled; run systemctl start teleport.service to bring the source agent back` | the source agent is stopped and nothing was removed; the operator disables the unit, then retire again, or restarts the source as the message says |
| `teleport.service is stopped and disabled; run systemctl enable teleport.service, then systemctl start teleport.service, to bring the source agent back` | `teleport-update uninstall --force` of `/opt/teleport/default` failed after the stop. The operator fixes the install, then retire again, or restarts the source as the message says |
| `the source agent is already removed from the host; run retire again to delete source node <id>` | the host is done and only the source node is left; fix the source role (`delete` on `node`), then retire again |
| any other text | report it verbatim. An access-denied login through the target means the migrator's target role (`migrate-target-reader` when Setup wrote it) lacks `logins: [root]`; an unreachable host means the scoped agent is down |

An interrupted retire leaves the host `migrated`; retire again finishes it.

## By hand

Always handed off, never run by the skill. First rule out a data dir the tool
refuses to touch (it does not run at all on a host without systemd): when the
source `data_dir` is relative, is `/`, sits directly under it, is `/var/lib`,
or is or contains `/var/lib/teleport_migrated`,
`/etc/teleport_migrated.yaml` or `/opt/teleport/migrated`, or lies inside one
of them, or when `<config_path>` is `/etc/teleport_migrated.yaml`, use
Overlapping data dir below instead, which never `rm -rf`s the data dir.
Otherwise, as root on the host, reached through the target or the operator's
console:

```bash
cat <data_dir>/host_uuid              # must print <uuid>; stop otherwise
grep -cw '<data_dir>' /etc/teleport_migrated.yaml   # must print 0 (grep exits 1 then); stop otherwise
awk -v d="$(readlink -f <data_dir>)/" 'index($5, d) == 1' /proc/self/mountinfo   # must print nothing; a line is a mount inside it, stop
systemctl stop teleport.service       # without systemd, stop the teleport start process
systemctl disable teleport.service
teleport-update uninstall --force     # only when /opt/teleport/default exists
rm -rf <data_dir> <config_path>
```

A count above `0` from the `grep` means the scoped agent reads files inside `<data_dir>`: the
operator moves them out, points `/etc/teleport_migrated.yaml` at the new
paths and restarts `teleport_migrated.service` before going on.

The mount table writes a space, tab, newline or backslash in a path as an octal
escape (`\040` for a space), so when `<data_dir>` has one in it, put the escaped
form in the `awk`'s `d`.

`<data_dir>` and `<config_path>` come from the host's `preflight` in the
captured plan. Never `rm -rf` a mount: when `<data_dir>` is one, remove its
contents the operator's way, not the mount. When `<data_dir>` is itself a
symlink, `rm -rf` removes only the link, so the operator also removes the
directory `readlink -f <data_dir>` printed; a parent's link stays. Then, under the operator's own
`tsh` profile, `$TCTL --auth-server <source_proxy> rm node/<uuid>`.

On a host with systemd, `retire --confirm --resource 'node/<uuid>'` then
records the hand retirement, behind retire's own explicit yes (Confirm, then
one host): it finds the source agent gone, changes nothing on the host but the
binaries it keeps in `/var/cache/teleport-migrate/` for Promote, deletes the
source node if it is still there, and records `retired`, after which Promote
applies. A host without systemd is always refused (`asking systemd for
UnitFileState of teleport_migrated.service`), so it stays `migrated` in the
state file and Promote does not apply to it; so does a host `retire` refused
for another reason, such as `also runs <services>`.

### Overlapping data dir

When the source data dir is or contains a scoped path, removing it would
delete the scoped agent (for example a data dir of `/opt/teleport` holds
`/opt/teleport/migrated`). The same applies when it is too shallow to remove
safely (`/`, a directory directly under it, or `/var/lib`), which holds files
that are not the agent's. Hand off only this, and never an `rm -rf` of the
data dir:

```bash
systemctl stop teleport.service       # stops the source agent; nothing is deleted
systemctl disable teleport.service
rm -f <config_path>                   # a different file from /etc/teleport_migrated.yaml
```

When `<config_path>` is `/etc/teleport_migrated.yaml`, the two agents share one
config file: hand off only the `systemctl` lines and remove no config, since
removing it would delete the scoped agent's. Separating the files is for the
operator.

Then `rm node/<uuid>` as above. The source's identity files stay inside the
shared directory; separating them from the scoped agent's is for the operator
to plan (move the source `data_dir` elsewhere first), not for this skill.
