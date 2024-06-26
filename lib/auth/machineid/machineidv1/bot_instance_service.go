/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	"log/slog"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport"
	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
)

// BotInstanceServiceConfig holds configuration options for the BotInstance gRPC
// service.
type BotInstanceServiceConfig struct {
	Authorizer authz.Authorizer
	Backend    services.BotInstance
	Logger     *slog.Logger
	Clock      clockwork.Clock
}

// NewBotInstanceService returns a new instance of the BotInstanceService.
func NewBotInstanceService(cfg BotInstanceServiceConfig) (*BotInstanceService, error) {
	switch {
	case cfg.Backend == nil:
		return nil, trace.BadParameter("backend service is required")
	case cfg.Authorizer == nil:
		return nil, trace.BadParameter("authorizer is required")
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.With(teleport.ComponentKey, "bot_instance.service")
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}

	return &BotInstanceService{
		logger:     cfg.Logger,
		authorizer: cfg.Authorizer,
		backend:    cfg.Backend,
		clock:      cfg.Clock,
	}, nil
}

// BotInstanceService implements the teleport.machineid.v1.BotInstanceService RPC service.
type BotInstanceService struct {
	pb.UnimplementedBotInstanceServiceServer

	backend    services.BotInstance
	authorizer authz.Authorizer
	logger     *slog.Logger
	clock      clockwork.Clock
}

// DeleteBotInstance deletes a bot specific bot instance
func (b *BotInstanceService) DeleteBotInstance(ctx context.Context, req *pb.DeleteBotInstanceRequest) (*emptypb.Empty, error) {
	authCtx, err := b.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindBotInstance, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := b.backend.DeleteBotInstance(ctx, req.BotName, req.InstanceId); err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}

// GetBotInstance retrieves a specific bot instance
func (b *BotInstanceService) GetBotInstance(ctx context.Context, req *pb.GetBotInstanceRequest) (*pb.BotInstance, error) {
	authCtx, err := b.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindBotInstance, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	res, err := b.backend.GetBotInstance(ctx, req.BotName, req.InstanceId)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return res, nil
}

// ListBotInstances returns a list of bot instances matching the criteria in the request
func (b *BotInstanceService) ListBotInstances(ctx context.Context, req *pb.ListBotInstancesRequest) (*pb.ListBotInstancesResponse, error) {
	authCtx, err := b.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindBotInstance, types.VerbRead, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	res, nextToken, err := b.backend.ListBotInstances(ctx, req.FilterBotName, int(req.PageSize), req.PageToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &pb.ListBotInstancesResponse{
		BotInstances:  res,
		NextPageToken: nextToken,
	}, nil
}

// SubmitHeartbeat records heartbeat information for a bot
func (b *BotInstanceService) SubmitHeartbeat(ctx context.Context, req *pb.SubmitHeartbeatRequest) (*pb.SubmitHeartbeatResponse, error) {
	// TODO: to be implemented in follow-up PR alongside bot instance creation.
	return nil, trace.NotImplemented("TODO")
}
