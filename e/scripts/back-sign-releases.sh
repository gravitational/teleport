#!/usr/bin/env bash
# One-time script to retroactively sign historical Teleport Linux release tarballs.
# Downloads each published .sha256 file from the CDN, signs that digest with the
# prod KMS key, and uploads the detached .sig to the production published releases
# bucket.
#
# Usage:
#   # Local validation (no S3 upload, keeps .sig files for inspection):
#   ./back-sign-releases.sh --sign-only --version v16.0.0
#   ./back-sign-releases.sh --sign-only --version v16.0.0 --output-dir /tmp/sigs
#
#   # Dry-run (no downloads, no signing, no uploads):
#   ./back-sign-releases.sh --dry-run --version v17.7.26
#
#   # Full run (requires elevated AWS permissions on the teleport-prod-build account):
#   ./back-sign-releases.sh --version v17.7.26
#   ./back-sign-releases.sh --version v18.9.1 --overwrite
#   ./back-sign-releases.sh    # all versions
set -euo pipefail

# ── Configuration ─────────────────────────────────────────────────────────────

SIGNING_KEY_ARN="arn:aws:kms:us-west-2:146628656107:key/0631e4c0-bebd-4fb0-927a-b2637ef6d1fd"
SIGNING_ROLE_ARN="arn:aws:iam::146628656107:role/tf-auto-updates-artifact-signing-gha"
PUBLISHED_BUCKET="tp-146628656107-us-west-2-releases-published-prod"
CDN_BASE_URL="https://cdn.teleport.dev"
AWS_REGION="us-west-2"
# Used for --sign-only (local KMS access). Admin role has kms:Sign; Operator does not.
# GHA full runs use OIDC credentials and ignore this profile.
AWS_PROFILE="teleport-prod-build-admin"
export AWS_PROFILE

# Credential refresh interval: re-assume role every 45 min (role max is 1h)
CRED_REFRESH_INTERVAL=2700
CREDS_ASSUMED_AT=0

# ── Version list ───────────────────────────────────────────────────────────────
# Validated against branch/v16, branch/v17, branch/v18 CHANGELOGs and git tags.
# Versions present on the download page but missing from git tags are included.

VERSIONS=(
  # v15 (last release only)
  v15.5.4

  # v16
  v16.0.0 v16.0.1 v16.0.2 v16.0.3 v16.0.4
  v16.1.0 v16.1.1 v16.1.2 v16.1.3 v16.1.4 v16.1.8
  v16.2.0 v16.2.1 v16.2.2
  v16.3.0
  v16.4.0 v16.4.1 v16.4.2 v16.4.3 v16.4.4 v16.4.5 v16.4.6 v16.4.7 v16.4.8 v16.4.9 v16.4.10 v16.4.11 v16.4.12 v16.4.13 v16.4.14 v16.4.16 v16.4.17 v16.4.18
  v16.5.0 v16.5.1 v16.5.2 v16.5.3 v16.5.4 v16.5.5 v16.5.6 v16.5.7 v16.5.8 v16.5.9 v16.5.10 v16.5.11 v16.5.13 v16.5.14 v16.5.15 v16.5.16 v16.5.17 v16.5.18

  # v17
  v17.0.0 v17.0.1 v17.0.2 v17.0.3 v17.0.4 v17.0.5 v17.0.6
  v17.1.0 v17.1.1 v17.1.2 v17.1.3 v17.1.4 v17.1.5 v17.1.6
  v17.2.0 v17.2.1 v17.2.2 v17.2.3 v17.2.4 v17.2.5 v17.2.6 v17.2.7 v17.2.8 v17.2.9
  v17.3.0 v17.3.1 v17.3.2 v17.3.3 v17.3.4
  v17.4.0 v17.4.1 v17.4.2 v17.4.3 v17.4.4 v17.4.5 v17.4.6 v17.4.7 v17.4.8
  v17.5.0 v17.5.1 v17.5.2 v17.5.3 v17.5.4 v17.5.5 v17.5.6
  v17.6.0
  v17.7.0 v17.7.1 v17.7.2 v17.7.3 v17.7.4 v17.7.5 v17.7.6 v17.7.7 v17.7.8 v17.7.9 v17.7.10 v17.7.11 v17.7.12 v17.7.13 v17.7.14 v17.7.15 v17.7.16 v17.7.17 v17.7.18 v17.7.19 v17.7.20 v17.7.21 v17.7.22 v17.7.23 v17.7.24 v17.7.25 v17.7.26 v17.7.27

  # v18
  v18.0.0 v18.0.1 v18.0.2
  v18.1.0 v18.1.1 v18.1.2 v18.1.3 v18.1.4 v18.1.5 v18.1.6 v18.1.7 v18.1.8
  v18.2.0 v18.2.1 v18.2.2 v18.2.3 v18.2.4 v18.2.5 v18.2.6 v18.2.7 v18.2.8 v18.2.9 v18.2.10
  v18.3.0 v18.3.1 v18.3.2
  v18.4.0 v18.4.1 v18.4.2
  v18.5.0 v18.5.1
  v18.6.0 v18.6.1 v18.6.2 v18.6.3 v18.6.4 v18.6.5 v18.6.6 v18.6.7 v18.6.8
  v18.7.0 v18.7.1 v18.7.2 v18.7.3 v18.7.4 v18.7.5 v18.7.6
  v18.8.0 v18.8.1 v18.8.2 v18.8.3
  v18.9.0 v18.9.1 v18.9.2
  v18.10.0 v18.10.1 v18.10.3
)

