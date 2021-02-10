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

package auth

import (
	"bytes"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/u2f"

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
