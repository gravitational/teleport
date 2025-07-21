package jamf_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/e/lib/jamf"
	jamffake "github.com/gravitational/teleport/e/lib/jamf/fake"
	"github.com/gravitational/teleport/e/lib/jamf/testenv"
)

func TestClient_authn(t *testing.T) {
	clock := clockwork.NewFakeClock()
	env := testenv.MustNew(&testenv.Opts{
		Clock: clock,
	})
	defer env.Close()

	ctx := context.Background()

	t.Run("automatic authn", func(t *testing.T) {
		client := env.MustNewClient()

		// Absence of errors is good enough.
		mustGetComputersInventory(t, client)
	})

	t.Run("automatic keep-alive", func(t *testing.T) {
		client := env.MustNewClient()

		mustGetComputersInventory(t, client) // Make sure a token exists.
		var tokenBefore string
		if authToken := client.AuthToken(); authToken == nil {
			t.Fatal("Client AuthToken is unexpectedly nil")
		} else {
			tokenBefore = authToken.Token
		}

		// Hit the API again, token should be reused.
		mustGetComputersInventory(t, client)
		if got := client.AuthToken().Token; got != tokenBefore {
			t.Errorf("Client AuthToken changed unexpectedly: %v vs %v", got, tokenBefore)
		}

		// Advance the clock closer to expiration.
		clock.Advance(jamffake.TokenExpiryPeriod - 1*time.Second)

		// Hit the API, a token refresh is now expected.
		mustGetComputersInventory(t, client)
		if got := client.AuthToken().Token; got == tokenBefore {
			t.Errorf("Client AuthToken not refreshed: %v vs %v", got, tokenBefore)
		}
	})

	t.Run("acquire new token after expiration", func(t *testing.T) {
		client := env.MustNewClient()
		mustGetComputersInventory(t, client) // Make sure a token exists.

		// Expire current token.
		// The next API action can only work by acquiring a new token.
		clock.Advance(jamffake.TokenExpiryPeriod * 2)

		mustGetComputersInventory(t, client)
	})

	t.Run("invalidated token healed", func(t *testing.T) {
		const badToken = "who's bad?"
		client := env.MustNewClient()
		client.SetAuthToken(&jamf.AuthToken{
			Token:   badToken,
			Expires: clock.Now().Add(1 * time.Hour),
		})

		mustGetComputersInventory(t, client)

		// Assert that the token changed.
		if client.AuthToken().Token == badToken {
			t.Error("Client AuthToken unexpectedly unchanged")
		}
	})

	t.Run(`try base URL with "/api" suffix`, func(t *testing.T) {
		client, err := jamf.NewClient(ctx, jamf.ClientOpts{
			Clock:      clock,
			Logger:     env.Logger,
			HTTPClient: env.HTTPClient,
			APIURL:     strings.TrimSuffix(env.APIEndpoint, "/api"),
			Username:   testenv.DefaultUsers[0].Username,
			Password:   testenv.DefaultUsers[0].Password,
		})
		if err != nil {
			t.Fatalf("NewClient returned err=%v, want nil", err)
		}
		mustGetComputersInventory(t, client) // Just to be sure
	})

	t.Run("invalid credentials fail creation", func(t *testing.T) {
		if _, err := jamf.NewClient(ctx, jamf.ClientOpts{
			Clock:      clock,
			Logger:     env.Logger,
			HTTPClient: env.HTTPClient,
			APIURL:     env.APIEndpoint,
			Username:   "invalid",
			Password:   "not a password",
		}); err == nil {
			t.Error("NewClient returned err=nil, wanted invalid credentials error")
		}
	})

	t.Run("repeated authn failures cause ErrMaxAuthnAttemptsReached", func(t *testing.T) {
		client, err := env.NewClient()
		if err != nil {
			t.Fatalf("NewClient failed: %v", err)
		}
		mustGetComputersInventory(t, client) // client works to begin with

		// Change underlying users.
		defer func() { env.API.SetUsers(testenv.DefaultUsers) }()
		env.API.SetUsers([]*jamffake.User{
			{
				Username: testenv.DefaultUsers[0].Username,
				Password: "changed password",
			},
			{
				Username: testenv.DefaultUsers[1].Username,
				Password: "another changed password",
			},
		})
		// Expire current token.
		// All subsequence authn attempts should fail.
		clock.Advance(jamffake.TokenExpiryPeriod * 2)

		// Further authn attempts should all fail.
		req := &jamf.GetComputersInventoryRequest{}
		const maxAttempts = 20 // We should reach an error before this.
		for range maxAttempts {
			if _, err := client.GetComputersInventory(ctx, req); errors.Is(err, jamf.ErrMaxAuthnAttemptsReached) {
				return // Test successful
			}
		}
		t.Fatal("Client never reached ErrMaxAuthnAttemptsReached")
	})
}

func TestClient_clientCredentialsAuthn(t *testing.T) {
	clock := clockwork.NewFakeClock()
	env := testenv.MustNew(&testenv.Opts{
		Clock: clock,
	})
	defer env.Close()

	// Setup API for client credential authn.
	const clientID = "llama-UUID"
	const clientSecret = "supersecretsecret"
	api := env.API
	api.SetUsers(nil)
	api.SetAPIClients([]*jamffake.APIClient{
		{
			ID:     clientID,
			Secret: clientSecret,
		},
	})

	ctx := context.Background()
	client, err := jamf.NewClient(ctx, jamf.ClientOpts{
		Clock:        env.Clock,
		Logger:       env.Logger,
		HTTPClient:   env.HTTPClient,
		APIURL:       env.APIEndpoint,
		ClientID:     clientID,
		ClientSecret: clientSecret,
	})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	t.Run("ok", func(t *testing.T) {
		// Absence of errors is good enough for us.
		mustGetComputersInventory(t, client)
	})

	t.Run("renew token just before expiration", func(t *testing.T) {
		tokenBefore := client.AuthToken().GetAccessToken()
		clock.Advance(jamffake.CredentialExpiryPeriod - 4*time.Second)
		mustGetComputersInventory(t, client)
		require.NotEqual(t, tokenBefore, client.AuthToken().GetAccessToken(),
			"Client AuthToken not refreshed",
		)
	})

	// Unlikely to happen due to the client checking the expiry of the token before sending a request.
	// But it does simulate a situation in which the API disagrees with the client wrt token expiry.
	t.Run("acquire new token after expiration", func(t *testing.T) {
		// Advance time to make the API consider the token to be expired.
		clock.Advance(jamffake.CredentialExpiryPeriod + 1*time.Second)

		// Make the client think the token is still valid.
		authToken := client.AuthToken()
		authToken.Expires = clock.Now().Add(3 * time.Hour)
		client.SetAuthToken(authToken)

		mustGetComputersInventory(t, client)
		require.NotEqual(t, authToken.GetAccessToken(), client.AuthToken().GetAccessToken(),
			"Client AuthToken not refreshed",
		)
	})
}

func mustGetComputersInventory(t *testing.T, client *jamf.Client) {
	t.Helper()

	ctx := context.Background()
	if _, err := client.GetComputersInventory(ctx, &jamf.GetComputersInventoryRequest{}); err != nil {
		t.Fatalf("GetComputersInventory failed: %v", err)
	}
}
