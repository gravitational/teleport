# MacOS/Darwin variables for packaging, signing and notarizing.
#
# These are parameterized per environment, with `prod/build` for official
# releases and `stage/build` for development testing. These environment names
# come from our configuration in GitHub Actions. These parameters may be
# moved to the GitHub Actions environments, however we'll always keep the
# development testing variables defined here so as to be able to run the
# signing locally for development purposes.
#
# A new set of signing parameters would also require a new provisioning
# profile alongside build.assets/macos/*/*.provisioningprofile. We also need
# to update these profiles if the keys are changed/rotated.

# Default environment name if not specified. This is currently for running
# locally instead of from GitHub Actions, where ENVIRONMENT_NAME would not be
# set.
ENVIRONMENT_NAME ?= stage/build

# CLEAN_ENV_NAME replaces hyphens with underscores as hyphens are not valid in
# environment variable names (make is ok with them, but they get exported, and
# we want that to be clean).
CLEAN_ENV_NAME = $(subst /,_,$(ENVIRONMENT_NAME))

# Variables defined below are defined with the clean environment name suffix to
# specify the appropriate value for that environment. The unsuffixed names
# select the appropriate value based on `CLEAN_ENV_NAME`

# Developer "team" and keys.
#
# TEAMID is an Apple-assigned identifier for a developer. It has two keys, one
# for signing binaries (application) and one for signing packages/images
# (installer). The keys are identified by name per-environment which we use to
# sign binaries and packages. We do not use the hash. Key names can be view by
# running `security find-identity`.
TEAMID = $(TEAMID_$(CLEAN_ENV_NAME))
DEVELOPER_ID_APPLICATION = $(DEVELOPER_KEY_NAME_$(CLEAN_ENV_NAME))
DEVELOPER_ID_INSTALLER = $(INSTALLER_KEY_NAME_$(CLEAN_ENV_NAME))

# CSC_NAME is the key ID for signing used by electron-builder for signing
# Teleport Connect. electron-builder does not want the "Developer ID Application: "
# prefix on the key so strip it off
CSC_NAME = $(subst Developer ID Application: ,,$(DEVELOPER_ID_APPLICATION))

# Bundle IDs identify packages/images. We use different bundle IDs for
# release and development.
TELEPORT_BUNDLEID = $(TELEPORT_BUNDLEID_$(CLEAN_ENV_NAME))
TSH_BUNDLEID = $(TSH_BUNDLEID_$(CLEAN_ENV_NAME))
TCTL_BUNDLEID = $(TCTL_BUNDLEID_$(CLEAN_ENV_NAME))

# TSH_SKELETON is a directory name relative to build.assets/macos/
TSH_SKELETON = $(TSH_SKELETON_$(CLEAN_ENV_NAME))
TCTL_SKELETON = $(TCTL_SKELETON_$(CLEAN_ENV_NAME))

# --- prod/build environment (promote is the old name and will be removed)
# Key names can be found on https://goteleport.com/security
TEAMID_prod_build = QH8AA5B8UP
DEVELOPER_KEY_NAME_prod_build = Developer ID Application: Gravitational Inc.
INSTALLER_KEY_NAME_prod_build = Developer ID Installer: Gravitational Inc.
TELEPORT_BUNDLEID_prod_build = com.gravitational.teleport
TSH_BUNDLEID_prod_build = $(TEAMID).com.gravitational.teleport.tsh
TSH_SKELETON_prod_build = tsh
TCTL_BUNDLEID_prod_build = $(TEAMID).com.gravitational.teleport.tctl
TCTL_SKELETON_prod_build = tctl

# --- stage/build environment (build is the old name and will be removed)
TEAMID_stage_build = K497G57PDJ
DEVELOPER_KEY_NAME_stage_build = Developer ID Application: Ada Lin
INSTALLER_KEY_NAME_stage_build = Developer ID Installer: Ada Lin
TELEPORT_BUNDLEID_stage_build = com.goteleport.dev
TSH_BUNDLEID_stage_build = $(TEAMID).com.goteleport.tshdev
TSH_SKELETON_stage_build = tshdev
TCTL_BUNDLEID_stage_build = $(TEAMID).com.goteleport.tctldev
TCTL_SKELETON_stage_build = tctldev

