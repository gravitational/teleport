/*
Copyright 2023 Gravitational, Inc.

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
	case h.PublicKey == nil:
		return trace.BadParameter("headless authentication resource must have non-empty publicKey")
	case h.Metadata.Name != NewHeadlessAuthenticationID(h.PublicKey):
		return trace.BadParameter("headless authentication authentication resource name must be derived from public key")
	}

	return nil
}

// NewHeadlessAuthenticationID returns a new SHA256 (Version 5) UUID
// based on the supplied ssh public key.
func NewHeadlessAuthenticationID(pubKey []byte) string {
	return uuid.NewHash(sha256.New(), uuid.Nil, pubKey, 5).String()
}
