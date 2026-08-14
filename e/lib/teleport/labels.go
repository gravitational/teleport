package teleport

import "github.com/gravitational/teleport/api/types"

const (
	// OktaOrgURLLabel is the label for which Okta organization an object belongs to.
	OktaOrgURLLabel = types.OktaOrgURLLabel

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

	// EntraMemberOfGroupTrait is the trait that an access list imported from Entra ID assigns to its members.
	EntraMemberOfGroupTrait = "entra/member-of-group"

	// OktaAppHiddenLabel is the label that indicates that an Okta app is hidden from user Okta portal.
	OktaAppHiddenLabel = types.TeleportInternalLabelPrefix + "okta-app-hidden"

	// OktaACLReviewerRoleLabel is the label that indicates that a role is an Okta ACL reviewer role.
	OktaACLReviewerRoleLabel = types.TeleportInternalLabelPrefix + "okta-accesslist-reviewer-role"

	// SCIMAttrsLabel to store SCIM attributes from SCIM create/update requests. They are
	// needed to be able to adhere to the SCIM spec but we are not yet sure how we'd like to
	// structure them in the user type.
	SCIMAttrsLabel = types.TeleportInternalLabelPrefix + "scim-attrs"
)
