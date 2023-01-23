#!/bin/bash
set -eu

###
# Default after-install.tpl copied from electron-builder.
# https://github.com/electron-userland/electron-builder/blob/v24.0.0-alpha.5/packages/app-builder-lib/templates/linux/after-install.tpl
###

# SUID chrome-sandbox for Electron 5+
chmod 4755 "/opt/${sanitizedProductName}/chrome-sandbox" || true

update-mime-database /usr/share/mime || true
update-desktop-database /usr/share/applications || true

###
# Custom after-install.tpl script.
###

APP="/opt/${sanitizedProductName}"
BIN=/usr/local/bin
TSH_SYMLINK_SOURCE=$APP/resources/bin/tsh
TSH_SYMLINK_TARGET=$BIN/tsh

# Create $BIN if it doesn't exist.
[ ! -d "$BIN" ] && mkdir -p "$BIN"

# Link to the Electron app binary.
ln -sf "$APP/${executable}" "$BIN/${executable}"

# Link to the bundled tsh if the symlink doesn't exist already. Otherwise echo a message unless the
# link points to teleport-connect's tsh already.
# Does TSH_SYMLINK_TARGET not exist?
if [ ! -e "$TSH_SYMLINK_TARGET" ]; then
  ln -s "$TSH_SYMLINK_SOURCE" "$TSH_SYMLINK_TARGET"
else
  message="${executable}: Skipping symlinking $TSH_SYMLINK_TARGET to $TSH_SYMLINK_SOURCE"

  # Is TSH_SYMLINK_TARGET a symlink?
  if [ -L "$TSH_SYMLINK_TARGET" ]; then
    # Does TSH_SYMLINK_TARGET point at something else than TSH_SYMLINK_SOURCE?
    # If TSH_SYMLINK_TARGET exists and it points at TSH_SYMLINK_SOURCE already, don't do anything.
    if [ ! "$TSH_SYMLINK_TARGET" -ef "$TSH_SYMLINK_SOURCE" ]; then
      message+=" because the symlink already exists and it points to $(realpath $TSH_SYMLINK_TARGET)."
      echo "$message"
    fi
  else
    message+=" because $TSH_SYMLINK_TARGET already exists and it isn't a symlink."
    echo "$message"
  fi
fi

# vim: syntax=sh
