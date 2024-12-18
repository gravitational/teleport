package oktaservice

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/bcrypt"

	oktapb "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/okta/api"
	"github.com/gravitational/teleport/e/lib/okta/common"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
)

func getOktaPluginCredentials(req *oktapb.CreateIntegrationRequest) ([]*types.PluginStaticCredentialsV1, error) {
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
	credsFromReq            *oktapb.OktaAPICredentials
	oktaOrganization        string
	pluginCredentialsLabels map[string]string
	connectorID             string
}

func (p *createOktaClientParams) validate() error {
	if p.oktaOrganization == "" {
		return trace.BadParameter("missing Okta organization URL")
	}
	if p.credsFromReq.GetSswsBearerToken() == "" && p.credsFromReq.GetOauthId() == "" {
		return trace.BadParameter("missing Okta API credentials")
	}
	return nil
}

// createOktaClient creates Okta client from the request payload or saved credentials.
// This function is shared function between Create and Update plugin flow.
// The credentials from the request payload have higher priority than the saved credentials
// to support the case when the user wants to update the Okta credentials.
func (s *Service) createOktaClient(ctx context.Context, params *createOktaClientParams) (api.Client, error) {
	if params.credsFromReq != nil {
		oktaClient, err := s.createOktaClientFromRequestPayload(ctx, params)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return oktaClient, nil
	}
	// Okta credentials were not provided in the request.
	// We will try to create Okta client from saved credentials stored Teleport backend.
	oktaClient, err := s.createOktaClientFromSavedCred(ctx, params)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return oktaClient, nil
}

func (s *Service) createOktaClientFromSavedCred(ctx context.Context, params *createOktaClientParams) (api.Client, error) {
	staticCreds, err := s.credsBackend.GetPluginStaticCredentialsByLabels(ctx, params.pluginCredentialsLabels)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var authProvider api.AuthProvider
	for _, v := range staticCreds {
		// We will try to create Okta client from saved credentials stored Teleport backend.
		// Okta credentials can be SSWS, OAuth or SCIM token
		// but for client auth we only need OAuth or SSWS token.
		authProvider, err = s.oktaAuthProviderFromStaticCreds(ctx, v)
		if err != nil {
			continue
		}
		break
	}
	if authProvider == nil {
		return nil, trace.BadParameter("no Okta credentials found")
	}

	return s.apiClientProviderFn(ctx, api.ClientConfig{
		HTTPClient: &http.Client{
			Transport: s.roundTripper,
			Timeout:   time.Second * 7,
		},
		Endpoint:     params.oktaOrganization,
		AuthProvider: authProvider,
	})
}

func (s *Service) oktaAuthProviderFromStaticCreds(ctx context.Context, staticCred types.PluginStaticCredentials) (api.AuthProvider, error) {
	purpose, _ := staticCred.GetLabel(common.CredPurposeLabel)
	switch purpose {
	case common.CredPurposeOktaOauth:
		clientID, _ := staticCred.GetOAuthClientSecret()
		return api.NewOauthProviderWithOktaCASigner(ctx, api.OauthOktaCACredentialsConfig{
			OAuthClientID: clientID,
			AuthService:   s.authCache,
			CAKeyStore:    s.jwtSigner,
			Clock:         s.clock,
		}), nil
	case common.CredPurposeOktaAuth, common.CredPurposeOktaAPITokenWithSCIMOnlyIntegration, "":
		return api.NewSSWSAuthProvider(staticCred.GetAPIToken()), nil
	}
	return nil, trace.BadParameter("unexpected credential purpose %q", purpose)
}

func (s *Service) createOktaClientFromRequestPayload(ctx context.Context, params *createOktaClientParams) (api.Client, error) {
	if err := params.validate(); err != nil {
		return nil, trace.Wrap(err)
	}

	var authProvider api.AuthProvider
	switch {
	case params.credsFromReq.GetOauthId() != "":
		authProvider = api.NewOauthProviderWithOktaCASigner(ctx, api.OauthOktaCACredentialsConfig{
			OAuthClientID: params.credsFromReq.GetOauthId(),
			AuthService:   s.authCache,
			CAKeyStore:    s.jwtSigner,
			Clock:         s.clock,
		})
	case params.credsFromReq.GetSswsBearerToken() != "":
		authProvider = api.NewSSWSAuthProvider(params.credsFromReq.GetSswsBearerToken())
	default:
		return nil, trace.BadParameter("missing Okta API credentials")
	}

	oktaClient, err := s.apiClientProviderFn(ctx, api.ClientConfig{
		HTTPClient: &http.Client{
			Transport: s.roundTripper,
			Timeout:   defaults.HTTPRequestTimeout,
		},
		Endpoint:     params.oktaOrganization,
		AuthProvider: authProvider,
		Log:          slog.With("okta_url", params.oktaOrganization),
	})
	if err != nil {
		return nil, trace.Wrap(err)
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
