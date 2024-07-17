/*
Copyright 2015-2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package webclient

import (
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/utils/keys"
)

const (
	// WebConfigAuthProviderOIDCType is OIDC provider type
	WebConfigAuthProviderOIDCType = "oidc"
	// WebConfigAuthProviderOIDCURL is OIDC webapi endpoint.
	// redirect_url MUST be the last query param, see the comment in parseSSORequestParams for an explanation.
	WebConfigAuthProviderOIDCURL = "/v1/webapi/oidc/login/web?connector_id=:providerName&redirect_url=:redirect"

	// WebConfigAuthProviderSAMLType is SAML provider type
	WebConfigAuthProviderSAMLType = "saml"
	// WebConfigAuthProviderSAMLURL is SAML webapi endpoint.
	// redirect_url MUST be the last query param, see the comment in parseSSORequestParams for an explanation.
	WebConfigAuthProviderSAMLURL = "/v1/webapi/saml/sso?connector_id=:providerName&redirect_url=:redirect"

	// WebConfigAuthProviderGitHubType is GitHub provider type
	WebConfigAuthProviderGitHubType = "github"
	// WebConfigAuthProviderGitHubURL is GitHub webapi endpoint
	// redirect_url MUST be the last query param, see the comment in parseSSORequestParams for an explanation.
	WebConfigAuthProviderGitHubURL = "/v1/webapi/github/login/web?connector_id=:providerName&redirect_url=:redirect"
)

// WebConfig is web application configuration served by the backend to be used in frontend apps.
type WebConfig struct {
	// Auth contains Teleport auth. preferences
	Auth WebConfigAuthSettings `json:"auth,omitempty"`
	// CanJoinSessions disables joining sessions
	CanJoinSessions bool `json:"canJoinSessions"`
	// ProxyClusterName is the name of the local cluster
	ProxyClusterName string `json:"proxyCluster,omitempty"`
	// IsCloud is a flag that determines if cloud features are enabled.
	IsCloud bool `json:"isCloud,omitempty"`
	// TunnelPublicAddress is the public ssh tunnel address
	TunnelPublicAddress string `json:"tunnelPublicAddress,omitempty"`
	// RecoveryCodesEnabled is a flag that determines if recovery codes are enabled in the cluster.
	RecoveryCodesEnabled bool `json:"recoveryCodesEnabled,omitempty"`
	// UIConfig is the configuration for the web UI
	UI UIConfig `json:"ui,omitempty"`
	// IsDashboard is a flag that determines if the cluster is running as a "dashboard".
	// The web UI for dashboards provides functionality for downloading self-hosted licenses and
	// Teleport Enterprise binaries.
	IsDashboard bool `json:"isDashboard,omitempty"`
	// IsUsageBasedBilling determines if the cloud user subscription is usage-based (pay-as-you-go).
	IsUsageBasedBilling bool `json:"isUsageBasedBilling,omitempty"`
	// AutomaticUpgrades describes whether agents should automatically upgrade.
	AutomaticUpgrades bool `json:"automaticUpgrades"`
	// AutomaticUpgradesTargetVersion is the agents version (eg kube agent helm chart) that should be installed.
	// Eg, v13.4.3
	// Only present when AutomaticUpgrades are enabled.
	AutomaticUpgradesTargetVersion string `json:"automaticUpgradesTargetVersion,omitempty"`
	// CustomTheme is a string that represents the name of the custom theme that the WebUI should use.
	CustomTheme string `json:"customTheme"`
	// Questionnaire indicates whether cluster users should get an onboarding questionnaire
	Questionnaire bool `json:"questionnaire"`
	// IsStripeManaged indicates if the cluster billing & lifecycle is managed via Stripe
	IsStripeManaged bool `json:"isStripeManaged"`
	// PremiumSupport indicates whether the customer has premium support
	PremiumSupport bool `json:"premiumSupport"`
	// Edition is the edition of Teleport
	Edition string `json:"edition"`
	// PlayableDatabaseProtocols is a list of database protocols which session
	// recordings can be played.
	PlayableDatabaseProtocols []string `json:"playable_db_protocols"`
	// entitlements define a customer’s access to a specific features
	Entitlements map[string]EntitlementInfo `json:"entitlements,omitempty"`

	// Deprecated Fields
	// Deprecated: IsTeam is true if [Features.ProductType] = Team
	// Prefer checking the cluster features over this flag, as this will be removed.
	IsTeam bool `json:"isTeam"`
	// HideInaccessibleFeatures is true when features should be undiscoverable to users without the necessary permissions.
	// Usually, in order to encourage discoverability of features, we show UI elements even if the user doesn't have permission to access them,
	// this flag disables that behavior.
	// Deprecated, use entitlements
	HideInaccessibleFeatures bool `json:"hideInaccessibleFeatures"`
	// IsIGSEnabled is true if [Features.IdentityGovernance] = true
	// Deprecated, use entitlements
	IsIGSEnabled bool `json:"isIgsEnabled"`
	// IsPolicyEnabled is true if [Features.Policy] = true
	// Deprecated, use entitlements
	IsPolicyEnabled bool `json:"isPolicyEnabled"`
	// featureLimits define limits for features.
	// Typically used with feature teasers if feature is not enabled for the
	// product type eg: Team product contains teasers to upgrade to Enterprise.
	// Deprecated, use entitlements
	FeatureLimits FeatureLimits `json:"featureLimits"`
	// ExternalAuditStorage indicates whether the EAS feature is enabled in the cluster.
	// Deprecated, use entitlements
	ExternalAuditStorage bool `json:"externalAuditStorage"`
	// JoinActiveSessions indicates whether joining active sessions via web UI is enabled
	// Deprecated, use entitlements
	JoinActiveSessions bool `json:"joinActiveSessions"`
	// AccessRequests indicates whether access requests are enabled
	// Deprecated, use entitlements
	AccessRequests bool `json:"accessRequests"`
	// TrustedDevices indicates whether trusted devices page is enabled
	// Deprecated, use entitlements
	TrustedDevices bool `json:"trustedDevices"`
	// OIDC indicates whether the OIDC integration flow is enabled
	// Deprecated, use entitlements
	OIDC bool `json:"oidc"`
	// SAML indicates whether the SAML integration flow is enabled
	// Deprecated, use entitlements
	SAML bool `json:"saml"`
	// MobileDeviceManagement indicates whether adding Jamf plugin is enabled
	// Deprecated, use entitlements
	MobileDeviceManagement bool `json:"mobileDeviceManagement"`
}

// EntitlementInfo is the state and limits of a particular entitlement; Example for feature X:
// { Enabled: true,  Limit: 0 }   => unlimited access to feature X
// { Enabled: true,  Limit: >0 }  => limited access to feature X
// { Enabled: false, Limit: >=0 } => no access to feature X
type EntitlementInfo struct {
	// Enabled indicates the feature is 'on' if true; feature is disabled if false
	Enabled bool `json:"enabled"`
	// Limit indicates the allotted amount of use when limited; if 0 use is unlimited
	Limit int32 `json:"limit"`
}

// featureLimits define limits for features.
// Typically used with feature teasers if feature is not enabled for the
// product type eg: Team product contains teasers to upgrade to Enterprise.
type FeatureLimits struct {
	// Limit for the number of access list creatable when feature is
	// not enabled.
	AccessListCreateLimit int `json:"accessListCreateLimit"`
	// Defines the max number of days to include in an access report if
	// feature is not enabled.
	AccessMonitoringMaxReportRangeLimit int `json:"accessMonitoringMaxReportRangeLimit"`
	// AccessRequestMonthlyRequestLimit is the usage-based limit for the number of
	// access requests created in a calendar month.
	AccessRequestMonthlyRequestLimit int `json:"AccessRequestMonthlyRequestLimit"`
}

// UIConfig provides config options for the web UI served by the proxy service.
type UIConfig struct {
	// ScrollbackLines is the max number of lines the UI terminal can display in its history
	ScrollbackLines int `json:"scrollbackLines,omitempty"`
	// ShowResources determines which resources are shown in the web UI. Default if unset is "requestable"
	// which means resources the user has access to and resources they can request will be shown in the
	// resources UI. If set to `accessible_only`, only resources the user already has access to will be shown.
	ShowResources constants.ShowResources `json:"showResources,omitempty"`
}

// WebConfigAuthProvider describes auth. provider
type WebConfigAuthProvider struct {
	// Name is this provider ID
	Name string `json:"name,omitempty"`
	// DisplayName is this provider display name
	DisplayName string `json:"displayName,omitempty"`
	// Type is this provider type
	Type string `json:"type,omitempty"`
	// WebAPIURL is this provider webapi URL
	WebAPIURL string `json:"url,omitempty"`
}

// WebConfigAuthSettings describes auth configuration
type WebConfigAuthSettings struct {
	// SecondFactor is the type of second factor to use in authentication.
	SecondFactor constants.SecondFactorType `json:"second_factor,omitempty"`
	// Providers contains a list of configured auth providers
	Providers []WebConfigAuthProvider `json:"providers,omitempty"`
	// LocalAuthEnabled is a flag that enables local authentication
	LocalAuthEnabled bool `json:"localAuthEnabled"`
	// AllowPasswordless is true if passwordless logins are allowed.
	AllowPasswordless bool `json:"allowPasswordless,omitempty"`
	// AuthType is the authentication type.
	AuthType string `json:"authType"`
	// PreferredLocalMFA is a server-side hint for clients to pick an MFA method
	// when various options are available.
	// It is empty if there is nothing to suggest.
	PreferredLocalMFA constants.SecondFactorType `json:"preferredLocalMfa,omitempty"`
	// LocalConnectorName is the name of the local connector.
	LocalConnectorName string `json:"localConnectorName,omitempty"`
	// PrivateKeyPolicy is the configured private key policy for the cluster.
	PrivateKeyPolicy keys.PrivateKeyPolicy `json:"privateKeyPolicy,omitempty"`
	// MOTD is message of the day. MOTD is displayed to users before login.
	MOTD string `json:"motd"`
}
