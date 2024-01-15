package web

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"

	"github.com/gravitational/teleport"
	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/okta"
	"github.com/gravitational/teleport/e/lib/plugins"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/lib/web"
)

const (
	oktaSSOConnectorName = "okta-integration"

	// oktaSCIMTokenBytes is the number of random bytes used to generate the
	// SCIM bearer token. The actual token will be longer due to Base64
	// encoding.
	oktaSCIMTokenBytes = 40

	// oktaSCIMTokenName is the name for the credential that will hold the SCIM
	// bearer token
	oktaSCIMTokenName = types.PluginTypeOkta + "-scim-token"
)

func installOktaPlugin(ctx context.Context, sessCtx *web.SessionContext, w http.ResponseWriter, r *http.Request, p *Plugin) (*ui.Plugin, error) {
	orgURLText := r.FormValue("orgURL")
	apiToken := r.FormValue("apiToken")

	// If the user supplies an invalid URL or bare hostame, then the
	// integration will appear to install but fail to start with obscure
	// errors only visible in the Teleport log file.
	//
	// To avoid this, we helpfully supply a sensible-default `https`
	// scheme if necessary
	orgURL, err := lib.AddrToURL(orgURLText)
	if err != nil {
		return nil, trace.Wrap(err, "malformed Okta url")
	}

	scimToken, err := generateSCIMBearerToken()
	if err != nil {
		return nil, trace.Wrap(err, "generating SCIM token")
	}

	scimTokenHash, err := bcrypt.GenerateFromPassword([]byte(scimToken), bcrypt.DefaultCost)
	if err != nil {
		return nil, trace.Wrap(err, "generating SCIM token")
	}

	req := &pluginspb.CreatePluginRequest{
		Plugin: &types.PluginV1{
			SubKind: types.PluginSubkindAccess,
			Metadata: types.Metadata{
				Labels: map[string]string{
					plugins.HostedPluginLabel: "true",
				},
				Name: types.PluginTypeOkta,
			},
			Spec: types.PluginSpecV1{
				Settings: &types.PluginSpecV1_Okta{
					Okta: &types.PluginOktaSettings{
						OrgUrl:         orgURL.String(),
						SsoConnectorId: oktaSSOConnectorName,
						EnableUserSync: true,
					},
				},
			},
		},
		StaticCredentialsList: []*types.PluginStaticCredentialsV1{
			// The Okta API token that Teleport will use to authenticate wth Okta
			{
				ResourceHeader: types.ResourceHeader{
					Metadata: types.Metadata{
						Name: types.PluginTypeOkta,
						Labels: map[string]string{
							okta.CredPurposeLabel: okta.CredPurposeOktaAuth,
						},
					},
				},
				Spec: &types.PluginStaticCredentialsSpecV1{
					Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
						APIToken: apiToken,
					},
				},
			},
			// The bearer token that Okta will use to authenticate with Teleport
			// when provisioning users & groups with SCIM
			{
				ResourceHeader: types.ResourceHeader{
					Metadata: types.Metadata{
						Name: oktaSCIMTokenName,
						Labels: map[string]string{
							okta.CredPurposeLabel: okta.CredPurposeSCIMToken,
						},
					},
				},
				Spec: &types.PluginStaticCredentialsSpecV1{
					Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
						APIToken: string(scimTokenHash),
					},
				},
			},
		},
		CredentialLabels: map[string]string{
			eteleport.OktaOrgURLLabel: orgURL.String(),
		},
	}

	uiPlugin, err := installPlugin(ctx, sessCtx, req, p)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	oktaSpec := &ui.OktaPluginSpec{}
	uiPlugin.Spec = oktaSpec

	p.Log.Infof("Creating SSO Connection to %s", orgURL)
	connectorInfo, err := createSSOConnector(ctx, sessCtx, orgURL.String(), apiToken)
	if err != nil {
		// Failure to create the SSO connector and Okta app is not considered a
		// big enough reason to  call the installation off; the okta sync
		// service can still run without it.
		//
		// We should, however, let the UI know that something bad has happened
		// so it can give guidance, rather than leave the user wondering what's
		// going on.
		p.Log.WithError(err).Error("Failed creating SSO connector.")
		if trace.IsAlreadyExists(err) {
			oktaSpec.Error = fmt.Sprintf("SSO connector %s already exists.", oktaSSOConnectorName)
		} else {
			oktaSpec.Error = err.Error()
		}
		return uiPlugin, nil
	}

	oktaSpec.SCIMBearerToken = scimToken
	oktaSpec.TeleportSSOConnector = oktaSSOConnectorName
	oktaSpec.OktaAppID = connectorInfo.OktaAppID
	oktaSpec.OktaAppName = connectorInfo.OktaAppName

	return uiPlugin, nil
}

func generateSCIMBearerToken() (string, error) {
	buf := make([]byte, oktaSCIMTokenBytes)
	if _, err := cryptorand.Read(buf); err != nil {
		return "", trace.Wrap(err)
	}

	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func createSSOConnector(ctx context.Context, sessCtx *web.SessionContext, endpoint, apiToken string) (*okta.SSOConnectorInfo, error) {
	log := logrus.WithField(trace.Component, teleport.Component(types.PluginTypeOkta))

	oktaClient, err := okta.NewClient(ctx, okta.ClientConfig{
		Endpoint:   endpoint,
		Token:      apiToken,
		Log:        log,
		StatusSink: nil})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	client, err := sessCtx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pingInfo, err := client.Ping(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	publicURL, err := lib.AddrToURL(pingInfo.ProxyPublicAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	connectorInfo, err := okta.CreateSSOConnector(ctx, okta.ConnectorArgs{
		ConnectorName:        oktaSSOConnectorName,
		OktaClient:           oktaClient,
		SAMLConnectorService: client,
		ClusterName:          pingInfo.ClusterName,
		PublicURL:            publicURL,
		Log:                  log,
	})

	if err != nil {
		if trace.IsAlreadyExists(err) {
			log.Warnf("Did not create connector %q. Connector already exists.",
				oktaSSOConnectorName)
		}
		return nil, trace.Wrap(err, "creating SSO connector")
	}

	return connectorInfo, nil
}
