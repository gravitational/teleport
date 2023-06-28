/*
Copyright 2021 Gravitational, Inc.

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

package auth

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/wrappers"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
	"github.com/gravitational/teleport/lib/utils"
)

// BotResourceName returns the default name for resources associated with the
// given named bot.
func BotResourceName(botName string) string {
	return "bot-" + strings.ReplaceAll(botName, " ", "-")
}

// createBotRole creates a role from a bot template with the given parameters.
func createBotRole(ctx context.Context, s *Server, botName string, resourceName string, roleRequests []string) (types.Role, error) {
	role, err := types.NewRole(resourceName, types.RoleSpecV6{
		Options: types.RoleOptions{
			// TODO: inherit TTLs from cert length?
			MaxSessionTTL: types.Duration(12 * time.Hour),
		},
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				// Bots read certificate authorities to watch for CA rotations
				types.NewRule(types.KindCertAuthority, []string{types.VerbReadNoSecrets}),
			},
			Impersonate: &types.ImpersonateConditions{
				Roles: roleRequests,
			},
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	meta := role.GetMetadata()
	meta.Description = fmt.Sprintf("Automatically generated role for bot %s", botName)
	if meta.Labels == nil {
		meta.Labels = map[string]string{}
	}
	meta.Labels[types.BotLabel] = botName
	role.SetMetadata(meta)

	err = s.CreateRole(ctx, role)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return role, nil
}

// createBotUser creates a new backing User for bot use. A role with a
// matching name must already exist (see createBotRole).
func createBotUser(
	ctx context.Context,
	s *Server,
	botName string,
	resourceName string,
	traits wrappers.Traits,
) (types.User, error) {
	user, err := types.NewUser(resourceName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	user.SetRoles([]string{resourceName})

	metadata := user.GetMetadata()
	metadata.Labels = map[string]string{
		types.BotLabel:           botName,
		types.BotGenerationLabel: "0",
	}
	user.SetMetadata(metadata)
	user.SetTraits(traits)

	if err := s.CreateUser(ctx, user); err != nil {
		return nil, trace.Wrap(err)
	}

	return user, nil
}

// createBot creates a new certificate renewal bot from a bot request.
func (s *Server) createBot(ctx context.Context, req *proto.CreateBotRequest) (*proto.CreateBotResponse, error) {
	if req.Name == "" {
		return nil, trace.BadParameter("bot name must not be empty")
	}

	resourceName := BotResourceName(req.Name)

	// Ensure conflicting resources don't already exist.
	// We skip the cache here to allow for bot recreation shortly after bot
	// deletion.
	_, err := s.Services.GetRole(ctx, resourceName)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if roleExists := (err == nil); roleExists {
		return nil, trace.AlreadyExists("cannot add bot: role %q already exists", resourceName)
	}
	_, err = s.Services.GetUser(resourceName, false)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if userExists := (err == nil); userExists {
		return nil, trace.AlreadyExists("cannot add bot: user %q already exists", resourceName)
	}

	// Ensure at least one role was requested.
	if len(req.Roles) == 0 {
		return nil, trace.BadParameter("cannot add bot: at least one role is required")
	}

	// Ensure all requested roles exist.
	for _, roleName := range req.Roles {
		_, err := s.GetRole(ctx, roleName)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	provisionToken, err := s.checkOrCreateBotToken(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create the resources.
	if _, err := createBotRole(ctx, s, req.Name, resourceName, req.Roles); err != nil {
		return nil, trace.Wrap(err)
	}

	if _, err := createBotUser(ctx, s, req.Name, resourceName, req.Traits); err != nil {
		return nil, trace.Wrap(err)
	}

	tokenTTL := time.Duration(0)
	if exp := provisionToken.Expiry(); !exp.IsZero() {
		tokenTTL = time.Until(exp)
	}

	// Emit usage analytics event for bot creation.
	s.AnonymizeAndSubmit(&usagereporter.BotCreateEvent{
		UserName:    authz.ClientUsername(ctx),
		BotUserName: resourceName,
		RoleName:    resourceName,
		BotName:     req.Name,
		RoleCount:   int64(len(req.Roles)),
		JoinMethod:  string(provisionToken.GetJoinMethod()),
	})

	return &proto.CreateBotResponse{
		TokenID:    provisionToken.GetName(),
		UserName:   resourceName,
		RoleName:   resourceName,
		TokenTTL:   proto.Duration(tokenTTL),
		JoinMethod: provisionToken.GetJoinMethod(),
	}, nil
}

// deleteBotUser removes an existing bot user, ensuring that it has bot labels
// matching the bot before deleting anything.
func (s *Server) deleteBotUser(ctx context.Context, botName, resourceName string) error {
	user, err := s.GetUser(resourceName, false)
	if err != nil {
		return trace.Wrap(err, "could not fetch expected bot user %s", resourceName)
	}

	label, ok := user.GetMetadata().Labels[types.BotLabel]
	if !ok {
		err = trace.Errorf("will not delete user %s that is missing label %s; delete the user manually if desired", resourceName, types.BotLabel)
	} else if label != botName {
		err = trace.Errorf("will not delete user %s with mismatched label %s = %s", resourceName, types.BotLabel, label)
	} else {
		err = s.DeleteUser(ctx, resourceName)
	}

	return err
}

// deleteBotRole removes an existing bot role, ensuring that it has bot labels
// matching the bot before deleting anything.
func (s *Server) deleteBotRole(ctx context.Context, botName, resourceName string) error {
	role, err := s.GetRole(ctx, resourceName)
	if err != nil {
		return trace.Wrap(err, "could not fetch expected bot role %s", resourceName)
	}

	label, ok := role.GetMetadata().Labels[types.BotLabel]
	if !ok {
		err = trace.Errorf("will not delete role %s that is missing label %s; delete the role manually if desired", resourceName, types.BotLabel)
	} else if label != botName {
		err = trace.Errorf("will not delete role %s with mismatched label %s = %s", resourceName, types.BotLabel, label)
	} else {
		err = s.DeleteRole(ctx, resourceName)
	}

	return err
}

func (s *Server) deleteBot(ctx context.Context, botName string) error {
	// Note: this does not remove any locks for the bot's user / role. That
	// might be convenient in case of accidental bot locking but there doesn't
	// seem to be any automatic deletion of locks in teleport today (other
	// than expiration). Consistency around security controls seems important
	// but we can revisit this if desired.
	resourceName := BotResourceName(botName)

	userErr := s.deleteBotUser(ctx, botName, resourceName)
	roleErr := s.deleteBotRole(ctx, botName, resourceName)
	return trace.NewAggregate(userErr, roleErr)
}

// getBotUsers fetches all Users with the BotLabel field set. Users are fetched
// without secrets.
func (s *Server) getBotUsers(ctx context.Context) ([]types.User, error) {
	var botUsers []types.User

	users, err := s.GetUsers(false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, user := range users {
		if _, ok := user.GetMetadata().Labels[types.BotLabel]; ok {
			botUsers = append(botUsers, user)
		}
	}

	return botUsers, nil
}

// checkOrCreateBotToken checks the existing token if given, or creates a new
// random dynamic provision token which allows bots to join with the given
// botName. Returns the token and any error.
func (s *Server) checkOrCreateBotToken(ctx context.Context, req *proto.CreateBotRequest) (types.ProvisionToken, error) {
	botName := req.Name

	// if the request includes a TokenID it should already exist
	if req.TokenID != "" {
		provisionToken, err := s.GetToken(ctx, req.TokenID)
		if err != nil {
			if trace.IsNotFound(err) {
				return nil, trace.NotFound("token with name %q not found, create the token or do not set TokenName: %v",
					req.TokenID, err)
			}
			return nil, trace.Wrap(err)
		}
		if !provisionToken.GetRoles().Include(types.RoleBot) {
			return nil, trace.BadParameter("token %q is not valid for role %q",
				req.TokenID, types.RoleBot)
		}
		if provisionToken.GetBotName() != botName {
			return nil, trace.BadParameter("token %q is valid for bot with name %q, not %q",
				req.TokenID, provisionToken.GetBotName(), botName)
		}
		switch provisionToken.GetJoinMethod() {
		case types.JoinMethodToken,
			types.JoinMethodIAM,
			types.JoinMethodGitHub,
			types.JoinMethodGitLab,
			types.JoinMethodAzure,
			types.JoinMethodCircleCI:
		default:
			return nil, trace.BadParameter(
				"token %q has join method %q which is not supported for bots. Supported join methods are %v",
				req.TokenID, provisionToken.GetJoinMethod(), []types.JoinMethod{
					types.JoinMethodToken,
					types.JoinMethodIAM,
					types.JoinMethodGitHub,
					types.JoinMethodGitLab,
					types.JoinMethodAzure,
					types.JoinMethodCircleCI,
				})
		}
		return provisionToken, nil
	}

	// create a new random dynamic token
	tokenName, err := utils.CryptoRandomHex(TokenLenBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ttl := time.Duration(req.TTL)
	if ttl == 0 {
		ttl = defaults.DefaultBotJoinTTL
	}

	tokenSpec := types.ProvisionTokenSpecV2{
		Roles:      types.SystemRoles{types.RoleBot},
		JoinMethod: types.JoinMethodToken,
		BotName:    botName,
	}
	token, err := types.NewProvisionTokenFromSpec(tokenName, s.clock.Now().Add(ttl), tokenSpec)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.UpsertToken(ctx, token); err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO: audit log event

	return token, nil
}

// validateGenerationLabel validates and updates a generation label.
func (s *Server) validateGenerationLabel(ctx context.Context, user types.User, certReq *certRequest, currentIdentityGeneration uint64) error {
	// Fetch the user, bypassing the cache. We might otherwise fetch a stale
	// value in case of a rapid certificate renewal.
	user, err := s.Services.GetUser(user.GetName(), false)
	if err != nil {
		return trace.Wrap(err)
	}

	var currentUserGeneration uint64
	label, labelOk := user.GetMetadata().Labels[types.BotGenerationLabel]
	if labelOk {
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
		metadata.Labels[types.BotGenerationLabel] = fmt.Sprint(certReq.generation)
		newUser.SetMetadata(metadata)

		// Note: we bypass the RBAC check on purpose as bot users should not
		// have user update permissions.
		if err := s.CompareAndSwapUser(ctx, newUser, user); err != nil {
			// If this fails it's likely to be some miscellaneous competing
			// write. The request should be tried again - if it's malicious,
			// someone will get a generation mismatch and trigger a lock.
			return trace.CompareFailed("Database comparison failed, try the request again")
		}

		return nil
	}

	// The current generations must match to continue:
	if currentIdentityGeneration != currentUserGeneration {
		// Lock the bot user indefinitely.
		lock, err := types.NewLock(uuid.New().String(), types.LockSpecV2{
			Target: types.LockTarget{
				User: user.GetName(),
			},
			Message: fmt.Sprintf(
				"The bot user %q has been locked due to a certificate generation mismatch, possibly indicating a stolen certificate.",
				user.GetName(),
			),
		})
		if err != nil {
			return trace.Wrap(err)
		}
		if err := s.UpsertLock(ctx, lock); err != nil {
			return trace.Wrap(err)
		}

		// Emit an audit event.
		userMetadata := authz.ClientUserMetadata(ctx)
		if err := s.emitter.EmitAuditEvent(s.closeCtx, &apievents.RenewableCertificateGenerationMismatch{
			Metadata: apievents.Metadata{
				Type: events.RenewableCertificateGenerationMismatchEvent,
				Code: events.RenewableCertificateGenerationMismatchCode,
			},
			UserMetadata: userMetadata,
		}); err != nil {
			log.WithError(err).Warn("Failed to emit renewable cert generation mismatch event")
		}

		return trace.AccessDenied(
			"renewable cert generation mismatch: stored=%v, presented=%v",
			currentUserGeneration, currentIdentityGeneration,
		)
	}

	// Update the user with the new generation count.
	newGeneration := currentIdentityGeneration + 1

	// As above, commit some crimes to clone the User.
	newUser, err := s.Services.GetUser(user.GetName(), false)
	if err != nil {
		return trace.Wrap(err)
	}
	metadata := newUser.GetMetadata()
	metadata.Labels[types.BotGenerationLabel] = fmt.Sprint(newGeneration)
	newUser.SetMetadata(metadata)

	if err := s.CompareAndSwapUser(ctx, newUser, user); err != nil {
		// If this fails it's likely to be some miscellaneous competing
		// write. The request should be tried again - if it's malicious,
		// someone will get a generation mismatch and trigger a lock.
		return trace.CompareFailed("Database comparison failed, try the request again")
	}

	// And lastly, set the generation on the cert request.
	certReq.generation = newGeneration

	return nil
}

// generateInitialBotCerts is used to generate bot certs and overlaps
// significantly with `generateUserCerts()`. However, it omits a number of
// options (impersonation, access requests, role requests, actual cert renewal,
// and most UserCertsRequest options that don't relate to bots) and does not
// care if the current identity is Nop.  This function does not validate the
// current identity at all; the caller is expected to validate that the client
// is allowed to issue the (possibly renewable) certificates.
func (s *Server) generateInitialBotCerts(ctx context.Context, username string, pubKey []byte, expires time.Time, renewable bool) (*proto.Certs, error) {
	var err error

	// Extract the user and role set for whom the certificate will be generated.
	// This should be safe since this is typically done against a local user.
	//
	// This call bypasses RBAC check for users read on purpose.
	// Users who are allowed to impersonate other users might not have
	// permissions to read user data.
	user, err := s.GetUser(username, false)
	if err != nil {
		log.WithError(err).Debugf("Could not impersonate user %v. The user could not be fetched from local store.", username)
		return nil, trace.AccessDenied("access denied")
	}

	// Do not allow SSO users to be impersonated.
	if user.GetUserType() == types.UserTypeSSO {
		log.Warningf("Tried to issue a renewable cert for externally managed user %v, this is not supported.", username)
		return nil, trace.AccessDenied("access denied")
	}

	// Cap the cert TTL to the MaxRenewableCertTTL.
	if max := s.GetClock().Now().Add(defaults.MaxRenewableCertTTL); expires.After(max) {
		expires = max
	}

	// Inherit the user's roles and traits verbatim.
	accessInfo := services.AccessInfoFromUser(user)
	clusterName, err := s.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	checker, err := services.NewAccessChecker(accessInfo, clusterName.GetClusterName(), s)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// renewable cert request must include a generation
	var generation uint64
	if renewable {
		generation = 1
	}

	// Generate certificate
	certReq := certRequest{
		user:          user,
		ttl:           expires.Sub(s.GetClock().Now()),
		publicKey:     pubKey,
		checker:       checker,
		traits:        accessInfo.Traits,
		renewable:     renewable,
		includeHostCA: true,
		generation:    generation,
	}

	if err := s.validateGenerationLabel(ctx, user, &certReq, 0); err != nil {
		return nil, trace.Wrap(err)
	}

	certs, err := s.generateUserCert(certReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return certs, nil
}
