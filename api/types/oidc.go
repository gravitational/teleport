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
	"net/url"
	"time"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/utils"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

// OIDCConnector specifies configuration for Open ID Connect compatible external
// identity provider, e.g. google in some organization
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
	// GetRedirectURLs returns list of redirect URLs.
	GetRedirectURLs() []string
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
	// SetRedirectURLs sets the list of redirectURLs
	SetRedirectURLs([]string)
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
}

// NewOIDCConnector returns a new OIDCConnector based off a name and OIDCConnectorSpecV3.
func NewOIDCConnector(name string, spec OIDCConnectorSpecV3) (OIDCConnector, error) {
	o := &OIDCConnectorV3{
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
func (o *OIDCConnectorV3) SetPrompt(p string) {
	o.Spec.Prompt = p
}

// GetPrompt returns OIDC prompt value,
// * if not set, default to select_account for backwards compatibility
// * if set to none, it will be omitted
// * and any other non empty value, pass it as is
func (o *OIDCConnectorV3) GetPrompt() string {
	if o.Spec.Prompt == "" {
		return constants.OIDCPromptSelectAccount
	}
	if o.Spec.Prompt == constants.OIDCPromptNone {
		return ""
	}
	return o.Spec.Prompt
}

// GetGoogleServiceAccountURI returns an optional path to google service account file
func (o *OIDCConnectorV3) GetGoogleServiceAccountURI() string {
	return o.Spec.GoogleServiceAccountURI
}

// GetGoogleServiceAccount returns a string representing a Google service account
func (o *OIDCConnectorV3) GetGoogleServiceAccount() string {
	return o.Spec.GoogleServiceAccount
}

// SetGoogleServiceAccount sets a string representing a Google service account
func (o *OIDCConnectorV3) SetGoogleServiceAccount(s string) {
	o.Spec.GoogleServiceAccount = s
}

// GetGoogleAdminEmail returns a google admin user email
func (o *OIDCConnectorV3) GetGoogleAdminEmail() string {
	return o.Spec.GoogleAdminEmail
}

// GetVersion returns resource version
func (o *OIDCConnectorV3) GetVersion() string {
	return o.Version
}

// GetSubKind returns resource sub kind
func (o *OIDCConnectorV3) GetSubKind() string {
	return o.SubKind
}

// SetSubKind sets resource subkind
func (o *OIDCConnectorV3) SetSubKind(s string) {
	o.SubKind = s
}

// GetKind returns resource kind
func (o *OIDCConnectorV3) GetKind() string {
	return o.Kind
}

// GetResourceID returns resource ID
func (o *OIDCConnectorV3) GetResourceID() int64 {
	return o.Metadata.ID
}

// SetResourceID sets resource ID
func (o *OIDCConnectorV3) SetResourceID(id int64) {
	o.Metadata.ID = id
}

// WithoutSecrets returns an instance of resource without secrets.
func (o *OIDCConnectorV3) WithoutSecrets() Resource {
	if o.GetClientSecret() == "" && o.GetGoogleServiceAccount() == "" {
		return o
	}
	o2 := *o

	o2.SetClientSecret("")
	o2.SetGoogleServiceAccount("")

	return &o2
}

// V3 returns V3 version of the resource
func (o *OIDCConnectorV3) V3() *OIDCConnectorV3 {
	return o
}

// SetDisplay sets friendly name for this provider.
func (o *OIDCConnectorV3) SetDisplay(display string) {
	o.Spec.Display = display
}

// GetMetadata returns object metadata
func (o *OIDCConnectorV3) GetMetadata() Metadata {
	return o.Metadata
}

// SetExpiry sets expiry time for the object
func (o *OIDCConnectorV3) SetExpiry(expires time.Time) {
	o.Metadata.SetExpiry(expires)
}

// Expiry returns object expiry setting
func (o *OIDCConnectorV3) Expiry() time.Time {
	return o.Metadata.Expiry()
}

// GetName returns the name of the connector
func (o *OIDCConnectorV3) GetName() string {
	return o.Metadata.GetName()
}

// SetName sets client secret to some value
func (o *OIDCConnectorV3) SetName(name string) {
	o.Metadata.SetName(name)
}

// SetIssuerURL sets client secret to some value
func (o *OIDCConnectorV3) SetIssuerURL(issuerURL string) {
	o.Spec.IssuerURL = issuerURL
}

// SetRedirectURLs sets the list of redirectURLs
func (o *OIDCConnectorV3) SetRedirectURLs(redirectURLs []string) {
	o.Spec.RedirectURLs = redirectURLs
}

// SetACR sets the Authentication Context Class Reference (ACR) value.
func (o *OIDCConnectorV3) SetACR(acrValue string) {
	o.Spec.ACR = acrValue
}

// SetProvider sets the identity provider.
func (o *OIDCConnectorV3) SetProvider(identityProvider string) {
	o.Spec.Provider = identityProvider
}

// SetScope sets additional scopes set by provider
func (o *OIDCConnectorV3) SetScope(scope []string) {
	o.Spec.Scope = scope
}

// SetClaimsToRoles sets dynamic mapping from claims to roles
func (o *OIDCConnectorV3) SetClaimsToRoles(claims []ClaimMapping) {
	o.Spec.ClaimsToRoles = claims
}

// SetClientID sets id for authentication client (in our case it's our Auth server)
func (o *OIDCConnectorV3) SetClientID(clintID string) {
	o.Spec.ClientID = clintID
}

// SetClientSecret sets client secret to some value
func (o *OIDCConnectorV3) SetClientSecret(secret string) {
	o.Spec.ClientSecret = secret
}

// GetIssuerURL is the endpoint of the provider, e.g. https://accounts.google.com
func (o *OIDCConnectorV3) GetIssuerURL() string {
	return o.Spec.IssuerURL
}

// GetClientID is id for authentication client (in our case it's our Auth server)
func (o *OIDCConnectorV3) GetClientID() string {
	return o.Spec.ClientID
}

// GetClientSecret is used to authenticate our client and should not
// be visible to end user
func (o *OIDCConnectorV3) GetClientSecret() string {
	return o.Spec.ClientSecret
}

// GetRedirectURLs returns a list of the connector's redirect URLs.
func (o *OIDCConnectorV3) GetRedirectURLs() []string {
	return o.Spec.RedirectURLs
}

// GetACR returns the Authentication Context Class Reference (ACR) value.
func (o *OIDCConnectorV3) GetACR() string {
	return o.Spec.ACR
}

// GetProvider returns the identity provider.
func (o *OIDCConnectorV3) GetProvider() string {
	return o.Spec.Provider
}

// GetDisplay - Friendly name for this provider.
func (o *OIDCConnectorV3) GetDisplay() string {
	if o.Spec.Display != "" {
		return o.Spec.Display
	}
	return o.GetName()
}

// GetScope is additional scopes set by provider
func (o *OIDCConnectorV3) GetScope() []string {
	return o.Spec.Scope
}

// GetClaimsToRoles specifies dynamic mapping from claims to roles
func (o *OIDCConnectorV3) GetClaimsToRoles() []ClaimMapping {
	return o.Spec.ClaimsToRoles
}

// GetClaims returns list of claims expected by mappings
func (o *OIDCConnectorV3) GetClaims() []string {
	var out []string
	for _, mapping := range o.Spec.ClaimsToRoles {
		out = append(out, mapping.Claim)
	}
	return utils.Deduplicate(out)
}

// GetTraitMappings returns the OIDCConnector's TraitMappingSet
func (o *OIDCConnectorV3) GetTraitMappings() TraitMappingSet {
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
func (o *OIDCConnectorV3) setStaticFields() {
	o.Kind = KindOIDCConnector
}

// CheckAndSetDefaults checks and set default values for any missing fields.
func (o *OIDCConnectorV3) CheckAndSetDefaults() error {
	o.setStaticFields()

	switch o.Version {
	case V2, V3:
		// V2 is also supported
	case "":
		o.Version = V3
	default:
		return trace.BadParameter("Version: invalid OIDC connector version %v", o.Version)
	}

	if err := o.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if name := o.Metadata.Name; utils.SliceContainsStr(constants.SystemConnectors, name) {
		return trace.BadParameter("ID: invalid connector name, %v is a reserved name", name)
	}

	if o.Spec.ClientID == "" {
		return trace.BadParameter("ClientID: missing client id")
	}

	if len(o.GetClaimsToRoles()) == 0 {
		return trace.BadParameter("claims_to_roles is empty, authorization with connector would never assign any roles")
	}
	for _, v := range o.Spec.ClaimsToRoles {
		if len(v.Roles) == 0 {
			return trace.BadParameter("add roles in claims_to_roles")
		}
	}

	if _, err := url.Parse(o.GetIssuerURL()); err != nil {
		return trace.BadParameter("bad IssuerURL '%v', err: %v", o.GetIssuerURL(), err)
	}

	// DELETE IN 11.0.0
	o.CheckSetRedirectURL()

	if len(o.GetRedirectURLs()) == 0 {
		return trace.BadParameter("RedirectURL: missing redirect_url")
	}
	for _, redirectURL := range o.GetRedirectURLs() {
		if _, err := url.Parse(redirectURL); err != nil {
			return trace.BadParameter("bad RedirectURL '%v', err: %v", redirectURL, err)
		}
	}

	if o.GetGoogleServiceAccountURI() != "" && o.GetGoogleServiceAccount() != "" {
		return trace.BadParameter("one of either google_service_account_uri or google_service_account is supported, not both")
	}

	if o.GetGoogleServiceAccountURI() != "" {
		uri, err := utils.ParseSessionsURI(o.GetGoogleServiceAccountURI())
		if err != nil {
			return trace.Wrap(err)
		}
		if uri.Scheme != "file" {
			return trace.BadParameter("only file:// scheme is supported for google_service_account_uri")
		}
		if o.GetGoogleAdminEmail() == "" {
			return trace.BadParameter("whenever google_service_account_uri is specified, google_admin_email should be set as well, read https://developers.google.com/identity/protools/OAuth2ServiceAccount#delegatingauthority for more details")
		}
	}

	if o.GetGoogleServiceAccount() != "" {
		if o.GetGoogleAdminEmail() == "" {
			return trace.BadParameter("whenever google_service_account is specified, google_admin_email should be set as well, read https://developers.google.com/identity/protocols/OAuth2ServiceAccount#delegatingauthority for more details")
		}
	}

	return nil
}

// RedirectURL must be checked/set when communicating with an old server or client.
// DELETE IN 11.0.0
func (o *OIDCConnectorV3) CheckSetRedirectURL() {
	if o.Spec.RedirectURL == "" && len(o.Spec.RedirectURLs) != 0 {
		o.Spec.RedirectURL = o.Spec.RedirectURLs[0]
	} else if len(o.Spec.RedirectURLs) == 0 && o.Spec.RedirectURL != "" {
		o.Spec.RedirectURLs = []string{o.Spec.RedirectURL}
	}
}

// Check returns nil if all parameters are great, err otherwise
func (i *OIDCAuthRequest) Check() error {
	if i.ConnectorID == "" {
		return trace.BadParameter("ConnectorID: missing value")
	}
	if i.StateToken == "" {
		return trace.BadParameter("StateToken: missing value")
	}
	if len(i.PublicKey) != 0 {
		_, _, _, _, err := ssh.ParseAuthorizedKey(i.PublicKey)
		if err != nil {
			return trace.BadParameter("PublicKey: bad key: %v", err)
		}
		if (i.CertTTL > defaults.MaxCertDuration) || (i.CertTTL < defaults.MinCertDuration) {
			return trace.BadParameter("CertTTL: wrong certificate TTL")
		}
	}

	// we could collapse these two checks into one, but the error message would become ambiguous.
	if i.SSOTestFlow && i.ConnectorSpec == nil {
		return trace.BadParameter("ConnectorSpec cannot be nil when SSOTestFlow is true")
	}

	if !i.SSOTestFlow && i.ConnectorSpec != nil {
		return trace.BadParameter("ConnectorSpec must be nil when SSOTestFlow is false")
	}

	return nil
}
