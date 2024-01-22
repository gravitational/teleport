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

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
)

// WorkloadIdentityServiceConfig holds configuration options for
// the WorkloadIdentity gRPC service.
type WorkloadIdentityServiceConfig struct {
	Authorizer authz.Authorizer
	Cache      Cache
	Backend    Backend
	Logger     logrus.FieldLogger
	Emitter    apievents.Emitter
	Reporter   usagereporter.UsageReporter
	Clock      clockwork.Clock
}

// NewWorkloadIdentityService returns a new instance of the
// WorkloadIdentityService.
func NewWorkloadIdentityService(
	cfg WorkloadIdentityServiceConfig,
) (*WorkloadIdentityService, error) {
	switch {
	case cfg.Cache == nil:
		return nil, trace.BadParameter("cache service is required")
	case cfg.Backend == nil:
		return nil, trace.BadParameter("backend service is required")
	case cfg.Authorizer == nil:
		return nil, trace.BadParameter("authorizer is required")
	case cfg.Emitter == nil:
		return nil, trace.BadParameter("emitter is required")
	case cfg.Reporter == nil:
		return nil, trace.BadParameter("reporter is required")
	}

	if cfg.Logger == nil {
		cfg.Logger = logrus.WithField(trace.Component, "bot.service")
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}

	return &WorkloadIdentityService{
		logger:     cfg.Logger,
		authorizer: cfg.Authorizer,
		cache:      cfg.Cache,
		backend:    cfg.Backend,
		emitter:    cfg.Emitter,
		reporter:   cfg.Reporter,
		clock:      cfg.Clock,
	}, nil
}

// WorkloadIdentityService implements the teleport.machineid.v1.WorkloadIdentity
// RPC service.
type WorkloadIdentityService struct {
	pb.UnimplementedWorkloadIdentityServiceServer

	cache      Cache
	backend    Backend
	authorizer authz.Authorizer
	logger     logrus.FieldLogger
	emitter    apievents.Emitter
	reporter   usagereporter.UsageReporter
	clock      clockwork.Clock
}

func (wis *WorkloadIdentityService) signX509SVID(ctx context.Context, req *pb.SVIDRequest) (*pb.SVIDResponse, error) {
	// TODO: Authn/authz

	res := &pb.SVIDResponse{
		SpiffeId:    "",
		Hint:        req.Hint,
		Certificate: nil,
	}

	// TODO: Sign

	// TODO: Audit and analytics event

	return res, nil
}

func (wis *WorkloadIdentityService) SignX509SVIDs(ctx context.Context, req *pb.SignX509SVIDsRequest) (*pb.SignX509SVIDsResponse, error) {
	if len(req.Svids) == 0 {
		return nil, trace.BadParameter("svids: must be non-empty")
	}

	res := &pb.SignX509SVIDsResponse{}
	for i, svidReq := range req.Svids {
		svidRes, err := wis.signX509SVID(ctx, svidReq)
		if err != nil {
			return nil, trace.Wrap(err, "signing svid %d", i)
		}
		res.Svids = append(res.Svids, svidRes)
	}

	return res, nil
}
