package oktaapi

import (
	"context"
	"crypto"

	"github.com/go-jose/go-jose/v3"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/okta/okta-sdk-golang/v2/okta"

	"github.com/gravitational/teleport/api/types"
	libjwt "github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/services"
)

// NewOauthProvider returns a new Okta OAuth client with here token will be signed by the injected signer.
func NewOauthProvider(ctx context.Context, clientID string, signer signer) *OAuthProvider {
	return &OAuthProvider{
		ctx:           ctx,
		oauthClientID: clientID,
		signer:        signer,
	}
}

// NewOauthProviderWithOktaCASigner returns a new Okta OAuth client where token will be signed by the Teleport Okta CA.
func NewOauthProviderWithOktaCASigner(ctx context.Context, config OauthOktaCACredentialsConfig) *OAuthProvider {
	return &OAuthProvider{
		ctx:           ctx,
		oauthClientID: config.OAuthClientID,
		signer: &oktaCASigner{
			authService: config.AuthService,
			caKeyStore:  config.CAKeyStore,
			clock:       config.Clock,
		},
	}
}

// OauthOktaCACredentialsConfig is the configuration for the Okta OAuth client.
// It is used to authenticate with Okta using the Teleport Okta CA.
type OauthOktaCACredentialsConfig struct {
	// OAuthClientID is the OAuth client ID
	OAuthClientID string
	// AuthService is the Teleport authentication service from where they
	// Teleport Okta CA credentials are fetched.
	AuthService authService
	// CAKeyStore is the Teleport CA key store.
	CAKeyStore caKeyStore
	// Clock is the clock used to get the current time.
	Clock clockwork.Clock
}

type caKeyStore interface {
	GetJWTSigner(ctx context.Context, ca types.CertAuthority) (crypto.Signer, error)
}

// OAuthProvider is an Okta OAuth provider that will allow to authenticate
// using the private key returned by keyGetter and the OAuth client ID.
// https://developer.okta.com/docs/guides/implement-oauth-for-okta/main/
type OAuthProvider struct {
	ctx           context.Context
	oauthClientID string
	signer        signer
}

// GetAuthOptions returns the Okta client configuration options.
func (s OAuthProvider) GetAuthOptions() []okta.ConfigSetter {
	return []okta.ConfigSetter{
		okta.WithAuthorizationMode("PrivateKey"),
		okta.WithClientId(s.oauthClientID),
		okta.WithPrivateKeySigner(&s),
	}
}

// Sign signs the payload. It is used to sign the JWT token by the CA returned by keyGetter (Okta CA).
// This function is called by the Okta SDK during the authentication process.
// Okta SDK doesn't allow a dynamic fetcher. Where the Teleport CA can be rotated and the Okta SDK
// doesn't provide a way to fetch the new CA. So, we have to implement the signer interface to
// provide the Okta SDK with the signing key.
func (s OAuthProvider) Sign(payload []byte) (*jose.JSONWebSignature, error) {
	signature, err := s.signer.Sign(s.ctx, payload)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return signature, nil
}

// Options returns the signer options.
// Needed to fit the OKTA SDK interface.
func (s OAuthProvider) Options() jose.SignerOptions {
	return jose.SignerOptions{}
}

type authService interface {
	GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error)
	// GetClusterName returns the name of the cluster.
	GetClusterName(ctx context.Context) (types.ClusterName, error)
}

// authOktaCASigningKeys implements the keyGetter interface
// and returns the Teleport OKTA CA signing key that is used to sign the JWT token.
type oktaCASigner struct {
	// AuthService is the Teleport authentication service.
	authService authService
	caKeyStore  caKeyStore
	clock       clockwork.Clock
}

func (o *oktaCASigner) Sign(ctx context.Context, payload []byte) (*jose.JSONWebSignature, error) {
	name, err := o.authService.GetClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ca, err := o.authService.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.OktaCA,
		DomainName: name.GetClusterName(),
	}, /* loadKeys */ true)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	signer, err := o.caKeyStore.GetJWTSigner(ctx, ca)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	key, err := services.GetJWTSigner(signer, name.GetClusterName(), o.clock)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	kid, err := libjwt.KeyID(signer.Public())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	signature, err := key.SignPayload(payload, (&jose.SignerOptions{}).WithHeader("kid", kid))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return signature, nil
}

type signer interface {
	Sign(ctx context.Context, payload []byte) (*jose.JSONWebSignature, error)
}
