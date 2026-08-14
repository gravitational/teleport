package integrationv1

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/zitadel/oidc/v3/pkg/client"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"google.golang.org/protobuf/types/known/timestamppb"

	integrationpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/observability/otelhttp"
)

func (s *Service) CompleteOAuthProxyExchange(ctx context.Context, req *integrationpb.CompleteOAuthProxyExchangeRequest) (*integrationpb.CompleteOAuthProxyExchangeResponse, error) {
	switch {
	case req.GetName() == "":
		return nil, trace.BadParameter("name is required")
	case req.GetAuthorizationCode() == "":
		return nil, trace.BadParameter("authorization code is required")
	case req.GetCodeVerifier() == "":
		return nil, trace.BadParameter("code_verifier is required")
	case req.GetRedirectUri() == "":
		return nil, trace.BadParameter("redirect_uri is required")
	}

	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindIntegration, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	// With secrets.
	ig, err := s.backend.GetIntegration(ctx, req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if ig.GetSubKind() != types.IntegrationSubKindOAuthProxy {
		return nil, trace.BadParameter("invalid integration subkind %q", ig.GetSubKind())
	}

	spec := ig.GetOAuthProxyIntegrationSpec()
	if spec.ClientId == "" || spec.Issuer == "" || len(spec.Scopes) == 0 {
		return nil, trace.BadParameter("bad spec")
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	dc, err := client.Discover(timeoutCtx, spec.Issuer, otelhttp.DefaultClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	u, err := url.Parse(dc.TokenEndpoint)
	if err != nil {
		return nil, trace.BadParameter("invalid token endpoint")
	}

	q := url.Values{}
	q.Add("grant_type", "authorization_code")
	q.Add("code", req.GetAuthorizationCode())
	q.Add("code_verifier", req.GetCodeVerifier())
	q.Add("client_id", spec.ClientId)
	q.Add("redirect_uri", req.GetRedirectUri())

	r, err := http.NewRequestWithContext(ctx, "POST", u.String(), strings.NewReader(q.Encode()))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := otelhttp.DefaultClient.Do(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var e oidc.Error
		if err := json.NewDecoder(resp.Body).Decode(&e); err != nil {
			return nil, trace.Wrap(err)
		}
		return nil, trace.Errorf("status: %d, error: %s", resp.StatusCode, e.Unwrap())
	}

	var accessTokenResp oidc.AccessTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&accessTokenResp); err != nil {
		return nil, trace.Wrap(err)
	}

	// expires_in should always be set by Okta, but just in case...
	if accessTokenResp.ExpiresIn == 0 {
		return nil, trace.Errorf("expect expires_in to be non-zero")
	}

	if err := ig.SetCredentials(&types.PluginCredentialsV1{
		Credentials: &types.PluginCredentialsV1_Oauth2AccessToken{
			Oauth2AccessToken: &types.PluginOAuth2AccessTokenCredentials{
				AccessToken:  accessTokenResp.AccessToken,
				RefreshToken: accessTokenResp.RefreshToken,
				Expires:      s.clock.Now().Add(time.Duration(accessTokenResp.ExpiresIn) * time.Second),
			},
		},
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	_, err = s.backend.UpdateIntegration(ctx, ig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &integrationpb.CompleteOAuthProxyExchangeResponse{
		Expires: timestamppb.New(ig.GetCredentials().GetOauth2AccessToken().Expires),
	}, nil
}
