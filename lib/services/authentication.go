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

	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/pquerna/otp/totp"
	"github.com/tstranex/u2f"
	"golang.org/x/crypto/bcrypt"
)

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
