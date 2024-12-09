// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package workloadidentityv1

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
)

// IssuanceServiceConfig holds configuration options for the IssuanceService.
type IssuanceServiceConfig struct {
	Authorizer authz.Authorizer
	Cache      workloadIdentityReader
	Clock      clockwork.Clock
	Emitter    apievents.Emitter
	Logger     *slog.Logger
}

// IssuanceService is the gRPC service for managing workload identity resources.
// It implements the workloadidentityv1pb.WorkloadIdentityIssuanceServiceServer.
type IssuanceService struct {
	workloadidentityv1pb.UnimplementedWorkloadIdentityIssuanceServiceServer

	authorizer authz.Authorizer
	cache      workloadIdentityReader
	clock      clockwork.Clock
	emitter    apievents.Emitter
	logger     *slog.Logger
}

// NewIssuanceService returns a new instance of the IssuanceService.
func NewIssuanceService(cfg *IssuanceServiceConfig) (*IssuanceService, error) {
	switch {
	case cfg.Cache == nil:
		return nil, trace.BadParameter("cache service is required")
	case cfg.Authorizer == nil:
		return nil, trace.BadParameter("authorizer is required")
	case cfg.Emitter == nil:
		return nil, trace.BadParameter("emitter is required")
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.With(teleport.ComponentKey, "workload_identity_issuance.service")
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	return &IssuanceService{
		authorizer: cfg.Authorizer,
		cache:      cfg.Cache,
		clock:      cfg.Clock,
		emitter:    cfg.Emitter,
		logger:     cfg.Logger,
	}, nil
}

func (s *IssuanceService) IssueWorkloadIdentity(
	ctx context.Context,
	req *workloadidentityv1pb.IssueWorkloadIdentityRequest,
) (*workloadidentityv1pb.IssueWorkloadIdentityResponse, error) {
	return nil, trace.NotImplemented("not implemented")
}
