package intune_test

import (
	"errors"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/e/lib/intune"
	intunefake "github.com/gravitational/teleport/e/lib/intune/fake"
	"github.com/gravitational/teleport/e/lib/intune/testenv"
)

func TestClient_authn(t *testing.T) {
	clock := clockwork.NewFakeClock()
	env := testenv.MustNew(t, &testenv.Config{
		Clock: clock,
	})

	t.Run("automatic authn", func(t *testing.T) {
		client := env.MustNewClient(t)
		// Absence of errors is good enough.
		mustListManagedDevices(t, client)
	})

	t.Run("renew token just before expiration", func(t *testing.T) {
		client := env.MustNewClient(t)

		tokenBefore := client.AccessToken()
		require.NotNil(t, tokenBefore)

		// Hit the API again, token should be reused.
		mustListManagedDevices(t, client)
		require.Equal(t, tokenBefore.AccessToken, client.AccessToken().AccessToken,
			"Client AccessToken changed unexpectedly",
		)

		// Advance the clock closer to expiration.
		clock.Advance(intunefake.AccessTokenExpiryPeriod - 4*time.Second)

		// Hit the API, a token refresh is now expected.
		mustListManagedDevices(t, client)
		require.NotEqual(t, tokenBefore.AccessToken, client.AccessToken().AccessToken,
			"Client AccessToken not refreshed",
		)
	})

	// Unlikely to happen due to the client checking the expiry of the token before sending a request.
	// But it does simulate a situation in which the API disagrees with the client wrt token expiry.
	t.Run("acquire new token after expiration", func(t *testing.T) {
		client := env.MustNewClient(t)

		// Advance time to make the API consider the token to be expired.
		clock.Advance(intunefake.AccessTokenExpiryPeriod + time.Second)

		// Make the client think the token is still valid.
		accessToken := client.AccessToken()
		require.NotNil(t, accessToken)
		accessToken.Expires = clock.Now().Add(3 * time.Hour)
		client.SetAccessToken(accessToken)

		mustListManagedDevices(t, client)
		require.NotEqual(t, accessToken.AccessToken, client.AccessToken().AccessToken,
			"Client AccessToken not refreshed",
		)
	})

	// Similar to the test above, but simulates a situation in which the access token itself got
	// invalidated.
	t.Run("invalidated token healed", func(t *testing.T) {
		badToken := "foo"
		client := env.MustNewClient(t)
		client.SetAccessToken(&intune.AccessToken{
			AccessToken: badToken,
			Expires:     clock.Now().Add(time.Hour),
		})

		mustListManagedDevices(t, client)
		require.NotEqual(t, badToken, client.AccessToken().AccessToken,
			"Client AccessToken not refreshed",
		)
	})

	t.Run("invalid tenant fails creation", func(t *testing.T) {
		_, err := intune.NewClient(t.Context(), intune.ClientConfig{
			APIConfig: intune.APIConfig{
				AppCredentials: intune.AppCredentials{Tenant: "foo", ClientID: "invalid", ClientSecret: "not a secret"},
			},
			Logger:     env.Logger,
			HTTPClient: env.HTTPClient,
			Clock:      env.Clock,
		})
		require.ErrorIs(t, err, intune.ErrIntuneClientTenantNotFound)
	})

	t.Run("invalid client ID fails creation", func(t *testing.T) {
		_, err := intune.NewClient(t.Context(), intune.ClientConfig{
			APIConfig: intune.APIConfig{
				AppCredentials: intune.AppCredentials{Tenant: testenv.DefaultApps[0].Tenant, ClientID: "invalid", ClientSecret: "not a secret"},
			},
			Logger:     env.Logger,
			HTTPClient: env.HTTPClient,
			Clock:      env.Clock,
		})
		require.ErrorIs(t, err, intune.ErrIntuneClientInvalidCredentials)
	})

	t.Run("invalid client secret fails creation", func(t *testing.T) {
		_, err := intune.NewClient(t.Context(), intune.ClientConfig{
			APIConfig: intune.APIConfig{
				AppCredentials: intune.AppCredentials{Tenant: testenv.DefaultApps[0].Tenant, ClientID: testenv.DefaultApps[0].ClientID, ClientSecret: "not a secret"},
			},
			Logger:     env.Logger,
			HTTPClient: env.HTTPClient,
			Clock:      env.Clock,
		})
		require.ErrorIs(t, err, intune.ErrIntuneClientInvalidCredentials)
	})

	t.Run("repeated authn failures cause ErrMaxAuthnAttemptsReached", func(t *testing.T) {
		client := env.MustNewClient(t)
		mustListManagedDevices(t, client)

		// Change registered API apps.
		t.Cleanup(func() { env.API.SetApps(testenv.DefaultApps) })
		env.API.SetApps([]*intune.AppCredentials{
			{
				Tenant:       testenv.DefaultApps[0].Tenant,
				ClientID:     testenv.DefaultApps[0].ClientID,
				ClientSecret: "changed client secret",
			},
		})
		// Expire current token.
		clock.Advance(intunefake.AccessTokenExpiryPeriod * 2)

		const maxAttempts = 5 // We should reach an error before this.
		for range maxAttempts {
			if err := client.ListManagedDevices(t.Context()); errors.Is(err, intune.ErrMaxAuthnAttemptsReached) {
				return // Test successful
			}
		}
		t.Fatal("Client never reached ErrMaxAuthnAttemptsReached")
	})
}

func mustListManagedDevices(t *testing.T, client *intune.Client) {
	t.Helper()
	require.NoError(t, client.ListManagedDevices(t.Context()))
}
