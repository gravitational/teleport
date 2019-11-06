/*
Copyright 2017 Gravitational, Inc.

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
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/pquerna/otp/totp"
	"github.com/tstranex/u2f"
)

// AuthPreference defines the authentication preferences for a specific
// cluster. It defines the type (local, oidc) and second factor (off, otp, oidc).
// AuthPreference is a configuration resource, never create more than one instance
// of it.
type AuthPreference interface {
	// Expiry returns object expiry setting
	Expiry() time.Time
	// SetExpiry sets object expiry
	SetExpiry(time.Time)

	// GetResourceID returns resource ID
	GetResourceID() int64
	// SetResourceID sets resource ID
	SetResourceID(int64)

	// GetType gets the type of authentication: local, saml, or oidc.
	GetType() string
	// SetType sets the type of authentication: local, saml, or oidc.
	SetType(string)

	// GetSecondFactor gets the type of second factor: off, otp or u2f.
	GetSecondFactor() string
	// SetSecondFactor sets the type of second factor: off, otp, or u2f.
	SetSecondFactor(string)

	// GetConnectorName gets the name of the OIDC or SAML connector to use. If
	// this value is empty, we fall back to the first connector in the backend.
	GetConnectorName() string
	// GetConnectorName sets the name of the OIDC or SAML connector to use. If
	// this value is empty, we fall back to the first connector in the backend.
	SetConnectorName(string)

	// GetU2F gets the U2F configuration settings.
	GetU2F() (*U2F, error)
	// SetU2F sets the U2F configuration settings.
	SetU2F(*U2F)

	// CheckAndSetDefaults sets and default values and then
	// verifies the constraints for AuthPreference.
	CheckAndSetDefaults() error

	// String represents a human readable version of authentication settings.
	String() string
}

// NewAuthPreference is a convenience method to to create AuthPreferenceV2.
func NewAuthPreference(spec AuthPreferenceSpecV2) (AuthPreference, error) {
	return &AuthPreferenceV2{
		Kind:    KindClusterAuthPreference,
		Version: V2,
		Metadata: Metadata{
			Name:      MetaNameClusterAuthPreference,
			Namespace: defaults.Namespace,
		},
		Spec: spec,
	}, nil
}

// AuthPreferenceV2 implements AuthPreference.
type AuthPreferenceV2 struct {
	// Kind is a resource kind - always resource.
	Kind string `json:"kind"`

	// SubKind is a resource sub kind
	SubKind string `json:"sub_kind,omitempty"`

	// Version is a resource version.
	Version string `json:"version"`

	// Metadata is metadata about the resource.
	Metadata Metadata `json:"metadata"`

	// Spec is the specification of the resource.
	Spec AuthPreferenceSpecV2 `json:"spec"`
}

// SetExpiry sets expiry time for the object
func (s *AuthPreferenceV2) SetExpiry(expires time.Time) {
	s.Metadata.SetExpiry(expires)
}

// Expirey returns object expiry setting
func (s *AuthPreferenceV2) Expiry() time.Time {
	return s.Metadata.Expiry()
}

// GetResourceID returns resource ID
func (c *AuthPreferenceV2) GetResourceID() int64 {
	return c.Metadata.ID
}

// SetResourceID sets resource ID
func (c *AuthPreferenceV2) SetResourceID(id int64) {
	c.Metadata.ID = id
}

// GetKind returns resource kind
func (c *AuthPreferenceV2) GetKind() string {
	return c.Kind
}

// GetSubKind returns resource subkind
func (c *AuthPreferenceV2) GetSubKind() string {
	return c.SubKind
}

// SetSubKind sets resource subkind
func (c *AuthPreferenceV2) SetSubKind(sk string) {
	c.SubKind = sk
}

// GetType returns the type of authentication.
func (c *AuthPreferenceV2) GetType() string {
	return c.Spec.Type
}

// SetType sets the type of authentication.
func (c *AuthPreferenceV2) SetType(s string) {
	c.Spec.Type = s
}

// GetSecondFactor returns the type of second factor.
func (c *AuthPreferenceV2) GetSecondFactor() string {
	return c.Spec.SecondFactor
}

// SetSecondFactor sets the type of second factor.
func (c *AuthPreferenceV2) SetSecondFactor(s string) {
	c.Spec.SecondFactor = s
}

// GetConnectorName gets the name of the OIDC or SAML connector to use. If
// this value is empty, we fall back to the first connector in the backend.
func (c *AuthPreferenceV2) GetConnectorName() string {
	return c.Spec.ConnectorName
}

// GetConnectorName sets the name of the OIDC or SAML connector to use. If
// this value is empty, we fall back to the first connector in the backend.
func (c *AuthPreferenceV2) SetConnectorName(cn string) {
	c.Spec.ConnectorName = cn
}

// GetU2F gets the U2F configuration settings.
func (c *AuthPreferenceV2) GetU2F() (*U2F, error) {
	if c.Spec.U2F == nil {
		return nil, trace.NotFound("U2F configuration not found")
	}
	return c.Spec.U2F, nil
}

// SetU2F sets the U2F configuration settings.
func (c *AuthPreferenceV2) SetU2F(u2f *U2F) {
	c.Spec.U2F = u2f
}

// CheckAndSetDefaults verifies the constraints for AuthPreference.
func (c *AuthPreferenceV2) CheckAndSetDefaults() error {
	// if nothing is passed in, set defaults
	if c.Spec.Type == "" {
		c.Spec.Type = teleport.Local
	}
	if c.Spec.SecondFactor == "" {
		c.Spec.SecondFactor = teleport.OTP
	}

	// make sure type makes sense
	switch c.Spec.Type {
	case teleport.Local, teleport.OIDC, teleport.SAML, teleport.Github:
	default:
		return trace.BadParameter("authentication type %q not supported", c.Spec.Type)
	}

	// make sure second factor makes sense
	switch c.Spec.SecondFactor {
	case teleport.OFF, teleport.OTP, teleport.U2F:
	default:
		return trace.BadParameter("second factor type %q not supported", c.Spec.SecondFactor)
	}

	return nil
}

// String represents a human readable version of authentication settings.
func (c *AuthPreferenceV2) String() string {
	return fmt.Sprintf("AuthPreference(Type=%q,SecondFactor=%q)", c.Spec.Type, c.Spec.SecondFactor)
}

// AuthPreferenceSpecV2 is the actual data we care about for AuthPreferenceV2.
type AuthPreferenceSpecV2 struct {
	// Type is the type of authentication.
	Type string `json:"type"`

	// SecondFactor is the type of second factor.
	SecondFactor string `json:"second_factor,omitempty"`

	// ConnectorName is the name of the OIDC or SAML connector. If this value is
	// not set the first connector in the backend will be used.
	ConnectorName string `json:"connector_name,omitempty"`

	// U2F are the settings for the U2F device.
	U2F *U2F `json:"u2f,omitempty"`
}

// U2F defines settings for U2F device.
type U2F struct {
	// AppID returns the application ID for universal second factor.
	AppID string `json:"app_id,omitempty"`

	// Facets returns the facets for universal second factor.
	Facets []string `json:"facets,omitempty"`
}

const AuthPreferenceSpecSchemaTemplate = `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
	"type": {
		"type": "string"
	},
	"second_factor": {
		"type": "string"
	},
	"connector_name": {
		"type": "string"
	},
	"u2f": {
		"type": "object",
        "additionalProperties": false,
		"properties": {
			"app_id": {
				"type": "string"
			},
			"facets": {
				"type": "array",
				"items": {
					"type": "string"
				}
			}
		}
	}%v
  }
}`

// GetAuthPreferenceSchema returns the schema with optionally injected
// schema for extensions.
func GetAuthPreferenceSchema(extensionSchema string) string {
	var authPreferenceSchema string
	if authPreferenceSchema == "" {
		authPreferenceSchema = fmt.Sprintf(AuthPreferenceSpecSchemaTemplate, "")
	} else {
		authPreferenceSchema = fmt.Sprintf(AuthPreferenceSpecSchemaTemplate, ","+extensionSchema)
	}
	return fmt.Sprintf(V2SchemaTemplate, MetadataSchema, authPreferenceSchema, DefaultDefinitions)
}

// AuthPreferenceMarshaler implements marshal/unmarshal of AuthPreference implementations
// mostly adds support for extended versions.
type AuthPreferenceMarshaler interface {
	Marshal(c AuthPreference, opts ...MarshalOption) ([]byte, error)
	Unmarshal(bytes []byte, opts ...MarshalOption) (AuthPreference, error)
}

var authPreferenceMarshaler AuthPreferenceMarshaler = &TeleportAuthPreferenceMarshaler{}

func SetAuthPreferenceMarshaler(m AuthPreferenceMarshaler) {
	marshalerMutex.Lock()
	defer marshalerMutex.Unlock()
	authPreferenceMarshaler = m
}

func GetAuthPreferenceMarshaler() AuthPreferenceMarshaler {
	marshalerMutex.Lock()
	defer marshalerMutex.Unlock()
	return authPreferenceMarshaler
}

type TeleportAuthPreferenceMarshaler struct{}

// Unmarshal unmarshals role from JSON or YAML.
func (t *TeleportAuthPreferenceMarshaler) Unmarshal(bytes []byte, opts ...MarshalOption) (AuthPreference, error) {
	var authPreference AuthPreferenceV2

	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	cfg, err := collectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.SkipValidation {
		if err := utils.FastUnmarshal(bytes, &authPreference); err != nil {
			return nil, trace.BadParameter(err.Error())
		}
	} else {
		err := utils.UnmarshalWithSchema(GetAuthPreferenceSchema(""), &authPreference, bytes)
		if err != nil {
			return nil, trace.BadParameter(err.Error())
		}
	}
	if cfg.ID != 0 {
		authPreference.SetResourceID(cfg.ID)
	}
	if !cfg.Expires.IsZero() {
		authPreference.SetExpiry(cfg.Expires)
	}
	return &authPreference, nil
}

// Marshal marshals role to JSON or YAML.
func (t *TeleportAuthPreferenceMarshaler) Marshal(c AuthPreference, opts ...MarshalOption) ([]byte, error) {
	return json.Marshal(c)
}

// GetPubKeyDecoded decodes the DER encoded PubKey field into an `ecdsa.PublicKey` instance.
func (reg *U2FRegistrationData) GetPubKeyDecoded() (*ecdsa.PublicKey, error) {
	pubKeyI, err := x509.ParsePKIXPublicKey(reg.PubKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pubKey, ok := pubKeyI.(*ecdsa.PublicKey)
	if !ok {
		return nil, trace.Errorf("expected *ecdsa.PublicKey, got %T", pubKeyI)
	}
	return pubKey, nil
}

// Check validates basic u2f registration values
func (reg *U2FRegistrationData) Check() error {
	if len(reg.KeyHandle) < 1 {
		return trace.BadParameter("missing u2f key handle")
	}
	if len(reg.PubKey) < 1 {
		return trace.BadParameter("missing u2f pubkey")
	}
	if _, err := reg.GetPubKeyDecoded(); err != nil {
		return trace.BadParameter("invalid u2f pubkey")
	}
	return nil
}

// Equals checks equality (nil safe).
func (lhs *U2FRegistrationData) Equals(rhs *U2FRegistrationData) bool {
	if (lhs == nil) || (rhs == nil) {
		return (lhs == nil) && (rhs == nil)
	}
	if !bytes.Equal(lhs.Raw, rhs.Raw) {
		return false
	}
	if !bytes.Equal(lhs.KeyHandle, rhs.KeyHandle) {
		return false
	}
	return bytes.Equal(lhs.PubKey, rhs.PubKey)
}

// GetU2FRegistration decodes the u2f registration data and builds the expected
// registration object.  Returns (nil,nil) if no registration data is present.
func (l *LocalAuthSecrets) GetU2FRegistration() (*u2f.Registration, error) {
	if l.U2FRegistration == nil {
		return nil, nil
	}
	pubKey, err := l.U2FRegistration.GetPubKeyDecoded()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &u2f.Registration{
		Raw:       l.U2FRegistration.Raw,
		KeyHandle: l.U2FRegistration.KeyHandle,
		PubKey:    *pubKey,
	}, nil
}

// SetU2FRegistration encodes and stores a u2f registration.  Use nil to
// delete an existing registration.
func (l *LocalAuthSecrets) SetU2FRegistration(reg *u2f.Registration) error {
	if reg == nil {
		l.U2FRegistration = nil
		return nil
	}
	pubKeyDer, err := x509.MarshalPKIXPublicKey(&reg.PubKey)
	if err != nil {
		return trace.Wrap(err)
	}
	l.U2FRegistration = &U2FRegistrationData{
		Raw:       reg.Raw,
		KeyHandle: reg.KeyHandle,
		PubKey:    pubKeyDer,
	}
	if err := l.U2FRegistration.Check(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Check validates local auth secret members.
func (l *LocalAuthSecrets) Check() error {
	if len(l.PasswordHash) > 0 {
		if _, err := bcrypt.Cost(l.PasswordHash); err != nil {
			return trace.BadParameter("invalid password hash")
		}
	}
	if len(l.TOTPKey) > 0 {
		if _, err := totp.GenerateCode(l.TOTPKey, time.Time{}); err != nil {
			return trace.BadParameter("invalid TOTP key")
		}
	}
	if l.U2FRegistration != nil {
		if err := l.U2FRegistration.Check(); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// Equals checks equality (nil safe).
func (lhs *LocalAuthSecrets) Equals(rhs *LocalAuthSecrets) bool {
	if (lhs == nil) || (rhs == nil) {
		return (lhs == nil) && (rhs == nil)
	}
	if !bytes.Equal(lhs.PasswordHash, rhs.PasswordHash) {
		return false
	}
	if !(lhs.TOTPKey == rhs.TOTPKey) {
		return false
	}
	if !(lhs.U2FCounter == rhs.U2FCounter) {
		return false
	}
	return lhs.U2FRegistration.Equals(rhs.U2FRegistration)
}

// LocalAuthSecretsSchema is a JSON schema for LocalAuthSecrets
const LocalAuthSecretsSchema = `{
    "type": "object",
    "additionalProperties": false,
    "properties": {
        "password_hash": {"type": "string"},
        "totp_key": {"type": "string"},
        "u2f_registration": {
            "type": "object",
            "additionalProperties": false,
            "properties": {
                "raw": {"type": "string"},
                "key_handle": {"type": "string"},
                "pubkey": {"type": "string"}
            }
        },
        "u2f_counter": {"type": "number"}
    }
}`
