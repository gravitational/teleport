# Promote

Replaces the scoped agent on a retired host with an agent on the default
paths, the way Apply replaced the source agent: a second agent starts as
`teleport.service` from `/etc/teleport.yaml` with `/var/lib/teleport` as its
data dir, joins the target in the same scope with the same labels, and the
suffixed install is then removed. The promoted agent is a new node in the
target with its own host UUID; access to the host does not change, because
its scope and labels do not. The host's state becomes `promoted`. Promote is
optional: a retired host left on `teleport_migrated` runs a complete agent
that teleport-update keeps updating.

It acts on hosts in state `retired`, and reaches each as root through the
target cluster, as Retire does (`logins: [root]` in the migrator's target
role, `migrate-target-reader` when Setup wrote it). It creates a token for the
promoted agent as Apply does, so the migrator needs the same token rights in
the target as for Apply.

The state file keeps each promoted host's chain: `id` is the source agent,
`scoped_id` is the scoped agent Promote replaced, and `target_id` is the
promoted agent.

## Confirm, then one host

Read the host from the captured plan JSON (`plan --show=retired
--format=json`) by running `jq` or `python3 -c` over it in the shell. Before
asking, or before running on a pre-answered yes, name the host, its scope and
UUID; say that sessions through the scoped agent end when it is removed, that
the host comes back as a new node in the target with a new host UUID in the
same scope, and that a promoted agent that does not join is stopped again with
the scoped agent still serving. Ask, then:

```bash
$TELEPORT migrate -c <config_path> promote --resource 'node/<uuid>'
```

```text
promoting hosts: 1
<hostname>: promoted
1 promoted, 0 failed
```

On success, report that the host's state is now `promoted`, that it runs as
`teleport.service` from `/etc/teleport.yaml`, and its new `target_id` from a
fresh `plan --show=promoted --format=json`, with the `scoped_id` it replaced.

The `promoting hosts: N` line and the per-host lines go to stderr as the run
proceeds; the report goes to stdout. The rest (no `--resource`: every
`retired` host) is offered only after that host reports `1 promoted, 0
failed`, behind its own confirmation naming them. Each host has 15 minutes,
which is not a flag. When any host failed, `promote` exits non-zero and prints
`hosts failed: N; see the report above` on stderr after the report. On a host
in any other state, `--resource` is refused with `"<ref>" is not a retired
host; run plan and check its state`.

One thing the operator should know before a yes: `promote` passes
`--overwrite`, so on a `.deb` or `.rpm` host it rewrites the package's
`teleport.service` unit and the `/usr/local/bin` symlinks while the package
stays installed. A later package upgrade can undo them.

## What it does

On each host, through the target:

1. Creates a token for the promoted agent in the host's target scope, under a
   name of its own, so the scoped agent's token stays until the end. A
   delegated join method uses the operator's token, as Apply does.
2. On the scoped agent, writes `/etc/teleport.yaml`: the scoped config with
   `data_dir` back to `/var/lib/teleport`, the PID file renamed to
   `teleport.pid`, each listen address Apply shifted (`diag_addr` and the SSH,
   Kubernetes and metrics addresses of enabled services) moved back down, and
   the new token. It runs the scoped install's
   `/opt/teleport/migrated/bin/teleport-update enable --proxy <proxy>
   --overwrite`, adding `--base-url` when the scoped install's `update.yaml`
   names one, then enables and restarts `teleport.service`.
3. Waits up to 5 minutes for the promoted agent's `host_uuid`, then up to 5
   for the target to list it in the scope. When either wait runs out, or the
   step ends without finishing, it stops and disables `teleport.service`
   again; the host stays `retired` with the scoped agent serving.
4. Through the promoted agent, removes the suffixed install as Rollback does,
   and the fetch cache.
5. Deletes the scoped agent's node and Apply's token from the target, and
   records `promoted`.

A host promoted by hand, whose scoped agent is gone from the target, is
reached through the agent labeled for it and only recorded as `promoted`
under that agent's UUID: no token, and nothing on the host changes. A run cut
short before the promoted agent joined leaves the host `retired`, and after
it `promoted`; running `promote` again replaces what an earlier attempt
installed.

## Failures

The report's `<hostname> (node/<uuid>): <reason>` lines, quoted verbatim, each
hostname and reason rendered as data. The tool wraps the text below, for
example with a context line first, so match on the distinctive substring,
never on the start of the line:

