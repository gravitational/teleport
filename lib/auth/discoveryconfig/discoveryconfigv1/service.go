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

package discoveryconfigv1

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport"
	discoveryconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryconfig/v1"
	"github.com/gravitational/teleport/api/metadata"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	conv "github.com/gravitational/teleport/api/types/discoveryconfig/convert/v1"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/aws"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

// ServiceConfig holds configuration options for the DiscoveryConfig gRPC service.
type ServiceConfig struct {
	// Logger is the logger to use.
	Logger *slog.Logger

	// Authorizer is the authorizer to use.
	Authorizer authz.Authorizer

	// Backend is the backend for storing DiscoveryConfigs.
	Backend services.DiscoveryConfigs

	// Clock is the clock.
	Clock clockwork.Clock

	// Emitter emits audit events.
	Emitter apievents.Emitter
}

// CheckAndSetDefaults checks the ServiceConfig fields and returns an error if
// a required param is not provided.
// Authorizer, Cache and Backend are required params
func (s *ServiceConfig) CheckAndSetDefaults() error {
	if s.Authorizer == nil {
		return trace.BadParameter("authorizer is required")
	}
	if s.Backend == nil {
		return trace.BadParameter("backend is required")
	}
	if s.Emitter == nil {
		return trace.BadParameter("emitter is required")
	}

	if s.Logger == nil {
		s.Logger = slog.With(teleport.ComponentKey, "discoveryconfig_crud_service")
	}

	if s.Clock == nil {
		s.Clock = clockwork.NewRealClock()
	}

	return nil
}

// Service implements the teleport.DiscoveryConfig.v1.DiscoveryConfigService RPC service.
type Service struct {
	discoveryconfigv1.UnimplementedDiscoveryConfigServiceServer

	log        *slog.Logger
	authorizer authz.Authorizer
	backend    services.DiscoveryConfigs
	clock      clockwork.Clock
	emitter    apievents.Emitter
}

// NewService returns a new DiscoveryConfigs gRPC service.
func NewService(cfg ServiceConfig) (*Service, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Service{
		log:        cfg.Logger,
		authorizer: cfg.Authorizer,
		backend:    cfg.Backend,
		clock:      cfg.Clock,
		emitter:    cfg.Emitter,
	}, nil
}

// ListDiscoveryConfigs returns a paginated list of all DiscoveryConfig resources.
func (s *Service) ListDiscoveryConfigs(ctx context.Context, req *discoveryconfigv1.ListDiscoveryConfigsRequest) (*discoveryconfigv1.ListDiscoveryConfigsResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindDiscoveryConfig, types.VerbRead, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	results, nextKey, err := s.backend.ListDiscoveryConfigs(ctx, int(req.GetPageSize()), req.GetNextToken())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	dcs := make([]*discoveryconfigv1.DiscoveryConfig, len(results))
	for i, r := range results {
		downgraded, err := MaybeDowngradeDiscoveryConfig(ctx, r)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		dcs[i] = conv.ToProto(downgraded)
	}

	return &discoveryconfigv1.ListDiscoveryConfigsResponse{
		DiscoveryConfigs: dcs,
		NextKey:          nextKey,
	}, nil
}

