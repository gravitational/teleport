package oktaapi

import (
	"context"
	"os"
	"testing"

	"github.com/go-jose/go-jose/v3"
	"github.com/gravitational/trace"
	"github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/stretchr/testify/require"
)

func TestNewSSWSAuthProvider(t *testing.T) {
	orgURL := os.Getenv("OKTA_CLIENT_ORGURL")
	oktaClientToken := os.Getenv("OKTA_CLIENT_TOKEN")
	if orgURL == "" || oktaClientToken == "" {
		t.Skip("OKTA_CLIENT_ORGURL or OKTA_CLIENT_TOKEN not set")
	}

	ctx := context.Background()
	client, err := New(ctx, Config{
		OrgUrl:       orgURL,
		AuthProvider: NewSSWSAuthProvider(oktaClientToken),
	})
	require.NoError(t, err)

	_, err = client.ListUsers(ctx)
	require.NoError(t, err)
}

type fileKeyGetter struct {
	file  string
	keyID string
}

func (f fileKeyGetter) Sign(ctx context.Context, payload []byte) (*jose.JSONWebSignature, error) {
	buff, err := os.ReadFile(f.file)
	if err != nil {
		return nil, err
	}
	signer, err := okta.CreateKeySigner(string(buff), f.keyID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return signer.Sign(payload)
}

func TestOAuthProvider(t *testing.T) {
	orgURL := os.Getenv("OKTA_CLIENT_ORGURL")
	oktaClientID := os.Getenv("OKTA_CLIENT_CLIENTID")
	privateKey := os.Getenv("OKTA_CLIENT_PRIVATEKEY")
	privateKeyID := os.Getenv("OKTA_CLIENT_PRIVATEKEYID")

	if orgURL == "" || oktaClientID == "" || privateKey == "" || privateKeyID == "" {
		t.Skip("OKTA_CLIENT_ORGURL, OKTA_CLIENT_CLIENTID, OKTA_CLIENT_PRIVATEKEY, OKTA_CLIENT_PRIVATEKEYID not set")
	}

	ctx := context.Background()

	client, err := New(ctx, Config{
		OrgUrl: orgURL,
		AuthProvider: NewOauthProvider(ctx, oktaClientID, &fileKeyGetter{
			file:  privateKey,
			keyID: privateKeyID,
		}),
	})
	require.NoError(t, err)

	_, err = client.ListUsers(ctx)
	require.NoError(t, err)
}
