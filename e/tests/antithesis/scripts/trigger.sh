#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
ANTITHESIS_DIR="$(cd -- "$SCRIPT_DIR/.." && pwd -P)"

SUT="${SUT:-core}"
COMMON_ENV_FILE="${COMMON_ENV_FILE:-$ANTITHESIS_DIR/.env}"
SUT_ENV_FILE="${SUT_ENV_FILE:-}"
PARAMS_FILE=""
IMAGES_FILE=""
NETRC_FILE="${NETRC_FILE:-$HOME/.netrc}"
TENANT_OVERRIDE=""
LAUNCHER_OVERRIDE=""
DRY_RUN=0

usage() {
	cat <<'EOF'
Usage: trigger.sh [options] [sut]

Build launch params for the selected SUT and trigger Antithesis.

Options:
  --sut <name|dir>         SUT name or directory        (env: SUT; default: core)
  --common-env <path>      Common env file              (env: COMMON_ENV_FILE; default: e/tests/antithesis/.env)
  --sut-env <path>         Selected SUT env file        (env: SUT_ENV_FILE; default: <sut-dir>/.env)
  --params-file <path>     Base params JSON             (default: <sut-dir>/params.json)
  --images-file <path>     Image list script            (default: <sut-dir>/images.sh)
  --tenant <name>          Antithesis tenant            (env: ANTITHESIS_TENANT)
  --launcher <name>        Launcher name                (env: ANTITHESIS_LAUNCHER)
  --netrc <path>           Path to .netrc               (env: NETRC_FILE; default: ~/.netrc)
  --dry-run                Print the request instead of sending it
  -h, --help               Show this help
EOF
}

error() {
	echo "ERROR: $*" >&2
	exit 1
}

require_arg() {
	local opt="$1"
	local val="${2:-}"
	[[ -n "$val" ]] || error "$opt requires an argument"
}

while [[ $# -gt 0 ]]; do
	case "$1" in
	--sut)
		require_arg "$1" "${2:-}"
		SUT="$2"
		shift 2
		;;
	--common-env)
		require_arg "$1" "${2:-}"
		COMMON_ENV_FILE="$2"
		shift 2
		;;
	--sut-env)
		require_arg "$1" "${2:-}"
		SUT_ENV_FILE="$2"
		shift 2
		;;
	--params-file)
		require_arg "$1" "${2:-}"
		PARAMS_FILE="$2"
		shift 2
		;;
	--images-file)
		require_arg "$1" "${2:-}"
		IMAGES_FILE="$2"
		shift 2
		;;
	--tenant)
		require_arg "$1" "${2:-}"
		TENANT_OVERRIDE="$2"
		shift 2
		;;
	--launcher)
		require_arg "$1" "${2:-}"
		LAUNCHER_OVERRIDE="$2"
		shift 2
		;;
	--netrc)
		require_arg "$1" "${2:-}"
		NETRC_FILE="$2"
		shift 2
		;;
	--dry-run)
		DRY_RUN=1
		shift
		;;
	-h | --help)
		usage
		exit 0
		;;
	--)
		shift
		break
		;;
	-*)
		echo "ERROR: unknown option: $1" >&2
		usage >&2
		exit 2
		;;
	*)
		SUT="$1"
		shift
		;;
	esac
done

command -v jq >/dev/null || error "jq not found"
if [[ "$DRY_RUN" != "1" ]]; then
	command -v curl >/dev/null || error "curl not found"
fi

resolve_sut_dir() {
	local sut="$1"

	if [[ -d "$sut" ]]; then
		cd -- "$sut" && pwd -P
		return
	fi

	if [[ -d "$ANTITHESIS_DIR/sut/$sut" ]]; then
		cd -- "$ANTITHESIS_DIR/sut/$sut" && pwd -P
		return
	fi

	error "SUT not found: $sut"
}

SUT_DIR="$(resolve_sut_dir "$SUT")"
PARAMS_FILE="${PARAMS_FILE:-$SUT_DIR/params.json}"
IMAGES_FILE="${IMAGES_FILE:-$SUT_DIR/images.sh}"
SUT_ENV_FILE="${SUT_ENV_FILE:-$SUT_DIR/.env}"

if [[ -e "$COMMON_ENV_FILE" ]]; then
    [[ -r "$COMMON_ENV_FILE" ]] || error "common env file not readable: $COMMON_ENV_FILE"
    # shellcheck source=/dev/null
    source "$COMMON_ENV_FILE"
fi

[[ -r "$SUT_ENV_FILE" ]] || error "SUT env file not readable: $SUT_ENV_FILE"
[[ -r "$PARAMS_FILE" ]] || error "params file not readable: $PARAMS_FILE"
[[ -r "$IMAGES_FILE" ]] || error "images file not readable: $IMAGES_FILE"

set -a
# shellcheck source=/dev/null
source "$SUT_ENV_FILE"
set +a

declare -a IMAGES=()
CONFIG_IMAGE=""
# shellcheck source=/dev/null
source "$IMAGES_FILE"

[[ "${#IMAGES[@]}" -gt 0 ]] || error "IMAGES is empty after sourcing $IMAGES_FILE"

IMAGES_STR="$(IFS=';'; printf '%s' "${IMAGES[*]}")"
TENANT="${TENANT_OVERRIDE:-${ANTITHESIS_TENANT:-}}"
LAUNCHER="${LAUNCHER_OVERRIDE:-${ANTITHESIS_LAUNCHER:-}}"

[[ -n "$TENANT" ]] || error "tenant not set (--tenant / ANTITHESIS_TENANT)"
[[ -n "$LAUNCHER" ]] || error "launcher not set (--launcher / ANTITHESIS_LAUNCHER)"

BODY="$(
	jq -n \
		--slurpfile params "$PARAMS_FILE" \
		--arg images "$IMAGES_STR" \
		--arg config_image "${CONFIG_IMAGE:-}" \
		--arg duration "${ANTITHESIS_DURATION:-}" \
		--arg recipients "${ANTITHESIS_RECIPIENTS:-}" \
		'{
			params: (
				$params[0]
				+ {"antithesis.images": $images}
				+ (if $config_image != "" then {"antithesis.config_image": $config_image} else {} end)
				+ (if $duration != "" then {"antithesis.duration": $duration} else {} end)
				+ (if $recipients != "" then {"antithesis.report.recipients": $recipients} else {} end)
				| with_entries(select(.value != ""))
			)
		}'
)"

URL="https://${TENANT}.antithesis.com/api/v1/launch/${LAUNCHER}"
if [[ "$DRY_RUN" == "1" ]]; then
	printf 'POST %s\n' "$URL"
	printf '%s\n' "$BODY" | jq .
	exit 0
fi

[[ -r "$NETRC_FILE" ]] || error "netrc file not readable: $NETRC_FILE"

printf '%s' "$BODY" | curl --fail --show-error \
	--netrc --netrc-file "$NETRC_FILE" \
	-H "Content-Type: application/json" \
	-X POST "$URL" \
	-d @-
