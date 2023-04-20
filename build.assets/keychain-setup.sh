#!/bin/bash
#
# keychain-setup.sh creates a MacOS keychain for application binary signing
# and for installer package signing. Each use separate keys that need to
# be loaded into a keychain.
#
# This is intended to be called from CI to set up the keychain for signing
# and notarizing MacOS binaries and packages/images. It can be called manually
# during development if you have the development keys and notarization
# username/password in your environment.
#
# This is MVP - the intention is to write all the keychain management, signing
# and notarizing as a Go program.
#
#-----------------------------------------------------------------------------
usage() {
	cat <<EOF
Usage: ${0##*/} {<options>...}
Available options:
  -a <envvar>    take base64-encoded application key from <envvar>
  -a @<file>     take application key from <file>
  -A <envvar>    take password for application key from <envvar>
  -i <envvar>    take base64-encoded installation key from <envvar>
  -i @<file>     take installation key from <file>
  -I <envvar>    take password for installation key from <envvar>
  -k <keychain>  create <keychain>.keychain (default "build")
  -p <password>  use <password> on keychain (default "insecure")
  -v             verbose. Print commands before running them
  -n             dry run. Do not run commands, just print them
EOF
}

#-----------------------------------------------------------------------------
APPLICATION_KEY_FILE=''
APPLICATION_KEY_PASSWORD=''
INSTALLER_KEY_FILE=''
INSTALLER_KEY_PASSWORD=''
KEYCHAIN='build'
KEYCHAIN_PASSWORD='insecure' # Does not need to be secret on CI, as the keychain is removed after.
VERBOSE=false
DRY_RUN=false

# This should be an array of filenames, but bash on MacOS (3.2.57) seems to unset
# the array before the EXIT trap fires and we get an unset variable error when
# trying to remove the files listed in the array.
tmpfiles=''

#-----------------------------------------------------------------------------
main() {
	set -euo pipefail

	# Always remove temp files, even if dry run
	trap 'DRY_RUN=false rm -f ${tmpfiles}' EXIT
	parse_args "$@"

	create_keychain "${KEYCHAIN}" "${KEYCHAIN_PASSWORD}"
	add_key "${APPLICATION_KEY_FILE}" "${APPLICATION_KEY_PASSWORD}" "${KEYCHAIN}" "${KEYCHAIN_PASSWORD}"
	add_key "${INSTALLER_KEY_FILE}" "${INSTALLER_KEY_PASSWORD}" "${KEYCHAIN}" "${KEYCHAIN_PASSWORD}"
}

# Create a keychain ($1) with a password ($2) and put it on the user keychain
# search path.
create_keychain() {
	local keychain="$1" password="$2"
	run security create-keychain -p "${password}" "${keychain}"
	run security unlock-keychain -p "${password}" "${keychain}"
	run security set-keychain-settings "${keychain}" # keep keychain unlocked

	# Add the new keychain to the search path, otherwise codesign does not find the keys
	local kpath
	kpath="$(security list-keychains -d user | sed 's/.*"\([^"]*\)"/\1/')"
	# shellcheck disable=SC2086
	# (Double quote to prevent globbing and word splitting)
	# We want word splitting on ${kpath}
	run security list-keychains -d user -s "${keychain}" ${kpath}
}

# Add a key from a file ($1) protected with a passphrase ($2) to a keychain ($3)
# protected with a password ($4). This is to allow `/usr/bin/codesign` and
# `/usr/bin/productsign` to access the key.
# If the key file name is empty, add_key returns without doing anything.
add_key() {
	local keyfile="$1" passphrase="$2" keychain="$3" keychain_password="$4"
	if [[ -z "${keyfile}" ]]; then
		return 0
	fi
	run security import "${keyfile}" \
		-k "${keychain}" -P "${passphrase}" \
		-T /usr/bin/codesign \
		-T /usr/bin/productsign
	# Set ACLs so the key can be used for code signing.
	# Note: This selects all the signing keys (-s) in the keychain to be usable
	# for code signing. Not a problem because the keychain is just for that only
	# and only contains the keys we've just added.
	run security set-key-partition-list \
		-S 'apple-tool:,apple:,codesign:' \
		-s -k "${keychain_password}" "${keychain}"
}

#-----------------------------------------------------------------------------
parse_args() {
	OPTSTRING=':A:I:a:i:k:np:v'
	while getopts "${OPTSTRING}" opt; do
		case "${opt}" in
		a)
			if [[ -n "${APPLICATION_KEY_FILE}" ]]; then
				error 'Application key specified multiple times'
			fi
			APPLICATION_KEY_FILE="$(get_key "${OPTARG}" appkey)"
			;;
		A)
			require_var "${OPTARG}"
			APPLICATION_KEY_PASSWORD="${!OPTARG}"
			;;
		i)
			if [[ -n "${INSTALLER_KEY_FILE}" ]]; then
				error 'Installation key specified multiple times'
			fi
			INSTALLER_KEY_FILE="$(get_key "${OPTARG}" instkey)"
			;;
		I)
			require_var "${OPTARG}"
			INSTALLER_KEY_PASSWORD="${!OPTARG}"
			;;
		k)
			KEYCHAIN="${OPTARG}"
			;;
		p)
			if [[ -z "${OPTARG}" ]]; then
				error 'Keychain password cannot be empty'
			fi
			KEYCHAIN_PASSWORD="${OPTARG}"
			;;

		n)
			DRY_RUN=true
			;;
		v)
			VERBOSE=true
			;;
		\?)
			error_usage 'Invalid option: -%s\n' "${OPTARG}" >&2
			;;
		:)
			error_usage 'Option -%s requires an argument\n' "${OPTARG}" >&2
			;;
		esac
	done
	shift $((OPTIND - 1))

	# Keychains have to end with ".keychain". Add it if necessary.
	KEYCHAIN="${KEYCHAIN%.keychain}.keychain"
}

