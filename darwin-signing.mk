# MacOS/Darwin variables for packaging, signing and notarizing.
#
# These are parameterized per environment, with `promote` for official
# releases and `build` for development testing. These environment names
# come from our configuration in GitHub Actions.

# Default environment name if not specified.
# Once migrated fully to GitHub actions, we should change this to
# `build` as the default.
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

# CSC_NAME is the key ID for signing used by electron-builder for signing
# Teleport Connect.
CSC_NAME = $(DEVELOPER_ID_APPLICATION)

# Don't export DEVELOPER_ID_APPLICATION, DEVELOPER_ID_INSTALLER or CSC_NAME as
# it causes them to be evaluated, which shells out to the `security` command.
# They should only be evaluated if used. Any variables below that reference
# these are also unexported for the same reason.
unexport CSC_NAME DEVELOPER_ID_APPLICATION DEVELOPER_ID_INSTALLER

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

# SHOULD_NOTARIZE evalutes to "true" if we should sign and notarize binaries,
# and the empty string if not. We only notarize if APPLE_USERNAME and
# APPLE_PASSWORD are set in the environment.
SHOULD_NOTARIZE = $(if $(and $(APPLE_USERNAME),$(APPLE_PASSWORD)),true)

# NOTARIZE_BINARIES runs the notarize-apple-binaries tool. It is expected that
# the current working directory is the root of the OSS Teleport repo, so to call
# from the enterprise repo, invoke it as:
#     cd .. && $(NOTARIZE_BINARIES)
# It will not run the command if $APPLE_USERNAME or $APPLE_PASSWORD are empty.
# It uses the make $(if ...) construct instead of doing it in the shell so as
# to not evaluate its arguments (DEVELOPER_ID_APPLICATION) if we are not
# goint to use them, preventing a missing key error defined above.
NOTARIZE_BINARIES = $(if $(SHOULD_NOTARIZE),$(notarize_binaries_cmd),$(not_notarizing_cmd))
unexport NOTARIZE_BINARIES

not_notarizing_cmd = echo Not notarizing binaries. APPLE_USERNAME or APPLE_PASSWORD not set.

notary_dir = $(BUILDDIR)/notarize
notary_file = $(BUILDDIR)/notarize.zip

# notarize_binaries_cmd must be a single command - multiple commands must be
# joined with "&& \". This is so the command can be prefixed with "cd .. &&"
# for the enterprise invocation.
define notarize_binaries_cmd
	codesign \
		--sign $(DEVELOPER_ID_APPLICATION) \
		--force \
		--verbose \
		--timestamp \
		--options runtime \
		$(ABSOLUTE_BINARY_PATHS) && \
	rm -rf $(notary_dir) && \
	mkdir $(notary_dir) && \
	ditto $(ABSOLUTE_BINARY_PATHS) $(notary_dir) && \
	ditto -c -k $(notary_dir) $(notary_file) && \
	xcrun notarytool submit $(notary_file) \
		--team-id="$(TEAMID)" \
		--apple-id="$(APPLE_USERNAME)" \
		--password="$(APPLE_PASSWORD)" \
		--wait && \
	rm -rf $(notary_dir) $(notary_file)
endef
unexport notarize_binaries_cmd
