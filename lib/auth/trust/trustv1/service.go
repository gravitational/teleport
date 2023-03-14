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

	"github.com/gravitational/teleport/api/defaults"
	trustpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/trust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
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
	authorizer, err := s.authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

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

	if err := authorizer.action(ctx, contextCA, readVerb); err != nil {
		return nil, trace.Wrap(err)
	}

	// Retrieve the requested CA and perform RBAC on it to ensure that
	// the user has access to this particular CA.
	ca, err := s.cache.GetCertAuthority(ctx, types.CertAuthID{Type: types.CertAuthType(req.Type), DomainName: req.Domain}, req.IncludeKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authorizer.action(ctx, ca, readVerb); err != nil {
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
	authorizer, err := s.authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authorizer.action(ctx, nil, types.VerbList, types.VerbReadNoSecrets); err != nil {
		return nil, trace.Wrap(err)
	}

	if req.IncludeKey {
		if err := authorizer.action(ctx, nil, types.VerbRead); err != nil {
			return nil, trace.Wrap(err)
		}
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

func (s *Service) authorize(ctx context.Context) (*authorizer, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		switch {
		// propagate connection problem errors, so we can differentiate
		// between connection failed and access denied
		case trace.IsConnectionProblem(err):
			return nil, trace.ConnectionProblem(err, "failed to connect to the database")
		case trace.IsNotFound(err):
			// user not found, wrap error with access denied
			return nil, trace.Wrap(err, "access denied")
		case trace.IsAccessDenied(err):
			// don't print stack trace, just log the warning
			s.logger.Warn(err)
		case keys.IsPrivateKeyPolicyError(err):
			// private key policy errors should be returned to the client
			// unaltered so that they know to reauthenticate with a valid key.
			return nil, trace.Unwrap(err)
		default:
			s.logger.Warn(trace.DebugReport(err))
		}

		return nil, trace.AccessDenied("access denied")
	}

	return &authorizer{Context: authCtx}, nil
}

type authorizer struct {
	*authz.Context
}

// authorize ensures the client has access to perform the requested
// actions on the certificate on the provided certificate authority.
func (a *authorizer) action(ctx context.Context, ca types.CertAuthority, verbs ...string) error {
	ruleCtx := &services.Context{
		User:     a.User,
		Resource: ca,
	}

	var errs []error
	for _, verb := range verbs {
		errs = append(errs, a.Checker.CheckAccessToRule(ruleCtx, defaults.Namespace, types.KindCertAuthority, verb, false))
	}

	// Convert generic aggregate error to AccessDenied.
	if err := trace.NewAggregate(errs...); err != nil {
		return trace.AccessDenied(err.Error())
	}

	return nil
}
