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
	"github.com/gravitational/teleport/tool/tctl/sso/tester"
)

func handleOIDCConnector(c auth.ClientI, connBytes []byte) (*tester.AuthRequestInfo, error) {
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

func oidcTest(c auth.ClientI, connector types.OIDCConnector) (*tester.AuthRequestInfo, error) {
	ctx := context.Background()
	// get connector spec
	var spec types.OIDCConnectorSpecV3
	switch oidcConnector := connector.(type) {
	case *types.OIDCConnectorV3:
		spec = oidcConnector.Spec
	default:
		return nil, trace.BadParameter("Unrecognized oidc connector version: %T. Provide supported connector version.", oidcConnector)
	}

	requestInfo := &tester.AuthRequestInfo{}

	makeRequest := func(req client.SSOLoginConsoleReq) (*client.SSOLoginConsoleResponse, error) {
		oidcRequest := types.OIDCAuthRequest{
			ConnectorID:       req.ConnectorID + "-" + connector.GetName(),
			Type:              constants.OIDC,
			CheckUser:         false,
			PublicKey:         req.PublicKey,
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

	requestInfo.Config = &client.RedirectorConfig{SSOLoginConsoleRequestFn: makeRequest}
	return requestInfo, nil
}

func getInfoFieldsOIDC(diag *types.SSODiagnosticInfo, debug bool) []string {
	return []string{
		tester.GetDiagMessage(
			diag.OIDCClaims != nil,
			true,
			tester.FormatJSON("[OIDC] Claims", diag.OIDCClaims)),
		tester.GetDiagMessage(
			diag.OIDCClaimsToRoles != nil,
			true,
			tester.FormatYAML("[OIDC] Claims to roles", diag.OIDCClaimsToRoles),
		),
		tester.GetDiagMessage(
			diag.OIDCClaimsToRolesWarnings != nil,
			true,
			formatSSOWarnings("[OIDC] Claims to roles warnings", diag.OIDCClaimsToRolesWarnings),
		),
		tester.GetDiagMessage(
			diag.OIDCTraitsFromClaims != nil,
			debug,
			tester.FormatJSON("[OIDC] Calculated user traits", diag.OIDCTraitsFromClaims),
		),
		tester.GetDiagMessage(
			diag.OIDCIdentity != nil,
			true,
			tester.FormatJSON("[OIDC] Calculated identity", diag.OIDCIdentity),
		),
		tester.GetDiagMessage(
			diag.OIDCConnectorTraitMapping != nil,
			debug,
			tester.FormatYAML("[OIDC] Connector trait mapping", diag.OIDCConnectorTraitMapping),
		),
	}
}
