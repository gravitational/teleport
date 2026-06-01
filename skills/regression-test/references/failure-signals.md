# Failure signals reference

Generic patterns that suggest a regression rather than a test issue. Each scenario file also has its own scenario-specific `Failure signals` section — read both when diagnosing a failure.

## Log patterns

Grep against `docker logs <project>-<service> 2>&1`:

- `panic:` — Go panic. Any panic in any service is a regression. Strong signal.
- `runtime error:` — usually follows a panic; pair with the goroutine trace.
- `goroutine \d+ \[` — accompanies panics and deadlocks (paired with hangs).
- `level=error` — error-level entries. Many are benign (retry loops); look for *clusters* around the failing step's timestamp, not isolated errors.
- `unknown field` / `unknown type` — protobuf/schema incompatibility. **Strong signal of version-skew regression.**
- `unsupported version` / `unsupported protocol` — RPC version negotiation failure.
- `permission denied` in node logs while the flow expects access granted — could be the access-request regression or RBAC schema drift.
- `cluster requires a license` — license file wasn't mounted or auth couldn't read it. Bring-up issue, not a regression.
- `failed to join cluster` in proxy/node logs — token wrong, or proxy/node version can't speak to auth's join protocol.

## Step-level signals

- **Exit 124** from a `timeout`-wrapped step → hang. Investigate logs of all three services for RPC retries.
- **Exit 255** from `tsh ssh` → typically an SSH-layer error; check node logs first, then proxy.
- **Exit 1** from `tsh request approve/assume` → check auth logs for the request state machine.
- **stderr contains `connection refused`** → service hasn't started or crashed; check container status with `docker ps -a`.
- **stderr contains `certificate signed by unknown authority`** → forgot `--insecure`. Test bug, not a regression.
- **stderr contains `the cluster requires a license`** → license issue. Test bug, not a regression.

## Suspect-component heuristic

When a step fails, assign suspicion based on the version mismatch in the current tuple:

| Step type | Default suspect |
|-----------|-----------------|
| Auth-side state changes (request created, role assigned, user added) | auth |
| Talks to proxy (`tsh login`, `tsh ls`, `tsh status`) | proxy |
| Goes through proxy to node (`tsh ssh`, port-forward) | node — unless proxy version also differs |
| Identity signing (`tctl auth sign`) | auth |

If all three components share the same version and a step still fails, it's likely a single-version baseline bug, not a version-skew regression. Flag this to the user — they may have hit something fixable in a patch release.

## What's NOT a regression

- "Image not found" pulling the distroless image → the version tag doesn't exist in ECR. Verify the tag.
- Missing `e/fixtures/license-eub.pem` → license file not in the repo. Skill setup error.
- `docker compose: command not found` → docker isn't installed or compose v2 isn't available.
- `tctl: command not found` from `docker exec` → wrong image, or `/usr/local/bin/tctl` path moved. Skill maintenance.
