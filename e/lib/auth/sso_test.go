package auth

import (
	"crypto"
	"encoding/json"
	"net/url"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/e/lib/auth/oidctest"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/plugin"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
)

func newTestTLSServer(t *testing.T, m modules.Modules, lc LicenseChecker, opts ...authtest.TestTLSServerOption) *authtest.TLSServer {
	t.Helper()
	as, err := authtest.NewAuthServer(authtest.AuthServerConfig{
		Dir:     t.TempDir(),
		Clock:   clockwork.NewFakeClock(),
		Modules: m,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, as.Close()) })

	srv, err := as.NewTestTLSServer(opts...)
	require.NoError(t, err)

	registerSAMLService(t, &SAMLAuthServiceConfig{Auth: as.AuthServer, LicenseChecker: lc})
	registerOIDCService(t, &OIDCAuthServiceConfig{Auth: as.AuthServer, LicenseChecker: lc})

	t.Cleanup(func() { require.NoError(t, srv.Close()) })
	return srv
}

func registerSAMLService(t *testing.T, cfg *SAMLAuthServiceConfig) *SAMLAuthService {
	t.Helper()
	sas, err := NewSAMLAuthService(cfg)
	require.NoError(t, err)
	cfg.Auth.SetSAMLService(sas)
	return sas
}

func registerOIDCService(t *testing.T, cfg *OIDCAuthServiceConfig) *OIDCAuthService {
	t.Helper()
	oas, err := NewOIDCAuthService(cfg)
	require.NoError(t, err)
	cfg.Auth.SetOIDCService(oas)
	return oas
}

