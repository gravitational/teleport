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
	"github.com/gravitational/teleport/lib/services"
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
		return trace.BadParameter("bound_keypair join method requires ProvisionTokenV2, got %T", token)
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
		return trace.BadParameter("bound_keypair join method requires ProvisionTokenV2, got %T", token)
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

// applyBoundKeypairToken applies a bound_keypair provision token supplied via
// --apply-on-startup.
//
// The join path rejects tokens without an initialized status.bound_keypair, so
// we initialize it here (the normal admin path does this too, but
// apply-on-startup writes spec-only YAML straight to storage and leaves status
// nil).
//
// apply-on-startup re-runs on every auth restart. If the token already exists,
// we preserve its status so a restart never resets a bot's join state
// (recovery counters, bound public key).
func applyBoundKeypairToken(ctx context.Context, service *Services, token types.ProvisionToken) error {
	tokenV2, ok := token.(*types.ProvisionTokenV2)
	if !ok {
		return trace.BadParameter("bound_keypair join method requires ProvisionTokenV2, got %T", token)
	}

	if tokenV2.Spec.BoundKeypair == nil {
		return trace.BadParameter("bound_keypair token requires non-nil spec.bound_keypair")
	}
	if err := validateBoundKeypairTokenSpec(tokenV2.Spec.BoundKeypair); err != nil {
		return trace.Wrap(err)
	}

	// Patch an existing token to keep its status; create it if absent. This
	// loop only handles the create/patch race (PatchToken already retries
	// compare failures): patch, and if the token doesn't exist yet, create.
	const attempts = 3
	for range attempts {
		_, err := service.PatchToken(ctx, tokenV2.GetName(), func(existing types.ProvisionToken) (types.ProvisionToken, error) {
			return prepareAppliedBoundKeypairToken(tokenV2, existing)
		})
		if !trace.IsNotFound(err) {
			// Covers success (err == nil) and any non-recoverable error.
			return trace.Wrap(err)
		}

		// The token does not exist yet. Initialize status.bound_keypair
		// (including the registration secret) and create it; a lost create race
		// retries the patch above.
		cloned, err := services.CloneProvisionToken(tokenV2)
		if err != nil {
			return trace.Wrap(err)
		}
		created := cloned.(*types.ProvisionTokenV2)
		// Never trust status from config: drop it so a config-supplied
		// bound_public_key can't bind a key without the join ceremony.
		created.Status = nil
		if err := populateRegistrationSecret(created); err != nil {
			return trace.Wrap(err)
		}
		if err := service.CreateToken(ctx, created); err == nil {
			return nil
		} else if !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}
	}

	return trace.LimitExceeded("failed to apply bound_keypair token %q after %d attempts", tokenV2.GetName(), attempts)
}

// prepareAppliedBoundKeypairToken returns a copy of desired that keeps
// existing's revision and status. It is called by PatchToken, always with a
// non-nil existing token.
//
// A re-apply updates spec but never status: spec fields can be edited freely,
// but to change status you must delete and recreate the token. This matches the
// Update/Upsert RPCs and stops an edit from knocking a bot offline.
func prepareAppliedBoundKeypairToken(desired *types.ProvisionTokenV2, existing types.ProvisionToken) (*types.ProvisionTokenV2, error) {
	cloned, err := services.CloneProvisionToken(desired)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	updated := cloned.(*types.ProvisionTokenV2)

	updated.SetRevision(existing.GetRevision())
	updated.Status = nil
	if status := existing.GetBoundKeypairStatus(); status != nil {
		updated.Status = &types.ProvisionTokenStatusV2{
			BoundKeypair: status,
		}
	}

	if err := populateRegistrationSecret(updated); err != nil {
		return nil, trace.Wrap(err)
	}
	return updated, nil
}
