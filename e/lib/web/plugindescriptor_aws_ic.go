package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	crewjamsamlsp "github.com/crewjam/saml/samlsp"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/common"
	"github.com/gravitational/teleport/api/types/samlsp"
	icsdk "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
	"github.com/gravitational/teleport/e/lib/web/ui"
	awsicui "github.com/gravitational/teleport/e/lib/web/ui/awsic"
	samlidpui "github.com/gravitational/teleport/e/lib/web/ui/samlidp"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/integrations/awsoidc/credprovider"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/web"
)

// awsICPluginDescriptor implements the AWS Identity Center specific
// version of the pluginDescriptor interface
type awsICPluginDescriptor struct {
	// HTTPClient is used for testing
	HTTPClient *http.Client
}

// HandleInstallRequest implements pluginDescriptor.
// Creates SAML service provider first and then creates the plugin.
func (a awsICPluginDescriptor) HandleInstallRequest(ctx context.Context, sessCtx *web.SessionContext, w http.ResponseWriter, r *http.Request, p *Plugin) (*ui.Plugin, error) {
	authClient, err := sessCtx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	inputs, err := a.awsICPluginInputs(ctx, r.Form, authClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	samlSP, err := samlidpui.TransformToProtoType(samlidpui.CreateSAMLIdPServiceProviderRequest{
		Name:             inputs.samlServiceProviderName,
		EntityDescriptor: inputs.samlServiceProviderMetadata,
		Labels: map[string]string{
			types.OriginLabel: common.OriginAWSIdentityCenter,
		},
		Preset: samlsp.AWSIdentityCenter,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = authClient.CreateSAMLIdPServiceProvider(ctx, samlSP)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	req := &pluginspb.CreatePluginRequest{
		Plugin: &types.PluginV1{
			Metadata: types.Metadata{
				Name: types.PluginTypeAWSIdentityCenter,
				Labels: map[string]string{
					types.HostedPluginLabel: "true",
				},
			},
			Spec: types.PluginSpecV1{
				Settings: &types.PluginSpecV1_AwsIc{
					AwsIc: &types.PluginAWSICSettings{
						IntegrationName:         inputs.oidcIntegrationName,
						Region:                  inputs.region,
						Arn:                     inputs.arn,
						AccessListDefaultOwners: inputs.accessListDefaultOwners,
						ProvisioningSpec: &types.AWSICProvisioningSpec{
							BaseUrl: inputs.scimBaseURL,
						},
						SamlIdpServiceProviderName: samlSP.GetName(),
					},
				},
			},
		},
		StaticCredentials: &types.PluginStaticCredentialsV1{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{
					Labels: map[string]string{
						"aws-ic/scim-api-endpoint": inputs.scimBaseURL,
					},
					Name: types.PluginTypeAWSIdentityCenter,
				},
			},
			Spec: &types.PluginStaticCredentialsSpecV1{
				Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
					APIToken: inputs.scimAccessToken,
				},
			},
		},
	}

	_, err = authClient.PluginsClient().CreatePlugin(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	uiPlugin, err := ui.NewPlugin(req.GetPlugin())
	return uiPlugin, trace.Wrap(err)
}

const (
	pluginConfigAWSICValidateSAML = "validateSAML"
	pluginConfigAWSICValidateSCIM = "validateSCIM"
)

// HandleValidateConfigRequest handles requests for "/enterprise/plugins/validate" path.
func (a awsICPluginDescriptor) HandleValidateConfigRequest(ctx context.Context, sessCtx *web.SessionContext, form url.Values, p *Plugin) error {
	client, err := sessCtx.GetClient()
	if err != nil {
		return trace.Wrap(err)
	}

	resourceToValidate := form.Get("resourceToValidate")
	if resourceToValidate == "" {
		return trace.BadParameter("name of the resource to validate cannot be empty")
	}
	switch resourceToValidate {
	case pluginConfigAWSICValidateSAML:
		return a.validateSAMLServiceProvider(ctx, client, form.Get(awsICPluginSAMLServiceProviderNameField), form.Get(awsICPluginSAMLServiceProviderMetadataField))
	case pluginConfigAWSICValidateSCIM:
		return a.validateSCIM(ctx, form.Get(awsICPluginSCIMBaseURLField), form.Get(awsICPluginSCIMAccessTokenField))
	default:
		return trace.NotImplemented("validation for %q is not implemented for AWS IC plugin", resourceToValidate)
	}
}

// TranslateCallbackCookie implements pluginDescriptor.
func (awsICPluginDescriptor) TranslateCallbackCookie(*types.PluginSpecV1, *pluginOnboardingCookie) error {
	// This always returns not implemented, since AWS IC plugin onboarding does not use the OAuth2 web flow.
	return trace.NotImplemented("TranslateCallbackCookie is not implemented for AWS IC")
}

type awsICPluginFormData struct {
	name                        string
	region                      string
	arn                         string
	oidcIntegrationName         string
	accessListDefaultOwners     []string
	samlServiceProviderName     string
	samlServiceProviderMetadata string
	scimBaseURL                 string
	scimAccessToken             string
}

const (
	awsICPluginNameField                        = "name"
	awsICPluginOIDCIntegrationNameField         = "oidcIntegrationName"
	awsICPluginAccessListDefaultOwnersField     = "accessListDefaultOwners"
	awsICPluginICRegionField                    = "region"
	awsICPluginICARNField                       = "arn"
	awsICPluginSAMLServiceProviderNameField     = "samlServiceProviderName"
	awsICPluginSAMLServiceProviderMetadataField = "samlServiceProviderMetadata"
	awsICPluginSCIMBaseURLField                 = "scimBaseURL"
	awsICPluginSCIMAccessTokenField             = "scimAccessToken"
)

func (a awsICPluginDescriptor) awsICPluginInputs(ctx context.Context, form url.Values, authClient authclient.ClientI) (*awsICPluginFormData, error) {
	var accessListDefaultOwners []string
	if err := json.Unmarshal([]byte(form.Get(awsICPluginAccessListDefaultOwnersField)), &accessListDefaultOwners); err != nil {
		return nil, trace.Wrap(err, "cannot unmarshal accessListDefaultOwners")
	}

	parsedInputs := &awsICPluginFormData{
		name:                        form.Get(awsICPluginNameField),
		oidcIntegrationName:         form.Get(awsICPluginOIDCIntegrationNameField),
		accessListDefaultOwners:     accessListDefaultOwners,
		region:                      form.Get(awsICPluginICRegionField),
		arn:                         form.Get(awsICPluginICARNField),
		samlServiceProviderName:     form.Get(awsICPluginSAMLServiceProviderNameField),
		samlServiceProviderMetadata: form.Get(awsICPluginSAMLServiceProviderMetadataField),
		scimBaseURL:                 form.Get(awsICPluginSCIMBaseURLField),
		scimAccessToken:             form.Get(awsICPluginSCIMAccessTokenField),
	}

	if err := parsedInputs.check(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := a.validateOIDCIntegrationExists(ctx, authClient, parsedInputs.oidcIntegrationName); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := a.validateSAMLServiceProvider(ctx, authClient, parsedInputs.samlServiceProviderName, parsedInputs.samlServiceProviderMetadata); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := a.validateSCIM(ctx, parsedInputs.scimBaseURL, parsedInputs.scimAccessToken); err != nil {
		return nil, trace.Wrap(err)
	}

	return parsedInputs, nil
}

func (e *awsICPluginFormData) check() error {
	if e.name == "" {
		return trace.BadParameter("plugin name is required")
	}

	if e.oidcIntegrationName == "" {
		return trace.BadParameter("integration name is required")
	}

	if len(e.accessListDefaultOwners) == 0 {
		return trace.BadParameter("access list default owners is required")
	}

	if e.region == "" {
		return trace.BadParameter("Identity Center region is required")
	}

	if e.arn == "" {
		return trace.BadParameter("Identity Center instance ARN is required")
	}

	return nil
}

// validateSCIM validates SCIM credential by querying ServiceProviderConfig endpoint.
// https://docs.aws.amazon.com/singlesignon/latest/developerguide/serviceproviderconfig.html
// should always return a hardcoded error to prevent
// users from abusing this validation service for SSRF.
func (a awsICPluginDescriptor) validateSCIM(ctx context.Context, baseURL, accessToken string) error {
	if baseURL == "" {
		return trace.BadParameter("AWS Identity Center SCIM base URL is required")
	}
	if accessToken == "" {
		return trace.BadParameter("AWS Identity Center SCIM access token is required")
	}

	url, err := url.Parse(baseURL)
	if err != nil {
		return trace.Wrap(err)
	}
	if url.Scheme != "https" {
		return trace.BadParameter("unsupported url scheme %q for base URL, must be https", url.Scheme)
	}

	httpClient, err := a.getHTTPClient()
	if err != nil {
		return trace.Wrap(err)
	}

	scimClient, err := scimsdk.New(&scimsdk.Config{
		HTTPClient: httpClient,
		Endpoint:   baseURL,
		Token:      accessToken,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if err := scimClient.Ping(ctx); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// validateSAMLServiceProvider is used to pre-validate SAML service provider metadata file
// configured for AWS Identity Center. It specifically performs validations listed below:
// - the SAML service provider name is unique in Teleport cluster.
// - the SAML service provider entity ID is unique in Teleport cluster.
// - the metadata XML file is a valid SAML IdP service provider entity descriptor XML format.
// - the metadata XML does not contain unsupported ACS binding values.
func (a awsICPluginDescriptor) validateSAMLServiceProvider(ctx context.Context, client authclient.ClientI, name, metadata string) error {
	if name == "" {
		return trace.BadParameter("AWS Identity Center SAML service provider service provider name is required")
	}
	if metadata == "" {
		return trace.BadParameter("AWS Identity Center SAML service provider service provider metadata is required")
	}
	sp, err := client.GetSAMLIdPServiceProvider(ctx, name)
	if err == nil && sp != nil {
		return trace.AlreadyExists("SAML IdP service provider with name %q already exists", sp.GetName())
	}

	// TransformToProtoType performs basic spec validation
	_, err = samlidpui.TransformToProtoType(samlidpui.CreateSAMLIdPServiceProviderRequest{
		Name:             name,
		EntityDescriptor: metadata,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	ed, err := crewjamsamlsp.ParseMetadata([]byte(metadata))
	if err != nil {
		return trace.BadParameter("invalid metadata for AWS Identity Center SAML service provider: %v", err)
	}

	if err := services.FilterSAMLEntityDescriptor(ed, false /* quiet */); err != nil {
		return trace.BadParameter("metadata for AWS Identity Center SAML service provider contains unsupported ACS bindings: %v", err)
	}

	if err := a.ensureEntityIDIsUnique(ctx, client, name, ed.EntityID); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// ensureEntityIDIsUnique loops through existing SAML service providers to find duplicate enity ID.
// TODO(sshah): expose this method in the RPC so it function can be reused.
func (a awsICPluginDescriptor) ensureEntityIDIsUnique(ctx context.Context, authClient authclient.ClientI, name, entityID string) error {
	var nextToken string
	for {
		var sps []types.SAMLIdPServiceProvider
		var err error
		sps, nextToken, err = authClient.ListSAMLIdPServiceProviders(ctx, 100, nextToken)
		if err != nil {
			return trace.Wrap(err)
		}

		for _, sp := range sps {
			if sp.GetName() != name && sp.GetEntityID() == entityID {
				return trace.AlreadyExists("%s %q has the same entity ID %q", types.KindSAMLIdPServiceProvider, sp.GetName(), sp.GetEntityID())
			}
		}
		if nextToken == "" {
			break
		}
	}

	return nil
}

func (a awsICPluginDescriptor) validateOIDCIntegrationExists(ctx context.Context, client authclient.ClientI, integrationName string) error {
	if integrationName == "" {
		return trace.BadParameter("OIDC integration name is required")
	}

	// TODO(sshah): validate audience value once https://github.com/gravitational/teleport/pull/47725
	// is available
	_, err := client.GetIntegration(ctx, integrationName)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (a awsICPluginDescriptor) getHTTPClient() (*http.Client, error) {
	client := a.HTTPClient
	if client == nil {
		client, err := defaults.HTTPClient()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}
	return client, nil
}

// awsICPluginIdentityCenterClient creates a new Identity Center SDK client.
func awsICPluginIdentityCenterClient(ctx context.Context, req awsicui.FetchICResourceRequest, authClient authclient.ClientI) (icsdk.Client, error) {
	awsConfig, err := credprovider.CreateAWSConfigForIntegration(ctx, credprovider.Config{
		Region:                req.Region,
		IntegrationName:       req.IntegrationName,
		IntegrationGetter:     authClient,
		AWSOIDCTokenGenerator: authClient,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return icsdk.New(icsdk.Config{
		AWSConfig:   awsConfig,
		InstanceARN: req.Arn,
	})
}

// awsICPluginListPermissionSets lists Identity Center permissions sets.
func (p *Plugin) awsICPluginListPermissionSets(w http.ResponseWriter, r *http.Request, params httprouter.Params, sessCtx *web.SessionContext) (interface{}, error) {
	var req awsicui.FetchICResourceRequest
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, trace.Wrap(err)
	}
	icClient, err := awsICPluginIdentityCenterClient(r.Context(), req, p.GetProxyClient())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	permSet, err := icClient.ListPermissionSets(r.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return awsicui.PermissionSets(permSet), nil
}

// awsICPluginAccountsWithAssignedPermSets lists Identity Center accounts with assigned permission sets.
func (p *Plugin) awsICPluginAccountsWithAssignedPermSets(w http.ResponseWriter, r *http.Request, params httprouter.Params, sessCtx *web.SessionContext) (interface{}, error) {
	var req awsicui.FetchICResourceRequest
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, trace.Wrap(err)
	}
	icClient, err := awsICPluginIdentityCenterClient(r.Context(), req, p.GetProxyClient())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accountWithPermSetARNs, err := icClient.ListAccountsWithAssignedPermissionSetARNs(r.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	psermSets, err := icClient.ListPermissionSets(r.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return awsicui.AccountWithPermissionSets(accountWithPermSetARNs, icsdk.ToPermissionSetMap(psermSets)), nil
}

// awsICPluginGroupsWithAssignment lists Identity Center groups with assigned accounts and permission sets.
func (p *Plugin) awsICPluginGroupsWithAccountAndPermAssignment(w http.ResponseWriter, r *http.Request, params httprouter.Params, sessCtx *web.SessionContext) (interface{}, error) {
	var req awsicui.FetchICResourceRequest
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, trace.Wrap(err)
	}
	icClient, err := awsICPluginIdentityCenterClient(r.Context(), req, p.GetProxyClient())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	groupWithAssignments, err := icClient.ListGroupsWithAccountAndPermAssignment(r.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	psermSets, err := icClient.ListPermissionSets(r.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	accounts, err := icClient.ListAccounts(r.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return awsicui.GroupAccountAndPermAssignments(groupWithAssignments, icsdk.ToAccountMap(accounts), icsdk.ToPermissionSetMap(psermSets)), nil
}
