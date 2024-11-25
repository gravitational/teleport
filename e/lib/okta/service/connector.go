package oktaservice

import (
	"context"
	"fmt"
	"net/url"

	"github.com/gravitational/trace"

	oktapb "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/okta/common"
	"github.com/gravitational/teleport/e/lib/okta/common/sso"
	"github.com/gravitational/teleport/integrations/lib"
)

func validateOrganization(req *oktapb.CreateIntegrationRequest, connector *sso.SAMLConnectorInfo) error {
	reqOrgURL := req.GetOktaOrganizationUrl()
	if reqOrgURL == "" && req.GetSsoMetadataUrl() != "" {
		// If the organization is not provided, we can't validate it.
		// This is fine, as the organization is not required for the flow with metadata URL where the organization
		// extracted from the connector metadata.
		return nil
	}
	if reqOrgURL == "" {
		return trace.BadParameter("Okta organization URL is required")
	}
	if reqOrgURL != connector.OktaOrg {
		return trace.BadParameter(
			"The provided Okta organization URL %q does not match the expected Okta organization URL from the connector: %q.\n"+
				"Please ensure you're using the correct Okta organization URL that matches the Okta organization in the Okta connector %s.", reqOrgURL, connector.OktaOrg, connector.Connector.GetName())
	}
	return nil
}

func (s *Service) pluginInstallCreateSAMLConnector(ctx context.Context, req *oktapb.CreateIntegrationRequest, connectorName, clusterName string, publicURL *url.URL) (*sso.SAMLConnectorInfo, error) {
	if req.GetSsoMetadataUrl() != "" {
		// Create a new SAML connector from the metadata URL.
		// This is the flow where the SAML application is pre-created in the Okta organization.
		// And Teleport just needs to create a SAML connector from the metadata.
		connInfo, err := sso.CreateSAMLConnectorFromMetadatURL(ctx, sso.ConnectorArgs{
			ConnectorName:        connectorName,
			SAMLConnectorService: s.authService,
			ClusterName:          clusterName,
			PublicURL:            publicURL,
			Logger:               s.logger,
			MetadataURL:          req.GetSsoMetadataUrl(),
			HTTPClient:           s.roundTripper,
		})
		if err != nil {
			return nil, trace.Wrap(err, "creating new SAML connector")
		}
		return connInfo, nil
	}
	oktaClient, err := s.createOktaClientForPluginInstall(ctx, req, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Create a Teleport SAML application in the Okta organization.
	// and corresponding SAML connector in Teleport.
	// For that the okta.apps.manage is required to create the SAML app in Okta organization.
	// Also, the flow tries to Assign okta Groups Everyone and created Okta SAML app.
	connInfo, err := sso.CreateSAMLConnector(ctx, sso.ConnectorArgs{
		ConnectorName:        connectorName,
		OktaClient:           oktaClient,
		SAMLConnectorService: s.authService,
		ClusterName:          clusterName,
		PublicURL:            publicURL,
		Logger:               s.logger,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return connInfo, nil
}

func (s *Service) pluginInstallReuseExistingSAMLConnector(ctx context.Context, req *oktapb.CreateIntegrationRequest, samlConnector types.SAMLConnector) (*sso.SAMLConnectorInfo, error) {
	if req.GetApiCredentials() == nil {
		connInfo, err := s.buildBasicConnectorInfo(samlConnector)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if err := validateOrganization(req, connInfo); err != nil {
			return nil, trace.Wrap(err)
		}
		return connInfo, nil
	}
	oktaClient, err := s.createOktaClientForPluginInstall(ctx, req, samlConnector)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	connInfo, err := sso.ValidateSAMLConnector(ctx, samlConnector, oktaClient)
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
	return connInfo, nil
}

func (s *Service) getOrCreateSAMLConnector(ctx context.Context, req *oktapb.CreateIntegrationRequest) (*sso.SAMLConnectorInfo, error) {
	connectorName := req.GetReuseConnector()
	if connectorName == "" {
		connectorName = common.OktaSSOConnectorName
	}
	// Remove the MFA resp from the context before getting the connector.
	// Otherwise, it will be consumed before the Create which actually
	// requires the MFA.
	// TODO(Joerger): Explicitly provide MFA response only where it is
	// needed instead of removing it like this.
	ctxMFA := mfa.ContextWithMFAResponse(ctx, nil)
	pingInfo, err := s.authService.Ping(ctxMFA)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	publicURL, err := lib.AddrToURL(pingInfo.ProxyPublicAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	samlConnector, err := s.authService.GetSAMLConnector(ctxMFA, connectorName, false)
	switch {
	case err != nil && !trace.IsNotFound(err):
		return nil, trace.Wrap(err, "fetching SAML connector %s", connectorName)
	case trace.IsNotFound(err):
		connInfo, createErr := s.pluginInstallCreateSAMLConnector(ctx, req, connectorName, pingInfo.ClusterName, publicURL)
		if createErr != nil {
			if trace.IsAccessDenied(err) {
				return nil, trace.AccessDenied("Could not create Okta SAML application. Please ensure that your API token has \"Manage applications\" permission or, if your token is scoped to a resource set, that it grants access to all apps. These permissions are only required during this integration setup flow and can be removed afterwards.\n\n%s", err)
			}
			return nil, trace.Wrap(createErr, "creating SAML connector %s", connectorName)
		}
		return connInfo, nil
	default:
		// If the connector already exists, reuse it.
		// And try to fetch additional details about Okta setup like Okta Application Name or Okta Application ID
		// that are usable to display Okta plugins status page.
		connInfo, reuseErr := s.pluginInstallReuseExistingSAMLConnector(ctx, req, samlConnector)
		if reuseErr != nil {
			return nil, trace.Wrap(reuseErr, "reusing existing SAML connector %s", connectorName)
		}
		return connInfo, nil
	}
}
