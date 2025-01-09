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
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
)

func handleOIDCConnector(c *authclient.Client, connBytes []byte) (*AuthRequestInfo, error) {
	conn, err := services.UnmarshalOIDCConnector(connBytes)
	if err != nil {
		return nil, trace.Wrap(err, "Unable to load OIDC connector. Correct the definition and try again.")
	}
	requestInfo, err := oidcTest(c, conn)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return requestInfo, nil
}

func oidcTest(c *authclient.Client, connector types.OIDCConnector) (*AuthRequestInfo, error) {
	ctx := context.Background()
	// get connector spec
	var spec types.OIDCConnectorSpecV3
	switch oidcConnector := connector.(type) {
	case *types.OIDCConnectorV3:
		spec = oidcConnector.Spec
	default:
		return nil, trace.BadParameter("Unrecognized oidc connector version: %T. Provide supported connector version.", oidcConnector)
	}

	requestInfo := &AuthRequestInfo{}

	makeRequest := func(req client.SSOLoginConsoleReq) (*client.SSOLoginConsoleResponse, error) {
		if err := req.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		oidcRequest := types.OIDCAuthRequest{
			ConnectorID:       req.ConnectorID + "-" + connector.GetName(),
			Type:              constants.OIDC,
			CheckUser:         false,
			SshPublicKey:      req.SSHPubKey,
			TlsPublicKey:      req.TLSPubKey,
			CertTTL:           defaults.OIDCAuthRequestTTL,
			CreateWebSession:  false,
			ClientRedirectURL: req.RedirectURL,
			RouteToCluster:    req.RouteToCluster,
			SSOTestFlow:       true,
			ConnectorSpec:     &spec,
		}

		request, err := c.CreateOIDCAuthRequest(ctx, oidcRequest)
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

func getInfoFieldsOIDC(diag *types.SSODiagnosticInfo, debug bool) []string {
	return []string{
		GetDiagMessage(
			diag.OIDCClaims != nil,
			true,
			FormatJSON("[OIDC] Claims", diag.OIDCClaims)),
		GetDiagMessage(
			diag.OIDCClaimsToRoles != nil,
			true,
			FormatYAML("[OIDC] Claims to roles", diag.OIDCClaimsToRoles),
		),
		GetDiagMessage(
			diag.OIDCClaimsToRolesWarnings != nil,
			true,
			formatSSOWarnings("[OIDC] Claims to roles warnings", diag.OIDCClaimsToRolesWarnings),
		),
		GetDiagMessage(
			diag.OIDCTraitsFromClaims != nil,
			debug,
			FormatJSON("[OIDC] Calculated user traits", diag.OIDCTraitsFromClaims),
		),
		GetDiagMessage(
			diag.OIDCIdentity != nil,
			true,
			FormatJSON("[OIDC] Calculated identity", diag.OIDCIdentity),
		),
		GetDiagMessage(
			diag.OIDCConnectorTraitMapping != nil,
			debug,
			FormatYAML("[OIDC] Connector trait mapping", diag.OIDCConnectorTraitMapping),
		),
	}
}
