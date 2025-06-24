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
	"context"
	"fmt"
	"strconv"
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
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/machineid/machineidv1"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// legacyValidateGenerationLabel validates and updates a generation label.
// TODO(timothyb89): This is deprecated in favor of bot instance generation
// counters. Upgrade/downgrade compatibility will be provided through v17.
// TODO(timothyb89): In v18, we should explicitly remove generation counters
// labels from the bot user.
// REMOVE IN V18: Use bot instance generation counters instead.
func (a *Server) legacyValidateGenerationLabel(ctx context.Context, username string, certReq *certRequest, currentIdentityGeneration uint64) error {
	// Fetch the user, bypassing the cache. We might otherwise fetch a stale
	// value in case of a rapid certificate renewal.
	user, err := a.Services.GetUser(ctx, username, false)
	if err != nil {
		return trace.Wrap(err)
	}

	var currentUserGeneration uint64
	label := user.BotGenerationLabel()
	if label != "" {
		currentUserGeneration, err = strconv.ParseUint(label, 10, 64)
		if err != nil {
			return trace.BadParameter("user has invalid value for label %q", types.BotGenerationLabel)
		}
	}

	// If there is no existing generation on any of the user, identity, or
	// cert request, we have nothing to do here.
	if currentUserGeneration == 0 && currentIdentityGeneration == 0 && certReq.generation == 0 {
		return nil
	}

	// By now, we know a generation counter is in play _somewhere_ and this is a
	// bot certs. Bot certs should include the host CA so that they can make
	// Teleport API calls.
	certReq.includeHostCA = true

	// If the certReq already has generation set, it was explicitly requested
	// (presumably this is the initial set of renewable certs). We'll want to
	// commit that value to the User object.
	if certReq.generation > 0 {
		// ...however, if the user already has a stored generation, bail.
		// (bots should be deleted and recreated if their certs expire)
		if currentUserGeneration > 0 {
			return trace.BadParameter(
				"user %q has already been issued a renewable certificate and cannot be issued another; consider deleting and recreating the bot",
				user.GetName(),
			)
		}

		// Sanity check that the requested generation is 1.
		if certReq.generation != 1 {
			return trace.BadParameter("explicitly requested generation %d is not equal to 1, this is a logic error", certReq.generation)
		}

		userV2, ok := user.(*types.UserV2)
		if !ok {
			return trace.BadParameter("unsupported version of user: %T", user)
		}
		newUser := apiutils.CloneProtoMsg(userV2)
		metadata := newUser.GetMetadata()
		generation := fmt.Sprint(certReq.generation)
		metadata.Labels[types.BotGenerationLabel] = generation
		newUser.SetMetadata(metadata)

		// Note: we bypass the RBAC check on purpose as bot users should not
		// have user update permissions.
		if err := a.CompareAndSwapUser(ctx, newUser, user); err != nil {
			// If this fails it's likely to be some miscellaneous competing
			// write. The request should be tried again - if it's malicious,
			// someone will get a generation mismatch and trigger a lock.
			return trace.CompareFailed("Database comparison failed, try the request again")
		}

		return nil
	}

	// The current generations must match to continue:
	if currentIdentityGeneration != currentUserGeneration {
		if err := a.tryLockBotDueToGenerationMismatch(ctx, user.GetName()); err != nil {
			a.logger.WarnContext(ctx, "Failed to lock bot when a generation mismatch was detected",
				"error", err,
				"bot", user.GetName(),
			)
		}

		return trace.AccessDenied(
			"renewable cert generation mismatch: stored=%v, presented=%v",
			currentUserGeneration, currentIdentityGeneration,
		)
	}

	// Update the user with the new generation count.
	newGeneration := currentIdentityGeneration + 1

	// As above, commit some crimes to clone the User.
	newUser, err := a.Services.GetUser(ctx, user.GetName(), false)
	if err != nil {
		return trace.Wrap(err)
	}
	metadata := newUser.GetMetadata()
	metadata.Labels[types.BotGenerationLabel] = fmt.Sprint(newGeneration)
	newUser.SetMetadata(metadata)

	if err := a.CompareAndSwapUser(ctx, newUser, user); err != nil {
		// If this fails it's likely to be some miscellaneous competing
		// write. The request should be tried again - if it's malicious,
		// someone will get a generation mismatch and trigger a lock.
		return trace.CompareFailed("Database comparison failed, try the request again")
	}

	// And lastly, set the generation on the cert request.
	certReq.generation = newGeneration

	return nil
}

