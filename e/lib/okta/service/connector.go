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
	if reqOrgURL == "" {
		// If the organization is not provided, we can't validate it.
		// This is fine, as the organization is not required for the flow with metadata URL where the organization
		// extracted from the connector metadata.
		if req.GetSsoMetadataUrl() != "" {
			return nil
		}
		// If no OrgURL is provided, but the connector has an OktaOrg, we can use that.
		if connector.OktaOrg != "" {
			return nil
		}
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
		connInfo, err := sso.CreateSAMLConnectorFromMetadataURL(ctx, sso.ConnectorArgs{
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

	connInfo, err := sso.ValidateSAMLConnector(ctx, oktaClient, samlConnector)
	if err != nil {
		msg := err.Error()
		if trace.IsAccessDenied(err) {
			msg = fmt.Sprintf("Could not access Okta SAML application. Please ensure that your API Services app or your API token has \"Manage applications\" permission and the assigned resource set grants access to all apps. These permissions are only required during this integration setup flow and can be removed afterwards.\n\n%s", err)
		}
		// Using the CompareFailed error here results in the HTTP response
		// returning http.StatusPreconditionFailed, which we can use as a signal
		// to the UI that the problem is the underlying SAML connector.
		return nil, trace.CompareFailed("%s", msg)
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
	ctxNoMFA := mfa.ContextWithMFAResponse(ctx, nil)

	var connInfo *sso.SAMLConnectorInfo
	samlConnector, err := s.authService.GetSAMLConnector(ctxNoMFA, connectorName, false)
	switch {
	case err != nil && !trace.IsNotFound(err):
		return nil, trace.Wrap(err, "fetching SAML connector %q", connectorName)
	case trace.IsNotFound(err):
		pingInfo, err := s.authService.Ping(ctxNoMFA)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		publicURL, err := lib.AddrToURL(pingInfo.ProxyPublicAddr)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		connInfo, err = s.pluginInstallCreateSAMLConnector(ctx, req, connectorName, pingInfo.ClusterName, publicURL)
		if err != nil {
			if trace.IsAccessDenied(err) {
				return nil, trace.AccessDenied("Could not create Okta SAML application. Please ensure that your API Services app or your API token has \"Manage applications\" permission and the assigned resource set grants access to all apps. These permissions are only required during this integration setup flow and can be removed afterwards.\n\n%s", err)
			}
			return nil, trace.Wrap(err, "creating SAML connector %q", connectorName)
		}
	default:
		orgUrl, err := sso.ExtractOktaOrganizationFromURL(samlConnector.GetSSO())
		if err != nil {
			return nil, trace.Wrap(err, "extracting Okta org URL from connector %q", samlConnector.GetName())
		}
		if req.GetOktaOrganizationUrl() == "" {
			req.OktaOrganizationUrl = orgUrl
		} else if req.GetOktaOrganizationUrl() != orgUrl {
			return nil, trace.BadParameter("request Okta org URL %q does not match existing connector Okta org URL %q", req.GetOktaOrganizationUrl(), orgUrl)
		}
		// If the connector already exists, reuse it.
		// And try to fetch additional details about Okta setup like Okta Application Name or Okta Application ID
		// that are usable to display Okta plugins status page.
		connInfo, err = s.pluginInstallReuseExistingSAMLConnector(ctx, req, samlConnector)
		if err != nil {
			return nil, trace.Wrap(err, "reusing existing SAML connector %q", connectorName)
		}
	}

	// If credentials are provided Okta app ID is required because users assigned to the Okta
	// SAML app are the users that are being synchronized if user sync is enabled and user sync
	// is the minimal level of sync.
	if req.GetApiCredentials() != nil {
		err := s.setAppId(ctx, connInfo, req)
		if err != nil {
			return nil, trace.Wrap(err, "fetching Okta SAML app ID")
		}
	}

	return connInfo, nil
}

// setAppId fetches the Okta SAML app ID for the connector and sets it in the connector info if it
// isn't already set.
func (s *Service) setAppId(ctx context.Context, info *sso.SAMLConnectorInfo, req *oktapb.CreateIntegrationRequest) error {
	if info.OktaAppID != "" {
		// App ID already set.
		return nil
	}
	if info.Connector == nil {
		return trace.BadParameter("connector missing in the connector info")
	}
	if req.GetApiCredentials() == nil {
		return trace.BadParameter("API credentials missing in create integration request")
	}
	if req.GetOktaOrganizationUrl() == "" {
		return trace.BadParameter("Okta organization URL missing in the create integration request")
	}

	createOktaClientParams := &createOktaClientParams{
		credsFromReq:     req.GetApiCredentials(),
		oktaOrganization: req.GetOktaOrganizationUrl(),
	}
	oktaClient, err := s.createOktaClient(ctx, createOktaClientParams)
	if err != nil {
		return trace.Wrap(err)
	}

	appId, err := sso.FetchOktaAppIdFromConnector(ctx, oktaClient, info.Connector)
	if err != nil {
		return trace.Wrap(err)
	}

	info.OktaAppID = appId
	return nil
}
