# Bootstrap reference

Operational details for spinning up and configuring a regression-test cluster. Load when prepping a scenario or diagnosing a bring-up failure.

## Images

Enterprise distroless: `public.ecr.aws/gravitational/teleport-ent-distroless:<version>` (e.g. `18.8.2`).

If a version isn't found in this registry, verify the tag exists at <https://gallery.ecr.aws/gravitational/teleport-ent-distroless> before proceeding. The skill does not currently probe alternate registries.

Distroless images contain no shell. Use `docker exec <container> <binary> <args>` (no shell needed — Docker exec calls binaries directly).

## Enterprise license

`cluster/up.sh` copies `e/fixtures/license-eub.pem` from the repo root to `cluster/license-eub.pem`, and the compose file mounts it into the auth container at `/var/lib/teleport/license.pem`.

Don't check the copied `cluster/license-eub.pem` into git — it's a runtime artifact.

## Network

Each cluster gets its own docker network named `<project>-net`. The `tsh.sh` wrapper joins this network for each command.

Hostnames inside the network:
- `teleport-auth` — auth service, port 3025
- `teleport-proxy` — proxy service, port 3080 (multiplexed)
- `teleport-node` — SSH node, internal only

Nothing is exposed to the host by default. To debug a running cluster from the host, edit `cluster/compose.tmpl.yml` and add a `ports:` clause to the proxy service (e.g. `127.0.0.1:13080:3080`).

## Join tokens

Auth's config defines a static cluster-join token `regression-test-token-9b3f` (accepted for `proxy,node` roles). Proxy and node configs reference it under `teleport.join_params`. No dynamic provisioning needed.

## Cluster name

`regression.local` (set in `cluster/configs/auth.yaml`). This name appears in resource paths used by access requests (e.g. `/regression.local/node/<uuid>`).

## Authentication

Auth config sets `authentication.type: local` and `second_factor: off` to make user setup non-interactive for tests.

## Self-signed proxy cert

The proxy serves a self-signed TLS cert from the cluster's CA on first boot. **All tsh commands must pass `--insecure`** to skip verification.

## Common tctl recipes

All admin operations run inside the auth container:

```bash
docker exec <project>-auth tctl <args>
```

For resource creation, pipe a heredoc through `-i`:

```bash
docker exec -i <project>-auth tctl create -f - <<EOF
kind: role
version: v7
metadata:
  name: requester
spec:
  allow:
    request:
      search_as_roles: [node-access]
EOF
```

Common queries:
- `tctl status` — auth health, cluster info, CA pins.
- `tctl get nodes` — registered SSH nodes (note: tctl get nodes returns the node UUID as metadata.name).
- `tctl get proxies` — registered proxies.
- `tctl get access_requests` — pending/approved/denied requests with full state.
- `tctl get users` — defined users.
- `tctl users add <name> --roles=<r1,r2>` — create user (prints invite URL; ignore it if using identity files).

## Extracting variables from tctl output

JSON output is the easiest to parse:

```bash
NODE_ID=$(docker exec <project>-auth tctl get nodes --format=json | jq -r '.[0].metadata.name')
REQ_ID=$(docker exec <project>-auth tctl get access_requests --format=json | jq -r '.[0].metadata.name')
```

If `jq` isn't available, fall back to `grep`/`sed` against `--format=json` or use `--format=text` and parse the table.
