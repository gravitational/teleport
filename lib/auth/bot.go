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
	"strings"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/trace"
)

// botResourceName returns the default name for resources associated with the
// given named bot.
func botResourceName(botName string) string {
	return "bot-" + strings.ReplaceAll(botName, " ", "-")
}

// createBotRole creates a role from a bot template with the given parameters.
func createBotRole(ctx context.Context, s *Server, botName string, resourceName string, roleRequests []string) error {
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
		return trace.Wrap(err)
	}

	meta := role.GetMetadata()
	meta.Description = fmt.Sprintf("Automatically generated role for bot %s", botName)
	if meta.Labels == nil {
		meta.Labels = map[string]string{}
	}
	meta.Labels[types.BotLabel] = botName

	rolev5, ok := role.(*types.RoleV5)
	if !ok {
		return trace.BadParameter("unsupported role version %v", role)
	}
	rolev5.Metadata = meta

	return s.UpsertRole(ctx, role)
}

// createBotUser creates a new backing User for bot use. A role with a
// matching name must already exist (see createBotRole).
func createBotUser(ctx context.Context, s *Server, botName string, resourceName string) error {
	user, err := types.NewUser(resourceName)
	if err != nil {
		return trace.Wrap(err)
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
		return trace.Wrap(err)
	}

	return nil
}

// createBot creates a new certificate renewal bot from a bot request.
func (s *Server) createBot(ctx context.Context, req *proto.CreateBotRequest) (*proto.CreateBotResponse, error) {
	if req.TokenID != "" {
		// TODO: IAM joining for bots
		return nil, trace.NotImplemented("IAM join for bots is not yet supported")
	}

	if req.Name == "" {
		return nil, trace.BadParameter("bot name must not be empty")
	}

	resourceName := botResourceName(req.Name)

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

	// Create the resources.
	if err := createBotRole(ctx, s, req.Name, resourceName, req.Roles); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := createBotUser(ctx, s, req.Name, resourceName); err != nil {
		return nil, trace.Wrap(err)
	}

	ttl := time.Duration(req.TTL)
	if ttl == 0 {
		ttl = defaults.DefaultBotJoinTTL
	}

	token, err := s.CreateBotJoinToken(ctx, CreateUserTokenRequest{
		Name: resourceName,
		TTL:  ttl,
		Type: UserTokenTypeBot,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &proto.CreateBotResponse{
		TokenID:  token.GetName(),
		UserName: resourceName,
		RoleName: resourceName,
		TokenTTL: proto.Duration(ttl),
	}, nil
}

// deleteBotUser removes an existing bot user, ensuring that it has bot labels
// matching the bot before deleting anything.
func (s *Server) deleteBotUser(ctx context.Context, botName, resourceName string) error {
	user, err := s.GetUser(resourceName, false)
	if err != nil {
		err = trace.WrapWithMessage(err, "could not fetch expected bot user %s", resourceName)
	} else {
		label, ok := user.GetMetadata().Labels[types.BotLabel]
		if !ok {
			err = trace.Errorf("will not delete user %s that is missing label %s", resourceName, types.BotLabel)
		} else if label != botName {
			err = trace.Errorf("will not delete user %s with mismatched label %s = %s", resourceName, types.BotLabel, label)
		} else {
			err = s.DeleteUser(ctx, resourceName)
		}
	}

	return err
}

// deleteBotRole removes an existing bot role, ensuring that it has bot labels
// matching the bot before deleting anything.
func (s *Server) deleteBotRole(ctx context.Context, botName, resourceName string) error {
	role, err := s.GetRole(ctx, resourceName)
	if err != nil {
		err = trace.WrapWithMessage(err, "could not fetch expected bot role %s", resourceName)
	} else {
		label, ok := role.GetMetadata().Labels[types.BotLabel]
		if !ok {
			err = trace.Errorf("will not delete role %s that is missing label %s", resourceName, types.BotLabel)
		} else if label != botName {
			err = trace.Errorf("will not delete role %s with mismatched label %s = %s", resourceName, types.BotLabel, label)
		} else {
			err = s.DeleteRole(ctx, resourceName)
		}
	}

	return err
}

func (s *Server) deleteBot(ctx context.Context, botName string) error {
	// TODO:
	// remove any locks for the bot's impersonator role?
	// remove the bot's user
	resourceName := botResourceName(botName)

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

// CreateBotJoinToken creates a new joining token for bots.
func (s *Server) CreateBotJoinToken(ctx context.Context, req CreateUserTokenRequest) (types.UserToken, error) {
	err := req.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if req.Type != UserTokenTypeBot {
		return nil, trace.BadParameter("invalid bot token request type")
	}

	_, err = s.GetUser(req.Name, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO: ensure that the user is a bot user?
	token, err := s.newUserToken(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO: deleteUserTokens?
	token, err = s.Identity.CreateUserToken(ctx, token)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO: audit log event. partially implemented, may need events.proto and
	// dynamic.go, etc (unless we can reuse existing UserTokenCreate event)
	// if err := s.emitter.EmitAuditEvent(ctx, &apievents.UserTokenCreate{
	// 	Metadata: apievents.Metadata{
	// 		Type: events.BotTokenCreateEvent,
	// 		Code: events.ResetPasswordTokenCreateCode,
	// 	},
	// 	UserMetadata: apievents.UserMetadata{
	// 		User:         ClientUsername(ctx),
	// 		Impersonator: ClientImpersonator(ctx),
	// 	},
	// 	ResourceMetadata: apievents.ResourceMetadata{
	// 		Name:    req.Name,
	// 		TTL:     req.TTL.String(),
	// 		Expires: s.GetClock().Now().UTC().Add(req.TTL),
	// 	},
	// }); err != nil {
	// 	log.WithError(err).Warn("Failed to emit create reset password token event.")
	// }

	return token, nil
}
