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
	"maps"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport"
	discoveryconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryconfig/v1"
	"github.com/gravitational/teleport/api/metadata"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	conv "github.com/gravitational/teleport/api/types/discoveryconfig/convert/v1"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/aws"
	"github.com/gravitational/teleport/api/utils/retryutils"
	prehogv1a "github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
	"github.com/gravitational/teleport/lib/utils"
)

// ServiceConfig holds configuration options for the DiscoveryConfig gRPC service.
type ServiceConfig struct {
	// Logger is the logger to use.
	Logger *slog.Logger

	// Authorizer is the authorizer to use.
	Authorizer authz.Authorizer

	// Backend stores regular DiscoveryConfigs.
	Backend services.DiscoveryConfigs
	// SyntheticBackend is the isolated backend for synthetic DiscoveryConfigs.
	SyntheticBackend services.SyntheticDiscoveryConfigs

	// Clock is the clock.
	Clock clockwork.Clock

	// Emitter emits audit events.
	Emitter apievents.Emitter

	// UsageReporter is the reporter for sending usage events.
	UsageReporter usagereporter.UsageReporter
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
	if s.SyntheticBackend == nil {
		return trace.BadParameter("synthetic backend is required")
	}
	if s.Emitter == nil {
		return trace.BadParameter("emitter is required")
	}
	if s.UsageReporter == nil {
		return trace.BadParameter("usage reporter is required")
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

	log              *slog.Logger
	authorizer       authz.Authorizer
	backend          services.DiscoveryConfigs
	syntheticBackend services.SyntheticDiscoveryConfigs
	clock            clockwork.Clock
	emitter          apievents.Emitter
	usageReporter    usagereporter.UsageReporter
}

const discoveryConfigWriteAttempts = 4

// NewService returns a new DiscoveryConfigs gRPC service.
func NewService(cfg ServiceConfig) (*Service, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Service{
		log:              cfg.Logger,
		authorizer:       cfg.Authorizer,
		backend:          cfg.Backend,
		syntheticBackend: cfg.SyntheticBackend,
		clock:            cfg.Clock,
		emitter:          cfg.Emitter,
		usageReporter:    cfg.UsageReporter,
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

	return discoveryconfigv1.ListDiscoveryConfigsResponse_builder{
		DiscoveryConfigs: dcs,
		NextKey:          nextKey,
	}.Build(), nil
}

// ListSyntheticDiscoveryConfigs explicitly lists owner-managed synthetic inventory.
func (s *Service) ListSyntheticDiscoveryConfigs(ctx context.Context, req *discoveryconfigv1.ListDiscoveryConfigsRequest) (*discoveryconfigv1.ListDiscoveryConfigsResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindDiscoveryConfig, types.VerbRead, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	results, next, err := s.syntheticBackend.ListSyntheticDiscoveryConfigs(ctx, int(req.GetPageSize()), req.GetNextToken())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := make([]*discoveryconfigv1.DiscoveryConfig, len(results))
	for i, dc := range results {
		out[i] = conv.ToProto(dc)
	}
	return &discoveryconfigv1.ListDiscoveryConfigsResponse{DiscoveryConfigs: out, NextKey: next}, nil
}

// GetDiscoveryConfig returns the specified DiscoveryConfig resource.
//
// This legacy RPC intentionally serves regular resources only: clients
// released before synthetic resources existed cannot decode a synthetic
// resource's empty spec, so serving one here would turn a successful RPC into
// a confusing client-side validation failure. Current API clients combine
// this RPC with GetSyntheticDiscoveryConfig for regular-first, synthetic-second
// named lookup.
func (s *Service) GetDiscoveryConfig(ctx context.Context, req *discoveryconfigv1.GetDiscoveryConfigRequest) (*discoveryconfigv1.DiscoveryConfig, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindDiscoveryConfig, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	dc, err := s.backend.GetDiscoveryConfig(ctx, req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	downgraded, err := MaybeDowngradeDiscoveryConfig(ctx, dc)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return conv.ToProto(downgraded), nil
}

// GetSyntheticDiscoveryConfig returns owner-published synthetic inventory by
// name. See GetDiscoveryConfig for why synthetic resources are not served
// through the legacy RPC.
func (s *Service) GetSyntheticDiscoveryConfig(ctx context.Context, req *discoveryconfigv1.GetDiscoveryConfigRequest) (*discoveryconfigv1.DiscoveryConfig, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindDiscoveryConfig, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	dc, err := s.syntheticBackend.GetSyntheticDiscoveryConfig(ctx, req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return conv.ToProto(dc), nil
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

	name := req.GetDiscoveryConfig().GetHeader().GetMetadata().GetName()
	if err := s.checkReservedWrite(ctx, name, true); err != nil {
		return nil, trace.Wrap(err)
	}
	// checkReservedWrite passes a reserved name only when a grandfathered
	// regular config occupies it, so a create can never succeed; answer
	// without racing the backend, where a concurrent deletion would let the
	// create recreate a reserved name.
	if discoveryconfig.IsReservedSyntheticName(name) {
		return nil, trace.AlreadyExists("discovery config %q already exists", name)
	}
	dc, err := regularDiscoveryConfigFromProto(req.GetDiscoveryConfig())
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

	s.emitUsageEvent(resp, prehogv1a.DiscoveryConfigAction_DISCOVERY_CONFIG_ACTION_CREATE)

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

	if err := s.checkReservedWrite(ctx, req.GetDiscoveryConfig().GetHeader().GetMetadata().GetName(), false); err != nil {
		return nil, trace.Wrap(err)
	}
	dc, err := regularDiscoveryConfigFromProto(req.GetDiscoveryConfig())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Preserve the stored status. Synthetic resources are in a different key
	// range, so regular updates retain their historical write behavior.
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

	s.emitUsageEvent(resp, prehogv1a.DiscoveryConfigAction_DISCOVERY_CONFIG_ACTION_UPDATE)

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

	if err := s.checkReservedWrite(ctx, req.GetDiscoveryConfig().GetHeader().GetMetadata().GetName(), true); err != nil {
		return nil, trace.Wrap(err)
	}
	dc, err := regularDiscoveryConfigFromProto(req.GetDiscoveryConfig())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Set the status to an empty struct to clear any status that may have been set
	// in the request.
	dc.Status = discoveryconfig.Status{}

	resp, err := s.upsertRegularDiscoveryConfig(ctx, dc)
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

	s.emitUsageEvent(resp, prehogv1a.DiscoveryConfigAction_DISCOVERY_CONFIG_ACTION_CREATE)

	return conv.ToProto(resp), nil
}

// UpsertSyntheticDiscoveryConfig publishes only the authenticated owner's
// inventory. Periodic machine renewals intentionally emit neither generic
// DiscoveryConfig CRUD audit events nor lifecycle usage events: like discovery
// status updates, they report observed state rather than a user configuration
// change.
func (s *Service) UpsertSyntheticDiscoveryConfig(ctx context.Context, req *discoveryconfigv1.UpsertSyntheticDiscoveryConfigRequest) (*discoveryconfigv1.DiscoveryConfig, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	identity, ok := authCtx.Identity.(authz.BuiltinRole)
	if !ok || !authz.HasBuiltinRole(*authCtx, string(types.RoleDiscovery)) || identity.GetServerID() == "" {
		return nil, trace.AccessDenied("synthetic DiscoveryConfigs may only be published by their owning Discovery Service")
	}
	name := discoveryconfig.SyntheticName(identity.GetServerID())
	if _, err := s.backend.GetDiscoveryConfig(ctx, name); err == nil {
		return nil, trace.AlreadyExists("regular discovery config %q occupies the synthetic owner name", name)
	} else if !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	synthetic := conv.StatusFromProto(&discoveryconfigv1.DiscoveryConfigStatus{Synthetic: req.GetSynthetic()}).Synthetic
	if synthetic == nil {
		return nil, trace.BadParameter("synthetic inventory is required")
	}
	// The wire-size budget is API policy rather than a resource-construction
	// invariant, so it stays here rather than in the constructor.
	if !req.GetSynthetic().GetMatchersTruncated() && req.GetSynthetic().GetMatchers() != nil && proto.Size(req.GetSynthetic().GetMatchers()) > discoveryconfig.SyntheticMatcherDetailBudget {
		return nil, trace.LimitExceeded("synthetic matcher detail exceeds maximum size of %d bytes", discoveryconfig.SyntheticMatcherDetailBudget)
	}
	dc, err := discoveryconfig.NewSyntheticDiscoveryConfig(identity.GetServerID(), *synthetic)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for attempt := range discoveryConfigWriteAttempts {
		if err := waitForDiscoveryConfigRetry(ctx, attempt); err != nil {
			return nil, err
		}
		dc.SetExpiry(s.clock.Now().Add(discoveryconfig.SyntheticDiscoveryConfigTTL))
		existing, err := s.syntheticBackend.GetSyntheticDiscoveryConfig(ctx, name)
		if trace.IsNotFound(err) {
			// A fresh publication must not carry status merged from a record
			// observed by an earlier iteration and deleted since.
			dc.Status = discoveryconfig.Status{Synthetic: dc.Status.Synthetic}
			dc.SetRevision("")
			if _, err := services.MarshalSyntheticDiscoveryConfig(dc); err != nil {
				return nil, trace.Wrap(err)
			}
			created, err := s.syntheticBackend.CreateSyntheticDiscoveryConfig(ctx, dc)
			if trace.IsAlreadyExists(err) {
				continue
			}
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return conv.ToProto(created), nil
		}
		if err != nil {
			return nil, trace.Wrap(err)
		}
		inventory := dc.Status.Synthetic
		dc.Status = existing.Status
		dc.Status.Synthetic = inventory
		dc.SetRevision(existing.GetRevision())
		if _, err := services.MarshalSyntheticDiscoveryConfig(dc); err != nil {
			return nil, trace.Wrap(err)
		}
		updated, err := s.syntheticBackend.ConditionalUpdateSyntheticDiscoveryConfig(ctx, dc)
		if trace.IsCompareFailed(err) {
			continue
		}
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return conv.ToProto(updated), nil
	}
	return nil, trace.LimitExceeded("synthetic discovery config write exceeded retry limit")
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

	// Fetch the DiscoveryConfig before deletion to capture metadata for the usage event.
	dc, err := s.backend.GetDiscoveryConfig(ctx, req.GetName())
	if trace.IsNotFound(err) {
		if _, syntheticErr := s.syntheticBackend.GetSyntheticDiscoveryConfig(ctx, req.GetName()); syntheticErr == nil {
			return nil, trace.AccessDenied("synthetic discovery config %q is owner-managed", req.GetName())
		} else if !trace.IsNotFound(syntheticErr) {
			return nil, trace.Wrap(syntheticErr)
		}
	}
	if err != nil && !trace.IsNotFound(err) {
		s.log.WarnContext(ctx, "Skipping DiscoveryConfig delete usage event due to GetDiscoveryConfig failure.",
			"discovery_config_name", req.GetName(),
			"error", err)
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
			Name: req.GetName(),
		},
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
	}); err != nil {
		s.log.WarnContext(ctx, "Failed to emit discovery config delete event.", "error", err)
	}

	if dc != nil {
		s.emitUsageEvent(dc, prehogv1a.DiscoveryConfigAction_DISCOVERY_CONFIG_ACTION_DELETE)
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
	for attempt := range discoveryConfigWriteAttempts {
		if err := waitForDiscoveryConfigRetry(ctx, attempt); err != nil {
			return nil, err
		}
		dc, err := s.backend.GetDiscoveryConfig(ctx, req.GetName())
		switch {
		case trace.IsNotFound(err):
			if !discoveryconfig.IsReservedSyntheticName(req.GetName()) {
				return nil, trace.NotFound("discovery config %q not found", req.GetName())
			}
			dc, err = s.syntheticBackend.GetSyntheticDiscoveryConfig(ctx, req.GetName())
			if trace.IsNotFound(err) {
				return nil, trace.NotFound("discovery config %q not found", req.GetName())
			}
			if err != nil {
				return nil, trace.Wrap(err)
			}
			identity, ok := authCtx.Identity.(authz.BuiltinRole)
			if !ok || identity.GetServerID() == "" {
				return nil, trace.AccessDenied("synthetic status requires a builtin Discovery Service identity")
			}
			if req.GetName() != discoveryconfig.SyntheticName(identity.GetServerID()) {
				return nil, trace.AccessDenied("synthetic discovery config status is owner-managed")
			}
			status := conv.StatusFromProto(req.GetStatus())
			for serverID := range status.ServerStatus {
				if serverID != identity.GetServerID() {
					return nil, trace.BadParameter("synthetic status contains foreign server %q", serverID)
				}
			}
			status.Synthetic = dc.Status.Synthetic
			dc.Status = status
			if err := validateSyntheticReportSize(dc); err != nil {
				return nil, trace.Wrap(err)
			}
			resp, err := s.syntheticBackend.ConditionalUpdateSyntheticDiscoveryConfig(ctx, dc)
			if trace.IsCompareFailed(err) {
				continue
			}
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return conv.ToProto(resp), nil
		case err != nil:
			return nil, trace.Wrap(err)
		}
		status := conv.StatusFromProto(req.GetStatus())
		status.Synthetic = nil
		dc.Status = status
		resp, err := s.backend.UpdateDiscoveryConfig(ctx, dc)
		if err != nil {
			if trace.IsCompareFailed(err) {
				continue
			}
			return nil, trace.Wrap(err)
		}

		return conv.ToProto(resp), nil
	}
	return nil, trace.LimitExceeded("discovery config status update exceeded retry limit; retry the request")
}

// validateSyntheticReportSize verifies both the report merged with the
// current inventory and the report with the count-only fallback a publisher
// uses when detailed inventory no longer fits. This prevents report updates
// from consuming the space required to keep owner renewal available.
func validateSyntheticReportSize(dc *discoveryconfig.DiscoveryConfig) error {
	if _, err := services.MarshalSyntheticDiscoveryConfig(dc); err != nil {
		return trace.Wrap(err)
	}

	fallback := dc.Clone()
	fallback.Status.Synthetic = &discoveryconfig.SyntheticStatus{
		DiscoveryGroup:    dc.Status.Synthetic.DiscoveryGroup,
		MatchersTruncated: true,
		MatcherCounts: &discoveryconfig.StaticMatcherCounts{
			AWS:         math.MaxUint32,
			Azure:       math.MaxUint32,
			GCP:         math.MaxUint32,
			Kube:        math.MaxUint32,
			AccessGraph: math.MaxUint32,
		},
	}
	_, err := services.MarshalSyntheticDiscoveryConfig(fallback)
	return trace.Wrap(err)
}

func (s *Service) upsertRegularDiscoveryConfig(ctx context.Context, dc *discoveryconfig.DiscoveryConfig) (*discoveryconfig.DiscoveryConfig, error) {
	if discoveryconfig.IsReservedSyntheticName(dc.GetName()) {
		// checkReservedWrite already verified that a regular config predating
		// the reservation occupies this name; an upsert must not recreate one
		// after concurrent deletion.
		updated, err := s.backend.UpdateDiscoveryConfig(ctx, dc)
		if trace.IsNotFound(err) {
			return nil, reservedSyntheticNameError(dc.GetName())
		}
		return updated, trace.Wrap(err)
	}
	return s.backend.UpsertDiscoveryConfig(ctx, dc)
}

func reservedSyntheticNameError(name string) error {
	return trace.BadParameter("discovery config name %q is reserved for synthetic discovery configs; a pre-existing config with this name can be updated but not recreated; rename it to keep managing it", name)
}

// checkReservedWrite gates the generic write RPCs for reserved synthetic
// names so get-then-mutate flows (web UI edits, tctl edit) fail with the
// ownership story rather than conversion or existence noise. It runs before
// proto conversion on purpose: a round-tripped synthetic resource has an
// empty spec that would otherwise fail validation with a misleading
// "discovery group is missing".
//
// Regular configs that predate the reservation remain manageable (nil). A name
// occupied by owner-published synthetic inventory is readable but owner-managed
// (AccessDenied, matching DeleteDiscoveryConfig). When rejectUnoccupied is set,
// an unoccupied reserved name is rejected with the migration-oriented
// BadParameter used by create and upsert. Update leaves it to the backend to
// return its normal NotFound response.
func (s *Service) checkReservedWrite(ctx context.Context, name string, rejectUnoccupied bool) error {
	if !discoveryconfig.IsReservedSyntheticName(name) {
		return nil
	}
	if _, err := s.backend.GetDiscoveryConfig(ctx, name); err == nil {
		return nil
	} else if !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if _, err := s.syntheticBackend.GetSyntheticDiscoveryConfig(ctx, name); err == nil {
		return trace.AccessDenied("synthetic discovery config %q is owner-managed", name)
	} else if !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if !rejectUnoccupied {
		return nil
	}
	return reservedSyntheticNameError(name)
}

func waitForDiscoveryConfigRetry(ctx context.Context, attempt int) error {
	if attempt == 0 {
		return nil
	}
	t := time.NewTimer(retryutils.FullJitter(time.Duration(300*attempt) * time.Millisecond))
	defer t.Stop()
	select {
	case <-ctx.Done():
		return trace.Wrap(ctx.Err())
	case <-t.C:
		return nil
	}
}

// regularDiscoveryConfigFromProto preserves the legacy write behavior: all
// subkinds are discarded before conversion and therefore cannot influence
// validation or normalization of a regular write.
func regularDiscoveryConfigFromProto(msg *discoveryconfigv1.DiscoveryConfig) (*discoveryconfig.DiscoveryConfig, error) {
	if msg == nil {
		return conv.FromProto(nil)
	}
	msg = proto.CloneOf(msg)
	if msg.GetHeader() != nil {
		msg.GetHeader().SubKind = ""
	}
	return conv.FromProto(msg)
}

// emitUsageEvent emits a DiscoveryConfigEvent usage event.
func (s *Service) emitUsageEvent(dc *discoveryconfig.DiscoveryConfig, action prehogv1a.DiscoveryConfigAction) {
	resourceTypes, cloudProviders := extractDiscoveryConfigMetadata(dc)

	creationMethod := CreationMethodGuided
	if _, ok := dc.GetMetadata().Labels[types.IACToolLabel]; ok {
		creationMethod = CreationMethodIAC
	}

	s.usageReporter.AnonymizeAndSubmit(&usagereporter.DiscoveryConfigEvent{
		Action:              action,
		DiscoveryConfigName: dc.GetName(),
		ResourceTypes:       resourceTypes,
		CloudProviders:      cloudProviders,
		CreationMethod:      creationMethod,
	})
}

// extractDiscoveryConfigMetadata extracts resource types and cloud providers
// for DiscoveryConfigEvent fields.
func extractDiscoveryConfigMetadata(dc *discoveryconfig.DiscoveryConfig) (resourceTypes, cloudProviders []string) {
	cloudProviderSet := make(map[string]struct{})
	resourceTypeSet := make(map[string]struct{})

	addMatcher := func(cloud string, types []string) {
		if len(types) == 0 {
			return
		}

		c := strings.ToLower(cloud)

		cloudProviderSet[c] = struct{}{}
		for _, t := range types {
			label := c + ":" + t
			resourceTypeSet[label] = struct{}{}
		}
	}

	for _, m := range dc.Spec.AWS {
		addMatcher(types.CloudAWS, m.Types)
	}
	for _, m := range dc.Spec.Azure {
		addMatcher(types.CloudAzure, m.Types)
	}
	for _, m := range dc.Spec.GCP {
		addMatcher(types.CloudGCP, m.Types)
	}
	for _, m := range dc.Spec.Kube {
		addMatcher(types.DiscoveredResourceKubernetes, m.Types)
	}

	return slices.Collect(maps.Keys(resourceTypeSet)), slices.Collect(maps.Keys(cloudProviderSet))
}

// MaybeDowngradeDiscoveryConfig tests the client version passed through the gRPC metadata,
// and if necessary downgrades the Discovery Config resource for compatibility with the older client.
// The following rules are applied:
//   - if version is lower than 18.5.0, the AWS wildcard region is replaced with "aws-global" sentinel region
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

var minSupportedDiscoveryConfigAWSWildcardRegionVersion = semver.Version{Major: 18, Minor: 5, Patch: 0}

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
