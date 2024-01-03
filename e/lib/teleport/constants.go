package teleport

const (
	// ComponentSAMLIdP is a SAML identity provider component.
	//
	//nolint:revive // Because we want this to be IdP.
	ComponentSAMLIdP = "idp.saml"

	// ComponentOkta is an Okta service component.
	ComponentOkta = "okta"

	// ComponentOktaAssignmentReconciler is an Okta assignment reconciler component.
	ComponentOktaAssignmentReconciler = "okta.assignment-reconciler"

	// ComponentOktaAccessRequestReconciler is an access request reconciler component.
	ComponentOktaAccessRequestReconciler = "okta.access-request-reconciler"

	// ComponentOktaUserAssignmentCreator is a user assignment creator component.
	ComponentOktaUserAssignmentCreator = "okta.user-assignment-creator"

	// ComponentOktaConnected is a Okta connected component.
	ComponentOktaConnected = "okta.connected"

	// ComponentJamf is the Jamf service component.
	ComponentJamf = "jamf"

	// ComponentPluginManager is a plugin manager component.
	ComponentPluginManager = "pluginmanager"

	// ComponentUserMonitor is a user monitor component.
	ComponentUserMonitor = "usermonitor"
)
