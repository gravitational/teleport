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

package auth

import (
	"cmp"
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/client/proto"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/internal/cert"
	"github.com/gravitational/teleport/lib/auth/machineid/machineidv1"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

func sshPublicKeyToPKIXPEM(pubKey []byte) ([]byte, error) {
	cryptoPubKey, err := sshutils.CryptoPublicKey(pubKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return keys.MarshalPublicKey(cryptoPubKey)
}

// tryLockBotDueToGenerationMismatch creates a lock for the given bot user and
// emits a `RenewableCertificateGenerationMismatch` audit event.
func (a *Server) tryLockBotDueToGenerationMismatch(
	ctx context.Context, botName, botInstanceID, joinTokenName string, renewable bool,
) error {
	var spec types.LockSpecV2
	if renewable {
		// Renewable implies `token` joining. These are one-time use secrets
		// and will not be embedded in the TLS identity, so we can't target
		// the join token and should instead rely on the bot instance ID. As
		// there is a 1:1 relationship between bot instance and "token"-type
		// token, this should be functionally equivalent.
		spec = types.LockSpecV2{
			Target: types.LockTarget{
				BotInstanceID: botInstanceID,
			},
			Message: fmt.Sprintf(
				"The bot instance %s/%s has been locked due to a certificate "+
					"generation mismatch, possibly indicating a stolen "+
					"certificate.",
				botName, botInstanceID,
			),
			CreatedAt: a.clock.Now(),
		}
	} else {
		spec = types.LockSpecV2{
			Target: types.LockTarget{
				JoinToken: joinTokenName,
			},
			Message: fmt.Sprintf(
				"Bot joins via the token %q have been locked due to a "+
					"certificate generation mismatch by %s/%s, possibly "+
					"indicating a stolen certificate.",
				joinTokenName, botName, botInstanceID,
			),
			CreatedAt: a.clock.Now(),
		}
	}

	lock, err := types.NewLock(uuid.New().String(), spec)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := a.UpsertLock(ctx, lock); err != nil {
		return trace.Wrap(err)
	}

	// Emit an audit event.
	userMetadata := authz.ClientUserMetadata(ctx)
	if err := a.emitter.EmitAuditEvent(a.closeCtx, &apievents.RenewableCertificateGenerationMismatch{
		Metadata: apievents.Metadata{
			Type: events.RenewableCertificateGenerationMismatchEvent,
			Code: events.RenewableCertificateGenerationMismatchCode,
		},
		UserMetadata: userMetadata,
	}); err != nil {
		a.logger.WarnContext(ctx, "Failed to emit renewable cert generation mismatch event", "error", err)
	}

	return nil
}

// shouldEnforceGenerationCounter decides if generation counter checks should be
// enforced for a given join method. Note that in certain situations the counter
// may still not technically be enforced, for example, when onboarding a new bot
// or recovering a bound keypair bot.
func shouldEnforceGenerationCounter(renewable bool, joinMethod string) bool {
	if renewable {
		return true
	}

	// Note: token renewals are handled by the `renewable` check above, since
	// those certs are issued via `ServerWithRoles.generateUserCerts()` and do
	// not have an associated join method.
	switch joinMethod {
	case string(types.JoinMethodBoundKeypair):
		return true
	default:
		return false
	}
}

func authRecordForUpdate(
	req *cert.Request,
	joinMethod string,
	now time.Time,
	templateAuthRecord *machineidv1pb.BotInstanceStatusAuthentication,
) (*machineidv1pb.BotInstanceStatusAuthentication, error) {
	publicKeyPEM := req.TLSPublicKey
	if publicKeyPEM == nil {
		// At least one of tlsPublicKey or sshPublicKey will be set, this is
		// validated by [req.check].
		var err error
		publicKeyPEM, err = sshPublicKeyToPKIXPEM(req.SSHPublicKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	authRecord := machineidv1pb.BotInstanceStatusAuthentication_builder{
		AuthenticatedAt: timestamppb.New(now),
		PublicKey:       publicKeyPEM,
		JoinMethod:      joinMethod,
	}.Build()

	if templateAuthRecord != nil {
		authRecord.SetJoinToken(templateAuthRecord.GetJoinToken())
		authRecord.SetJoinAttrs(templateAuthRecord.GetJoinAttrs())
	}

	return authRecord, nil
}

// updateBotInstance updates the bot instance associated with the context
// identity, if any. If the optional `templateAuthRecord` is provided, various
// metadata fields will be copied into the newly generated auth record.
func (a *Server) updateBotInstance(
	ctx context.Context, req *cert.Request,
	templateAuthRecord *machineidv1pb.BotInstanceStatusAuthentication,
	currentIdentityGeneration int32,
) error {
	if req.BotName == "" {
		// Only applies to bot identities
		return nil
	}
	// We expect all renewals to have a bot instance ID and current identity
	// generation. n.b: The bot instance ID may refer to a bot instance that no
	// longer exists.
	if req.BotInstanceID == "" {
		return trace.AccessDenied("bot identity is missing a bot instance ID")
	}
	if currentIdentityGeneration <= 0 {
		return trace.AccessDenied("a current identity generation must be provided")
	}

	// Presumed to be a token join unless a `templateAuthRecord` says otherwise.
	// This accounts for the call from GenerateUserCerts - where no auth record
	// is provided to updateBotInstance. We know that all calls to
	// GenerateUserCerts must be token join based.
	joinMethod := cmp.Or(
		templateAuthRecord.GetJoinMethod(), string(types.JoinMethodToken),
	)

	existingInstance, err := a.BotInstance.GetBotInstance(
		ctx, machineidv1pb.GetBotInstanceRequest_builder{
			BotScope:   req.BotScope,
			BotName:    req.BotName,
			InstanceId: req.BotInstanceID,
		}.Build(),
	)
	if trace.IsNotFound(err) {
		// The identity references an instance record that no longer exists
		// (admin deletion, expiry, or backend rollback).

		// For methods that enforce the generation counter, the instance record
		// backing it is gone, so the presented generation cannot be verified.
		// Treat it as a mismatch.
		if shouldEnforceGenerationCounter(req.Renewable, joinMethod) {
			if err := a.tryLockBotDueToGenerationMismatch(
				ctx, req.BotName, req.BotInstanceID, req.JoinToken, req.Renewable,
			); err != nil {
				a.logger.WarnContext(
					ctx,
					"Failed to lock bot when a generation mismatch was detected",
					"error", err,
				)
			}

			return trace.AccessDenied(
				"cert generation mismatch for bot %s/%s: instance not found, presented=%v",
				req.BotName, req.BotInstanceID, currentIdentityGeneration,
			)
		}

		// For other methods, we gracefully fall back to creating a new bot
		// instance. A hard error is unattractive as we'd place tbot into a
		// failure loop until the identity expires.
		newInstanceID, err := uuid.NewRandom()
		if err != nil {
			return trace.Wrap(err)
		}

		a.logger.WarnContext(
			ctx,
			"bot rejoined with a nonzero generation but its instance record was not found, a fresh instance will be issued",
			"bot_name", req.BotName,
			"missing_instance_id", req.BotInstanceID,
			"new_instance_id", logutils.StringerAttr(newInstanceID),
			"identity_generation", currentIdentityGeneration,
			"join_method", joinMethod,
		)

		authRecord, err := authRecordForUpdate(
			req, joinMethod, a.GetClock().Now(), templateAuthRecord,
		)
		if err != nil {
			return trace.Wrap(err)
		}
		// Reset generation back to 1 for the new instance.
		authRecord.SetGeneration(1)

		bi := newBotInstance(machineidv1pb.BotInstanceSpec_builder{
			BotName:    req.BotName,
			InstanceId: newInstanceID.String(),
		}.Build(),
			authRecord,
			a.GetClock().Now().Add(req.TTL+machineidv1.ExpiryMargin),
		)
		bi.SetScope(req.BotScope)

		if _, err := a.BotInstance.CreateBotInstance(ctx, bi); err != nil {
			return trace.Wrap(err)
		}

		req.BotInstanceID = newInstanceID.String()
		req.Generation = 1

		return nil
	}
	if err != nil {
		return trace.Wrap(err)
	}

	// Fetch latest generation from bot instance authn record history.
	var instanceGeneration int32
	if auths := existingInstance.GetStatus().GetLatestAuthentications(); len(auths) > 0 {
		instanceGeneration = auths[len(auths)-1].GetGeneration()
	}

	log := a.logger.With(
		"bot_name", req.BotName,
		"bot_instance_id", req.BotInstanceID,
	)

	if currentIdentityGeneration != instanceGeneration {
		// Generation counter enforcement depends on the type of cert and join
		// method (if any - token renewals technically have no join method.)
		if shouldEnforceGenerationCounter(req.Renewable, joinMethod) {
			if err := a.tryLockBotDueToGenerationMismatch(
				ctx, req.BotName, req.BotInstanceID, req.JoinToken, req.Renewable,
			); err != nil {
				log.WarnContext(
					ctx,
					"Failed to lock bot when a generation mismatch was detected",
					"error", err,
				)
			}

			return trace.AccessDenied(
				"cert generation mismatch for bot %s/%s: stored=%v, presented=%v",
				req.BotName, req.BotInstanceID,
				instanceGeneration, currentIdentityGeneration,
			)
		}
		// We'll still log the check failure, but won't deny access. This
		// log data will help make an informed decision about reliability of
		// the generation counter for all join methods in the future.
		const msg = "Bot generation counter mismatch detected. This check is not enforced for this join method, " +
			"but may indicate multiple uses of a bot identity and possibly a compromised certificate."
		log.WarnContext(ctx, msg,
			"bot_instance_generation", instanceGeneration,
			"bot_identity_generation", currentIdentityGeneration,
			"bot_join_method", joinMethod,
		)
	}

	authRecord, err := authRecordForUpdate(
		req, joinMethod, a.GetClock().Now(), templateAuthRecord,
	)
	if err != nil {
		return trace.Wrap(err)
	}

	// Increment the generation counter the cert and bot instance. The counter
	// should be incremented and stored even if it is not validated above.
	newGeneration := instanceGeneration + 1
	authRecord.SetGeneration(newGeneration)
	req.Generation = uint64(newGeneration)

	_, err = a.BotInstance.PatchBotInstance(ctx, services.PatchBotInstanceOpts{
		Bot:        scopes.QualifiedName{Scope: req.BotScope, Name: req.BotName},
		InstanceID: req.BotInstanceID,
		UpdateFn: func(bi *machineidv1pb.BotInstance) (*machineidv1pb.BotInstance, error) {
			if !bi.HasStatus() {
				bi.SetStatus(&machineidv1pb.BotInstanceStatus{})
			}

			// Update the record's expiration timestamp based on the request TTL
			// plus an expiry margin.
			bi.GetMetadata().SetExpires(timestamppb.New(a.GetClock().Now().Add(req.TTL + machineidv1.ExpiryMargin)))

			// If we're at or above the limit, remove enough of the front elements
			// to make room for the new one at the end.
			if len(bi.GetStatus().GetLatestAuthentications()) >= machineidv1.AuthenticationHistoryLimit {
				toRemove := len(bi.GetStatus().GetLatestAuthentications()) - machineidv1.AuthenticationHistoryLimit + 1
				bi.GetStatus().SetLatestAuthentications(bi.GetStatus().GetLatestAuthentications()[toRemove:])
			}

			// An initial auth record should have been added during initial join,
			// but if not, add it now.
			if !bi.GetStatus().HasInitialAuthentication() {
				log.WarnContext(ctx, "bot instance is missing its initial authentication record, a new one will be added")
				bi.GetStatus().SetInitialAuthentication(authRecord)
			}

			bi.GetStatus().SetLatestAuthentications(append(bi.GetStatus().GetLatestAuthentications(), authRecord))

			return bi, nil
		},
	})

	return trace.Wrap(err)
}

// newBotInstance constructs a new bot instance from a spec and initial authentication
func newBotInstance(
	spec *machineidv1pb.BotInstanceSpec,
	initialAuth *machineidv1pb.BotInstanceStatusAuthentication,
	expires time.Time,
) *machineidv1pb.BotInstance {
	return machineidv1pb.BotInstance_builder{
		Kind:    types.KindBotInstance,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Expires: timestamppb.New(expires),
		}.Build(),
		Spec: spec,
		Status: machineidv1pb.BotInstanceStatus_builder{
			InitialAuthentication: initialAuth,
			LatestAuthentications: []*machineidv1pb.BotInstanceStatusAuthentication{initialAuth},
		}.Build(),
	}.Build()
}

// generateInitialBotCerts is used to generate bot certs and overlaps
// significantly with `generateUserCerts()`. However, it omits a number of
// options (impersonation, access requests, role requests, actual cert renewal,
// and most UserCertsRequest options that don't relate to bots) and does not
// care if the current identity is Nop.  This function does not validate the
// current identity at all; the caller is expected to validate that the client
// is allowed to issue the (possibly renewable) certificates.
//
// Returns a second argument of the bot instance ID for inclusion in audit logs.
func (a *Server) generateInitialBotCerts(
	ctx context.Context, botName, username, loginIP string,
	sshPubKey, tlsPubKey []byte,
	expires time.Time, renewable bool,
	initialAuth *machineidv1pb.BotInstanceStatusAuthentication,
	existingInstanceID string, previousInstanceID string, currentIdentityGeneration int32,
	joinAttrs *workloadidentityv1pb.JoinAttrs,
) (*proto.Certs, string, error) {
	var err error

	// Extract the user and role set for whom the certificate will be generated.
	// This should be safe since this is typically done against a local user.
	//
	// This call bypasses RBAC check for users read on purpose.
	// Users who are allowed to impersonate other users might not have
	// permissions to read user data.
	userState, err := a.GetUserOrLoginState(ctx, username)
	if err != nil {
		a.logger.DebugContext(ctx, "Could not impersonate user - the user could not be fetched from local store",
			"error", err,
			"user", username,
		)
		return nil, "", trace.AccessDenied("access denied")
	}

	// Do not allow SSO users to be impersonated.
	if userState.GetUserType() == types.UserTypeSSO {
		a.logger.WarnContext(ctx, "Tried to issue a renewable cert for externally managed user, this is not supported", "user", username)
		return nil, "", trace.AccessDenied("access denied")
	}

	// Cap the cert TTL to the MaxRenewableCertTTL.
	if max := a.GetClock().Now().Add(defaults.MaxRenewableCertTTL); expires.After(max) {
		expires = max
	}

	// Inherit the user's roles and traits verbatim.
	accessInfo := services.AccessInfoFromUserState(userState)

	botScope, _ := userState.GetLabel(types.BotScopeLabel)
	scopeAwareChecker, err := a.AccessCheckerForScope(
		ctx,
		botScope, // treated as unscoped when empty
		userState,
		[]types.ResourceAccessID{}, // bots do not support access requests
	)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	// Generate certificate
	certReq := cert.Request{
		User:           userState,
		TTL:            expires.Sub(a.GetClock().Now()),
		SSHPublicKey:   sshPubKey,
		TLSPublicKey:   tlsPubKey,
		CheckerContext: scopeAwareChecker,
		Traits:         accessInfo.Traits,
		Renewable:      renewable,
		IncludeHostCA:  true,
		LoginIP:        loginIP,
		BotName:        botName,
		BotScope:       botScope,
		BotInternal:    true,
		JoinAttributes: joinAttrs,
	}

	// Set the join token cert field for non-renewable identities. This is used
	// for lock targeting; token name lock targets are particularly useful for
	// token-joined bots and it's a secret value, so we don't bother setting it.
	// (The renewable flag implies token joining.)
	if !renewable {
		certReq.JoinToken = initialAuth.GetJoinToken()
	}

	if existingInstanceID == "" {
		// If no existing instance ID is known, create a new one.
		uuid, err := uuid.NewRandom()
		if err != nil {
			return nil, "", trace.Wrap(err)
		}

		initialAuth.SetGeneration(1)

		bi := newBotInstance(machineidv1pb.BotInstanceSpec_builder{
			BotName:            botName,
			InstanceId:         uuid.String(),
			PreviousInstanceId: previousInstanceID,
		}.Build(), initialAuth, expires.Add(machineidv1.ExpiryMargin))
		bi.SetScope(botScope)

		_, err = a.BotInstance.CreateBotInstance(ctx, bi)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}

		certReq.BotInstanceID = uuid.String()
		certReq.Generation = 1
	} else {
		// Otherwise, reuse the existing instance ID, and pass the
		// initialAuth along. `updateBotInstance()` replaces the ID on the
		// request if it mints a fresh instance.
		certReq.BotInstanceID = existingInstanceID

		if err := a.updateBotInstance(
			ctx, &certReq, initialAuth, currentIdentityGeneration,
		); err != nil {
			return nil, "", trace.Wrap(err)
		}
	}

	certs, err := a.GenerateUserCerts(ctx, certReq)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	return certs, certReq.BotInstanceID, nil
}
