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

package trustv1

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/emptypb"

	trustpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/trust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
)

type authServer interface {
	// GenerateHostCert uses the private key of the CA to sign the public key of
	// the host (along with metadata like host ID, node name, roles, and ttl)
	// to generate a host certificate.
	GenerateHostCert(ctx context.Context, hostPublicKey []byte, hostID, nodeName string, principals []string, clusterName string, role types.SystemRole, ttl time.Duration) ([]byte, error)

	// RotateCertAuthority starts or restarts certificate authority rotation process.
	RotateCertAuthority(ctx context.Context, req types.RotateRequest) error
}

// ServiceConfig holds configuration options for
// the trust gRPC service.
type ServiceConfig struct {
	Authorizer authz.Authorizer
	Cache      services.AuthorityGetter
	Backend    services.Trust
	Logger     *logrus.Entry
	AuthServer authServer
}

// Service implements the teleport.trust.v1.TrustService RPC service.
type Service struct {
	trustpb.UnimplementedTrustServiceServer
	authorizer authz.Authorizer
	cache      services.AuthorityGetter
	backend    services.Trust
	authServer authServer
	logger     *logrus.Entry
}

// NewService returns a new trust gRPC service.
func NewService(cfg *ServiceConfig) (*Service, error) {
	switch {
	case cfg.Cache == nil:
		return nil, trace.BadParameter("cache is required")
	case cfg.Backend == nil:
		return nil, trace.BadParameter("backend is required")
	case cfg.Authorizer == nil:
		return nil, trace.BadParameter("authorizer is required")
	case cfg.AuthServer == nil:
		return nil, trace.BadParameter("authServer is required")
	case cfg.Logger == nil:
		cfg.Logger = logrus.WithField(trace.Component, "trust.service")
	}

	return &Service{
		logger:     cfg.Logger,
		authorizer: cfg.Authorizer,
		cache:      cfg.Cache,
		backend:    cfg.Backend,
		authServer: cfg.AuthServer,
	}, nil
}

