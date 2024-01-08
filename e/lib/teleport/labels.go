package teleport

import "github.com/gravitational/teleport/api/types"

const (
	// OktaOrgURLLabel is the label for which Okta organization an object belongs to.
	OktaOrgURLLabel = "okta/org"

	// OktaGroupIDLabel is the label for the Okta group ID on user group objects.
	OktaGroupIDLabel = types.TeleportInternalLabelPrefix + "okta-group-id"

	// OktaAppIDLabel is the label for the Okta application ID on appserver objects.
	OktaAppIDLabel = types.TeleportInternalLabelPrefix + "okta-app-id"

	// OktaUserIDLabel is the label for the Okta user ID on User objects.
	OktaUserIDLabel = types.TeleportInternalLabelPrefix + "okta-user-id"

	// OktaUserStatusLabel is the label for the Okta user status on User objects.
	OktaUserStatusLabel = types.TeleportInternalLabelPrefix + "okta-user-status"

	// OktaLockReasonLabel is the label for the lock reason on Lock objects.
	OktaLockReasonLabel = types.TeleportInternalLabelPrefix + "okta-lock-reason"

	// OktaAssignmentSourceLabel is the label for the source of the Okta assignment.
	OktaAssignmentSourceLabel = types.TeleportInternalLabelPrefix + "source"

	// OktaTraitPrefix is a prefix added to traits sourced from the Okta user DB
	OktaTraitPrefix = "okta/"

	// PluginLabel is a unique label generated for plugins that use static credentials
	// in order to ensure that the static credentials are only readable by specific plugins.
	PluginLabel = types.TeleportInternalLabelPrefix + "plugin"
)
