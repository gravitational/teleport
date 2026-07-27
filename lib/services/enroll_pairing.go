// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package services

import (
	"context"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
)

// EnrollPairing manages mobile device enrollment pairings.
type EnrollPairing interface {
	// CreateEnrollPairing creates a new EnrollPairing for user in the
	// AWAITING_DEVICE state with a short TTL.
	// Returns AlreadyExists if a pairing already exists for user.
	CreateEnrollPairing(ctx context.Context, user string) (*devicepb.EnrollPairing, error)

	// GetCurrentEnrollPairing returns the EnrollPairing for user.
	// Returns NotFound if no pairing exists.
	GetCurrentEnrollPairing(ctx context.Context, user string) (*devicepb.EnrollPairing, error)

	// GetEnrollPairingByToken returns the EnrollPairing whose status token
	// matches token. Returns NotFound if no pairing matches.
	GetEnrollPairingByToken(ctx context.Context, token string) (*devicepb.EnrollPairing, error)

	// RequestEnrollPairingApproval transitions pairing from AWAITING_DEVICE to
	// AWAITING_APPROVAL, persisting device for the Web UI to display and for
	// retry gating, and returns the updated pairing.
	// Returns CompareFailed if the pairing is no longer awaiting a device.
	RequestEnrollPairingApproval(ctx context.Context, pairing *devicepb.EnrollPairing, device *devicepb.EnrollPairingDevice) (*devicepb.EnrollPairing, error)
}

// MarshalEnrollPairing marshals an EnrollPairing resource to JSON.
func MarshalEnrollPairing(pairing *devicepb.EnrollPairing, opts ...MarshalOption) ([]byte, error) {
	return MarshalProtoResource(pairing, opts...)
}

// UnmarshalEnrollPairing unmarshals an EnrollPairing resource from JSON.
func UnmarshalEnrollPairing(data []byte, opts ...MarshalOption) (*devicepb.EnrollPairing, error) {
	return UnmarshalProtoResource[*devicepb.EnrollPairing](data, opts...)
}
