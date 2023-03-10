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

	if err := s.authorize(ctx, contextCA, readVerb); err != nil {
		return nil, trace.Wrap(err)
	}

	// Retrieve the requested CA and perform RBAC on it to ensure that
	// the user has access to this particular CA.
	ca, err := s.cache.GetCertAuthority(ctx, types.CertAuthID{Type: types.CertAuthType(req.Type), DomainName: req.Domain}, req.IncludeKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.authorize(ctx, ca, readVerb); err != nil {
		return nil, trace.Wrap(err)
	}

	authority, ok := ca.(*types.CertAuthorityV2)
	if !ok {
		return nil, trace.BadParameter("unexpected ca type %T", ca)
	}

	return authority, nil
}

// authorize ensures the client has access to perform the requested
// actions on the certificate on the provided certificate authority.
func (s *Service) authorize(ctx context.Context, ca types.CertAuthority, verbs ...string) error {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		switch {
		// propagate connection problem errors, so we can differentiate
		// between connection failed and access denied
		case trace.IsConnectionProblem(err):
			return trace.ConnectionProblem(err, "failed to connect to the database")
		case trace.IsNotFound(err):
			// user not found, wrap error with access denied
			return trace.Wrap(err, "access denied")
		case trace.IsAccessDenied(err):
			// don't print stack trace, just log the warning
			s.logger.Warn(err)
		case keys.IsPrivateKeyPolicyError(err):
			// private key policy errors should be returned to the client
			// unaltered so that they know to reauthenticate with a valid key.
			return trace.Unwrap(err)
		default:
			s.logger.Warn(trace.DebugReport(err))
		}

		return trace.AccessDenied("access denied")
	}

	ruleCtx := &services.Context{
		User:     authCtx.User,
		Resource: ca,
	}

	var errs []error
	for _, verb := range verbs {
		errs = append(errs, authCtx.Checker.CheckAccessToRule(ruleCtx, defaults.Namespace, types.KindCertAuthority, verb, false))
	}

	// Convert generic aggregate error to AccessDenied.
	if err := trace.NewAggregate(errs...); err != nil {
		return trace.AccessDenied(err.Error())
	}

	return nil
}
