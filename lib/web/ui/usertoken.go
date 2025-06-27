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
	Expiry time.Time `json:"expiry"`
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
