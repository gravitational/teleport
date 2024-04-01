// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package tester

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
)

func handleSAMLConnector(c *auth.Client, connBytes []byte) (*AuthRequestInfo, error) {
	conn, err := services.UnmarshalSAMLConnector(connBytes)
	if err != nil {
		return nil, trace.Wrap(err, "Unable to load SAML connector. Correct the definition and try again.")
	}
	requestInfo, err := samlTest(c, conn)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return requestInfo, nil
}

func samlTest(c *auth.Client, samlConnector types.SAMLConnector) (*AuthRequestInfo, error) {
	ctx := context.Background()
	// get connector spec
	var spec types.SAMLConnectorSpecV2
	switch samlConnector := samlConnector.(type) {
	case *types.SAMLConnectorV2:
		spec = samlConnector.Spec
	default:
		return nil, trace.BadParameter("Unrecognized SAML connector version: %T. Provide supported connector version.", samlConnector)
	}

	requestInfo := &AuthRequestInfo{}

	makeRequest := func(req client.SSOLoginConsoleReq) (*client.SSOLoginConsoleResponse, error) {
		samlRequest := types.SAMLAuthRequest{
			ConnectorID:       req.ConnectorID + "-" + samlConnector.GetName(),
			Type:              constants.SAML,
			CheckUser:         false,
			PublicKey:         req.PublicKey,
			CertTTL:           defaults.SAMLAuthRequestTTL,
			CreateWebSession:  false,
			ClientRedirectURL: req.RedirectURL,
			RouteToCluster:    req.RouteToCluster,
			SSOTestFlow:       true,
			ConnectorSpec:     &spec,
		}

		request, err := c.CreateSAMLAuthRequest(ctx, samlRequest)
		if request != nil {
			requestInfo.RequestID = request.ID
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

func getInfoFieldsSAML(diag *types.SSODiagnosticInfo, debug bool) []string {
	return []string{
		GetDiagMessage(
			diag.SAMLAttributesToRoles != nil,
			true,
			FormatYAML("[SAML] Attributes to roles", diag.SAMLAttributesToRoles),
		),
		GetDiagMessage(
			diag.SAMLAttributesToRolesWarnings != nil,
			true,
			formatSSOWarnings("[SAML] Attributes mapping warning", diag.SAMLAttributesToRolesWarnings),
		),
		GetDiagMessage(
			diag.SAMLAttributeStatements != nil,
			true,
			FormatYAML("[SAML] Attributes statements", diag.SAMLAttributeStatements),
		),
		GetDiagMessage(
			diag.SAMLAssertionInfo != nil,
			debug,
			FormatJSON("[SAML] Assertion info", diag.SAMLAssertionInfo),
		),
		GetDiagMessage(
			diag.SAMLTraitsFromAssertions != nil,
			debug,
			FormatJSON("[SAML] Calculated user traits", diag.SAMLTraitsFromAssertions),
		),
		GetDiagMessage(
			diag.SAMLConnectorTraitMapping != nil,
			debug,
			FormatYAML("[SAML] Connector trait mapping", diag.SAMLConnectorTraitMapping),
		),
	}
}
