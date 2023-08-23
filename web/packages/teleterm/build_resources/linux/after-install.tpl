#!/bin/bash
set -eu

###
# Default after-install.tpl copied from electron-builder.
# https://github.com/electron-userland/electron-builder/blob/v24.4.0/packages/app-builder-lib/templates/linux/after-install.tpl
###

# SUID chrome-sandbox for Electron 5+
chmod 4755 "/opt/${sanitizedProductName}/chrome-sandbox" || true

# update-mime-database and update-desktop-database might be missing from minimal variants of some
# Linux distributions.
if hash update-mime-database 2>/dev/null; then
  update-mime-database /usr/share/mime || true
fi

if hash update-desktop-database 2>/dev/null; then
  update-desktop-database /usr/share/applications || true
fi

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
if type update-alternatives 2>/dev/null >&1; then
  # Remove previous link if it doesn't use update-alternatives
  if [ -L "$BIN/${executable}" -a -e "$BIN/${executable}" -a "`readlink "$BIN/${executable}"`" != "/etc/alternatives/${executable}" ]; then
    rm -f "$BIN/${executable}"
  fi
  update-alternatives --install "$BIN/${executable}" "${executable}" "$APP/${executable}" 100
else
  ln -sf "$APP/${executable}" "$BIN/${executable}"
fi

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
