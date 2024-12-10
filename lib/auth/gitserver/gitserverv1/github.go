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
	"fmt"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/gitserver/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/integration/credentials"
	"github.com/gravitational/teleport/lib/authz"
)

func (s *Service) CreateGitHubAuthRequest(ctx context.Context, in *pb.CreateGitHubAuthRequestRequest) (*types.GithubAuthRequest, error) {
	if in.Request == nil {
		return nil, trace.BadParameter("missing github auth request")
	}
	if err := types.ValidateGitHubOrganizationName(in.Organization); err != nil {
		return nil, trace.Wrap(err)
	}
	if in.Request.SSOTestFlow {
		return nil, trace.BadParameter("sso test flow is not supported when creating GitHub auth request for authenticated user")
	}
	if in.Request.CreateWebSession {
		return nil, trace.BadParameter("CreateWebSession is not supported when creating GitHub auth request for authenticated user")
	}

	authCtx, gitServer, err := s.authAndFindServerByOrg(ctx, in.Organization)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s.cfg.Log.DebugContext(ctx, "Creating GitHub auth request for authenticated user.",
		"user", authCtx.User.GetName(),
		"org", gitServer.GetGitHub().Organization,
		"integration", gitServer.GetGitHub().Integration,
	)

	// Make a "temporary" connector spec and save it in the request.
	uuid := uuid.NewString()
	spec, err := s.makeGithubConnectorSpec(ctx, uuid, gitServer.GetGitHub().Organization, gitServer.GetGitHub().Integration)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	in.Request.ConnectorID = uuid
	in.Request.ConnectorSpec = spec
	in.Request.AuthenticatedUser = authCtx.User.GetName()
	in.Request.CertTTL = authCtx.Identity.GetIdentity().Expires.Sub(s.cfg.clock.Now())
	in.Request.ClientLoginIP = authCtx.Identity.GetIdentity().LoginIP

	// More params of in.Request will get updated and checked by
	// s.cfg.GitHubAuthRequestCreator.
	request, err := s.cfg.GitHubAuthRequestCreator(ctx, *in.Request)
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

func (s *Service) makeGithubConnectorSpec(ctx context.Context, uuid, org, integration string) (*types.GithubConnectorSpecV3, error) {
	ref, err := credentials.GetIntegrationRef(ctx, integration, s.cfg.Backend)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cred, err := credentials.GetByPurpose(ctx, ref, credentials.PurposeGitHubOAuth, s.cfg.Backend)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clientID, clientSecret := cred.GetOAuthClientSecret()
	if clientID == "" || clientSecret == "" {
		return nil, trace.BadParameter("no OAuth client ID or secret found for integration %v", integration)
	}

	return &types.GithubConnectorSpecV3{
		ClientID:       clientID,
		ClientSecret:   clientSecret,
		RedirectURL:    fmt.Sprintf("https://%s/v1/webapi/github/callback", s.cfg.ProxyPublicAddrGetter()),
		EndpointURL:    types.GithubURL,
		APIEndpointURL: types.GithubAPIURL,
		// TODO(greedy52) the GitHub OAuth flow for authenticated user does not
		// require team-to-roles mapping. Put some placeholders for now to make
		// param-checks happy.
		TeamsToRoles: []types.TeamRolesMapping{{
			Organization: org,
			Team:         uuid,
			Roles:        []string{uuid},
		}},
	}, nil
}
