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

package tester

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
)

func githubTest(c *authclient.Client, connector types.GithubConnector) (*AuthRequestInfo, error) {
	ctx := context.Background()
	// get connector spec
	var spec types.GithubConnectorSpecV3
	switch ghConnector := connector.(type) {
	case *types.GithubConnectorV3:
		spec = ghConnector.Spec
	default:
		return nil, trace.BadParameter("Unrecognized GitHub connector version: %T. Provide supported connector version.", ghConnector)
	}

	requestInfo := &AuthRequestInfo{}

	makeRequest := func(req client.SSOLoginConsoleReq) (*client.SSOLoginConsoleResponse, error) {
		if err := req.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		ghRequest := types.GithubAuthRequest{
			ConnectorID:       req.ConnectorID + "-" + connector.GetName(),
			Type:              constants.Github,
			SshPublicKey:      req.SSHPubKey,
			TlsPublicKey:      req.TLSPubKey,
			CertTTL:           defaults.GithubAuthRequestTTL,
			CreateWebSession:  false,
			ClientRedirectURL: req.RedirectURL,
			RouteToCluster:    req.RouteToCluster,
			SSOTestFlow:       true,
			ConnectorSpec:     &spec,
		}

		request, err := c.CreateGithubAuthRequest(ctx, ghRequest)

		if request != nil {
			requestInfo.RequestID = request.StateToken
		}
		requestInfo.RequestCreateErr = err

		if err != nil {
			return nil, trace.Wrap(err)
		}

		return &client.SSOLoginConsoleResponse{RedirectURL: request.RedirectURL}, nil
	}

	requestInfo.SSOLoginConsoleRequestFn = makeRequest
	return requestInfo, nil
}

func handleGithubConnector(c *authclient.Client, connBytes []byte) (*AuthRequestInfo, error) {
	conn, err := services.UnmarshalGithubConnector(connBytes)
	if err != nil {
		return nil, trace.Wrap(err, "Unable to load GitHub connector. Correct the definition and try again.")
	}
	requestInfo, err := githubTest(c, conn)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return requestInfo, nil
}

func getGithubDiagInfoFields(diag *types.SSODiagnosticInfo, debug bool) []string {
	return []string{
		GetDiagMessage(
			diag.GithubTokenInfo != nil,
			debug,
			FormatJSON("[GitHub] OAuth2 token info", diag.GithubTokenInfo),
		),
		GetDiagMessage(
			diag.GithubClaims != nil,
			true,
			FormatYAML("[GitHub] Received claims", diag.GithubClaims),
		),
		GetDiagMessage(
			diag.GithubTeamsToLogins != nil,
			true,
			FormatYAML("[GitHub] Connector team to logins mapping", diag.GithubTeamsToLogins),
		),
		GetDiagMessage(
			diag.GithubTeamsToRoles != nil,
			true,
			FormatYAML("[GitHub] Connector team to roles mapping", diag.GithubTeamsToRoles),
		),
	}
}
