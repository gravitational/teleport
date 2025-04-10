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
		purpose, ok := cred.GetAllLabels()[common.CredPurposeLabel]
		if !ok {
			continue
		}
		switch purpose {
		case common.CredPurposeOktaOauth:
			out.HasOauthCredentials = true
		case common.CredPurposeSCIMToken:
			out.HasScimToken = true
		case common.CredPurposeOktaAuth, "":
			out.HasSsmToken = true
		}
	}
	return &out
}

type createOktaClientParams struct {
	orgUrl               string
	requestCreds         *oktapb.OktaAPICredentials
	pluginStaticCredsRef *types.PluginStaticCredentialsRef
}

// createOktaClient creates Okta client from the request payload or saved credentials.
// This function is shared function between Create and Update plugin flow.
// The credentials from the request payload have higher priority than the saved credentials
// to support the case when the user wants to update the Okta credentials.
func (s *Service) createOktaClient(ctx context.Context, params createOktaClientParams) (oktaapi.Interface, error) {
	if params.orgUrl == "" {
		return nil, trace.BadParameter("missing Okta organization URL")
	}

	var selectedCreds oktaplugin.SelectedOktaCredentials
	switch {
	case params.requestCreds != nil:
		selectedCreds = oktaplugin.SelectedOktaCredentials{
			OauthClientId: params.requestCreds.GetOauthId(),
			ApiToken:      params.requestCreds.GetSswsBearerToken(),
		}
	case params.pluginStaticCredsRef != nil:
		staticCreds, err := oktaplugin.GetStaticCredentials(ctx, s.credsBackend, params.pluginStaticCredsRef)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		selectedCreds, err = oktaplugin.SelectOktaCredentials(staticCreds)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	default:
		return nil, trace.BadParameter("either request credentials or plugin credential ref missing")
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

	oktaClient, err := s.apiClientProviderFn(ctx, oktaapi.Config{
		TestHTTPClient: &http.Client{
			Transport: s.roundTripper,
			Timeout:   defaults.HTTPRequestTimeout,
		},
		OrgUrl:       params.orgUrl,
		AuthProvider: authProvider,
		Log:          s.logger,
	})
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
				Labels: map[string]string{common.CredPurposeLabel: common.CredPurposeOktaAuth},
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
				Labels: map[string]string{common.CredPurposeLabel: common.CredPurposeOktaOauth},
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
				Labels: map[string]string{common.CredPurposeLabel: common.CredPurposeSCIMToken},
			},
		},
		Spec: &types.PluginStaticCredentialsSpecV1{Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{APIToken: scimToken}},
	}
}
