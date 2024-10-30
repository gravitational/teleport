#!/bin/bash
set -eu

# Do not touch symlinks if the package is being upgraded.
#
# Why?
#
# During an upgrade, RPM packages call after-install of new version first followed by after-remove
# of the old version. deb packages do this in reverse order. See README.md in this directory for
# more details.
#
# So, for RPM packages we should not remove the symlinks if the package is being upgraded, otherwise
# the user would end up with no symlinks after an upgrade.
#
# How?
#
# Both deb and RPM pass arguments to the scripts. rpm passes the number of packages of the given
# name which will be left on the system when the action completes. deb passes "upgrade" during an
# upgrade. We can check those args to determine if the package is being upgraded or removed.
#
# https://www.debian.org/doc/debian-policy/ch-maintainerscripts.html#details-of-unpack-phase-of-installation-or-upgrade
# https://docs.fedoraproject.org/en-US/packaging-guidelines/Scriptlets/#_syntax
#
# Is the first argument "upgrade" or "1"?
if [ "$1" = "upgrade" ] || [ "$1" = "1" ]; then
  echo "${executable}: Upgrade detected, skipping symlink operations"
  exit 0
fi

APP="/opt/${sanitizedProductName}"
BIN=/usr/local/bin
TSH_SYMLINK_TARGET=$BIN/tsh

# Remove the link to the Electron app binary.
if type update-alternatives >/dev/null 2>&1; then
  update-alternatives --remove "${executable}" "$APP/${executable}"
 else
  rm -f "$BIN/${executable}"
 fi

# At this point, the app has already been removed from disk. If TSH_SYMLINK_TARGET used to point at
# tsh bundled with the teleport-connect package, it is a broken symlink now.
#
# Is TSH_SYMLINK_TARGET a link that points at a file which doesn't exist?
if [ -L "$TSH_SYMLINK_TARGET" ] && [ ! -e "$TSH_SYMLINK_TARGET" ]; then
  rm -f "$TSH_SYMLINK_TARGET"
fi

APPARMOR_PROFILE_DEST="/etc/apparmor.d/teleport-connect"

# Remove apparmor profile.
if [ -f "$APPARMOR_PROFILE_DEST" ]; then
  rm -f "$APPARMOR_PROFILE_DEST"
fi

# vim: syntax=sh