// GetCertAuthority retrieves the matching certificate authority.
func (s *Service) GetCertAuthority(ctx context.Context, req *trustpb.GetCertAuthorityRequest) (*types.CertAuthorityV2, error) {
	readVerb := types.VerbReadNoSecrets
	if req.IncludeKey {
		readVerb = types.VerbRead
	}

	// Before looking up the requested CA perform RBAC on a dummy CA to
	// determine if the user has access to CAs in general. This helps prevent
	// leaking information about the requested CA if the call to GetCertAuthority
	// fails.
	contextCA, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        types.CertAuthType(req.Type),
		ClusterName: req.Domain,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	_, err = authz.AuthorizeResourceWithVerbs(ctx, s.logger, s.authorizer, false, contextCA, readVerb)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Retrieve the requested CA and perform RBAC on it to ensure that
	// the user has access to this particular CA.
	ca, err := s.cache.GetCertAuthority(ctx, types.CertAuthID{Type: types.CertAuthType(req.Type), DomainName: req.Domain}, req.IncludeKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	_, err = authz.AuthorizeResourceWithVerbs(ctx, s.logger, s.authorizer, false, ca, readVerb)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authority, ok := ca.(*types.CertAuthorityV2)
	if !ok {
		return nil, trace.BadParameter("unexpected ca type %T", ca)
	}

	return authority, nil
}

// GetCertAuthorities retrieves the cert authorities with the specified type.
func (s *Service) GetCertAuthorities(ctx context.Context, req *trustpb.GetCertAuthoritiesRequest) (*trustpb.GetCertAuthoritiesResponse, error) {
	verbs := []string{types.VerbList, types.VerbReadNoSecrets}

	if req.IncludeKey {
		verbs = append(verbs, types.VerbRead)
	}

	_, err := authz.AuthorizeWithVerbs(ctx, s.logger, s.authorizer, false, types.KindCertAuthority, verbs...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cas, err := s.cache.GetCertAuthorities(ctx, types.CertAuthType(req.Type), req.IncludeKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp := &trustpb.GetCertAuthoritiesResponse{CertAuthoritiesV2: make([]*types.CertAuthorityV2, 0, len(cas))}

	for _, ca := range cas {
		cav2, ok := ca.(*types.CertAuthorityV2)
		if !ok {
			return nil, trace.BadParameter("cert authority has invalid type %T", ca)
		}

		resp.CertAuthoritiesV2 = append(resp.CertAuthoritiesV2, cav2)
	}

	return resp, nil
}

// DeleteCertAuthority deletes the matching cert authority.
func (s *Service) DeleteCertAuthority(ctx context.Context, req *trustpb.DeleteCertAuthorityRequest) (*emptypb.Empty, error) {
	authzCtx, err := authz.AuthorizeWithVerbs(ctx, s.logger, s.authorizer, false, types.KindCertAuthority, types.VerbDelete)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authz.AuthorizeAdminAction(ctx, authzCtx); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.backend.DeleteCertAuthority(ctx, types.CertAuthID{DomainName: req.Domain, Type: types.CertAuthType(req.Type)}); err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}

// UpsertCertAuthority creates or updates the provided cert authority.
func (s *Service) UpsertCertAuthority(ctx context.Context, req *trustpb.UpsertCertAuthorityRequest) (*types.CertAuthorityV2, error) {
	if req.CertAuthority == nil {
		return nil, trace.BadParameter("missing certificate authority")
	}

	if err := services.ValidateCertAuthority(req.CertAuthority); err != nil {
		return nil, trace.Wrap(err)
	}

	authzCtx, err := authz.AuthorizeResourceWithVerbs(ctx, s.logger, s.authorizer, false, req.CertAuthority, types.VerbCreate, types.VerbUpdate)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Support reused MFA for bulk tctl create requests.
	if err := authz.AuthorizeAdminActionAllowReusedMFA(ctx, authzCtx); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.backend.UpsertCertAuthority(ctx, req.CertAuthority); err != nil {
		return nil, trace.Wrap(err)
	}

	return req.CertAuthority, nil
}

// RotateCertAuthority rotates a cert authority.
func (s *Service) RotateCertAuthority(ctx context.Context, req *trustpb.RotateCertAuthorityRequest) (*trustpb.RotateCertAuthorityResponse, error) {
	authzCtx, err := authz.AuthorizeWithVerbs(ctx, s.logger, s.authorizer, false, types.KindCertAuthority, types.VerbCreate, types.VerbUpdate)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authz.AuthorizeAdminAction(ctx, authzCtx); err != nil {
		return nil, trace.Wrap(err)
	}

	rotateRequest := types.RotateRequest{
		Type:        types.CertAuthType(req.Type),
		TargetPhase: req.TargetPhase,
		Mode:        req.Mode,
	}

	if req.GracePeriod != nil {
		duration := req.GracePeriod.AsDuration()
		rotateRequest.GracePeriod = &duration
	}

	if req.Schedule != nil {
		rotateRequest.Schedule = &types.RotationSchedule{
			UpdateClients: req.Schedule.UpdateClients.AsTime(),
			UpdateServers: req.Schedule.UpdateServers.AsTime(),
			Standby:       req.Schedule.Standby.AsTime(),
		}
	}

	if err := s.authServer.RotateCertAuthority(ctx, rotateRequest); err != nil {
		return nil, trace.Wrap(err)
	}

	return &trustpb.RotateCertAuthorityResponse{}, nil
}

// GenerateHostCert takes a public key in the OpenSSH `authorized_keys` format
// and returns a SSH certificate signed by the Host CA.
func (s *Service) GenerateHostCert(
	ctx context.Context, req *trustpb.GenerateHostCertRequest,
) (*trustpb.GenerateHostCertResponse, error) {
	// Perform special authz as we allow for `where` rules on the `host_cert`
	// resource type.
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, authz.ConvertAuthorizerError(ctx, s.logger, err)
	}
	ruleCtx := &services.Context{
		User: authCtx.User,
		HostCert: &services.HostCertContext{
			HostID:      req.HostId,
			NodeName:    req.NodeName,
			Principals:  req.Principals,
			ClusterName: req.ClusterName,
			Role:        types.SystemRole(req.Role),
			TTL:         req.Ttl.AsDuration(),
		},
	}
	_, err = authz.AuthorizeContextWithVerbs(
		ctx,
		s.logger,
		authCtx,
		false,
		ruleCtx,
		types.KindHostCert,
		types.VerbCreate,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO (Joerger): in v16.0.0, this endpoint should require admin action authorization
	// once the deprecated http endpoint is removed in use.

	// Call through to the underlying implementation on auth.Server. At some
	// point in the future, we may wish to pull more of that implementation
	// up to here.
	cert, err := s.authServer.GenerateHostCert(
		ctx,
		req.Key,
		req.HostId,
		req.NodeName,
		req.Principals,
		req.ClusterName,
		types.SystemRole(req.Role),
		req.Ttl.AsDuration(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &trustpb.GenerateHostCertResponse{
		SshCertificate: cert,
	}, nil
}
