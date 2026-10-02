/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package gitserverv1

import (
	"context"

	"github.com/gravitational/trace"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/gitserver/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
)

func (s *Service) CreateGitHubAuthRequest(ctx context.Context, in *pb.CreateGitHubAuthRequestRequest) (*types.GithubAuthRequest, error) {
	if !in.HasRequest() {
		return nil, trace.BadParameter("missing github auth request")
	}
	if err := types.ValidateGitHubOrganizationName(in.GetOrganization()); err != nil {
		return nil, trace.Wrap(err)
	}
	if in.GetRequest().SSOTestFlow {
		return nil, trace.BadParameter("sso test flow is not supported when creating GitHub auth request for authenticated user")
	}
	if in.GetRequest().CreateWebSession {
		return nil, trace.BadParameter("CreateWebSession is not supported when creating GitHub auth request for authenticated user")
	}

	authCtx, gitServer, err := s.authAndFindServerByOrg(ctx, in.GetOrganization())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Check against the unmapped identity, if the user is a remote user
	// the mapped identity will not have some of the below fields populated.
	unmappedIdent := authCtx.UnmappedIdentity.GetIdentity()
	if unmappedIdent.DisallowReissue {
		return nil, trace.AccessDenied("access denied: identity is not allowed to reissue certificates")
	} else if unmappedIdent.Impersonator != "" {
		return nil, trace.AccessDenied("access denied: impersonated user cannot create GitHub auth request")
	} else if unmappedIdent.IsDelegationSession() {
		return nil, trace.AccessDenied("access denied: delegation session cannot create GitHub auth request")
	} else if unmappedIdent.IsBot() {
		return nil, trace.AccessDenied("access denied: bot cannot create GitHub auth request")
	} else if unmappedIdent.GetUserMetadata().UserKind == events.UserKind_USER_KIND_SYSTEM {
		return nil, trace.AccessDenied("access denied: system user cannot create GitHub auth request")
	}

	s.cfg.Log.DebugContext(ctx, "Creating GitHub auth request for authenticated user.",
		"user", authCtx.User.GetName(),
		"org", gitServer.GetGitHub().Organization,
		"integration", gitServer.GetGitHub().Integration,
	)

	// The cert TTL should be taken from the mapped identity which will
	// have an adjusted TTL if the user is a remote user.
	identity := authCtx.Identity.GetIdentity()

	// ConnectorID is not meaningful for the integration flow.
	in.GetRequest().ConnectorID = gitServer.GetGitHub().Integration
	in.GetRequest().Integration = gitServer.GetGitHub().Integration
	in.GetRequest().AuthenticatedUser = authCtx.User.GetName()
	in.GetRequest().CertTTL = identity.Expires.Sub(s.cfg.clock.Now())
	in.GetRequest().ClientLoginIP = identity.LoginIP

	// More params of in.Request will get updated and checked by
	// s.cfg.GitHubAuthRequestCreator.
	request, err := s.cfg.GitHubAuthRequestCreator(ctx, *in.GetRequest())
	return request, trace.Wrap(err)
}

func (s *Service) authAndFindServerByOrg(ctx context.Context, org string) (*authz.Context, types.Server, error) {
	authCtx, err := s.authorize(ctx, types.VerbRead, types.VerbList)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// We assume the list of servers is small and this function is called
	// rarely. Use a cache when these assumptions change.
	var servers []types.Server
	var next string
	for {
		servers, next, err = s.cfg.Backend.ListGitServers(ctx, apidefaults.DefaultChunkSize, next)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		for _, server := range servers {
			if spec := server.GetGitHub(); spec != nil && spec.Organization == org {
				if err := s.checkAccess(authCtx, server); err == nil {
					return authCtx, server, nil
				}
			}
		}
		if next == "" {
			break
		}
	}
	return nil, nil, trace.NotFound("git server with organization %q not found", org)
}
