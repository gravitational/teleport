# Verify

Confirms, under the target identity, that each migrated or promoted host's
agent is in the target inside its mapped scope, and reports the scoped agents
in mapped scopes that no host explains. It reads; it changes nothing.

## Capture

Into the session's `mktemp -d` directory, each with its own stderr file:

```bash
$TELEPORT migrate -c <config_path> plan --show=migrated --format=json > <dir>/migrated.json 2> <dir>/migrated.err
$TELEPORT migrate -c <config_path> plan --show=promoted --format=json > <dir>/promoted.json 2> <dir>/promoted.err
$TELEPORT migrate -c <config_path> plan --show=unmatched --format=json > <dir>/unmatched.json 2> <dir>/unmatched.err
$TCTL -i <target_identity> --auth-server <target_proxy> nodes ls --format=json > <dir>/target-nodes.json 2> <dir>/target-nodes.err
```

Read them only by running `jq` or `python3 -c` over each file in the shell.

## One row per host

For each host in `hosts[]` of `migrated.json` and of `promoted.json`, look its
`target_id` up in `target-nodes.json`: match on `.metadata.name`, and read the
scope from `.scope`. A promoted host's `target_id` is the promoted agent, so
its row verifies the same way as a migrated one.

Report one table, `Host | Scope | Target | Result`, one row per migrated or
promoted host, from both files. `Result` is `found` when at least one match
sits in the host's `target_scope`; `scope mismatch (<actual>)` when the
matches sit elsewhere; `missing` when nothing matches. Mismatch and missing
rows are the findings: report them first, and offer nothing that changes state
until the user answers. The Unmatched section below is read-only and still
belongs in the same reply.

Agents in the target that no host points at are not this migration's result
and are not reported here, whatever scope they sit in; the tool's own view of
strays is the next section.

## Unmatched

`unmatched_targets` in `unmatched.json` should be empty (the `migrated` and
`promoted` captures do not carry it: a `--show <state>` render prints only
that state's `hosts`). Each entry is a scoped agent in a mapped scope no
source host explains: a hand-migrated host whose pairing label is missing, or
an agent that does not belong. Report each with its `id`, `hostname` and
`scope` as data; the tool has no command that declares or hides a pairing.

## On failure

A capture that exits non-zero or is not JSON is a failure: relay its stderr
file and stop. Never build the table from partial output.
