#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
Usage: export-sanitize.sh --branch <branch> [options]

Prepare a sanitized export-staging branch for a source branch.

Options:
  --branch <branch>        Source branch name (for example: master)
  --remote <remote>        Git remote to use (default: origin)
  --push-stage             Force-push export-staging/<branch> to the remote
  --create-pr              Create a promotion PR with gh after pushing staging
  -h, --help               Show this help text

Environment:
  PR_TITLE                 Optional PR title override
  PR_BODY                  Optional PR body override

Notes:
  - Requires existing export/<branch> and export-checkpoint/<branch>.
  - This script rewrites local branch history. Run it in a disposable clone or CI checkout.
EOF
}

die() {
  echo "error: $*" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "missing required command: $1"
}

run_filter_repo() {
  local refs=$1
  local filter_args=()
  local provenance_callback
  local path_globs=(
    'e'
    'e/**'
    'e2e'
    'e2e/**'
    'rfd/**'
    '.github/**'
    'skills/**'
    '.environments/**'

    # Test files, integration tests, and test utilities
    '**/*_test.go'
    'integration/**'
    'integration/appaccess/fixtures.go'
    'integration/appaccess/jwt.go'
    'integration/appaccess/pack.go'
    'integration/db/fixture.go'
    'integration/helpers/**'
    'integration/hsm/helpers.go'
    'integration/proxy/proxy_helpers.go'
    'integrations/access/datadog/testlib/**'
    'integrations/access/discord/testlib/**'
    'integrations/access/email/testlib/**'
    'integrations/access/jira/testlib/**'
    'integrations/access/mattermost/testlib/**'
    'integrations/access/msteams/testlib/**'
    'integrations/access/opsgenie/testlib/**'
    'integrations/access/pagerduty/testlib/**'
    'integrations/access/servicenow/testlib/**'
    'integrations/access/slack/testlib/**'
    'integrations/lib/testing/integration/accessrequestsuite.go'
    'integrations/lib/testing/integration/app.go'
    'integrations/lib/testing/integration/authhelper.go'
    'integrations/lib/testing/integration/suite.go'
    'integrations/operator/controllers/resources/testlib/**'
    'lib/auth/authtest/**'
    'lib/auth/helpers.go'
    'lib/auth/keystore/testhelpers.go'
    'lib/backend/test/**'
    'lib/benchmark/**'
    'lib/cryptosuites/internal/rsa/rsa.go'
    'lib/cryptosuites/precompute.go'
    'lib/events/test/**'
    'lib/modules/modulestest/**'
    'lib/modules/test.go'
    'lib/service/service.go'
    'lib/services/local/users.go'
    'lib/services/samltest/**'
    'lib/services/suite/**'
    'lib/subca/testenv/**'
    'lib/tbot/workloadidentity/workloadattest/sigstore/sigstoretest/sigstoretest.go'
    'lib/teleterm/gatewaytest/**'
    'lib/utils/cli.go'
    'lib/utils/mcptest/**'
    'lib/utils/testutils/**'
    'integrations/lib/testing/**'
    'integrations/operator/controllers/resources/testlib/**.go'
  )

  local path_glob
  for path_glob in "${path_globs[@]}"; do
    filter_args+=(--path-glob "$path_glob")
  done

  provenance_callback=$(cat <<'PYTHON'
subject = commit.message.splitlines()[0] if commit.message else b""
commit.message = (
    subject
    + b"\n\nExport-Source-Commit: "
    + commit.original_id
    + b"\n"
)
PYTHON
)

  git filter-repo --force \
    --refs "$refs" \
    --invert-paths \
    --commit-callback "$provenance_callback" \
    "${filter_args[@]}"
}

