package scim

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"

	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/e/tests/common/idp"
)

func TestOauthToken(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClock()

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
		common.WithClock(clock),
	)

	clientID, clientSecret := createGenericSCIMPluginOauth(t, sut)

	baseURL := buildURL(sut.ProxyAddr, "/v1/webapi/scim/generic")
	tokenURL := buildURL(sut.ProxyAddr, "/v1/webapi/plugin/generic/token")

	insecureHTTPClient := &http.Client{
		Transport: insecureTransport(),
	}

	config := clientcredentials.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		TokenURL:     tokenURL.String(),
	}

	ctx := context.WithValue(t.Context(), oauth2.HTTPClient, insecureHTTPClient)
	oauthHttpClient := config.Client(ctx)

	t.Run("valid token allows access", func(t *testing.T) {
		resp := mustHTTPGet(t, oauthHttpClient, baseURL.String()+"/ResourceTypes")
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("invalid token denies access", func(t *testing.T) {
		httpClient := newBearerClient("invalid-token")
		resp := mustHTTPGet(t, httpClient, baseURL.String()+"/ResourceTypes")
		defer resp.Body.Close()
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("token expiration", func(t *testing.T) {
		token := mustGetToken(t, config, ctx)

		httpClient := newBearerClient(token.AccessToken)
		resp := mustHTTPGet(t, httpClient, baseURL.String()+"/ResourceTypes")
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		// Simulate token expiration
		clock.Advance(token.Expiry.Sub(clock.Now()) + time.Minute*2)

		// Should now be unauthorized after clock advancement after token expiry
		resp = mustHTTPGet(t, httpClient, baseURL.String()+"/ResourceTypes")
		defer resp.Body.Close()
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

		// Refresh token and retry the call
		newToken := mustGetToken(t, config, ctx)
		httpClient = newBearerClient(newToken.AccessToken)
		resp = mustHTTPGet(t, httpClient, baseURL.String()+"/ResourceTypes")
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})
}
