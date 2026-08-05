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

package oktaservice

import (
	"context"
	"crypto"
	"log/slog"
	"net/http"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	oktapb "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	oktaapi "github.com/gravitational/teleport/e/lib/okta/api"
	"github.com/gravitational/teleport/e/lib/okta/common/sso"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils"
)

// ServiceConfig is the service config for the Okta gRPC service.
type ServiceConfig struct {
	// Backend is the backend to use.
	Backend backend.Backend

	// Logger is the logger to use.
	Logger *slog.Logger

	// Modules defines build time constraints and licensed features.
	Modules modules.Modules

	// Authorizer is the authorizer to use.
	Authorizer authz.Authorizer

	// OktaImportRules is the Okta import rules service to use.
	OktaImportRules services.OktaImportRules

	// OktaAssignments is the Okta assignments service to use.
	OktaAssignments services.OktaAssignments
	// JWTSigner is the JWT signer getter to use.
	JWTSigner jwtSignerGetter
	// Clock is the clock to use.
	Clock clockwork.Clock

	// AuthCache is the auth cache to use.
	AuthCache authCache
	// AuthService is the auth service to use.
	AuthService authServer
	// PluginService is the plugin service to use.
	PluginService pluginService
	// RoundTripper is the HTTP round tripper to use.
	RoundTripper http.RoundTripper
	// PluginBackend is the plugin backend to use.
	PluginBackend services.Plugins
	// CredsBackend is the plugin static credentials backend to use.
	CredsBackend services.PluginStaticCredentials
}

type jwtSignerGetter interface {
	GetJWTSigner(ctx context.Context, ca types.CertAuthority) (crypto.Signer, error)
}

type pluginService interface {
	CreatePlugin(context.Context, *pluginspb.CreatePluginRequest) (*emptypb.Empty, error)
}

func (c *ServiceConfig) CheckAndSetDefaults() error {
	if c.Backend == nil {
		return trace.BadParameter("backend is missing")
	}

	if c.Modules == nil {
		return trace.BadParameter("modules is missing")
	}

	if c.Logger == nil {
		c.Logger = slog.With(teleport.ComponentKey, "okta_crud_service")
	}

	if c.Authorizer == nil {
		return trace.BadParameter("authorizer is missing")
	}

	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}

	var err error
	var oktaSvc *local.OktaService
	if c.OktaImportRules == nil || c.OktaAssignments == nil {
		oktaSvc, err = local.NewOktaService(c.Backend, c.Backend.Clock())
		if err != nil {
			return trace.Wrap(err)
		}
	}

	if c.OktaImportRules == nil {
		c.OktaImportRules = oktaSvc
	}

	if c.OktaAssignments == nil {
		c.OktaAssignments = oktaSvc
	}
	if c.JWTSigner == nil {
		return trace.BadParameter("key store is missing")
	}

	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	if c.RoundTripper == nil {
		c.RoundTripper = http.DefaultTransport
	}

	if c.PluginBackend == nil {
		c.PluginBackend = local.NewPluginsService(c.Backend)
	}
	if c.CredsBackend == nil {
		s, err := local.NewPluginStaticCredentialsService(c.Backend)
		if err != nil {
			return trace.Wrap(err)
		}
		c.CredsBackend = s
	}
	return nil
}

var _ oktapb.OktaServiceServer = (*Service)(nil)

// authCache is an interface for fetching cert authority resources.
type authCache interface {
	GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error)
	// GetClusterName returns the name of the cluster.
	GetClusterName(ctx context.Context) (types.ClusterName, error)
}

type authServer interface {
	Ping(ctx context.Context) (proto.PingResponse, error)
	GetClusterName(ctx context.Context) (types.ClusterName, error)
	sso.SAMLConnectorService
}

type Service struct {
	oktapb.UnimplementedOktaServiceServer

	logger              *slog.Logger
	authorizer          authz.Authorizer
	modules             modules.Modules
	oktaImportRules     services.OktaImportRules
	oktaAssignments     services.OktaAssignments
	jwtSigner           jwtSignerGetter
	authCache           authCache
	authService         authServer
	pluginService       pluginService
	cache               *utils.FnCache
	roundTripper        http.RoundTripper
	pluginBackend       services.Plugins
	credsBackend        services.PluginStaticCredentials
	apiClientProviderFn oktaapi.OktaClientFn
	clock               clockwork.Clock
}

