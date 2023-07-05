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
	// IsDashboard is a flag that determines if the cluster is running as a "dashboard".
	// The web UI for dashboards provides functionality for downloading self-hosted licenses and
	// Teleport Enterprise binaries.
	IsDashboard bool `json:"isDashboard,omitempty"`
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
