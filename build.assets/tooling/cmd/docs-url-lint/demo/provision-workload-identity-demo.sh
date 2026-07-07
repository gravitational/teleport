#!/usr/bin/env bash
#
# provision-workload-identity-demo.sh [demo-dir]
#
# Sets up two local, single-node Teleport clusters for the docs-url-lint demo
# on WorkloadIdentity rule validation - one running a build with the
# docs-link fix (lib/services/workload_identity.go's
# ValidateWorkloadIdentity), one running an already-installed release build
# that predates the fix - so the "before" and "after" error messages can be
# shown side by side without any live rebuilding. This validation only runs
# on the Auth Service, not in `tctl` itself - confirmed by checking that
# neither the function nor its error strings are linked into a built tctl
# binary - so each variant is a separate `teleport` (Auth Service) process;
# neither variant needs its own `tctl` built from source (see below).
#
# The two variants are named teleport-a (has the fix) and teleport-b (does
# not) rather than anything descriptive like "fixed"/"broken" - an agent
# handed only one variant's directory should have no way to tell there's a
# second, contrasting one.
#
#   1. Builds teleport-a/teleport-bin from this working tree's HEAD
#      (includes the docs-link fix). teleport-b uses whatever `teleport` is
#      already on PATH - no release of Teleport has ever had this fix, so
#      any already-installed release build serves as a genuine "before"
#      baseline without needing to build or check out anything extra.
#   2. Generates a single-node, auth-only config for each (via `teleport
#      configure`) on first run, each with its own data dir and port
#      (teleport-a: 3025, teleport-b: 3026), and starts (or restarts, if
#      already running) both Auth Services in the background.
#   3. Writes a `tctl` wrapper per variant that uses whatever `tctl` is
#      already on PATH (no need to build one - Teleport servers support
#      clients up to one major version behind, and this working tree's dev
#      build is at most one major version ahead of an officially released
#      tctl; see docs/pages/includes/compatibility.mdx), pointed at that
#      variant's cluster config. Each wrapper also disables client-tools
#      auto-update, which would otherwise try to fetch a stock release
#      matching teleport-a's unreleased dev version.
#   4. Writes the same draft WorkloadIdentity resource (workload-identity.yaml)
#      into both variant directories, with a plausible but invalid rule
#      expression - `in [...]` set-membership syntax, which reads naturally
#      to anyone used to Python or SQL but isn't supported by Teleport's
#      expression language - for an agent to discover and fix using the
#      docs link in teleport-a's error.
#
# Prerequisites:
#   - The docs-link fix already present in this working tree's
#     lib/services/workload_identity.go.
#   - `teleport` and `tctl` on PATH, both a release version that's the same
#     major version as this branch or one major version older (neither
#     needs the docs-link fix: teleport-b is supposed to predate it, and
#     tctl never runs this validation itself).
#
# Defaults to ~/workload-identity-setup if no directory is given.

set -euo pipefail

DEMO_DIR="${1:-$HOME/workload-identity-setup}"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../../.." && pwd)"

for cmd in teleport tctl; do
	if ! command -v "$cmd" > /dev/null 2>&1; then
		echo "$cmd not found on PATH; install a release $cmd (same major version" >&2
		echo "as this branch, or one major version older) before running this script." >&2
		exit 1
	fi
done
SYSTEM_TELEPORT="$(command -v teleport)"

mkdir -p "$DEMO_DIR"

draft_resource() {
	cat > "$1" << 'YAMLEOF'
kind: workload_identity
version: v1
metadata:
  name: build-pipeline
spec:
  spiffe:
    id: /build-pipeline
  rules:
    allow:
      - expression: 'workload.kubernetes.namespace in ["ci", "ci-canary"]'
YAMLEOF
}

