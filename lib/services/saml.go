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
	"encoding/json"
	"fmt"
	jwt "github.com/dgrijalva/jwt-go"
	"github.com/gravitational/teleport/lib/utils"
	"net/url"

	"github.com/gravitational/trace"
)

// SAMLConnector specifies configuration for saml compatible external
// identity provider, e.g. ms adfs, okta, shibboleth, etc. in some organisation
type SAMLConnector interface {
	// Name is a provider name, 'e.g.' google, used internally
	GetName() string
	// Issuer URL is the IDP metadata url of the provider, e.g. https://server/federationmetadata/2007-06/federationmetadata.xml
	GetIssuerURL() string
	// path of certificate used to sign saml requests and decrypt responses
	GetPathCert() string
	// path of associated key used to sign saml requests and decrypt responses
	GetPathKey() string
	// RedirectURL - Identity provider will use this URL to redirect
	// client's browser back to it after successfull authentication
	// Should match the URL on Provider's side
	GetRedirectURL() string
	// Display - Friendly name for this provider.
	GetDisplay() string
	// Scope is additional scopes set by provder
	GetScope() []string
	// ClaimsToRoles specifies dynamic mapping from claims to roles
	GetClaimsToRoles() []ClaimMapping
	// GetClaims returns list of claims expected by mappings
	GetClaims() []string
	// MapClaims maps claims to roles
	MapClaims(claims utils.TokenClaims) []string
	// Check SAML connector for errors
	Check() error
	// SetPathCert sets the saml cert path to some value
	SetPathCert(secret string)
	// SetPathCert sets the saml key path to some value
	SetPathKey(string)
	// SetName sets a provider name
	SetName(string)
	// SetIssuerURL sets the endpoint of the provider
	SetIssuerURL(string)
	// SetRedirectURL sets RedirectURL
	SetRedirectURL(string)
	// SetScope sets additional scopes set by provider
	SetScope([]string)
	// SetClaimsToRoles sets dynamic mapping from claims to roles
	SetClaimsToRoles([]ClaimMapping)
	// SetDisplay sets friendly name for this provider.
	SetDisplay(string)
}

var SAMLconnectorMarshaler SAMLConnectorMarshaler = &TeleportSAMLConnectorMarshaler{}

// SetSAMLConnectorMarshaler sets global user marshaler
func SetSAMLConnectorMarshaler(m SAMLConnectorMarshaler) {
	marshalerMutex.Lock()
	defer marshalerMutex.Unlock()
	SAMLconnectorMarshaler = m
}

// GetSAMLConnectorMarshaler returns currently set user marshaler
func GetSAMLConnectorMarshaler() SAMLConnectorMarshaler {
	marshalerMutex.RLock()
	defer marshalerMutex.RUnlock()
	return SAMLconnectorMarshaler
}

// SAMLConnectorMarshaler implements marshal/unmarshal of User implementations
// mostly adds support for extended versions
type SAMLConnectorMarshaler interface {
	// UnmarshalSAMLConnector unmarshals connector from binary representation
	UnmarshalSAMLConnector(bytes []byte) (SAMLConnector, error)
	// MarshalSAMLConnector marshals connector to binary representation
	MarshalSAMLConnector(c SAMLConnector, opts ...MarshalOption) ([]byte, error)
}

// GetSAMLConnectorSchema returns schema for SAMLConnector
func GetSAMLConnectorSchema() string {
	return fmt.Sprintf(SAMLConnectorV2SchemaTemplate, MetadataSchema, SAMLConnectorSpecV2Schema)
}

type TeleportSAMLConnectorMarshaler struct{}

// UnmarshalSAMLConnector unmarshals connector from
func (*TeleportSAMLConnectorMarshaler) UnmarshalSAMLConnector(bytes []byte) (SAMLConnector, error) {
	var h ResourceHeader
	err := json.Unmarshal(bytes, &h)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case "":
		var c SAMLConnectorV1
		err := json.Unmarshal(bytes, &c)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return c.V2(), nil
	case V2:
		var c SAMLConnectorV2
		if err := utils.UnmarshalWithSchema(GetSAMLConnectorSchema(), &c, bytes); err != nil {
			return nil, trace.BadParameter(err.Error())
		}
		return &c, nil
	}

	return nil, trace.BadParameter("SAML connector resource version %v is not supported", h.Version)
}

// MarshalUser marshals SAML connector into JSON
func (*TeleportSAMLConnectorMarshaler) MarshalSAMLConnector(c SAMLConnector, opts ...MarshalOption) ([]byte, error) {
	cfg, err := collectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	type connv1 interface {
		V1() *SAMLConnectorV1
	}

	type connv2 interface {
		V2() *SAMLConnectorV2
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
		return json.Marshal(v.V2())
	default:
		return nil, trace.BadParameter("version %v is not supported", version)
	}
}

