package jamf_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

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

	// Sanity check fake behavior. Following tests depend on it.
	t.Run("bad token fails", func(t *testing.T) {
		client := env.MustNewClient()
		client.SetAuthToken(&jamf.AuthToken{
			Token:   "who's bad?",
			Expires: clock.Now().Add(1 * time.Hour),
		})

		_, err := client.GetComputersInventory(ctx, &jamf.GetComputersInventoryRequest{})
		if err == nil {
			t.Fatalf("GetComputersInventory succeeded, want Unauthorized error")
		}
		apiErr := &jamf.APIError{}
		if !errors.As(err, &apiErr) {
			t.Fatalf("GetComputersInventory returned err=%v, want jamf.APIError", err)
		}
		if got, want := apiErr.StatusCode, 401; got != want {
			t.Errorf("GetComputersInventory returned status=%v, want %v", got, want)
		}
	})

	t.Run("automatic authn", func(t *testing.T) {
		client := env.MustNewClient()

		// Absence of errors is good enough.
		if _, err := client.GetComputersInventory(ctx, &jamf.GetComputersInventoryRequest{}); err != nil {
			t.Fatalf("GetComputersInventory failed: %v", err)
		}
	})

	mustGetComputersInventory := func(t *testing.T, client *jamf.Client) {
		if _, err := client.GetComputersInventory(ctx, &jamf.GetComputersInventoryRequest{}); err != nil {
			t.Fatalf("GetComputersInventory failed: %v", err)
		}
	}

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
		clock.Advance(jamffake.TokenExpiryPeriod - 1*time.Minute)

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

	t.Run("repeated authn failures cause ErrMaxAuthnAttemptsReached", func(t *testing.T) {
		client, err := jamf.NewClient(jamf.ClientOpts{
			Clock:      clock,
			Logger:     env.Logger,
			HTTPClient: env.HTTPClient,
			APIURL:     env.APIEndpoint,
			Username:   "invalid",
			Password:   "not a password",
		})
		if err != nil {
			t.Fatalf("NewClient failed: %v", err)
		}
		client.UsePlainHTTP()

		req := &jamf.GetComputersInventoryRequest{}
		const maxAttempts = 20 // We should reach an error before this.
		for i := 0; i < maxAttempts; i++ {
			if _, err := client.GetComputersInventory(ctx, req); errors.Is(err, jamf.ErrMaxAuthnAttemptsReached) {
				return // Test successful
			}
		}
		t.Fatal("Client never reached ErrMaxAuthnAttemptsReached")
	})
}
