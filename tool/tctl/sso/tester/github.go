// Copyright 2022 Gravitational, Inc
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

package tester

import (
	"context"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
)

func githubTest(c auth.ClientI, connector types.GithubConnector) (*AuthRequestInfo, error) {
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
		ghRequest := types.GithubAuthRequest{
			ConnectorID:       req.ConnectorID + "-" + connector.GetName(),
			Type:              constants.Github,
			PublicKey:         req.PublicKey,
			CertTTL:           types.Duration(defaults.GithubAuthRequestTTL),
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

	requestInfo.Config = &client.RedirectorConfig{SSOLoginConsoleRequestFn: makeRequest}
	return requestInfo, nil
}

func handleGithubConnector(c auth.ClientI, connBytes []byte) (*AuthRequestInfo, error) {
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
	}
}