# ── Artifact templates ─────────────────────────────────────────────────────────
# VERSION is substituted at runtime. 13 artifacts per version (2,301 total).
# centos7 variants are byte-identical copies of the amd64 build (Makefile:714,
# e/Makefile:533) — they still need independent .sig files since they're
# separate objects on CDN.
# OSS has no FIPS variants (not registered in the manifest classifier).

OSS_TEMPLATES=(
  "teleport-VERSION-linux-386-bin.tar.gz"
  "teleport-VERSION-linux-amd64-bin.tar.gz"
  "teleport-VERSION-linux-amd64-centos7-bin.tar.gz"
  "teleport-VERSION-linux-arm-bin.tar.gz"
  "teleport-VERSION-linux-arm64-bin.tar.gz"
)

ENT_TEMPLATES=(
  "teleport-ent-VERSION-linux-386-bin.tar.gz"
  "teleport-ent-VERSION-linux-amd64-bin.tar.gz"
  "teleport-ent-VERSION-linux-amd64-centos7-bin.tar.gz"
  "teleport-ent-VERSION-linux-amd64-fips-bin.tar.gz"
  "teleport-ent-VERSION-linux-amd64-centos7-fips-bin.tar.gz"
  "teleport-ent-VERSION-linux-arm-bin.tar.gz"
  "teleport-ent-VERSION-linux-arm64-bin.tar.gz"
  "teleport-ent-VERSION-linux-arm64-fips-bin.tar.gz"
)

# ── Flags ─────────────────────────────────────────────────────────────────────

DRY_RUN=false
SIGN_ONLY=false
OUTPUT_DIR=""
FILTER_VERSION=""
LIMIT=0
OVERWRITE=false

usage() {
  cat <<EOF
Usage: $(basename "$0") [OPTIONS]

Back-sign historical Teleport Linux release tarballs.

Options:
  --sign-only            Sign locally from published .sha256 files; save .sig
                         files to --output-dir.
                         No S3 upload. Uses the local AWS profile directly (no role
                         assumption). Use this to validate signing before a CI run.
  --output-dir DIR       Where to write .sig files in --sign-only mode.
                         Default: ./sig-output
  --dry-run              Print planned actions without downloading, signing, or uploading.
                         Still verifies AWS auth and role assumption.
  --version VERSION      Process only this exact version (e.g. v16.0.0).
  --limit N              Stop after N artifacts (useful for quick spot checks).
  --overwrite            Re-sign artifacts that already have a .sig.
                         Default: skip existing.
  -h, --help             Show this help.

Recommended flow:
  1. $(basename "$0") --sign-only --version v16.0.0          # local validation
     cosign verify-blob --key /tmp/signing-key.pem \\
       --insecure-ignore-tlog \\
       --signature sig-output/teleport-v16.0.0-linux-amd64-bin.tar.gz.sig \\
       /path/to/teleport-v16.0.0-linux-amd64-bin.tar.gz
  2. $(basename "$0") --version v16.0.0                      # full run via GHA
  3. $(basename "$0")                                         # all versions via GHA
EOF
}

while [[ $# -gt 0 ]]; do
  case $1 in
    --sign-only)  SIGN_ONLY=true ;;
    --output-dir) OUTPUT_DIR="$2"; shift ;;
    --dry-run)    DRY_RUN=true ;;
    --version)    FILTER_VERSION="$2"; shift ;;
    --limit)      LIMIT="$2"; shift ;;
    --overwrite)  OVERWRITE=true ;;
    -h|--help)    usage; exit 0 ;;
    *) echo "ERROR: unknown flag: $1"; usage; exit 1 ;;
  esac
  shift
done

# ── Helpers ────────────────────────────────────────────────────────────────────

log()  { echo "[$(date '+%H:%M:%S')] $*"; }
info() { echo "  $*"; }

