#!/bin/bash
#
# Common functions for build scripts. Meant to be sourced, not executed.

# Enables dry-run for some commands.
# Toggle this via flags in your main script.
DRY_RUN_PREFIX=''

# Teleport / tsh certificates/info.
# Used by other scripts.
#shellcheck disable=SC2034
readonly DEVELOPER_ID_APPLICATION='0FFD3E3413AB4C599C53FBB1D8CA690915E33D83'
#shellcheck disable=SC2034
readonly DEVELOPER_ID_INSTALLER='82B625AD327C241B378A54B4B254BB08CE71B5DF'
readonly TEAMID='QH8AA5B8UP'
#shellcheck disable=SC2034
readonly TSH_BUNDLEID="$TEAMID.com.gravitational.teleport.tsh"
#shellcheck disable=SC2034
readonly TSH_SKELETON='tsh' # relative to build.assets/macos/

# tshdev certs/info.
#readonly DEVELOPER_ID_APPLICATION='A5604F285B0957134EA099AC515BD9E0787228AC'
#readonly DEVELOPER_ID_INSTALLER='C1A831A974DF69563432C87A4979F7982DD91FBE'
#readonly TEAMID='K497G57PDJ'
#readonly TSH_BUNDLEID="$TEAMID.com.goteleport.tshdev"
#readonly TSH_SKELETON='tshdev' # relative to build.assets/macos/

# TARBALL_CACHE is used by find_or_fetch_tarball.
readonly TARBALL_CACHE=/tmp/teleport-tarballs

# log writes arguments to stderr.
log() {
  echo "$*" >&2
}

# find_or_fetch_tarball finds a local tarfile or attempts to download it from
# https://get.gravitational.com.
#
# Downloaded files are kept under /tmp/teleport-tarball.
#
# * tarname is the path to the tarfile.
#   Relative paths are resolved inside /tmp/teleport-tarball.
# * ret is the name of the output variable for the tarball path.
find_or_fetch_tarball() {
  local tarname="$1"
  local ret="$2"

  if [[ -z "$tarname" || -z "$ret" ]]; then
    log 'find_or_fetch_tarball: tarname and ret required'
    return 1
  fi

  if [[ "$tarname" != /* ]]; then
    tarname="$TARBALL_CACHE/$tarname"
  fi

  if [[ -f "$tarname" ]]; then
    eval "$ret='$tarname'"
    return 0
  fi

  if [[ "$tarname" != "$TARBALL_CACHE"/* ]]; then
    log "File $tarname not found"
    return 1
  fi

  local d=''
  d="$(dirname "$tarname")"
  mkdir -p "$d"

  local url=''
  url="https://get.gravitational.com/$(basename "$tarname")"

  log "Downloading $url to $d"
  curl -fsSLo "$tarname" "$url"
  eval "$ret='$tarname'"
  return 0
}

# notarize notarizes a target file.
#
# Relies on APPLE_USERNAME and APPLE_PASSWORD environment variables.
#
# * target is the target file.
# * teamid is the Apple Team ID.
# * bundleid is the application Bundle ID.
notarize() {
  local target="$1"
  local teamid="$2"
  local bundleid="$3"

  # XCode 13+.
  if xcrun notarytool --version 1>/dev/null 2>&1; then
    $DRY_RUN_PREFIX xcrun notarytool submit "$target" \
      --team-id="$teamid" \
      --apple-id="$APPLE_USERNAME" \
      --password="$APPLE_PASSWORD" \
      --wait
    $DRY_RUN_PREFIX xcrun stapler staple "$target"
    return 0
  fi

  # XCode 12.
  local gondir=''
  gondir="$(mktemp -d)"
  # Early expansion on purpose.
  #shellcheck disable=SC2064
  trap "rm -fr '$gondir'" EXIT

  # Gon configuration file needs a proper extension.
  local goncfg="$gondir/gon.json"
  cat >"$goncfg" <<EOF
{
  "notarize": [{
    "path": "$target",
    "bundle_id": "$bundleid",
    "staple": true
  }],
  "apple_id": {
    "username": "$APPLE_USERNAME",
    "password": "@env:APPLE_PASSWORD"
  }
}
EOF
  if [[ -n "$DRY_RUN_PREFIX" ]]; then
    log "gon configuration:"
    cat "$goncfg"
  fi
  $DRY_RUN_PREFIX gon "$goncfg"
}
