package web

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/bcrypt"

	"github.com/gravitational/teleport/api/client/proto"
	oktav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/lib/modules"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/teleport/lib/web"
)

// oktaPluginDescriptor is an empty type used to implement an Okta-specific
// version of the pluginDescriptor interface
type oktaPluginDescriptor struct{}

// Static assertion that oktaPluginDescriptor implements the pluginDescriptor
// interface
var _ pluginDescriptor = oktaPluginDescriptor{}

// HandleValidateConfigRequest tests the Okta client configuration supplied in
// the form.
func (oktaPluginDescriptor) HandleValidateConfigRequest(ctx context.Context, sessCtx *web.SessionContext, form url.Values, p *Plugin) error {
	clusterFeatures := p.h.GetClusterFeatures()
	args := validateOktaPluginInputsArgs{
		form:            form,
		clusterFeatures: &clusterFeatures,
		logger:          p.Logger,
	}
	_, err := args.validateOktaConfig(ctx, sessCtx)
	return trace.Wrap(err)
}

// HandleInstallRequest installs the Okta plugin
func (oktaPluginDescriptor) HandleInstallRequest(ctx context.Context, sessCtx *web.SessionContext, w http.ResponseWriter, r *http.Request, p *Plugin) (*ui.Plugin, error) {
	clusterFeatures := p.h.GetClusterFeatures()
	return installOktaPlugin(ctx, installOktaPluginArgs{
		validateOktaPluginInputsArgs: validateOktaPluginInputsArgs{
			form:            r.Form,
			clusterFeatures: &clusterFeatures,
			logger:          p.Logger,
		},
		sessCtx: sessCtx,
		plugin:  p,
	})
}

// TranslateCallbackCookie implements PluginDescriptor for oktaPluginDescriptor,
// always returning "Not Implemented".
func (oktaPluginDescriptor) TranslateCallbackCookie(*types.PluginSpecV1, *pluginOnboardingCookie) error {
	return trace.NotImplemented("TranslateCallbackCookie")
}

// installOktaPluginArgs contains all of the options for installing the Okta
// plugin
type installOktaPluginArgs struct {
	validateOktaPluginInputsArgs
	sessCtx *web.SessionContext
	plugin  *Plugin

	// signingKeypair is an optional keypair to use when creating the SAML
	// connector. A sensible default will be created if not supplied.
	signingKeypair *types.AsymmetricKeyPair
}