func TestOIDCAuthCompat(t *testing.T) {
	t.Parallel()
	testModules := &modulestest.Modules{
		TestFeatures: modules.Features{Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
			entitlements.OIDC: {Enabled: true},
		}},
	}

	ctx := t.Context()
	srv := newTestTLSServer(t, testModules, ValidLicense{}, func(cfg *authtest.TLSServerConfig) {
		authPlugin, err := NewPlugin(Config{
			License:        ValidLicense{},
			LicenseChecker: ValidLicense{},
			Modules:        testModules,
		})
		require.NoError(t, err)
		reg := plugin.NewRegistry()
		reg.Add(authPlugin)
		cfg.APIConfig.PluginRegistry = reg
	})

	_, err := authtest.CreateRole(ctx, srv.Auth(), "access", types.RoleSpecV6{})
	require.NoError(t, err)

	// Create a fake OIDC IdP that will authorize a fake user.
	idp, err := oidctest.NewIdPServer()
	require.NoError(t, err)
	t.Cleanup(idp.Close)

	// Replace the registered OIDC service with one whose backchannel HTTP
	// client trusts the fake IdP's TLS certificate.
	registerOIDCService(t, &OIDCAuthServiceConfig{
		Auth:           srv.Auth(),
		Client:         idp.Client(),
		LicenseChecker: ValidLicense{},
	})

	connector, err := types.NewOIDCConnector("example", types.OIDCConnectorSpecV3{
		IssuerURL:    idp.URL,
		ClientID:     "test",
		ClientSecret: "secret",
		PKCEMode:     string(constants.OIDCPKCEModeDisabled),
		Scope:        []string{oidc.ScopeOpenID, oidc.ScopeEmail, oidc.ScopeProfile},
		ClaimsToRoles: []types.ClaimMapping{{
			Claim: "groups",
			Value: "access",
			Roles: []string{"access"},
		}},
		RedirectURLs: wrappers.Strings{
			idp.URL + "/proxy/oidc/callback",
		},
	})
	require.NoError(t, err)

	_, err = srv.Auth().CreateOIDCConnector(ctx, connector)
	require.NoError(t, err)

	proxyClient, err := srv.NewClient(authtest.TestBuiltin(types.RoleProxy))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, proxyClient.Close()) })

	sshKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.Ed25519)
	require.NoError(t, err)
	sshPub, err := ssh.NewPublicKey(sshKey.Public())
	require.NoError(t, err)
	sshPubBytes := ssh.MarshalAuthorizedKey(sshPub)

	tlsKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)
	tlsPubBytes, err := keys.MarshalPublicKey(tlsKey.Public())
	require.NoError(t, err)

	for _, tc := range []struct {
		desc                 string
		sshPubKey, tlsPubKey []byte
		createWebSession     bool
		expectSSHSubjectKey  ssh.PublicKey
		expectTLSSubjectKey  crypto.PublicKey
	}{
		{
			desc: "no keys",
		},
		{
			desc:                "both keys",
			sshPubKey:           sshPubBytes,
			tlsPubKey:           tlsPubBytes,
			expectSSHSubjectKey: sshPub,
			expectTLSSubjectKey: tlsKey.Public(),
		},
		{
			desc:                "only ssh",
			sshPubKey:           sshPubBytes,
			expectSSHSubjectKey: sshPub,
		},
		{
			desc:                "only tls",
			tlsPubKey:           tlsPubBytes,
			expectTLSSubjectKey: tlsKey.Public(),
		},
		{
			desc:             "web session",
			createWebSession: true,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			// Validation runs over both the gRPC RPC used by proxies on v19+
			// and the legacy HTTP endpoint still used by older proxies.
			// TODO(strideynet): DELETE IN v20.0.0 - remove the http transport
			// along with the legacy endpoint.
			for _, transport := range []struct {
				name      string
				roundtrip func(t *testing.T, q url.Values) *authclient.OIDCAuthResponse
			}{
				{
					name: "wrapper",
					roundtrip: func(t *testing.T, q url.Values) *authclient.OIDCAuthResponse {
						// The authclient.Client wrapper used by production
						// callers, which tries gRPC and falls back to HTTP.
						resp, err := proxyClient.ValidateOIDCAuthCallback(t.Context(), q)
						require.NoError(t, err)
						return resp
					},
				},
				{
					name: "grpc",
					roundtrip: func(t *testing.T, q url.Values) *authclient.OIDCAuthResponse {
						// Call the gRPC client directly rather than via the
						// authclient.Client wrapper, so a broken RPC can't be
						// masked by the wrapper's HTTP fallback.
						resp, err := proxyClient.APIClient.ValidateOIDCAuthCallback(t.Context(),
							authclient.ValidateOIDCAuthCallbackRequestToProto(q))
						require.NoError(t, err)
						return authclient.OIDCAuthResponseFromProto(resp)
					},
				},
				{
					name: "http",
					roundtrip: func(t *testing.T, q url.Values) *authclient.OIDCAuthResponse {
						// Raw request pinning the legacy wire format sent by
						// older proxies.
						out, err := proxyClient.HTTPClient.PostJSON(t.Context(),
							proxyClient.HTTPClient.Endpoint("oidc", "requests", "validate"),
							struct {
								Query url.Values `json:"query"`
							}{Query: q})
						require.NoError(t, err)
						var raw struct {
							Username      string                     `json:"username"`
							Identity      types.ExternalIdentity     `json:"identity"`
							Session       json.RawMessage            `json:"session"`
							Cert          []byte                     `json:"cert"`
							TLSCert       []byte                     `json:"tls_cert"`
							Req           authclient.OIDCAuthRequest `json:"req"`
							HostSigners   []json.RawMessage          `json:"host_signers"`
							MFAToken      string                     `json:"mfa_token"`
							ClientOptions authclient.ClientOptions   `json:"client_options"`
						}
						require.NoError(t, json.Unmarshal(out.Bytes(), &raw))
						resp := &authclient.OIDCAuthResponse{
							Username:      raw.Username,
							Identity:      raw.Identity,
							Cert:          raw.Cert,
							TLSCert:       raw.TLSCert,
							Req:           raw.Req,
							MFAToken:      raw.MFAToken,
							ClientOptions: raw.ClientOptions,
						}
						if len(raw.Session) != 0 {
							session, err := services.UnmarshalWebSession(raw.Session)
							require.NoError(t, err)
							resp.Session = session
						}
						for _, rawCA := range raw.HostSigners {
							ca, err := services.UnmarshalCertAuthority(rawCA)
							require.NoError(t, err)
							resp.HostSigners = append(resp.HostSigners, ca)
						}
						return resp
					},
				},
			} {
				t.Run(transport.name, func(t *testing.T) {
					req, err := proxyClient.CreateOIDCAuthRequest(t.Context(), types.OIDCAuthRequest{
						ConnectorID:      connector.GetName(),
						CheckUser:        true,
						SshPublicKey:     tc.sshPubKey,
						TlsPublicKey:     tc.tlsPubKey,
						CreateWebSession: tc.createWebSession,
						CertTTL:          time.Hour,
					})
					require.NoError(t, err)

					// Simulate the user authenticating at the IdP, which
					// stores an auth code for the request's state token.
					code, err := idp.Authorize(t.Context(), &oidc.AuthRequest{
						Scopes:      []string{oidc.ScopeOpenID, oidc.ScopeEmail, oidc.ScopeProfile},
						ClientID:    connector.GetClientID(),
						RedirectURI: connector.GetRedirectURLs()[0],
						State:       req.StateToken,
						Display:     "none",
					}, "id1")
					require.NoError(t, err)

					resp := transport.roundtrip(t, url.Values{
						"code":  []string{code},
						"state": []string{req.StateToken},
					})

					// The proxy should get back the keys exactly as it sent them.
					require.Equal(t, tc.sshPubKey, resp.Req.SSHPubKey)
					require.Equal(t, tc.tlsPubKey, resp.Req.TLSPubKey)

					// Make sure the subject key in the issued SSH cert matches the
					// expected key and didn't get accidentally switched.
					if tc.expectSSHSubjectKey != nil {
						sshCert, err := sshutils.ParseCertificate(resp.Cert)
						require.NoError(t, err)
						require.Equal(t, tc.expectSSHSubjectKey, sshCert.Key)
					} else {
						// No SSH cert should be issued if we didn't ask for one.
						require.Empty(t, resp.Cert)
					}

					// Make sure the subject key in the issued TLS cert matches the
					// expected key and didn't get accidentally switched.
					if tc.expectTLSSubjectKey != nil {
						tlsCert, err := tlsca.ParseCertificatePEM(resp.TLSCert)
						require.NoError(t, err)
						require.Equal(t, tc.expectTLSSubjectKey, tlsCert.PublicKey)
					} else {
						// No TLS cert should be issued if we didn't ask for one.
						require.Empty(t, resp.TLSCert)
					}

					// A web session, secrets included, is issued only when
					// requested, and the host CA only alongside certs.
					if tc.createWebSession {
						require.NotNil(t, resp.Session)
						require.NotEmpty(t, resp.Session.GetName())
						require.NotEmpty(t, resp.Session.GetSSHPriv())
						require.NotEmpty(t, resp.Session.GetBearerToken())
					} else {
						require.Nil(t, resp.Session)
					}
					if tc.expectSSHSubjectKey != nil || tc.expectTLSSubjectKey != nil {
						require.NotEmpty(t, resp.HostSigners)
					} else {
						require.Empty(t, resp.HostSigners)
					}
				})
			}
		})
	}
}
