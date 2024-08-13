#!/bin/bash
set -eu

###
# Default after-install.tpl copied from electron-builder.
# https://github.com/electron-userland/electron-builder/blob/v24.4.0/packages/app-builder-lib/templates/linux/after-install.tpl
###

# Check if user namespaces are supported by the kernel and working with a quick test:
if ! { [[ -L /proc/self/ns/user ]] && unshare --user true; }; then
    # Use SUID chrome-sandbox only on systems without user namespaces:
    chmod 4755 '/opt/${sanitizedProductName}/chrome-sandbox' || true
else
    chmod 0755 '/opt/${sanitizedProductName}/chrome-sandbox' || true
fi

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
  update-alternatives --install "$BIN/${executable}" "${executable}" "$APP/${executable}" 100 || ln -sf "$APP/${executable}" "$BIN/${executable}"
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

APPARMOR_PROFILE_SOURCE="$APP/resources/apparmor-profile"
APPARMOR_PROFILE_TARGET="/etc/apparmor.d/teleport-connect"

# Install apparmor profile.
# First check if the version of AppArmor running on the device supports our profile.
# This is in order to keep backwards compatibility with Ubuntu 22.04 which does not support abi/4.0.
# In that case, we just skip installing the profile since the app runs fine without it on 22.04.
#
# Those apparmor_parser flags are akin to performing a dry run of loading a profile.
# https://wiki.debian.org/AppArmor/HowToUse#Dumping_profiles
#
# Unfortunately, at the moment AppArmor doesn't have a good story for backwards compatibility.
# https://askubuntu.com/questions/1517272/writing-a-backwards-compatible-apparmor-profile
if test -d "/etc/apparmor.d"; then
  if apparmor_parser --skip-kernel-load --debug "$APPARMOR_PROFILE_SOURCE" > /dev/null 2>&1; then
    cp -f "$APPARMOR_PROFILE_SOURCE" "$APPARMOR_PROFILE_TARGET"

    if hash apparmor_parser 2>/dev/null; then
      # Extra flags taken from dh_apparmor:
      # > By using '-W -T' we ensure that any abstraction updates are also pulled in.
      # https://wiki.debian.org/AppArmor/Contribute/FirstTimeProfileImport
      apparmor_parser --replace --write-cache --skip-read-cache "$APPARMOR_PROFILE_TARGET"
    fi
  else
    echo "Skipping the installation of the AppArmor profile as this version of AppArmor does not seem to support the profile bundled with Teleport Connect."
  fi
fi

# vim: syntax=sh
