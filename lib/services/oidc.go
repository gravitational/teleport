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

package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"text/template"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/coreos/go-oidc/jose"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
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
	// Scope is additional scopes set by provder
	GetScope() []string
	// ClaimsToRoles specifies dynamic mapping from claims to roles
	GetClaimsToRoles() []ClaimMapping
	// GetClaims returns list of claims expected by mappings
	GetClaims() []string
	// MapClaims maps claims to roles
	MapClaims(claims jose.Claims) []string
	// Check checks OIDC connector for errors
	Check() error
	// CheckAndSetDefaults checks and set default values for any missing fields.
	CheckAndSetDefaults() error
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
	// GetGoogleAdminEmail returns a google admin user email
	// https://developers.google.com/identity/protocols/OAuth2ServiceAccount#delegatingauthority
	// "Note: Although you can use service accounts in applications that run from a G Suite domain, service accounts are not members of your G Suite account and aren’t subject to domain policies set by G Suite administrators. For example, a policy set in the G Suite admin console to restrict the ability of G Suite end users to share documents outside of the domain would not apply to service accounts."
	GetGoogleAdminEmail() string
}

// NewOIDCConnector returns a new OIDCConnector based off a name and OIDCConnectorSpecV2.
func NewOIDCConnector(name string, spec OIDCConnectorSpecV2) OIDCConnector {
	return &OIDCConnectorV2{
		Kind:    KindOIDCConnector,
		Version: V2,
		Metadata: Metadata{
			Name:      name,
			Namespace: defaults.Namespace,
		},
		Spec: spec,
	}
}

var connectorMarshaler OIDCConnectorMarshaler = &TeleportOIDCConnectorMarshaler{}

// SetOIDCConnectorMarshaler sets global user marshaler
func SetOIDCConnectorMarshaler(m OIDCConnectorMarshaler) {
	marshalerMutex.Lock()
	defer marshalerMutex.Unlock()
	connectorMarshaler = m
}

// GetOIDCConnectorMarshaler returns currently set user marshaler
func GetOIDCConnectorMarshaler() OIDCConnectorMarshaler {
	marshalerMutex.RLock()
	defer marshalerMutex.RUnlock()
	return connectorMarshaler
}

// OIDCConnectorMarshaler implements marshal/unmarshal of User implementations
// mostly adds support for extended versions
type OIDCConnectorMarshaler interface {
	// UnmarshalOIDCConnector unmarshals connector from binary representation
	UnmarshalOIDCConnector(bytes []byte, opts ...MarshalOption) (OIDCConnector, error)
	// MarshalOIDCConnector marshals connector to binary representation
	MarshalOIDCConnector(c OIDCConnector, opts ...MarshalOption) ([]byte, error)
}

// GetOIDCConnectorSchema returns schema for OIDCConnector
func GetOIDCConnectorSchema() string {
	return fmt.Sprintf(OIDCConnectorV2SchemaTemplate, MetadataSchema, OIDCConnectorSpecV2Schema)
}

type TeleportOIDCConnectorMarshaler struct{}

// UnmarshalOIDCConnector unmarshals connector from
func (*TeleportOIDCConnectorMarshaler) UnmarshalOIDCConnector(bytes []byte, opts ...MarshalOption) (OIDCConnector, error) {
	cfg, err := collectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var h ResourceHeader
	err = utils.FastUnmarshal(bytes, &h)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case "":
		var c OIDCConnectorV1
		err := json.Unmarshal(bytes, &c)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return c.V2(), nil
	case V2:
		var c OIDCConnectorV2
		if cfg.SkipValidation {
			if err := utils.FastUnmarshal(bytes, &c); err != nil {
				return nil, trace.BadParameter(err.Error())
			}
		} else {
			if err := utils.UnmarshalWithSchema(GetOIDCConnectorSchema(), &c, bytes); err != nil {
				return nil, trace.BadParameter(err.Error())
			}
		}

		if err := c.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.ID != 0 {
			c.SetResourceID(cfg.ID)
		}
		if !cfg.Expires.IsZero() {
			c.SetExpiry(cfg.Expires)
		}
		return &c, nil
	}

	return nil, trace.BadParameter("OIDC connector resource version %v is not supported", h.Version)
}

// MarshalUser marshals OIDC connector into JSON
func (*TeleportOIDCConnectorMarshaler) MarshalOIDCConnector(c OIDCConnector, opts ...MarshalOption) ([]byte, error) {
	cfg, err := collectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	type connv1 interface {
		V1() *OIDCConnectorV1
	}

	type connv2 interface {
		V2() *OIDCConnectorV2
	}
	version := cfg.GetVersion()
	switch version {
	case V1:
		v, ok := c.(connv1)
		if !ok {
			return nil, trace.BadParameter("don't know how to marshal %v", V1)
		}
		return json.Marshal(v.V1())
	case V2:
		v, ok := c.(connv2)
		if !ok {
			return nil, trace.BadParameter("don't know how to marshal %v", V2)
		}
		v2 := v.V2()
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *v2
			copy.SetResourceID(0)
			v2 = &copy
		}
		return utils.FastMarshal(v2)
	default:
		return nil, trace.BadParameter("version %v is not supported", version)
	}
}

