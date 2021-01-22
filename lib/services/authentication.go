/*
Copyright 2021 Gravitational, Inc.

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

// Package types contains all types and logic required by the Teleport API.

package services

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/pquerna/otp/totp"
	"github.com/tstranex/u2f"
	"golang.org/x/crypto/bcrypt"
)

// AuthPreference defines the authentication preferences for a specific
// cluster. It defines the type (local, oidc) and second factor (off, otp, oidc).
// AuthPreference is a configuration resource, never create more than one instance
// of it.
type AuthPreference interface {
	// Resource provides common resource properties.
	Resource

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
	// SetConnectorName sets the name of the OIDC or SAML connector to use. If
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
	pref := AuthPreferenceV2{
		Kind:    KindClusterAuthPreference,
		Version: V2,
		Metadata: Metadata{
			Name:      MetaNameClusterAuthPreference,
			Namespace: defaults.Namespace,
		},
		Spec: spec,
	}

	if err := pref.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &pref, nil
}

// DefaultAuthPreference returns the default authentication preferences.
func DefaultAuthPreference() AuthPreference {
	return &AuthPreferenceV2{
		Kind:    KindClusterAuthPreference,
		Version: V2,
		Metadata: Metadata{
			Name:      MetaNameClusterAuthPreference,
			Namespace: defaults.Namespace,
		},
		Spec: AuthPreferenceSpecV2{
			Type:         constants.Local,
			SecondFactor: constants.OTP,
		},
	}
}

// AuthPreferenceV2 implements AuthPreference.
type AuthPreferenceV2 struct {
	// Kind is a resource kind - always resource.
	Kind string `json:"kind"`

	// SubKind is a resource sub kind.
	SubKind string `json:"sub_kind,omitempty"`

	// Version is a resource version.
	Version string `json:"version"`

	// Metadata is metadata about the resource.
	Metadata Metadata `json:"metadata"`

	// Spec is the specification of the resource.
	Spec AuthPreferenceSpecV2 `json:"spec"`
}

// GetVersion returns resource version.
func (c *AuthPreferenceV2) GetVersion() string {
	return c.Version
}

// GetName returns the name of the resource.
func (c *AuthPreferenceV2) GetName() string {
	return c.Metadata.Name
}

// SetName sets the name of the resource.
func (c *AuthPreferenceV2) SetName(e string) {
	c.Metadata.Name = e
}

// SetExpiry sets expiry time for the object.
func (c *AuthPreferenceV2) SetExpiry(expires time.Time) {
	c.Metadata.SetExpiry(expires)
}

// Expiry returns object expiry setting.
func (c *AuthPreferenceV2) Expiry() time.Time {
	return c.Metadata.Expiry()
}

// SetTTL sets Expires header using the provided clock.
// Use SetExpiry instead.
// DELETE IN 7.0.0
func (c *AuthPreferenceV2) SetTTL(clock types.Clock, ttl time.Duration) {
	c.Metadata.SetTTL(clock, ttl)
}

// GetMetadata returns object metadata.
func (c *AuthPreferenceV2) GetMetadata() Metadata {
	return c.Metadata
}

// GetResourceID returns resource ID.
func (c *AuthPreferenceV2) GetResourceID() int64 {
	return c.Metadata.ID
}

// SetResourceID sets resource ID.
func (c *AuthPreferenceV2) SetResourceID(id int64) {
	c.Metadata.ID = id
}

// GetKind returns resource kind.
func (c *AuthPreferenceV2) GetKind() string {
	return c.Kind
}

// GetSubKind returns resource subkind.
func (c *AuthPreferenceV2) GetSubKind() string {
	return c.SubKind
}

// SetSubKind sets resource subkind.
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

// SetConnectorName sets the name of the OIDC or SAML connector to use. If
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
	// make sure we have defaults for all metadata fields
	err := c.Metadata.CheckAndSetDefaults()
	if err != nil {
		return trace.Wrap(err)
	}

	// if nothing is passed in, set defaults
	if c.Spec.Type == "" {
		c.Spec.Type = constants.Local
	}
	if c.Spec.SecondFactor == "" {
		c.Spec.SecondFactor = constants.OTP
	}

	// make sure type makes sense
	switch c.Spec.Type {
	case constants.Local, constants.OIDC, constants.SAML, constants.Github:
	default:
		return trace.BadParameter("authentication type %q not supported", c.Spec.Type)
	}

	// make sure second factor makes sense
	switch c.Spec.SecondFactor {
	case constants.OFF, constants.OTP, constants.U2F:
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

// GetU2FRegistrationDataPubKeyDecoded decodes the DER encoded PubKey field into an `ecdsa.PublicKey` instance.
func GetU2FRegistrationDataPubKeyDecoded(reg *U2FRegistrationData) (*ecdsa.PublicKey, error) {
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

// ValidateU2FRegistrationData validates basic u2f registration values
func ValidateU2FRegistrationData(reg *U2FRegistrationData) error {
	if len(reg.KeyHandle) < 1 {
		return trace.BadParameter("missing u2f key handle")
	}
	if len(reg.PubKey) < 1 {
		return trace.BadParameter("missing u2f pubkey")
	}
	if _, err := GetU2FRegistrationDataPubKeyDecoded(reg); err != nil {
		return trace.BadParameter("invalid u2f pubkey")
	}
	return nil
}

// U2FRegistrationDataEquals checks equality (nil safe).
func U2FRegistrationDataEquals(reg *U2FRegistrationData, other *U2FRegistrationData) bool {
	if (reg == nil) || (other == nil) {
		return (reg == nil) && (other == nil)
	}
	if !bytes.Equal(reg.Raw, other.Raw) {
		return false
	}
	if !bytes.Equal(reg.KeyHandle, other.KeyHandle) {
		return false
	}
	return bytes.Equal(reg.PubKey, other.PubKey)
}

// GetLocalAuthSecretsU2FRegistration decodes the u2f registration data and builds the expected
// registration object.  Returns (nil,nil) if no registration data is present.
func GetLocalAuthSecretsU2FRegistration(l *LocalAuthSecrets) (*u2f.Registration, error) {
	if l.U2FRegistration == nil {
		return nil, nil
	}
	pubKey, err := GetU2FRegistrationDataPubKeyDecoded(l.U2FRegistration)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &u2f.Registration{
		Raw:       l.U2FRegistration.Raw,
		KeyHandle: l.U2FRegistration.KeyHandle,
		PubKey:    *pubKey,
	}, nil
}

// SetLocalAuthSecretsU2FRegistration encodes and stores a u2f registration.  Use nil to
// delete an existing registration.
func SetLocalAuthSecretsU2FRegistration(l *LocalAuthSecrets, reg *u2f.Registration) error {
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
	if err := ValidateU2FRegistrationData(l.U2FRegistration); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// ValidateLocalAuthSecrets validates local auth secret members.
func ValidateLocalAuthSecrets(l *LocalAuthSecrets) error {
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
		if err := ValidateU2FRegistrationData(l.U2FRegistration); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// LocalAuthSecretsEquals checks equality (nil safe).
func LocalAuthSecretsEquals(l *LocalAuthSecrets, other *LocalAuthSecrets) bool {
	if (l == nil) || (other == nil) {
		return (l == nil) && (other == nil)
	}
	if !bytes.Equal(l.PasswordHash, other.PasswordHash) {
		return false
	}
	if !(l.TOTPKey == other.TOTPKey) {
		return false
	}
	if !(l.U2FCounter == other.U2FCounter) {
		return false
	}
	return U2FRegistrationDataEquals(l.U2FRegistration, other.U2FRegistration)
}

// AuthPreferenceSpecSchemaTemplate is JSON schema for AuthPreferenceSpec
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

// UnmarshalAuthPreference unmarshals the AuthPreference resource from JSON.
func UnmarshalAuthPreference(bytes []byte, opts ...MarshalOption) (AuthPreference, error) {
	var authPreference AuthPreferenceV2

	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	cfg, err := CollectOptions(opts)
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

// MarshalAuthPreference marshals the AuthPreference resource to JSON.
func MarshalAuthPreference(c AuthPreference, opts ...MarshalOption) ([]byte, error) {
	return json.Marshal(c)
}