// GetDiscoveryConfig returns the specified DiscoveryConfig resource.
func (s *Service) GetDiscoveryConfig(ctx context.Context, req *discoveryconfigv1.GetDiscoveryConfigRequest) (*discoveryconfigv1.DiscoveryConfig, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindDiscoveryConfig, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	dc, err := s.backend.GetDiscoveryConfig(ctx, req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	downgraded, err := MaybeDowngradeDiscoveryConfig(ctx, dc)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return conv.ToProto(downgraded), nil
}

// CreateDiscoveryConfig creates a new DiscoveryConfig resource.
func (s *Service) CreateDiscoveryConfig(ctx context.Context, req *discoveryconfigv1.CreateDiscoveryConfigRequest) (*discoveryconfigv1.DiscoveryConfig, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindDiscoveryConfig, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	dc, err := conv.FromProto(req.GetDiscoveryConfig())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Set the status to an empty struct to clear any status that may have been set
	// in the request.
	dc.Status = discoveryconfig.Status{}

	resp, err := s.backend.CreateDiscoveryConfig(ctx, dc)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.emitter.EmitAuditEvent(ctx, &apievents.DiscoveryConfigCreate{
		Metadata: apievents.Metadata{
			Type: events.DiscoveryConfigCreateEvent,
			Code: events.DiscoveryConfigCreateCode,
		},
		UserMetadata: authCtx.GetUserMetadata(),
		ResourceMetadata: apievents.ResourceMetadata{
			Name:    resp.GetName(),
			Expires: resp.Expiry(),
		},
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
	}); err != nil {
		s.log.WarnContext(ctx, "Failed to emit discovery config create event.", "error", err)
	}

	return conv.ToProto(resp), nil
}

// UpdateDiscoveryConfig updates an existing DiscoveryConfig.
func (s *Service) UpdateDiscoveryConfig(ctx context.Context, req *discoveryconfigv1.UpdateDiscoveryConfigRequest) (*discoveryconfigv1.DiscoveryConfig, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindDiscoveryConfig, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	dc, err := conv.FromProto(req.GetDiscoveryConfig())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Set the status to the existing status to ensure it is not cleared.
	oldDiscoveryConfig, err := s.backend.GetDiscoveryConfig(ctx, dc.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	dc.Status = oldDiscoveryConfig.Status

	resp, err := s.backend.UpdateDiscoveryConfig(ctx, dc)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.emitter.EmitAuditEvent(ctx, &apievents.DiscoveryConfigUpdate{
		Metadata: apievents.Metadata{
			Type: events.DiscoveryConfigUpdateEvent,
			Code: events.DiscoveryConfigUpdateCode,
		},
		UserMetadata: authCtx.GetUserMetadata(),
		ResourceMetadata: apievents.ResourceMetadata{
			Name:    resp.GetName(),
			Expires: resp.Expiry(),
		},
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
	}); err != nil {
		s.log.WarnContext(ctx, "Failed to emit discovery config update event.", "error", err)
	}

	return conv.ToProto(resp), nil
}

// UpsertDiscoveryConfig creates or updates a DiscoveryConfig.
func (s *Service) UpsertDiscoveryConfig(ctx context.Context, req *discoveryconfigv1.UpsertDiscoveryConfigRequest) (*discoveryconfigv1.DiscoveryConfig, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindDiscoveryConfig, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	dc, err := conv.FromProto(req.GetDiscoveryConfig())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Set the status to an empty struct to clear any status that may have been set
	// in the request.
	dc.Status = discoveryconfig.Status{}

	resp, err := s.backend.UpsertDiscoveryConfig(ctx, dc)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.emitter.EmitAuditEvent(ctx, &apievents.DiscoveryConfigCreate{
		Metadata: apievents.Metadata{
			Type: events.DiscoveryConfigCreateEvent,
			Code: events.DiscoveryConfigCreateCode,
		},
		UserMetadata: authCtx.GetUserMetadata(),
		ResourceMetadata: apievents.ResourceMetadata{
			Name:    resp.GetName(),
			Expires: resp.Expiry(),
		},
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
	}); err != nil {
		s.log.WarnContext(ctx, "Failed to emit discovery config create event.", "error", err)
	}

	return conv.ToProto(resp), nil
}

// DeleteDiscoveryConfig removes the specified DiscoveryConfig resource.
func (s *Service) DeleteDiscoveryConfig(ctx context.Context, req *discoveryconfigv1.DeleteDiscoveryConfigRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindDiscoveryConfig, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.backend.DeleteDiscoveryConfig(ctx, req.GetName()); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.emitter.EmitAuditEvent(ctx, &apievents.DiscoveryConfigDelete{
		Metadata: apievents.Metadata{
			Type: events.DiscoveryConfigDeleteEvent,
			Code: events.DiscoveryConfigDeleteCode,
		},
		UserMetadata: authCtx.GetUserMetadata(),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: req.Name,
		},
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
	}); err != nil {
		s.log.WarnContext(ctx, "Failed to emit discovery config delete event.", "error", err)
	}

	return &emptypb.Empty{}, nil
}

// DeleteAllDiscoveryConfigs removes all DiscoveryConfig resources.
func (s *Service) DeleteAllDiscoveryConfigs(ctx context.Context, _ *discoveryconfigv1.DeleteAllDiscoveryConfigsRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindDiscoveryConfig, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.backend.DeleteAllDiscoveryConfigs(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.emitter.EmitAuditEvent(ctx, &apievents.DiscoveryConfigDeleteAll{
		Metadata: apievents.Metadata{
			Type: events.DiscoveryConfigDeleteAllEvent,
			Code: events.DiscoveryConfigDeleteAllCode,
		},
		UserMetadata:       authCtx.GetUserMetadata(),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
	}); err != nil {
		s.log.WarnContext(ctx, "Failed to emit discovery config delete all event.", "error", err)
	}

	return &emptypb.Empty{}, nil
}

// UpdateDiscoveryConfigStatus updates the status of a DiscoveryConfig.
func (s *Service) UpdateDiscoveryConfigStatus(ctx context.Context, req *discoveryconfigv1.UpdateDiscoveryConfigStatusRequest) (*discoveryconfigv1.DiscoveryConfig, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authCtx, string(types.RoleDiscovery)) {
		return nil, trace.AccessDenied("UpdateDiscoveryConfigStatus request can be only executed by a Discovery Service")
	}

	for {
		dc, err := s.backend.GetDiscoveryConfig(ctx, req.GetName())
		switch {
		case trace.IsNotFound(err):
			return nil, trace.NotFound("discovery config %q not found", req.GetName())
		case err != nil:
			return nil, trace.Wrap(err)
		}

		dc.Status = conv.StatusFromProto(req.GetStatus())
		resp, err := s.backend.UpdateDiscoveryConfig(ctx, dc)
		if err != nil {
			if trace.IsCompareFailed(err) {
				continue
			}
			return nil, trace.Wrap(err)
		}

		return conv.ToProto(resp), nil
	}
}

// MaybeDowngradeDiscoveryConfig tests the client version passed through the gRPC metadata,
// and if necessary downgrades the Discovery Config resource for compatibility with the older client.
// The following rules are applied:
//   - if version is lower than 18.4.2, the AWS wildcard region is replaced with "aws-global" sentinel region
//     this ensures the client can still discover other resources without erroring out.
func MaybeDowngradeDiscoveryConfig(ctx context.Context, dc *discoveryconfig.DiscoveryConfig) (*discoveryconfig.DiscoveryConfig, error) {
	clientVersionString, ok := metadata.ClientVersionFromContext(ctx)
	if !ok {
		// This client is not reporting its version via gRPC metadata, which means it's a client really old or a third-party client.
		// For those, downgrading the resource will do more harm than good, so the resource is returned as is.
		return dc, nil
	}

	clientVersion, err := semver.NewVersion(clientVersionString)
	if err != nil {
		return nil, trace.BadParameter("unrecognized client version: %s is not a valid semver", clientVersionString)
	}

	dc = maybeDowngradeDiscoveryConfigAWSWildcardRegion(dc, clientVersion)
	return dc, nil
}

var minSupportedDiscoveryConfigAWSWildcardRegionVersion = semver.Version{Major: 18, Minor: 4, Patch: 2}

// For Auth Server v20.0.0, the expected minimum supported client version is v19.0.0, which supports the AWS wildcard region.
// This function should be deleted at that time.
//
// TODO(@marco): DELETE IN v20.0.0.
func maybeDowngradeDiscoveryConfigAWSWildcardRegion(dc *discoveryconfig.DiscoveryConfig, clientVersion *semver.Version) *discoveryconfig.DiscoveryConfig {
	if supported, err := utils.MinVerWithoutPreRelease(
		clientVersion.String(),
		minSupportedDiscoveryConfigAWSWildcardRegionVersion.String()); supported || err != nil {
		return dc
	}

	var changed bool

	originalDiscoveryConfig := dc

	dc = dc.Clone()
	awsMatchers := dc.Spec.AWS
	awsMatchersWithoutRegionWildcard := make([]types.AWSMatcher, 0, len(awsMatchers))
	for _, awsMatcher := range awsMatchers {
		if len(awsMatcher.Regions) == 1 && awsMatcher.Regions[0] == types.Wildcard {
			awsMatcher.Regions = []string{aws.AWSGlobalRegion}
			changed = true
		}
		awsMatchersWithoutRegionWildcard = append(awsMatchersWithoutRegionWildcard, awsMatcher)
	}

	if !changed {
		return originalDiscoveryConfig
	}

	dc.Spec.AWS = awsMatchersWithoutRegionWildcard
	reason := fmt.Sprintf(`Client version %q does not support discovering all regions. Either update the Discovery Service agent to at least %s or enumerate all the regions in %q discovery config.`,
		clientVersion, minSupportedDiscoveryConfigAWSWildcardRegionVersion, dc.GetName())
	if dc.Metadata.Labels == nil {
		dc.Metadata.Labels = make(map[string]string, 1)
	}
	dc.Metadata.Labels[types.TeleportDowngradedLabel] = reason
	return dc
}