// SAMLConnectorV2 is version 1 resource spec for SAML connector
type SAMLConnectorV2 struct {
	// Kind is a resource kind
	Kind string `json:"kind"`
	// Version is version
	Version string `json:"version"`
	// Metadata is connector metadata
	Metadata Metadata `json:"metadata"`
	// Spec contains connector specification
	Spec SAMLConnectorSpecV2 `json:"spec"`
}

// V2 returns V2 version of the resource
func (o *SAMLConnectorV2) V2() *SAMLConnectorV2 {
	return o
}

// V1 converts SAMLConnectorV2 to SAMLConnectorV1 format
func (o *SAMLConnectorV2) V1() *SAMLConnectorV1 {
	return &SAMLConnectorV1{
		ID:            o.Metadata.Name,
		IssuerURL:     o.Spec.IssuerURL,
		PathCert:      o.Spec.PathCert,
		PathKey:       o.Spec.PathKey,
		RedirectURL:   o.Spec.RedirectURL,
		Display:       o.Spec.Display,
		Scope:         o.Spec.Scope,
		ClaimsToRoles: o.Spec.ClaimsToRoles,
	}
}

// SetDisplay sets friendly name for this provider.
func (o *SAMLConnectorV2) SetDisplay(display string) {
	o.Spec.Display = display
}

// SetName sets client secret to some value
func (o *SAMLConnectorV2) SetName(name string) {
	o.Metadata.Name = name
}

// SetIssuerURL sets client secret to some value
func (o *SAMLConnectorV2) SetIssuerURL(issuerURL string) {
	o.Spec.IssuerURL = issuerURL
}

// SetRedirectURL sets client secret to some value
func (o *SAMLConnectorV2) SetRedirectURL(redirectURL string) {
	o.Spec.RedirectURL = redirectURL
}

// SetScope sets additional scopes set by provider
func (o *SAMLConnectorV2) SetScope(scope []string) {
	o.Spec.Scope = scope
}

// SetClaimsToRoles sets dynamic mapping from claims to roles
func (o *SAMLConnectorV2) SetClaimsToRoles(claims []ClaimMapping) {
	o.Spec.ClaimsToRoles = claims
}

// SetClientID sets id for authentication client (in our case it's our Auth server)
func (o *SAMLConnectorV2) SetPathCert(path_cert string) {
	o.Spec.PathCert = path_cert
}

// SetClientSecret sets client secret to some value
func (o *SAMLConnectorV2) SetPathKey(path_key string) {
	o.Spec.PathCert = path_key
}

// ID is a provider id, 'e.g.' google, used internally
func (o *SAMLConnectorV2) GetName() string {
	return o.Metadata.Name
}

// Issuer URL is the endpoint of the provider, e.g. https://accounts.google.com
func (o *SAMLConnectorV2) GetIssuerURL() string {
	return o.Spec.IssuerURL
}

// GetPathCert is the path of the certificate that will we used to signed auth requests
func (o *SAMLConnectorV2) GetPathCert() string {
	return o.Spec.PathCert
}

// GetPathCert is the path of the key that will we used to signed auth requests
func (o *SAMLConnectorV2) GetPathKey() string {
	return o.Spec.PathKey
}

// RedirectURL - Identity provider will use this URL to redirect
// client's browser back to it after successfull authentication
// Should match the URL on Provider's side
func (o *SAMLConnectorV2) GetRedirectURL() string {
	return o.Spec.RedirectURL
}

// Display - Friendly name for this provider.
func (o *SAMLConnectorV2) GetDisplay() string {
	if o.Spec.Display != "" {
		return o.Spec.Display
	}
	return o.GetName()
}

// Scope is additional scopes set by provder
func (o *SAMLConnectorV2) GetScope() []string {
	return o.Spec.Scope
}

// ClaimsToRoles specifies dynamic mapping from claims to roles
func (o *SAMLConnectorV2) GetClaimsToRoles() []ClaimMapping {
	return o.Spec.ClaimsToRoles
}

// GetClaims returns list of claims expected by mappings
func (o *SAMLConnectorV2) GetClaims() []string {
	var out []string
	for _, mapping := range o.Spec.ClaimsToRoles {
		out = append(out, mapping.Claim)
	}
	return utils.Deduplicate(out)
}

type TokenClaims struct {
	jwt.StandardClaims
	Attributes map[string][]string `json:"attr"`
}

