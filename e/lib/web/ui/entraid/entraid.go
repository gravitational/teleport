package entraid

import (
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/plugins/filter"
)

// EntraPluginSpec holds UI specific fields for the Microsoft Entra ID plugin.
type EntraPluginSpec struct {
	// DefaultOwners are the default owners for all the imported access lists.
	DefaultOwners []string `json:"defaultOwners,omitempty"`
	// AccessListOwnersSource is the source of the Access List owners.
	AccessListOwnersSource string `json:"accessListOwnersSource,omitempty"`
	// SSOConnectorID is the name of the Teleport SSO connector created and
	// used by the Entra ID plugin.
	SSOConnectorID string `json:"ssoConnectorId,omitempty"`
	// CredentialsSource specifies the source of the credentials used to
	// authenticate with the Graph API.
	CredentialsSource string `json:"credentialSource,omitempty"`
	// TenantID is the Microsoft Entra ID tenant ID.
	TenantID string `json:"tenantId,omitempty"`
	// EntraAppID is the Microsoft Entra ID enterprise application ID.
	// The enterprise application is used to configure SSO and Graph API permissions.
	EntraAppID string `json:"entraAppId,omitempty"`
	// GroupFilters configures which groups should be included or excluded.
	GroupFilters filter.Inputs `json:"groupFilters,omitempty"`
	// AccessGraphEnabled is the enabled/disabled state of the Access Graph sync config.
	// Note: The proto PluginEntraIDSettings type does not maintain boolean state.
	// It only stores the AppSsoSettingsCache (Microsoft Entra ID SSO settings) field
	// and the plugin runtime treats existence of this cache value as an "enabled"
	// state. AppSsoSettingsCache value can be large and the UI only needs the
	// enabled/disabled state.
	AccessGraphEnabled bool `json:"accessGraphEnabled,omitempty"`
}

// PluginSpecType implements PluginSpec for EntraPluginSpec
func (*EntraPluginSpec) PluginSpecType() types.PluginType {
	return types.PluginTypeEntraID
}

// AccessGraphSyncEnabled returns true if [AppSsoSettingsCache] is set.
func AccessGraphSyncEnabled(in *types.PluginEntraIDAccessGraphSettings) bool {
	return in != nil && len(in.AppSsoSettingsCache) > 0
}
