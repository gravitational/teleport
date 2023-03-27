/*
Copyright 2020 Gravitational, Inc.

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

package ui

import "time"

// ResetPasswordToken describes a reset password token UI object.
type ResetPasswordToken struct {
	// TokenID is token ID
	TokenID string `json:"tokenId"`
	// User is user name associated with this token
	User string `json:"user"`
	// QRCode is a QR code value
	QRCode []byte `json:"qrCode,omitempty"`
	// Expiry is token expiration time
	Expiry time.Time `json:"expiry,omitempty"`
}

// RecoveryCodes describes RecoveryCodes UI object.
type RecoveryCodes struct {
	// Codes are user's new recovery codes.
	Codes []string `json:"codes,omitempty"`
	// Created is when the codes were created.
	Created *time.Time `json:"created,omitempty"`
}

// ChangedUserAuthn describes response after successfully changing authn.
type ChangedUserAuthn struct {
	Recovery                RecoveryCodes `json:"recovery"`
	PrivateKeyPolicyEnabled bool          `json:"privateKeyPolicyEnabled,omitempty"`
}