// MapClaims maps claims to roles
func (o *SAMLConnectorV2) MapClaims(claims utils.TokenClaims) []string {
	var roles []string
	for _, mapping := range o.Spec.ClaimsToRoles {
		claimValues := claims.Attributes[mapping.Claim]
		for _, claimValue := range claimValues {
			if claimValue == mapping.Value {
				roles = append(roles, mapping.Roles...)
			}
		}
	}
	return utils.Deduplicate(roles)
}

// Check returns nil if all parameters are great, err otherwise
func (o *SAMLConnectorV2) Check() error {
	if o.Metadata.Name == "" {
		return trace.BadParameter("ID: missing connector name")
	}
	if _, err := url.Parse(o.Spec.IssuerURL); err != nil {
		return trace.BadParameter("IssuerURL: bad url: '%v'", o.Spec.IssuerURL)
	}
	if _, err := url.Parse(o.Spec.RedirectURL); err != nil {
		return trace.BadParameter("RedirectURL: bad url: '%v'", o.Spec.RedirectURL)
	}
	/* if o.Spec.ClientID == "" {
		return trace.BadParameter("ClientID: missing client id")
	}
	if o.Spec.ClientSecret == "" {
		return trace.BadParameter("ClientSecret: missing client secret")
	} */
	return nil
}

// SAMLConnectorV2SchemaTemplate is a template JSON Schema for user
const SAMLConnectorV2SchemaTemplate = `{
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

// SAMLConnectorSpecV2 specifies configuration for Open ID Connect compatible external
// identity provider, e.g. google in some organisation
type SAMLConnectorSpecV2 struct {
	// Issuer URL is the endpoint of the provider, e.g. https://accounts.google.com
	IssuerURL string `json:"issuer_url"`
	// ClientID is id for authentication client (in our case it's our Auth server)
	PathCert string `json:"path_cert"`
	PathKey  string `json:"path_key"`
	// ClientSecret is used to authenticate our client and should not
	// be visible to end user
	ClientSecret string `json:"client_secret"`
	// RedirectURL - Identity provider will use this URL to redirect
	// client's browser back to it after successfull authentication
	// Should match the URL on Provider's side
	RedirectURL string `json:"redirect_url"`
	// Display - Friendly name for this provider.
	Display string `json:"display,omitempty"`
	// Scope is additional scopes set by provder
	Scope []string `json:"scope,omitempty"`
	// ClaimsToRoles specifies dynamic mapping from claims to roles
	ClaimsToRoles []ClaimMapping `json:"claims_to_roles,omitempty"`
}

// SAMLConnectorSpecV2Schema is a JSON Schema for SAML Connector
var SAMLConnectorSpecV2Schema = fmt.Sprintf(`{
  "type": "object",
  "additionalProperties": false,
  "required": ["issuer_url", "path_cert", "path_key", "redirect_url"],
  "properties": {
    "issuer_url": {"type": "string"},
    "path_cert": {"type": "string"},
    "path_key": {"type": "string"},
    "redirect_url": {"type": "string"},
    "display": {"type": "string"},
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

// SAMLConnectorV1 specifies configuration for Open ID Connect compatible external
// identity provider, e.g. google in some organisation
type SAMLConnectorV1 struct {
	// ID is a provider id, 'e.g.' google, used internally
	ID string `json:"id"`
	// Issuer URL is the endpoint of the provider, e.g. https://accounts.google.com
	IssuerURL string `json:"issuer_url"`
	PathCert  string `json:"path_cert"`
	PathKey   string `json:"path_key"`
	// ClientID is id for authentication client (in our case it's our Auth server)
	RedirectURL string `json:"redirect_url"`
	// Display - Friendly name for this provider.
	Display string `json:"display"`
	// Scope is additional scopes set by provder
	Scope []string `json:"scope"`
	// ClaimsToRoles specifies dynamic mapping from claims to roles
	ClaimsToRoles []ClaimMapping `json:"claims_to_roles"`
}

// V1 returns V1 version of the resource
func (o *SAMLConnectorV1) V1() *SAMLConnectorV1 {
	return o
}

// V2 returns V2 version of the connector
func (o *SAMLConnectorV1) V2() *SAMLConnectorV2 {
	return &SAMLConnectorV2{
		Kind:    KindSAMLConnector,
		Version: V2,
		Metadata: Metadata{
			Name: o.ID,
		},
		Spec: SAMLConnectorSpecV2{
			IssuerURL:     o.IssuerURL,
			PathCert:      o.PathCert,
			PathKey:       o.PathKey,
			RedirectURL:   o.RedirectURL,
			Display:       o.Display,
			Scope:         o.Scope,
			ClaimsToRoles: o.ClaimsToRoles,
		},
	}
}
