/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package auth

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/boundkeypair"
	"github.com/gravitational/teleport/lib/boundkeypair/experiment"
	"github.com/gravitational/teleport/lib/jwt"
	libsshutils "github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/trace"
)

// validateBoundKeypairTokenSpec performs some basic validation checks on a
// bound_keypair-type join token.
func validateBoundKeypairTokenSpec(spec *types.ProvisionTokenSpecV2BoundKeypair) error {
	// Various constant checks, shared between creation and update. Many of
	// these checks are temporary and will be removed alongside the experiment
	// flag.
	if !experiment.Enabled() {
		return trace.BadParameter("bound keypair joining experiment is not enabled")
	}

	if spec.RotateOnNextRenewal {
		return trace.NotImplemented("spec.bound_keypair.rotate_on_next_renewal is not yet implemented")
	}

	if spec.Onboarding.InitialJoinSecret != "" {
		return trace.NotImplemented("spec.bound_keypair.initial_join_secret is not yet implemented")
	}

	if spec.Onboarding.InitialPublicKey == "" {
		return trace.NotImplemented("spec.bound_keypair.initial_public_key is currently required")
	}

	if !spec.Joining.Unlimited {
		return trace.NotImplemented("spec.bound_keypair.joining.unlimited cannot currently be `false`")
	}

	if !spec.Joining.Insecure {
		return trace.NotImplemented("spec.bound_keypair.joining.insecure cannot currently be `false`")
	}

	return nil
}

func (a *Server) initialBoundKeypairStatus(spec *types.ProvisionTokenSpecV2BoundKeypair) *types.ProvisionTokenStatusV2BoundKeypair {
	return &types.ProvisionTokenStatusV2BoundKeypair{
		InitialJoinSecret: spec.Onboarding.InitialJoinSecret,
		BoundPublicKey:    spec.Onboarding.InitialPublicKey,
	}
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

	// TODO: Is this wise? End users _shouldn't_ modify this, but this could
	// interfere with cluster backup/restore. Options seem to be:
	// - Let users create/update with status fields. They can break things, but
	//   maybe that's okay. No backup/restore implications.
	// - Ignore status fields during creation and update. Any set value will be
	//   discarded here, and during update. This would still have consequences
	//   during cluster restores, but wouldn't raise errors, and the status
	//   field would otherwise be protected from easy tampering. Users might be
	//   confused as no user-visible errors would be raised if they used
	//   `tctl edit`.
	// - Raise an error if status fields are changed. Worst restore
	//   implications, but tampering won't be easy, and will have some UX.
	if tokenV2.Status.BoundKeypair != nil {
		return trace.BadParameter("cannot create a bound_keypair token with set status")
	}

	// TODO: Populate initial_join_secret if needed.

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

	// TODO: Populate initial_join_secret if needed, but only if no previous
	// resource exists.

	// TODO: Probably won't want to tweak status here; that's best done during
	// the join ceremony.
	// if tokenV2.Status == nil {
	// 	tokenV2.Status = &types.ProvisionTokenStatusV2{}
	// }

	// if tokenV2.Status.BoundKeypair == nil {
	// 	tokenV2.Status.BoundKeypair = a.initialBoundKeypairStatus(spec)
	// }

	// TODO: Follow up changes to include:
	// - Compare and swap / conditional updates
	// - Proper checking for previous resource
	return trace.Wrap(a.UpsertToken(ctx, token))
}

