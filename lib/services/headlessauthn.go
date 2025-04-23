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

package services

import (
	"crypto/sha256"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// HeadlessAuthenticationUserStubID is the ID of a headless authentication stub.
const HeadlessAuthenticationUserStubID = "stub"

// ValidateHeadlessAuthentication verifies that the headless authentication has
// all of the required fields set. Headless authentication stubs will not pass
// this validation.
func ValidateHeadlessAuthentication(h *types.HeadlessAuthentication) error {
	if err := h.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	switch {
	case h.State.IsUnspecified():
		return trace.BadParameter("headless authentication resource state must be specified")
	case h.Version != types.V1:
		return trace.BadParameter("unsupported headless authentication resource version %q, current supported version is %s", h.Version, types.V1)
	case len(h.SshPublicKey) == 0:
		return trace.BadParameter("headless authentication resource must have non-empty SSH public key")
	case h.Metadata.Name != NewHeadlessAuthenticationID(h.SshPublicKey):
		return trace.BadParameter("headless authentication authentication resource name must be derived from public key")
	}

	return nil
}

// NewHeadlessAuthenticationID returns a new SHA256 (Version 5) UUID
// based on the supplied ssh public key.
func NewHeadlessAuthenticationID(pubKey []byte) string {
	return uuid.NewHash(sha256.New(), uuid.Nil, pubKey, 5).String()
}
