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

package machineidv1

import (
	"context"
	"slices"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
)

// GetBotUsersLegacy fetches all Users with the BotLabel field set. Users are fetched
// without secrets.
// TODO(noah): DELETE IN 16.0.0
// Deprecated: Switch to [BotService.ListBots].
func (bs *BotService) GetBotUsersLegacy(ctx context.Context) ([]types.User, error) {
	bs.logger.Warn("Deprecated GetBotUsers RPC called. Upgrade your client. From V16.0.0, this will fail!")
	authCtx, err := bs.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindUser, types.VerbList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	var botUsers []types.User
	var pageToken string
	for {
		users, token, err := bs.cache.ListUsers(
			ctx, 0, pageToken, false,
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, user := range users {
			if _, ok := user.GetMetadata().Labels[types.BotLabel]; ok {
				botUsers = append(botUsers, user)
			}
		}

		if token == "" {
			break
		}
		pageToken = token
	}

	return botUsers, nil
}

func (bs *BotService) checkOrCreateBotToken(ctx context.Context, req *proto.CreateBotRequest) (types.ProvisionToken, error) {
	if req.TokenID != "" {
		// if the request includes a TokenID it should already exist
		token, err := bs.backend.GetToken(ctx, req.TokenID)
		if err != nil {
			if trace.IsNotFound(err) {
				return nil, trace.NotFound("token with name %q not found, create the token or do not set TokenName: %v",
					req.TokenID, err)
			}
			return nil, trace.Wrap(err)
		}
		if !token.GetRoles().Include(types.RoleBot) {
			return nil, trace.BadParameter("token %q is not valid for role %q",
				req.TokenID, types.RoleBot)
		}
		if token.GetBotName() != req.Name {
			return nil, trace.BadParameter("token %q is valid for bot with name %q, not %q",
				req.TokenID, token.GetBotName(), req.Name)
		}

		if !slices.Contains(SupportedJoinMethods, token.GetJoinMethod()) {
			return nil, trace.BadParameter(
				"token %q has join method %q which is not supported for bots. Supported join methods are %v",
				req.TokenID,
				token.GetJoinMethod(),
				SupportedJoinMethods,
			)
		}
		return token, nil
	}

	tokenName, err := utils.CryptoRandomHex(defaults.TokenLenBytes)
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
		BotName:    req.Name,
	}
	token, err := types.NewProvisionTokenFromSpec(tokenName, bs.clock.Now().Add(ttl), tokenSpec)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := bs.backend.UpsertToken(ctx, token); err != nil {
		return nil, trace.Wrap(err)
	}
	return token, nil
}

// CreateBotLegacy creates a bot and a join token.
// TODO(noah): DELETE IN 16.0.0
// Deprecated: Switch to calling [BotService.CreateBot] and CreateToken separately.
func (bs *BotService) CreateBotLegacy(ctx context.Context, req *proto.CreateBotRequest) (*proto.CreateBotResponse, error) {
	bs.logger.Warn("Deprecated CreateBot RPC called. Upgrade your client. From V16.0.0, this will fail!")
	if _, err := bs.createBotAuthz(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	newReq := &pb.CreateBotRequest{
		Bot: &pb.Bot{
			Metadata: &headerv1.Metadata{
				Name: req.Name,
			},
			Spec: &pb.BotSpec{
				Roles:  req.Roles,
				Traits: []*pb.Trait{},
			},
		},
	}
	for k, v := range req.Traits {
		if len(v) == 0 {
			continue
		}
		newReq.Bot.Spec.Traits = append(newReq.Bot.Spec.Traits, &pb.Trait{
			Name:   k,
			Values: v,
		})
	}
	bot, err := bs.CreateBot(ctx, newReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Now we perform the legacy behavior of creating or checking an existing token
	token, err := bs.checkOrCreateBotToken(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tokenTTL := time.Duration(0)
	if exp := token.Expiry(); !exp.IsZero() {
		tokenTTL = time.Until(exp)
	}

	return &proto.CreateBotResponse{
		TokenID:    token.GetName(),
		UserName:   bot.Status.UserName,
		RoleName:   bot.Status.RoleName,
		TokenTTL:   proto.Duration(tokenTTL),
		JoinMethod: token.GetJoinMethod(),
	}, nil
}
