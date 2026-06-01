---
name: regression-test
description: Run Teleport cross-version regression tests by spinning up an ephemeral Docker Compose cluster with user-specified auth/proxy/node versions, executing a scenario playbook that simulates user CLI flows (tsh, tctl), and reporting per-step pass/fail. Use this skill when the user asks to run the regression test, test Teleport version compatibility, check for version-skew regressions, or names specific Teleport version combinations to test (e.g. "run the regression test using 18.8.2", "test auth/proxy 18.7.2 with node 18.8.2", "run combinations of 18.7.2 and 18.8.2").
---

# Teleport Regression Test

Catch cross-version regressions in Teleport. Each invocation spins up a fresh ephemeral cluster with chosen component versions, runs scenario playbooks against it, and reports pass/fail per step.

## Triggering examples

- "run the regression test using Teleport 18.8.2" → one tuple, all three components = 18.8.2.
- "run the regression test with auth/proxy 18.7.2 and node 18.8.2" → one mixed tuple.
- "run combinations of 18.7.2 and 18.8.2" → 8 tuples (2³). **Confirm with the user before running** when tuple count > 3.
- "run the resource access request regression on 18.8.2" → name the scenario explicitly.
- "run the regression test" (no scenario, no versions) → list scenarios from `scenarios/`, ask which; then ask for versions.

## Components and versions

Three component axes: `auth`, `proxy`, `node`. The tsh client version equals the proxy version (v1 simplification).

## Workflow

For each `(auth, proxy, node)` tuple:

### 1. Up

```
./cluster/up.sh <auth> <proxy> <node>
```

The last line of stdout is the project name — capture it; you need it for teardown, tsh calls, and `docker exec` admin commands. All progress output goes to stderr. Safe to run concurrently with other clusters — see **Combinatorial requests** for the parallel workflow.

The script waits for auth, proxy, and node to be healthy and registered. If it returns non-zero, report cluster-bring-up failure with the last 100 lines of each container's log and skip this tuple (still run teardown).

### 2. Prep

Read the scenario file from `scenarios/<name>.md`. Follow its **Cluster prep** section to create roles, users, identity files, and any other initial resources. Most admin operations run inside the auth container:

```
docker exec -i <project>-auth tctl create -f - <<EOF
<resource yaml>
EOF
```

See `references/bootstrap.md` for common tctl recipes and `references/multi-actor.md` for the per-actor identity-file pattern.

### 3. Flow

Walk the **Flow** section step by step. For each step:

- Run tsh as a specific actor via `./cluster/tsh.sh <project> <actor> <proxy-version> <tsh-args...>`.
- Run admin queries via `docker exec <project>-auth tctl ...`.
- Wrap each step in `timeout 30` (or whatever the step prescribes). On macOS there is no `timeout` binary — use a portable wrapper such as `perl -e 'alarm shift; exec @ARGV' 30 <cmd...>` (a timeout kills the wrapper with exit 142).
- Capture exit code, stdout, stderr, and elapsed time.
- Compare against the step's expected outcome. Extract variables (request IDs, node UUIDs) from stdout when the scenario directs.

### 4. Failure handling

When a step fails or times out:

- Read the scenario's **Failure signals** section. Grep the listed patterns against container logs:
  ```
  docker logs <project>-auth 2>&1 | tail -100
  docker logs <project>-proxy 2>&1 | tail -100
  docker logs <project>-node 2>&1 | tail -100
  ```
- Identify the suspect component using the scenario's failure signals + the generic patterns in `references/failure-signals.md`.
- Save the failing-step output and the log tails for the final report.
- **Do not abort the tuple's teardown.** Continue to step 5.

### 5. Down

Always run, regardless of outcome:

```
./cluster/down.sh <project>
```

Idempotent and best-effort — safe to run even if some containers already exited.

## Combinatorial requests

When the user phrases versions as "combinations of A and B" (or similar), enumerate the full Cartesian product across components.

