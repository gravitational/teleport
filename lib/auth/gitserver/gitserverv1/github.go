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
	"github.com/gravitational/teleport/lib/authz"
)

func (s *Service) CreateGitHubAuthRequest(ctx context.Context, in *pb.CreateGitHubAuthRequestRequest) (*types.GithubAuthRequest, error) {
	if !in.HasRequest() {
		return nil, trace.BadParameter("missing github auth request")
	}
	if in.GetRequest().SSOTestFlow {
		return nil, trace.BadParameter("sso test flow is not supported when creating GitHub auth request for authenticated user")
	}
	if in.GetRequest().CreateWebSession {
		return nil, trace.BadParameter("CreateWebSession is not supported when creating GitHub auth request for authenticated user")
	}

	authCtx, gitServer, err := s.authAndFindServer(ctx, in.GetRequest().ConnectorID, in.GetOrganization())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s.cfg.Log.DebugContext(ctx, "Creating GitHub auth request for authenticated user.",
		"user", authCtx.User.GetName(),
		"git_server", gitServer.GetName(),
		"org", gitServer.GetGitHub().Organization,
		"integration", gitServer.GetGitHub().Integration,
	)

	// ConnectorID is the git server name for the authenticated user flow, so
	// the callback knows which git server the user authorized.
	in.GetRequest().ConnectorID = gitServer.GetName()
	in.GetRequest().Integration = gitServer.GetGitHub().Integration
	in.GetRequest().AuthenticatedUser = authCtx.User.GetName()
	in.GetRequest().CertTTL = authCtx.Identity.GetIdentity().Expires.Sub(s.cfg.clock.Now())
	in.GetRequest().ClientLoginIP = authCtx.Identity.GetIdentity().LoginIP

	// More params of in.Request will get updated and checked by
	// s.cfg.GitHubAuthRequestCreator.
	request, err := s.cfg.GitHubAuthRequestCreator(ctx, *in.GetRequest())
	return request, trace.Wrap(err)
}

// authAndFindServer finds the git server by name when provided, otherwise by
// GitHub organization.
func (s *Service) authAndFindServer(ctx context.Context, name, org string) (*authz.Context, types.Server, error) {
	if name == "" {
		return s.authAndFindServerByOrg(ctx, org)
	}

	authCtx, err := s.authorize(ctx, types.VerbRead)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	server, err := s.cfg.Backend.GetGitServer(ctx, name)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	if err := s.checkAccess(authCtx, server); err != nil {
		if trace.IsAccessDenied(err) {
			return nil, nil, trace.NotFound("git server %q not found", name)
		}
		return nil, nil, trace.Wrap(err)
	}
	spec := server.GetGitHub()
	if spec == nil {
		return nil, nil, trace.BadParameter("git server %q is not a GitHub server", name)
	}
	if org != "" && spec.Organization != org {
		return nil, nil, trace.BadParameter("git server %q is not for organization %q", name, org)
	}
	return authCtx, server, nil
}

func (s *Service) authAndFindServerByOrg(ctx context.Context, org string) (*authz.Context, types.Server, error) {
	if err := types.ValidateGitHubOrganizationName(org); err != nil {
		return nil, nil, trace.Wrap(err)
	}
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
