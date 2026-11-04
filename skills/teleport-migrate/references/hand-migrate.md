# Hand-migrate

For a host Plan blocked with a reason the operator resolves by hand. The result
is the same as Apply's: a scoped agent under the suffix `migrated` beside the
source agent, paired with the host. Produce the block for `runner` = hand
off, or run it over
`$TSH -i <source_identity> --proxy <source_proxy> ssh root@<uuid>` for
`runner` = run it for me, one host per confirmation. For a hand-off, the fetch
and push lines go out without their `$TSH … ssh` wrapper, for the operator's
own transport: `cat <config_path>` run on the host, copied out into
`<dir>/source.yaml`, and `umask 077 && cat > /etc/teleport_migrated.yaml`
copying `<dir>/scoped.yaml` in, with the `reconfigure` line for the workstation
between them. On a hand-off the session runs no line of the block, not even
to rehearse or check it: the token mint, the fetch, `reconfigure` and the push
are all the operator's. The session runs only reads: the `plan` capture and
`$TCTL status` for the pin.

Refuse any host whose services include `db_service`, with the tool's own
reason: it cannot run in a scope yet and the services move as one. Refuse a
host Plan blocked because Teleport's SELinux module is installed: step 3's
`teleport-update enable`, and the `uninstall` that rolls it back, remove the
module from under the source agent too, so there is no side-by-side path for
it today; leave it on its source agent.

Common values, read from the host in the captured `plan.json`:

- `<config_path>` and `<data_dir>`: `preflight.config_path` and
  `preflight.data_dir`. `/etc/teleport.yaml` and `/var/lib/teleport` are
  only their defaults.
- `<uuid>`: the host's `id`, its UUID, and the SSH selector
  `root@<uuid>`. The scoped agent carries it as the label
  `teleport.dev/reconfigure-source-uuid` so Plan pairs the two. The hostname
  is display data: it names the host in
  prose and nowhere else.
- `<dir>`: a private directory for this host from `mktemp -d`, removed with
  `rm -rf -- <dir>` when the host is done, on success or failure; the files in
  it hold join secrets. Three fixed names live in it: `source.yaml`, the
  fetched config, `scoped.yaml`, the rendered one, and `token`, the minted
  join token.
