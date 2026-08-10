package oktaservice

import (
	"context"
	"fmt"

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

func (s *Service) pluginInstallCreateSAMLConnector(ctx context.Context, req *oktapb.CreateIntegrationRequest, connectorName string) (*sso.SAMLConnectorInfo, error) {
	if req.GetSsoMetadataUrl() == "" {
		return nil, trace.BadParameter("SSO metadata URL must be provided to create the SAML connector")
	}

	// Remove the MFA resp from the context before getting the connector.
	// Otherwise, it will be consumed before the Create which actually
	// requires the MFA.
	// TODO(Joerger): Explicitly provide MFA response only where it is
	// needed instead of removing it like this.
	ctxNoMFA := mfa.ContextWithMFAResponse(ctx, nil)

	pingInfo, err := s.authService.Ping(ctxNoMFA)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	publicURL, err := lib.AddrToURL(pingInfo.ProxyPublicAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create a new SAML connector from the metadata URL.  The SAML application must be
	// pre-created in Okta and Teleport just needs to create a SAML connector from the
	// metadata.
	connInfo, err := sso.CreateSAMLConnectorFromMetadataURL(ctx, sso.ConnectorArgs{
		ConnectorName:        connectorName,
		DisplayName:          common.OktaSSOConnectorDisplay,
		SAMLConnectorService: s.authService,
		ClusterName:          pingInfo.ClusterName,
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
	oktaClient, err := s.createOktaClient(ctx, req, nil)
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

func getSAMLConnectorName(req *oktapb.CreateIntegrationRequest) string {
	if reuseConnector := req.GetReuseConnector(); reuseConnector != "" {
		return reuseConnector
	}
	return common.OktaSSOConnectorName
}

func (s *Service) getSAMLConnector(ctx context.Context, name string) (types.SAMLConnector, error) {
	// Remove the MFA resp from the context before getting the connector.
	// Otherwise, it will be consumed before the Create which actually
	// requires the MFA.
	// TODO(Joerger): Explicitly provide MFA response only where it is
	// needed instead of removing it like this.
	ctxNoMFA := mfa.ContextWithMFAResponse(ctx, nil)

	samlConnector, err := s.authService.GetSAMLConnector(ctxNoMFA, name, false /* withSecrets */)
	return samlConnector, trace.Wrap(err)
}

func (s *Service) ensureSAMLConnector(ctx context.Context, req *oktapb.CreateIntegrationRequest, samlConnector types.SAMLConnector) (*sso.SAMLConnectorInfo, error) {
	var err error
	var connInfo *sso.SAMLConnectorInfo

	samlConnectorName := getSAMLConnectorName(req)
	if samlConnector != nil {
		samlConnectorName = samlConnector.GetName()
	}

	if samlConnector == nil {
		connInfo, err = s.pluginInstallCreateSAMLConnector(ctx, req, samlConnectorName)
		if err != nil {
			if trace.IsAccessDenied(err) {
				return nil, trace.AccessDenied("Could not create Okta SAML application. Please ensure that your API Services app or your API token has \"Manage applications\" permission and the assigned resource set grants access to all apps. These permissions are only required during this integration setup flow and can be removed afterwards.\n\n%s", err)
			}
			return nil, trace.Wrap(err, "creating SAML connector %q", samlConnectorName)
		}
	} else {
		// If the connector already exists, reuse it.
		// And try to fetch additional details about Okta setup like Okta Application Name or Okta Application ID
		// that are usable to display Okta plugins status page.
		connInfo, err = s.pluginInstallReuseExistingSAMLConnector(ctx, req, samlConnector)
		if err != nil {
			return nil, trace.Wrap(err, "reusing existing SAML connector %q", samlConnectorName)
		}
	}

	// If credentials are provided Okta app ID is required because users assigned to the Okta
	// SAML app are the users that are being synchronized if user sync is enabled and user sync
	// is the minimal level of sync.
	if req.GetApiCredentials() != nil {
		oktaClient, err := s.createOktaClient(ctx, req, nil)
		if err != nil {
			return nil, trace.Wrap(err, "creating Okta client")
		}
		connInfo, err := sso.FetchOktaSAMLConnectorInfo(ctx, oktaClient, connInfo.Connector)
		if err != nil {
			return nil, trace.Wrap(err, "fetching Okta SAML app info")
		}
		return connInfo, nil
	}

	return connInfo, nil
}