// OIDCConnectorV2 is version 1 resource spec for OIDC connector
type OIDCConnectorV2 struct {
	// Kind is a resource kind
	Kind string `json:"kind"`
	// SubKind is a resource sub kind
	SubKind string `json:"sub_kind,omitempty"`
	// Version is version
	Version string `json:"version"`
	// Metadata is connector metadata
	Metadata Metadata `json:"metadata"`
	// Spec contains connector specification
	Spec OIDCConnectorSpecV2 `json:"spec"`
}

// SetPrompt sets OIDC prompt value
func (o *OIDCConnectorV2) SetPrompt(p string) {
	o.Spec.Prompt = &p
}

// GetPrompt returns OIDC prompt value,
// * if not set, in this case defaults to select_account for backwards compatibility
// * set to empty string, in this case it will be omitted
// * and any non empty value, passed as is
func (o *OIDCConnectorV2) GetPrompt() string {
	if o.Spec.Prompt == nil {
		return teleport.OIDCPromptSelectAccount
	}
	return *o.Spec.Prompt
}

// GetGoogleServiceAccountURI returns an optional path to google service account file
func (o *OIDCConnectorV2) GetGoogleServiceAccountURI() string {
	return o.Spec.GoogleServiceAccountURI
}

// GetGoogleAdminEmail returns a google admin user email
func (o *OIDCConnectorV2) GetGoogleAdminEmail() string {
	return o.Spec.GoogleAdminEmail
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
	if o.GetClientSecret() == "" {
		return o
	}
	o2 := *o
	o2.SetClientSecret("")
	return &o2
}

// V2 returns V2 version of the resource
func (o *OIDCConnectorV2) V2() *OIDCConnectorV2 {
	return o
}