- `<token>`: a scoped token in the target scope granting `Node` plus `App` for
  `app_service` and `Kube` for `kubernetes_service`. Mint one per host,
  single-use, 12h (Apply's own tokens are single-use with a 30-minute TTL):

  ```bash
  $TCTL -i <target_identity> --auth-server <target_proxy> scoped tokens add \
    --type=<node[,app][,kube]> --scope=<scope> --assign-scope=<scope> --mode=single_use --ttl=12h --format=text > <dir>/token
  grep -Eqx '[A-Za-z0-9_@%+=:,./-]+' <dir>/token
  ```

  It writes one value, `<scope>::<name>:<secret>`, to `<dir>/token`; the
  `grep -Eqx` refuses any other shape, and a refusal ends the run. Step 2
  passes that whole value to `--token` as `"$(cat <dir>/token)"`; the scope
  prefix is what makes the join scoped and the secret is the credential. It
  is a secret: it stays in `<dir>/token`, out of the transcript, the reply and
  the command line the session prints, and the rendered config is never
  logged.
- `<pin>`: the target's CA pin, the `CA pins:` line of
  `$TCTL -i <target_identity> --auth-server <target_proxy> status`, already
  run in the Gate. Its shape is `sha256:` followed by 64 hex digits; refuse
  any other value. Write the value that line printed into the command,
  single-quoted; never hand off `'<pin>'` or any placeholder for a value the
  session has read. A source
  config that sets `ca_pin` cannot be retargeted without it.

## SSH host

Covers `no systemd on the host`, `managed by the source cluster's discovery
service`, `join method <m> is not supported`, and `bound_keypair uses a static
key`.

The config is rendered on the workstation, not the host: source agents may
predate `teleport reconfigure`, and rendering locally keeps the token out of
the host's argv. Every command on the host runs as root. Three steps.

1. Fetch the source config into `<dir>` (created by `mktemp -d`, so
   owner-only):

   ```bash
   $TSH -i <source_identity> --proxy <source_proxy> ssh 'root@<uuid>' cat '<config_path>' > <dir>/source.yaml
   ```

2. Render the scoped config locally and check the pairing label survived
   (a `host_uuid` under the input's data dir on the workstation silently
   wins over `--node-labels`):

   ```bash
   $TELEPORT reconfigure \
     --input <dir>/source.yaml \
     --output <dir>/scoped.yaml --overwrite \
     --proxy-server <target_proxy> --ca-pin '<pin>' \
     --token "$(cat <dir>/token)" --join-method token \
     --data-dir /var/lib/teleport_migrated --shift-ports \
     --node-labels teleport.dev/reconfigure-source-uuid=<uuid>
   grep -q 'reconfigure-source-uuid: <uuid>' <dir>/scoped.yaml
   ```

   `reconfigure` copies the config and applies only the flags given, so
   labels, services and addresses survive; `--token` and `--join-method`
   replace the source's join. For a delegated join (iam, ec2, azure, …) whose
   scoped token of the same bare name now exists in the scope, pass
   `--token '<scope>::<name>'` with no secret and omit `--join-method`, so the
   host keeps its own method.

   `reconfigure` refuses to carry values two agents on one host would collide
   on. `--shift-ports` does what Apply does for the addresses: it raises by
   10000 the port of each of `diag_addr`, `ssh_service.listen_addr`,
   `kubernetes_service.listen_addr` and `metrics_service.listen_addr` the
   source config sets on an enabled service, and of the default SSH address
   under `force_listen`. A source `pid_file` still needs `--pid-file` with
   `teleport_migrated.pid` in the source PID file's directory.

   If the copy has no `auth_service`/`proxy_service` sections (an agent
   started with `--roles`), add `auth_service:\n  enabled: no` and
   `proxy_service:\n  enabled: no` to it before rendering; `reconfigure
   --proxy-server` refuses a config whose proxy_service is not explicitly
   disabled. A `version: v1`/`v2` config takes `--auth-server <target_proxy>`
   instead of `--proxy-server`.

3. Push the file owner-only (the rendered config holds the join secret, and a
   plain redirect would create it world-readable under the default umask),
   then install and start the agent:

   ```bash
   $TSH -i <source_identity> --proxy <source_proxy> ssh 'root@<uuid>' 'umask 077 && cat > /etc/teleport_migrated.yaml' < <dir>/scoped.yaml
   $TSH -i <source_identity> --proxy <source_proxy> ssh 'root@<uuid>' teleport-update --install-suffix migrated enable --proxy <target_proxy>
   $TSH -i <source_identity> --proxy <source_proxy> ssh 'root@<uuid>' systemctl enable teleport_migrated
   $TSH -i <source_identity> --proxy <source_proxy> ssh 'root@<uuid>' systemctl start teleport_migrated
   ```

   Remove `<dir>` afterwards, on success or failure.

Without systemd, `teleport-update enable` cannot install a unit. Replace the
second, third and fourth lines of step 3 (the push still runs) with the teleport
binary already on the host, started as a plain background process:

```bash
$TSH -i <source_identity> --proxy <source_proxy> ssh 'root@<uuid>' 'setsid nohup teleport start -c /etc/teleport_migrated.yaml >/var/log/teleport_migrated.log 2>&1 &'
```

The agent then runs the source's teleport version and is not auto-updated;
say so.

## Then pair

Refresh the host with `plan --check --resource 'node/<uuid>'`; re-capture
`plan.json`. On the hand-off path the reply ends by telling the user to run
`plan --check --resource 'node/<uuid>'` and, if the host is still not
`migrated`, to confirm the scoped agent carries the
`teleport.dev/reconfigure-source-uuid` label. The check pairs
the host on the label `teleport.dev/reconfigure-source-uuid=<uuid>` on the
scoped agent (`matched_by: label`) or, failing that, on the `host_uuid` of the
install under `teleport_migrated`, which preflight reads (`matched_by:
check`). When the host is still not `migrated` (the agent has not joined, or
it joined in another scope), the agent shows under `unmatched_targets`. Fix
that cause, restart `teleport_migrated` if the config changed, and
`plan --check` again; the skill has no command that pairs them by declaration.

## On failure

Each listed line runs in its own call, as listed, and only after the previous
line's result is back; never join two with `&&` or `;`, and never issue two
in parallel or in one batch of tool calls. The first line that fails or is refused ends the run: report it
verbatim with the host, remove `<dir>`, and run nothing further on the host
or in the target, not even the no-systemd start line after a refused push.
Nothing here is retried automatically; a half-installed suffix is
removed with rollback.md's by-hand steps before a second try.
