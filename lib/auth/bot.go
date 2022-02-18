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

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

// BotResourceName returns the default name for resources associated with the
// given named bot.
func BotResourceName(botName string) string {
	return "bot-" + strings.ReplaceAll(botName, " ", "-")
}

// createBotRole creates a role from a bot template with the given parameters.
func createBotRole(ctx context.Context, s *Server, botName string, resourceName string, roleRequests []string) (types.Role, error) {
	role, err := types.NewRole(resourceName, types.RoleSpecV5{
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

	err = s.UpsertRole(ctx, role)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return role, nil
}

// createBotUser creates a new backing User for bot use. A role with a
// matching name must already exist (see createBotRole).
func createBotUser(ctx context.Context, s *Server, botName string, resourceName string) (types.User, error) {
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

	// Traits need to be set to silence "failed to find roles or traits" warning
	user.SetTraits(map[string][]string{
		teleport.TraitLogins:     {},
		teleport.TraitKubeUsers:  {},
		teleport.TraitKubeGroups: {},
	})

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
	_, err := s.GetRole(ctx, resourceName)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if roleExists := (err == nil); roleExists {
		return nil, trace.AlreadyExists("cannot add bot: role %q already exists", resourceName)
	}

	_, err = s.GetUser(resourceName, false)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if userExists := (err == nil); userExists {
		return nil, trace.AlreadyExists("cannot add bot: user %q already exists", resourceName)
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

	if _, err := createBotUser(ctx, s, req.Name, resourceName); err != nil {
		return nil, trace.Wrap(err)
	}

	return &proto.CreateBotResponse{
		TokenID:    provisionToken.GetName(),
		UserName:   resourceName,
		RoleName:   resourceName,
		TokenTTL:   proto.Duration(provisionToken.GetMetadata().Expires.Sub(time.Now())),
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
// botName. Returns the token name.
func (s *Server) checkOrCreateBotToken(ctx context.Context, req *proto.CreateBotRequest) (types.ProvisionToken, error) {
	resourceName := BotResourceName(req.Name)

	// if the request includes a TokenID it should already exist
	if req.TokenID != "" {
		provisionToken, err := s.GetToken(ctx, req.TokenID)
		if err != nil {
			if trace.IsNotFound(err) {
				return nil, trace.NotFound("token with name %q not found, create the token or do not set TokenName: %v",
					req.TokenID, err)
			} else {
				return nil, trace.Wrap(err)
			}
		}
		if !provisionToken.GetRoles().Include(types.RoleBot) {
			return nil, trace.BadParameter("token %q is not valid for role %q",
				req.TokenID, types.RoleBot)
		}
		if provisionToken.GetBotName() != resourceName {
			return nil, trace.BadParameter("token %q is valid for bot with name %q, not %q",
				req.TokenID, provisionToken.GetBotName(), resourceName)
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
		Roles:      []types.SystemRole{types.RoleBot},
		JoinMethod: types.JoinMethodToken,
		BotName:    resourceName,
	}
	token, err := types.NewProvisionTokenFromSpec(tokenName, time.Now().Add(ttl), tokenSpec)
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
	user, err := s.Identity.GetUser(user.GetName(), false)
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

	// The requested generation should only be explicitly set when a token
	// is being exchanged for an initial set of user certs, otherwise the
	// requested generation will be automatically set to
	// (currentIdentityGeneration + 1) below.
	if certReq.generation > 0 {
		// If the user already has a generation set, it would indicate that the
		// token is being reused.
		//
		// Regular/secret tokens will be deleted before returning signed user
		// certs and cannot be reused, but bot tokens which make use of the EC2
		// or IAM join methods are expected to be used repeatedly to fetch certs
		// with generation 1, rather than renewing the original certs in a
		// chain of increasing generations.
		//
		// Sanity check that the requested generation is equal to 1 and the
		// current user generation is 0 (unset) or 1 (if an EC2 or IAM token is
		// being reused).
		if certReq.generation != 1 || currentUserGeneration > 1 {
			return trace.BadParameter(
				"user %q has already been issued a renewable certificate and cannot be issued another; consider deleting and recreating the bot",
				user.GetName(),
			)
		}

		if currentUserGeneration == 1 {
			// This is the case where an EC2 or IAM join token is being reused
			// and the user already has the correct generation set.
			return nil
		}

		// Fetch a fresh copy of the user we can mutate safely. We can't
		// implement a protobuf clone on User due to protobuf's proto.Clone()
		// panicing when the user object has traits set, and a JSON
		// marshal/unmarshal creates an import cycle so... here we are.
		// There's a tiny chance the underlying user is mutated between calls
		// to GetUser() but we're comparing with an older value so it'll fail
		// safely.
		newUser, err := s.Identity.GetUser(user.GetName(), false)
		if err != nil {
			return trace.Wrap(err)
		}
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
		userMetadata := ClientUserMetadata(ctx)
		if s.emitter.EmitAuditEvent(s.closeCtx, &apievents.RenewableCertificateGenerationMismatch{
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
	newUser, err := s.Identity.GetUser(user.GetName(), false)
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

// generateInitialRenewableUserCerts is used to generate renewable bot certs
// and overlaps significantly with `generateUserCerts()`. However, it omits a
// number of options (impersonation, access requests, role requests, actual
// cert renewal, and most UserCertsRequest options that don't relate to bots)
// and does not care if the current identity is Nop.
// This function does not validate the current identity at all; the caller is
// expected to validate that the client is allowed to issue the renewable
// certificates.
func (s *Server) generateInitialRenewableUserCerts(ctx context.Context, username string, pubKey []byte, expires time.Time) (*proto.Certs, error) {
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
	if user.GetCreatedBy().Connector != nil {
		log.Warningf("Tried to issue a renewable cert for externally managed user %v, this is not supported.", username)
		return nil, trace.AccessDenied("access denied")
	}

	// Cap the cert TTL to the MaxRenewableCertTTL.
	if max := s.GetClock().Now().Add(defaults.MaxRenewableCertTTL); expires.After(max) {
		expires = max
	}

	// Inherit the user's roles and traits verbatim.
	roles := user.GetRoles()
	traits := user.GetTraits()

	parsedRoles, err := services.FetchRoleList(roles, s, traits)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// add implicit roles to the set and build a checker
	checker := services.NewRoleSet(parsedRoles...)

	// Generate certificate
	certReq := certRequest{
		user:          user,
		ttl:           expires.Sub(s.GetClock().Now()),
		publicKey:     pubKey,
		checker:       checker,
		traits:        user.GetTraits(),
		renewable:     true,
		includeHostCA: true,
		generation:    1,
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
