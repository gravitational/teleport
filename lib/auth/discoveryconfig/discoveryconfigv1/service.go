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
	"slices"
	"strings"
	"time"

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
	Backend services.DiscoveryConfigsInternal
	// StaticSnapshotBackend is the isolated backend for owner-published
	// static snapshot DiscoveryConfigs.
	StaticSnapshotBackend services.StaticSnapshotDiscoveryConfigs

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
	if s.StaticSnapshotBackend == nil {
		return trace.BadParameter("static snapshot backend is required")
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

	log             *slog.Logger
	authorizer      authz.Authorizer
	backend         services.DiscoveryConfigsInternal
	snapshotBackend services.StaticSnapshotDiscoveryConfigs
	clock           clockwork.Clock
	emitter         apievents.Emitter
	usageReporter   usagereporter.UsageReporter
}

const (
	// discoveryConfigWriteAttempts bounds every CAS retry loop on the
	// discovery config write paths. Contention per record comes from a
	// handful of periodic writers (the owner's publication and status
	// reporting for a snapshot, a discovery group's status reports for a
	// regular config), so a few backed-off retries converge; 4 keeps the
	// worst-case added wait under 2s (jitter draws total at most
	// 1*base + 2*base + 3*base = 1.8s), and exhaustion returns
	// CompareFailed for the caller's next periodic cycle to retry.
	discoveryConfigWriteAttempts = 4
	// discoveryConfigWriteBackoffBase scales the jittered linear backoff
	// before CAS retry attempt N, drawn from [0, N*base).
	discoveryConfigWriteBackoffBase = 300 * time.Millisecond
)

// NewService returns a new DiscoveryConfigs gRPC service.
func NewService(cfg ServiceConfig) (*Service, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Service{
		log:             cfg.Logger,
		authorizer:      cfg.Authorizer,
		backend:         cfg.Backend,
		snapshotBackend: cfg.StaticSnapshotBackend,
		clock:           cfg.Clock,
		emitter:         cfg.Emitter,
		usageReporter:   cfg.UsageReporter,
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

// GetDiscoveryConfig returns the named DiscoveryConfig resource: regular
// resources first, falling back to owner-published static snapshots for
// reserved snapshot names. When both exist, the grandfathered regular
// resource wins.
//
// The snapshot fallback never returns matcher inventory to a Discovery
// Service identity: a service may fetch its own snapshot inventory-stripped
// (envelope and status only, the same shape as the write echoes; see
// staticSnapshotOwnerResponse) so read-modify-write status reporting can
// merge against stored history, while every foreign snapshot name answers
// the unoccupied NotFound. Together this closes every read channel that
// could feed a snapshot's spec-carried matchers back into a service as
// dynamic configuration, without disclosing which snapshots exist.
// Clients that predate the static-snapshot subkind (or report no version)
// also receive NotFound: their unconditional validation cannot decode a
// group-less snapshot, so a successful RPC would surface as a misleading
// client-side "discovery group is missing" failure.
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
		if !trace.IsNotFound(err) || !discoveryconfig.IsReservedStaticSnapshotName(req.GetName()) {
			return nil, trace.Wrap(err)
		}
		if authz.HasBuiltinRole(*authCtx, string(types.RoleDiscovery)) {
			identity, ok := authCtx.Identity.(authz.BuiltinRole)
			if !ok || identity.GetServerID() == "" || req.GetName() != discoveryconfig.StaticSnapshotName(identity.GetServerID()) {
				return nil, trace.Wrap(err)
			}
			snapshot, snapshotErr := s.snapshotBackend.GetStaticSnapshotDiscoveryConfig(ctx, req.GetName())
			if snapshotErr != nil {
				return nil, trace.Wrap(snapshotErr)
			}
			return staticSnapshotOwnerResponse(snapshot), nil
		}
		supported, supportErr := staticSnapshotClientSupported(ctx)
		if supportErr != nil {
			return nil, trace.Wrap(supportErr)
		}
		if !supported {
			return nil, trace.Wrap(err)
		}
		snapshot, snapshotErr := s.snapshotBackend.GetStaticSnapshotDiscoveryConfig(ctx, req.GetName())
		if snapshotErr != nil {
			return nil, trace.Wrap(snapshotErr)
		}
		// Snapshots follow the same response pipeline as regular configs.
		// Every current downgrade rule targets clients older than the
		// snapshot support gate, so this is a no-op today; it keeps future
		// compatibility rules from silently applying only to regular configs.
		downgraded, downgradeErr := MaybeDowngradeDiscoveryConfig(ctx, snapshot)
		if downgradeErr != nil {
			return nil, trace.Wrap(downgradeErr)
		}
		return conv.ToProto(downgraded), nil
	}

	downgraded, err := MaybeDowngradeDiscoveryConfig(ctx, dc)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return conv.ToProto(downgraded), nil
}