// V1 converts OIDCConnectorV2 to OIDCConnectorV1 format
func (o *OIDCConnectorV2) V1() *OIDCConnectorV1 {
	return &OIDCConnectorV1{
		ID:            o.Metadata.Name,
		IssuerURL:     o.Spec.IssuerURL,
		ClientID:      o.Spec.ClientID,
		ClientSecret:  o.Spec.ClientSecret,
		RedirectURL:   o.Spec.RedirectURL,
		Display:       o.Spec.Display,
		Scope:         o.Spec.Scope,
		ClaimsToRoles: o.Spec.ClaimsToRoles,
	}
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

// Expires returns object expiry setting
func (o *OIDCConnectorV2) Expiry() time.Time {
	return o.Metadata.Expiry()
}

// SetTTL sets Expires header using realtime clock
func (o *OIDCConnectorV2) SetTTL(clock clockwork.Clock, ttl time.Duration) {
	o.Metadata.SetTTL(clock, ttl)
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

// Issuer URL is the endpoint of the provider, e.g. https://accounts.google.com
func (o *OIDCConnectorV2) GetIssuerURL() string {
	return o.Spec.IssuerURL
}

// ClientID is id for authentication client (in our case it's our Auth server)
func (o *OIDCConnectorV2) GetClientID() string {
	return o.Spec.ClientID
}

// ClientSecret is used to authenticate our client and should not
// be visible to end user
func (o *OIDCConnectorV2) GetClientSecret() string {
	return o.Spec.ClientSecret
}

// RedirectURL - Identity provider will use this URL to redirect
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

// Display - Friendly name for this provider.
func (o *OIDCConnectorV2) GetDisplay() string {
	if o.Spec.Display != "" {
		return o.Spec.Display
	}
	return o.GetName()
}

// Scope is additional scopes set by provder
func (o *OIDCConnectorV2) GetScope() []string {
	return o.Spec.Scope
}

// ClaimsToRoles specifies dynamic mapping from claims to roles
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

// MapClaims maps claims to roles
func (o *OIDCConnectorV2) MapClaims(claims jose.Claims) []string {
	var roles []string
	for _, mapping := range o.Spec.ClaimsToRoles {
		for claimName := range claims {
			if claimName != mapping.Claim {
				continue
			}
			var claimValues []string
			claimValue, ok, _ := claims.StringClaim(claimName)
			if ok {
				claimValues = []string{claimValue}
			} else {
				claimValues, _, _ = claims.StringsClaim(claimName)
			}
		claimLoop:
			for _, claimValue := range claimValues {
				for _, role := range mapping.Roles {
					outRole, err := utils.ReplaceRegexp(mapping.Value, role, claimValue)
					switch {
					case err != nil:
						if trace.IsNotFound(err) {
							log.Debugf("Failed to match expression %v, replace with: %v input: %v, err: %v", mapping.Value, role, claimValue, err)
						}
						// this claim value clearly did not match, move on to another
						continue claimLoop
						// skip empty replacement or empty role
					case outRole == "":
					case outRole != "":
						roles = append(roles, outRole)
					}
				}
			}
		}
	}
	return utils.Deduplicate(roles)
}

func executeStringTemplate(raw string, claims jose.Claims) (string, error) {
	tmpl, err := template.New("dynamic-roles").Parse(raw)
	if err != nil {
		return "", trace.Wrap(err)
	}
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, claims)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return buf.String(), nil
}

func executeSliceTemplate(raw []string, claims jose.Claims) ([]string, error) {
	var sl []string

	for _, v := range raw {
		tmpl, err := template.New("dynamic-roles").Parse(v)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		var buf bytes.Buffer
		err = tmpl.Execute(&buf, claims)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		sl = append(sl, buf.String())
	}

	return sl, nil
}

// Check returns nil if all parameters are great, err otherwise
func (o *OIDCConnectorV2) Check() error {
	if o.Metadata.Name == "" {
		return trace.BadParameter("ID: missing connector name")
	}
	if o.Metadata.Name == teleport.Local {
		return trace.BadParameter("ID: invalid connector name %v is a reserved name", teleport.Local)
	}
	if _, err := url.Parse(o.Spec.IssuerURL); err != nil {
		return trace.BadParameter("IssuerURL: bad url: '%v'", o.Spec.IssuerURL)
	}
	if _, err := url.Parse(o.Spec.RedirectURL); err != nil {
		return trace.BadParameter("RedirectURL: bad url: '%v'", o.Spec.RedirectURL)
	}
	if o.Spec.ClientID == "" {
		return trace.BadParameter("ClientID: missing client id")
	}

	// make sure claim mappings have either roles or a role template
	for _, v := range o.Spec.ClaimsToRoles {
		hasRoles := false
		if len(v.Roles) > 0 {
			hasRoles = true
		}
		hasRoleTemplate := false
		if v.RoleTemplate != nil {
			hasRoleTemplate = true
		}

		// we either need to have roles or role templates not both or neither
		// ! ( hasRoles XOR hasRoleTemplate )
		if hasRoles == hasRoleTemplate {
			return trace.BadParameter("need roles or role template (not both or none)")
		}
	}

	if o.Spec.GoogleServiceAccountURI != "" {
		uri, err := utils.ParseSessionsURI(o.Spec.GoogleServiceAccountURI)
		if err != nil {
			return trace.Wrap(err)
		}
		if uri.Scheme != teleport.SchemeFile {
			return trace.BadParameter("only %v:// scheme is supported for google_service_account_uri", teleport.SchemeFile)
		}
		if o.Spec.GoogleAdminEmail == "" {
			return trace.BadParameter("whenever google_service_account_uri is specified, google_admin_email should be set as well, read https://developers.google.com/identity/protocols/OAuth2ServiceAccount#delegatingauthority for more details")
		}
	}

	return nil
}

// CheckAndSetDefaults checks and set default values for any missing fields.
func (o *OIDCConnectorV2) CheckAndSetDefaults() error {
	err := o.Metadata.CheckAndSetDefaults()
	if err != nil {
		return trace.Wrap(err)
	}

	err = o.Check()
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// OIDCConnectorV2SchemaTemplate is a template JSON Schema for user
const OIDCConnectorV2SchemaTemplate = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["kind", "spec", "metadata", "version"],
  "properties": {
    "kind": {"type": "string"},
    "version": {"type": "string", "default": "v1"},
    "metadata": %v,
    "spec": %v
  }
}`

// OIDCConnectorSpecV2 specifies configuration for Open ID Connect compatible external
// identity provider:
//
// https://openid.net/specs/openid-connect-core-1_0.html
//
type OIDCConnectorSpecV2 struct {
	// Issuer URL is the endpoint of the provider, e.g. https://accounts.google.com
	IssuerURL string `json:"issuer_url"`
	// ClientID is id for authentication client (in our case it's our Auth server)
	ClientID string `json:"client_id"`
	// ClientSecret is used to authenticate our client and should not
	// be visible to end user
	ClientSecret string `json:"client_secret"`
	// RedirectURL - Identity provider will use this URL to redirect
	// client's browser back to it after successful authentication
	// Should match the URL on Provider's side
	RedirectURL string `json:"redirect_url"`
	// ACR is an Authentication Context Class Reference value. The meaning of the ACR
	// value is context-specific and varies for identity providers.
	ACR string `json:"acr_values,omitempty"`
	// Provider is the external identity provider.
	Provider string `json:"provider,omitempty"`
	// Display - Friendly name for this provider.
	Display string `json:"display,omitempty"`
	// Scope is additional scopes set by provder
	Scope []string `json:"scope,omitempty"`
	// Prompt is optional OIDC prompt, empty string omits prompt
	// if not specified, defaults to select_account for backwards compatibility
	// otherwise, is set to a value specified in this field
	Prompt *string `json:"prompt,omitempty"`
	// ClaimsToRoles specifies dynamic mapping from claims to roles
	ClaimsToRoles []ClaimMapping `json:"claims_to_roles,omitempty"`
	// GoogleServiceAccountURI is a path to google service account uri
	GoogleServiceAccountURI string `json:"google_service_account_uri,omitempty"`
	// GoogleAdminEmail is email of google admin to impersonate
	GoogleAdminEmail string `json:"google_admin_email,omitempty"`
}

// OIDCConnectorSpecV2Schema is a JSON Schema for OIDC Connector
var OIDCConnectorSpecV2Schema = fmt.Sprintf(`{
  "type": "object",
  "additionalProperties": false,
  "required": ["issuer_url", "client_id", "client_secret", "redirect_url"],
  "properties": {
    "issuer_url": {"type": "string"},
    "client_id": {"type": "string"},
    "client_secret": {"type": "string"},
    "redirect_url": {"type": "string"},
    "acr_values": {"type": "string"},
    "provider": {"type": "string"},
    "display": {"type": "string"},
    "prompt": {"type": "string"},
    "google_service_account_uri": {"type": "string"},
    "google_admin_email": {"type": "string"},
    "scope": {
      "type": "array",
      "items": {
        "type": "string"
      }
    },
    "claims_to_roles": {
      "type": "array",
      "items": %v
    }
  }
}`, ClaimMappingSchema)

// GetClaimNames returns a list of claim names from the claim values
func GetClaimNames(claims jose.Claims) []string {
	var out []string
	for claim := range claims {
		out = append(out, claim)
	}
	return out
}

// ClaimMapping is OIDC claim mapping that maps
// claim name to teleport roles
type ClaimMapping struct {
	// Claim is OIDC claim name
	Claim string `json:"claim"`
	// Value is claim value to match
	Value string `json:"value"`
	// Roles is a list of static teleport roles to match.
	Roles []string `json:"roles,omitempty"`
	// RoleTemplate a template role that will be filled out with claims.
	RoleTemplate *RoleV2 `json:"role_template,omitempty"`
}

// ClaimMappingSchema is JSON schema for claim mapping
var ClaimMappingSchema = fmt.Sprintf(`{
  "type": "object",
  "additionalProperties": false,
  "required": ["claim", "value" ],
  "properties": {
    "claim": {"type": "string"},
    "value": {"type": "string"},
    "roles": {
      "type": "array",
      "items": {
        "type": "string"
      }
    },
    "role_template": %v
  }
}`, GetRoleSchema(V2, ""))

// OIDCConnectorV1 specifies configuration for Open ID Connect compatible external
// identity provider, e.g. google in some organisation
type OIDCConnectorV1 struct {
	// ID is a provider id, 'e.g.' google, used internally
	ID string `json:"id"`
	// Issuer URL is the endpoint of the provider, e.g. https://accounts.google.com
	IssuerURL string `json:"issuer_url"`
	// ClientID is id for authentication client (in our case it's our Auth server)
	ClientID string `json:"client_id"`
	// ClientSecret is used to authenticate our client and should not
	// be visible to end user
	ClientSecret string `json:"client_secret"`
	// RedirectURL - Identity provider will use this URL to redirect
	// client's browser back to it after successful authentication
	// Should match the URL on Provider's side
	RedirectURL string `json:"redirect_url"`
	// Display - Friendly name for this provider.
	Display string `json:"display"`
	// Scope is additional scopes set by provder
	Scope []string `json:"scope"`
	// ClaimsToRoles specifies dynamic mapping from claims to roles
	ClaimsToRoles []ClaimMapping `json:"claims_to_roles"`
}

// V1 returns V1 version of the resource
func (o *OIDCConnectorV1) V1() *OIDCConnectorV1 {
	return o
}

// V2 returns V2 version of the connector
func (o *OIDCConnectorV1) V2() *OIDCConnectorV2 {
	return &OIDCConnectorV2{
		Kind:    KindOIDCConnector,
		Version: V2,
		Metadata: Metadata{
			Name: o.ID,
		},
		Spec: OIDCConnectorSpecV2{
			IssuerURL:     o.IssuerURL,
			ClientID:      o.ClientID,
			ClientSecret:  o.ClientSecret,
			RedirectURL:   o.RedirectURL,
			Display:       o.Display,
			Scope:         o.Scope,
			ClaimsToRoles: o.ClaimsToRoles,
		},
	}
}