#-----------------------------------------------------------------------------
# Run a command. If $VERBOSE or $DRY_RUN is true, echo the command. If
# $DRY_RUN is true, don't actually run the command, only print it.
run() {
	if "${VERBOSE}" || "${DRY_RUN}"; then
		echo "$@"
	fi
	"${DRY_RUN}" || "$@"
}

# Require an environment variable be set and not empty
require_var() {
	local var="$1"
	if [[ -z "${!var:-}" ]]; then
		error 'env var "%s" unset or empty' "${var}"
	fi
	return 0
}

# Require a file exists
require_file() {
	local file="$1"
	if ! [[ -f "${file}" ]]; then
		error 'File does not exist: %s' "${file}"
	fi
	return 0
}

# Get a key from an argument. If the argument starts with @, the rest is taken
# as a filename containing the key. Otherwise it is taken as an environment
# variable name which contains a base64 encoded key, which is decoded and placed
# into a temp file. The filename of the key is output to stdout. If a temp file
# was created, it will be removed when the script exits.
get_key() {
	local key="$1" keytype="$2" fname
	# If the key starts with an @, take the rest as a filename. Otherwise
	# its an environment variable name.
	if [[ "${key}" =~ ^@ ]]; then
		fname="${key:1}"
		require_file "${fname}"
	else
		require_var "${key}"
		fname="$(mktemp -t "${keytype}").p12"
		tmpfiles="${tmpfiles} ${fname}"
		printenv "${key}" | base64 --decode > "${fname}"
	fi
	echo "${fname}"
}

error() {
	# shellcheck disable=SC2059
	# (Don't use variables in the printf format string)
	# we take a format string arg - the format string IS a variable
	printf "$@" >&2
	printf '\n' >&2
	exit 1
}

error_usage() {
	# shellcheck disable=SC2059
	# (Don't use variables in the printf format string)
	# we take a format string arg - the format string IS a variable
	printf "$@" >&2
	printf '\n' >&2
	usage >&2
	exit 1
}

#-----------------------------------------------------------------------------
# Only run main if executed as a script and not sourced.
if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then main "$@"; fi

