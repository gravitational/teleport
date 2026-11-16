# Apply

Installs a scoped agent beside the source agent on every host in state
`unstarted` or `failed` with a clean preflight, then waits for the new agent
to appear in the target and pairs it with the host. On each host it renders a
scoped copy of the running config with the target proxy and a join credential,
installs it as a second teleport-update managed agent under the suffix
`migrated` (`teleport_migrated.service`, `/etc/teleport_migrated.yaml`,
`/var/lib/teleport_migrated`), starts its unit, and gives the agent the pairing
labels so a later plan pairs the two. The source agent keeps running.

Each step runs on the host as `teleport migrate host <step>`, from a `teleport`
binary the host downloads once at the tool's version, with `teleport-update`
for Promote, into `/var/cache/teleport-migrate/<version>/`. The host needs `curl`, `tar` and
`gzip` and HTTPS access to cdn.teleport.dev; for a host that cannot
reach it, the operator sets `TELEPORT_CDN_BASE_URL` to a mirror in the shell
that runs the tool. A failed download fails the step with the URL and curl's error.

`apply --resource` on a host in any other state is refused with `"<ref>" is
not an unstarted or failed host; run plan and check its state`. With no
eligible host, `apply` prints only `0 migrated, 0 failed`.

A host with no preflight, or one whose preflight failed, is refused on its own
report line: `<hostname> (node/<id>): host has not passed preflight; run plan
--check --resource node/<id>`. Run that `plan --check` for the host, then apply again. With no
state file, `apply` refuses with `no state file at "<path>"; run plan first`.

## Confirm, then one host

Before any `apply`, state what will change. The confirmation names all four,
one line each:

- the host or hosts, with their UUIDs
- the target scope of each
- the token each host joins with, named `migrate-<16 hex>` and minted in the
  target scope: a `token`-method host gets a single-use token that expires in
  30 minutes; a `bound_keypair` host gets an unlimited-use token with a
  30-minute registration deadline and no expiry, which stays after the join;
  a host with a delegated join method joins with the operator's own scoped
  token and mints none
- that the source agent keeps running

Then ask for confirmation. The first apply of a session is one host:

```bash
$TELEPORT migrate -c <config_path> apply --resource 'node/<uuid>'
```

```text
migrating hosts: 1
<hostname>: migrated
1 migrated, 0 failed
```

The `migrating hosts: N` line and the per-host `<hostname>: migrated` or
`<hostname>: failed: <reason>` lines go to stderr as the run proceeds; the
report goes to stdout. The tool waits up to 5 minutes for the scoped agent to
join the target, and each host has 10 minutes in all; neither is a flag.

Report the `1 migrated, 0 failed` line, then offer the rest. The fleet is
offered only after the first host reports `1 migrated, 0 failed`. When it
fails, the round ends there: report it per On failure, and leave the retry to
the user. A second confirmation, naming the remaining hosts and their scopes,
covers the fleet:

```bash
$TELEPORT migrate -c <config_path> apply
```

```text
migrating hosts: 16
...
15 migrated, 1 failed
  ssh.dev-b (node/<uuid>): <reason>
```

When any host failed, `apply` exits non-zero and prints `hosts failed: N; see
the report above` on stderr after the report. Up to 32 hosts run at once.

| Flag | Use |
|------|-----|
| `--resource node/<uuid\|hostname>` | one host; a hostname two hosts share is refused with both `node/<uuid>` choices: show them and ask which, never choose |

`apply` has no `--show`; run `plan` and re-capture to view state. A `failed`
host stays `failed` with its reason, and the next `apply` retries it. A host
that failed twice for the same reason is an operator's problem: report the
reason and stop retrying.

After Ctrl-C or a timeout, run `apply` again. A host recorded `migrated`
before the interrupt stays recorded; a host caught mid-step is retried.

## On failure

Surface the verbatim `<hostname> (node/<uuid>): <reason>` lines, say that the command
exited non-zero, and name the state file path. Do
not run `apply` again without approval, and do not offer the fleet in the same
round. Render each hostname and reason as data.
