package pgevents

import (
	"context"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/gravitational/trace"
	"github.com/jackc/pgx/v4"
	"github.com/sirupsen/logrus"
)

// AzureBeforeConnect will return a pgx BeforeConnect function suitable for
// Azure AD authentication. The returned function will set the password of the
// pgx.ConnConfig to a token for the relevant scope, fetching it and reusing it
// until expired (a burst of connections right at backend start is expected). If
// a client ID is provided, authentication will only be attempted as the managed
// identity with said ID rather than with all the default credentials.
func AzureBeforeConnect(clientID string, log logrus.FieldLogger) (func(ctx context.Context, config *pgx.ConnConfig) error, error) {
	var cred azcore.TokenCredential
	if clientID != "" {
		log.WithField("azure_client_id", clientID).Debug("Using Azure AD authentication with managed identity.")
		c, err := azidentity.NewManagedIdentityCredential(&azidentity.ManagedIdentityCredentialOptions{
			ID: azidentity.ClientID(clientID),
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		cred = c
	} else {
		log.Debug("Using Azure AD authentication with default credentials.")
		c, err := azidentity.NewDefaultAzureCredential(nil)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		cred = c
	}

	var mu sync.Mutex
	var cachedToken azcore.AccessToken

	beforeConnect := func(ctx context.Context, config *pgx.ConnConfig) error {
		mu.Lock()
		token := cachedToken
		mu.Unlock()

		// to account for clock drift between us, the database, and the IDMS,
		// refresh the token 10 minutes before we think it will expire
		if token.ExpiresOn.After(time.Now().Add(10 * time.Minute)) {
			log.WithField("ttl", time.Until(token.ExpiresOn).String()).Debug("Reusing cached Azure access token.")
			config.Password = token.Token
			return nil
		}

		log.Debug("Fetching new Azure access token.")
		token, err := cred.GetToken(ctx, policy.TokenRequestOptions{
			Scopes: []string{"https://ossrdbms-aad.database.windows.net/.default"},
		})
		if err != nil {
			return trace.Wrap(err)
		}

		log.WithField("ttl", time.Until(token.ExpiresOn).String()).Debug("Fetched Azure access token.")
		config.Password = token.Token

		mu.Lock()
		cachedToken = token
		mu.Unlock()

		return nil
	}

	return beforeConnect, nil
}
