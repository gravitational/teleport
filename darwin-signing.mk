# MacOS/Darwin variables for packaging, signing and notarizing.
#
# These are parameterized per environment, with `promote` for official
# releases and `build` for development testing. These environment names
# come from our configuration in GitHub Actions.

# Default environment name if not specified. This is currently for Drone
# which does not set `ENVIRONMENT_NAME`. Once migrated fully to GitHub
# actions, we should change this to `build` as the default.
ENVIRONMENT_NAME ?= promote

# Variables defined here are defined with the environment name suffix
# to specify the appropriate value for that environment. The unsuffixed
# names select the appropriate value based on `ENVIRONMENT_NAME`

# Developer "team" and keys.
# TEAMID is an Apple-assigned identifier for a developer. It has two keys,
# one for signing binaries (application) and one for signing packages/images
# (installer). The keys are identified by name per-environment which we use
# to extract the key IDs. Key names can be view by running `security find-identity`.
#
# NOTE: If you need to export the DEVELOPER_ID_{APPLICATION,INSTALLER}
# variables to the environment for a command, it should be done within the
# recipe containing the command using $(eval export DEVELOPER_ID_APPLICATION ...).
# This is so the `security` shell command is only run to extract the key ID
# if necessary. If exported at the top level, it will run every time `make`
# is run.
#
# e.g.
# pkg:
#         $(eval export DEVELOPER_ID_APPLICATION DEVELOPER_ID_INSTALLER)
#         ./build.assets/build-package.sh ...
#
TEAMID = $(TEAMID_$(ENVIRONMENT_NAME))
DEVELOPER_ID_APPLICATION = $(call get_key_id,$(DEVELOPER_KEY_NAME_$(ENVIRONMENT_NAME)))
DEVELOPER_ID_INSTALLER = $(call get_key_id,$(INSTALLER_KEY_NAME_$(ENVIRONMENT_NAME)))

# Don't export DEVELOPER_ID_APPLICATION or DEVELOPER_ID_INSTALLER as it
# evaluates them. They should only be evaluated them if used.
unexport DEVELOPER_ID_APPLICATION DEVELOPER_ID_INSTALLER

# Bundle IDs identify packages/images. We use different bundle IDs for
# release and development.
TELEPORT_BUNDLEID = $(TELEPORT_BUNDLEID_$(ENVIRONMENT_NAME))
TSH_BUNDLEID = $(TSH_BUNDLEID_$(ENVIRONMENT_NAME))

# TSH_SKELETON is a directory name relative to build.assets/macos/
TSH_SKELETON = $(TSH_SKELETON_$(ENVIRONMENT_NAME))

# --- promote environment
# Key names can be found on https://goteleport.com/security
TEAMID_promote = QH8AA5B8UP
DEVELOPER_KEY_NAME_promote = Developer ID Application: Gravitational Inc.
INSTALLER_KEY_NAME_promote = Developer ID Installer: Gravitational Inc.
TELEPORT_BUNDLEID_promote = com.gravitational.teleport
TSH_BUNDLEID_promote = $(TEAMID).com.gravitational.teleport.tsh
TSH_SKELETON_promote = tsh

# --- build environment
TEAMID_build = K497G57PDJ
DEVELOPER_KEY_NAME_build = Developer ID Application: Ada Lin
INSTALLER_KEY_NAME_build = Developer ID Installer: Ada Lin
TELEPORT_BUNDLEID_build = com.goteleport.dev
TSH_BUNDLEID_build = $(TEAMID).com.goteleport.tshdev
TSH_SKELETON_build = tshdev

# --- utility
# Extract application/installer key ID from keychain. This looks at all
# keychains in the search path. It should be used with $(call ...).
# e.g. $(call get_key_id,Key Name goes here)
get_key_id = $(or $(word 2,$(shell $(get_key_id_cmd))), $(missing_key_error))
get_key_id_cmd = security find-identity -v -s codesigning | grep --fixed-strings --max-count=1 "$(1)"
missing_key_error = $(error Could not find key named "$(1)" in keychain)

# Dont export missing_key_error or get_key_id as it evaluates them
unexport missing_key_error get_key_id
