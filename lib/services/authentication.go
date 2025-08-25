/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

// Package types contains all types and logic required by the Teleport API.

package services

import (
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
		if err := d.CheckAndSetDefaults(); err != nil {
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
	d, err := types.NewMFADevice(name, uuid.New().String(), addedAt, &types.MFADevice_Totp{Totp: &types.TOTPDevice{
		Key: key,
	}})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return d, nil
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
		return nil, trace.BadParameter("%s", err)
	}
	if err := authPreference.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
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
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch c := c.(type) {
	case *types.AuthPreferenceV2:
		if err := c.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		if !cfg.PreserveRevision {
			copy := *c
			copy.SetRevision("")
			c = &copy
		}
		return utils.FastMarshal(c)
	default:
		return nil, trace.BadParameter("unsupported type for auth preference: %T", c)
	}
}
