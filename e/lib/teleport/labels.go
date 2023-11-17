package teleport

import "github.com/gravitational/teleport/api/types"

const (
	// OktaOrgURLLabel is the label for which Okta organization an object belongs to.
	OktaOrgURLLabel = "okta/org"

	// OktaGroupIDLabel is the label for the Okta group ID on user group objects.
	OktaGroupIDLabel = types.TeleportInternalLabelPrefix + "okta-group-id"

	// OktaAppIDLabel is the label for the Okta application ID on appserver objects.
	OktaAppIDLabel = types.TeleportInternalLabelPrefix + "okta-app-id"

	// OktaAssignmentSourceLabel is the label for the source of the Okta assignment.
	OktaAssignmentSourceLabel = types.TeleportInternalLabelPrefix + "source"

	// PluginLabel is a unique label generated for plugins that use static credentials
	// in order to ensure that the static credentials are only readable by specific plugins.
	PluginLabel = types.TeleportInternalLabelPrefix + "plugin"
)
