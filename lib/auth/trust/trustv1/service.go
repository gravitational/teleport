// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package trustv1

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/emptypb"

	trustpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/trust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
)

// ServiceConfig holds configuration options for
// the trust gRPC service.
type ServiceConfig struct {
	Authorizer authz.Authorizer
	Cache      services.AuthorityGetter
	Backend    services.Trust
	Logger     *logrus.Entry
}

// Service implements the teleport.trust.v1.TrustService RPC service.
type Service struct {
	trustpb.UnimplementedTrustServiceServer
	authorizer authz.Authorizer
	cache      services.AuthorityGetter
	backend    services.Trust
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
	case cfg.Logger == nil:
		cfg.Logger = logrus.WithField(trace.Component, "trust.service")
	}

	return &Service{
		logger:     cfg.Logger,
		authorizer: cfg.Authorizer,
		cache:      cfg.Cache,
		backend:    cfg.Backend,
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
	_, err := authz.AuthorizeWithVerbs(ctx, s.logger, s.authorizer, false, types.KindCertAuthority, types.VerbDelete)
	if err != nil {
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

	if _, err := authz.AuthorizeResourceWithVerbs(ctx, s.logger, s.authorizer, false, req.CertAuthority, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.backend.UpsertCertAuthority(ctx, req.CertAuthority); err != nil {
		return nil, trace.Wrap(err)
	}

	return req.CertAuthority, nil
}
