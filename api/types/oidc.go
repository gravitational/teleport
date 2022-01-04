/*
Copyright 2020 Gravitational, Inc.

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

package types

import (
	"time"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/utils"

	"github.com/gravitational/trace"
)

// OIDCConnector specifies configuration for Open ID Connect compatible external
// identity provider, e.g. google in some organisation
type OIDCConnector interface {
	// ResourceWithSecrets provides common methods for objects
	ResourceWithSecrets
	// Issuer URL is the endpoint of the provider, e.g. https://accounts.google.com
	GetIssuerURL() string
	// ClientID is id for authentication client (in our case it's our Auth server)
	GetClientID() string
	// ClientSecret is used to authenticate our client and should not
	// be visible to end user
	GetClientSecret() string
	// RedirectURL - Identity provider will use this URL to redirect
	// client's browser back to it after successful authentication
	// Should match the URL on Provider's side
	GetRedirectURL() string
	// GetACR returns the Authentication Context Class Reference (ACR) value.
	GetACR() string
	// GetProvider returns the identity provider.
	GetProvider() string
	// Display - Friendly name for this provider.
	GetDisplay() string
	// Scope is additional scopes set by provider
	GetScope() []string
	// ClaimsToRoles specifies dynamic mapping from claims to roles
	GetClaimsToRoles() []ClaimMapping
	// GetClaims returns list of claims expected by mappings
	GetClaims() []string
	// GetTraitMappings converts gets all claim mappings in the
	// generic trait mapping format.
	GetTraitMappings() TraitMappingSet
	// SetClientSecret sets client secret to some value
	SetClientSecret(secret string)
	// SetClientID sets id for authentication client (in our case it's our Auth server)
	SetClientID(string)
	// SetIssuerURL sets the endpoint of the provider
	SetIssuerURL(string)
	// SetRedirectURL sets RedirectURL
	SetRedirectURL(string)
	// SetPrompt sets OIDC prompt value
	SetPrompt(string)
	// GetPrompt returns OIDC prompt value,
	GetPrompt() string
	// SetACR sets the Authentication Context Class Reference (ACR) value.
	SetACR(string)
	// SetProvider sets the identity provider.
	SetProvider(string)
	// SetScope sets additional scopes set by provider
	SetScope([]string)
	// SetClaimsToRoles sets dynamic mapping from claims to roles
	SetClaimsToRoles([]ClaimMapping)
	// SetDisplay sets friendly name for this provider.
	SetDisplay(string)
	// GetGoogleServiceAccountURI returns path to google service account URI
	GetGoogleServiceAccountURI() string
	// GetGoogleServiceAccount returns google service account json for Google
	GetGoogleServiceAccount() string
	// SetGoogleServiceAccount sets the google service account json contents
	SetGoogleServiceAccount(string)
	// GetGoogleAdminEmail returns a google admin user email
	// https://developers.google.com/identity/protocols/OAuth2ServiceAccount#delegatingauthority
	// "Note: Although you can use service accounts in applications that run from a Google Workspace (formerly G Suite) domain, service accounts are not members of your Google Workspace account and aren’t subject to domain policies set by  administrators. For example, a policy set in the Google Workspace admin console to restrict the ability of end users to share documents outside of the domain would not apply to service accounts."
	GetGoogleAdminEmail() string
	// GetGoogleTransitiveGroups returns true if the connector is meant to fetch
	// the list of transitive groups or just the groups that users directly
	// belong to.
	GetGoogleTransitiveGroups() bool
}

// NewOIDCConnector returns a new OIDCConnector based off a name and OIDCConnectorSpecV2.
func NewOIDCConnector(name string, spec OIDCConnectorSpecV2) (OIDCConnector, error) {
	o := &OIDCConnectorV2{
		Metadata: Metadata{
			Name: name,
		},
		Spec: spec,
	}
	if err := o.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return o, nil
}

// SetPrompt sets OIDC prompt value
func (o *OIDCConnectorV2) SetPrompt(p string) {
	o.Spec.Prompt = p
}

// GetPrompt returns OIDC prompt value,
// * if not set, default to select_account for backwards compatibility
// * if set to none, it will be omitted
// * and any other non empty value, pass it as is
func (o *OIDCConnectorV2) GetPrompt() string {
	if o.Spec.Prompt == "" {
		return constants.OIDCPromptSelectAccount
	}
	if o.Spec.Prompt == constants.OIDCPromptNone {
		return ""
	}
	return o.Spec.Prompt
}

// GetGoogleServiceAccountURI returns an optional path to google service account file
func (o *OIDCConnectorV2) GetGoogleServiceAccountURI() string {
	return o.Spec.GoogleServiceAccountURI
}

// GetGoogleServiceAccount returns a string representing a Google service account
func (o *OIDCConnectorV2) GetGoogleServiceAccount() string {
	return o.Spec.GoogleServiceAccount
}

// SetGoogleServiceAccount sets a string representing a Google service account
func (o *OIDCConnectorV2) SetGoogleServiceAccount(s string) {
	o.Spec.GoogleServiceAccount = s
}

// GetGoogleAdminEmail returns a google admin user email
func (o *OIDCConnectorV2) GetGoogleAdminEmail() string {
	return o.Spec.GoogleAdminEmail
}

// GetGoogleTransitiveGroups returns true if we should fetch transitive groups
func (o *OIDCConnectorV2) GetGoogleTransitiveGroups() bool {
	return o.Spec.GoogleTransitiveGroups
}

// GetVersion returns resource version
func (o *OIDCConnectorV2) GetVersion() string {
	return o.Version
}

// GetSubKind returns resource sub kind
func (o *OIDCConnectorV2) GetSubKind() string {
	return o.SubKind
}

// SetSubKind sets resource subkind
func (o *OIDCConnectorV2) SetSubKind(s string) {
	o.SubKind = s
}

// GetKind returns resource kind
func (o *OIDCConnectorV2) GetKind() string {
	return o.Kind
}

// GetResourceID returns resource ID
func (o *OIDCConnectorV2) GetResourceID() int64 {
	return o.Metadata.ID
}

// SetResourceID sets resource ID
func (o *OIDCConnectorV2) SetResourceID(id int64) {
	o.Metadata.ID = id
}

// WithoutSecrets returns an instance of resource without secrets.
func (o *OIDCConnectorV2) WithoutSecrets() Resource {
	if o.GetClientSecret() == "" && o.GetGoogleServiceAccount() == "" {
		return o
	}
	o2 := *o

	o2.SetClientSecret("")
	o2.SetGoogleServiceAccount("")

	return &o2
}

// V2 returns V2 version of the resource
func (o *OIDCConnectorV2) V2() *OIDCConnectorV2 {
	return o
}

// SetDisplay sets friendly name for this provider.
func (o *OIDCConnectorV2) SetDisplay(display string) {
	o.Spec.Display = display
}

// GetMetadata returns object metadata
func (o *OIDCConnectorV2) GetMetadata() Metadata {
	return o.Metadata
}

// SetExpiry sets expiry time for the object
func (o *OIDCConnectorV2) SetExpiry(expires time.Time) {
	o.Metadata.SetExpiry(expires)
}

// Expiry returns object expiry setting
func (o *OIDCConnectorV2) Expiry() time.Time {
	return o.Metadata.Expiry()
}

// GetName returns the name of the connector
func (o *OIDCConnectorV2) GetName() string {
	return o.Metadata.GetName()
}

// SetName sets client secret to some value
func (o *OIDCConnectorV2) SetName(name string) {
	o.Metadata.SetName(name)
}

// SetIssuerURL sets client secret to some value
func (o *OIDCConnectorV2) SetIssuerURL(issuerURL string) {
	o.Spec.IssuerURL = issuerURL
}

// SetRedirectURL sets client secret to some value
func (o *OIDCConnectorV2) SetRedirectURL(redirectURL string) {
	o.Spec.RedirectURL = redirectURL
}

// SetACR sets the Authentication Context Class Reference (ACR) value.
func (o *OIDCConnectorV2) SetACR(acrValue string) {
	o.Spec.ACR = acrValue
}

// SetProvider sets the identity provider.
func (o *OIDCConnectorV2) SetProvider(identityProvider string) {
	o.Spec.Provider = identityProvider
}

// SetScope sets additional scopes set by provider
func (o *OIDCConnectorV2) SetScope(scope []string) {
	o.Spec.Scope = scope
}

// SetClaimsToRoles sets dynamic mapping from claims to roles
func (o *OIDCConnectorV2) SetClaimsToRoles(claims []ClaimMapping) {
	o.Spec.ClaimsToRoles = claims
}

// SetClientID sets id for authentication client (in our case it's our Auth server)
func (o *OIDCConnectorV2) SetClientID(clintID string) {
	o.Spec.ClientID = clintID
}

// SetClientSecret sets client secret to some value
func (o *OIDCConnectorV2) SetClientSecret(secret string) {
	o.Spec.ClientSecret = secret
}

// GetIssuerURL is the endpoint of the provider, e.g. https://accounts.google.com
func (o *OIDCConnectorV2) GetIssuerURL() string {
	return o.Spec.IssuerURL
}

// GetClientID is id for authentication client (in our case it's our Auth server)
func (o *OIDCConnectorV2) GetClientID() string {
	return o.Spec.ClientID
}

// GetClientSecret is used to authenticate our client and should not
// be visible to end user
func (o *OIDCConnectorV2) GetClientSecret() string {
	return o.Spec.ClientSecret
}

// GetRedirectURL - Identity provider will use this URL to redirect
// client's browser back to it after successful authentication
// Should match the URL on Provider's side
func (o *OIDCConnectorV2) GetRedirectURL() string {
	return o.Spec.RedirectURL
}

// GetACR returns the Authentication Context Class Reference (ACR) value.
func (o *OIDCConnectorV2) GetACR() string {
	return o.Spec.ACR
}

// GetProvider returns the identity provider.
func (o *OIDCConnectorV2) GetProvider() string {
	return o.Spec.Provider
}

// GetDisplay - Friendly name for this provider.
func (o *OIDCConnectorV2) GetDisplay() string {
	if o.Spec.Display != "" {
		return o.Spec.Display
	}
	return o.GetName()
}

// GetScope is additional scopes set by provider
func (o *OIDCConnectorV2) GetScope() []string {
	return o.Spec.Scope
}

// GetClaimsToRoles specifies dynamic mapping from claims to roles
func (o *OIDCConnectorV2) GetClaimsToRoles() []ClaimMapping {
	return o.Spec.ClaimsToRoles
}

// GetClaims returns list of claims expected by mappings
func (o *OIDCConnectorV2) GetClaims() []string {
	var out []string
	for _, mapping := range o.Spec.ClaimsToRoles {
		out = append(out, mapping.Claim)
	}
	return utils.Deduplicate(out)
}

// GetTraitMappings returns the OIDCConnector's TraitMappingSet
func (o *OIDCConnectorV2) GetTraitMappings() TraitMappingSet {
	tms := make([]TraitMapping, 0, len(o.Spec.ClaimsToRoles))
	for _, mapping := range o.Spec.ClaimsToRoles {
		tms = append(tms, TraitMapping{
			Trait: mapping.Claim,
			Value: mapping.Value,
			Roles: mapping.Roles,
		})
	}
	return TraitMappingSet(tms)
}

// setStaticFields sets static resource header and metadata fields.
func (o *OIDCConnectorV2) setStaticFields() {
	o.Kind = KindOIDCConnector
	o.Version = V2
}

// CheckAndSetDefaults checks and set default values for any missing fields.
func (o *OIDCConnectorV2) CheckAndSetDefaults() error {
	o.setStaticFields()
	if err := o.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if o.Metadata.Name == constants.Local {
		return trace.BadParameter("ID: invalid connector name, %v is a reserved name", constants.Local)
	}
	if o.Spec.ClientID == "" {
		return trace.BadParameter("ClientID: missing client id")
	}

	// make sure claim mappings have either roles or a role template
	for _, v := range o.Spec.ClaimsToRoles {
		if len(v.Roles) == 0 {
			return trace.BadParameter("add roles in claims_to_roles")
		}
	}

	return nil
}
