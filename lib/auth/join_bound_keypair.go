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
	"crypto"
	"crypto/subtle"
	"encoding/json"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/boundkeypair"
	"github.com/gravitational/teleport/lib/boundkeypair/boundkeypairexperiment"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/jwt"
	libsshutils "github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
)

type boundKeypairValidator interface {
	IssueChallenge() (*boundkeypair.ChallengeDocument, error)
	ValidateChallengeResponse(issued *boundkeypair.ChallengeDocument, compactResponse string) error
}

type createBoundKeypairValidator func(subject string, clusterName string, publicKey crypto.PublicKey) (boundKeypairValidator, error)

// validateBoundKeypairTokenSpec performs some basic validation checks on a
// bound_keypair-type join token.
func validateBoundKeypairTokenSpec(spec *types.ProvisionTokenSpecV2BoundKeypair) error {
	// Various constant checks, shared between creation and update. Many of
	// these checks are temporary and will be removed alongside the experiment
	// flag.
	if !boundkeypairexperiment.Enabled() {
		return trace.BadParameter("bound keypair joining experiment is not enabled")
	}

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

// issueBoundKeypairChallenge creates a new challenge for the given marshaled
// public key in ssh authorized_keys format, requests a solution from the
// client using the given `challengeResponse` function, and validates the
// response.
func (a *Server) issueBoundKeypairChallenge(
	ctx context.Context,
	marshaledKey string,
	challengeResponse client.RegisterUsingBoundKeypairChallengeResponseFunc,
) error {
	key, err := libsshutils.CryptoPublicKey([]byte(marshaledKey))
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

	a.logger.DebugContext(ctx, "issuing bound keypair challenge", "key_id", keyID)

	validator, err := a.createBoundKeypairValidator(keyID, clusterName.GetClusterName(), key)
	if err != nil {
		return trace.Wrap(err)
	}

	challenge, err := validator.IssueChallenge()
	if err != nil {
		return trace.Wrap(err, "generating a challenge document")
	}

	marshalledChallenge, err := json.Marshal(challenge)
	if err != nil {
		return trace.Wrap(err)
	}

	response, err := challengeResponse(&proto.RegisterUsingBoundKeypairMethodResponse{
		Response: &proto.RegisterUsingBoundKeypairMethodResponse_Challenge{
			Challenge: &proto.RegisterUsingBoundKeypairChallenge{
				PublicKey: marshaledKey,
				Challenge: string(marshalledChallenge),
			},
		},
	})
	if err != nil {
		return trace.Wrap(err, "requesting a signed challenge")
	}

	solutionResponse, ok := response.Payload.(*proto.RegisterUsingBoundKeypairMethodRequest_ChallengeResponse)
	if !ok {
		return trace.BadParameter("client provided unexpected challenge response type %T", response.Payload)
	}

	if err := validator.ValidateChallengeResponse(
		challenge,
		string(solutionResponse.ChallengeResponse.Solution),
	); err != nil {
		// TODO: Consider access denied instead?
		return trace.Wrap(err, "validating challenge response")
	}

	a.logger.InfoContext(ctx, "bound keypair challenge response verified successfully", "key_id", keyID)

	return nil
}

// shouldRequestBoundKeypairRotation determines if a keypair rotation should be
// requested given configured token field values.
func shouldRequestBoundKeypairRotation(rotateAfter, lastRotatedAt *time.Time, now time.Time) bool {
	if rotateAfter == nil {
		// Field not set, nothing to do.
		return false
	}

	if rotateAfter.After(now) {
		// We haven't reached the rotation threshold, nothing to do.
		return false
	}

	if lastRotatedAt == nil {
		// There has not been a previous rotation, so rotate now.
		return true
	}

	// Otherwise, rotate only if a rotation hasn't already taken place, i.e.
	// `lastRotatedAt` is before the requested timestamp
	return lastRotatedAt.Before(*rotateAfter)
}

// ensurePublicKeysNotEqual ensures the two public keys, in ssh authorized_keys
// format, are parseable public keys and are not equal to one another. This can
// be used to validate that clients actually provided a new key after receiving
// a rotation request.
func ensurePublicKeysNotEqual(a, b string) error {
	aParsed, err := libsshutils.CryptoPublicKey([]byte(a))
	if err != nil {
		return trace.Wrap(err)
	}

	aEq, ok := aParsed.(interface {
		Equal(x crypto.PublicKey) bool
	})
	if !ok {
		return trace.BadParameter("invalid public key type %T", aParsed)
	}

	bParsed, err := libsshutils.CryptoPublicKey([]byte(b))
	if err != nil {
		return trace.Wrap(err)
	}

	if aEq.Equal(bParsed) {
		return trace.BadParameter("public key may not be reused after rotation")
	}

	return nil
}

// requestBoundKeypairRotation requests that clients generate a new keypair and
// send the public key, then issues a signing challenge to ensure ownership of
// the new key.
func (a *Server) requestBoundKeypairRotation(
	ctx context.Context,
	challengeResponse client.RegisterUsingBoundKeypairChallengeResponseFunc,
) (string, error) {
	cap, err := a.GetAuthPreference(ctx)
	if err != nil {
		return "", trace.Wrap(err)
	}

	a.logger.InfoContext(ctx, "requesting bound keypair rotation", "suite", cap.GetSignatureAlgorithmSuite())

	// Request a new marshaled public key from the client.
	response, err := challengeResponse(&proto.RegisterUsingBoundKeypairMethodResponse{
		Response: &proto.RegisterUsingBoundKeypairMethodResponse_Rotation{
			Rotation: &proto.RegisterUsingBoundKeypairRotationRequest{
				SignatureAlgorithmSuite: cap.GetSignatureAlgorithmSuite(),
			},
		},
	})
	if err != nil {
		return "", trace.Wrap(err, "requesting a new public key")
	}

	pubKeyResponse, ok := response.Payload.(*proto.RegisterUsingBoundKeypairMethodRequest_RotationResponse)
	if !ok {
		return "", trace.BadParameter("client provided unexpected keypair request response type %T", response.Payload)
	}

	// Issue a challenge against this new key to ensure ownership.
	pubKey := pubKeyResponse.RotationResponse.PublicKey
	if err := a.issueBoundKeypairChallenge(ctx, pubKey, challengeResponse); err != nil {
		return "", trace.Wrap(err, "solving challenge for new public key")
	}

	return pubKey, nil
}

// boundKeypairStatusMutator is a function called to mutate a bound keypair
// status during a call to PatchProvisionToken(). These functions may be called
// repeatedly if e.g. revision checks fail. To ensure invariants remain in
// place, mutator functions may make assertions to ensure the provided backend
// state is still sane before the update is committed.
type boundKeypairStatusMutator func(*types.ProvisionTokenSpecV2BoundKeypair, *types.ProvisionTokenStatusV2BoundKeypair) error

// mutateStatusConsumeRecovery consumes a "hard" join on the backend, incrementing
// the recovery counter. This verifies that the backend recovery count has not
// changed, and that total join count is at least the value when the mutator was
// created.
func mutateStatusConsumeRecovery(mode boundkeypair.RecoveryMode, expectRecoveryCount uint32, expectMinRecoveryLimit uint32) boundKeypairStatusMutator {
	now := time.Now()

	return func(spec *types.ProvisionTokenSpecV2BoundKeypair, status *types.ProvisionTokenStatusV2BoundKeypair) error {
		// Ensure we have the expected number of rejoins left to prevent going
		// below zero.
		if status.RecoveryCount != expectRecoveryCount {
			return trace.AccessDenied("unexpected backend state")
		}

		// Ensure the allowed join count has at least not decreased, but allow
		// for collision with potentially increased values.
		if spec.Recovery.Limit < expectMinRecoveryLimit {
			return trace.AccessDenied("unexpected backend state")
		}

		status.RecoveryCount += 1
		status.LastRecoveredAt = &now

		return nil
	}
}

// mutateStatusBoundPublicKey is a mutator that updates the bound public key
// value. It ensures the backend public key is still the expected value before
// performing the update.
func mutateStatusBoundPublicKey(newPublicKey, expectPreviousKey string) boundKeypairStatusMutator {
	return func(_ *types.ProvisionTokenSpecV2BoundKeypair, status *types.ProvisionTokenStatusV2BoundKeypair) error {
		if status.BoundPublicKey != expectPreviousKey {
			return trace.AccessDenied("unexpected backend state")
		}

		status.BoundPublicKey = newPublicKey

		return nil
	}
}

// mutateStatusBoundBotInstance updates the bot instance ID currently bound to
// this token. It ensures the expected previous ID is still the bound value
// before performing the update.
func mutateStatusBoundBotInstance(newBotInstance, expectPreviousBotInstance string) boundKeypairStatusMutator {
	return func(_ *types.ProvisionTokenSpecV2BoundKeypair, status *types.ProvisionTokenStatusV2BoundKeypair) error {
		if status.BoundBotInstanceID != expectPreviousBotInstance {
			return trace.AccessDenied("unexpected backend state")
		}

		status.BoundBotInstanceID = newBotInstance

		return nil
	}
}

// mutateStatusLastRotatedAt updates the `status.LastRotatedAt` field to
// indicate a keypair rotation has taken place. It ensures the previous value
// has not changed before performing the update.
func mutateStatusLastRotatedAt(newValue, expectPrevValue *time.Time) boundKeypairStatusMutator {
	return func(_ *types.ProvisionTokenSpecV2BoundKeypair, status *types.ProvisionTokenStatusV2BoundKeypair) error {
		switch {
		case expectPrevValue == nil && status.LastRotatedAt == nil:
			// no issue
		case expectPrevValue != nil && status.LastRotatedAt == nil:
			fallthrough
		case expectPrevValue == nil && status.LastRotatedAt != nil:
			fallthrough
		case !expectPrevValue.Equal(*status.LastRotatedAt):
			return trace.AccessDenied("unexpected backend state")
		}

		status.LastRotatedAt = newValue

		return nil
	}
}

// mutateStatusClearRegistrationSecret clears the registration secret field to
// prevent further join attempts using this secret.
func mutateStatusClearRegistrationSecret(oldValue string) boundKeypairStatusMutator {
	return func(_ *types.ProvisionTokenSpecV2BoundKeypair, status *types.ProvisionTokenStatusV2BoundKeypair) error {
		if status.RegistrationSecret != oldValue {
			return trace.AccessDenied("unexpected backend state")
		}

		status.RegistrationSecret = ""
		return nil
	}
}

// formatTimePointer stringifies a *time.Time for logging, but gracefully
// handles nil values.
func formatTimePointer(t *time.Time) string {
	if t == nil {
		return "nil"
	}

	return t.String()
}

// RegisterUsingBoundKeypairMethod handles joining requests for the bound
// keypair join method. If successful, returns
func (a *Server) RegisterUsingBoundKeypairMethod(
	ctx context.Context,
	req *proto.RegisterUsingBoundKeypairInitialRequest,
	challengeResponse client.RegisterUsingBoundKeypairChallengeResponseFunc,
) (_ *client.BoundKeypairRegistrationResponse, err error) {
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

	log := a.logger.With("token", ptv2.GetName())

	if ptv2.Status == nil {
		ptv2.Status = &types.ProvisionTokenStatusV2{}
	}
	if ptv2.Status.BoundKeypair == nil {
		ptv2.Status.BoundKeypair = &types.ProvisionTokenStatusV2BoundKeypair{}
	}

	clusterName, err := a.GetClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	spec := ptv2.Spec.BoundKeypair
	status := ptv2.Status.BoundKeypair
	hasBoundPublicKey := status.BoundPublicKey != ""
	hasBoundBotInstance := status.BoundBotInstanceID != ""
	hasIncomingBotInstance := req.JoinRequest.BotInstanceID != ""
	hasJoinsRemaining := status.RecoveryCount < spec.Recovery.Limit

	recoveryMode, err := boundkeypair.ParseRecoveryMode(spec.Recovery.Mode)
	if err != nil {
		return nil, trace.Wrap(err, "parsing recovery mode")
	}

	// Join state is required after the initial join (first recovery), so long
	// as the mode is not insecure.
	joinStateRequired := status.RecoveryCount > 0 && recoveryMode != boundkeypair.RecoveryModeInsecure
	hasIncomingJoinState := len(req.PreviousJoinState) > 0

	// if set, the bound bot instance will be updated in the backend
	expectNewBotInstance := false

	// the bound public key; may change during initial join or rotation. used to
	// inform the returned public key value.
	boundPublicKey := status.BoundPublicKey

	// Mutators to use during the token resource status patch at the end.
	var mutators []boundKeypairStatusMutator

	// Get the join state JWT signer CA
	ca, err := a.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.BoundKeypairCA,
		DomainName: clusterName.GetClusterName(),
	}, /* loadKeys */ true)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var joinState *boundkeypair.JoinState
	if joinStateRequired {
		// If join state is required but missing, raise an error.
		if !hasIncomingJoinState {
			return nil, trace.AccessDenied("previous join state is required but was not provided")
		}

		log.DebugContext(ctx, "join state verification required, verifying")
		joinState, err = boundkeypair.VerifyJoinState(
			ca,
			string(req.PreviousJoinState),
			&boundkeypair.JoinStateParams{
				Clock:       a.clock,
				ClusterName: clusterName.GetClusterName(),
				Token:       ptv2,
			},
		)
		if err != nil {
			// TODO: Once we have token-specific locking, generate a lock; this
			// indicates the keypair may have been compromised.
			// TODO: Audit log event for this.
			log.ErrorContext(ctx, "bound keypair join state verification failed", "error", err)
			return nil, trace.AccessDenied("join state verification failed")
		}

		// Now that we've verified it, make sure the previous bot instance ID is
		// passed along to generateCerts. This will only be used if a new bot
		// instance is generated.
		req.JoinRequest.PreviousBotInstanceID = joinState.BotInstanceID

		log.DebugContext(ctx, "join state verified successfully", "join_state", joinState)

		// Note: we don't verify join state if it isn't expected. This is partly
		// to ensure server-side recovery will work if join state desyncs - a
		// cluster admin can change the recovery mode to insecure or reset the
		// recovery counter to zero and start over with a fresh join state, with
		// no client intervention.
	}

	switch {
	case !hasBoundPublicKey && !hasIncomingBotInstance:
		// Normal initial join attempt. No bound key, and no incoming bot
		// instance. Consumes a recovery attempt.
		if recoveryMode == boundkeypair.RecoveryModeStandard && !hasJoinsRemaining {
			return nil, trace.AccessDenied("no recovery attempts remaining")
		}

		if spec.Onboarding.InitialPublicKey != "" {
			// An initial public key was configured, so we can immediately ask
			// the client to complete a challenge.
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
			)

			boundPublicKey = spec.Onboarding.InitialPublicKey
		} else if status.RegistrationSecret != "" {
			// Shared error message for all registration secret check failures.
			const errMsg = "a valid registration secret is required"

			// A registration secret is expected.
			if req.InitialJoinSecret == "" {
				log.WarnContext(ctx, "denying join attempt, client failed to provide required registration secret")
				return nil, trace.AccessDenied(errMsg)
			}

			if spec.Onboarding.MustRegisterBefore != nil {
				if a.clock.Now().After(*spec.Onboarding.MustRegisterBefore) {
					log.WarnContext(
						ctx,
						"denying join attempt due to expired registration secret",
						"must_register_before",
						spec.Onboarding.MustRegisterBefore,
					)
					return nil, trace.AccessDenied(errMsg)
				}
			}

			// Verify the secret.
			if subtle.ConstantTimeCompare([]byte(status.RegistrationSecret), []byte(req.InitialJoinSecret)) != 1 {
				log.WarnContext(ctx, "denying join attempt, client provided incorrect registration secret")
				return nil, trace.AccessDenied(errMsg)
			}

			// Ask the client for a new public key.
			newPubKey, err := a.requestBoundKeypairRotation(ctx, challengeResponse)
			if err != nil {
				return nil, trace.Wrap(err, "requesting public key")
			}

			// The rotation process verifies private key ownership, so we can
			// consider it it bound. Note that for our purposes here, this
			// initial join will not count as a rotation.
			mutators = append(
				mutators,
				mutateStatusBoundPublicKey(newPubKey, ""),
				mutateStatusClearRegistrationSecret(status.RegistrationSecret),
			)

			boundPublicKey = newPubKey
		} else {
			return nil, trace.BadParameter("either an initial public key or registration secret is required")
		}

		// If we reach this point, it counts as a recovery, so add a join
		// mutator.
		mutators = append(
			mutators,
			mutateStatusConsumeRecovery(recoveryMode, status.RecoveryCount, spec.Recovery.Limit),
		)

		expectNewBotInstance = true
	case !hasBoundPublicKey && hasIncomingBotInstance:
		// Not allowed, at least at the moment. This would imply e.g. trying to
		// change auth methods.
		return nil, trace.BadParameter("cannot perform first bound keypair join with existing credentials")
	case hasBoundPublicKey && !hasBoundBotInstance:
		// TODO: Bad backend state, or maybe an incomplete previous join
		// attempt. This shouldn't be a possible state, but we should handle it
		// sanely anyway.
		return nil, trace.BadParameter("bad backend state, please recreate the join token")
	case hasBoundPublicKey && hasBoundBotInstance && hasIncomingBotInstance:
		// Standard rejoin case, does not consume a rejoin.
		if status.BoundBotInstanceID != req.JoinRequest.BotInstanceID {
			return nil, trace.AccessDenied("bot instance mismatch")
		}

		if err := a.issueBoundKeypairChallenge(
			ctx,
			status.BoundPublicKey,
			challengeResponse,
		); err != nil {
			return nil, trace.Wrap(err)
		}

		// Nothing else to do, no key change
	case hasBoundPublicKey && hasBoundBotInstance && !hasIncomingBotInstance:
		// Hard rejoin case, the client identity expired and a new bot instance
		// is required. Consumes a rejoin.
		if recoveryMode == boundkeypair.RecoveryModeStandard && !hasJoinsRemaining {
			// Recovery limit only applies in "standard" mode.
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
			mutateStatusConsumeRecovery(recoveryMode, status.RecoveryCount, spec.Recovery.Limit),
		)

		expectNewBotInstance = true
	default:
		log.ErrorContext(
			ctx, "unexpected state",
			"has_bound_public_key", hasBoundPublicKey,
			"has_bound_bot_instance", hasBoundBotInstance,
			"has_incoming_bot_instance", hasIncomingBotInstance,
			"spec", spec,
			"status", status,
		)
		return nil, trace.BadParameter("unexpected state")
	}

	// If we've crossed a keypair rotation threshold, request one now.
	now := a.clock.Now()
	if shouldRequestBoundKeypairRotation(spec.RotateAfter, status.LastRotatedAt, now) {
		log.DebugContext(
			ctx, "requesting keypair rotation",
			"rotate_after", formatTimePointer(spec.RotateAfter),
			"last_rotated_at", formatTimePointer(status.LastRotatedAt),
		)
		newPubKey, err := a.requestBoundKeypairRotation(ctx, challengeResponse)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// Don't let clients provide the same key again.
		if err := ensurePublicKeysNotEqual(boundPublicKey, newPubKey); err != nil {
			return nil, trace.Wrap(err)
		}

		mutators = append(mutators,
			mutateStatusBoundPublicKey(newPubKey, boundPublicKey),
			mutateStatusLastRotatedAt(&now, status.LastRotatedAt),
		)

		boundPublicKey = newPubKey

		// TODO: Follow up with an audit log event for this.
	}

	certs, botInstanceID, err := a.generateCertsBot(
		ctx,
		ptv2,
		req.JoinRequest,
		nil, // TODO: extended claims for this type?
		nil, // TODO: workload id claims
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if expectNewBotInstance {
		mutators = append(
			mutators,
			mutateStatusBoundBotInstance(botInstanceID, status.BoundBotInstanceID),
		)
	}

	// A reference to the final provision token state; may be modified below via
	// mutators.
	finalToken := ptv2

	if len(mutators) > 0 {
		patched, err := a.PatchToken(ctx, ptv2.GetName(), func(token types.ProvisionToken) (types.ProvisionToken, error) {
			ptv2, ok := provisionToken.(*types.ProvisionTokenV2)
			if !ok {
				return nil, trace.BadParameter("expected *types.ProvisionTokenV2, got %T", provisionToken)
			}

			// Apply all mutators. Individual mutators may make additional
			// assertions to ensure invariants haven't changed.
			for _, mutator := range mutators {
				if err := mutator(ptv2.Spec.BoundKeypair, ptv2.Status.BoundKeypair); err != nil {
					return nil, trace.Wrap(err, "applying status mutator")
				}
			}

			return ptv2, nil
		})
		if err != nil {
			return nil, trace.Wrap(err, "committing updated token state, please try again")
		}

		finalToken, ok = patched.(*types.ProvisionTokenV2)
		if !ok {
			// This should be impossible, but if it did fail, we can't generate
			// a join state without an accurate token. The certs we just
			// generated will be useless, so just return an error.
			return nil, trace.BadParameter("expected *types.ProvisionTokenV2, got %T", provisionToken)
		}
	}

	signer, err := a.GetKeyStore().GetJWTSigner(ctx, ca)
	if err != nil {
		return nil, trace.Wrap(err, "issuing join state document")
	}

	newJoinState, err := boundkeypair.IssueJoinState(signer, &boundkeypair.JoinStateParams{
		Clock:       a.clock,
		ClusterName: clusterName.GetClusterName(),
		Token:       finalToken,
	})
	if err != nil {
		return nil, trace.Wrap(err, "issuing join state document")
	}

	return &client.BoundKeypairRegistrationResponse{
		Certs:          certs,
		BoundPublicKey: boundPublicKey,
		JoinState:      []byte(newJoinState),
	}, nil
}