func sshPublicKeyToPKIXPEM(pubKey []byte) ([]byte, error) {
	cryptoPubKey, err := sshutils.CryptoPublicKey(pubKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return keys.MarshalPublicKey(cryptoPubKey)
}

// commitGenerationCounterToBotUser updates the legacy generation counter label
// for a user, but only when the counter is greater than the previous value. If
// multiple bot instances exist for a given bot user, only the largest counter
// value is persisted. This ensures that, if a cluster is downgraded, exactly
// one bot instance (with the largest generation value) will be able to
// reauthenticate.
func (a *Server) commitLegacyGenerationCounterToBotUser(ctx context.Context, username string, newValue uint64) error {
	var err error

	for range 3 {
		user, err := a.Services.GetUser(ctx, username, false)
		if err != nil {
			return trace.Wrap(err)
		}

		var currentUserGeneration uint64
		label := user.BotGenerationLabel()
		if label != "" {
			currentUserGeneration, err = strconv.ParseUint(label, 10, 64)
			if err != nil {
				return trace.BadParameter("user has invalid value for label %q", types.BotGenerationLabel)
			}
		}

		if newValue <= currentUserGeneration {
			// Nothing to do, value is up to date
			return nil
		}

		// Clone the user and update the generation label.
		userV2, ok := user.(*types.UserV2)
		if !ok {
			return trace.BadParameter("unsupported version of user: %T", user)
		}
		newUser := apiutils.CloneProtoMsg(userV2)
		metadata := newUser.GetMetadata()
		metadata.Labels[types.BotGenerationLabel] = fmt.Sprint(newValue)
		newUser.SetMetadata(metadata)

		// Attempt to commit the change. If it fails due to a comparison
		// failure, try again.
		err = a.CompareAndSwapUser(ctx, newUser, user)
		if err == nil {
			return nil
		} else if trace.IsCompareFailed(err) {
			continue
		} else {
			return trace.Wrap(err)
		}
	}

	return trace.Wrap(err)
}

// tryLockBotDueToGenerationMismatch creates a lock for the given bot user and
// emits a `RenewableCertificateGenerationMismatch` audit event.
func (a *Server) tryLockBotDueToGenerationMismatch(ctx context.Context, username string) error {
	// TODO: In the future, consider only locking the current join method / token.

	// Lock the bot user indefinitely.
	lock, err := types.NewLock(uuid.New().String(), types.LockSpecV2{
		Target: types.LockTarget{
			User: username,
		},
		Message: fmt.Sprintf(
			"The bot user %q has been locked due to a certificate generation mismatch, possibly indicating a stolen certificate.",
			username,
		),
	})
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

// updateBotInstance updates the bot instance associated with the context
// identity, if any. If the optional `templateAuthRecord` is provided, various
// metadata fields will be copied into the newly generated auth record.
func (a *Server) updateBotInstance(
	ctx context.Context, req *certRequest,
	username, botName, botInstanceID string,
	templateAuthRecord *machineidv1pb.BotInstanceStatusAuthentication,
	currentIdentityGeneration int32,
) error {
	if botName == "" {
		// Only applies to bot identities
		return nil
	}

	// Check if this bot instance actually exists.
	var instanceGeneration int32
	instanceNotFound := false
	if botInstanceID != "" {
		existingInstance, err := a.BotInstance.GetBotInstance(ctx, botName, botInstanceID)
		if trace.IsNotFound(err) {
			instanceNotFound = true
		} else if err != nil {
			// Some other error, bail.
			return trace.Wrap(err)
		} else {
			// We have an existing instance, so fetch its generation.
			auths := existingInstance.Status.LatestAuthentications
			if len(auths) > 0 {
				latest := auths[len(auths)-1]
				instanceGeneration = latest.Generation
			}
		}
	}

	var publicKeyPEM []byte
	if req.tlsPublicKey != nil {
		publicKeyPEM = req.tlsPublicKey
	} else {
		// At least one of tlsPublicKey or sshPublicKey will be set, this is validated by [req.check].
		var err error
		publicKeyPEM, err = sshPublicKeyToPKIXPEM(req.sshPublicKey)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	authRecord := &machineidv1pb.BotInstanceStatusAuthentication{
		AuthenticatedAt: timestamppb.New(a.GetClock().Now()),
		PublicKey:       publicKeyPEM,

		// Note: This is presumed to be a token join. If not, a
		// `templateAuthRecord` should be provided to override this value.
		JoinMethod: string(types.JoinMethodToken),
	}

	if templateAuthRecord != nil {
		authRecord.JoinToken = templateAuthRecord.JoinToken
		authRecord.JoinMethod = templateAuthRecord.JoinMethod
		authRecord.JoinAttrs = templateAuthRecord.JoinAttrs
	}

	// An empty bot instance most likely means a bot is rejoining after an
	// upgrade, so a new bot instance should be generated. We may consider
	// making this an error in the future.
	if botInstanceID == "" || instanceNotFound {
		instanceID, err := uuid.NewRandom()
		if err != nil {
			return trace.Wrap(err)
		}

		// TODO(timothyb89): consider making this the only path where bot
		// instances are created. We could call `updateBotInstance()`
		// unconditionally from `generateInitialBotCerts()` and have only one
		// codepath. (But may need to clean up log messages.)

		// Set the initial generation counter. Note that with bot instances, the
		// counter is now set for all join methods, but only enforced for token
		// joins.
		if currentIdentityGeneration > 0 {
			// If the incoming identity has a nonzero generation, validate it
			// using the legacy check. This will increment the counter on the
			// request automatically
			if err := a.legacyValidateGenerationLabel(ctx, username, req, uint64(currentIdentityGeneration)); err != nil {
				return trace.Wrap(err)
			}

			// Copy the value from the request into the auth record.
			authRecord.Generation = int32(req.generation)
		} else {
			// Otherwise, just set it to 1.
			req.generation = 1
			authRecord.Generation = 1
		}

		a.logger.InfoContext(ctx, "bot has no valid instance ID, a new instance will be generated",
			"bot_name", botName,
			"invalid_instance_id", botInstanceID,
			"new_instance_id", logutils.StringerAttr(instanceID),
		)

		expires := a.GetClock().Now().Add(req.ttl + machineidv1.ExpiryMargin)

		bi := newBotInstance(&machineidv1pb.BotInstanceSpec{
			BotName:    botName,
			InstanceId: instanceID.String(),
		}, authRecord, expires)

		if _, err := a.BotInstance.CreateBotInstance(ctx, bi); err != nil {
			return trace.Wrap(err)
		}

		// Add the new ID to the cert request
		req.botInstanceID = instanceID.String()

		return nil
	}

	log := a.logger.With(
		"bot_name", botName,
		"bot_instance_id", botInstanceID,
	)

	if currentIdentityGeneration == 0 {
		// Nothing to do.
		log.WarnContext(ctx, "bot attempted to fetch certificates without providing a current identity generation, this is not allowed")

		return trace.AccessDenied("a current identity generation must be provided")
	} else if currentIdentityGeneration > 0 && currentIdentityGeneration != instanceGeneration {
		// Generation counter enforcement depends on the type of cert and join
		// method (if any - token renewals technically have no join method.)
		if shouldEnforceGenerationCounter(req.renewable, authRecord.JoinMethod) {
			if err := a.tryLockBotDueToGenerationMismatch(ctx, username); err != nil {
				log.WarnContext(ctx, "Failed to lock bot when a generation mismatch was detected", "error", err)
			}

			return trace.AccessDenied(
				"renewable cert generation mismatch for bot %s/%s: stored=%v, presented=%v",
				botName, botInstanceID,
				instanceGeneration, currentIdentityGeneration,
			)
		} else {
			// We'll still log the check failure, but won't deny access. This
			// log data will help make an informed decision about reliability of
			// the generation counter for all join methods in the future.
			const msg = "Bot generation counter mismatch detected. This check is not enforced for this join method, " +
				"but may indicate multiple uses of a bot identity and possibly a compromised certificate."
			log.WarnContext(ctx, msg,
				"bot_instance_generation", instanceGeneration,
				"bot_identity_generation", currentIdentityGeneration,
				"bot_join_method", authRecord.JoinMethod,
			)
		}
	}

	// Increment the generation counter the cert and bot instance. The counter
	// should be incremented and stored even if it is not validated above.
	newGeneration := instanceGeneration + 1
	authRecord.Generation = newGeneration
	req.generation = uint64(newGeneration)

	// Commit the generation counter to the bot user for downgrade
	// compatibility, but only if this is a renewable identity. Previous
	// versions only expect a nonzero generation counter for token joins, so
	// setting this for other methods will break compatibility.
	// Note: new join methods that enforce generation counter checks will not
	// write a generation counter to user labels (e.g. bound keypair).
	if req.renewable {
		if err := a.commitLegacyGenerationCounterToBotUser(ctx, username, uint64(newGeneration)); err != nil {
			log.WarnContext(ctx, "unable to commit legacy generation counter to bot user", "error", err)
		}
	}

	_, err := a.BotInstance.PatchBotInstance(ctx, botName, botInstanceID, func(bi *machineidv1pb.BotInstance) (*machineidv1pb.BotInstance, error) {
		if bi.Status == nil {
			bi.Status = &machineidv1pb.BotInstanceStatus{}
		}

		// Update the record's expiration timestamp based on the request TTL
		// plus an expiry margin.
		bi.Metadata.Expires = timestamppb.New(a.GetClock().Now().Add(req.ttl + machineidv1.ExpiryMargin))

		// If we're at or above the limit, remove enough of the front elements
		// to make room for the new one at the end.
		if len(bi.Status.LatestAuthentications) >= machineidv1.AuthenticationHistoryLimit {
			toRemove := len(bi.Status.LatestAuthentications) - machineidv1.AuthenticationHistoryLimit + 1
			bi.Status.LatestAuthentications = bi.Status.LatestAuthentications[toRemove:]
		}

		// An initial auth record should have been added during initial join,
		// but if not, add it now.
		if bi.Status.InitialAuthentication == nil {
			log.WarnContext(ctx, "bot instance is missing its initial authentication record, a new one will be added")
			bi.Status.InitialAuthentication = authRecord
		}

		bi.Status.LatestAuthentications = append(bi.Status.LatestAuthentications, authRecord)

		return bi, nil
	})

	return trace.Wrap(err)
}

// newBotInstance constructs a new bot instance from a spec and initial authentication
func newBotInstance(
	spec *machineidv1pb.BotInstanceSpec,
	initialAuth *machineidv1pb.BotInstanceStatusAuthentication,
	expires time.Time,
) *machineidv1pb.BotInstance {
	return &machineidv1pb.BotInstance{
		Kind:    types.KindBotInstance,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Expires: timestamppb.New(expires),
		},
		Spec: spec,
		Status: &machineidv1pb.BotInstanceStatus{
			InitialAuthentication: initialAuth,
			LatestAuthentications: []*machineidv1pb.BotInstanceStatusAuthentication{initialAuth},
		},
	}
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
	clusterName, err := a.GetClusterName(ctx)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	checker, err := services.NewAccessChecker(accessInfo, clusterName.GetClusterName(), a)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	// Generate certificate
	certReq := certRequest{
		user:           userState,
		ttl:            expires.Sub(a.GetClock().Now()),
		sshPublicKey:   sshPubKey,
		tlsPublicKey:   tlsPubKey,
		checker:        checker,
		traits:         accessInfo.Traits,
		renewable:      renewable,
		includeHostCA:  true,
		loginIP:        loginIP,
		botName:        botName,
		joinAttributes: joinAttrs,
	}

	if existingInstanceID == "" {
		// If no existing instance ID is known, create a new one.
		uuid, err := uuid.NewRandom()
		if err != nil {
			return nil, "", trace.Wrap(err)
		}

		initialAuth.Generation = 1

		bi := newBotInstance(&machineidv1pb.BotInstanceSpec{
			BotName:            botName,
			InstanceId:         uuid.String(),
			PreviousInstanceId: previousInstanceID,
		}, initialAuth, expires.Add(machineidv1.ExpiryMargin))

		_, err = a.BotInstance.CreateBotInstance(ctx, bi)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}

		certReq.botInstanceID = uuid.String()
		certReq.generation = 1
	} else {
		// Otherwise, reuse the existing instance ID, and pass the
		// initialAuth along.

		// Note: botName is derived from the provision token rather than any
		// value sent by the client, so we can trust it.
		if err := a.updateBotInstance(
			ctx, &certReq, username, botName, existingInstanceID,
			initialAuth, currentIdentityGeneration,
		); err != nil {
			return nil, "", trace.Wrap(err)
		}

		// Only set the bot instance ID if it's empty; `updateBotInstance()`
		// may set it if a new instance is created.
		if certReq.botInstanceID == "" {
			certReq.botInstanceID = existingInstanceID
		}
	}

	certs, err := a.generateUserCert(ctx, certReq)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	return certs, certReq.botInstanceID, nil
}