check_prereqs() {
  local missing=()
  for cmd in aws cosign curl xxd; do
    command -v "$cmd" &>/dev/null || missing+=("$cmd")
  done
  if [[ ${#missing[@]} -gt 0 ]]; then
    echo "ERROR: missing required tools: ${missing[*]}"
    exit 1
  fi
  log "Prerequisites OK (aws, cosign, curl, xxd)"
}

assume_signing_role() {
  log "Assuming signing role: $SIGNING_ROLE_ARN"
  local creds
  creds=$(aws sts assume-role \
    --role-arn "$SIGNING_ROLE_ARN" \
    --role-session-name "back-signing-$(date +%s)" \
    --region "$AWS_REGION" \
    --output json)

  export AWS_ACCESS_KEY_ID
  export AWS_SECRET_ACCESS_KEY
  export AWS_SESSION_TOKEN
  AWS_ACCESS_KEY_ID=$(echo "$creds"    | python3 -c "import sys,json; c=json.load(sys.stdin)['Credentials']; print(c['AccessKeyId'])")
  AWS_SECRET_ACCESS_KEY=$(echo "$creds" | python3 -c "import sys,json; c=json.load(sys.stdin)['Credentials']; print(c['SecretAccessKey'])")
  AWS_SESSION_TOKEN=$(echo "$creds"    | python3 -c "import sys,json; c=json.load(sys.stdin)['Credentials']; print(c['SessionToken'])")

  CREDS_ASSUMED_AT=$(date +%s)
  log "Role assumed. Credentials valid for ~1h."
}

# Re-assume the role if credentials are older than CRED_REFRESH_INTERVAL seconds.
refresh_creds_if_needed() {
  local now
  now=$(date +%s)
  if (( now - CREDS_ASSUMED_AT >= CRED_REFRESH_INTERVAL )); then
    log "Credentials nearing expiry — refreshing."
    assume_signing_role
  fi
}

# Returns 0 if .sig exists in S3, 1 otherwise.
sig_exists_in_s3() {
  local artifact="$1"
  aws s3api head-object \
    --bucket "$PUBLISHED_BUCKET" \
    --key "public/${artifact}.sig" \
    --region "$AWS_REGION" \
    &>/dev/null
}

# Returns the HTTP status code for a CDN HEAD request.
cdn_head_status() {
  curl -s -o /dev/null -w "%{http_code}" --head --max-time 10 "${CDN_BASE_URL}/$1"
}

# Parse the published SHA-256 file into the 64-char hex digest that
# teleport-update later verifies against the downloaded artifact.
parse_checksum_hex() {
  local artifact="$1"
  local checksum_path="$2"
  local digest_hex

  digest_hex="$(awk 'NR==1 {print $1}' "$checksum_path")"
  if [[ -z "$digest_hex" || ! "$digest_hex" =~ ^[0-9a-fA-F]{64}$ ]]; then
    echo "ERROR: malformed checksum file for ${artifact}: ${checksum_path}"
    exit 1
  fi

  printf '%s\n' "$digest_hex"
}

sign_artifact_digest() {
  local digest_hex="$1"
  local sig_path="$2"
  local digest_path

  digest_path="$(mktemp)"
  trap 'rm -f "$digest_path"' RETURN
  printf '%s' "$digest_hex" | xxd -r -p > "$digest_path"

  aws kms sign \
    --key-id "$SIGNING_KEY_ARN" \
    --region "$AWS_REGION" \
    --signing-algorithm ECDSA_SHA_256 \
    --message-type DIGEST \
    --message "fileb://${digest_path}" \
    --query Signature \
    --output text > "$sig_path"

  rm -f "$digest_path"
  trap - RETURN
}

process_artifact() {
  local artifact="$1"
  local sig_key="public/${artifact}.sig"
  local checksum_url="${CDN_BASE_URL}/${artifact}.sha256"

  # ── Check checksum availability on CDN ─────────────────────────────────────
  local http_status
  http_status=$(cdn_head_status "${artifact}.sha256")
  if [[ "$http_status" == "404" ]]; then
    info "SKIP  [checksum not on CDN]  $artifact"
    return 0
  elif [[ "$http_status" != "200" ]]; then
    echo "ERROR: checksum CDN returned HTTP $http_status for $artifact"
    exit 1
  fi

  # ── Sign-only mode: download published digest + sign, no S3 ───────────────
  if [[ "$SIGN_ONLY" == "true" ]]; then
    local out_sig="${OUTPUT_DIR}/${artifact}.sig"
    if [[ -f "$out_sig" && "$OVERWRITE" == "false" ]]; then
      info "SKIP  [sig exists locally]  $artifact"
      return 0
    fi

    local tmpdir
    tmpdir=$(mktemp -d)
    trap 'rm -rf "$tmpdir"' RETURN

    local checksum_path="${tmpdir}/${artifact}.sha256"
    local sig_path="${tmpdir}/${artifact}.sig"
    local digest_hex

    info "Downloading ${artifact}.sha256"
    curl -fsSL --progress-bar -o "$checksum_path" "$checksum_url"
    digest_hex="$(parse_checksum_hex "$artifact" "$checksum_path")"

    info "Signing"
    sign_artifact_digest "$digest_hex" "$sig_path"

    cp "$sig_path" "$out_sig"
    info "OK  .sig saved to $out_sig"
    rm -rf "$tmpdir"
    trap - RETURN
    return 0
  fi

  # ── Check existing .sig in S3 ──────────────────────────────────────────────
  local already_signed=false
  if sig_exists_in_s3 "$artifact"; then
    already_signed=true
    if [[ "$OVERWRITE" == "false" ]]; then
      info "SKIP  [sig exists]  $artifact"
      return 0
    fi
  fi

  # ── Dry-run output ─────────────────────────────────────────────────────────
  if [[ "$DRY_RUN" == "true" ]]; then
    local action="SIGN"
    [[ "$already_signed" == "true" ]] && action="OVERWRITE"
    info "DRY-RUN  [$action]"
    info "  checksum:    $checksum_url"
    info "  s3 dest:     s3://${PUBLISHED_BUCKET}/${sig_key}"
    info "  signing key: $SIGNING_KEY_ARN"
    info "  sig exists:  $already_signed"
    return 0
  fi

  # ── Download digest → sign → upload → cleanup ─────────────────────────────
  local tmpdir
  tmpdir=$(mktemp -d)
  trap 'rm -rf "$tmpdir"' RETURN

  local checksum_path="${tmpdir}/${artifact}.sha256"
  local sig_path="${tmpdir}/${artifact}.sig"
  local digest_hex

  info "Downloading ${artifact}.sha256"
  curl -fsSL --progress-bar -o "$checksum_path" "$checksum_url"
  digest_hex="$(parse_checksum_hex "$artifact" "$checksum_path")"

  info "Signing"
  sign_artifact_digest "$digest_hex" "$sig_path"

  info "Uploading s3://${PUBLISHED_BUCKET}/${sig_key}"
  aws s3 cp "$sig_path" "s3://${PUBLISHED_BUCKET}/${sig_key}" \
    --region "$AWS_REGION" \
    --sse AES256 \
    --no-progress

  info "OK  $artifact"
  rm -rf "$tmpdir"
  trap - RETURN
}

# ── Main ───────────────────────────────────────────────────────────────────────

main() {
  check_prereqs

  # Build the version list to process.
  local versions=()
  if [[ -n "$FILTER_VERSION" ]]; then
    # Validate the requested version exists in the list.
    local found=false
    for v in "${VERSIONS[@]}"; do
      [[ "$v" == "$FILTER_VERSION" ]] && { found=true; break; }
    done
    if [[ "$found" == "false" ]]; then
      echo "WARNING: $FILTER_VERSION is not in the validated version list."
      echo "Proceeding anyway — it may be a newly released version."
    fi
    versions=("$FILTER_VERSION")
  else
    versions=("${VERSIONS[@]}")
  fi

  # Auth check.
  log "Checking base AWS identity (profile: $AWS_PROFILE)..."
  if ! aws sts get-caller-identity --query 'Arn' --output text 2>/dev/null; then
    echo ""
    echo "ERROR: AWS credentials are missing or expired for profile '$AWS_PROFILE'."
    echo ""
    echo "Log in first, then re-run this script:"
    echo "  aws sso login --profile $AWS_PROFILE"
    exit 1
  fi

  if [[ "$SIGN_ONLY" == "true" ]]; then
    [[ -z "$OUTPUT_DIR" ]] && OUTPUT_DIR="./sig-output"
    mkdir -p "$OUTPUT_DIR"
    log "Sign-only mode. Signatures will be written to: $(realpath "$OUTPUT_DIR")"
  elif [[ "$DRY_RUN" == "true" ]]; then
    log "DRY-RUN: verifying role assumption..."
    assume_signing_role
  else
    assume_signing_role
  fi

  local processed=0
  local skipped=0
  local total_versions=${#versions[@]}
  local vi=0

  for version in "${versions[@]}"; do
    (( vi++ )) || true
    log "[$vi/$total_versions] $version"

    [[ "$SIGN_ONLY" == "false" ]] && refresh_creds_if_needed

    for tmpl in "${OSS_TEMPLATES[@]}" "${ENT_TEMPLATES[@]}"; do
      if [[ "$LIMIT" -gt 0 && "$processed" -ge "$LIMIT" ]]; then
        log "Limit of $LIMIT artifacts reached."
        log "Summary: $processed processed, $skipped skipped."
        return
      fi

      local artifact="${tmpl/VERSION/$version}"
      if process_artifact "$artifact"; then
        (( processed++ )) || true
      else
        (( skipped++ )) || true
      fi
    done
  done

  log "Complete. $processed artifacts processed, $skipped skipped."
}

main
