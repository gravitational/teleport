package ui

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// OktaPluginSpec holds information about the Okta plugin.
type OktaPluginSpec struct {
	// SCIMBearerToken is the plain text of the bearer token that Okta will use
	// to authenticate SCIM requests
	SCIMBearerToken string `json:"scimBearerToken,omitempty"`

	// OktaOrgURL is the Okta org's base URL
	OktaOrgURL string `json:"orgUrl,omitempty"`

	// OktaAppID is the Okta ID of the SAML App created during the Okta plugin
	// installation
	OktaAppID string `json:"oktaAppId,omitempty"`

	// OktaAppLabel is the human readable name of the Okta SAML app created
	// during the Okta plugin installation
	OktaAppLabel string `json:"oktaAppLabel,omitempty"`

	// OktaAppName is the unique app name of the Okta SAML app created during
	// the plugin installation
	OktaAppName string `json:"oktaAppName,omitempty"`

	// TeleportSSOConnector is the name of the Teleport SAML SSO connector
	// created by the plugin during installation
	TeleportSSOConnector string `json:"teleportSsoConnector,omitempty"`

	// DefaultOwners is the set of usernames that the integration assigns as
	// owners to any Access Lists that it creates
	DefaultOwners []string `json:"defaultOwners,omitempty"`

	// EnableUserSync is a flag indicating whether User Sync is enabled for
	// the plugin, regardless of whether it is currently running.
	EnableUserSync bool `json:"enableUserSync,omitempty"`
	// AssignDefaultRoles indicates whether the builtin okta-requester role should be
	// assigned to the synchronized users.
	AssignDefaultRoles bool `json:"assignDefaultRoles,omitempty"`
	// EnableAccessListSync indicates whether Access List Sync is enabled for
	// the plugin, regardless of whether it is currently running.
	EnableAccessListSync bool `json:"enableAccessListSync,omitempty"`
	// EnableAppGroupSync indicates whether App Group Sync is enabled for the
	// plugin, regardless of whether it is currently running.
	EnableAppGroupSync bool `json:"enableAppGroupSync,omitempty"`
	// EnableBidirectionalSync indicates whether changes made in Teleport
	// should be synced back to Okta.
	EnableBidirectionalSync bool `json:"enableBidirectionalSync,omitempty"`
	// EnableSystemLogExport indicates whether the Teleport Identity Security SIEM integration for Okta should be enabled.
	EnableSystemLogExport bool `json:"enableSystemLogExport,omitempty"`

	// CredentialInfo holds information about configured credentials in the plugin.
	CredentialInfo *OktaCredentialInfo `json:"credentialsInfo,omitempty"`

	// Error contains a description of any failures during plugin installation
	// that were deemed not serious enough to fail the plugin installation, but
	// may effect the operation of advanced features like User Sync or SCIM.
	Error string `json:"error,omitempty"`
}

// OktaCredentialInfo holds information about configured credentials in the Okta plugin.
type OktaCredentialInfo struct {
	// HasConfiguredSSMSToken represents whether the plugin has a saved SSM bearer token.
	HasConfiguredSSMSToken bool `json:"hasConfiguredSSMSToken,omitempty"`
	// HasConfiguredOauthCredentials represents whether the plugin has saved OAuth credentials.
	HasConfiguredOauthCredentials bool `json:"hasConfiguredOauthCredentials,omitempty"`
	// HasConfiguredSCIMToken represents whether the plugin has a saved SCIM bearer token.
	HasConfiguredSCIMToken bool `json:"hasConfiguredSCIMToken,omitempty"`
}

// PluginSpecType implements PluginSpec for OktaPluginSpec
func (*OktaPluginSpec) PluginSpecType() types.PluginType {
	return types.PluginTypeOkta
}

// makeOktaDetails generates UI-ready status details for the given plugin
// instance.
func makeOktaDetails(plugin *types.PluginV1) (*PluginDetails, error) {
	if plugin.GetType() != types.PluginTypeOkta {
		return nil, trace.BadParameter("only okta plugins supported")
	}

	oktaSettings := plugin.Spec.GetOkta()
	if oktaSettings == nil {
		return nil, trace.BadParameter("malformed okta plugin")
	}

	pluginStatusDetails := plugin.Status.GetOkta()
	if pluginStatusDetails == nil {
		return nil, trace.BadParameter("malformed okta plugin - missing status")
	}

	detailedStatus := &PluginDetails{Okta: pluginStatusDetails}

	return detailedStatus, nil
}

// PluginConfigOktaGroup is the plugin configuration of the Okta plugin.
type PluginConfigOktaGroup struct {
	// Name is the name of the group.
	Name string `json:"name"`
	// Description is the description of the group.
	Description string `json:"description,omitempty"`
}

// PluginConfigOktaApp is a representation of an Okta app for display during the
// plugin configuration of the Okta plugin.
type PluginConfigOktaApp struct {
	Name string `json:"name"`
}
