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
	"encoding/json"
	"fmt"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/u2f"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
	"golang.org/x/crypto/bcrypt"
)

// ValidateLocalAuthSecrets validates local auth secret members.
func ValidateLocalAuthSecrets(l *LocalAuthSecrets) error {
	if len(l.PasswordHash) > 0 {
		if _, err := bcrypt.Cost(l.PasswordHash); err != nil {
			return trace.BadParameter("invalid password hash")
		}
	}
	mfaNames := make(map[string]struct{}, len(l.MFA))
	for _, d := range l.MFA {
		if err := ValidateMFADevice(d); err != nil {
			return trace.BadParameter("MFA device named %q is invalid: %v", d.Metadata.Name, err)
		}
		if _, ok := mfaNames[d.Metadata.Name]; ok {
			return trace.BadParameter("MFA device named %q already exists", d.Metadata.Name)
		}
		mfaNames[d.Metadata.Name] = struct{}{}
	}
	return nil
}

// LocalAuthSecretsEquals checks equality (nil safe).
func LocalAuthSecretsEquals(l *LocalAuthSecrets, other *LocalAuthSecrets) bool {
	if (l == nil) || (other == nil) {
		return l == other
	}
	if !bytes.Equal(l.PasswordHash, other.PasswordHash) {
		return false
	}
	if len(l.MFA) != len(other.MFA) {
		return false
	}
	mfa := make(map[string]*types.MFADevice, len(l.MFA))
	for i, d := range l.MFA {
		mfa[d.Id] = l.MFA[i]
	}
	mfaOther := make(map[string]*types.MFADevice, len(other.MFA))
	for i, d := range other.MFA {
		mfaOther[d.Id] = other.MFA[i]
	}
	for id, d := range mfa {
		od, ok := mfaOther[id]
		if !ok {
			return false
		}
		if !mfaDeviceEquals(d, od) {
			return false
		}
	}
	return true
}

// NewTOTPDevice creates a TOTP MFADevice from the given key.
func NewTOTPDevice(name, key string, addedAt time.Time) (*types.MFADevice, error) {
	d := types.NewMFADevice(name, uuid.New(), addedAt)
	d.Device = &types.MFADevice_Totp{Totp: &types.TOTPDevice{
		Key: key,
	}}
	if err := ValidateMFADevice(d); err != nil {
		return nil, trace.Wrap(err)
	}
	return d, nil
}

// ValidateMFADevice validates the MFA device. It's a more in-depth version of
// MFADevice.CheckAndSetDefaults.
//
// TODO(awly): refactor to keep basic and deep validation on one place.
func ValidateMFADevice(d *types.MFADevice) error {
	if err := d.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	switch dd := d.Device.(type) {
	case *types.MFADevice_Totp:
		if err := validateTOTPDevice(dd.Totp); err != nil {
			return trace.Wrap(err)
		}
	case *types.MFADevice_U2F:
		if err := u2f.ValidateDevice(dd.U2F); err != nil {
			return trace.Wrap(err)
		}
	default:
		return trace.BadParameter("MFADevice has Device field of unknown type %T", d.Device)
	}
	return nil
}

func validateTOTPDevice(d *types.TOTPDevice) error {
	if d.Key == "" {
		return trace.BadParameter("TOTPDevice missing Key field")
	}
	return nil
}

func mfaDeviceEquals(d, other *types.MFADevice) bool {
	if (d == nil) || (other == nil) {
		return d == other
	}
	if d.Kind != other.Kind {
		return false
	}
	if d.SubKind != other.SubKind {
		return false
	}
	if d.Version != other.Version {
		return false
	}
	if d.Metadata.Name != other.Metadata.Name {
		return false
	}
	if d.Id != other.Id {
		return false
	}
	if !d.AddedAt.Equal(other.AddedAt) {
		return false
	}
	// Ignore LastUsed, it's a very dynamic field.
	if !totpDeviceEquals(d.GetTotp(), other.GetTotp()) {
		return false
	}
	if !u2fDeviceEquals(d.GetU2F(), other.GetU2F()) {
		return false
	}
	return true
}

func totpDeviceEquals(d, other *types.TOTPDevice) bool {
	if (d == nil) || (other == nil) {
		return d == other
	}
	return d.Key == other.Key
}

func u2fDeviceEquals(d, other *types.U2FDevice) bool {
	if (d == nil) || (other == nil) {
		return d == other
	}
	if !bytes.Equal(d.KeyHandle, other.KeyHandle) {
		return false
	}
	if !bytes.Equal(d.PubKey, other.PubKey) {
		return false
	}
	// Ignore the counter, it's a very dynamic value.
	return true
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
				},
				"device_attestation_cas": {
					"type": "array",
					"items": {
						"type": "string"
					}
				}
			}
		},
		"require_session_mfa": {
			"type": "boolean"
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
		"u2f_counter": {"type": "number"},
		"mfa": {
			"type": "array",
			"items": {
				"type": "object",
				"additionalProperties": false,
				"properties": {
					"kind": {"type": "string"},
					"subKind": {"type": "string"},
					"version": {"type": "string"},
					"metadata": {
						"type": "object",
						"additionalProperties": false,
						"properties": {
							"Name": {"type": "string"},
							"Namespace": {"type": "string"}
						}
					},
					"id": {"type": "string"},
					"name": {"type": "string"},
					"addedAt": {"type": "string"},
					"lastUsed": {"type": "string"},
					"totp": {
						"type": "object",
						"additionalProperties": false,
						"properties": {
							"key": {"type": "string"}
						}
					},
					"u2f": {
						"type": "object",
						"additionalProperties": false,
						"properties": {
							"raw": {"type": "string"},
							"keyHandle": {"type": "string"},
							"pubKey": {"type": "string"},
							"counter": {"type": "number"}
						}
					}
				}
			}
		}
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

	err = authPreference.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
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
