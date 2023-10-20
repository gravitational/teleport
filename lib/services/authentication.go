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
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/bcrypt"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// ValidateLocalAuthSecrets validates local auth secret members.
func ValidateLocalAuthSecrets(l *types.LocalAuthSecrets) error {
	if len(l.PasswordHash) > 0 {
		if _, err := bcrypt.Cost(l.PasswordHash); err != nil {
			return trace.BadParameter("invalid password hash")
		}
	}
	mfaNames := make(map[string]struct{}, len(l.MFA))
	for _, d := range l.MFA {
		if err := validateMFADevice(d); err != nil {
			return trace.BadParameter("MFA device named %q is invalid: %v", d.Metadata.Name, err)
		}
		if _, ok := mfaNames[d.Metadata.Name]; ok {
			return trace.BadParameter("MFA device named %q already exists", d.Metadata.Name)
		}
		mfaNames[d.Metadata.Name] = struct{}{}
	}
	if l.Webauthn != nil {
		if err := l.Webauthn.Check(); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// NewTOTPDevice creates a TOTP MFADevice from the given key.
func NewTOTPDevice(name, key string, addedAt time.Time) (*types.MFADevice, error) {
	d := types.NewMFADevice(name, uuid.New().String(), addedAt)
	d.Device = &types.MFADevice_Totp{Totp: &types.TOTPDevice{
		Key: key,
	}}
	if err := validateMFADevice(d); err != nil {
		return nil, trace.Wrap(err)
	}
	return d, nil
}

// validateMFADevice runs additional validations for OTP devices.
// Prefer adding new validation logic to types.MFADevice.CheckAndSetDefaults
// instead.
func validateMFADevice(d *types.MFADevice) error {
	if err := d.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	switch dd := d.Device.(type) {
	case *types.MFADevice_Totp:
		if err := validateTOTPDevice(dd.Totp); err != nil {
			return trace.Wrap(err)
		}
	case *types.MFADevice_U2F:
	case *types.MFADevice_Webauthn:
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

// UnmarshalAuthPreference unmarshals the AuthPreference resource from JSON.
func UnmarshalAuthPreference(bytes []byte, opts ...MarshalOption) (types.AuthPreference, error) {
	var authPreference types.AuthPreferenceV2

	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := utils.FastUnmarshal(bytes, &authPreference); err != nil {
		return nil, trace.BadParameter(err.Error())
	}
	if err := authPreference.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.ID != 0 {
		authPreference.SetResourceID(cfg.ID)
	}
	if cfg.Revision != "" {
		authPreference.SetRevision(cfg.Revision)
	}
	if !cfg.Expires.IsZero() {
		authPreference.SetExpiry(cfg.Expires)
	}
	return &authPreference, nil
}

// MarshalAuthPreference marshals the AuthPreference resource to JSON.
func MarshalAuthPreference(c types.AuthPreference, opts ...MarshalOption) ([]byte, error) {
	if err := c.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return json.Marshal(c)
}
