package oktaservice

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/e/lib/okta/api"
	"github.com/gravitational/teleport/e/lib/okta/common"
	"github.com/gravitational/teleport/e/lib/okta/common/sso"
	"github.com/gravitational/teleport/integrations/lib"
)

func (s *Service) getOrCreateSAMLConnector(ctx context.Context, oktaClient api.Client, connectorName string) (*sso.SAMLConnectorInfo, error) {
	// Remove the MFA resp from the context before pinging.
	// Otherwise, it will be consumed before the Create which actually
	// requires the MFA.
	// TODO(Joerger): Explicitly provide MFA response only where it is
	// needed instead of removing it like this.
	pingCtx := mfa.ContextWithMFAResponse(ctx, nil)
	pingInfo, err := s.authService.Ping(pingCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	publicURL, err := lib.AddrToURL(pingInfo.ProxyPublicAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if connectorName == "" {
		connectorName = common.OktaSSOConnectorName
	}

	// Remove the MFA resp from the context before getting the connector.
	// Otherwise, it will be consumed before the Create which actually
	// requires the MFA.
	// TODO(Joerger): Explicitly provide MFA response only where it is
	// needed instead of removing it like this.
	getConnectorCtx := mfa.ContextWithMFAResponse(ctx, nil)
	samlConnector, err := s.authService.GetSAMLConnector(getConnectorCtx, connectorName, false)
	if trace.IsNotFound(err) {
		connInfo, err := sso.CreateSAMLConnector(ctx, sso.ConnectorArgs{
			ConnectorName:        connectorName,
			OktaClient:           oktaClient,
			SAMLConnectorService: s.authService,
			ClusterName:          pingInfo.ClusterName,
			PublicURL:            publicURL,
			Logger:               s.logger,
		})
		if trace.IsAccessDenied(err) {
			return nil, trace.AccessDenied("Could not create Okta SAML application. Please ensure that your API token has \"Manage applications\" permission or, if your token is scoped to a resource set, that it grants access to all apps. These permissions are only required during this integration setup flow and can be removed afterwards.\n\n%s", err)
		}
		return connInfo, trace.Wrap(err, "creating new SAML connector")
	} else if err != nil {
		return nil, trace.Wrap(err, "fetching SAML connector %s", connectorName)
	}

	connectorInfo, err := sso.ValidateSAMLConnector(ctx, samlConnector, oktaClient)
	if err != nil {
		msg := err.Error()
		if trace.IsAccessDenied(err) {
			msg = fmt.Sprintf("Could not access Okta SAML application. Please ensure that your API token has \"Manage applications\" permission or, if your token is scoped to a resource set, that it grants access to all apps. These permissions are only required during this integration setup flow and can be removed afterwards.\n\n%s", err)
		}
		// Using the CompareFailed error here results in the HTTP response
		// returning http.StatusPreconditionFailed, which we can use as a signal
		// to the UI that the problem is the underlying SAML connector.
		return nil, trace.CompareFailed(msg)
	}
	return connectorInfo, nil
}
