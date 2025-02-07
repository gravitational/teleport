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

package clusterconfigv1

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"

	clusterconfigpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/clusterconfig"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/authz"
	dtconfig "github.com/gravitational/teleport/lib/devicetrust/config"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/readonly"
)

// Cache is used by the [Service] to query cluster config resources.
type Cache interface {
	GetAuthPreference(context.Context) (types.AuthPreference, error)
	GetClusterNetworkingConfig(ctx context.Context) (types.ClusterNetworkingConfig, error)
	GetSessionRecordingConfig(ctx context.Context) (types.SessionRecordingConfig, error)
	GetAccessGraphSettings(context.Context) (*clusterconfigpb.AccessGraphSettings, error)
}

// ReadOnlyCache abstracts over the required methods of [readonly.Cache].
type ReadOnlyCache interface {
	GetReadOnlyAuthPreference(context.Context) (readonly.AuthPreference, error)
	GetReadOnlyClusterNetworkingConfig(ctx context.Context) (readonly.ClusterNetworkingConfig, error)
	GetReadOnlySessionRecordingConfig(ctx context.Context) (readonly.SessionRecordingConfig, error)
	GetReadOnlyAccessGraphSettings(context.Context) (readonly.AccessGraphSettings, error)
}

// Backend is used by the [Service] to mutate cluster config resources.
type Backend interface {
	CreateAuthPreference(ctx context.Context, preference types.AuthPreference) (types.AuthPreference, error)
	UpdateAuthPreference(ctx context.Context, preference types.AuthPreference) (types.AuthPreference, error)
	UpsertAuthPreference(ctx context.Context, preference types.AuthPreference) (types.AuthPreference, error)

	CreateClusterNetworkingConfig(ctx context.Context, preference types.ClusterNetworkingConfig) (types.ClusterNetworkingConfig, error)
	UpdateClusterNetworkingConfig(ctx context.Context, preference types.ClusterNetworkingConfig) (types.ClusterNetworkingConfig, error)
	UpsertClusterNetworkingConfig(ctx context.Context, preference types.ClusterNetworkingConfig) (types.ClusterNetworkingConfig, error)

	CreateSessionRecordingConfig(ctx context.Context, preference types.SessionRecordingConfig) (types.SessionRecordingConfig, error)
	UpdateSessionRecordingConfig(ctx context.Context, preference types.SessionRecordingConfig) (types.SessionRecordingConfig, error)
	UpsertSessionRecordingConfig(ctx context.Context, preference types.SessionRecordingConfig) (types.SessionRecordingConfig, error)

	CreateAccessGraphSettings(context.Context, *clusterconfigpb.AccessGraphSettings) (*clusterconfigpb.AccessGraphSettings, error)
	UpdateAccessGraphSettings(context.Context, *clusterconfigpb.AccessGraphSettings) (*clusterconfigpb.AccessGraphSettings, error)
	UpsertAccessGraphSettings(context.Context, *clusterconfigpb.AccessGraphSettings) (*clusterconfigpb.AccessGraphSettings, error)
}

// ServiceConfig contain dependencies required to create a [Service].
type ServiceConfig struct {
	Cache                         Cache
	Backend                       Backend
	Authorizer                    authz.Authorizer
	Emitter                       apievents.Emitter
	AccessGraph                   AccessGraphConfig
	ReadOnlyCache                 ReadOnlyCache
	SignatureAlgorithmSuiteParams types.SignatureAlgorithmSuiteParams
}

// AccessGraphConfig contains the configuration about the access graph service
// and whether it is enabled or not.
type AccessGraphConfig struct {
	// Enabled is a flag that indicates whether the access graph service is enabled.
	Enabled bool
	// Address is the address of the access graph service. The address is in the
	// form of "host:port".
	Address string
	// CA is the PEM-encoded CA certificate of the access graph service.
	CA []byte
	// Insecure is a flag that indicates whether the access graph service should
	// skip verifying the server's certificate chain and host name.
	Insecure bool
}

// Service implements the teleport.clusterconfig.v1.ClusterConfigService RPC service.
type Service struct {
	clusterconfigpb.UnimplementedClusterConfigServiceServer

	cache                         Cache
	backend                       Backend
	authorizer                    authz.Authorizer
	emitter                       apievents.Emitter
	accessGraph                   AccessGraphConfig
	readOnlyCache                 ReadOnlyCache
	signatureAlgorithmSuiteParams types.SignatureAlgorithmSuiteParams
}