A refusal before anything is installed starts with `refusing to promote this
host automatically; promote it by hand, or clear what is in the way and
promote again`, then gives the reason. After the install starts, a failure
stops the promoted agent first and says `teleport.service was stopped and
disabled again and the scoped agent still serves`, then gives the cause.

| Reason contains | Next |
|-----------------|------|
| `no scoped install at /opt/teleport/migrated` | the host has no scoped install at the fixed suffix: By hand, or leave it |
| `listen address "<addr>" was not shifted by apply` | a listen address in the scoped config is not one Apply set, so the default cannot be derived: By hand |
| `the scoped config /etc/teleport_migrated.yaml is gone` | the scoped agent answers but its config is gone; the operator checks the host: By hand |
| `teleport_migrated.service is not running` | the session did not run through a running scoped agent, so nothing would keep serving; the operator checks the host: By hand |
| `the promoted config at "/etc/teleport.yaml" belongs to another migration` / `the promoted config at "/etc/teleport.yaml" has no` / `teleport.service is running without /etc/teleport.yaml` / `/var/lib/teleport holds another agent's host_uuid` | something other than this migration is on the default paths; the operator moves it aside, then `promote` again |
| `the scoped agent <uuid> is not in the target, and no other agent in <scope> carries this host's migration label` | the scoped agent is gone and no promoted agent was found; the operator checks the host from its console |
| `the scoped agent <uuid> is not in the target, and <n> agents in <scope> carry this host's migration label` | more than one agent claims the host; the operator removes the stale nodes or labels, then `promote` again |
| `teleport.service is not running with a host_uuid in /var/lib/teleport` | a promote by hand stopped part way; the operator finishes it, then `promote` again to record it |
| `teleport.service wrote no host_uuid within 5m0s` | the promoted agent did not start; the operator reads `journalctl -u teleport.service` on the host, fixes the cause, then `promote` again |
| `waiting for the promoted agent to join; the promoted agent was stopped and disabled again and the scoped agent still serves` | the promoted agent started but never joined, often a network path to the target; fix it, then `promote` again |
| `the promote step did not finish; the promoted agent was stopped and disabled again and the scoped agent still serves` | the session to the host ended mid-step; `promote` again |
| `stopping the promoted agent; stop and disable teleport.service on the host by hand` | the promoted agent did not join and could not be stopped; the operator stops and disables `teleport.service`, then `promote` again |
| `the promoted agent joined, but removing the scoped install failed; remove it by hand` | the host is `promoted` and both agents run; the operator removes the suffixed install through the promoted agent (By hand, last block) |
| `host promoted, target cleanup incomplete` | the scoped node or Apply's token is still in the target; the reason names the `tctl` command |
| `another migrate command is still installing on this host after 5m0s` | another run holds the host's install lock; wait, then `promote` again |
| any other text | report it verbatim; often `teleport-update enable`'s own output, quoted with its command line, or SSH through the target |

## By hand

Leaving the host on `teleport_migrated` needs nothing more and is the answer
for most refusals. An operator who wants the default paths anyway does what
the tool does, from the guide's "Promote the scoped agent" section
(`docs/pages/zero-trust-access/rbac-get-started/migrate-to-scopes.mdx`):

1. A scoped token in the host's target scope, as in Hand-migrate.
2. The scoped config, fetched through the target with the target identity.
3. `$TELEPORT reconfigure --input <scoped> --output <promoted> --token "$TOKEN"
   --join-method token --data-dir /var/lib/teleport`, plus `--pid-file` with
   `teleport.pid` when the scoped config sets one, and, for each address Apply
   shifted, its flag with the port lowered by 10,000: `--diag-addr`,
   `--ssh-listen-addr`, `--kube-listen-addr`, `--metrics-listen-addr`. A
   delegated join method leaves out `--join-method`.
4. On the host: the promoted config as `/etc/teleport.yaml`, readable only by
   root; `/opt/teleport/migrated/bin/teleport-update enable --proxy <proxy>
   --overwrite`; `systemctl enable` and `restart teleport.service`; the
   promoted agent's UUID from `/var/lib/teleport/host_uuid`.
5. Through the promoted agent (a session through the scoped agent ends when it
   stops): the suffixed install removed as in Rollback's by-hand steps, then
   `$TCTL rm node <scope>::<scoped uuid>`, logged in without a scope. The local
   copies of both configs are deleted, since they hold join tokens.

A later `promote --resource node/<uuid>` records the host as `promoted` and
changes nothing on it. Hand this off; the skill does not run it.