// NewService creates a new Okta gRPC service.
func NewService(cfg ServiceConfig) (*Service, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	// cache is used to cache the results of getApps, getGroups Okta API result
	// invoked during Okta Enrollment flow to present the user with a list of
	// apps and groups to choose from.
	cache, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL:   time.Minute,
		Clock: cfg.Clock,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Service{
		logger:              cfg.Logger,
		authorizer:          cfg.Authorizer,
		modules:             cfg.Modules,
		oktaImportRules:     cfg.OktaImportRules,
		oktaAssignments:     cfg.OktaAssignments,
		jwtSigner:           cfg.JWTSigner,
		authCache:           cfg.AuthCache,
		authService:         cfg.AuthService,
		pluginService:       cfg.PluginService,
		cache:               cache,
		roundTripper:        cfg.RoundTripper,
		pluginBackend:       cfg.PluginBackend,
		credsBackend:        cfg.CredsBackend,
		apiClientProviderFn: oktaapi.New,
		clock:               cfg.Clock,
	}, nil
}

// ListOktaImportRules returns a paginated list of all Okta import rule resources.
func (s *Service) ListOktaImportRules(ctx context.Context, req *oktapb.ListOktaImportRulesRequest) (*oktapb.ListOktaImportRulesResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindOktaImportRule, types.VerbRead, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	results, nextPageToken, err := s.oktaImportRules.ListOktaImportRules(ctx, int(req.GetPageSize()), req.GetPageToken())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	importRulesV1 := make([]*types.OktaImportRuleV1, len(results))
	for i, r := range results {
		v1, ok := r.(*types.OktaImportRuleV1)
		if !ok {
			return nil, trace.BadParameter("unexpected Okta import rule type %T", r)
		}
		importRulesV1[i] = v1
	}

	return oktapb.ListOktaImportRulesResponse_builder{
		ImportRules:   importRulesV1,
		NextPageToken: nextPageToken,
	}.Build(), nil
}