// NewService validates the provided configuration and returns a [Service].
func NewService(cfg ServiceConfig) (*Service, error) {
	switch {
	case cfg.Cache == nil:
		return nil, trace.BadParameter("cache service is required")
	case cfg.Backend == nil:
		return nil, trace.BadParameter("backend service is required")
	case cfg.Authorizer == nil:
		return nil, trace.BadParameter("authorizer is required")
	case cfg.Emitter == nil:
		return nil, trace.BadParameter("emitter is required")
	}

	if cfg.ReadOnlyCache == nil {
		readOnlyCache, err := readonly.NewCache(readonly.CacheConfig{
			Upstream: cfg.Cache,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		cfg.ReadOnlyCache = readOnlyCache
	}

	return &Service{
		cache:                         cfg.Cache,
		backend:                       cfg.Backend,
		authorizer:                    cfg.Authorizer,
		emitter:                       cfg.Emitter,
		accessGraph:                   cfg.AccessGraph,
		readOnlyCache:                 cfg.ReadOnlyCache,
		signatureAlgorithmSuiteParams: cfg.SignatureAlgorithmSuiteParams,
	}, nil
}

// GetAuthPreference returns the locally cached auth preference.
func (s *Service) GetAuthPreference(ctx context.Context, _ *clusterconfigpb.GetAuthPreferenceRequest) (*types.AuthPreferenceV2, error) {
	authzCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authzCtx.CheckAccessToKind(types.KindClusterAuthPreference, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	pref, err := s.readOnlyCache.GetReadOnlyAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authPrefV2, ok := pref.Clone().(*types.AuthPreferenceV2)
	if !ok {
		return nil, trace.Wrap(trace.BadParameter("unexpected auth preference type %T (expected %T)", pref, authPrefV2))
	}

	return authPrefV2, nil
}

// CreateAuthPreference creates a new auth preference if one does not exist. This
// is an internal API and is not exposed via [clusterconfigv1.ClusterConfigServiceServer]. It
// is only meant to be called directly from the first time an Auth instance is started.
func (s *Service) CreateAuthPreference(ctx context.Context, p types.AuthPreference) (types.AuthPreference, error) {
	authzCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authzCtx, string(types.RoleAuth)) {
		return nil, trace.AccessDenied("this request can be only executed by an auth server")
	}

	if err := services.ValidateAuthPreference(p); err != nil {
		return nil, trace.Wrap(err)
	}

	// check that the given RequireMFAType is supported in this build.
	if p.GetPrivateKeyPolicy().IsHardwareKeyPolicy() && modules.GetModules().BuildType() != modules.BuildEnterprise {
		return nil, trace.AccessDenied("Hardware Key support is only available with an enterprise license")
	}

	if err := dtconfig.ValidateConfigAgainstModules(p.GetDeviceTrust()); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := p.CheckSignatureAlgorithmSuite(s.signatureAlgorithmSuiteParams); err != nil {
		return nil, trace.Wrap(err)
	}

	created, err := s.backend.CreateAuthPreference(ctx, p)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authPrefV2, ok := created.(*types.AuthPreferenceV2)
	if !ok {
		return nil, trace.Wrap(trace.BadParameter("unexpected auth preference type %T (expected %T)", created, authPrefV2))
	}

	return authPrefV2, nil
}

func eventStatus(err error) apievents.Status {
	var msg string
	if err != nil {
		msg = err.Error()
	}

	return apievents.Status{
		Success:     err == nil,
		Error:       msg,
		UserMessage: msg,
	}
}

// UpdateAuthPreference conditionally updates an existing auth preference if the value
// wasn't concurrently modified.
func (s *Service) UpdateAuthPreference(ctx context.Context, req *clusterconfigpb.UpdateAuthPreferenceRequest) (*types.AuthPreferenceV2, error) {
	authzCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authzCtx.CheckAccessToKind(types.KindClusterAuthPreference, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authzCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := services.ValidateAuthPreference(req.GetAuthPreference()); err != nil {
		return nil, trace.Wrap(err)
	}

	// check that the given RequireMFAType is supported in this build.
	if req.AuthPreference.GetPrivateKeyPolicy().IsHardwareKeyPolicy() && modules.GetModules().BuildType() != modules.BuildEnterprise {
		return nil, trace.AccessDenied("Hardware Key support is only available with an enterprise license")
	}

	if err := dtconfig.ValidateConfigAgainstModules(req.AuthPreference.GetDeviceTrust()); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := req.AuthPreference.CheckSignatureAlgorithmSuite(s.signatureAlgorithmSuiteParams); err != nil {
		return nil, trace.Wrap(err)
	}

	req.AuthPreference.SetOrigin(types.OriginDynamic)

	original, err := s.cache.GetAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	updated, err := s.backend.UpdateAuthPreference(ctx, req.AuthPreference)

	if auditErr := s.emitter.EmitAuditEvent(ctx, &apievents.AuthPreferenceUpdate{
		Metadata: apievents.Metadata{
			Type: events.AuthPreferenceUpdateEvent,
			Code: events.AuthPreferenceUpdateCode,
		},
		UserMetadata:       authzCtx.GetUserMetadata(),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status:             eventStatus(err),
		AdminActionsMFA:    GetAdminActionsMFAStatus(original, req.AuthPreference),
	}); auditErr != nil {
		slog.WarnContext(ctx, "Failed to emit auth preference update event event.", "error", auditErr)
	}

	// don't handle the update error until after we emit an audit event
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authPrefV2, ok := updated.(*types.AuthPreferenceV2)
	if !ok {
		return nil, trace.Wrap(trace.BadParameter("unexpected auth preference type %T (expected %T)", updated, authPrefV2))
	}

	return authPrefV2, nil
}

// UpsertAuthPreference creates a new auth preference or overwrites an existing auth preference.
func (s *Service) UpsertAuthPreference(ctx context.Context, req *clusterconfigpb.UpsertAuthPreferenceRequest) (*types.AuthPreferenceV2, error) {
	authzCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authzCtx.CheckAccessToKind(types.KindClusterAuthPreference, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	// Support reused MFA for bulk tctl create requests.
	if err := authzCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := services.ValidateAuthPreference(req.GetAuthPreference()); err != nil {
		return nil, trace.Wrap(err)
	}

	// check that the given RequireMFAType is supported in this build.
	if req.AuthPreference.GetPrivateKeyPolicy().IsHardwareKeyPolicy() && modules.GetModules().BuildType() != modules.BuildEnterprise {
		return nil, trace.AccessDenied("Hardware Key support is only available with an enterprise license")
	}

	if err := dtconfig.ValidateConfigAgainstModules(req.AuthPreference.GetDeviceTrust()); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := req.AuthPreference.CheckSignatureAlgorithmSuite(s.signatureAlgorithmSuiteParams); err != nil {
		return nil, trace.Wrap(err)
	}

	req.AuthPreference.SetOrigin(types.OriginDynamic)

	original, err := s.cache.GetAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	updated, err := s.backend.UpsertAuthPreference(ctx, req.AuthPreference)

	if auditErr := s.emitter.EmitAuditEvent(ctx, &apievents.AuthPreferenceUpdate{
		Metadata: apievents.Metadata{
			Type: events.AuthPreferenceUpdateEvent,
			Code: events.AuthPreferenceUpdateCode,
		},
		UserMetadata:       authzCtx.GetUserMetadata(),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status:             eventStatus(err),
		AdminActionsMFA:    GetAdminActionsMFAStatus(original, req.AuthPreference),
	}); auditErr != nil {
		slog.WarnContext(ctx, "Failed to emit auth preference update event event.", "error", auditErr)
	}

	// don't handle the update error until after we emit an audit event
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authPrefV2, ok := updated.(*types.AuthPreferenceV2)
	if !ok {
		return nil, trace.Wrap(trace.BadParameter("unexpected auth preference type %T (expected %T)", updated, authPrefV2))
	}

	return authPrefV2, nil
}

// ResetAuthPreference restores the auth preferences to the default value.
func (s *Service) ResetAuthPreference(ctx context.Context, _ *clusterconfigpb.ResetAuthPreferenceRequest) (*types.AuthPreferenceV2, error) {
	authzCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authzCtx.CheckAccessToKind(types.KindClusterAuthPreference, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authzCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	defaultPreference := types.DefaultAuthPreference()
	defaultPreference.SetDefaultSignatureAlgorithmSuite(s.signatureAlgorithmSuiteParams)
	const iterationLimit = 3
	// Attempt a few iterations in case the conditional update fails
	// due to spurious networking conditions.
	for i := 0; i < iterationLimit; i++ {
		pref, err := s.cache.GetAuthPreference(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if pref.Origin() == types.OriginConfigFile {
			return nil, trace.BadParameter("auth preference has been defined via the config file and cannot be reset back to defaults dynamically.")
		}

		defaultPreference.SetRevision(pref.GetRevision())

		reset, err := s.backend.UpdateAuthPreference(ctx, defaultPreference)
		if trace.IsCompareFailed(err) {
			continue
		}

		if auditErr := s.emitter.EmitAuditEvent(ctx, &apievents.AuthPreferenceUpdate{
			Metadata: apievents.Metadata{
				Type: events.AuthPreferenceUpdateEvent,
				Code: events.AuthPreferenceUpdateCode,
			},
			UserMetadata:       authzCtx.GetUserMetadata(),
			ConnectionMetadata: authz.ConnectionMetadata(ctx),
			Status:             eventStatus(err),
			AdminActionsMFA:    GetAdminActionsMFAStatus(pref, defaultPreference),
		}); auditErr != nil {
			slog.WarnContext(ctx, "Failed to emit auth preference update event event.", "error", auditErr)
		}

		// don't handle the update error until after we emit an audit event
		if err != nil {
			return nil, trace.Wrap(err)
		}

		authPrefV2, ok := reset.(*types.AuthPreferenceV2)
		if !ok {
			return nil, trace.Wrap(trace.BadParameter("unexpected auth preference type %T (expected %T)", reset, authPrefV2))
		}

		return authPrefV2, nil
	}

	return nil, trace.LimitExceeded("failed to reset networking config within %v iterations", iterationLimit)
}

// GetAdminActionsMFAStatus returns whether MFA for admin actions was
// altered when the auth preferences were updated.
func GetAdminActionsMFAStatus(oldPref, newPref types.AuthPreference) apievents.AdminActionsMFAStatus {
	if oldPref.IsAdminActionMFAEnforced() == newPref.IsAdminActionMFAEnforced() {
		return apievents.AdminActionsMFAStatus_ADMIN_ACTIONS_MFA_STATUS_UNCHANGED
	}
	if newPref.IsAdminActionMFAEnforced() {
		return apievents.AdminActionsMFAStatus_ADMIN_ACTIONS_MFA_STATUS_ENABLED
	}
	return apievents.AdminActionsMFAStatus_ADMIN_ACTIONS_MFA_STATUS_DISABLED
}

// GetClusterNetworkingConfig returns the locally cached networking configuration.
func (s *Service) GetClusterNetworkingConfig(ctx context.Context, _ *clusterconfigpb.GetClusterNetworkingConfigRequest) (*types.ClusterNetworkingConfigV2, error) {
	authzCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authzCtx.CheckAccessToKind(types.KindClusterNetworkingConfig, types.VerbRead); err != nil {
		if err2 := authzCtx.CheckAccessToKind(types.KindClusterConfig, types.VerbRead); err2 != nil {
			return nil, trace.Wrap(err)
		}
	}

	netConfig, err := s.readOnlyCache.GetReadOnlyClusterNetworkingConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfgV2, ok := netConfig.Clone().(*types.ClusterNetworkingConfigV2)
	if !ok {
		return nil, trace.Wrap(trace.BadParameter("unexpected cluster networking config type %T (expected %T)", netConfig, cfgV2))
	}
	return cfgV2, nil
}

// CreateClusterNetworkingConfig creates a new cluster networking configuration if one does not exist.
// This is an internal API and is not exposed via [clusterconfigv1.ClusterConfigServiceServer]. It
// is only meant to be called directly from the first time an Auth instance is started.
func (s *Service) CreateClusterNetworkingConfig(ctx context.Context, cfg types.ClusterNetworkingConfig) (types.ClusterNetworkingConfig, error) {
	authzCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authzCtx, string(types.RoleAuth)) {
		return nil, trace.AccessDenied("this request can be only executed by an auth server")
	}

	tst, err := cfg.GetTunnelStrategyType()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if tst == types.ProxyPeering && modules.GetModules().BuildType() != modules.BuildEnterprise {
		return nil, trace.AccessDenied("proxy peering is an enterprise-only feature")
	}

	created, err := s.backend.CreateClusterNetworkingConfig(ctx, cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfgV2, ok := created.(*types.ClusterNetworkingConfigV2)
	if !ok {
		return nil, trace.Wrap(trace.BadParameter("unexpected cluster networking config type %T (expected %T)", created, cfgV2))
	}

	return cfgV2, nil
}

// UpdateClusterNetworkingConfig conditionally updates an existing networking configuration if the
// value wasn't concurrently modified.
func (s *Service) UpdateClusterNetworkingConfig(ctx context.Context, req *clusterconfigpb.UpdateClusterNetworkingConfigRequest) (*types.ClusterNetworkingConfigV2, error) {
	authzCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authzCtx.CheckAccessToKind(types.KindClusterNetworkingConfig, types.VerbUpdate); err != nil {
		if err2 := authzCtx.CheckAccessToKind(types.KindClusterConfig, types.VerbUpdate); err2 != nil {
			return nil, trace.Wrap(err)
		}
	}

	// Support reused MFA for bulk tctl create requests.
	if err := authzCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	tst, err := req.ClusterNetworkConfig.GetTunnelStrategyType()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if tst == types.ProxyPeering && modules.GetModules().BuildType() != modules.BuildEnterprise {
		return nil, trace.AccessDenied("proxy peering is an enterprise-only feature")
	}

	req.ClusterNetworkConfig.SetOrigin(types.OriginDynamic)

	oldCfg, err := s.cache.GetClusterNetworkingConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	newCfg := req.GetClusterNetworkConfig()

	if err := ValidateCloudNetworkConfigUpdate(*authzCtx, newCfg, oldCfg); err != nil {
		return nil, trace.Wrap(err)
	}

	updated, err := s.backend.UpdateClusterNetworkingConfig(ctx, req.ClusterNetworkConfig)

	if err := s.emitter.EmitAuditEvent(ctx, &apievents.ClusterNetworkingConfigUpdate{
		Metadata: apievents.Metadata{
			Type: events.ClusterNetworkingConfigUpdateEvent,
			Code: events.ClusterNetworkingConfigUpdateCode,
		},
		UserMetadata:       authzCtx.GetUserMetadata(),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status:             eventStatus(err),
	}); err != nil {
		slog.WarnContext(ctx, "Failed to emit cluster networking config update event event.", "error", err)
	}

	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfgV2, ok := updated.(*types.ClusterNetworkingConfigV2)
	if !ok {
		return nil, trace.Wrap(trace.BadParameter("unexpected cluster networking config type %T (expected %T)", updated, cfgV2))
	}

	return cfgV2, nil
}

// UpsertClusterNetworkingConfig creates a new networking configuration or overwrites an existing configuration.
func (s *Service) UpsertClusterNetworkingConfig(ctx context.Context, req *clusterconfigpb.UpsertClusterNetworkingConfigRequest) (*types.ClusterNetworkingConfigV2, error) {
	authzCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authzCtx.CheckAccessToKind(types.KindClusterNetworkingConfig, types.VerbCreate, types.VerbUpdate); err != nil {
		if err2 := authzCtx.CheckAccessToKind(types.KindClusterConfig, types.VerbCreate, types.VerbUpdate); err2 != nil {
			return nil, trace.Wrap(err)
		}
	}

	// Support reused MFA for bulk tctl create requests.
	if err := authzCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	tst, err := req.ClusterNetworkConfig.GetTunnelStrategyType()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if tst == types.ProxyPeering && modules.GetModules().BuildType() != modules.BuildEnterprise {
		return nil, trace.AccessDenied("proxy peering is an enterprise-only feature")
	}

	req.ClusterNetworkConfig.SetOrigin(types.OriginDynamic)

	oldCfg, err := s.cache.GetClusterNetworkingConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	newCfg := req.GetClusterNetworkConfig()

	if err := ValidateCloudNetworkConfigUpdate(*authzCtx, newCfg, oldCfg); err != nil {
		return nil, trace.Wrap(err)
	}

	upserted, err := s.backend.UpsertClusterNetworkingConfig(ctx, newCfg)

	if err := s.emitter.EmitAuditEvent(ctx, &apievents.ClusterNetworkingConfigUpdate{
		Metadata: apievents.Metadata{
			Type: events.ClusterNetworkingConfigUpdateEvent,
			Code: events.ClusterNetworkingConfigUpdateCode,
		},
		UserMetadata:       authzCtx.GetUserMetadata(),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status:             eventStatus(err),
	}); err != nil {
		slog.WarnContext(ctx, "Failed to emit cluster networking config update event event.", "error", err)
	}

	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfgV2, ok := upserted.(*types.ClusterNetworkingConfigV2)
	if !ok {
		return nil, trace.Wrap(trace.BadParameter("unexpected cluster networking config type %T (expected %T)", upserted, cfgV2))
	}

	return cfgV2, nil
}

// ResetClusterNetworkingConfig restores the networking configuration to the default value.
func (s *Service) ResetClusterNetworkingConfig(ctx context.Context, _ *clusterconfigpb.ResetClusterNetworkingConfigRequest) (*types.ClusterNetworkingConfigV2, error) {
	authzCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authzCtx.CheckAccessToKind(types.KindClusterNetworkingConfig, types.VerbUpdate); err != nil {
		if err2 := authzCtx.CheckAccessToKind(types.KindClusterConfig, types.VerbUpdate); err2 != nil {
			return nil, trace.Wrap(err)
		}
	}

	if err := authzCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	defaultConfig := types.DefaultClusterNetworkingConfig()

	oldCfg, err := s.cache.GetClusterNetworkingConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := ValidateCloudNetworkConfigUpdate(*authzCtx, defaultConfig, oldCfg); err != nil {
		return nil, trace.Wrap(err)
	}

	const iterationLimit = 3
	// Attempt a few iterations in case the conditional update fails
	// due to spurious networking conditions.
	for i := 0; i < iterationLimit; i++ {
		cfg, err := s.cache.GetClusterNetworkingConfig(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if cfg.Origin() == types.OriginConfigFile {
			return nil, trace.BadParameter("cluster networking configuration has been defined in the auth server's config file and cannot be set back to defaults dynamically.")
		}

		defaultConfig.SetRevision(cfg.GetRevision())

		reset, err := s.backend.UpdateClusterNetworkingConfig(ctx, defaultConfig)
		if trace.IsCompareFailed(err) {
			continue
		}

		if err := s.emitter.EmitAuditEvent(ctx, &apievents.ClusterNetworkingConfigUpdate{
			Metadata: apievents.Metadata{
				Type: events.ClusterNetworkingConfigUpdateEvent,
				Code: events.ClusterNetworkingConfigUpdateCode,
			},
			UserMetadata:       authzCtx.GetUserMetadata(),
			ConnectionMetadata: authz.ConnectionMetadata(ctx),
			Status:             eventStatus(err),
		}); err != nil {
			slog.WarnContext(ctx, "Failed to emit cluster networking config update event event.", "error", err)
		}

		if err != nil {
			return nil, trace.Wrap(err)
		}

		cfgV2, ok := reset.(*types.ClusterNetworkingConfigV2)
		if !ok {
			return nil, trace.Wrap(trace.BadParameter("unexpected cluster networking config type %T (expected %T)", reset, cfgV2))
		}

		return cfgV2, nil
	}

	return nil, trace.LimitExceeded("failed to reset networking config within %v iterations", iterationLimit)
}

// ValidateCloudNetworkConfigUpdate validates that that [newConfig] is a valid update of [oldConfig]. Cloud
// customers are not allowed to edit certain fields of the cluster networking config, and even if they were,
// the edits would be overwritten by the values from the static config file every time an auth process starts
// up.
func ValidateCloudNetworkConfigUpdate(authzCtx authz.Context, newConfig, oldConfig types.ClusterNetworkingConfig) error {
	if authz.HasBuiltinRole(authzCtx, string(types.RoleAdmin)) {
		return nil
	}

	if !modules.GetModules().Features().Cloud {
		return nil
	}

	const cloudUpdateFailureMsg = "cloud tenants cannot update %q"

	if newConfig.GetProxyListenerMode() != oldConfig.GetProxyListenerMode() {
		return trace.BadParameter(cloudUpdateFailureMsg, "proxy_listener_mode")
	}
	newtst, _ := newConfig.GetTunnelStrategyType()
	oldtst, _ := oldConfig.GetTunnelStrategyType()
	if newtst != oldtst {
		return trace.BadParameter(cloudUpdateFailureMsg, "tunnel_strategy")
	}

	if newConfig.GetKeepAliveInterval() != oldConfig.GetKeepAliveInterval() {
		return trace.BadParameter(cloudUpdateFailureMsg, "keep_alive_interval")
	}

	if newConfig.GetKeepAliveCountMax() != oldConfig.GetKeepAliveCountMax() {
		return trace.BadParameter(cloudUpdateFailureMsg, "keep_alive_count_max")
	}

	return nil
}

// GetSessionRecordingConfig returns the locally cached networking configuration.
func (s *Service) GetSessionRecordingConfig(ctx context.Context, _ *clusterconfigpb.GetSessionRecordingConfigRequest) (*types.SessionRecordingConfigV2, error) {
	authzCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authzCtx.CheckAccessToKind(types.KindSessionRecordingConfig, types.VerbRead); err != nil {
		if err2 := authzCtx.CheckAccessToKind(types.KindClusterConfig, types.VerbRead); err2 != nil {
			return nil, trace.Wrap(err)
		}
	}

	netConfig, err := s.readOnlyCache.GetReadOnlySessionRecordingConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfgV2, ok := netConfig.Clone().(*types.SessionRecordingConfigV2)
	if !ok {
		return nil, trace.Wrap(trace.BadParameter("unexpected session recording config type %T (expected %T)", netConfig, cfgV2))
	}
	return cfgV2, nil
}

// CreateSessionRecordingConfig creates a new cluster networking configuration if one does not exist.
// This is an internal API and is not exposed via [clusterconfigv1.ClusterConfigServiceServer]. It
// is only meant to be called directly from the first time an Auth instance is started.
func (s *Service) CreateSessionRecordingConfig(ctx context.Context, cfg types.SessionRecordingConfig) (types.SessionRecordingConfig, error) {
	authzCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authzCtx, string(types.RoleAuth)) {
		return nil, trace.AccessDenied("this request can be only executed by an auth server")
	}

	created, err := s.backend.CreateSessionRecordingConfig(ctx, cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfgV2, ok := created.(*types.SessionRecordingConfigV2)
	if !ok {
		return nil, trace.Wrap(trace.BadParameter("unexpected session recording config type %T (expected %T)", created, cfgV2))
	}

	return cfgV2, nil
}

// UpdateSessionRecordingConfig conditionally updates an existing networking configuration if the
// value wasn't concurrently modified.
func (s *Service) UpdateSessionRecordingConfig(ctx context.Context, req *clusterconfigpb.UpdateSessionRecordingConfigRequest) (*types.SessionRecordingConfigV2, error) {
	authzCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authzCtx.CheckAccessToKind(types.KindSessionRecordingConfig, types.VerbUpdate); err != nil {
		if err2 := authzCtx.CheckAccessToKind(types.KindClusterConfig, types.VerbUpdate); err2 != nil {
			return nil, trace.Wrap(err)
		}
	}

	// Support reused MFA for bulk tctl create requests.
	if err := authzCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	req.SessionRecordingConfig.SetOrigin(types.OriginDynamic)

	updated, err := s.backend.UpdateSessionRecordingConfig(ctx, req.SessionRecordingConfig)

	if err := s.emitter.EmitAuditEvent(ctx, &apievents.SessionRecordingConfigUpdate{
		Metadata: apievents.Metadata{
			Type: events.SessionRecordingConfigUpdateEvent,
			Code: events.SessionRecordingConfigUpdateCode,
		},
		UserMetadata:       authzCtx.GetUserMetadata(),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status:             eventStatus(err),
	}); err != nil {
		slog.WarnContext(ctx, "Failed to emit session recording config update event event.", "error", err)
	}

	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfgV2, ok := updated.(*types.SessionRecordingConfigV2)
	if !ok {
		return nil, trace.Wrap(trace.BadParameter("unexpected session recording config type %T (expected %T)", updated, cfgV2))
	}

	return cfgV2, nil
}

// UpsertSessionRecordingConfig creates a new networking configuration or overwrites an existing configuration.
func (s *Service) UpsertSessionRecordingConfig(ctx context.Context, req *clusterconfigpb.UpsertSessionRecordingConfigRequest) (*types.SessionRecordingConfigV2, error) {
	authzCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authzCtx.CheckAccessToKind(types.KindSessionRecordingConfig, types.VerbCreate, types.VerbUpdate); err != nil {
		if err2 := authzCtx.CheckAccessToKind(types.KindClusterConfig, types.VerbCreate, types.VerbUpdate); err2 != nil {
			return nil, trace.Wrap(err)
		}
	}

	// Support reused MFA for bulk tctl create requests.
	if err := authzCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	req.SessionRecordingConfig.SetOrigin(types.OriginDynamic)

	upserted, err := s.backend.UpsertSessionRecordingConfig(ctx, req.SessionRecordingConfig)

	if err := s.emitter.EmitAuditEvent(ctx, &apievents.SessionRecordingConfigUpdate{
		Metadata: apievents.Metadata{
			Type: events.SessionRecordingConfigUpdateEvent,
			Code: events.SessionRecordingConfigUpdateCode,
		},
		UserMetadata:       authzCtx.GetUserMetadata(),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status:             eventStatus(err),
	}); err != nil {
		slog.WarnContext(ctx, "Failed to emit session recording config update event event.", "error", err)
	}

	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfgV2, ok := upserted.(*types.SessionRecordingConfigV2)
	if !ok {
		return nil, trace.Wrap(trace.BadParameter("unexpected session recording config type %T (expected %T)", upserted, cfgV2))
	}

	return cfgV2, nil
}

// ResetSessionRecordingConfig restores the networking configuration to the default value.
func (s *Service) ResetSessionRecordingConfig(ctx context.Context, _ *clusterconfigpb.ResetSessionRecordingConfigRequest) (*types.SessionRecordingConfigV2, error) {
	authzCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authzCtx.CheckAccessToKind(types.KindSessionRecordingConfig, types.VerbUpdate); err != nil {
		if err2 := authzCtx.CheckAccessToKind(types.KindClusterConfig, types.VerbUpdate); err2 != nil {
			return nil, trace.Wrap(err)
		}
	}

	if err := authzCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}
	defaultConfig := types.DefaultSessionRecordingConfig()
	const iterationLimit = 3
	// Attempt a few iterations in case the conditional update fails
	// due to spurious networking conditions.
	for i := 0; i < iterationLimit; i++ {

		cfg, err := s.cache.GetSessionRecordingConfig(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if cfg.Origin() == types.OriginConfigFile {
			return nil, trace.BadParameter("session recording configuration has been defined in the auth server's config file and cannot be set back to defaults dynamically.")
		}

		defaultConfig.SetRevision(cfg.GetRevision())

		reset, err := s.backend.UpsertSessionRecordingConfig(ctx, defaultConfig)
		if trace.IsCompareFailed(err) {
			continue
		}

		if err := s.emitter.EmitAuditEvent(ctx, &apievents.SessionRecordingConfigUpdate{
			Metadata: apievents.Metadata{
				Type: events.SessionRecordingConfigUpdateEvent,
				Code: events.SessionRecordingConfigUpdateCode,
			},
			UserMetadata:       authzCtx.GetUserMetadata(),
			ConnectionMetadata: authz.ConnectionMetadata(ctx),
			Status:             eventStatus(err),
		}); err != nil {
			slog.WarnContext(ctx, "Failed to emit session recording config update event event.", "error", err)
		}

		if err != nil {
			return nil, trace.Wrap(err)
		}

		cfgV2, ok := reset.(*types.SessionRecordingConfigV2)
		if !ok {
			return nil, trace.Wrap(trace.BadParameter("unexpected session recording config type %T (expected %T)", reset, cfgV2))
		}

		return cfgV2, nil
	}
	return nil, trace.LimitExceeded("failed to reset networking config within %v iterations", iterationLimit)
}

func (s *Service) GetClusterAccessGraphConfig(ctx context.Context, _ *clusterconfigpb.GetClusterAccessGraphConfigRequest) (*clusterconfigpb.GetClusterAccessGraphConfigResponse, error) {
	authzCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.IsLocalOrRemoteService(*authzCtx) {
		return nil, trace.AccessDenied("this request can be only executed by a Teleport service")
	}

	// If the policy feature is disabled in the license, return a disabled response.
	if !modules.GetModules().Features().GetEntitlement(entitlements.Policy).Enabled && !modules.GetModules().Features().AccessGraph {
		return &clusterconfigpb.GetClusterAccessGraphConfigResponse{
			AccessGraph: &clusterconfigpb.AccessGraphConfig{
				Enabled: false,
			},
		}, nil
	}

	var sshScanEnabled bool
	switch obj, err := s.readOnlyCache.GetReadOnlyAccessGraphSettings(ctx); {
	case err != nil && !trace.IsNotFound(err):
		return nil, trace.Wrap(err)
	case err == nil:
		sshScanEnabled = obj.SecretsScanConfig() == clusterconfigpb.AccessGraphSecretsScanConfig_ACCESS_GRAPH_SECRETS_SCAN_CONFIG_ENABLED
	}

	return &clusterconfigpb.GetClusterAccessGraphConfigResponse{
		AccessGraph: &clusterconfigpb.AccessGraphConfig{
			Enabled:  s.accessGraph.Enabled,
			Address:  s.accessGraph.Address,
			Ca:       s.accessGraph.CA,
			Insecure: s.accessGraph.Insecure,
			SecretsScanConfig: &clusterconfigpb.AccessGraphSecretsScanConfiguration{
				SshScanEnabled: sshScanEnabled,
			},
		},
	}, nil
}

func (s *Service) GetAccessGraphSettings(ctx context.Context, _ *clusterconfigpb.GetAccessGraphSettingsRequest) (*clusterconfigpb.AccessGraphSettings, error) {
	authzCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authzCtx.CheckAccessToKind(types.KindAccessGraphSettings, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := s.readOnlyCache.GetReadOnlyAccessGraphSettings(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return cfg.Clone(), nil
}

func (s *Service) CreateAccessGraphSettings(ctx context.Context, req *clusterconfigpb.CreateAccessGraphSettingsRequest) (*clusterconfigpb.AccessGraphSettings, error) {
	authzCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authzCtx.CheckAccessToKind(types.KindAccessGraphSettings, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authzCtx, string(types.RoleAuth)) {
		return nil, trace.AccessDenied("this request can be only executed by an auth server")
	}

	cfg := req.GetAccessGraphSettings()
	if err := clusterconfig.ValidateAccessGraphSettings(cfg); err != nil {
		return nil, trace.Wrap(err)
	}

	created, err := s.backend.CreateAccessGraphSettings(ctx, cfg)
	if auditErr := s.emitter.EmitAuditEvent(ctx, &apievents.AccessGraphSettingsUpdate{
		Metadata: apievents.Metadata{
			Type: events.AccessGraphSettingsUpdateEvent,
			Code: events.AccessGraphSettingsUpdateCode,
		},
		UserMetadata:       authzCtx.GetUserMetadata(),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status:             eventStatus(err),
	}); auditErr != nil {
		slog.WarnContext(ctx, "Failed to emit AccessGraphSettings update event.", "error", auditErr)
	}

	// don't handle the update error until after we emit an audit event
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return created, nil
}

func (s *Service) UpdateAccessGraphSettings(ctx context.Context, req *clusterconfigpb.UpdateAccessGraphSettingsRequest) (*clusterconfigpb.AccessGraphSettings, error) {
	authzCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authzCtx.CheckAccessToKind(types.KindAccessGraphSettings, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authzCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	if !modules.GetModules().Features().GetEntitlement(entitlements.Policy).Enabled && !modules.GetModules().Features().AccessGraph {
		return nil, trace.AccessDenied("access graph is feature isn't enabled")
	}

	cfg := req.GetAccessGraphSettings()
	if err := clusterconfig.ValidateAccessGraphSettings(cfg); err != nil {
		return nil, trace.Wrap(err)
	}

	rsp, err := s.backend.UpdateAccessGraphSettings(ctx, cfg)

	if auditErr := s.emitter.EmitAuditEvent(ctx, &apievents.AccessGraphSettingsUpdate{
		Metadata: apievents.Metadata{
			Type: events.AccessGraphSettingsUpdateEvent,
			Code: events.AccessGraphSettingsUpdateCode,
		},
		UserMetadata:       authzCtx.GetUserMetadata(),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status:             eventStatus(err),
	}); auditErr != nil {
		slog.WarnContext(ctx, "Failed to emit AccessGraphSettings update event.", "error", auditErr)
	}

	if err != nil {
		return nil, trace.Wrap(err)
	}

	return rsp, nil
}

func (s *Service) UpsertAccessGraphSettings(ctx context.Context, req *clusterconfigpb.UpsertAccessGraphSettingsRequest) (*clusterconfigpb.AccessGraphSettings, error) {
	authzCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authzCtx.CheckAccessToKind(types.KindAccessGraphSettings, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authzCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	if !modules.GetModules().Features().GetEntitlement(entitlements.Policy).Enabled && !modules.GetModules().Features().AccessGraph {
		return nil, trace.AccessDenied("access graph is feature isn't enabled")
	}

	cfg := req.GetAccessGraphSettings()
	if err := clusterconfig.ValidateAccessGraphSettings(cfg); err != nil {
		return nil, trace.Wrap(err)
	}

	rsp, err := s.backend.UpsertAccessGraphSettings(ctx, cfg)

	if auditErr := s.emitter.EmitAuditEvent(ctx, &apievents.AccessGraphSettingsUpdate{
		Metadata: apievents.Metadata{
			Type: events.AccessGraphSettingsUpdateEvent,
			Code: events.AccessGraphSettingsUpdateCode,
		},
		UserMetadata:       authzCtx.GetUserMetadata(),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status:             eventStatus(err),
	}); auditErr != nil {
		slog.WarnContext(ctx, "Failed to emit AccessGraphSettings update event.", "error", auditErr)
	}

	if err != nil {
		return nil, trace.Wrap(err)
	}

	return rsp, nil
}

func (s *Service) ResetAccessGraphSettings(ctx context.Context, _ *clusterconfigpb.ResetAccessGraphSettingsRequest) (*clusterconfigpb.AccessGraphSettings, error) {
	authzCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authzCtx.CheckAccessToKind(types.KindAccessGraphSettings, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authzCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	if !modules.GetModules().Features().GetEntitlement(entitlements.Policy).Enabled && !modules.GetModules().Features().AccessGraph {
		return nil, trace.AccessDenied("access graph is feature isn't enabled")
	}

	obj, err := clusterconfig.NewAccessGraphSettings(&clusterconfigpb.AccessGraphSettingsSpec{
		SecretsScanConfig: clusterconfigpb.AccessGraphSecretsScanConfig_ACCESS_GRAPH_SECRETS_SCAN_CONFIG_DISABLED,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	rsp, err := s.backend.UpsertAccessGraphSettings(ctx, obj)

	if auditErr := s.emitter.EmitAuditEvent(ctx, &apievents.AccessGraphSettingsUpdate{
		Metadata: apievents.Metadata{
			Type: events.AccessGraphSettingsUpdateEvent,
			Code: events.AccessGraphSettingsUpdateCode,
		},
		UserMetadata:       authzCtx.GetUserMetadata(),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status:             eventStatus(err),
	}); auditErr != nil {
		slog.WarnContext(ctx, "Failed to emit AccessGraphSettings update event.", "error", auditErr)
	}

	if err != nil {
		return nil, trace.Wrap(err)
	}

	return rsp, nil
}