# SHOULD_NOTARIZE evalutes to "true" if we should sign and notarize binaries,
# and the empty string if not. We only notarize if APPLE_USERNAME and
# APPLE_PASSWORD are set in the environment.
SHOULD_NOTARIZE = $(if $(and $(APPLE_USERNAME),$(APPLE_PASSWORD)),true)

notary_dir = $(BUILDDIR)/notarize
notary_file = $(BUILDDIR)/notarize.zip

MAC_TOOLING_FLAGS = --retry 2 $(if $(SHOULD_NOTARIZE),,--dry-run)
MAC_TOOLING = go run github.com/gravitational/shared-workflows/tools/mac-distribution@v0.0.0
MAC_TOOLING_CMD = $(MAC_TOOLING) $(MAC_TOOLING_FLAGS)

NOTARIZE_BINARIES = $(notarize_binaries_cmd)

define notarize_binaries_cmd
	@echo Notarizing binaries
	$(MAC_TOOLING_CMD) notarize $(BINARIES)
endef

NOTARIZE_TSH_APP = $(if $(SHOULD_NOTARIZE),$(notarize_tsh_app),$(not_notarizing_cmd))
define notarize_tsh_app
	$(call notarize_app_bundle,$(TSH_APP_BUNDLE),$(TSH_BUNDLEID),$(TSH_APP_ENTITLEMENTS))
endef

NOTARIZE_TCTL_APP = $(if $(SHOULD_NOTARIZE),$(notarize_tctl_app),$(not_notarizing_cmd))
define notarize_tctl_app
	$(call notarize_app_bundle,$(TCTL_APP_BUNDLE),$(TCTL_BUNDLEID),$(TCTL_APP_ENTITLEMENTS))
endef

NOTARIZE_TELEPORT_PKG = $(if $(SHOULD_NOTARIZE),$(notarize_teleport_pkg),$(not_notarizing_cmd))
define notarize_teleport_pkg
	$(call notarize_pkg,$(TELEPORT_PKG_UNSIGNED),$(TELEPORT_PKG_SIGNED))
endef

define notarize_app_bundle
	$(eval $@_BUNDLE = $(1))
	$(eval $@_BUNDLE_ID = $(2))
	$(eval $@_ENTITLEMENTS = $(3))
	codesign \
		--sign '$(DEVELOPER_ID_APPLICATION)' \
		--identifier "$($@_BUNDLE_ID)" \
		--force \
		--verbose \
		--timestamp \
		--options kill,hard,runtime \
		--entitlements "$($@_ENTITLEMENTS)" \
		"$($@_BUNDLE)"

	ditto -c -k --keepParent $($@_BUNDLE) $(notary_file)
	xcrun notarytool submit $(notary_file) \
		--team-id="$(TEAMID)" \
		--apple-id="$(APPLE_USERNAME)" \
		--password="$(APPLE_PASSWORD)" \
		--wait
	rm $(notary_file)
endef

define notarize_pkg
	$(eval $@_IN_PKG = $(1))
	$(eval $@_OUT_PKG = $(2))
	productsign \
		--sign '$(DEVELOPER_ID_INSTALLER)' \
		--timestamp \
		$($@_IN_PKG) \
		$($@_OUT_PKG)
	xcrun notarytool submit $($@_OUT_PKG) \
		--team-id="$(TEAMID)" \
		--apple-id="$(APPLE_USERNAME)" \
		--password="$(APPLE_PASSWORD)" \
		--wait
	xcrun stapler staple $($@_OUT_PKG)
endef

echo_var = @echo $(1)=\''$($(1))'\'

.PHONY: print-darwin-signing-vars
print-darwin-signing-vars:
	$(call echo_var,ENVIRONMENT_NAME)
	$(call echo_var,CLEAN_ENV_NAME)
	$(call echo_var,TEAMID)
	$(call echo_var,DEVELOPER_ID_APPLICATION)
	$(call echo_var,DEVELOPER_ID_INSTALLER)
	$(call echo_var,CSC_NAME)
	$(call echo_var,TELEPORT_BUNDLEID)
	$(call echo_var,TSH_BUNDLEID)
	$(call echo_var,TSH_SKELETON)
	$(call echo_var,TCTL_BUNDLEID)
	$(call echo_var,TCTL_SKELETON)