// GetOktaImportRule returns the specified Okta import rule resources.
func (s *Service) GetOktaImportRule(ctx context.Context, req *oktapb.GetOktaImportRuleRequest) (*types.OktaImportRuleV1, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindOktaImportRule, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	importRule, err := s.oktaImportRules.GetOktaImportRule(ctx, req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	importRuleV1, ok := importRule.(*types.OktaImportRuleV1)
	if !ok {
		return nil, trace.BadParameter("unexpected Okta import rule type %T", importRule)
	}

	return importRuleV1, nil
}

// CreateOktaImportRule creates a new Okta import rule resource.
func (s *Service) CreateOktaImportRule(ctx context.Context, req *oktapb.CreateOktaImportRuleRequest) (*types.OktaImportRuleV1, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindOktaImportRule, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	returnedRule, err := s.oktaImportRules.CreateOktaImportRule(ctx, req.GetImportRule())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	returnedRuleV1, ok := returnedRule.(*types.OktaImportRuleV1)
	if !ok {
		return nil, trace.BadParameter("expected returned import rule of OktaImportRuleV1, got %T", returnedRuleV1)
	}
	return returnedRuleV1, trace.Wrap(err)
}

// UpdateOktaImportRule updates an existing Okta import rule resource.
func (s *Service) UpdateOktaImportRule(ctx context.Context, req *oktapb.UpdateOktaImportRuleRequest) (*types.OktaImportRuleV1, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindOktaImportRule, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	returnedRule, err := s.oktaImportRules.UpdateOktaImportRule(ctx, req.GetImportRule())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	returnedRuleV1, ok := returnedRule.(*types.OktaImportRuleV1)
	if !ok {
		return nil, trace.BadParameter("expected returned import rule of OktaImportRuleV1, got %T", returnedRuleV1)
	}
	return returnedRuleV1, trace.Wrap(err)
}

// DeleteOktaImportRule removes the specified Okta import rule resource.
func (s *Service) DeleteOktaImportRule(ctx context.Context, req *oktapb.DeleteOktaImportRuleRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindOktaImportRule, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, trace.Wrap(s.oktaImportRules.DeleteOktaImportRule(ctx, req.GetName()))
}

// DeleteAllOktaImportRules removes all Okta import rules.
func (s *Service) DeleteAllOktaImportRules(ctx context.Context, _ *oktapb.DeleteAllOktaImportRulesRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindOktaImportRule, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, trace.Wrap(s.oktaImportRules.DeleteAllOktaImportRules(ctx))
}

// ListOktaAssignments returns a paginated list of all Okta assignment resources.
func (s *Service) ListOktaAssignments(ctx context.Context, req *oktapb.ListOktaAssignmentsRequest) (*oktapb.ListOktaAssignmentsResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindOktaAssignment, types.VerbList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	results, nextPageToken, err := s.oktaAssignments.ListOktaAssignments(ctx, int(req.GetPageSize()), req.GetPageToken())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	assignmentsV1 := make([]*types.OktaAssignmentV1, len(results))
	for i, a := range results {
		v1, ok := a.(*types.OktaAssignmentV1)
		if !ok {
			return nil, trace.BadParameter("unexpected Okta assignment type %T", a)
		}
		assignmentsV1[i] = v1
	}

	return oktapb.ListOktaAssignmentsResponse_builder{
		Assignments:   assignmentsV1,
		NextPageToken: nextPageToken,
	}.Build(), nil
}

// GetOktaAssignment returns the specified Okta assignment resources.
func (s *Service) GetOktaAssignment(ctx context.Context, req *oktapb.GetOktaAssignmentRequest) (*types.OktaAssignmentV1, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindOktaAssignment, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	assignment, err := s.oktaAssignments.GetOktaAssignment(ctx, req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	assignmentV1, ok := assignment.(*types.OktaAssignmentV1)
	if !ok {
		return nil, trace.BadParameter("unexpected Okta assignment type %T", assignment)
	}

	return assignmentV1, nil
}

// CreateOktaAssignment creates a new Okta assignment resource.
func (s *Service) CreateOktaAssignment(ctx context.Context, req *oktapb.CreateOktaAssignmentRequest) (*types.OktaAssignmentV1, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindOktaAssignment, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	returnedAssignment, err := s.oktaAssignments.CreateOktaAssignment(ctx, req.GetAssignment())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	returnedAssignmentV1, ok := returnedAssignment.(*types.OktaAssignmentV1)
	if !ok {
		return nil, trace.BadParameter("expected OktaAssignmentV1, got %T", returnedAssignmentV1)
	}
	return returnedAssignmentV1, trace.Wrap(err)
}

// UpdateOktaAssignment updates an existing Okta assignment resource.
func (s *Service) UpdateOktaAssignment(ctx context.Context, req *oktapb.UpdateOktaAssignmentRequest) (*types.OktaAssignmentV1, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindOktaAssignment, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	returnedAssignment, err := s.oktaAssignments.UpdateOktaAssignment(ctx, req.GetAssignment())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	returnedAssignmentV1, ok := returnedAssignment.(*types.OktaAssignmentV1)
	if !ok {
		return nil, trace.BadParameter("expected OktaAssignmentV1, got %T", returnedAssignmentV1)
	}
	return returnedAssignmentV1, trace.Wrap(err)
}

// ConditionalUpdateOktaAssignment updates an existing Okta assignment resource using CAS operation.
func (s *Service) ConditionalUpdateOktaAssignment(ctx context.Context, req *oktapb.ConditionalUpdateOktaAssignmentRequest) (*oktapb.ConditionalUpdateOktaAssignmentResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindOktaAssignment, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	item, err := s.oktaAssignments.ConditionalUpdateOktaAssignment(ctx, req.GetAssignment())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	v1, ok := item.(*types.OktaAssignmentV1)
	if !ok {
		return nil, trace.BadParameter("expected OktaAssignmentV1, got %T", item)
	}
	return oktapb.ConditionalUpdateOktaAssignmentResponse_builder{
		Assignment: v1,
	}.Build(), nil
}

// UpsertOktaAssignment upserts an Okta assignment resource, creating it if it doesn't exist or updating it if it does.
func (s *Service) UpsertOktaAssignment(ctx context.Context, req *oktapb.UpsertOktaAssignmentRequest) (*oktapb.UpsertOktaAssignmentResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindOktaAssignment, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	item, err := s.oktaAssignments.UpsertOktaAssignment(ctx, req.GetAssignment())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	v1, ok := item.(*types.OktaAssignmentV1)
	if !ok {
		return nil, trace.BadParameter("expected OktaAssignmentV1, got %T", item)
	}
	return oktapb.UpsertOktaAssignmentResponse_builder{
		Assignment: v1,
	}.Build(), nil
}

// DeleteOktaAssignment removes the specified Okta assignment resource.
func (s *Service) DeleteOktaAssignment(ctx context.Context, req *oktapb.DeleteOktaAssignmentRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindOktaAssignment, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if rev := req.GetRevision(); rev != "" {
		return &emptypb.Empty{}, trace.Wrap(s.oktaAssignments.ConditionalDeleteOktaAssignment(ctx, req.GetName(), rev))
	}

	return &emptypb.Empty{}, trace.Wrap(s.oktaAssignments.DeleteOktaAssignment(ctx, req.GetName()))
}

// DeleteAllOktaAssignments removes all Okta assignments.
func (s *Service) DeleteAllOktaAssignments(ctx context.Context, _ *oktapb.DeleteAllOktaAssignmentsRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindOktaAssignment, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, trace.Wrap(s.oktaAssignments.DeleteAllOktaAssignments(ctx))
}
