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
# to sign binaries and packages. We do not use the hash. Key names can be
# view by running `security find-identity`.

TEAMID = $(TEAMID_$(ENVIRONMENT_NAME))
DEVELOPER_ID_APPLICATION = $(DEVELOPER_KEY_NAME_$(ENVIRONMENT_NAME))
DEVELOPER_ID_INSTALLER = $(INSTALLER_KEY_NAME_$(ENVIRONMENT_NAME))

# CSC_NAME is the key ID for signing used by electron-builder for signing
# Teleport Connect. electron-builder does not want the "Developer ID Application: "
# prefix on the key so strip it off
CSC_NAME = $(subst Developer ID Application: ,,$(DEVELOPER_ID_APPLICATION))

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

# SHOULD_NOTARIZE evalutes to "true" if we should sign and notarize binaries,
# and the empty string if not. We only notarize if APPLE_USERNAME and
# APPLE_PASSWORD are set in the environment.
SHOULD_NOTARIZE = $(if $(and $(APPLE_USERNAME),$(APPLE_PASSWORD)),true)

# NOTARIZE_BINARIES signs and notarizes $(BINARIES).
# It will not run the command if $APPLE_USERNAME or $APPLE_PASSWORD are empty.
NOTARIZE_BINARIES = $(if $(SHOULD_NOTARIZE),$(notarize_binaries_cmd),$(not_notarizing_cmd))

not_notarizing_cmd = echo Not notarizing binaries. APPLE_USERNAME or APPLE_PASSWORD not set.

notary_dir = $(BUILDDIR)/notarize
notary_file = $(BUILDDIR)/notarize.zip

define notarize_binaries_cmd
	codesign \
		--sign '$(DEVELOPER_ID_APPLICATION)' \
		--force \
		--verbose \
		--timestamp \
		--options runtime \
		$(BINARIES)
	rm -rf $(notary_dir)
	mkdir $(notary_dir)
	ditto $(BINARIES) $(notary_dir)
	ditto -c -k $(notary_dir) $(notary_file)
	xcrun notarytool submit $(notary_file) \
		--team-id="$(TEAMID)" \
		--apple-id="$(APPLE_USERNAME)" \
		--password="$(APPLE_PASSWORD)" \
		--wait
	rm -rf $(notary_dir) $(notary_file)
endef
