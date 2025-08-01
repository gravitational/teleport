package teleport

const (
	// ComponentSAMLIdP is a SAML identity provider component.
	//
	//nolint:revive // Because we want this to be IdP.
	ComponentSAMLIdP = "idp.saml"

	// ComponentOkta is an Okta service component.
	ComponentOkta = "okta"

	// ComponentOktaClient is an Okta API client component.
	ComponentOktaClient = "okta:client"

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

	// ComponentIntune is the Intune service component.
	ComponentIntune = "intune"

	// ComponentPluginManager is a plugin manager component.
	ComponentPluginManager = "pluginmanager"

	// ComponentUserMonitor is a user monitor component.
	ComponentUserMonitor = "usermonitor"

	// ComponentGitlab is the Gitlab service component.
	ComponentGitlab = "gitlab"

	// ComponentGithub is the Github service component.
	ComponentGithub = "github"

	// ComponentEntraID is the Entra ID service component.
	ComponentEntraID = "entra-id"

	// ComponentNetIQ is the NetIQ service component.
	ComponentNetIQ = "netiq"

	// ComponentAWSIC is the AWS IAM Identity Center component.
	ComponentAWSIC = "aws:ic"
	// ComponentAWSICPrincipalProvisioner is the AWS IAM Identity Center
	// principal provisioner component.
	ComponentAWSICPrincipalProvisioner = ComponentAWSIC + ":pr"
	// ComponentAWSICAssignmentProvisioner is the AWS IAM Identity Center
	// permission assignment provisioner component.
	ComponentAWSICAssignmentProvisioner = ComponentAWSIC + ":ap"
	// ComponentAWSICAssignmentCalculator is the AWS IAM Identity Center
	// assignment calculator component.
	ComponentAWSICAssignmentCalculator = ComponentAWSIC + ":ac"
	// ComponentAWSICResourceMonitor is the AWS IAM Identity Center
	// resource monitor component.
	ComponentAWSICResourceMonitor = ComponentAWSIC + ":rm"
	// ComponentAWSICSDK is the AWS IAM Identity Center SDK component.
	ComponentAWSICSDK = ComponentAWSIC + ":sd"
)
