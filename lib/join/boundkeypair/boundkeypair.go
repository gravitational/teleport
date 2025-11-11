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

package boundkeypair

import (
	"context"
	"crypto"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/keystore"
	"github.com/gravitational/teleport/lib/boundkeypair"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/join/internal/authz"
	"github.com/gravitational/teleport/lib/join/internal/diagnostic"
	"github.com/gravitational/teleport/lib/join/internal/messages"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/services/readonly"
	libsshutils "github.com/gravitational/teleport/lib/sshutils"
)

type BoundKeypairValidator interface {
	IssueChallenge() (*boundkeypair.ChallengeDocument, error)
	ValidateChallengeResponse(issued *boundkeypair.ChallengeDocument, compactResponse string) error
}

type CreateBoundKeypairValidator func(subject string, clusterName string, publicKey crypto.PublicKey) (BoundKeypairValidator, error)

// issueBoundKeypairChallenge creates a new challenge for the given marshaled
// public key in ssh authorized_keys format, requests a solution from the
// client using the given `challengeResponse` function, and validates the
// response.
func issueBoundKeypairChallenge(
	ctx context.Context,
	params *JoinParams,
	marshaledKey string,
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

	clusterName, err := params.AuthService.GetClusterName(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	params.Logger.DebugContext(ctx, "issuing bound keypair challenge", "key_id", keyID)

	validator, err := params.CreateBoundKeypairValidator(keyID, clusterName.GetClusterName(), key)
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

	solution, err := params.IssueChallenge(&messages.BoundKeypairChallenge{
		PublicKey: []byte(marshaledKey),
		Challenge: string(marshalledChallenge),
	})
	if err != nil {
		return trace.Wrap(err, "requesting a signed challenge")
	}

	if err := validator.ValidateChallengeResponse(
		challenge,
		string(solution.Solution),
	); err != nil {
		// TODO: Consider access denied instead?
		return trace.Wrap(err, "validating challenge response")
	}

	params.Logger.InfoContext(ctx, "bound keypair challenge response verified successfully", "key_id", keyID)

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
func requestBoundKeypairRotation(
	ctx context.Context,
	params *JoinParams,
) (string, error) {
	cap, err := params.AuthService.GetAuthPreference(ctx)
	if err != nil {
		return "", trace.Wrap(err)
	}

	params.Logger.InfoContext(ctx, "requesting bound keypair rotation", "suite", cap.GetSignatureAlgorithmSuite())

	// Request a new marshaled public key from the client.
	algoSuite := types.SignatureAlgorithmSuiteToString(cap.GetSignatureAlgorithmSuite())
	rotationResponse, err := params.IssueRotationRequest(&messages.BoundKeypairRotationRequest{
		SignatureAlgorithmSuite: algoSuite,
	})
	if err != nil {
		return "", trace.Wrap(err, "requesting a new public key")
	}

	// Issue a challenge against this new key to ensure ownership.
	pubKey := string(rotationResponse.PublicKey)
	if err := issueBoundKeypairChallenge(ctx, params, pubKey); err != nil {
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
func mutateStatusConsumeRecovery(expectRecoveryCount uint32, expectMinRecoveryLimit uint32) boundKeypairStatusMutator {
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

// emitBoundKeypairRecoveryEvent emits an audit event indicating a bound keypair
// token was used to recover a bot.
func emitBoundKeypairRecoveryEvent(
	ctx context.Context,
	params *JoinParams,
	token *types.ProvisionTokenV2,
	boundPublicKey string,
	recoveryCount uint32,
	err error,
) {
	var status apievents.Status
	if err == nil {
		status = apievents.Status{
			Success: true,
		}
	} else {
		status = apievents.Status{
			Success: false,
			Error:   err.Error(),
		}
	}

	if err := params.AuthService.EmitAuditEvent(context.WithoutCancel(ctx), &apievents.BoundKeypairRecovery{
		Metadata: apievents.Metadata{
			Type: events.BoundKeypairRecovery,
			Code: events.BoundKeypairRecoveryCode,
		},
		Status: status,
		ConnectionMetadata: apievents.ConnectionMetadata{
			RemoteAddr: params.Diag.Get().RemoteAddr,
		},
		TokenName:     token.GetName(),
		BotName:       token.GetBotName(),
		PublicKey:     boundPublicKey,
		RecoveryCount: recoveryCount,
		RecoveryMode:  token.Spec.BoundKeypair.Recovery.Mode,
	}); err != nil {
		params.Logger.WarnContext(ctx, "Failed to emit failed bound keypair recovery event", "error", err)
	}
}

// emitBoundKeypairRotationEvent emits an audit event indicating a bound keypair
// rotation occurred.
func emitBoundKeypairRotationEvent(
	ctx context.Context,
	params *JoinParams,
	token *types.ProvisionTokenV2,
	prevPublicKey, newPublicKey string,
	err error,
) {
	var status apievents.Status
	if err == nil {
		status = apievents.Status{
			Success: true,
		}
	} else {
		status = apievents.Status{
			Success: false,
			Error:   err.Error(),
		}
	}

	if err := params.AuthService.EmitAuditEvent(context.WithoutCancel(ctx), &apievents.BoundKeypairRotation{
		Metadata: apievents.Metadata{
			Type: events.BoundKeypairRotation,
			Code: events.BoundKeypairRotationCode,
		},
		Status: status,
		ConnectionMetadata: apievents.ConnectionMetadata{
			RemoteAddr: params.Diag.Get().RemoteAddr,
		},
		TokenName:         token.GetName(),
		BotName:           token.GetBotName(),
		PreviousPublicKey: prevPublicKey,
		NewPublicKey:      newPublicKey,
	}); err != nil {
		params.Logger.WarnContext(ctx, "Failed to emit failed bound keypair rotation event", "error", err)
	}
}

func tryLockBotInvalidJoinState(
	ctx context.Context,
	params *JoinParams,
	ptv2 *types.ProvisionTokenV2,
	validationError error,
) {
	log := params.Logger.With("join_token", ptv2.GetName(), "validation_error", validationError)

	if auditErr := params.AuthService.EmitAuditEvent(context.WithoutCancel(ctx), &apievents.BoundKeypairJoinStateVerificationFailed{
		Metadata: apievents.Metadata{
			Type: events.BoundKeypairJoinStateVerificationFailed,
			Code: events.BoundKeypairJoinStateVerificationFailedCode,
		},
		Status: apievents.Status{
			Success: false,
			Error:   validationError.Error(),
		},
		ConnectionMetadata: apievents.ConnectionMetadata{
			RemoteAddr: params.Diag.Get().RemoteAddr,
		},
		TokenName: ptv2.GetName(),
		BotName:   ptv2.GetBotName(),
	}); auditErr != nil {
		log.WarnContext(ctx, "Failed to emit failed join state verification event", "error", auditErr)
	}

	// Create a lock against this token.
	lock, err := types.NewLock(uuid.New().String(), types.LockSpecV2{
		Target: types.LockTarget{
			JoinToken: ptv2.GetName(),
		},
		Message: fmt.Sprintf(
			"The join token %q has been locked by bot %q after a client "+
				"failed to verify its join state, possibly indicating a "+
				"stolen keypair.",
			ptv2.GetName(), ptv2.GetBotName(),
		),
		CreatedAt: params.Clock.Now(),
	})
	if err != nil {
		params.Logger.ErrorContext(ctx, "Unable to create lock for bound keypair token")
		return
	}
	if err := params.AuthService.UpsertLock(ctx, lock); err != nil {
		log.ErrorContext(ctx, "Unable to create lock for bound keypair token after join state verification failed")
	}
}

// verifyBoundKeypairJoinState verifies the client's provided join state
// document if the current state of the token indicates the join state must be
// verified. If verification is required and fails, this returns an error and
// locks the token until a cluster admin can ensure the token hasn't been
// compromised. If verification is not required, this is a no-op. Join state
// should be verified whenever a client rejoins, but only after they have proven
// ownership of their private key.
func verifyBoundKeypairJoinState(
	ctx context.Context,
	params *JoinParams,
	ptv2 *types.ProvisionTokenV2,
	ca types.CertAuthority,
) (previousBotInstnceID string, err error) {
	recoveryMode, err := boundkeypair.ParseRecoveryMode(ptv2.Spec.BoundKeypair.Recovery.Mode)
	if err != nil {
		return "", trace.Wrap(err, "parsing recovery mode")
	}

	// Join state is required after the initial join (first recovery), so long
	// as the mode is not insecure.
	// Note: we don't verify join state if it isn't expected. This is partly
	// to ensure server-side recovery will work if join state desyncs - a
	// cluster admin can change the recovery mode to insecure or reset the
	// recovery counter to zero and start over with a fresh join state, with
	// no client intervention.
	joinStateRequired := ptv2.Status.BoundKeypair.RecoveryCount > 0 && recoveryMode != boundkeypair.RecoveryModeInsecure
	if !joinStateRequired {
		params.Logger.DebugContext(
			ctx,
			"skipping join state verification, not required due to token state",
			"recovery_count", ptv2.Status.BoundKeypair.RecoveryCount,
			"recovery_mode", ptv2.Spec.BoundKeypair.Recovery.Mode,
		)
		return "", nil
	}

	// If join state is required but missing, raise an error.
	hasIncomingJoinState := len(params.BoundKeypairInit.PreviousJoinState) > 0
	if !hasIncomingJoinState {
		return "", trace.AccessDenied("previous join state is required but was not provided")
	}

	params.Logger.DebugContext(ctx, "join state verification required, verifying")
	joinState, err := boundkeypair.VerifyJoinState(
		ca,
		string(params.BoundKeypairInit.PreviousJoinState),
		&boundkeypair.JoinStateParams{
			Clock:       params.Clock,
			ClusterName: ca.GetClusterName(), // equivalent to clusterName but saves a method param
			Token:       ptv2,
		},
	)
	if err != nil {
		params.Logger.ErrorContext(ctx, "bound keypair join state verification failed", "error", err)
		tryLockBotInvalidJoinState(ctx, params, ptv2, err)

		return "", trace.AccessDenied("join state verification failed")
	}

	params.Logger.DebugContext(ctx, "join state verified successfully", "join_state", joinState)
	// Now that we've verified it, return the previous bot instance ID.
	return joinState.BotInstanceID, nil
}

// verifyLocksForBoundKeypairToken checks if any token-level locks are in place
// against the given token.  This should ideally be called after the request has
// been authenticated (exact criteria varies depending on token state) but
// before the request has mutated anything on the server - including creation of
// additional locks: we don't want to allow continuous lock creation.
func verifyLocksForBoundKeypairToken(ctx context.Context, params *JoinParams, token *types.ProvisionTokenV2) error {
	readOnlyAuthPref, err := params.AuthService.GetReadOnlyAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(params.AuthService.CheckLockInForce(
		readOnlyAuthPref.GetLockingMode(),
		[]types.LockTarget{{JoinToken: token.GetName()}},
	))
}

// JoinParams holds all parameters necessary to handle a bound keypair join attempt.
type JoinParams struct {
	// AuthService is the auth service.
	AuthService AuthService
	// AuthCtx is authentication context for the request, relevant for re-join attempts.
	AuthCtx *authz.Context
	// Diag is the join attempt diagnostic.
	Diag *diagnostic.Diagnostic
	// ProvisionToken is the provision token used for the join attempt.
	ProvisionToken types.ProvisionToken
	// ClientInit is the ClientInit message sent by the joining client.
	ClientInit *messages.ClientInit
	// BoundKeypairInit is the BoundKeypairInit message sent by the joining client.
	BoundKeypairInit *messages.BoundKeypairInit
	// IssueChallenge sends a challenge to the joining client and returns the
	// response.
	IssueChallenge ChallengeResponseFunc
	// IssueRotationRequest sends a rotation request to the joining client and
	// returns the response.
	IssueRotationRequest RotationFunc
	// CreateBoundKeypairValidator is a function that creates a bound keypair
	// validator, used to override the validator in tests.
	CreateBoundKeypairValidator CreateBoundKeypairValidator
	// GenerateBotCerts is a function that generates bot certificates.
	GenerateBotCerts func(ctx context.Context, previousBotInstanceID string, claims any) (*messages.Certificates, string, error)
	// Clock is the clock.
	Clock clockwork.Clock
	// Logger is a logger.
	Logger *slog.Logger
}

// ChallengeResponseFunc is function that sends a bound keypair challenge and
// returns the response.
type ChallengeResponseFunc func(*messages.BoundKeypairChallenge) (*messages.BoundKeypairChallengeSolution, error)

// RotationFunc is function that sends a bound keypair rotation request and
// returns the response.
type RotationFunc func(*messages.BoundKeypairRotationRequest) (*messages.BoundKeypairRotationResponse, error)

func (p *JoinParams) checkAndSetDefaults() error {
	switch {
	case p.AuthService == nil:
		return trace.BadParameter("AuthService is required")
	case p.AuthCtx == nil:
		return trace.BadParameter("AuthCtx is required")
	case p.Diag == nil:
		return trace.BadParameter("Diag is required")
	case p.ProvisionToken == nil:
		return trace.BadParameter("ProvisionToken is required")
	case p.ClientInit == nil:
		return trace.BadParameter("ClientInit is required")
	case p.BoundKeypairInit == nil:
		return trace.BadParameter("BoundKeypairInit is required")
	case p.IssueChallenge == nil:
		return trace.BadParameter("IssueChallenge is required")
	case p.IssueRotationRequest == nil:
		return trace.BadParameter("IssueRotationRequest is required")
	case p.Logger == nil:
		return trace.BadParameter("Logger is required")
	}
	if p.CreateBoundKeypairValidator == nil {
		p.CreateBoundKeypairValidator = func(subject string, clusterName string, publicKey crypto.PublicKey) (BoundKeypairValidator, error) {
			return boundkeypair.NewChallengeValidator(subject, clusterName, publicKey)
		}
	}
	if p.Clock == nil {
		p.Clock = clockwork.NewRealClock()
	}
	return nil
}

// AuthService is the subset of the Auth service interface required to implement bound keypair joining.
type AuthService interface {
	EmitAuditEvent(ctx context.Context, e apievents.AuditEvent) error
	GetClusterName(context.Context) (types.ClusterName, error)
	GetCertAuthority(context.Context, types.CertAuthID, bool) (types.CertAuthority, error)
	GetKeyStore() *keystore.Manager
	PatchToken(context.Context, string, func(types.ProvisionToken) (types.ProvisionToken, error)) (types.ProvisionToken, error)
	GetAuthPreference(context.Context) (types.AuthPreference, error)
	GetReadOnlyAuthPreference(context.Context) (readonly.AuthPreference, error)
	UpsertLock(context.Context, types.Lock) error
	CheckLockInForce(constants.LockingMode, []types.LockTarget) error
}

// HandleBoundKeypairJoin handles joining requests for the bound keypair join
// method.
func HandleBoundKeypairJoin(
	ctx context.Context,
	params *JoinParams,
) (*messages.BotResult, error) {
	if err := params.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	// Only bot joining is supported at the moment - unique ID verification is
	// required and this is currently only implemented for bots.
	if types.SystemRole(params.ClientInit.SystemRole) != types.RoleBot {
		return nil, trace.BadParameter("bound keypair joining is only supported for bots")
	}

	ptv2, ok := params.ProvisionToken.(*types.ProvisionTokenV2)
	if !ok {
		return nil, trace.BadParameter("expected *types.ProvisionTokenV2, got %T", params.ProvisionToken)
	}

	log := params.Logger.With("token", ptv2.GetName())

	if ptv2.Status == nil {
		ptv2.Status = &types.ProvisionTokenStatusV2{}
	}
	if ptv2.Status.BoundKeypair == nil {
		ptv2.Status.BoundKeypair = &types.ProvisionTokenStatusV2BoundKeypair{}
	}

	clusterName, err := params.AuthService.GetClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	spec := ptv2.Spec.BoundKeypair
	status := ptv2.Status.BoundKeypair
	hasBoundPublicKey := status.BoundPublicKey != ""
	hasBoundBotInstance := status.BoundBotInstanceID != ""
	hasIncomingBotInstance := params.AuthCtx.BotInstanceID != ""
	hasJoinsRemaining := status.RecoveryCount < spec.Recovery.Limit

	recoveryMode, err := boundkeypair.ParseRecoveryMode(spec.Recovery.Mode)
	if err != nil {
		return nil, trace.Wrap(err, "parsing recovery mode")
	}

	// if set, the bound bot instance will be updated in the backend
	expectNewBotInstance := false

	// the bound public key; may change during initial join or rotation. used to
	// inform the returned public key value.
	boundPublicKey := status.BoundPublicKey

	// the recovery count; this is informational and used to generate extended
	// claims for audit log purposes. The actual enforced value is incremented
	// by `mutateStatusConsumeRecovery`.
	recoveryCount := status.RecoveryCount

	// Mutators to use during the token resource status patch at the end.
	var mutators []boundKeypairStatusMutator

	// Get the join state JWT signer CA
	ca, err := params.AuthService.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.BoundKeypairCA,
		DomainName: clusterName.GetClusterName(),
	}, /* loadKeys */ true)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var verifiedPreviousBotInstanceID string
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
			if err := issueBoundKeypairChallenge(
				ctx,
				params,
				spec.Onboarding.InitialPublicKey,
			); err != nil {
				log.WarnContext(ctx, "denying initial join attempt, client failed to complete challenge", "error", err)
				emitBoundKeypairRecoveryEvent(ctx, params, ptv2, spec.Onboarding.InitialPublicKey, 0, err)
				return nil, trace.AccessDenied("failed to complete challenge")
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
			if params.BoundKeypairInit.InitialJoinSecret == "" {
				log.WarnContext(ctx, "denying join attempt, client failed to provide required registration secret")
				emitBoundKeypairRecoveryEvent(ctx, params, ptv2, "", 0, trace.AccessDenied("no registration secret was provided"))
				return nil, trace.AccessDenied(errMsg)
			}

			if spec.Onboarding.MustRegisterBefore != nil {
				if params.Clock.Now().After(*spec.Onboarding.MustRegisterBefore) {
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
			if subtle.ConstantTimeCompare([]byte(status.RegistrationSecret), []byte(params.BoundKeypairInit.InitialJoinSecret)) != 1 {
				log.WarnContext(ctx, "denying join attempt, client provided incorrect registration secret")
				emitBoundKeypairRecoveryEvent(ctx, params, ptv2, "", 0, trace.AccessDenied("registration secret comparison failed"))
				return nil, trace.AccessDenied(errMsg)
			}

			// Ask the client for a new public key.
			newPubKey, err := requestBoundKeypairRotation(ctx, params)
			if err != nil {
				// Audit note: `requestBoundKeypairRotation()` will also emit an
				// audit event.
				emitBoundKeypairRecoveryEvent(ctx, params, ptv2, "", 0, err)
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
			// Audit note: this would be an implementation error, so doesn't
			// warrant an audit event.
			return nil, trace.BadParameter("either an initial public key or registration secret is required")
		}

		// If we reach this point, it counts as a recovery, so add a join
		// mutator.
		mutators = append(
			mutators,
			mutateStatusConsumeRecovery(status.RecoveryCount, spec.Recovery.Limit),
		)

		// Verify locks here, but only after we've tentatively authenticated the
		// request. We don't want to leak the lock status to random
		// unauthenticated clients, and by this point, we haven't mutated any
		// server-side state.
		if err := verifyLocksForBoundKeypairToken(ctx, params, ptv2); err != nil {
			return nil, trace.Wrap(err)
		}

		// Note: this is the initial join, so no join state to verify.

		recoveryCount += 1
		expectNewBotInstance = true
		emitBoundKeypairRecoveryEvent(ctx, params, ptv2, boundPublicKey, recoveryCount, nil)
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
		if err := issueBoundKeypairChallenge(
			ctx,
			params,
			status.BoundPublicKey,
		); err != nil {
			return nil, trace.Wrap(err)
		}

		// Verify locks here now that we've verified private key ownership but
		// before we check join state. Otherwise, we could allow a lock creation
		// loop.
		if err := verifyLocksForBoundKeypairToken(ctx, params, ptv2); err != nil {
			return nil, trace.Wrap(err)
		}

		// Once we've verified the client has the matching private key, validate
		// the join state. This must be done after a successful challenge to
		// make sure an otherwise unauthorized client can't trigger a lockout.
		// This also needs to be done before rotation to prevent an attacker
		// from rotating the key.
		verifiedPreviousBotInstanceID, err = verifyBoundKeypairJoinState(ctx, params, ptv2, ca)
		if err != nil {
			return nil, trace.AccessDenied("join state verification failed")
		}

		// Join state verification will check the instance IDs in the token and
		// join state document, but as a sanity check, we'll also ensure it
		// matches the value extracted from the certs.
		//
		// It should not be possible for this check to fail at this point, as
		// any event that might have cycled bot instance IDs should have also
		// modified the join state causing a failure above. In any case, we'll
		// keep this as a sanity check.
		if status.BoundBotInstanceID != params.AuthCtx.BotInstanceID {
			return nil, trace.AccessDenied("bot instance mismatch")
		}

		// Nothing else to do, no key change, no additional audit event; regular
		// bot join event will be emitted later.
	case hasBoundPublicKey && hasBoundBotInstance && !hasIncomingBotInstance:
		if err := issueBoundKeypairChallenge(
			ctx,
			params,
			status.BoundPublicKey,
		); err != nil {
			emitBoundKeypairRecoveryEvent(ctx, params, ptv2, boundPublicKey, recoveryCount, err)
			return nil, trace.Wrap(err)
		}

		// Hard rejoin case, the client identity expired and a new bot instance
		// is required. Consumes a rejoin.
		if recoveryMode == boundkeypair.RecoveryModeStandard && !hasJoinsRemaining {
			// Recovery limit only applies in "standard" mode.
			return nil, trace.AccessDenied("no rejoins remaining")
		}

		// Verify locks here now that we've verified private key ownership but
		// before we check join state. Otherwise, we could allow a lock creation
		// loop.
		if err := verifyLocksForBoundKeypairToken(ctx, params, ptv2); err != nil {
			return nil, trace.Wrap(err)
		}

		// As in the standard case above, once we've verified the client has the
		// matching private key, validate the join state.
		verifiedPreviousBotInstanceID, err = verifyBoundKeypairJoinState(ctx, params, ptv2, ca)
		if err != nil {
			return nil, trace.AccessDenied("join state verification failed")
		}

		mutators = append(
			mutators,
			mutateStatusConsumeRecovery(status.RecoveryCount, spec.Recovery.Limit),
		)

		recoveryCount += 1
		expectNewBotInstance = true
		emitBoundKeypairRecoveryEvent(ctx, params, ptv2, boundPublicKey, recoveryCount, nil)
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
	now := params.Clock.Now()
	if shouldRequestBoundKeypairRotation(spec.RotateAfter, status.LastRotatedAt, now) {
		log.DebugContext(
			ctx, "requesting keypair rotation",
			"rotate_after", formatTimePointer(spec.RotateAfter),
			"last_rotated_at", formatTimePointer(status.LastRotatedAt),
		)
		newPubKey, err := requestBoundKeypairRotation(ctx, params)
		if err != nil {
			emitBoundKeypairRotationEvent(ctx, params, ptv2, boundPublicKey, "", err)
			return nil, trace.Wrap(err)
		}

		// Don't let clients provide the same key again.
		if err := ensurePublicKeysNotEqual(boundPublicKey, newPubKey); err != nil {
			emitBoundKeypairRotationEvent(ctx, params, ptv2, boundPublicKey, newPubKey, err)
			return nil, trace.Wrap(err)
		}

		mutators = append(mutators,
			mutateStatusBoundPublicKey(newPubKey, boundPublicKey),
			mutateStatusLastRotatedAt(&now, status.LastRotatedAt),
		)

		emitBoundKeypairRotationEvent(ctx, params, ptv2, boundPublicKey, newPubKey, nil)
		boundPublicKey = newPubKey
	}

	certs, botInstanceID, err := params.GenerateBotCerts(ctx, verifiedPreviousBotInstanceID, &boundkeypair.Claims{
		PublicKey:     boundPublicKey,
		RecoveryCount: recoveryCount,
		RecoveryMode:  recoveryMode,
	})
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
		patched, err := params.AuthService.PatchToken(ctx, ptv2.GetName(), func(token types.ProvisionToken) (types.ProvisionToken, error) {
			ptv2, ok := params.ProvisionToken.(*types.ProvisionTokenV2)
			if !ok {
				return nil, trace.BadParameter("expected *types.ProvisionTokenV2, got %T", params.ProvisionToken)
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
			return nil, trace.BadParameter("expected *types.ProvisionTokenV2, got %T", params.ProvisionToken)
		}
	}

	signer, err := params.AuthService.GetKeyStore().GetJWTSigner(ctx, ca)
	if err != nil {
		return nil, trace.Wrap(err, "issuing join state document")
	}

	newJoinState, err := boundkeypair.IssueJoinState(signer, &boundkeypair.JoinStateParams{
		Clock:       params.Clock,
		ClusterName: clusterName.GetClusterName(),
		Token:       finalToken,
	})
	if err != nil {
		return nil, trace.Wrap(err, "issuing join state document")
	}

	return &messages.BotResult{
		Certificates: *certs,
		BoundKeypairResult: &messages.BoundKeypairResult{
			JoinState: []byte(newJoinState),
			PublicKey: []byte(boundPublicKey),
		},
	}, nil
}
