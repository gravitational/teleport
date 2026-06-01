# Resource access request to SSH node

End-to-end regression for resource-constrained access requests. Two actors: `alice` (the requester, has no node access by default) and `bob` (the reviewer). Alice requests access to a specific SSH node by resource ID, Bob approves, Alice assumes the requested role and SSHes in as root.

This is the canonical scenario for the resource access-request flow. **Known to break when the SSH node version is older than auth/proxy** — the failure surfaces as step 6 (the actual SSH connection) being denied even though the request is APPROVED.

## Cluster prep

### Roles

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
---
kind: role
version: v7
metadata:
  name: node-access
spec:
  allow:
    logins: [root]
    node_labels:
      env: ['test']
---
kind: role
version: v7
metadata:
  name: reviewer
spec:
  allow:
    review_requests:
      roles: [node-access]
EOF
```

### Users

Each test user has a per-user OIDC connector pointing at the mock IdP. `setup-user.sh` registers the user record, the connector, and the role mapping in one call; `sso-login.sh` then performs an actual `tsh login` against that connector. See `references/multi-actor.md` for the mechanics.

```bash
./cluster/setup-user.sh <project> alice requester
./cluster/setup-user.sh <project> bob   reviewer

./cluster/sso-login.sh  <project> alice <proxy-ver>
./cluster/sso-login.sh  <project> bob   <proxy-ver>
```

OIDC login yields a real, non-impersonated user session — required for `tsh login --request-id` to work (step 5).

### Capture the node's UUID

The resource path in step 3 needs the node's UUID:

```bash
NODE_ID=$(docker exec <project>-auth tctl get nodes --format=json | jq -r '.[0].metadata.name')
```

## Flow

Throughout, replace `<project>`, `<proxy-ver>`, `<node-id>` with the captured values. `<proxy-ver>` is the proxy version of the current tuple.

### Step 1 — alice's baseline access

```bash
timeout 30 ./cluster/tsh.sh <project> alice <proxy-ver> ls --insecure
```

**Expected:** exit 0, output shows no nodes (alice has no `node-access` role yet — only `requester`, which grants no logins).

If alice sees the node listed here, her base role is over-permissive — this is either a scenario bug or an RBAC regression. Abort and investigate.

### Step 2 — alice searches for nodes she can request

```bash
timeout 30 ./cluster/tsh.sh <project> alice <proxy-ver> request search --kind=node --labels=env=test --insecure
```

**Expected:** exit 0. Output includes the node's UUID and the `env=test` label, indicating the search-as-roles mechanism is finding nodes alice could request.

### Step 3 — alice creates the access request

```bash
timeout 30 ./cluster/tsh.sh <project> alice <proxy-ver> request create \
  --resource /regression.local/node/<node-id> \
  --reason regression-test \
  --nowait \
  --insecure
```

**`--nowait` is required** — without it, `tsh request create` blocks until the request is approved/denied/expires, and the scenario hangs.

**Expected:** exit 0. Stdout contains the new request ID — capture it as `REQ_ID` (or query via `tctl get access_requests --format=json | jq -r '.[0].metadata.name'`).

Verify the request is PENDING:

```bash
docker exec <project>-auth tctl get access_requests --format=json | jq -r '.[0].spec.state'
# Expected: 1 (PENDING)
```

### Step 4 — bob approves

```bash
timeout 30 ./cluster/tsh.sh <project> bob <proxy-ver> request review --approve $REQ_ID --insecure
```

**Expected:** exit 0. Verify:

```bash
docker exec <project>-auth tctl get access_requests --format=json | jq -r '.[0].spec.state'
# Expected: 2 (APPROVED)
```

### Step 5 — alice assumes the approved request

Use `sso-login.sh` (not `tsh.sh`) — re-running the SSO flow ensures the new cert is issued by the proxy correctly with the assumed role:

```bash
./cluster/sso-login.sh <project> alice <proxy-ver> --request-id=$REQ_ID
```

**Expected:** exit 0. Alice's cached cert in her tsh volume is now augmented with the `node-access` role.

Notes:
- `tsh request assume` does not exist — assuming an approved request is done via `tsh login --request-id=<id>`, which re-issues alice's cert with the requested role added.
- This step requires a real (non-impersonated) user session, which the OIDC connector provides.

Verify the assumed cert has the new role:

```bash
./cluster/tsh.sh <project> alice <proxy-ver> status --insecure
```

The output should list `node-access` among alice's roles.

### Step 6 — alice SSHes to the node (regression-critical)

```bash
timeout 30 ./cluster/tsh.sh <project> alice <proxy-ver> ssh --insecure root@<node-id> 'echo USER=$USER'
```

**Expected:** exit 0, stdout includes `USER=root`.

(The node container's shell is busybox-only, so we use a shell builtin instead of `whoami` or `id` — neither is wired up as a busybox applet in the distroless-debug image.)

**This is the regression-critical step.** If steps 1-5 all pass and step 6 fails with "access denied" or hangs, the resource access-request flow is broken for this version tuple.

## Failure signals

Scenario-specific patterns to look for when a step fails:

- **Step 6 returns "access denied" while request state was APPROVED at step 4:** the known regression. Likely cause: the node can't parse resource-constrained access certs emitted by a newer auth, or vice versa. **Suspect: node** (when its version differs from auth).
- **Step 6 hangs (exit 124):** RPC incompatibility between proxy and node, or session-join failure. **Suspect: whichever component's version differs from the others.**
- **Step 3 returns "unknown field" or "unknown resource type":** schema skew in the request creation path. **Suspect:** auth if it's older, tsh/proxy if they're older.
- **Step 4 returns "permission denied" for bob:** the `reviewer` role's `review_requests` clause isn't recognized. **Suspect: auth** (RBAC schema).
- **Step 5 succeeds but `tsh status` (verification) doesn't show `node-access`:** request assumption is silently broken. **Suspect: auth.**
- **Any `panic:` in any container's logs during the flow:** unconditional regression; suspect whichever component panicked.

## Out of scope for this scenario

- SSO / OIDC / SAML login flows
- MFA-gated sessions
- Trusted clusters / multi-cluster federation
- Long-lived requests (TTL expiry)
- Bulk resource requests (multiple `--resource` flags)

Add separate scenario files for those when needed.
