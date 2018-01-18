/*
Copyright 2015 Gravitational, Inc.

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

package ui

const (
	// WebConfigAuthProviderOIDCType is OIDC provider type
	WebConfigAuthProviderOIDCType = "oidc"
	// WebConfigAuthProviderOIDCURL is OIDC webapi endpoint
	WebConfigAuthProviderOIDCURL = "/v1/webapi/oidc/login/web?redirect_url=:redirect&connector_id=:providerName"

	// WebConfigAuthProviderSAMLType is SAML provider type
	WebConfigAuthProviderSAMLType = "saml"
	// WebConfigAuthProviderSAMLURL is SAML webapi endpoint
	WebConfigAuthProviderSAMLURL = "/v1/webapi/saml/sso?redirect_url=:redirect&connector_id=:providerName"

	// WebConfigAuthProviderGitHubType is GitHub provider type
	WebConfigAuthProviderGitHubType = "github"
	// WebConfigAuthProviderGitHubURL is GitHub webapi endpoint
	WebConfigAuthProviderGitHubURL = "/v1/webapi/github/login/web?redirect_url=:redirect&connector_id=:providerName"
)

// WebConfig is web application configuration
type WebConfig struct {
	// Auth contains Teleport auth. preferences
	Auth WebConfigAuthSettings `json:"auth,omitempty"`
	// ServerVersion is the version of Teleport that is running.
	ServerVersion string `json:"serverVersion"`
	// CanJoinSessions disables joining sessions
	CanJoinSessions bool `json:"canJoinSessions"`
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
	SecondFactor string `json:"second_factor,omitempty"`
	// Providers contains a list of configured auth providers
	Providers []WebConfigAuthProvider `json:"providers,omitempty"`
}