create_promotion_pr() {
  local pr_title
  local pr_body

  [[ $PUSH_STAGE -eq 1 ]] || die "--create-pr requires --push-stage"

  pr_title=${PR_TITLE:-"Export sanitized ${BRANCH}"}
  if [[ -n ${PR_BODY:-} ]]; then
    pr_body=$PR_BODY
  else
    pr_body=$(cat <<EOF
Automated promotion PR for sanitized export branch.

- Source branch: ${SRC_REF}
- Staging branch: ${STAGE_REF}
- Target branch: ${EXP_REF}

Post-merge action required:
- After this PR is merged, run the manual sync/checkpoint step (\`./export-sync.sh\`) to push \`export/${BRANCH}\` to OSS and advance \`export-checkpoint/${BRANCH}\`.
EOF
)
  fi

  gh pr create \
    --base "$EXP_REF" \
    --head "$STAGE_REF" \
    --title "$pr_title" \
    --body "$pr_body"
}

validate_prerequisites() {
  require_cmd git
  git filter-repo --version >/dev/null 2>&1 || die "missing required command: git filter-repo"

  if [[ $CREATE_PR -eq 1 ]]; then
    require_cmd gh
  fi

  if [[ -n $(git status --porcelain) ]]; then
    die "worktree is dirty; rerun in a clean clone"
  fi
}

validate_refs_exist() {
  git show-ref --verify --quiet "refs/remotes/$REMOTE/$SRC_REF" || die "missing remote source branch: $REMOTE/$SRC_REF"
  git show-ref --verify --quiet "refs/remotes/$REMOTE/$EXP_REF" || die "missing remote export branch: $REMOTE/$EXP_REF"
  git show-ref --verify --quiet "refs/tags/$CHECKPOINT_TAG" || die "missing checkpoint tag: $CHECKPOINT_TAG"
}

setup_local_branches() {
  git switch -C "$SRC_REF" "$REMOTE/$SRC_REF"
  git branch -f "$EXP_REF" "$REMOTE/$EXP_REF" >/dev/null
  git switch -C "$STAGE_REF" "$SRC_REF"
}

BRANCH=
REMOTE=origin
PUSH_STAGE=0
CREATE_PR=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --branch)
      [[ $# -ge 2 ]] || die "--branch requires a value"
      [[ -n ${2:-} ]] || die "--branch requires a value"
      BRANCH=$2
      shift 2
      ;;
    --remote)
      [[ $# -ge 2 ]] || die "--remote requires a value"
      [[ -n ${2:-} ]] || die "--remote requires a value"
      REMOTE=$2
      shift 2
      ;;
    --push-stage)
      PUSH_STAGE=1
      shift
      ;;
    --create-pr)
      CREATE_PR=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      die "unknown argument: $1"
      ;;
  esac
done

[[ -n "$BRANCH" ]] || {
  usage >&2
  die "--branch is required"
}

validate_prerequisites

SRC_REF="$BRANCH"
EXP_REF="export/$BRANCH"
STAGE_REF="export-staging/$BRANCH"
CHECKPOINT_TAG="export-checkpoint/$BRANCH"

git fetch "$REMOTE"
git fetch "$REMOTE" "+refs/tags/$CHECKPOINT_TAG:refs/tags/$CHECKPOINT_TAG"

validate_refs_exist
setup_local_branches

if git rev-list --min-parents=2 "$CHECKPOINT_TAG..$SRC_REF" | grep -q .; then
  die "merge commit detected in $CHECKPOINT_TAG..$SRC_REF; incremental export requires a linear range"
fi

git merge-base --is-ancestor "$CHECKPOINT_TAG" "$SRC_REF" || die "checkpoint tag $CHECKPOINT_TAG is not an ancestor of $SRC_REF"

run_filter_repo "$CHECKPOINT_TAG..$STAGE_REF"
git rebase --onto "$EXP_REF" "$CHECKPOINT_TAG" "$STAGE_REF"

if [[ $PUSH_STAGE -eq 1 ]]; then
  git push "$REMOTE" "+$STAGE_REF:$STAGE_REF"
fi

if [[ $CREATE_PR -eq 1 ]]; then
  create_promotion_pr
fi

echo "Prepared $STAGE_REF from $SRC_REF"
