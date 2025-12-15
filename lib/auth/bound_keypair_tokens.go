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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
)

// validateBoundKeypairTokenSpec performs some basic validation checks on a
// bound_keypair-type join token.
func validateBoundKeypairTokenSpec(spec *types.ProvisionTokenSpecV2BoundKeypair) error {
	if spec.Recovery == nil {
		return trace.BadParameter("spec.bound_keypair.recovery: field is required")
	}

	return nil
}

// populateRegistrationSecret populates the
// `status.BoundKeypair.RegistrationSecret` field of a bound keypair token. It
// should be called as part of any token creation or update to ensure the
// registration secret is made available if needed.
func populateRegistrationSecret(v2 *types.ProvisionTokenV2) error {
	if v2.GetJoinMethod() != types.JoinMethodBoundKeypair {
		return trace.BadParameter("must be called with a bound keypair token")
	}

	if v2.Spec.BoundKeypair == nil {
		v2.Spec.BoundKeypair = &types.ProvisionTokenSpecV2BoundKeypair{}
	}

	if v2.Status == nil {
		v2.Status = &types.ProvisionTokenStatusV2{}
	}
	if v2.Status.BoundKeypair == nil {
		v2.Status.BoundKeypair = &types.ProvisionTokenStatusV2BoundKeypair{}
	}
	if v2.Spec.BoundKeypair.Onboarding == nil {
		v2.Spec.BoundKeypair.Onboarding = &types.ProvisionTokenSpecV2BoundKeypair_OnboardingSpec{}
	}

	spec := v2.Spec.BoundKeypair
	status := v2.Status.BoundKeypair

	if status.BoundPublicKey != "" || spec.Onboarding.InitialPublicKey != "" {
		// A key has already been bound or preregistered, nothing to do.
		return nil
	}

	if status.RegistrationSecret != "" {
		// A secret has already been generated, nothing to do.
		return nil
	}

	if spec.Onboarding.RegistrationSecret != "" {
		// An explicit registration secret was provided, so copy it to status.
		status.RegistrationSecret = spec.Onboarding.RegistrationSecret
		return nil
	}

	// Otherwise, we have no key and no secret, so generate one now.
	s, err := utils.CryptoRandomHex(defaults.TokenLenBytes)
	if err != nil {
		return trace.Wrap(err)
	}

	status.RegistrationSecret = s
	return nil
}

func (a *Server) CreateBoundKeypairToken(ctx context.Context, token types.ProvisionToken) error {
	if token.GetJoinMethod() != types.JoinMethodBoundKeypair {
		return trace.BadParameter("must be called with a bound keypair token")
	}

	tokenV2, ok := token.(*types.ProvisionTokenV2)
	if !ok {
		return trace.BadParameter("%v join method requires ProvisionTokenV2", types.JoinMethodOracle)
	}

	spec := tokenV2.Spec.BoundKeypair
	if spec == nil {
		return trace.BadParameter("bound_keypair token requires non-nil spec.bound_keypair")
	}

	if err := validateBoundKeypairTokenSpec(spec); err != nil {
		return trace.Wrap(err)
	}

	// Not as much to do here - ideally we'd like to prevent users from
	// tampering with the status field, but we don't have a good mechanism to
	// stop that that wouldn't also break backup and restore. For now, it's
	// simpler and easier to just tell users not to edit those fields.

	if err := populateRegistrationSecret(tokenV2); err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(a.CreateToken(ctx, tokenV2))
}

func (a *Server) UpsertBoundKeypairToken(ctx context.Context, token types.ProvisionToken) error {
	if token.GetJoinMethod() != types.JoinMethodBoundKeypair {
		return trace.BadParameter("must be called with a bound keypair token")
	}

	tokenV2, ok := token.(*types.ProvisionTokenV2)
	if !ok {
		return trace.BadParameter("%v join method requires ProvisionTokenV2", types.JoinMethodOracle)
	}

	spec := tokenV2.Spec.BoundKeypair
	if spec == nil {
		return trace.BadParameter("bound_keypair token requires non-nil spec.bound_keypair")
	}

	if err := validateBoundKeypairTokenSpec(spec); err != nil {
		return trace.Wrap(err)
	}

	if err := populateRegistrationSecret(tokenV2); err != nil {
		return trace.Wrap(err)
	}

	// Implementation note: checkAndSetDefaults() impl for this token type is
	// called at insertion time as part of `tokenToItem()`
	return trace.Wrap(a.UpsertToken(ctx, token))
}
