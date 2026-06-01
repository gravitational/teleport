# Direct SSH by a pre-authorized user

Baseline (non-request) SSH regression. A single actor `alice` is granted the `node-access` role directly at user-creation time — no access request, no reviewer. Alice lists nodes (she should see the node) and SSHes in as root.

This is the control case for the resource-access-request scenario: it exercises the same SSH connection path (step 6 there) but without the access-request machinery, so a failure here points at plain RBAC/SSH version skew rather than the resource-constrained-cert path.

## Cluster prep

### Roles

```bash
docker exec -i <project>-auth tctl create -f - <<EOF
kind: role
version: v7
metadata:
  name: node-access
spec:
  allow:
    logins: [root]
    node_labels:
      env: ['test']
EOF
```

### Users

Alice gets `node-access` directly (contrast with the request scenario, where she starts as `requester`). One OIDC connector + one SSO login.

```bash
./cluster/setup-user.sh <project> alice node-access
./cluster/sso-login.sh  <project> alice <proxy-ver>
```

### Capture the node's UUID

```bash
NODE_ID=$(docker exec <project>-auth tctl get nodes --format=json | jq -r '.[0].metadata.name')
```

## Flow

### Step 1 — alice sees the node

```bash
timeout 30 ./cluster/tsh.sh <project> alice <proxy-ver> ls --insecure
```

**Expected:** exit 0, output lists the node. `tsh ls` shows the node **hostname** (`teleport-node`) and its labels (`env=test`), not the UUID — assert on the `env=test` label, not the node UUID. Alice has `node-access`, so the node is visible immediately — no request needed.

If alice sees no nodes here, her role isn't being applied — RBAC regression or connector mapping failure.

### Step 2 — alice SSHes to the node (regression-critical)

```bash
timeout 30 ./cluster/tsh.sh <project> alice <proxy-ver> ssh --insecure root@<node-id> 'echo USER=$USER'
```

**Expected:** exit 0, stdout includes `USER=root`.

(The node container's shell is busybox-only; use a shell builtin instead of `whoami`/`id`.)

**This is the regression-critical step.** A failure here with the user holding a direct grant indicates plain SSH/RBAC version skew between proxy and node — independent of the access-request path.

## Failure signals

- **Step 1 shows no nodes:** alice's `node-access` role isn't applied. **Suspect: auth** (RBAC/connector mapping) — or the connector claims-to-roles mapping in `setup-user.sh`.
- **Step 2 returns "access denied":** RBAC denial despite a direct grant. **Suspect: node** if its version differs from auth (cert/role parsing), else auth.
- **Step 2 hangs (exit 124):** proxy↔node RPC incompatibility or session-join failure. **Suspect: whichever component's version differs.**
- **Any `panic:` in any container's logs:** unconditional regression; suspect whichever component panicked.

## Out of scope

- Access requests (see `resource-access-request.md`)
- SSO mechanics, MFA, trusted clusters
