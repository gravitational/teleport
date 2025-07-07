package oktaservice

import (
	"context"
	"net/http"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/bcrypt"

	oktapb "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
	"github.com/gravitational/teleport/api/types"
	oktaapi "github.com/gravitational/teleport/e/lib/okta/api"
	"github.com/gravitational/teleport/e/lib/okta/common"
	oktaplugin "github.com/gravitational/teleport/e/lib/okta/plugin"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
)

func newOktaPluginCredentials(req *oktapb.CreateIntegrationRequest) ([]*types.PluginStaticCredentialsV1, error) {
	var out []*types.PluginStaticCredentialsV1
	if req.GetApiCredentials().GetOauthId() != "" {
		out = append(out, buildOAuthCredentials(req.GetApiCredentials().GetOauthId()))
	}
	if token := req.GetApiCredentials().GetSswsBearerToken(); token != "" {
		out = append(out, buildAPITokenCredentials(token))
	}
	if modules.GetModules().Features().GetEntitlement(entitlements.OktaSCIM).Enabled {
		if req.GetScimToken() != "" {
			scimTokenHash, err := bcrypt.GenerateFromPassword([]byte(req.GetScimToken()), bcrypt.DefaultCost)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			out = append(out, buildSCIMCredentials(string(scimTokenHash)))
		}
	}
	return out, nil
}

func buildCredentialsInfo(creds []*types.PluginStaticCredentialsV1) *types.PluginOktaCredentialsInfo {
	var out types.PluginOktaCredentialsInfo
	for _, cred := range creds {
		purpose, ok := cred.GetAllLabels()[types.OktaCredPurposeLabel]
		if !ok {
			continue
		}
		switch purpose {
		case common.CredPurposeOktaOauth:
			out.HasOauthCredentials = true
		case types.OktaCredPurposeSCIMToken:
			out.HasScimToken = true
		case types.OktaCredPurposeAuth, "":
			out.HasSsmToken = true
		}
	}
	return &out
}

type createOktaClientOption func(*oktaapi.Config)

func withOAuthScopes(scopes []string) createOktaClientOption {
	return func(cfg *oktaapi.Config) {
		cfg.Scopes = scopes
	}
}

// createOktaClient creates Okta client from the request payload or saved credentials. The
// credentials from the request payload have higher priority than the stored plugin credentials. If
// the client can't be created from the request and plugin are nil then createOktaClient will make
// an attempt to fetch the plugin from the backend.
func (s *Service) createOktaClient(ctx context.Context, req requestWithCredentials, plugin *types.PluginV1, opts ...createOktaClientOption) (oktaapi.Interface, error) {
	var orgURL string
	var selectedCreds oktaplugin.SelectedOktaCredentials
	switch {
	case req != nil && req.GetApiCredentials() != nil:
		if req.GetOktaOrganizationUrl() == "" {
			return nil, trace.BadParameter("request has credentials but no Okta org URL")
		}
		orgURL = req.GetOktaOrganizationUrl()
		selectedCreds = oktaplugin.SelectedOktaCredentials{
			OauthClientId: req.GetApiCredentials().GetOauthId(),
			ApiToken:      req.GetApiCredentials().GetSswsBearerToken(),
		}
	case plugin == nil:
		var err error
		plugin, err = oktaplugin.Get(ctx, s.pluginBackend, true /* withSecrets */)
		if trace.IsNotFound(err) {
			return nil, trace.BadParameter("Okta API credentials not provided in the request and Okta plugin does not exist")
		} else if err != nil {
			return nil, trace.Wrap(err, "getting Okta plugin")
		}
		fallthrough
	default:
		orgURL = plugin.Spec.GetOkta().OrgUrl
		staticCredsRef := plugin.GetCredentials().GetStaticCredentialsRef()
		staticCreds, err := oktaplugin.GetStaticCredentials(ctx, s.credsBackend, staticCredsRef)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		selectedCreds, err = oktaplugin.SelectOktaCredentials(staticCreds)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	var authProvider oktaapi.AuthProvider
	switch {
	case selectedCreds.OauthClientId != "":
		authProvider = oktaapi.NewOauthProviderWithOktaCASigner(ctx, oktaapi.OauthOktaCACredentialsConfig{
			OAuthClientID: selectedCreds.OauthClientId,
			AuthService:   s.authCache,
			CAKeyStore:    s.jwtSigner,
			Clock:         s.clock,
		})
	case selectedCreds.ApiToken != "":
		authProvider = oktaapi.NewSSWSAuthProvider(selectedCreds.ApiToken)
	default:
		return nil, trace.BadParameter("Okta API credentials not found in plugin static credentials")
	}

	cfg := oktaapi.Config{
		TestHTTPClient: &http.Client{
			Transport: s.roundTripper,
			Timeout:   defaults.HTTPRequestTimeout,
		},
		OrgUrl:       orgURL,
		AuthProvider: authProvider,
		Log:          s.logger,
	}

	for _, o := range opts {
		o(&cfg)
	}

	oktaClient, err := s.apiClientProviderFn(ctx, cfg)
	if err != nil {
		return nil, trace.Wrap(err, "creating Okta client")
	}

	return oktaClient, nil
}

func buildAPITokenCredentials(SSWSToken string) *types.PluginStaticCredentialsV1 {
	return &types.PluginStaticCredentialsV1{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name:   types.PluginTypeOkta,
				Labels: map[string]string{types.OktaCredPurposeLabel: types.OktaCredPurposeAuth},
			},
		},
		Spec: &types.PluginStaticCredentialsSpecV1{
			Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
				APIToken: SSWSToken,
			},
		},
	}
}

func buildOAuthCredentials(clientID string) *types.PluginStaticCredentialsV1 {
	return &types.PluginStaticCredentialsV1{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name:   types.PluginTypeOkta,
				Labels: map[string]string{types.OktaCredPurposeLabel: common.CredPurposeOktaOauth},
			},
		},
		Spec: &types.PluginStaticCredentialsSpecV1{
			Credentials: &types.PluginStaticCredentialsSpecV1_OAuthClientSecret{
				OAuthClientSecret: &types.PluginStaticCredentialsOAuthClientSecret{
					ClientId: clientID,
					// ClientSecret is not needed for Okta OAuth
					// with JWKS flow so we set it to dummy value to avoid validation errors.
					ClientSecret: "none",
				},
			},
		},
	}
}

func buildSCIMCredentials(scimToken string) *types.PluginStaticCredentialsV1 {
	return &types.PluginStaticCredentialsV1{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name:   common.OktaSCIMTokenName,
				Labels: map[string]string{types.OktaCredPurposeLabel: types.OktaCredPurposeSCIMToken},
			},
		},
		Spec: &types.PluginStaticCredentialsSpecV1{Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{APIToken: scimToken}},
	}
}
