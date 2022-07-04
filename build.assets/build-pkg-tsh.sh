#!/bin/bash
set -eu

# Flag variables
TELEPORT_TYPE=''     # -t, oss or ent
TELEPORT_VERSION=''  # -v, version, without leading 'v'
TARBALL_DIRECTORY='' # -s

usage() {
  log "Usage: $0 -t oss|eng -v version [-s tarball_directory] [-n]"
}

main() {
  local buildassets=''
  buildassets="$(dirname "$0")"

  # Don't follow sourced script.
  #shellcheck disable=SC1090
  #shellcheck disable=SC1091
  . "$buildassets/build-common.sh"

  local opt=''
  while getopts "t:v:s:n" opt; do
    case "$opt" in
      t)
        if [[ "$OPTARG" != "oss" && "$OPTARG" != "ent" ]]; then
          log "$0: invalid value for -$opt, want 'oss' or 'ent'"
          usage
          exit 1
        fi
        TELEPORT_TYPE="$OPTARG"
        ;;
      v)
        TELEPORT_VERSION="$OPTARG"
        ;;
      s)
        # Find out the absolute path to -s.
        if [[ "$OPTARG" != /* ]]; then
          OPTARG="$PWD/$OPTARG"
        fi
        TARBALL_DIRECTORY="$OPTARG"
        ;;
      n)
        DRY_RUN_PREFIX='echo + '  # declared by build-common.sh
        ;;
      *)
        usage
        exit 1
        ;;
    esac
  done
  shift $((OPTIND-1))

  # Cut leading 'v' from version, in case it's there.
  if [[ "$TELEPORT_VERSION" == v* ]]; then
    TELEPORT_VERSION="${TELEPORT_VERSION:1}"
  fi

  if [[ -z "$TELEPORT_TYPE" || -z "${TELEPORT_VERSION}" ]]; then
    usage
    exit 1
  fi

  # Verify environment varibles.
  if [[ "${APPLE_USERNAME:-}" == "" ]]; then
    echo "\
The APPLE_USERNAME environment variable needs to be set to the Apple ID used\
for notarization requests"
    exit 1
  fi
  if [[ "${APPLE_PASSWORD:-}" == "" ]]; then
    echo "\
The APPLE_PASSWORD environment variable needs to be set to an app-specific\
password created by APPLE_USERNAME"
    exit 1
  fi

  # Use similar find-or-download logic as build-package.sh for compatibility
  # purposes.
  local ent=''
  [[ "$TELEPORT_TYPE" == 'ent' ]] && ent='-ent'
  local tarname=''
  tarname="$(printf \
    "teleport%s-v%s-darwin-amd64-bin.tar.gz" \
    "$ent" "$TELEPORT_VERSION")"
  [[ -n "$TARBALL_DIRECTORY" ]] && tarname="$TARBALL_DIRECTORY/$tarname"

  tarout='' # find_or_fetch_tarball writes to this
  find_or_fetch_tarball "$tarname" tarout
  log "Using tarball at $tarout"
  tarname="$tarout"

  # Unpack tar, get ready to sign/notarize/package.
  local tmp=''
  tmp="$(mktemp -d)"
  [[ -n "$DRY_RUN_PREFIX" ]] && log "tmp = $tmp"
  $DRY_RUN_PREFIX trap "rm -fr '$tmp'" EXIT

  # $tmp/ (eventually) looks like this:
  #   teleport/tsh          # oss
  #   teleport-ent/tsh      # ent
  #   scripts               # cloned from build.assets
  #   root/tsh-vXXX.app     # package root
  #   tsh-vXXX.pkg.unsigned # created by the script
  #   tsh-vXXX.pkg          # created by the script
  mkdir "$tmp/root"

  # This creates either 'teleport/' or 'teleport-ent/' under tmp.
  # We only care about the 'tsh' file for the script.
  tar xzf "$tarname" -C "$tmp"

  # Copy and edit scripts, then write the correct VERSION variable.
  cp -r "$buildassets/macos/scripts" "$tmp/"
  sed -i '' "s/VERSION=''/VERSION='-v$TELEPORT_VERSION'/g" "$tmp/scripts"/*

  # Prepare app shell.
  local skel="$buildassets/macos/$TSH_SKELETON"
  local target="$tmp/root/tsh-v$TELEPORT_VERSION.app"
  cp -r "$skel/tsh.app" "$target"
  mkdir -p "$target/Contents/MacOS/"
  cp "$tmp"/teleport*/tsh "$target/Contents/MacOS/"

  # Sign app.
  $DRY_RUN_PREFIX codesign -f \
    -o kill,hard,runtime \
    -s "$DEVELOPER_ID_APPLICATION" \
    -i "$TSH_BUNDLEID" \
    --entitlements "$skel"/tsh*.entitlements \
    --timestamp \
    "$target"

  # Prepare and sign the installer package.
  # Note that the installer does __NOT__ have a `v` in the version number.
  target="$tmp/tsh-$TELEPORT_VERSION.pkg" # switches from app to pkg
  pkgbuild \
    --root "$tmp/root/" \
    --identifier "$TSH_BUNDLEID" \
    --version "v$TELEPORT_VERSION" \
    --install-location /Applications \
    --scripts "$tmp/scripts" \
    "$target.unsigned"
  $DRY_RUN_PREFIX productsign \
    --sign "$DEVELOPER_ID_INSTALLER" \
    --timestamp \
    "$target.unsigned" \
    "$target"
  # Make sure $target exists in case of dry runs.
  if [[ -n "$DRY_RUN_PREFIX" ]]; then
    cp "$target.unsigned" "$target"
  fi

  # Notarize.
  notarize "$target" "$TEAMID" "$TSH_BUNDLEID"

  # Copy resulting package to $PWD, generate hashes.
  mv "$target" .
  local bn=''
  bn="$(basename "$target")"
  shasum -a 256 "$bn" > "$bn.sha256"
}

main "$@"
