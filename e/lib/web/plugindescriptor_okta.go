package web

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/protobuf/proto"

	clientproto "github.com/gravitational/teleport/api/client/proto"
	oktav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/teleport/lib/web"
)

// oktaPluginDescriptor is an empty type used to implement an Okta-specific
// version of the pluginDescriptor interface
type oktaPluginDescriptor struct{}

// Static assertion that oktaPluginDescriptor implements the pluginDescriptor
// interface
var _ pluginDescriptor = oktaPluginDescriptor{}
var _ pluginUpdateHandler = oktaPluginDescriptor{}

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

// HandleUpdateRequest updates the Okta plugin
func (oktaPluginDescriptor) HandleUpdateRequest(ctx context.Context, sessCtx *web.SessionContext, req *ui.PluginUpdateRequest) (*ui.Plugin, error) {
	return updateOktaPlugin(ctx, sessCtx, req)
}

func updateOktaPlugin(ctx context.Context, sessCtx *web.SessionContext, req *ui.PluginUpdateRequest) (*ui.Plugin, error) {
	if req.Okta == nil {
		return nil, trace.BadParameter("missing Okta plugin configuration")
	}

	updateReq, err := validateOktaPluginUpdateInputs(req.Okta)
	if err != nil {
		return nil, trace.Wrap(err, "validating Okta plugin update parameters")
	}

	authOktaClient := oktav1.NewOktaServiceClient(sessCtx.GetClientConnection())
	resp, err := authOktaClient.UpdateIntegration(ctx, updateReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.NewPlugin(resp.GetPlugin())
}

// validateOktaPluginUpdateInputs validates the Okta plugin update request
func validateOktaPluginUpdateInputs(params *ui.OktaPluginUpdate) (*oktav1.UpdateIntegrationRequest, error) {
	if params == nil {
		return nil, trace.BadParameter("missing okta update parameters")
	}

	// Only expected to be set if updating UserSync settings, otherwise, will be nil, and we'll use saved credentials
	var oktaAPICreds *oktav1.OktaAPICredentials
	if params.ClientID != "" {
		oktaAPICreds = oktav1.OktaAPICredentials_builder{
			OauthId: proto.String(params.ClientID),
		}.Build()
	}

	var accessListSettings *oktav1.AccessListSettings
	if len(params.DefaultOwners) == 0 && params.EnableAccessListSync {
		return nil, trace.BadParameter("Default Owners are required for Access List Sync")
	}

	if params.EnableAccessListSync {
		accessListSettings = oktav1.AccessListSettings_builder{
			DefaultOwner: params.DefaultOwners,
			AppFilters:   params.AppFilters,
			GroupFilters: params.GroupFilters,
		}.Build()
	}

	return oktav1.UpdateIntegrationRequest_builder{
		ApiCredentials:            oktaAPICreds,
		ScimToken:                 params.SCIMToken,
		EnableUserSync:            params.EnableUserSync,
		DisableAssignDefaultRoles: !params.AssignDefaultRoles,
		EnableAppGroupSync:        params.EnableAppGroupSync,
		EnableAccessListSync:      params.EnableAccessListSync,
		AccessListSettings:        accessListSettings,
		EnableBidirectionalSync:   params.EnableBidirectionalSync,
		EnableSystemLogExport:     params.EnableSystemLogExport,
	}.Build(), nil
}

// TranslateCallbackCookie implements PluginDescriptor for oktaPluginDescriptor,
// always returning "Not Implemented".
func (oktaPluginDescriptor) TranslateCallbackCookie(*types.PluginSpecV1, *pluginOnboardingCookie) error {
	return trace.NotImplemented("TranslateCallbackCookie")
}

// HandleOAuthStart implements PluginDescriptor for oktaPluginDescriptor, always
// returning "Not Implemented".
func (oktaPluginDescriptor) HandleOAuthStart(ctx context.Context, sessCtx *web.SessionContext, w http.ResponseWriter, r *http.Request, p *Plugin) (*ui.OAuthPluginStartResponse, error) {
	return nil, trace.NotImplemented("HandleOAuthStart")
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
	var params *oktaPluginInputs
	var err error

	params, err = validateOktaPluginInputs(ctx, args.validateOktaPluginInputsArgs, args.sessCtx)
	if err != nil {
		return nil, trace.Wrap(err, "validating okta parameters")
	}

	oktaAPICreds := getOktaCredsFromParams(params)
	authOktaClient := oktav1.NewOktaServiceClient(args.sessCtx.GetClientConnection())
	resp, err := authOktaClient.CreateIntegration(ctx, oktav1.CreateIntegrationRequest_builder{
		ReuseConnector:            params.reuseConnector,
		SsoMetadataUrl:            params.metadataURL,
		OktaOrganizationUrl:       params.oktaOrgURL,
		ApiCredentials:            oktaAPICreds,
		ScimToken:                 params.scimBearerToken,
		EnableUserSync:            params.enableUserSync,
		DisableAssignDefaultRoles: false,
		EnableAppGroupSync:        params.enableAppGroupsSync,
		EnableAccessListSync:      params.enableAccessListSync,
		AccessListSettings: oktav1.AccessListSettings_builder{
			GroupFilters: params.groupFilters,
			AppFilters:   params.appFilters,
			DefaultOwner: params.defaultOwners,
		}.Build(),
		EnableBidirectionalSync: params.enableBidirectionalSync,
		EnableSystemLogExport:   params.enableSystemLogExport,
	}.Build())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	uiPlugin, err := ui.NewPlugin(resp.GetPlugin())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	spec, ok := uiPlugin.Spec.(*ui.OktaPluginSpec)
	if !ok {
		return nil, trace.BadParameter("unexpected plugin type")
	}
	if resp.GetConnectorInfo() == nil {
		return nil, trace.BadParameter("missing connector info")
	}
	spec.SCIMBearerToken = params.scimBearerToken
	spec.OktaAppID = resp.GetConnectorInfo().GetOktaAppId()
	spec.OktaAppName = resp.GetConnectorInfo().GetOktaAppName()
	spec.OktaAppLabel = resp.GetConnectorInfo().GetOktaAppLabels()
	spec.TeleportSSOConnector = resp.GetConnectorInfo().GetTeleportConnectorName()
	uiPlugin.Spec = spec
	return uiPlugin, nil
}

func getOktaCredsFromParams(params *oktaPluginInputs) *oktav1.OktaAPICredentials {
	var apiCreds *oktav1.OktaAPICredentials
	switch {
	case params.oauthClientID != "":
		apiCreds = oktav1.OktaAPICredentials_builder{
			OauthId: proto.String(params.oauthClientID),
		}.Build()
	case params.oktaAPIToken != "":
		apiCreds = oktav1.OktaAPICredentials_builder{
			SswsBearerToken: proto.String(params.oktaAPIToken),
		}.Build()
	}
	// If neither API token nor OAuth client ID is set in req, we return nil
	// and let the Okta client check for saved credentials.
	return apiCreds
}

type oktaPluginInputs struct {
	oktaOrgURL              string
	oktaAPIToken            string
	scimBearerToken         string
	scimBearerTokenHash     string
	groupFilters            []string
	appFilters              []string
	defaultOwners           []string
	enableAccessListSync    bool
	oauthClientID           string
	enableUserSync          bool
	enableAppGroupsSync     bool
	enableBidirectionalSync bool
	metadataURL             string
	reuseConnector          string
	// enableSystemLogExport is used to determine if the audit logs export
	// feature is enabled for the Okta plugin.
	enableSystemLogExport bool
}

type validateOktaPluginInputsArgs struct {
	form            url.Values
	httpClient      *http.Client
	clusterFeatures *clientproto.Features
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

func (args *validateOktaPluginInputsArgs) setURLs(inputs *oktaPluginInputs) error {
	oktaOrgURLText := args.form.Get("orgURL")
	metadataURLText := args.form.Get("metadataURL")
	if oktaOrgURLText == "" {
		if metadataURLText == "" {
			// If APIToken is also omitted, we should just return empty
			// as we may be updating an existing plugin.
			if args.form.Get("apiToken") == "" {
				return nil
			}
			return trace.BadParameter("missing Okta organization URL")
		}
	}

	// If the user supplies an invalid URL or bare hostname, then the
	// integration will appear to install but fail to start with obscure
	// errors only visible in the Teleport log file.
	//
	// To avoid this, we helpfully supply a sensible-default `https`
	// scheme if necessary
	if oktaOrgURLText != "" {
		orgURL, err := lib.AddrToURL(oktaOrgURLText)
		if err != nil {
			return trace.BadParameter("malformed Okta url")
		}
		inputs.oktaOrgURL = orgURL.String()
	}
	if metadataURLText != "" {
		metadataURL, err := lib.AddrToURL(metadataURLText)
		if err != nil {
			return trace.BadParameter("malformed metadata url")
		}
		inputs.metadataURL = metadataURL.String()
	}

	return nil
}

// validateOktaConfig extracts the Okta configuration inputs to the Okta plugin
// installer from the supplied form and validates them with a live request to
// the Okta organization.
func (args *validateOktaPluginInputsArgs) validateOktaConfig(ctx context.Context, sessCtx *web.SessionContext) (*oktaPluginInputs, error) {
	if err := args.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	args.logger.Log(ctx, logutils.TraceLevel, "Validating Okta client config")

	out := &oktaPluginInputs{}

	if err := args.setURLs(out); err != nil {
		return nil, trace.Wrap(err)
	}

	out.oktaAPIToken = args.form.Get("apiToken")
	out.oauthClientID = args.form.Get("clientID")
	out.scimBearerToken = args.form.Get("scimToken")
	out.enableAccessListSync = utils.AsBool(args.form.Get("enableAccessListSync"))
	// For compat, if unspecified, enable user sync + app/group sync
	out.enableUserSync = orDefault(args.form, "enableUserSync", true)
	out.enableAppGroupsSync = orDefault(args.form, "enableAppGroupsSync", out.enableAccessListSync)
	out.reuseConnector = args.form.Get("reuseConnector")
	// If access list sync is enabled, default to true
	out.enableBidirectionalSync = orDefault(args.form, "enableBidirectionalSync", out.enableAccessListSync)
	out.enableSystemLogExport = orDefault(args.form, "enableSystemLogExport", out.enableSystemLogExport)

	// We only want to validate the config via live req to the Okta org
	// if providing credentials or toggling sync options.
	shouldValidateAuth := out.oktaAPIToken != "" || out.oauthClientID != "" || out.enableAccessListSync || out.enableUserSync || out.enableAppGroupsSync

	req := oktav1.ValidateClientCredentialsRequest_builder{
		OktaOrganizationUrl: out.oktaOrgURL,
	}.Build()

	// Missing credentials are not an immediate error, as
	// we could be updating a plugin with existing saved credentials.
	if out.oauthClientID != "" {
		req.SetApiCredentials(oktav1.OktaAPICredentials_builder{
			OauthId: proto.String(out.oauthClientID),
		}.Build())
	} else if out.oktaAPIToken != "" {
		req.SetApiCredentials(oktav1.OktaAPICredentials_builder{
			SswsBearerToken: proto.String(out.oktaAPIToken),
		}.Build())
	}

	if shouldValidateAuth {
		authOktaClient := oktav1.NewOktaServiceClient(sessCtx.GetClientConnection())
		if _, err := authOktaClient.ValidateClientCredentials(ctx, req); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return out, nil
}

// orDefault returns the value of the key in the form, or true if the
// key is not set.
// This is needed for backward compatibility with the previous version of the
// Okta plugin where all feature by default where enabled.
func orDefault(form url.Values, key string, defaultValue bool) bool {
	v := form.Get(key)
	if v == "" {
		return defaultValue
	}
	return utils.AsBool(v)
}

// validateOktaPluginInputs extracts the Okta configuration inputs to the Okta
// plugin installer from the supplied form and validates them.
// TODO(kiosion): Remove legacy handling for APIToken in v19.
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

	args.logger.Log(ctx, logutils.TraceLevel, "Extracting Okta client config")

	out, err := args.validateOktaConfig(ctx, sessCtx)
	if err != nil {
		return nil, trace.Wrap(err, "invalid Okta config")
	}

	isLegacySetup := out.oktaAPIToken != ""
	hasOktaSCIM := modules.GetProtoEntitlement(args.clusterFeatures, entitlements.OktaSCIM).Enabled

	if isLegacySetup {
		if hasOktaSCIM {
			// If using legacy setup, Access List sync should always be enabled
			out.enableAccessListSync = true
			out.enableAppGroupsSync = true
			if out.scimBearerToken == "" {
				return nil, trace.BadParameter("missing SCIM bearer token")
			}
		} else {
			out.enableAccessListSync = false
			out.enableAppGroupsSync = false
			out.scimBearerToken = ""
			return out, nil
		}
	}

	if out.scimBearerToken != "" {
		if len(out.scimBearerToken) < minTokenLength {
			return nil, trace.BadParameter("SCIM bearer token must be at least %d characters", minTokenLength)
		}
		if len(out.scimBearerToken) > maxTokenLength {
			return nil, trace.BadParameter("SCIM bearer token must be no longer than %d characters", maxTokenLength)
		}
		scimTokenHash, err := bcrypt.GenerateFromPassword([]byte(out.scimBearerToken), args.bcryptCost)
		if err != nil {
			return nil, trace.BadParameter("hashing SCIM bearer token")
		}
		out.scimBearerTokenHash = string(scimTokenHash)
	}

	var defaultOwners []string
	defaultOwnersString := args.form.Get("defaultOwners")
	if defaultOwnersString != "" {
		defaultOwners = []string{}
		if err := json.Unmarshal([]byte(defaultOwnersString), &defaultOwners); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	out.defaultOwners = defaultOwners
	if defaultOwners == nil {
		out.enableAccessListSync = false
		out.enableAppGroupsSync = false
		return out, nil
	}

	out.appFilters, err = getOktaFilters(args.form.Get("appFilters"))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out.groupFilters, err = getOktaFilters(args.form.Get("groupFilters"))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return out, nil
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
