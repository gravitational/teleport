package web

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/client/proto"
	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/netiq"
	netiqclient "github.com/gravitational/teleport/e/lib/netiq/client"
	"github.com/gravitational/teleport/e/lib/plugins"
	"github.com/gravitational/teleport/e/lib/web/ui"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/teleport/lib/web"
)

// netIQPluginDescriptor is an empty type used to implement an NetIQ-specific
// version of the pluginDescriptor interface
type netIQPluginDescriptor struct{}

// Static assertion that netIQPluginDescriptor implements the pluginDescriptor
// interface
var _ pluginDescriptor = netIQPluginDescriptor{}

// HandleValidateConfigRequest tests the NetIQ client configuration supplied in
// the form.
func (netIQPluginDescriptor) HandleValidateConfigRequest(ctx context.Context, sessCtx *web.SessionContext, form url.Values, p *Plugin) error {
	clusterFeatures := p.h.GetClusterFeatures()
	args := validateNetIQPluginInputsArgs{
		form:            form,
		clusterFeatures: &clusterFeatures,
		logger:          p.Logger,
	}
	_, err := args.validateNetIQConfig(ctx)
	return trace.Wrap(err)
}

// HandleInstallRequest installs the NetIQ plugin
func (netIQPluginDescriptor) HandleInstallRequest(ctx context.Context, sessCtx *web.SessionContext, w http.ResponseWriter, r *http.Request, p *Plugin) (*ui.Plugin, error) {
	clusterFeatures := p.h.GetClusterFeatures()
	return installNetIQPlugin(ctx, installNetIQPluginArgs{
		validateNetIQPluginInputsArgs: validateNetIQPluginInputsArgs{
			form:            r.Form,
			clusterFeatures: &clusterFeatures,
			logger:          p.Logger,
		},
		sessCtx: sessCtx,
		plugin:  p,
	})
}

// TranslateCallbackCookie implements PluginDescriptor for netIQPluginDescriptor,
// always returning "Not Implemented".
func (netIQPluginDescriptor) TranslateCallbackCookie(*types.PluginSpecV1, *pluginOnboardingCookie) error {
	return trace.NotImplemented("TranslateCallbackCookie")
}

// HandleOAuthStart implements PluginDescriptor for netIQPluginDescriptor, always
// returning "Not Implemented".
func (netIQPluginDescriptor) HandleOAuthStart(ctx context.Context, sessCtx *web.SessionContext, w http.ResponseWriter, r *http.Request, p *Plugin) (*ui.OAuthPluginStartResponse, error) {
	return nil, trace.NotImplemented("HandleOAuthStart")
}

// installNetIQPluginArgs contains all of the options for installing the NetIQ
// plugin
type installNetIQPluginArgs struct {
	validateNetIQPluginInputsArgs
	sessCtx *web.SessionContext
	plugin  *Plugin
}

// CheckAndSetDefaults checks the supplied [plugin args, providing default
// values as necessary
func (args *installNetIQPluginArgs) CheckAndSetDefaults() error {
	if err := args.validateNetIQPluginInputsArgs.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if args.sessCtx == nil {
		return trace.BadParameter("must supply session context")
	}

	if args.plugin == nil {
		return trace.BadParameter("must supply web ui plugin")
	}

	return nil
}

func installNetIQPlugin(ctx context.Context, args installNetIQPluginArgs) (*ui.Plugin, error) {
	params, err := validateNetIQPluginInputs(ctx, args.validateNetIQPluginInputsArgs)
	if err != nil {
		return nil, trace.Wrap(err, "validating NetIQ parameters")
	}

	creds, err := getNetIQPluginCredentials(params)
	if err != nil {
		return nil, trace.Wrap(err, "failed to get NetIQ plugin credentials")
	}

	plugin, err := createNetIQPlugin(params)
	if err != nil {
		return nil, trace.Wrap(err, "failed to create NetIQ plugin")
	}

	createPluginRequest := pluginspb.CreatePluginRequest_builder{
		Plugin:                plugin,
		StaticCredentialsList: creds,
		CredentialLabels: map[string]string{
			netiq.NetIQOrgURLLabel: params.apiURL,
		},
	}.Build()

	cl, err := args.sessCtx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err, "failed to get client")
	}

	if _, err = cl.PluginsClient().CreatePlugin(ctx, createPluginRequest); err != nil {
		return nil, trace.Wrap(err, "failed to create NetIQ plugin")
	}

	uiPlugin, err := ui.NewPlugin(plugin)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return uiPlugin, nil
}

type netIQPluginInputs struct {
	oAuthClientID         string
	oAuthClientSecret     string
	ospURL                string
	apiURL                string
	identityVaultUser     string
	identityVaultPassword string
	insecure              bool
}

type validateNetIQPluginInputsArgs struct {
	form            url.Values
	clusterFeatures *proto.Features
	logger          *slog.Logger
}

func (args *validateNetIQPluginInputsArgs) CheckAndSetDefaults() error {
	// NOTE: args.httpClient may legitimately be nil. An appropriate default
	//       client will be created when the NetIQ client is created.
	if args.form == nil {
		return trace.BadParameter("form must be supplied")
	}
	if args.clusterFeatures == nil {
		return trace.BadParameter("cluster features must be supplied")
	}
	if args.logger == nil {
		args.logger = slog.Default()
	}

	return nil
}