# setup_cluster VARIANT_DIR PORT TELEPORT_BIN
# Generates config (if needed) pinned to PORT, (re)starts a TELEPORT_BIN
# Auth Service for this variant, writes a tctl wrapper, waits for
# readiness, and writes the draft resource.
setup_cluster() {
	local dir="$1" port="$2" teleport_bin="$3"

	if [ ! -f "$dir/teleport.yaml" ]; then
		"$teleport_bin" configure \
			--cluster-name=localhost \
			--roles=auth \
			--data-dir="$dir/data" \
			-o file://"$dir/teleport.yaml" > /dev/null
		# `teleport configure` always defaults auth_service.listen_addr to
		# 0.0.0.0:3025; repoint it at this variant's port so both clusters
		# can run at once. A no-op for teleport-a, which keeps 3025.
		sed -i.bak "s/^  listen_addr: 0.0.0.0:3025/  listen_addr: 0.0.0.0:${port}/" "$dir/teleport.yaml"
		rm -f "$dir/teleport.yaml.bak"
	fi

	# Matching on --config=<path> (rather than the teleport binary's own
	# path) works whether this variant runs a binary built just for it or
	# one shared with other things on this machine, like teleport-b's
	# system-installed binary.
	pkill -f -- "--config=$dir/teleport.yaml" 2>/dev/null || true
	sleep 1
	mkdir -p "$dir/data"
	nohup "$teleport_bin" start --config="$dir/teleport.yaml" \
		> "$dir/teleport.log" 2>&1 &
	disown

	# tctl points at this variant's cluster config so it doesn't need extra
	# flags, and disables client-tools auto-update, which would otherwise
	# try to fetch a stock release matching teleport-a's unreleased dev
	# version. This comment lives here, not in the generated wrapper, so
	# the file an agent actually reads in the demo directory stays plain.
	cat > "$dir/tctl" << WRAPEOF
#!/usr/bin/env bash
export TELEPORT_TOOLS_VERSION=off
export TELEPORT_CONFIG_FILE="$dir/teleport.yaml"
exec tctl "\$@"
WRAPEOF
	chmod +x "$dir/tctl"

	for _ in $(seq 1 30); do
		if "$dir/tctl" status > /dev/null 2>&1; then
			break
		fi
		sleep 1
	done
	if ! "$dir/tctl" status > /dev/null 2>&1; then
		echo "Cluster at ${dir} did not come up in time; see ${dir}/teleport.log" >&2
		exit 1
	fi

	"$dir/tctl" rm workload_identity/build-pipeline > /dev/null 2>&1 || true
	draft_resource "$dir/workload-identity.yaml"
}

echo "==> Building teleport-a (this working tree, has the docs-link fix) into ${DEMO_DIR}/teleport-a"
mkdir -p "$DEMO_DIR/teleport-a"
rm -f "$DEMO_DIR/teleport-a/teleport-bin"
(cd "$REPO_ROOT" && go build -o "$DEMO_DIR/teleport-a/teleport-bin" ./tool/teleport)

echo "==> Using the system teleport (${SYSTEM_TELEPORT}, no docs-link fix) for teleport-b"
mkdir -p "$DEMO_DIR/teleport-b"

echo "==> Starting both local Auth Services"
setup_cluster "$DEMO_DIR/teleport-a" 3025 "$DEMO_DIR/teleport-a/teleport-bin"
setup_cluster "$DEMO_DIR/teleport-b" 3026 "$SYSTEM_TELEPORT"

echo "==> Done. Demo directory: ${DEMO_DIR}"
echo "    teleport-a (has the docs link): ${DEMO_DIR}/teleport-a/tctl create -f ${DEMO_DIR}/teleport-a/workload-identity.yaml"
echo "    teleport-b (no docs link):      ${DEMO_DIR}/teleport-b/tctl create -f ${DEMO_DIR}/teleport-b/workload-identity.yaml"
echo "    To stop teleport-a: pkill -f -- '--config=${DEMO_DIR}/teleport-a/teleport.yaml'"
echo "    To stop teleport-b: pkill -f -- '--config=${DEMO_DIR}/teleport-b/teleport.yaml'"