// minStaticSnapshotClientVersion is the first stable release whose
// DiscoveryConfig validation accepts a group-less static snapshot spec.
// All v19 prereleases are excluded because multiple builds can report the
// same prerelease version despite straddling the introduction of the static
// snapshot contract.
//
// TODO: DELETE IN v20.0.0, when the oldest supported client is v19.
var minStaticSnapshotClientVersion = semver.Version{Major: 19}

// staticSnapshotClientSupported reports whether the calling client can decode
// a static snapshot. Clients that do not report a version are treated as too
// old: unlike resource downgrading, where serving the resource unchanged is
// the safe default, serving a possibly group-less snapshot to an unknown
// client risks a misleading client-side validation failure. A version that
// does not parse is an error, matching MaybeDowngradeDiscoveryConfig, so the
// caller surfaces it instead of folding it into a misleading NotFound.
func staticSnapshotClientSupported(ctx context.Context) (bool, error) {
	clientVersionString, ok := metadata.ClientVersionFromContext(ctx)
	if !ok {
		return false, nil
	}
	clientVersion, err := semver.NewVersion(clientVersionString)
	if err != nil {
		return false, trace.BadParameter("unrecognized client version: %s is not a valid semver", clientVersionString)
	}
	return !clientVersion.LessThan(minStaticSnapshotClientVersion), nil
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

	if err := checkReservedName(req.GetDiscoveryConfig().GetHeader().GetMetadata().GetName()); err != nil {
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

	if err := checkReservedName(req.GetDiscoveryConfig().GetHeader().GetMetadata().GetName()); err != nil {
		return nil, trace.Wrap(err)
	}
	dc, err := conv.FromProto(req.GetDiscoveryConfig())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Preserve the stored status. Static snapshots are in a different key
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

	// Owner publication routes before generic RBAC: the Discovery role
	// deliberately has no generic write verbs on the kind. Every other caller
	// keeps the regular path, which discards subkinds, so a user cannot
	// self-declare a static snapshot to bypass regular validation.
	if req.GetDiscoveryConfig().GetHeader().GetSubKind() == discoveryconfig.SubKindStaticSnapshot &&
		authz.HasBuiltinRole(*authCtx, string(types.RoleDiscovery)) {
		return s.upsertStaticSnapshot(ctx, authCtx, req.GetDiscoveryConfig())
	}

	if err := authCtx.CheckAccessToKind(types.KindDiscoveryConfig, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := checkReservedName(req.GetDiscoveryConfig().GetHeader().GetMetadata().GetName()); err != nil {
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

	s.emitUsageEvent(resp, prehogv1a.DiscoveryConfigAction_DISCOVERY_CONFIG_ACTION_CREATE)

	return conv.ToProto(resp), nil
}

// upsertStaticSnapshot publishes only the authenticated owner's observed
// inventory. Periodic machine renewals intentionally emit neither generic
// DiscoveryConfig CRUD audit events nor lifecycle usage events: like discovery
// status updates, they report observed state rather than a user configuration
// change.
//
// Only the observed spec is taken from the request. Every trusted envelope
// field (name, subkind, origin, expiry, revision) is rebuilt server-side and
// the request's status is ignored: the publisher owns the spec, status
// reporting owns the status. Publication is fail-closed: a spec still
// carrying installer params is rejected inside the constructor rather than
// silently stripped.
func (s *Service) upsertStaticSnapshot(ctx context.Context, authCtx *authz.Context, msg *discoveryconfigv1.DiscoveryConfig) (*discoveryconfigv1.DiscoveryConfig, error) {
	identity, ok := authCtx.Identity.(authz.BuiltinRole)
	if !ok || identity.GetServerID() == "" {
		return nil, trace.AccessDenied("static snapshot discovery configs may only be published by their owning Discovery Service")
	}
	if msg.GetSpec() == nil {
		return nil, trace.BadParameter("spec is missing")
	}
	serverID := identity.GetServerID()
	name := discoveryconfig.StaticSnapshotName(serverID)
	if requested := msg.GetHeader().GetMetadata().GetName(); requested != "" && requested != name {
		return nil, trace.AccessDenied("static snapshot discovery configs are owner-managed; server %q publishes %q", serverID, name)
	}
	if _, err := s.backend.GetDiscoveryConfig(ctx, name); err == nil {
		return nil, trace.AlreadyExists("regular discovery config %q occupies the static snapshot owner name", name)
	} else if !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	dc, err := discoveryconfig.NewStaticSnapshotDiscoveryConfig(serverID, conv.SpecFromProto(msg.GetSpec()))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	retry, err := s.newDiscoveryConfigWriteRetry()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for attempt := range discoveryConfigWriteAttempts {
		if err := waitForDiscoveryConfigRetry(ctx, retry, attempt); err != nil {
			return nil, err
		}
		dc.SetExpiry(s.clock.Now().Add(discoveryconfig.StaticSnapshotTTL))
		existing, err := s.snapshotBackend.GetStaticSnapshotDiscoveryConfig(ctx, name)
		if trace.IsNotFound(err) {
			// A fresh publication must not carry status merged from a record
			// observed by an earlier iteration and deleted since.
			dc.Status = discoveryconfig.Status{}
			dc.SetRevision("")
			created, err := s.snapshotBackend.CreateStaticSnapshotDiscoveryConfig(ctx, dc)
			if trace.IsAlreadyExists(err) {
				continue
			}
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return staticSnapshotOwnerResponse(created), nil
		}
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// Renewal replaces the inventory and preserves whatever status
		// reporting has stored since. The storage layer enforces the
		// stored-size cap on the merged record.
		dc.Status = existing.Status
		dc.SetRevision(existing.GetRevision())
		updated, err := s.snapshotBackend.ConditionalUpdateStaticSnapshotDiscoveryConfig(ctx, dc)
		if trace.IsCompareFailed(err) {
			continue
		}
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return staticSnapshotOwnerResponse(updated), nil
	}
	// CompareFailed matches the status loops' exhaustion category and keeps
	// LimitExceeded reserved for the stored-size cap, so a publisher can tell
	// write contention (retry soon) from an oversized record (shrink first).
	return nil, trace.CompareFailed("static snapshot discovery config write failed after %d attempts", discoveryConfigWriteAttempts)
}

// staticSnapshotOwnerResponse is the RPC echo of a snapshot write, with the
// spec inventory removed. Snapshot write responses go only to the owning
// Discovery Service, which published the data and has no read use for the
// echo; stripping it enforces the "Discovery Service identities never
// receive snapshot matchers" invariant on every channel instead of
// documenting it on the Get gate alone. The envelope (name, subkind, expiry)
// and the merged status remain for the caller's bookkeeping.
func staticSnapshotOwnerResponse(dc *discoveryconfig.DiscoveryConfig) *discoveryconfigv1.DiscoveryConfig {
	msg := conv.ToProto(dc)
	msg.SetSpec(&discoveryconfigv1.DiscoveryConfigSpec{})
	return msg
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
	// Delete is the one write verb that discloses snapshot occupancy: the
	// owner-managed AccessDenied goes only to callers holding the read verb,
	// who could learn the occupancy via Get anyway.
	if trace.IsNotFound(err) && discoveryconfig.IsReservedStaticSnapshotName(req.GetName()) {
		if ownerErr := s.snapshotOwnerManagedForReaders(ctx, authCtx, req.GetName()); ownerErr != nil {
			return nil, trace.Wrap(ownerErr)
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
	retry, err := s.newDiscoveryConfigWriteRetry()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for attempt := range discoveryConfigWriteAttempts {
		if err := waitForDiscoveryConfigRetry(ctx, retry, attempt); err != nil {
			return nil, err
		}
		dc, err := s.backend.GetDiscoveryConfig(ctx, req.GetName())
		switch {
		case trace.IsNotFound(err):
			return s.updateStaticSnapshotStatus(ctx, authCtx, req)
		case err != nil:
			return nil, trace.Wrap(err)
		}
		dc.Status = conv.StatusFromProto(req.GetStatus())
		// The revision-checked update makes the retry loop real: a stale
		// working copy must not clobber a concurrent spec edit with the
		// spec it fetched before that edit.
		resp, err := s.backend.ConditionalUpdateDiscoveryConfig(ctx, dc)
		if err != nil {
			if trace.IsCompareFailed(err) {
				continue
			}
			return nil, trace.Wrap(err)
		}

		return conv.ToProto(resp), nil
	}
	return nil, trace.CompareFailed("discovery config status update failed after %d attempts", discoveryConfigWriteAttempts)
}

// updateStaticSnapshotStatus handles the status report for a name absent from
// the regular range, mirroring upsertStaticSnapshot: the identity, ownership,
// and payload checks are loop-invariant and run once; only the read-merge-CAS
// cycle retries.
//
// Ownership is derivable from the caller's identity alone, so it is checked
// before the snapshot store is ever consulted: a foreign reserved name
// answers the same NotFound as an unoccupied one, disclosing nothing about
// which snapshots exist and costing no backend read.
func (s *Service) updateStaticSnapshotStatus(ctx context.Context, authCtx *authz.Context, req *discoveryconfigv1.UpdateDiscoveryConfigStatusRequest) (*discoveryconfigv1.DiscoveryConfig, error) {
	identity, ok := authCtx.Identity.(authz.BuiltinRole)
	if !discoveryconfig.IsReservedStaticSnapshotName(req.GetName()) ||
		!ok || identity.GetServerID() == "" ||
		req.GetName() != discoveryconfig.StaticSnapshotName(identity.GetServerID()) {
		return nil, trace.NotFound("discovery config %q not found", req.GetName())
	}
	status := conv.StatusFromProto(req.GetStatus())
	for serverID := range status.ServerStatus {
		if serverID != identity.GetServerID() {
			return nil, trace.BadParameter("static snapshot status contains foreign server %q", serverID)
		}
	}
	retry, err := s.newDiscoveryConfigWriteRetry()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for attempt := range discoveryConfigWriteAttempts {
		if err := waitForDiscoveryConfigRetry(ctx, retry, attempt); err != nil {
			return nil, err
		}
		dc, err := s.snapshotBackend.GetStaticSnapshotDiscoveryConfig(ctx, req.GetName())
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("discovery config %q not found", req.GetName())
		}
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// Reporting owns the status and must not touch the spec inventory
		// or renew the publication expiry. The storage layer enforces the
		// stored-size cap on the merged record, so an oversized report is
		// rejected here rather than persisted; if a previously accepted
		// status ever blocks a larger inventory renewal, the record
		// TTL-expires and the fresh publication (which carries no status)
		// re-establishes the inventory, after which the oversized report
		// no longer fits and keeps failing loudly.
		dc.Status = status
		resp, err := s.snapshotBackend.ConditionalUpdateStaticSnapshotDiscoveryConfig(ctx, dc)
		if trace.IsCompareFailed(err) {
			continue
		}
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return staticSnapshotOwnerResponse(resp), nil
	}
	return nil, trace.CompareFailed("static snapshot status update failed after %d attempts", discoveryConfigWriteAttempts)
}

func reservedStaticSnapshotNameError(name string) error {
	return trace.BadParameter("discovery config name %q is reserved for static snapshot discovery configs; a pre-existing config with this name remains readable and deletable but cannot be modified; recreate it under a different name to keep managing it", name)
}

// checkReservedName rejects every generic write (Create, Update, Upsert) to
// a reserved static snapshot name so get-then-mutate flows (web UI edits,
// tctl edit) fail with the ownership story rather than conversion or
// existence noise. It runs before proto conversion on purpose: the regular
// write path discards subkinds, so a round-tripped group-less snapshot would
// otherwise fail validation with a misleading "discovery group is missing".
//
// The check is a pure name-shape test that reads nothing from the backend:
// the response is identical whether the name is occupied by a grandfathered
// regular config, a snapshot, or nothing, so the write verbs disclose no
// occupancy information to any caller. Snapshot-occupancy disclosure lives
// only in DeleteDiscoveryConfig, via snapshotOwnerManagedForReaders.
func checkReservedName(name string) error {
	if discoveryconfig.IsReservedStaticSnapshotName(name) {
		return reservedStaticSnapshotNameError(name)
	}
	return nil
}

// snapshotOwnerManagedForReaders returns the owner-managed AccessDenied when
// a snapshot occupies name and the caller holds the read verb; callers
// without it learn nothing (nil), so deletes of names they cannot read keep
// the unoccupied-name NotFound and disclose no snapshot existence.
func (s *Service) snapshotOwnerManagedForReaders(ctx context.Context, authCtx *authz.Context, name string) error {
	if authCtx.CheckAccessToKind(types.KindDiscoveryConfig, types.VerbRead) != nil {
		return nil
	}
	if _, err := s.snapshotBackend.GetStaticSnapshotDiscoveryConfig(ctx, name); err == nil {
		return trace.AccessDenied("static snapshot discovery config %q is owner-managed", name)
	} else if !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	return nil
}

// newDiscoveryConfigWriteRetry builds the jittered linear backoff shared by
// the discovery config CAS write loops: retry attempt N waits a duration
// drawn from [0, N*discoveryConfigWriteBackoffBase).
func (s *Service) newDiscoveryConfigWriteRetry() (*retryutils.RetryV2, error) {
	retry, err := retryutils.NewRetryV2(retryutils.RetryV2Config{
		Driver: retryutils.NewLinearDriver(discoveryConfigWriteBackoffBase),
		Max:    (discoveryConfigWriteAttempts - 1) * discoveryConfigWriteBackoffBase,
		Jitter: retryutils.FullJitter,
		Clock:  s.clock,
	})
	return retry, trace.Wrap(err)
}

// waitForDiscoveryConfigRetry waits for the next CAS retry slot, honoring
// context cancellation. The first attempt proceeds without waiting.
func waitForDiscoveryConfigRetry(ctx context.Context, retry *retryutils.RetryV2, attempt int) error {
	if attempt == 0 {
		return nil
	}
	retry.Inc()
	select {
	case <-ctx.Done():
		return trace.Wrap(ctx.Err())
	case <-retry.After():
	}
	return nil
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
