package web

import (
	"context"
	"net/http"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/okta"
	"github.com/gravitational/teleport/e/lib/plugins"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/lib/web"
)

const (
	oktaSSOConnectorName = "okta-integration"
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
		StaticCredentials: &types.PluginStaticCredentialsV1{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{
					Labels: map[string]string{
						"okta/org-url": orgURL.String(),
					},
					Name: types.PluginTypeOkta,
				},
			},
			Spec: &types.PluginStaticCredentialsSpecV1{
				Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
					APIToken: apiToken,
				},
			},
		},
	}

	ui, err := installPlugin(ctx, sessCtx, req, p)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	p.Log.Infof("Creating SSO Connection to %s", orgURL)
	if err = createSSOConnector(ctx, sessCtx, orgURL.String(), apiToken); err != nil {
		p.Log.WithError(err).Error("Failed creating SSO connector.")
	}

	return ui, nil
}

func createSSOConnector(ctx context.Context, sessCtx *web.SessionContext, endpoint, apiToken string) error {
	log := logrus.WithField(trace.Component, teleport.Component(types.PluginTypeOkta))

	oktaClient, err := okta.NewClient(ctx, okta.ClientConfig{
		Endpoint:   endpoint,
		Token:      apiToken,
		Log:        log,
		StatusSink: nil})
	if err != nil {
		return trace.Wrap(err)
	}

	client, err := sessCtx.GetClient()
	if err != nil {
		return trace.Wrap(err)
	}

	pingInfo, err := client.Ping(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	publicURL, err := lib.AddrToURL(pingInfo.ProxyPublicAddr)
	if err != nil {
		return trace.Wrap(err)
	}

	err = okta.CreateSSOConnector(ctx, okta.ConnectorArgs{
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
		return trace.Wrap(err, "creating SSO connector")
	}

	return nil
}
