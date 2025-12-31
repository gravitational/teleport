// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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
package auth

import (
	"context"

	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
)

// MFAService defines the interface for managing MFA resources in the backend.
type MFAService interface {
	// CreateValidatedMFAChallenge stores a ValidatedMFAChallenge resource for a given username.
	CreateValidatedMFAChallenge(
		ctx context.Context,
		username string,
		challenge *mfav1.ValidatedMFAChallenge,
	) (*mfav1.ValidatedMFAChallenge, error)

	// GetValidatedMFAChallenge retrieves a ValidatedMFAChallenge resource by username and challengeName.
	GetValidatedMFAChallenge(
		ctx context.Context,
		username string,
		challengeName string,
	) (*mfav1.ValidatedMFAChallenge, error)
}