// validateNetIQConfig extracts the NetIQ configuration inputs to the NetIQ plugin
// installer from the supplied form values, and validates the configuration by
// attempting to connect to the NetIQ API.
func (args *validateNetIQPluginInputsArgs) validateNetIQConfig(ctx context.Context) (*netIQPluginInputs, error) {
	if err := args.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	args.logger.Log(ctx, logutils.TraceLevel, "Extracting NetIQ client config")

	getAndValidateURLFromForm := func(key, name string) (string, error) {
		value := args.form.Get(key)
		if value == "" {
			return "", trace.BadParameter("missing %s", name)
		}
		if _, err := url.Parse(value); err != nil {
			return "", trace.BadParameter("invalid %s: %v", name, err)
		}
		return value, nil
	}

	osURL, err := getAndValidateURLFromForm("ospURL", "OSP URL")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	apiURL, err := getAndValidateURLFromForm("apiURL", "API URL")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	identityVaultUser := args.form.Get("identityVaultUser")
	if identityVaultUser == "" {
		return nil, trace.BadParameter("missing Identity Vault user")
	}

	identityVaultPassword := args.form.Get("identityVaultPassword")
	if identityVaultPassword == "" {
		return nil, trace.BadParameter("missing Identity Vault password")
	}

	oAuthClientID := args.form.Get("oAuthClientID")
	if oAuthClientID == "" {
		return nil, trace.BadParameter("missing OAuth client ID")
	}

	oAuthClientSecret := args.form.Get("oAuthClientSecret")
	if oAuthClientSecret == "" {
		return nil, trace.BadParameter("missing OAuth client secret")
	}

	insecure := args.form.Get("insecure") == "true"

	{
		client, err := netiqclient.New(
			ctx,
			netiqclient.Config{
				OSPURL:                osURL,
				APIURL:                apiURL,
				IdentityVaultUser:     identityVaultUser,
				IdentityVaultPassword: identityVaultPassword,
				InsecureSkipVerify:    insecure,
				OAuthClientID:         oAuthClientID,
				OAuthClientSecret:     oAuthClientSecret,
				Clock:                 clockwork.NewRealClock(),
			},
		)
		if err != nil {
			return nil, trace.Wrap(err, "failed to build NetIQ client. Check your configuration.")
		}

		if _, err := client.ListUsers(ctx); err != nil {
			return nil, trace.Wrap(err, "failed to retrieve NetIQ users. Check your configuration.")
		}
	}

	return &netIQPluginInputs{
		ospURL:                osURL,
		apiURL:                apiURL,
		oAuthClientID:         oAuthClientID,
		oAuthClientSecret:     oAuthClientSecret,
		identityVaultUser:     identityVaultUser,
		identityVaultPassword: identityVaultPassword,
		insecure:              insecure,
	}, nil
}

func validateNetIQPluginInputs(ctx context.Context, args validateNetIQPluginInputsArgs) (*netIQPluginInputs, error) {
	if err := args.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	params, err := args.validateNetIQConfig(ctx)
	return params, trace.Wrap(err, "invalid NetIQ config")
}

func getNetIQPluginCredentials(req *netIQPluginInputs) ([]*types.PluginStaticCredentialsV1, error) {
	var out []*types.PluginStaticCredentialsV1
	if req.oAuthClientID != "" {
		out = append(out, buildOAuthCredentials(req.oAuthClientID, req.oAuthClientSecret))
	}
	if req.identityVaultUser != "" {
		out = append(out, buildBasicAuthCredentials(req.identityVaultUser, req.identityVaultPassword))
	}

	return out, nil
}

func buildOAuthCredentials(clientID, clientSecret string) *types.PluginStaticCredentialsV1 {
	return &types.PluginStaticCredentialsV1{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name:   types.PluginTypeNetIQ + "-oauth",
				Labels: map[string]string{netiq.CredPurposeLabel: netiq.CredPurposeNetIQOauth},
			},
		},
		Spec: &types.PluginStaticCredentialsSpecV1{
			Credentials: &types.PluginStaticCredentialsSpecV1_OAuthClientSecret{
				OAuthClientSecret: &types.PluginStaticCredentialsOAuthClientSecret{
					ClientId:     clientID,
					ClientSecret: clientSecret,
				},
			},
		},
	}
}

func buildBasicAuthCredentials(user, password string) *types.PluginStaticCredentialsV1 {
	return &types.PluginStaticCredentialsV1{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name:   types.PluginTypeNetIQ + "-basic-auth",
				Labels: map[string]string{netiq.CredPurposeLabel: netiq.CredPurposeNetIQOauth},
			},
		},
		Spec: &types.PluginStaticCredentialsSpecV1{
			Credentials: &types.PluginStaticCredentialsSpecV1_BasicAuth{
				BasicAuth: &types.PluginStaticCredentialsBasicAuth{
					Username: user,
					Password: password,
				},
			},
		},
	}
}

func createNetIQPlugin(params *netIQPluginInputs) (*types.PluginV1, error) {
	plugin := &types.PluginV1{
		SubKind: types.PluginSubkindAccessGraph,
		Metadata: types.Metadata{
			Labels: map[string]string{
				plugins.HostedPluginLabel: "true",
			},
			Name: types.PluginTypeNetIQ,
		},
		Spec: types.PluginSpecV1{
			Settings: &types.PluginSpecV1_NetIq{
				NetIq: &types.PluginNetIQSettings{
					OauthIssuerEndpoint: params.ospURL,
					ApiEndpoint:         params.apiURL,
					InsecureSkipVerify:  params.insecure,
				},
			},
		},
	}

	return plugin, nil
}