func (a *Server) issueBoundKeypairChallenge(
	ctx context.Context,
	marshalledKey string,
	challengeResponse client.RegisterUsingBoundKeypairChallengeResponseFunc,
) error {
	key, err := libsshutils.CryptoPublicKey([]byte(marshalledKey))
	if err != nil {
		return trace.Wrap(err, "parsing bound public key")
	}

	// The particular subject value doesn't strictly need to be the name of the
	// bot or node (which may not be known, yet). Instead, we'll use the key ID,
	// which could at least be useful for the client to know which key the
	// challenge should be signed with.
	keyID, err := jwt.KeyID(key)
	if err != nil {
		return trace.Wrap(err, "determining the key ID")
	}

	clusterName, err := a.GetClusterName(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	a.logger.DebugContext(ctx, "Server.issueBoundKeypairChallenge(): preflight complete, issuing challenge", "pk", marshalledKey, "id", keyID)

	validator, err := boundkeypair.NewChallengeValidator(keyID, clusterName.GetClusterName(), key)
	if err != nil {
		return trace.Wrap(err)
	}

	challenge, err := validator.IssueChallenge()
	if err != nil {
		return trace.Wrap(err, "generating a challenge document")
	}

	a.logger.DebugContext(ctx, "Server.issueBoundKeypairChallenge(): issued new challenge", "challenge", challenge)

	marshalledChallenge, err := json.Marshal(challenge)
	if err != nil {
		return trace.Wrap(err)
	}

	a.logger.InfoContext(ctx, "requesting signed bound keypair joining challenge")

	response, err := challengeResponse(marshalledKey, string(marshalledChallenge))
	if err != nil {
		return trace.Wrap(err, "requesting a signed challenge")
	}

	a.logger.DebugContext(ctx, "Server.issueBoundKeypairChallenge(): challenge complete, verifying", "response", response)

	if err := validator.ValidateChallengeResponse(challenge.Nonce, string(response.Solution)); err != nil {
		// TODO: access denied instead?
		return trace.Wrap(err, "validating challenge response")
	}

	a.logger.InfoContext(ctx, "bound keypair challenge response verified successfully")

	return nil
}

// boundKeypairStatusMutator is a function called to mutate a bound keypair
// status during a call to PatchProvisionToken(). These functions may be called
// repeatedly if e.g. revision checks fail. To ensure invariants remain in
// place, mutator functions may make assertions
type boundKeypairStatusMutator func(*types.ProvisionTokenStatusV2BoundKeypair) error

func mutateStatusConsumeJoin(unlimited bool, expectRemainingJoins uint32) boundKeypairStatusMutator {
	now := time.Now()

	return func(status *types.ProvisionTokenStatusV2BoundKeypair) error {
		// Ensure we have the expected number of rejoins left to prevent going
		// below zero.
		// TODO: this could be >=? would avoid breaking if this happens to
		// collide with a user incrementing TotalJoins.
		if status.RemainingJoins != expectRemainingJoins {
			return trace.AccessDenied("unexpected backend state")
		}

		status.JoinCount += 1
		status.LastJoinedAt = &now

		if !unlimited {
			// TODO: decrement remaining joins (not yet implemented.)
			return trace.NotImplemented("only unlimited rejoining is currently supported")
		}

		return nil
	}
}

func mutateStatusBoundPublicKey(newPublicKey, expectPreviousKey string) boundKeypairStatusMutator {
	return func(status *types.ProvisionTokenStatusV2BoundKeypair) error {
		if status.BoundPublicKey != expectPreviousKey {
			return trace.AccessDenied("unexpected backend state")
		}

		status.BoundPublicKey = newPublicKey

		return nil
	}
}

func mutateStatusBoundBotInstance(newBotInstance, expectPreviousBotInstance string) boundKeypairStatusMutator {
	return func(status *types.ProvisionTokenStatusV2BoundKeypair) error {
		if status.BoundBotInstanceID != expectPreviousBotInstance {
			return trace.AccessDenied("unexpected backend state")
		}

		status.BoundBotInstanceID = newBotInstance

		return nil
	}
}

func (a *Server) RegisterUsingBoundKeypairMethod(
	ctx context.Context,
	req *proto.RegisterUsingBoundKeypairInitialRequest,
	challengeResponse client.RegisterUsingBoundKeypairChallengeResponseFunc,
) (_ *proto.Certs, err error) {
	a.logger.DebugContext(ctx, "Server.RegisterUsingBoundKeypairMethod()", "req", req)

	var provisionToken types.ProvisionToken
	var joinFailureMetadata any
	defer func() {
		// Emit a log message and audit event on join failure.
		if err != nil {
			a.handleJoinFailure(
				ctx, err, provisionToken, joinFailureMetadata, req.JoinRequest,
			)
		}
	}()

	// First, check the specified token exists, and is a bound keypair-type join
	// token.
	if err := req.JoinRequest.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	// Only bot joining is supported at the moment - unique ID verification is
	// required and this is currently only implemented for bots.
	if req.JoinRequest.Role != types.RoleBot {
		return nil, trace.BadParameter("bound keypair joining is only supported for bots")
	}

	provisionToken, err = a.checkTokenJoinRequestCommon(ctx, req.JoinRequest)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ptv2, ok := provisionToken.(*types.ProvisionTokenV2)
	if !ok {
		return nil, trace.BadParameter("expected *types.ProvisionTokenV2, got %T", provisionToken)
	}
	if ptv2.Spec.JoinMethod != types.JoinMethodBoundKeypair {
		return nil, trace.BadParameter("specified join token is not for `%s` method", types.JoinMethodBoundKeypair)
	}

	if ptv2.Status == nil {
		ptv2.Status = &types.ProvisionTokenStatusV2{}
	}
	if ptv2.Status.BoundKeypair == nil {
		ptv2.Status.BoundKeypair = &types.ProvisionTokenStatusV2BoundKeypair{}
	}

	spec := ptv2.Spec.BoundKeypair
	status := ptv2.Status.BoundKeypair
	hasBoundPublicKey := status.BoundPublicKey != ""
	hasBoundBotInstance := status.BoundBotInstanceID != ""
	hasIncomingBotInstance := req.JoinRequest.BotInstanceID != ""
	expectNewBotInstance := false

	// Mutators to use during the token resource status patch at the end.
	var mutators []boundKeypairStatusMutator

	switch {
	case !hasBoundPublicKey && !hasIncomingBotInstance:
		// Normal initial join attempt. No bound key, and no incoming bot
		// instance. Consumes a rejoin.
		if spec.Onboarding.InitialJoinSecret != "" {
			return nil, trace.NotImplemented("initial joining secrets are not yet supported")
		}

		if spec.Onboarding.InitialPublicKey == "" {
			return nil, trace.BadParameter("an initial public key is required")
		}

		if !spec.Joining.Unlimited && status.RemainingJoins == 0 {
			return nil, trace.AccessDenied("no rejoins remaining")
		}

		if err := a.issueBoundKeypairChallenge(
			ctx,
			spec.Onboarding.InitialPublicKey,
			challengeResponse,
		); err != nil {
			return nil, trace.Wrap(err)
		}

		// Now that we've confirmed the key, we can consider it bound.
		mutators = append(
			mutators,
			mutateStatusBoundPublicKey(spec.Onboarding.InitialPublicKey, ""),
			mutateStatusConsumeJoin(spec.Joining.Unlimited, status.RemainingJoins),
		)

		expectNewBotInstance = true
	case !hasBoundPublicKey && hasIncomingBotInstance:
		// Not allowed, at least at the moment. This would imply e.g. trying to
		// change auth methods.
		return nil, trace.BadParameter("cannot perform first bound keypair join with existing credentials")
	case hasBoundPublicKey && !hasBoundBotInstance:
		// TODO: Bad backend state, or maybe an incomplete previous join
		// attempt. This shouldn't be possible state, but we should handle it
		// sanely anyway.
		return nil, trace.BadParameter("bad backend state, please recreate the join token")
	case hasBoundPublicKey && hasBoundBotInstance && hasIncomingBotInstance:
		// Standard rejoin case, does not consume a rejoin.
		if status.BoundBotInstanceID != req.JoinRequest.BotInstanceID {
			return nil, trace.AccessDenied("bot instance mismatch")
		}

		if err := a.issueBoundKeypairChallenge(
			ctx,
			spec.Onboarding.InitialPublicKey,
			challengeResponse,
		); err != nil {
			return nil, trace.Wrap(err)
		}

		// Nothing else to do, no key change
	case hasBoundPublicKey && hasBoundBotInstance && !hasIncomingBotInstance:
		// Hard rejoin case, the client identity expired and a new bot instance
		// is required. Consumes a rejoin.
		if !spec.Joining.Unlimited && status.RemainingJoins == 0 {
			return nil, trace.AccessDenied("no rejoins remaining")
		}

		if err := a.issueBoundKeypairChallenge(
			ctx,
			status.BoundPublicKey,
			challengeResponse,
		); err != nil {
			return nil, trace.Wrap(err)
		}

		mutators = append(
			mutators,
			mutateStatusConsumeJoin(spec.Joining.Unlimited, status.RemainingJoins),
		)

		// TODO: decrement remaining joins

		expectNewBotInstance = true
	default:
		a.logger.ErrorContext(
			ctx, "unexpected state",
			"hasBoundPublicKey", hasBoundPublicKey,
			"hasBoundBotInstance", hasBoundBotInstance,
			"hasIncomingBotInstance", hasIncomingBotInstance,
			"spec", spec,
			"status", status,
		)
		return nil, trace.BadParameter("unexpected state")
	}

	if req.NewPublicKey != "" {
		// TODO
		return nil, trace.NotImplemented("key rotation not yet implemented")
	}

	a.logger.DebugContext(ctx, "Server.RegisterUsingBoundKeypairMethod(): challenge verified, issuing certs")

	certs, botInstanceID, err := a.generateCertsBot(
		ctx,
		ptv2,
		req.JoinRequest,
		nil, // TODO: extended claims for this type?
		nil, // TODO: workload id claims
	)

	if expectNewBotInstance {
		mutators = append(
			mutators,
			mutateStatusBoundBotInstance(botInstanceID, status.BoundBotInstanceID),
		)
	}

	if len(mutators) > 0 {
		if _, err := a.PatchToken(ctx, ptv2.GetName(), func(token types.ProvisionToken) (types.ProvisionToken, error) {
			ptv2, ok := provisionToken.(*types.ProvisionTokenV2)
			if !ok {
				return nil, trace.BadParameter("expected *types.ProvisionTokenV2, got %T", provisionToken)
			}

			// Apply all mutators. Individual mutators may make additional
			// assertions to ensure invariants haven't changed.
			for _, mutator := range mutators {
				if err := mutator(ptv2.Status.BoundKeypair); err != nil {
					return nil, trace.Wrap(err, "applying status mutator")
				}
			}

			return ptv2, nil
		}); err != nil {
			return nil, trace.Wrap(err, "commiting updated token state, please try again")
		}
	}

	return certs, trace.Wrap(err)
}
