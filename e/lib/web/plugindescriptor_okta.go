package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/okta"
	"github.com/gravitational/teleport/e/lib/plugins"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/web"
)

const (
	oktaSSOConnectorName = "okta-integration"

	// oktaSCIMTokenName is the name for the credential that will hold the SCIM
	// bearer token
	oktaSCIMTokenName = types.PluginTypeOkta + "-scim-token"
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
	args := validateOktaPluginInputsArgs{
		form:            form,
		clusterFeatures: &p.h.ClusterFeatures,
		log:             p.Log,
	}
	_, err := args.validateOktaConfig(ctx)
	return trace.Wrap(err)
}

// HandleInstallRequest installs the Okta plugin
func (oktaPluginDescriptor) HandleInstallRequest(ctx context.Context, sessCtx *web.SessionContext, w http.ResponseWriter, r *http.Request, p *Plugin) (*ui.Plugin, error) {
	return installOktaPlugin(ctx, installOktaPluginArgs{
		validateOktaPluginInputsArgs: validateOktaPluginInputsArgs{
			form:            r.Form,
			clusterFeatures: &p.h.ClusterFeatures,
			log:             p.Log,
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

// installOktaPluginWithHTTPClient allows the caller to supply an HTTP client to
// use when talking to the Okta API endpoint, for use in testing. The installer
// will default to using the standard Okta client settings if no HTTP client is
// provided.
func installOktaPlugin(ctx context.Context, args installOktaPluginArgs) (*ui.Plugin, error) {
	log := args.log.WithField(teleport.ComponentKey, teleport.Component(types.PluginTypeOkta))

	params, err := validateOktaPluginInputs(ctx, args.validateOktaPluginInputsArgs)
	if err != nil {
		return nil, trace.Wrap(err, "validating okta parameters")
	}
	log.Debug("Proceeding with installation.")

	log = log.WithField("oktaOrg", params.oktaOrgURL)
	log.Infof("Creating/Validating SAML connector for %s...", params.oktaOrgURL)

	connInfo, err := getOrCreateSAMLConnector(ctx, args.sessCtx, params.oktaClient, oktaSSOConnectorName, args.signingKeypair, log)
	if err != nil {
		log.WithError(err).Error("Failed to ensure SAML connector exists.")
		return nil, trace.Wrap(err)
	}

	// Set up the Okta API token that Teleport will use to authenticate with the
	// targeted Okta organization
	pluginCredentials := []*types.PluginStaticCredentialsV1{
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
					APIToken: params.oktaAPIToken,
				},
			},
		},
	}

	// Only create the SCIM token credential if IGS is enabled
	// todo (michellescripts) replace this with teleport.OktaSCIM
	if modules.GetProtoEntitlement(args.clusterFeatures, entitlements.Identity).Enabled {
		pluginCredentials = append(pluginCredentials, &types.PluginStaticCredentialsV1{
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
					APIToken: params.scimBearerTokenHash,
				},
			},
		})
	}

	// Finally, install the plugin into teleport
	log.Trace("Collating plugin resource")
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
						OrgUrl: params.oktaOrgURL,
						SyncSettings: &types.PluginOktaSyncSettings{
							SsoConnectorId:  oktaSSOConnectorName,
							AppId:           connInfo.OktaAppID,
							SyncUsers:       true,
							GroupFilters:    params.groupFilters,
							AppFilters:      params.appFilters,
							DefaultOwners:   params.defaultOwners,
							SyncAccessLists: params.enableAccessListSync,
						},
					},
				},
			},
		},
		StaticCredentialsList: pluginCredentials,
		CredentialLabels: map[string]string{
			eteleport.OktaOrgURLLabel: params.oktaOrgURL,
		},
	}

	log.Trace("Installing plugin")

	uiPlugin, err := installPlugin(ctx, args.sessCtx, req, args.plugin)
	if err != nil {
		log.WithError(err).Error("Failed plugin install")
		return nil, trace.Wrap(err)
	}

	log.Trace("Generating UI spec")

	uiPlugin.Spec = &ui.OktaPluginSpec{
		SCIMBearerToken:      params.scimBearerToken,
		OktaAppID:            connInfo.OktaAppID,
		OktaAppName:          connInfo.OktaAppName,
		OktaAppLabel:         connInfo.OktaAppLabel,
		TeleportSSOConnector: connInfo.Connector.GetName(),
	}

	return uiPlugin, nil
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
	oktaClient           okta.OktaClient
}

type validateOktaPluginInputsArgs struct {
	form            url.Values
	httpClient      *http.Client
	clusterFeatures *proto.Features
	log             *logrus.Entry

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
	if args.log == nil {
		args.log = logrus.NewEntry(logrus.StandardLogger())
	}
	args.bcryptCost = min(args.bcryptCost, bcrypt.MaxCost)
	if args.bcryptCost < bcrypt.MinCost {
		args.bcryptCost = bcrypt.DefaultCost
	}
	return nil
}

