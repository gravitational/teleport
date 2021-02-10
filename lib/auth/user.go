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

package auth

import (
	"bytes"
	"time"

	"github.com/gravitational/trace"
)

// ValidateUser validates the User and sets default values
func ValidateUser(u User) error {
	if err := u.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if localAuth := u.GetLocalAuth(); localAuth != nil {
		if err := ValidateLocalAuthSecrets(localAuth); err != nil {
			return trace.Wrap(err)
		}
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
	mfa := make(map[string]*MFADevice, len(l.MFA))
	for i, d := range l.MFA {
		mfa[d.Id] = l.MFA[i]
	}
	mfaOther := make(map[string]*MFADevice, len(other.MFA))
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

// UsersEquals checks if the users are equal
func UsersEquals(u User, other User) bool {
	if u.GetName() != other.GetName() {
		return false
	}
	otherIdentities := other.GetOIDCIdentities()
	if len(u.GetOIDCIdentities()) != len(otherIdentities) {
		return false
	}
	for i := range u.GetOIDCIdentities() {
		if !u.GetOIDCIdentities()[i].Equals(&otherIdentities[i]) {
			return false
		}
	}
	otherSAMLIdentities := other.GetSAMLIdentities()
	if len(u.GetSAMLIdentities()) != len(otherSAMLIdentities) {
		return false
	}
	for i := range u.GetSAMLIdentities() {
		if !u.GetSAMLIdentities()[i].Equals(&otherSAMLIdentities[i]) {
			return false
		}
	}
	otherGithubIdentities := other.GetGithubIdentities()
	if len(u.GetGithubIdentities()) != len(otherGithubIdentities) {
		return false
	}
	for i := range u.GetGithubIdentities() {
		if !u.GetGithubIdentities()[i].Equals(&otherGithubIdentities[i]) {
			return false
		}
	}
	return LocalAuthSecretsEquals(u.GetLocalAuth(), other.GetLocalAuth())
}

// LoginAttempt represents successful or unsuccessful attempt for user to login
type LoginAttempt struct {
	// Time is time of the attempt
	Time time.Time `json:"time"`
	// Success indicates whether attempt was successful
	Success bool `json:"bool"`
}

// Check checks parameters
func (la *LoginAttempt) Check() error {
	if la.Time.IsZero() {
		return trace.BadParameter("missing parameter time")
	}
	return nil
}