// CheckAndSetDefaults checks the supplied [plugin args, providing default
// values as necessary
func (args *installOktaPluginArgs) CheckAndSetDefaults() error {
	if err := args.validateOktaPluginInputsArgs.CheckAndSetDefaults(); err != nil {
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

func installOktaPlugin(ctx context.Context, args installOktaPluginArgs) (*ui.Plugin, error) {
	params, err := validateOktaPluginInputs(ctx, args.validateOktaPluginInputsArgs, args.sessCtx)
	if err != nil {
		return nil, trace.Wrap(err, "validating okta parameters")
	}

	oktaAPICreds, err := getOktaCredsFromParams(params)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authOktaClient := oktav1.NewOktaServiceClient(args.sessCtx.GetClientConnection())
	resp, err := authOktaClient.CreateIntegration(ctx, &oktav1.CreateIntegrationRequest{
		OktaOrganizationUrl:  params.oktaOrgURL,
		ApiCredentials:       oktaAPICreds,
		ScimToken:            params.scimBearerToken,
		EnableAccessListSync: params.enableAccessListSync,
		EnableUserSync:       true,
		EnableAppGroupSync:   true,
		AccessListSettings: &oktav1.AccessListSettings{
			GroupFilters: params.groupFilters,
			AppFilters:   params.appFilters,
			DefaultOwner: params.defaultOwners,
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	uiPlugin, err := ui.NewPlugin(resp.Plugin)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	uiPlugin.Spec = &ui.OktaPluginSpec{
		SCIMBearerToken:      params.scimBearerToken,
		OktaAppID:            resp.GetConnectorInfo().GetOktaAppId(),
		OktaAppName:          resp.GetConnectorInfo().GetOktaAppName(),
		OktaAppLabel:         resp.GetConnectorInfo().GetOktaAppLabels(),
		TeleportSSOConnector: resp.GetConnectorInfo().GetTeleportConnectorName(),
	}
	return uiPlugin, nil
}

func getOktaCredsFromParams(params *oktaPluginInputs) (*oktav1.OktaAPICredentials, error) {
	var apiCreds *oktav1.OktaAPICredentials
	switch {
	case params.oauthClientID != "":
		apiCreds = &oktav1.OktaAPICredentials{
			Auth: &oktav1.OktaAPICredentials_OauthId{
				OauthId: params.oauthClientID,
			},
		}
	case params.oktaAPIToken != "":
		apiCreds = &oktav1.OktaAPICredentials{
			Auth: &oktav1.OktaAPICredentials_SswsBearerToken{
				SswsBearerToken: params.oktaAPIToken,
			},
		}
	default:
		return nil, trace.BadParameter("missing Okta API token or OAuth client ID")
	}
	return apiCreds, nil
}

type oktaPluginInputs struct {
	oktaOrgURL           string
	oktaAPIToken         string
	scimBearerToken      string
	scimBearerTokenHash  string
	groupFilters         []string
	appFilters           []string
	defaultOwners        []string
	enableAccessListSync bool
	oauthClientID        string
}

type validateOktaPluginInputsArgs struct {
	form            url.Values
	httpClient      *http.Client
	clusterFeatures *proto.Features
	logger          *slog.Logger

	// bcryptCost is the bcryptCost to be used for hashing the SCIM user token.
	// Defaults to bcrypt.DefaultCost if unset.
	bcryptCost int
}

func (args *validateOktaPluginInputsArgs) CheckAndSetDefaults() error {
	// NOTE: args.httpClient may legitimately be nil. An appropriate default
	//       client will be created when the Okta client is created.
	if args.form == nil {
		return trace.BadParameter("form must be supplied")
	}
	if args.clusterFeatures == nil {
		return trace.BadParameter("cluster features must be supplied")
	}
	if args.logger == nil {
		args.logger = slog.Default()
	}
	args.bcryptCost = min(args.bcryptCost, bcrypt.MaxCost)
	if args.bcryptCost < bcrypt.MinCost {
		args.bcryptCost = bcrypt.DefaultCost
	}
	return nil
}

func getOktaAuth(apiToken, clientID string) (*oktav1.OktaAPICredentials, error) {
	switch {
	case clientID != "":
		return &oktav1.OktaAPICredentials{
			Auth: &oktav1.OktaAPICredentials_OauthId{
				OauthId: clientID,
			},
		}, nil

	case apiToken != "":
		return &oktav1.OktaAPICredentials{
			Auth: &oktav1.OktaAPICredentials_SswsBearerToken{
				SswsBearerToken: apiToken,
			},
		}, nil
	default:
		return nil, trace.BadParameter("missing Okta API token or OAuth client ID")
	}
}

// validateOktaConfig extracts the Okta configuration inputs to the Okta plugin
// installer from the supplied form and validates them with a live request to
// the Okta organization.
func (args *validateOktaPluginInputsArgs) validateOktaConfig(ctx context.Context, sessCtx *web.SessionContext) (*oktaPluginInputs, error) {
	if err := args.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	args.logger.Log(ctx, logutils.TraceLevel, "Extracting Okta client config")

	oktaOrgURLText := args.form.Get("orgURL")
	if oktaOrgURLText == "" {
		return nil, trace.BadParameter("missing Okta organization URL")
	}

	// If the user supplies an invalid URL or bare hostname, then the
	// integration will appear to install but fail to start with obscure
	// errors only visible in the Teleport log file.
	//
	// To avoid this, we helpfully supply a sensible-default `https`
	// scheme if necessary
	orgURL, err := lib.AddrToURL(oktaOrgURLText)
	if err != nil {
		return nil, trace.BadParameter("malformed Okta url")
	}

	oktaAPIToken := args.form.Get("apiToken")
	clientID := args.form.Get("clientID")
	creds, err := getOktaAuth(oktaAPIToken, clientID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authOktaClient := oktav1.NewOktaServiceClient(sessCtx.GetClientConnection())
	if _, err = authOktaClient.ValidateClientCredentials(ctx, &oktav1.ValidateClientCredentialsRequest{
		OktaOrganizationUrl: orgURL.String(),
		ApiCredentials:      creds,
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	return &oktaPluginInputs{
		oktaOrgURL:   orgURL.String(),
		oktaAPIToken: oktaAPIToken,
	}, nil
}

func validateOktaPluginInputs(ctx context.Context, args validateOktaPluginInputsArgs, sessCtx *web.SessionContext) (*oktaPluginInputs, error) {
	const (
		// minTokenLength is the minimum length an SCIM token should be.
		minTokenLength = 32
		// maxTokenLength is the maximum length accepted by bcrypt, an
		// SCIM token can't be longer than this.
		maxTokenLength = 72
	)
	if err := args.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	params, err := args.validateOktaConfig(ctx, sessCtx)
	if err != nil {
		return nil, trace.Wrap(err, "invalid Okta config")
	}

	if modules.GetProtoEntitlement(args.clusterFeatures, entitlements.OktaSCIM).Enabled {
		params.enableAccessListSync = true

		params.scimBearerToken = args.form.Get("scimToken")
		if params.scimBearerToken == "" {
			return nil, trace.BadParameter("missing SCIM bearer token")
		}
		if len(params.scimBearerToken) < minTokenLength {
			return nil, trace.BadParameter("SCIM bearer token must be at least %d characters", minTokenLength)
		}
		if len(params.scimBearerToken) > maxTokenLength {
			return nil, trace.BadParameter("SCIM bearer token must be no longer than %d characters", maxTokenLength)
		}

		scimTokenHash, err := bcrypt.GenerateFromPassword([]byte(params.scimBearerToken), args.bcryptCost)
		if err != nil {
			return nil, trace.BadParameter("hashing SCIM bearer token")
		}
		params.scimBearerTokenHash = string(scimTokenHash)

		var defaultOwners []string
		defaultOwnersString := args.form.Get("defaultOwners")
		if defaultOwnersString != "" {
			defaultOwners = []string{}
			if err := json.Unmarshal([]byte(defaultOwnersString), &defaultOwners); err != nil {
				return nil, trace.Wrap(err)
			}
		}
		params.defaultOwners = defaultOwners
		if defaultOwners == nil {
			params.enableAccessListSync = false
			return params, nil
		}

		params.appFilters, err = getOktaAppFilters(args.form)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		params.groupFilters, err = getOktaGroupFilters(args.form)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return params, nil
}

func getOktaFilters(filterString string) ([]string, error) {
	var filters []string
	if filterString != "" {
		filters = []string{}
		if err := json.Unmarshal([]byte(filterString), &filters); err != nil {
			return []string{}, trace.Wrap(err)
		}
	}

	return filters, nil
}

func getOktaGroupFilters(form url.Values) ([]string, error) {
	return getOktaFilters(form.Get("groupFilters"))
}

func getOktaAppFilters(form url.Values) ([]string, error) {
	return getOktaFilters(form.Get("appFilters"))
}