// validateOktaConfig extracts the Okta configuration inputs to the Okta plugin
// installer from the supplied form and validates them with a live request to
// the Okta organization.
func (args *validateOktaPluginInputsArgs) validateOktaConfig(ctx context.Context) (oktaPluginInputs, error) {
	if err := args.CheckAndSetDefaults(); err != nil {
		return oktaPluginInputs{}, trace.Wrap(err)
	}

	args.log.Trace("Extracting Okta client config")

	oktaOrgURLText := args.form.Get("orgURL")
	if oktaOrgURLText == "" {
		return oktaPluginInputs{}, trace.BadParameter("missing Okta organization URL")
	}

	oktaAPIToken := args.form.Get("apiToken")
	if oktaAPIToken == "" {
		return oktaPluginInputs{}, trace.BadParameter("missing Okta API token")
	}

	// If the user supplies an invalid URL or bare hostname, then the
	// integration will appear to install but fail to start with obscure
	// errors only visible in the Teleport log file.
	//
	// To avoid this, we helpfully supply a sensible-default `https`
	// scheme if necessary
	orgURL, err := lib.AddrToURL(oktaOrgURLText)
	if err != nil {
		return oktaPluginInputs{}, trace.BadParameter("malformed Okta url")
	}

	log := args.log.WithFields(logrus.Fields{
		teleport.ComponentKey: teleport.Component(types.PluginTypeOkta),
		"oktaOrg":             orgURL.String(),
	})

	oktaClient, err := okta.NewClient(ctx, okta.ClientConfig{
		HTTPClient: args.httpClient,
		Endpoint:   orgURL.String(),
		Token:      oktaAPIToken,
		Log:        log,
		StatusSink: nil,
	})
	if err != nil {
		return oktaPluginInputs{}, trace.Wrap(err, "constructing Okta client")
	}

	log.Debug("Validating Okta configuration...")
	if err := okta.TestCredentials(ctx, oktaClient); err != nil {
		return oktaPluginInputs{}, trace.BadParameter("bad Okta configuration: %s", err.Error())
	}
	log.Debug("Okta configuration looks good.")

	return oktaPluginInputs{
		oktaOrgURL:   orgURL.String(),
		oktaAPIToken: oktaAPIToken,
		oktaClient:   oktaClient,
	}, nil
}

func validateOktaPluginInputs(ctx context.Context, args validateOktaPluginInputsArgs) (oktaPluginInputs, error) {
	if err := args.CheckAndSetDefaults(); err != nil {
		return oktaPluginInputs{}, trace.Wrap(err)
	}

	params, err := args.validateOktaConfig(ctx)
	if err != nil {
		return oktaPluginInputs{}, trace.Wrap(err, "invalid Okta config")
	}

	// todo (michellescripts) replace with teleport.OktaScim
	if modules.GetProtoEntitlement(args.clusterFeatures, entitlements.Identity).Enabled {
		params.enableAccessListSync = true

		params.scimBearerToken = args.form.Get("scimToken")
		if params.scimBearerToken == "" {
			return oktaPluginInputs{}, trace.BadParameter("missing SCIM bearer token")
		}

		scimTokenHash, err := bcrypt.GenerateFromPassword([]byte(params.scimBearerToken), args.bcryptCost)
		if err != nil {
			return oktaPluginInputs{}, trace.BadParameter("hashing SCIM bearer token")
		}
		params.scimBearerTokenHash = string(scimTokenHash)

		var defaultOwners []string
		defaultOwnersString := args.form.Get("defaultOwners")
		if defaultOwnersString != "" {
			defaultOwners = []string{}
			if err := json.Unmarshal([]byte(defaultOwnersString), &defaultOwners); err != nil {
				return oktaPluginInputs{}, trace.Wrap(err)
			}
		}
		params.defaultOwners = defaultOwners
		if defaultOwners == nil {
			params.enableAccessListSync = false
			return params, nil
		}

		params.appFilters, err = getOktaAppFilters(args.form)
		if err != nil {
			return oktaPluginInputs{}, trace.Wrap(err)
		}

		params.groupFilters, err = getOktaGroupFilters(args.form)
		if err != nil {
			return oktaPluginInputs{}, trace.Wrap(err)
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

func getOrCreateSAMLConnector(ctx context.Context, sessCtx *web.SessionContext, oktaClient okta.OktaClient, samlConnectorName string, signingKeypair *types.AsymmetricKeyPair, log *logrus.Entry) (*okta.SAMLConnectorInfo, error) {
	log.Debug("Fetching cluster information")
	client, err := sessCtx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Remove the MFA resp from the context before pinging.
	// Otherwise, it will be consumed before the Create which actually
	// requires the MFA.
	// TODO(Joerger): Explicitly provide MFA response only where it is
	// needed instead of removing it like this.
	pingCtx := mfa.ContextWithMFAResponse(ctx, nil)
	pingInfo, err := client.Ping(pingCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	publicURL, err := lib.AddrToURL(pingInfo.ProxyPublicAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	log.Debugf("Retrieving SAML connector %q", samlConnectorName)

	// Remove the MFA resp from the context before getting the connector.
	// Otherwise, it will be consumed before the Create which actually
	// requires the MFA.
	// TODO(Joerger): Explicitly provide MFA response only where it is
	// needed instead of removing it like this.
	getConnectorCtx := mfa.ContextWithMFAResponse(ctx, nil)
	samlConnector, err := client.GetSAMLConnector(getConnectorCtx, samlConnectorName, false)
	if trace.IsNotFound(err) {
		log.Infof("SAML connector %s not found. Creating...", samlConnectorName)
		connInfo, err := okta.CreateSAMLConnector(ctx, okta.ConnectorArgs{
			ConnectorName:        samlConnectorName,
			OktaClient:           oktaClient,
			SAMLConnectorService: client,
			ClusterName:          pingInfo.ClusterName,
			PublicURL:            publicURL,
			SigningKeypair:       signingKeypair,
			Log:                  log,
		})
		return connInfo, trace.Wrap(err, "creating new SAML connector")
	} else if err != nil {
		return nil, trace.Wrap(err, "fetching SAML connector %s", samlConnectorName)
	}

	connectorInfo, err := okta.ValidateSAMLConnector(ctx, samlConnector, oktaClient)
	if err != nil {
		// Using the CompareFailed error here results in the HTTP request
		// returning http.StatusPreconditionFailed, which we can use as a signal
		// to the UI that the problem is the underlying SAML connector.
		return nil, trace.CompareFailed(err.Error())
	}

	return connectorInfo, nil
}