- If tuple count > 3: list the tuples and ask for confirmation before running (the run is heavier on the machine — each tuple is a full 4-container cluster).

### Running tuples in parallel

The cluster lifecycle scripts are concurrency-safe, so tuples run **in parallel** up to a concurrency limit (default **8**). Each cluster is fully isolated: a unique `COMPOSE_PROJECT_NAME`, its own docker network on a distinct `10.99.<slot>.0/24` subnet (auto-selected by `up.sh`, with overlap detected and retried), and its own `mock-oidc-users-<project>.json`. You do **not** need to assign subnets or slots yourself — just launch the runs.

Procedure:

1. Pick the concurrency limit `N` (default 8; lower it if the user asks or the machine is small). Never exceed 8 without the user's say-so.
2. If the tuple count ≤ `N`, run them all at once. Otherwise run in **batches** of `N` (start a batch, wait for it to finish, start the next).
3. For each tuple in a batch, run the whole per-tuple lifecycle (Up → Prep → Flow → Down) **in the background**, writing that tuple's output to its own log file. The simplest robust approach is to translate the chosen scenario's steps into a small per-tuple runner script, then launch one backgrounded invocation per tuple, e.g.:
   ```bash
   # results/ holds one log per tuple; each runner does up→prep→flow→down for its tuple
   mkdir -p results
   ./run-tuple.sh 18.7.6 18.7.6 18.7.2 >results/18.7.6_18.7.6_18.7.2.log 2>&1 &
   ./run-tuple.sh 18.7.6 18.7.2 18.7.6 >results/18.7.6_18.7.2_18.7.6.log 2>&1 &
   # ...up to N at a time...
   wait   # block until this batch's tuples all finish
   ```
   Keep `run-tuple.sh` as a throwaway in a temp dir (it encodes one scenario's flow; the `scenarios/*.md` files remain the source of truth). Each runner must **always tear its own cluster down**, even on failure, so a crash in one tuple never leaks containers or blocks the others.
4. After each batch's `wait`, parse the per-tuple logs for the pass/fail of every step and the suspect component, exactly as in the single-tuple Failure-handling section.

Notes:
- `up.sh` accepts an optional 4th arg to force a subnet slot (`up.sh <auth> <proxy> <node> <slot>`), but you normally omit it — auto-selection plus overlap-retry handles parallelism.
- Image pulls and the one-time mock-oidc image build are safe to run concurrently (docker dedupes), but a cleaner option is to pull/build once up front before launching a large batch.

## Final report

Print a markdown table at the top:

| Auth | Proxy | Node | Result | Failing step | Suspect |
|------|-------|------|--------|--------------|---------|
| 18.8.2 | 18.8.2 | 17.5.0 | FAIL | 6: tsh ssh access denied | node |
| 18.8.2 | 18.8.2 | 18.8.2 | PASS | — | — |

Below the table, for each FAIL tuple:
- The failing step's command, exit code, stdout/stderr (truncated to the relevant tail).
- The relevant log tail from the suspect component.
- A one-sentence root-cause guess derived from the scenario's failure signals.

For PASS tuples, no further detail. Keep the report scannable.

## References (load on demand)

- `references/bootstrap.md` — image registry, license file, networking, common tctl recipes.
- `references/multi-actor.md` — acting as different users; identity files; persistent per-actor state.
- `references/failure-signals.md` — generic regression-failure patterns to recognize in logs and exit codes.

## Scenarios

Available scenarios live in `scenarios/`. Each file contains: title, brief description, **Cluster prep** (resources to create), **Flow** (numbered steps with commands and expected outcomes), and **Failure signals** (failure shapes specific to that scenario).

Current library:
- `scenarios/resource-access-request.md` — end-to-end resource access request to an SSH node, with two actors (requester + reviewer).

To add a new scenario: write a markdown file in `scenarios/` following the same section structure. The skill discovers scenarios by listing this directory — no registration needed.
