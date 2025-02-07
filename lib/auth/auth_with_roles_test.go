/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package auth

import (
	"context"
	"crypto"
	"crypto/tls"
	"crypto/x509/pkix"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/gravitational/teleport"
	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	trustpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/trust/v1"
	userpreferencesv1 "github.com/gravitational/teleport/api/gen/proto/go/userpreferences/v1"
	"github.com/gravitational/teleport/api/metadata"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	apicommon "github.com/gravitational/teleport/api/types/common"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/installers"
	wanpb "github.com/gravitational/teleport/api/types/webauthn"
	"github.com/gravitational/teleport/api/types/wrappers"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
	"github.com/gravitational/teleport/lib/sshca"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils/pagination"
)

func TestGenerateUserCerts_MFAVerifiedFieldSet(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)

	u, err := createUserWithSecondFactors(srv)
	require.NoError(t, err)
	client, err := srv.NewClient(TestUser(u.username))
	require.NoError(t, err)

	// GenerateUserCerts requires MFA.
	client.SetMFAPromptConstructor(func(po ...mfa.PromptOpt) mfa.Prompt {
		return mfa.PromptFunc(func(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
			return u.webDev.SolveAuthn(chal)
		})
	})

	_, sshPubKey, _, tlsPubKey := newSSHAndTLSKeyPairs(t)

	for _, test := range []struct {
		desc           string
		getMFAResponse func() *proto.MFAAuthenticateResponse
		wantErr        string
	}{
		{
			desc: "valid mfa response",
			getMFAResponse: func() *proto.MFAAuthenticateResponse {
				// Get a totp code to re-auth.
				totpCode, err := totp.GenerateCode(u.totpDev.TOTPSecret, srv.AuthServer.Clock().Now().Add(30*time.Second))
				require.NoError(t, err)

				return &proto.MFAAuthenticateResponse{
					Response: &proto.MFAAuthenticateResponse_TOTP{
						TOTP: &proto.TOTPResponse{Code: totpCode},
					},
				}
			},
		},
		{
			desc: "valid empty mfa response",
			getMFAResponse: func() *proto.MFAAuthenticateResponse {
				return nil
			},
		},
		{
			desc:    "invalid mfa response",
			wantErr: "invalid totp token",
			getMFAResponse: func() *proto.MFAAuthenticateResponse {
				return &proto.MFAAuthenticateResponse{
					Response: &proto.MFAAuthenticateResponse_TOTP{
						TOTP: &proto.TOTPResponse{Code: "invalid-totp-code"},
					},
				}
			},
		},
	} {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			mfaResponse := test.getMFAResponse()
			certs, err := client.GenerateUserCerts(context.Background(), proto.UserCertsRequest{
				SSHPublicKey: sshPubKey,
				TLSPublicKey: tlsPubKey,
				Username:     u.username,
				Expires:      time.Now().Add(time.Hour),
				MFAResponse:  mfaResponse,
			})

			switch {
			case test.wantErr != "":
				require.True(t, trace.IsAccessDenied(err), "GenerateUserCerts returned err = %v (%T), wanted trace.AccessDenied", err, err)
				require.ErrorContains(t, err, test.wantErr)
				return
			default:
				require.NoError(t, err)
			}

			sshCert, err := sshutils.ParseCertificate(certs.SSH)
			require.NoError(t, err)
			mfaVerified := sshCert.Permissions.Extensions[teleport.CertExtensionMFAVerified]

			switch {
			case mfaResponse == nil:
				require.Empty(t, mfaVerified, "GenerateUserCerts returned certificate with non-empty CertExtensionMFAVerified")
			default:
				require.Equal(t, mfaVerified, u.totpDev.MFA.Id, "GenerateUserCerts returned certificate with unexpected CertExtensionMFAVerified")
			}
		})
	}
}

// TestLocalUserCanReissueCerts tests that local users can reissue
// certificates for themselves with varying TTLs.
func TestLocalUserCanReissueCerts(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)

	_, sshPubKey, _, tlsPubKey := newSSHAndTLSKeyPairs(t)

	start := srv.AuthServer.Clock().Now()

	for _, test := range []struct {
		desc         string
		renewable    bool
		roleRequests bool
		reqTTL       time.Duration
		expiresIn    time.Duration
	}{
		{
			desc:      "not-renewable",
			renewable: false,
			// expiration limited to duration of the user's session (default 1 hour)
			reqTTL:    4 * time.Hour,
			expiresIn: 1 * time.Hour,
		},
		{
			desc:         "not-renewable-role-requests",
			renewable:    false,
			roleRequests: true,
			// expiration is allowed to be pushed out into the future
			reqTTL:    4 * time.Hour,
			expiresIn: 4 * time.Hour,
		},
		{
			desc:      "renewable",
			renewable: true,
			reqTTL:    4 * time.Hour,
			expiresIn: 4 * time.Hour,
		},
		{
			desc:         "renewable-role-requests",
			renewable:    true,
			roleRequests: true,
			reqTTL:       4 * time.Hour,
			expiresIn:    4 * time.Hour,
		},
		{
			desc:      "max-renew",
			renewable: true,
			// expiration is allowed to be pushed out into the future,
			// but no more than the maximum renewable cert TTL
			reqTTL:    2 * defaults.MaxRenewableCertTTL,
			expiresIn: defaults.MaxRenewableCertTTL,
		},
		{
			desc:         "not-renewable-role-requests-max-renew",
			renewable:    false,
			roleRequests: true,
			reqTTL:       2 * defaults.MaxRenewableCertTTL,
			expiresIn:    defaults.MaxRenewableCertTTL,
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			ctx := context.Background()
			user, role, err := CreateUserAndRole(srv.Auth(), test.desc, []string{"role"}, nil)
			require.NoError(t, err)
			authPref, err := srv.Auth().GetAuthPreference(ctx)
			require.NoError(t, err)
			authPref.SetDefaultSessionTTL(types.Duration(test.expiresIn))
			_, err = srv.Auth().UpsertAuthPreference(ctx, authPref)
			require.NoError(t, err)

			var id TestIdentity
			if test.renewable {
				id = TestRenewableUser(user.GetName(), 0)

				meta := user.GetMetadata()
				meta.Labels = map[string]string{
					types.BotGenerationLabel: "0",
				}
				user.SetMetadata(meta)
				user, err = srv.Auth().UpsertUser(ctx, user)
				require.NoError(t, err)
			} else {
				id = TestUser(user.GetName())
			}

			client, err := srv.NewClient(id)
			require.NoError(t, err)

			req := proto.UserCertsRequest{
				SSHPublicKey: sshPubKey,
				TLSPublicKey: tlsPubKey,
				Username:     user.GetName(),
				Expires:      start.Add(test.reqTTL),
			}
			if test.roleRequests {
				// Reconfigure role to allow impersonation of its own role.
				role.SetImpersonateConditions(types.Allow, types.ImpersonateConditions{
					Roles: []string{role.GetName()},
				})
				role, err = srv.Auth().UpsertRole(ctx, role)
				require.NoError(t, err)

				req.UseRoleRequests = true
				req.RoleRequests = []string{role.GetName()}
			}

			certs, err := client.GenerateUserCerts(ctx, req)
			require.NoError(t, err)

			x509, err := tlsca.ParseCertificatePEM(certs.TLS)
			require.NoError(t, err)

			sshCert, err := sshutils.ParseCertificate(certs.SSH)
			require.NoError(t, err)

			require.WithinDuration(t, start.Add(test.expiresIn), x509.NotAfter, 1*time.Second)
			require.WithinDuration(t, start.Add(test.expiresIn), time.Unix(int64(sshCert.ValidBefore), 0), 1*time.Second)
		})
	}
}

// TestSSOUserCanReissueCert makes sure that SSO user can reissue certificate
// for themselves.
func TestSSOUserCanReissueCert(t *testing.T) {
	ctx := context.Background()
	srv := newTestTLSServer(t)

	// Create test SSO user.
	user, _, err := CreateUserAndRole(srv.Auth(), "sso-user", []string{"role"}, nil)
	require.NoError(t, err)
	user.SetCreatedBy(types.CreatedBy{
		Connector: &types.ConnectorRef{Type: "oidc", ID: "google"},
	})
	user, err = srv.Auth().UpdateUser(ctx, user)
	require.NoError(t, err)

	client, err := srv.NewClient(TestUser(user.GetName()))
	require.NoError(t, err)

	_, sshPubKey, _, tlsPubKey := newSSHAndTLSKeyPairs(t)
	_, err = client.GenerateUserCerts(ctx, proto.UserCertsRequest{
		SSHPublicKey: sshPubKey,
		TLSPublicKey: tlsPubKey,
		Username:     user.GetName(),
		Expires:      time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
}

func TestInstaller(t *testing.T) {
	ctx := context.Background()
	srv := newTestTLSServer(t)

	_, err := CreateRole(ctx, srv.Auth(), "test-empty", types.RoleSpecV6{})
	require.NoError(t, err)

	_, err = CreateRole(ctx, srv.Auth(), "test-read", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				{
					Resources: []string{types.KindInstaller},
					Verbs:     []string{types.VerbRead},
				},
			},
		},
	})
	require.NoError(t, err)
	_, err = CreateRole(ctx, srv.Auth(), "test-update", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				{
					Resources: []string{types.KindInstaller},
					Verbs:     []string{types.VerbUpdate, types.VerbCreate},
				},
			},
		},
	})
	require.NoError(t, err)
	_, err = CreateRole(ctx, srv.Auth(), "test-delete", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				{
					Resources: []string{types.KindInstaller},
					Verbs:     []string{types.VerbDelete},
				},
			},
		},
	})
	require.NoError(t, err)
	user, err := CreateUser(ctx, srv.Auth(), "testuser")
	require.NoError(t, err)

	inst, err := types.NewInstallerV1(installers.InstallerScriptName, "contents")
	require.NoError(t, err)
	err = srv.Auth().SetInstaller(ctx, inst)
	require.NoError(t, err)

	for _, tc := range []struct {
		roles           []string
		assert          require.ErrorAssertionFunc
		installerAction func(*authclient.Client) error
	}{{
		roles:  []string{"test-empty"},
		assert: require.Error,
		installerAction: func(c *authclient.Client) error {
			_, err := c.GetInstaller(ctx, installers.InstallerScriptName)
			return err
		},
	}, {
		roles:  []string{"test-read"},
		assert: require.NoError,
		installerAction: func(c *authclient.Client) error {
			_, err := c.GetInstaller(ctx, installers.InstallerScriptName)
			return err
		},
	}, {
		roles:  []string{"test-update"},
		assert: require.NoError,
		installerAction: func(c *authclient.Client) error {
			inst, err := types.NewInstallerV1(installers.InstallerScriptName, "new-contents")
			require.NoError(t, err)
			return c.SetInstaller(ctx, inst)
		},
	}, {
		roles:  []string{"test-delete"},
		assert: require.NoError,
		installerAction: func(c *authclient.Client) error {
			err := c.DeleteInstaller(ctx, installers.InstallerScriptName)
			return err
		},
	}} {
		user.SetRoles(tc.roles)
		user, err = srv.Auth().UpsertUser(ctx, user)
		require.NoError(t, err)

		client, err := srv.NewClient(TestUser(user.GetName()))
		require.NoError(t, err)
		tc.assert(t, tc.installerAction(client))
	}
}

func TestGithubAuthRequest(t *testing.T) {
	ctx := context.Background()
	srv := newTestTLSServer(t)

	emptyRole, err := CreateRole(ctx, srv.Auth(), "test-empty", types.RoleSpecV6{})
	require.NoError(t, err)

	access1Role, err := CreateRole(ctx, srv.Auth(), "test-access-1", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				{
					Resources: []string{types.KindGithubRequest},
					Verbs:     []string{types.VerbCreate},
				},
			},
		},
	})
	require.NoError(t, err)

	access2Role, err := CreateRole(ctx, srv.Auth(), "test-access-2", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				{
					Resources: []string{types.KindGithub},
					Verbs:     []string{types.VerbCreate},
				},
			},
		},
	})
	require.NoError(t, err)

	access3Role, err := CreateRole(ctx, srv.Auth(), "test-access-3", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				{
					Resources: []string{types.KindGithub, types.KindGithubRequest},
					Verbs:     []string{types.VerbCreate},
				},
			},
		},
	})
	require.NoError(t, err)

	readerRole, err := CreateRole(ctx, srv.Auth(), "test-access-4", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				{
					Resources: []string{types.KindGithubRequest},
					Verbs:     []string{types.VerbRead},
				},
			},
		},
	})
	require.NoError(t, err)

	conn, err := types.NewGithubConnector("example", types.GithubConnectorSpecV3{
		ClientID:     "example-client-id",
		ClientSecret: "example-client-secret",
		RedirectURL:  "https://localhost:3080/v1/webapi/github/callback",
		Display:      "sign in with github",
		TeamsToLogins: []types.TeamMapping{
			{
				Organization: "octocats",
				Team:         "idp-admin",
				Logins:       []string{"access"},
			},
		},
	})
	require.NoError(t, err)

	upserted, err := srv.Auth().UpsertGithubConnector(context.Background(), conn)
	require.NoError(t, err)
	require.NotNil(t, upserted)

	reqNormal := types.GithubAuthRequest{ConnectorID: conn.GetName(), Type: constants.Github}
	reqTest := types.GithubAuthRequest{ConnectorID: conn.GetName(), Type: constants.Github, SSOTestFlow: true, ConnectorSpec: &types.GithubConnectorSpecV3{
		ClientID:     "example-client-id",
		ClientSecret: "example-client-secret",
		RedirectURL:  "https://localhost:3080/v1/webapi/github/callback",
		Display:      "sign in with github",
		TeamsToLogins: []types.TeamMapping{
			{
				Organization: "octocats",
				Team:         "idp-admin",
				Logins:       []string{"access"},
			},
		},
	}}

	tests := []struct {
		desc               string
		roles              []string
		request            types.GithubAuthRequest
		expectAccessDenied bool
	}{
		{
			desc:               "empty role - no access",
			roles:              []string{emptyRole.GetName()},
			request:            reqNormal,
			expectAccessDenied: true,
		},
		{
			desc:               "can create regular request with normal access",
			roles:              []string{access1Role.GetName()},
			request:            reqNormal,
			expectAccessDenied: false,
		},
		{
			desc:               "cannot create sso test request with normal access",
			roles:              []string{access1Role.GetName()},
			request:            reqTest,
			expectAccessDenied: true,
		},
		{
			desc:               "cannot create normal request with connector access",
			roles:              []string{access2Role.GetName()},
			request:            reqNormal,
			expectAccessDenied: true,
		},
		{
			desc:               "cannot create sso test request with connector access",
			roles:              []string{access2Role.GetName()},
			request:            reqTest,
			expectAccessDenied: true,
		},
		{
			desc:               "can create regular request with combined access",
			roles:              []string{access3Role.GetName()},
			request:            reqNormal,
			expectAccessDenied: false,
		},
		{
			desc:               "can create sso test request with combined access",
			roles:              []string{access3Role.GetName()},
			request:            reqTest,
			expectAccessDenied: false,
		},
	}

	user, err := CreateUser(ctx, srv.Auth(), "dummy")
	require.NoError(t, err)

	userReader, err := CreateUser(ctx, srv.Auth(), "dummy-reader", readerRole)
	require.NoError(t, err)

	clientReader, err := srv.NewClient(TestUser(userReader.GetName()))
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			user.SetRoles(tt.roles)
			user, err = srv.Auth().UpsertUser(ctx, user)
			require.NoError(t, err)

			client, err := srv.NewClient(TestUser(user.GetName()))
			require.NoError(t, err)

			request, err := client.CreateGithubAuthRequest(ctx, tt.request)
			if tt.expectAccessDenied {
				require.Error(t, err)
				require.True(t, trace.IsAccessDenied(err), "expected access denied, got: %v", err)
				return
			}

			require.NoError(t, err)
			require.NotEmpty(t, request.StateToken)
			require.Equal(t, tt.request.ConnectorID, request.ConnectorID)

			requestCopy, err := clientReader.GetGithubAuthRequest(ctx, request.StateToken)
			require.NoError(t, err)
			require.Equal(t, request, requestCopy)
		})
	}
}

// TestGithubAuthCompat attempts to test github SSO authentication from the
// perspective of an Auth service receiving requests from a Proxy service. The
// Auth service on major version N should support proxies on version N and N-1,
// which may send a single user public key or split SSH and TLS public keys.
//
// The proxy originally sends the user public keys via gPRC
// CreateGithubAuthRequest, and later retrieves the request via HTTP
// github/requests/validate which must have the same format as the requested
// keys to support both old and new proxies.
func TestGithubAuthCompat(t *testing.T) {
	ctx := context.Background()
	srv := newTestTLSServer(t)

	connector, err := types.NewGithubConnector("example", types.GithubConnectorSpecV3{
		ClientID:     "example-client-id",
		ClientSecret: "example-client-secret",
		RedirectURL:  "https://localhost:3080/v1/webapi/github/callback",
		Display:      "sign in with github",
		TeamsToRoles: []types.TeamRolesMapping{{
			Organization: "octocats",
			Team:         "devs",
			Roles:        []string{"access"},
		}},
	})
	require.NoError(t, err)
	_, err = srv.Auth().UpsertGithubConnector(context.Background(), connector)
	require.NoError(t, err)

	srv.Auth().GithubUserAndTeamsOverride = func() (*GithubUserResponse, []GithubTeamResponse, error) {
		return &GithubUserResponse{
				Login: "alice",
			}, []GithubTeamResponse{{
				Name: "devs",
				Slug: "devs",
				Org:  GithubOrgResponse{Login: "octocats"},
			}}, nil
	}

	_, err = CreateRole(ctx, srv.Auth(), "access", types.RoleSpecV6{})
	require.NoError(t, err)

	proxyClient, err := srv.NewClient(TestBuiltin(types.RoleProxy))
	require.NoError(t, err)

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
		desc                         string
		pubKey, sshPubKey, tlsPubKey []byte
		expectSSHSubjectKey          ssh.PublicKey
		expectTLSSubjectKey          crypto.PublicKey
	}{
		{
			desc: "no keys",
		},
		{
			desc:                "single key",
			pubKey:              sshPubBytes,
			expectSSHSubjectKey: sshPub,
			expectTLSSubjectKey: sshKey.Public(),
		},
		{
			desc:                "split keys",
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
	} {
		t.Run(tc.desc, func(t *testing.T) {
			// Create the request over gRPC, this simulates to proxy creating the
			// initial request.
			req, err := proxyClient.CreateGithubAuthRequest(ctx, types.GithubAuthRequest{
				ConnectorID:  connector.GetName(),
				Type:         constants.Github,
				PublicKey:    tc.pubKey,
				SshPublicKey: tc.sshPubKey,
				TlsPublicKey: tc.tlsPubKey,
				CertTTL:      apidefaults.MinCertDuration,
			})
			require.NoError(t, err)

			// Simulate the proxy redirecting the user to github, getting a
			// response, and calling back to auth for validation.
			resp, err := proxyClient.ValidateGithubAuthCallback(ctx, url.Values{
				"code":  []string{"success"},
				"state": []string{req.StateToken},
			})
			require.NoError(t, err)

			// The proxy should get back the keys exactly as it sent them. Older
			// proxies won't look for the new split keys, and they do check for
			// the old single key to tell if this was a console or web request.
			require.Equal(t, tc.pubKey, resp.Req.PublicKey) //nolint:staticcheck // SA1019. Checking that deprecated field is set.
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
		})
	}
}

func TestSSODiagnosticInfo(t *testing.T) {
	ctx := context.Background()
	srv := newTestTLSServer(t)

	// empty role
	emptyRole, err := CreateRole(ctx, srv.Auth(), "test-empty", types.RoleSpecV6{})
	require.NoError(t, err)

	// privileged role
	privRole, err := CreateRole(ctx, srv.Auth(), "priv-access", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				{
					Resources: []string{types.KindSAMLRequest},
					Verbs:     []string{types.VerbRead},
				},
			},
		},
	})
	require.NoError(t, err)

	user, err := CreateUser(ctx, srv.Auth(), "dummy", emptyRole)
	require.NoError(t, err)

	userPriv, err := CreateUser(ctx, srv.Auth(), "superDummy", privRole)
	require.NoError(t, err)

	client, err := srv.NewClient(TestUser(user.GetName()))
	require.NoError(t, err)

	clientPriv, err := srv.NewClient(TestUser(userPriv.GetName()))
	require.NoError(t, err)

	// fresh server, no SSO diag info, request fails
	info, err := client.GetSSODiagnosticInfo(ctx, types.KindSAML, "XXX-INVALID-ID")
	require.Error(t, err)
	require.Nil(t, info)

	infoCreate := types.SSODiagnosticInfo{
		TestFlow: true,
		Error:    "aaa bbb ccc",
	}

	// invalid auth kind returns error, no information stored.
	err = srv.Auth().CreateSSODiagnosticInfo(ctx, "XXX-BAD-KIND", "ABC123", infoCreate)
	require.Error(t, err)
	info, err = client.GetSSODiagnosticInfo(ctx, "XXX-BAD-KIND", "XXX-INVALID-ID")
	require.Error(t, err)
	require.Nil(t, info)

	// proper record can be stored, retrieved, if user has access.
	err = srv.Auth().CreateSSODiagnosticInfo(ctx, types.KindSAML, "ABC123", infoCreate)
	require.NoError(t, err)

	info, err = client.GetSSODiagnosticInfo(ctx, types.KindSAML, "ABC123")
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))
	require.Nil(t, info)

	info, err = clientPriv.GetSSODiagnosticInfo(ctx, types.KindSAML, "ABC123")
	require.NoError(t, err)
	require.Equal(t, &infoCreate, info)
}

func TestGenerateUserCertsForHeadlessKube(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	srv := newTestTLSServer(t)

	const kubeClusterName = "kube-cluster-1"
	kubeCluster, err := types.NewKubernetesClusterV3(
		types.Metadata{
			Name: kubeClusterName,
		},
		types.KubernetesClusterSpecV3{},
	)
	require.NoError(t, err)

	kubeServer, err := types.NewKubernetesServerV3(
		types.Metadata{
			Name:   kubeClusterName,
			Labels: map[string]string{"name": kubeClusterName},
		},

		types.KubernetesServerSpecV3{
			HostID:   kubeClusterName,
			Hostname: "test",
			Cluster:  kubeCluster,
		},
	)
	require.NoError(t, err)

	_, err = srv.Auth().UpsertKubernetesServer(ctx, kubeServer)
	require.NoError(t, err)

	// Create test user1.
	user1, _, err := CreateUserAndRole(srv.Auth(), "user1", []string{"role1"}, nil)
	require.NoError(t, err)

	// Create test user2.
	user2, role2, err := CreateUserAndRole(srv.Auth(), "user2", []string{"role2"}, nil)
	require.NoError(t, err)

	role2Opts := role2.GetOptions()
	role2Opts.MaxSessionTTL = types.NewDuration(2 * time.Hour)
	role2.SetOptions(role2Opts)

	_, err = srv.Auth().UpsertRole(ctx, role2)
	require.NoError(t, err)

	user1, err = srv.Auth().UpdateUser(ctx, user1)
	require.NoError(t, err)

	user2, err = srv.Auth().UpdateUser(ctx, user2)
	require.NoError(t, err)
	authPrefs, err := srv.Auth().GetAuthPreference(ctx)
	require.NoError(t, err)

	_, sshPubKey, _, tlsPubKey := newSSHAndTLSKeyPairs(t)
	defaultDuration := authPrefs.GetDefaultSessionTTL().Duration()

	testCases := []struct {
		desc       string
		user       types.User
		expiration time.Time
	}{
		{
			desc:       "Roles don't have max_session_ttl set",
			user:       user1,
			expiration: srv.Auth().GetClock().Now().Add(defaultDuration),
		},
		{
			desc:       "Roles have max_session_ttl set, cert expiration adjusted",
			user:       user2,
			expiration: srv.Auth().GetClock().Now().Add(2 * time.Hour),
		},
	}

	for _, tt := range testCases {
		t.Run(tt.desc, func(t *testing.T) {
			user := TestUser(tt.user.GetName())
			user.TTL = defaultDuration
			client, err := srv.NewClient(user)
			require.NoError(t, err)

			certs, err := client.GenerateUserCerts(ctx, proto.UserCertsRequest{
				SSHPublicKey:      sshPubKey,
				TLSPublicKey:      tlsPubKey,
				Username:          tt.user.GetName(),
				Expires:           srv.Auth().GetClock().Now().Add(defaultDuration),
				KubernetesCluster: kubeClusterName,
				RequesterName:     proto.UserCertsRequest_TSH_KUBE_LOCAL_PROXY_HEADLESS,
				Usage:             proto.UserCertsRequest_Kubernetes,
			})
			require.NoError(t, err)

			// Parse the Identity
			tlsCert, err := tlsca.ParseCertificatePEM(certs.TLS)
			require.NoError(t, err)
			identity, err := tlsca.FromSubject(tlsCert.Subject, tlsCert.NotAfter)
			require.NoError(t, err)

			sshCert, err := sshutils.ParseCertificate(certs.SSH)
			require.NoError(t, err)

			require.WithinDuration(t, tt.expiration, identity.Expires, 10*time.Second,
				"Identity expiration is out of expected boundaries")
			require.WithinDuration(t, tt.expiration, time.Unix(int64(sshCert.ValidBefore), 0), 10*time.Second,
				"Identity expiration is out of expected boundaries")
		})
	}
}

func TestGenerateUserCertsWithMFAVerification(t *testing.T) {
	t.Parallel()

	const minVerificationDuration = 35 * time.Minute

	ctx := context.Background()
	srv := newTestTLSServer(t)

	const kubeClusterName = "kube-cluster-1"
	kubeCluster, err := types.NewKubernetesClusterV3(
		types.Metadata{
			Name: kubeClusterName,
		},
		types.KubernetesClusterSpecV3{},
	)
	require.NoError(t, err)

	kubeServer, err := types.NewKubernetesServerV3(
		types.Metadata{
			Name:   kubeClusterName,
			Labels: map[string]string{"name": kubeClusterName},
		},

		types.KubernetesServerSpecV3{
			HostID:   kubeClusterName,
			Hostname: "test",
			Cluster:  kubeCluster,
		},
	)
	require.NoError(t, err)

	_, err = srv.Auth().UpsertKubernetesServer(ctx, kubeServer)
	require.NoError(t, err)

	// Create test user1.
	user1, _, err := CreateUserAndRole(srv.Auth(), "user1", []string{"role1"}, nil)
	require.NoError(t, err)

	// Create test user2.
	user2, role2, err := CreateUserAndRole(srv.Auth(), "user2", []string{"role2"}, nil)
	require.NoError(t, err)

	role2Opts := role2.GetOptions()
	role2Opts.MFAVerificationInterval = minVerificationDuration
	role2Opts.RequireMFAType = types.RequireMFAType_SESSION
	role2.SetOptions(role2Opts)

	_, err = srv.Auth().UpsertRole(ctx, role2)
	require.NoError(t, err)

	// Create test user3.
	user3, role3, err := CreateUserAndRole(srv.Auth(), "user3", []string{"role3"}, nil)
	require.NoError(t, err)

	role3Opts := role3.GetOptions()
	role3Opts.MFAVerificationInterval = minVerificationDuration
	role3.SetOptions(role3Opts)

	_, err = srv.Auth().UpsertRole(ctx, role3)
	require.NoError(t, err)

	user1, err = srv.Auth().UpdateUser(ctx, user1)
	require.NoError(t, err)

	user2, err = srv.Auth().UpdateUser(ctx, user2)
	require.NoError(t, err)

	user3, err = srv.Auth().UpdateUser(ctx, user3)
	require.NoError(t, err)

	authPrefs, err := srv.Auth().GetAuthPreference(ctx)
	require.NoError(t, err)

	defaultDuration := authPrefs.GetDefaultSessionTTL().Duration()
	authClock := srv.Auth().GetClock()

	testCases := []struct {
		desc              string
		user              types.User
		expiration        time.Time
		clusterRequireMFA bool
		usage             proto.UserCertsRequest_CertUsage
		requester         proto.UserCertsRequest_Requester
	}{
		{
			desc:       "Roles don't have mfa_verification_interval set",
			user:       user1,
			expiration: authClock.Now().Add(defaultDuration),
			usage:      proto.UserCertsRequest_Kubernetes,
			requester:  proto.UserCertsRequest_TSH_KUBE_LOCAL_PROXY_HEADLESS,
		},
		{
			desc:       "Roles have mfa_verification_interval set, but MFA is not required",
			user:       user3,
			expiration: authClock.Now().Add(defaultDuration),
			usage:      proto.UserCertsRequest_Kubernetes,
			requester:  proto.UserCertsRequest_TSH_KUBE_LOCAL_PROXY_HEADLESS,
		},
		{
			desc:              "Roles have mfa_verification_interval set, MFA required only on cluster level",
			user:              user3,
			expiration:        authClock.Now().Add(minVerificationDuration),
			usage:             proto.UserCertsRequest_Kubernetes,
			requester:         proto.UserCertsRequest_TSH_KUBE_LOCAL_PROXY_HEADLESS,
			clusterRequireMFA: true,
		},
		{
			desc:       "Kube proxy, roles have mfa_verification_interval set, cert expiration adjusted",
			user:       user2,
			expiration: authClock.Now().Add(minVerificationDuration),
			usage:      proto.UserCertsRequest_Kubernetes,
			requester:  proto.UserCertsRequest_TSH_KUBE_LOCAL_PROXY,
		},
		{
			desc:       "Headless kube proxy, roles have mfa_verification_interval set, cert expiration adjusted",
			user:       user2,
			expiration: authClock.Now().Add(minVerificationDuration),
			usage:      proto.UserCertsRequest_Kubernetes,
			requester:  proto.UserCertsRequest_TSH_KUBE_LOCAL_PROXY_HEADLESS,
		},
		{
			desc:       "DB Proxy, roles have mfa_verification_interval set, cert expiration adjusted",
			user:       user2,
			expiration: authClock.Now().Add(minVerificationDuration),
			usage:      proto.UserCertsRequest_Database,
			requester:  proto.UserCertsRequest_TSH_DB_LOCAL_PROXY_TUNNEL,
		},
		{
			desc:       "App proxy, roles have mfa_verification_interval set, cert expiration adjusted",
			user:       user2,
			expiration: authClock.Now().Add(minVerificationDuration),
			usage:      proto.UserCertsRequest_App,
			requester:  proto.UserCertsRequest_TSH_APP_LOCAL_PROXY,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.desc, func(t *testing.T) {
			user := TestUser(tt.user.GetName())
			user.TTL = defaultDuration
			client, err := srv.NewClient(user)
			require.NoError(t, err)

			ap, ok := authPrefs.(*types.AuthPreferenceV2)
			require.True(t, ok)

			if tt.clusterRequireMFA {
				ap.Spec.RequireMFAType = types.RequireMFAType_SESSION
			} else {
				ap.Spec.RequireMFAType = types.RequireMFAType_OFF
			}

			_, err = srv.Auth().UpsertAuthPreference(ctx, ap)
			require.NoError(t, err)

			_, pub, err := testauthority.New().GenerateKeyPair()
			require.NoError(t, err)

			certs, err := client.GenerateUserCerts(ctx, proto.UserCertsRequest{
				PublicKey:         pub,
				Username:          tt.user.GetName(),
				Expires:           authClock.Now().Add(defaultDuration),
				KubernetesCluster: kubeClusterName,
				RouteToDatabase:   proto.RouteToDatabase{ServiceName: "test"},
				RouteToApp:        proto.RouteToApp{Name: "test"},
				RequesterName:     tt.requester,
				Usage:             tt.usage,
			})
			require.NoError(t, err)

			// Parse the Identity
			tlsCert, err := tlsca.ParseCertificatePEM(certs.TLS)
			require.NoError(t, err)
			identity, err := tlsca.FromSubject(tlsCert.Subject, tlsCert.NotAfter)
			require.NoError(t, err)
			require.Less(t, tt.expiration.Sub(identity.Expires).Abs(), 10*time.Second,
				"Identity expiration is out of expected boundaries")
		})
	}
}

func TestGenerateUserCertsWithRoleRequest(t *testing.T) {
	ctx := context.Background()
	srv := newTestTLSServer(t)

	emptyRole, err := CreateRole(ctx, srv.Auth(), "test-empty", types.RoleSpecV6{})
	require.NoError(t, err)

	accessFooRole, err := CreateRole(ctx, srv.Auth(), "test-access-foo", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Logins: []string{"foo"},
		},
	})
	require.NoError(t, err)

	accessBarRole, err := CreateRole(ctx, srv.Auth(), "test-access-bar", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Logins: []string{"bar"},
		},
	})
	require.NoError(t, err)

	loginsTraitsRole, err := CreateRole(ctx, srv.Auth(), "test-access-traits", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Logins: []string{"{{internal.logins}}"},
		},
	})
	require.NoError(t, err)

	impersonatorRole, err := CreateRole(ctx, srv.Auth(), "test-impersonator", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Impersonate: &types.ImpersonateConditions{
				Roles: []string{
					accessFooRole.GetName(),
					accessBarRole.GetName(),
					loginsTraitsRole.GetName(),
				},
			},
		},
	})
	require.NoError(t, err)

	denyBarRole, err := CreateRole(ctx, srv.Auth(), "test-deny", types.RoleSpecV6{
		Deny: types.RoleConditions{
			Impersonate: &types.ImpersonateConditions{
				Roles: []string{accessBarRole.GetName()},
			},
		},
	})
	require.NoError(t, err)

	dummyUserRole, err := types.NewRole("dummy-user-role", types.RoleSpecV6{})
	require.NoError(t, err)

	dummyUser, err := CreateUser(ctx, srv.Auth(), "dummy-user", dummyUserRole)
	require.NoError(t, err)

	dummyUserImpersonatorRole, err := CreateRole(ctx, srv.Auth(), "dummy-user-impersonator", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Impersonate: &types.ImpersonateConditions{
				Users: []string{dummyUser.GetName()},
				Roles: []string{dummyUserRole.GetName()},
			},
		},
	})
	require.NoError(t, err)

	tests := []struct {
		desc             string
		username         string
		userTraits       wrappers.Traits
		roles            []string
		roleRequests     []string
		useRoleRequests  bool
		expectPrincipals []string
		expectRoles      []string
		expectError      func(error) bool
	}{
		{
			desc:             "requesting all allowed roles",
			username:         "alice",
			roles:            []string{emptyRole.GetName(), impersonatorRole.GetName()},
			roleRequests:     []string{accessFooRole.GetName(), accessBarRole.GetName()},
			useRoleRequests:  true,
			expectPrincipals: []string{"foo", "bar"},
		},
		{
			desc:     "requesting a subset of allowed roles",
			username: "bob",
			userTraits: wrappers.Traits{
				// We don't expect this login trait to appear in the principals
				// as "test-access-foo" does not contain {{internal.logins}}
				constants.TraitLogins: []string{"trait-login"},
			},
			roles:            []string{emptyRole.GetName(), impersonatorRole.GetName()},
			roleRequests:     []string{accessFooRole.GetName()},
			useRoleRequests:  true,
			expectPrincipals: []string{"foo"},
		},
		{
			// Users traits should be preserved in role impersonation
			desc:     "requesting a role preserves user traits",
			username: "ash",
			userTraits: wrappers.Traits{
				constants.TraitLogins: []string{"trait-login"},
			},
			roles: []string{
				emptyRole.GetName(),
				impersonatorRole.GetName(),
			},
			roleRequests:     []string{loginsTraitsRole.GetName()},
			useRoleRequests:  true,
			expectPrincipals: []string{"trait-login"},
		},
		{
			// Users not using role requests should keep their own roles
			desc:            "requesting no roles",
			username:        "charlie",
			roles:           []string{emptyRole.GetName()},
			roleRequests:    []string{},
			useRoleRequests: false,
			expectRoles:     []string{emptyRole.GetName()},
		},
		{
			// An empty role request should fail when role requests are
			// expected.
			desc:            "requesting no roles with UseRoleRequests",
			username:        "charlie",
			roles:           []string{emptyRole.GetName()},
			roleRequests:    []string{},
			useRoleRequests: true,
			expectError: func(err error) bool {
				return trace.IsBadParameter(err)
			},
		},
		{
			desc:            "requesting a disallowed role",
			username:        "dave",
			roles:           []string{emptyRole.GetName()},
			roleRequests:    []string{accessFooRole.GetName()},
			useRoleRequests: true,
			expectError: func(err error) bool {
				return err != nil && trace.IsAccessDenied(err)
			},
		},
		{
			desc:            "requesting a nonexistent role",
			username:        "erin",
			roles:           []string{emptyRole.GetName()},
			roleRequests:    []string{"doesnotexist"},
			useRoleRequests: true,
			expectError: func(err error) bool {
				return err != nil && trace.IsNotFound(err)
			},
		},
		{
			desc:             "requesting an allowed role with a separate deny role",
			username:         "frank",
			roles:            []string{emptyRole.GetName(), impersonatorRole.GetName(), denyBarRole.GetName()},
			roleRequests:     []string{accessFooRole.GetName()},
			useRoleRequests:  true,
			expectPrincipals: []string{"foo"},
		},
		{
			desc:            "requesting a denied role",
			username:        "geoff",
			roles:           []string{emptyRole.GetName(), impersonatorRole.GetName(), denyBarRole.GetName()},
			roleRequests:    []string{accessBarRole.GetName()},
			useRoleRequests: true,
			expectError: func(err error) bool {
				return err != nil && trace.IsAccessDenied(err)
			},
		},
		{
			desc:            "misusing a role intended for user impersonation",
			username:        "helen",
			roles:           []string{emptyRole.GetName(), dummyUserImpersonatorRole.GetName()},
			roleRequests:    []string{dummyUserRole.GetName()},
			useRoleRequests: true,
			expectError: func(err error) bool {
				return err != nil && trace.IsAccessDenied(err)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			user, err := CreateUser(ctx, srv.Auth(), tt.username)
			require.NoError(t, err)
			t.Cleanup(func() {
				require.NoError(t, srv.Auth().DeleteUser(ctx, tt.username), "failed cleaning up testing user: %s", tt.username)
			})
			for _, role := range tt.roles {
				user.AddRole(role)
			}
			if tt.userTraits != nil {
				user.SetTraits(tt.userTraits)
			}
			user, err = srv.Auth().UpsertUser(ctx, user)
			require.NoError(t, err)

			client, err := srv.NewClient(TestUser(user.GetName()))
			require.NoError(t, err)

			_, sshPubKey, _, tlsPubKey := newSSHAndTLSKeyPairs(t)
			certs, err := client.GenerateUserCerts(ctx, proto.UserCertsRequest{
				SSHPublicKey:    sshPubKey,
				TLSPublicKey:    tlsPubKey,
				Username:        user.GetName(),
				Expires:         time.Now().Add(time.Hour),
				RoleRequests:    tt.roleRequests,
				UseRoleRequests: tt.useRoleRequests,
			})
			if tt.expectError != nil {
				require.True(t, tt.expectError(err), "error: %+v: %s", err, trace.DebugReport(err))
				return
			}
			require.NoError(t, err)

			// Parse the Identity
			impersonatedTLSCert, err := tlsca.ParseCertificatePEM(certs.TLS)
			require.NoError(t, err)
			impersonatedIdent, err := tlsca.FromSubject(impersonatedTLSCert.Subject, impersonatedTLSCert.NotAfter)
			require.NoError(t, err)

			userCert, err := sshutils.ParseCertificate(certs.SSH)
			require.NoError(t, err)

			userIdent, err := sshca.DecodeIdentity(userCert)
			require.NoError(t, err)

			if len(tt.expectPrincipals) > 0 {
				expectPrincipals := append(tt.expectPrincipals, teleport.SSHSessionJoinPrincipal)
				require.ElementsMatch(t, expectPrincipals, userIdent.Principals, "principals must match")
			}

			if tt.expectRoles != nil {
				require.ElementsMatch(t, tt.expectRoles, userIdent.Roles, "granted roles must match expected values")
			} else {
				require.ElementsMatch(t, tt.roleRequests, userIdent.Roles, "granted roles must match requests")
			}

			if len(tt.roleRequests) > 0 {
				require.NotEmpty(t, userIdent.Impersonator, "impersonator must be set if any role requests exist")
				require.Equal(t, tt.username, userIdent.Impersonator, "certificate must show self-impersonation")

				require.True(t, userIdent.DisallowReissue)
				require.True(t, impersonatedIdent.DisallowReissue)
			} else {
				require.False(t, userIdent.DisallowReissue)
				require.False(t, impersonatedIdent.DisallowReissue)
			}
		})
	}
}

// TestRoleRequestDenyReimpersonation make sure role requests can't be used to
// re-escalate privileges using a (perhaps compromised) set of role
// impersonated certs.
func TestRoleRequestDenyReimpersonation(t *testing.T) {
	ctx := context.Background()
	srv := newTestTLSServer(t)

	accessFooRole, err := CreateRole(ctx, srv.Auth(), "test-access-foo", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Logins: []string{"foo"},
		},
	})
	require.NoError(t, err)

	accessBarRole, err := CreateRole(ctx, srv.Auth(), "test-access-bar", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Logins: []string{"bar"},
		},
	})
	require.NoError(t, err)

	impersonatorRole, err := CreateRole(ctx, srv.Auth(), "test-impersonator", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Impersonate: &types.ImpersonateConditions{
				Roles: []string{accessFooRole.GetName(), accessBarRole.GetName()},
			},
		},
	})
	require.NoError(t, err)

	// Create a testing user.
	user, err := CreateUser(ctx, srv.Auth(), "alice")
	require.NoError(t, err)
	user.AddRole(impersonatorRole.GetName())
	user, err = srv.Auth().UpsertUser(ctx, user)
	require.NoError(t, err)

	// Generate cert with a role request.
	client, err := srv.NewClient(TestUser(user.GetName()))
	require.NoError(t, err)

	_, sshPubKey, tlsPrivKey, tlsPubKey := newSSHAndTLSKeyPairs(t)

	// Request certs for only the `foo` role.
	certs, err := client.GenerateUserCerts(ctx, proto.UserCertsRequest{
		SSHPublicKey: sshPubKey,
		TLSPublicKey: tlsPubKey,
		Username:     user.GetName(),
		Expires:      time.Now().Add(time.Hour),
		RoleRequests: []string{accessFooRole.GetName()},
	})
	require.NoError(t, err)

	// Make an impersonated client.
	impersonatedTLSCert, err := tls.X509KeyPair(certs.TLS, tlsPrivKey)
	require.NoError(t, err)
	impersonatedClient := srv.NewClientWithCert(impersonatedTLSCert)

	// Attempt a request.
	_, err = impersonatedClient.GetClusterName()
	require.NoError(t, err)

	// Attempt to generate new certs for a different (allowed) role.
	_, err = impersonatedClient.GenerateUserCerts(ctx, proto.UserCertsRequest{
		SSHPublicKey: sshPubKey,
		TLSPublicKey: tlsPubKey,
		Username:     user.GetName(),
		Expires:      time.Now().Add(time.Hour),
		RoleRequests: []string{accessBarRole.GetName()},
	})
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))

	// Attempt to generate new certs for the same role.
	_, err = impersonatedClient.GenerateUserCerts(ctx, proto.UserCertsRequest{
		SSHPublicKey: sshPubKey,
		TLSPublicKey: tlsPubKey,
		Username:     user.GetName(),
		Expires:      time.Now().Add(time.Hour),
		RoleRequests: []string{accessFooRole.GetName()},
	})
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))

	// Attempt to generate new certs with no role requests
	// (If allowed, this might issue certs for the original user without role
	// requests.)
	_, err = impersonatedClient.GenerateUserCerts(ctx, proto.UserCertsRequest{
		SSHPublicKey: sshPubKey,
		TLSPublicKey: tlsPubKey,
		Username:     user.GetName(),
		Expires:      time.Now().Add(time.Hour),
	})
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))
}

// TestGenerateDatabaseCert makes sure users and services with appropriate
// permissions can generate database certificates.
func TestGenerateDatabaseCert(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	// This user can't impersonate anyone and can't generate database certs.
	userWithoutAccess, _, err := CreateUserAndRole(srv.Auth(), "user", []string{"role1"}, nil)
	require.NoError(t, err)

	// This user can impersonate system role Db.
	userImpersonateDb, roleDb, err := CreateUserAndRole(srv.Auth(), "user-impersonate-db", []string{"role2"}, nil)
	require.NoError(t, err)
	roleDb.SetImpersonateConditions(types.Allow, types.ImpersonateConditions{
		Users: []string{string(types.RoleDatabase)},
		Roles: []string{string(types.RoleDatabase)},
	})
	_, err = srv.Auth().UpsertRole(ctx, roleDb)
	require.NoError(t, err)

	tests := []struct {
		desc      string
		identity  TestIdentity
		requester proto.DatabaseCertRequest_Requester
		err       string
	}{
		{
			desc:      "user can't sign database certs",
			identity:  TestUser(userWithoutAccess.GetName()),
			requester: proto.DatabaseCertRequest_TCTL,
			err:       "access denied",
		},
		{
			desc:      "user can impersonate Db and sign database certs",
			identity:  TestUser(userImpersonateDb.GetName()),
			requester: proto.DatabaseCertRequest_TCTL,
		},
		{
			desc:      "built-in admin can sign database certs",
			identity:  TestAdmin(),
			requester: proto.DatabaseCertRequest_TCTL,
		},
		{
			desc:     "database service can sign database certs",
			identity: TestBuiltin(types.RoleDatabase),
		},
	}

	// Generate CSR once for speed sake.
	priv, err := cryptosuites.GeneratePrivateKeyWithAlgorithm(cryptosuites.RSA2048)
	require.NoError(t, err)

	csr, err := tlsca.GenerateCertificateRequestPEM(pkix.Name{CommonName: "test"}, priv)
	require.NoError(t, err)

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			client, err := srv.NewClient(test.identity)
			require.NoError(t, err)

			_, err = client.GenerateDatabaseCert(ctx, &proto.DatabaseCertRequest{CSR: csr, RequesterName: test.requester})
			if test.err != "" {
				require.ErrorContains(t, err, test.err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

type testDynamicallyConfigurableRBACParams struct {
	kind                          string
	storeDefault, storeConfigFile func(*Server)
	get, set, reset               func(*ServerWithRoles) error
	alwaysReadable                bool
}

// TestDynamicConfigurationRBACVerbs tests the dynamic configuration RBAC verbs described
// in rfd/0016-dynamic-configuration.md § Implementation.
func testDynamicallyConfigurableRBAC(t *testing.T, p testDynamicallyConfigurableRBACParams) {
	testAuth, err := NewTestAuthServer(TestAuthServerConfig{Dir: t.TempDir()})
	require.NoError(t, err)

	testOperation := func(op func(*ServerWithRoles) error, allowRules []types.Rule, expectErr, withConfigFile bool) func(*testing.T) {
		return func(t *testing.T) {
			if withConfigFile {
				p.storeConfigFile(testAuth.AuthServer)
			} else {
				p.storeDefault(testAuth.AuthServer)
			}
			server := serverWithAllowRules(t, testAuth, allowRules)
			opErr := op(server)
			if expectErr {
				require.Error(t, opErr)
			} else {
				require.NoError(t, opErr)
			}
		}
	}

	// runTestCases generates all non-empty RBAC verb combinations and checks the expected
	// error for each operation.
	runTestCases := func(withConfigFile bool) {
		for _, canCreate := range []bool{false, true} {
			for _, canUpdate := range []bool{false, true} {
				for _, canRead := range []bool{false, true} {
					if !canRead && !canUpdate && !canCreate {
						continue
					}
					verbs := []string{}
					expectGetErr, expectSetErr, expectResetErr := true, true, true
					if canRead || p.alwaysReadable {
						verbs = append(verbs, types.VerbRead)
						expectGetErr = false
					}
					if canUpdate {
						verbs = append(verbs, types.VerbUpdate)
						if !withConfigFile {
							expectSetErr, expectResetErr = false, false
						}
					}
					if canCreate {
						verbs = append(verbs, types.VerbCreate)
						if canUpdate {
							expectSetErr = false
						}
					}
					allowRules := []types.Rule{
						{
							Resources: []string{p.kind},
							Verbs:     verbs,
						},
					}
					t.Run(fmt.Sprintf("get %v %v", verbs, withConfigFile), testOperation(p.get, allowRules, expectGetErr, withConfigFile))
					t.Run(fmt.Sprintf("set %v %v", verbs, withConfigFile), testOperation(p.set, allowRules, expectSetErr, withConfigFile))
					t.Run(fmt.Sprintf("reset %v %v", verbs, withConfigFile), testOperation(p.reset, allowRules, expectResetErr, withConfigFile))
				}
			}
		}
	}

	runTestCases(false)
	runTestCases(true)
}

func TestAuthPreferenceRBAC(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	testDynamicallyConfigurableRBAC(t, testDynamicallyConfigurableRBACParams{
		kind: types.KindClusterAuthPreference,
		storeDefault: func(s *Server) {
			s.UpsertAuthPreference(ctx, types.DefaultAuthPreference())
		},
		storeConfigFile: func(s *Server) {
			authPref := types.DefaultAuthPreference()
			authPref.SetOrigin(types.OriginConfigFile)
			s.UpsertAuthPreference(ctx, authPref)
		},
		get: func(s *ServerWithRoles) error {
			_, err := s.GetAuthPreference(ctx)
			return err
		},
		set: func(s *ServerWithRoles) error {
			return s.SetAuthPreference(ctx, types.DefaultAuthPreference())
		},
		reset: func(s *ServerWithRoles) error {
			return s.ResetAuthPreference(ctx)
		},
		alwaysReadable: true,
	})
}

func TestClusterNetworkingCloudUpdates(t *testing.T) {
	srv := newTestTLSServer(t)
	ctx := context.Background()
	_, err := srv.Auth().UpsertClusterNetworkingConfig(ctx, types.DefaultClusterNetworkingConfig())
	require.NoError(t, err)

	user, _, err := CreateUserAndRole(srv.Auth(), "username", []string{}, []types.Rule{
		{
			Resources: []string{
				types.KindClusterNetworkingConfig,
			},
			Verbs: services.RW(),
		},
	})
	require.NoError(t, err)

	for _, tc := range []struct {
		cloud                   bool
		identity                TestIdentity
		expectSetErr            string
		clusterNetworkingConfig types.ClusterNetworkingConfig
		name                    string
	}{
		{
			name:                    "non admin user can set existing values to the same value",
			cloud:                   true,
			identity:                TestUser(user.GetName()),
			clusterNetworkingConfig: types.DefaultClusterNetworkingConfig(),
		},
		{
			name:         "non admin user cannot set keep_alive_interval",
			cloud:        true,
			identity:     TestUser(user.GetName()),
			expectSetErr: "keep_alive_interval",
			clusterNetworkingConfig: newClusterNetworkingConf(t, types.ClusterNetworkingConfigSpecV2{
				KeepAliveInterval: types.Duration(time.Second * 20),
			}),
		},
		{
			name:         "non admin user cannot set tunnel_strategy",
			cloud:        true,
			identity:     TestUser(user.GetName()),
			expectSetErr: "tunnel_strategy",
			clusterNetworkingConfig: newClusterNetworkingConf(t, types.ClusterNetworkingConfigSpecV2{
				TunnelStrategy: &types.TunnelStrategyV1{
					Strategy: &types.TunnelStrategyV1_ProxyPeering{
						ProxyPeering: types.DefaultProxyPeeringTunnelStrategy(),
					},
				},
			}),
		},
		{
			name:         "non admin user cannot set proxy_listener_mode",
			cloud:        true,
			identity:     TestUser(user.GetName()),
			expectSetErr: "proxy_listener_mode",
			clusterNetworkingConfig: newClusterNetworkingConf(t, types.ClusterNetworkingConfigSpecV2{
				ProxyListenerMode: types.ProxyListenerMode_Multiplex,
			}),
		},
		{
			name:         "non admin user cannot set keep_alive_count_max",
			cloud:        true,
			identity:     TestUser(user.GetName()),
			expectSetErr: "keep_alive_count_max",
			clusterNetworkingConfig: newClusterNetworkingConf(t, types.ClusterNetworkingConfigSpecV2{
				KeepAliveCountMax: 55,
			}),
		},
		{
			name:     "non admin user can set client_idle_timeout",
			cloud:    true,
			identity: TestUser(user.GetName()),
			clusterNetworkingConfig: newClusterNetworkingConf(t, types.ClusterNetworkingConfigSpecV2{
				ClientIdleTimeout: types.Duration(time.Second * 67),
			}),
		},
		{
			name:     "admin user can set keep_alive_interval",
			cloud:    true,
			identity: TestAdmin(),
			clusterNetworkingConfig: newClusterNetworkingConf(t, types.ClusterNetworkingConfigSpecV2{
				KeepAliveInterval: types.Duration(time.Second * 67),
			}),
		},
		{
			name:     "non admin user can set keep_alive_interval on non cloud cluster",
			cloud:    false,
			identity: TestUser(user.GetName()),
			clusterNetworkingConfig: newClusterNetworkingConf(t, types.ClusterNetworkingConfigSpecV2{
				KeepAliveInterval: types.Duration(time.Second * 67),
			}),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			modules.SetTestModules(t, &modules.TestModules{
				TestBuildType: modules.BuildEnterprise,
				TestFeatures: modules.Features{
					Cloud: tc.cloud,
				},
			})

			client, err := srv.NewClient(tc.identity)
			require.NoError(t, err)

			err = client.SetClusterNetworkingConfig(ctx, tc.clusterNetworkingConfig.(*types.ClusterNetworkingConfigV2))
			if tc.expectSetErr != "" {
				assert.ErrorContains(t, err, tc.expectSetErr)
			} else {
				assert.NoError(t, err)
			}

			_, err = client.UpsertClusterNetworkingConfig(ctx, tc.clusterNetworkingConfig)
			if tc.expectSetErr != "" {
				assert.ErrorContains(t, err, tc.expectSetErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func newClusterNetworkingConf(t *testing.T, spec types.ClusterNetworkingConfigSpecV2) *types.ClusterNetworkingConfigV2 {
	c := &types.ClusterNetworkingConfigV2{
		Metadata: types.Metadata{
			Labels: map[string]string{
				types.OriginLabel: types.OriginDynamic,
			},
		},
		Spec: spec,
	}
	err := c.CheckAndSetDefaults()
	require.NoError(t, err)
	return c
}

func TestClusterNetworkingConfigRBAC(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	testDynamicallyConfigurableRBAC(t, testDynamicallyConfigurableRBACParams{
		kind: types.KindClusterNetworkingConfig,
		storeDefault: func(s *Server) {
			s.UpsertClusterNetworkingConfig(ctx, types.DefaultClusterNetworkingConfig())
		},
		storeConfigFile: func(s *Server) {
			netConfig := types.DefaultClusterNetworkingConfig()
			netConfig.SetOrigin(types.OriginConfigFile)
			s.UpsertClusterNetworkingConfig(ctx, netConfig)
		},
		get: func(s *ServerWithRoles) error {
			_, err := s.GetClusterNetworkingConfig(ctx)
			return err
		},
		set: func(s *ServerWithRoles) error {
			return s.SetClusterNetworkingConfig(ctx, types.DefaultClusterNetworkingConfig())
		},
		reset: func(s *ServerWithRoles) error {
			return s.ResetClusterNetworkingConfig(ctx)
		},
	})
}

func TestSessionRecordingConfigRBAC(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	testDynamicallyConfigurableRBAC(t, testDynamicallyConfigurableRBACParams{
		kind: types.KindSessionRecordingConfig,
		storeDefault: func(s *Server) {
			s.UpsertSessionRecordingConfig(ctx, types.DefaultSessionRecordingConfig())
		},
		storeConfigFile: func(s *Server) {
			recConfig := types.DefaultSessionRecordingConfig()
			recConfig.SetOrigin(types.OriginConfigFile)
			s.UpsertSessionRecordingConfig(ctx, recConfig)
		},
		get: func(s *ServerWithRoles) error {
			_, err := s.GetSessionRecordingConfig(ctx)
			return err
		},
		set: func(s *ServerWithRoles) error {
			return s.SetSessionRecordingConfig(ctx, types.DefaultSessionRecordingConfig())
		},
		reset: func(s *ServerWithRoles) error {
			return s.ResetSessionRecordingConfig(ctx)
		},
	})
}

// go test ./lib/auth -bench=. -run=^$ -v -benchtime 1x
// goos: darwin
// goarch: amd64
// pkg: github.com/gravitational/teleport/lib/auth
// cpu: Intel(R) Core(TM) i9-9880H CPU @ 2.30GHz
// BenchmarkListNodes
// BenchmarkListNodes/simple_labels
// BenchmarkListNodes/simple_labels-16                    1        1079886286 ns/op        525128104 B/op   8831939 allocs/op
// BenchmarkListNodes/simple_expression
// BenchmarkListNodes/simple_expression-16                1         770118479 ns/op        432667432 B/op   6514790 allocs/op
// BenchmarkListNodes/labels
// BenchmarkListNodes/labels-16                           1        1931843502 ns/op        741444360 B/op  15159333 allocs/op
// BenchmarkListNodes/expression
// BenchmarkListNodes/expression-16                       1        1040855282 ns/op        509643128 B/op   8120970 allocs/op
// BenchmarkListNodes/complex_labels
// BenchmarkListNodes/complex_labels-16                   1        2274376396 ns/op        792948904 B/op  17084107 allocs/op
// BenchmarkListNodes/complex_expression
// BenchmarkListNodes/complex_expression-16               1        1518800599 ns/op        738532920 B/op  12483748 allocs/op
// PASS
// ok      github.com/gravitational/teleport/lib/auth      11.679s
func BenchmarkListNodes(b *testing.B) {
	const nodeCount = 50_000
	const roleCount = 32

	ctx := context.Background()
	srv := newTestTLSServer(b)

	var ids []string
	for i := 0; i < roleCount; i++ {
		ids = append(ids, uuid.New().String())
	}

	ids[0] = "hidden"

	var hiddenNodes int
	// Create test nodes.
	for i := 0; i < nodeCount; i++ {
		name := uuid.New().String()
		id := ids[i%len(ids)]
		if id == "hidden" {
			hiddenNodes++
		}
		node, err := types.NewServerWithLabels(
			name,
			types.KindNode,
			types.ServerSpecV2{},
			map[string]string{
				"key":   id,
				"group": "users",
			},
		)
		require.NoError(b, err)

		_, err = srv.Auth().UpsertNode(ctx, node)
		require.NoError(b, err)
	}
	testNodes, err := srv.Auth().GetNodes(ctx, apidefaults.Namespace)
	require.NoError(b, err)
	require.Len(b, testNodes, nodeCount)

	for _, tc := range []struct {
		desc     string
		editRole func(types.Role, string)
	}{
		{
			desc: "simple labels",
			editRole: func(r types.Role, id string) {
				if id == "hidden" {
					r.SetNodeLabels(types.Deny, types.Labels{"key": {id}})
				} else {
					r.SetNodeLabels(types.Allow, types.Labels{"key": {id}})
				}
			},
		},
		{
			desc: "simple expression",
			editRole: func(r types.Role, id string) {
				if id == "hidden" {
					err = r.SetLabelMatchers(types.Deny, types.KindNode, types.LabelMatchers{
						Expression: `labels.key == "hidden"`,
					})
					require.NoError(b, err)
				} else {
					err := r.SetLabelMatchers(types.Allow, types.KindNode, types.LabelMatchers{
						Expression: fmt.Sprintf(`labels.key == %q`, id),
					})
					require.NoError(b, err)
				}
			},
		},
		{
			desc: "labels",
			editRole: func(r types.Role, id string) {
				r.SetNodeLabels(types.Allow, types.Labels{
					"key":   {id},
					"group": {"{{external.group}}"},
				})
				r.SetNodeLabels(types.Deny, types.Labels{"key": {"hidden"}})
			},
		},
		{
			desc: "expression",
			editRole: func(r types.Role, id string) {
				err := r.SetLabelMatchers(types.Allow, types.KindNode, types.LabelMatchers{
					Expression: fmt.Sprintf(`labels.key == %q && contains(user.spec.traits["group"], labels["group"])`,
						id),
				})
				require.NoError(b, err)
				err = r.SetLabelMatchers(types.Deny, types.KindNode, types.LabelMatchers{
					Expression: `labels.key == "hidden"`,
				})
				require.NoError(b, err)
			},
		},
		{
			desc: "complex labels",
			editRole: func(r types.Role, id string) {
				r.SetNodeLabels(types.Allow, types.Labels{
					"key": {"other", id, "another"},
					"group": {
						`{{regexp.replace(external.group, "^(.*)$", "$1")}}`,
						"{{email.local(external.email)}}",
					},
				})
				r.SetNodeLabels(types.Deny, types.Labels{"key": {"hidden"}})
			},
		},
		{
			desc: "complex expression",
			editRole: func(r types.Role, id string) {
				expr := fmt.Sprintf(
					`(labels.key == "other" || labels.key == %q || labels.key == "another") &&
					 (contains(email.local(user.spec.traits["email"]), labels["group"]) ||
						 contains(regexp.replace(user.spec.traits["group"], "^(.*)$", "$1"), labels["group"]))`,
					id)
				err := r.SetLabelMatchers(types.Allow, types.KindNode, types.LabelMatchers{
					Expression: expr,
				})
				require.NoError(b, err)
				err = r.SetLabelMatchers(types.Deny, types.KindNode, types.LabelMatchers{
					Expression: `labels.key == "hidden"`,
				})
				require.NoError(b, err)
			},
		},
	} {
		b.Run(tc.desc, func(b *testing.B) {
			benchmarkListNodes(
				b, ctx,
				nodeCount, hiddenNodes,
				srv,
				ids,
				tc.editRole,
			)
		})
	}
}

func benchmarkListNodes(
	b *testing.B, ctx context.Context,
	nodeCount, hiddenNodes int,
	srv *TestTLSServer,
	ids []string,
	editRole func(r types.Role, id string),
) {
	var roles []types.Role
	for _, id := range ids {
		role, err := types.NewRole(fmt.Sprintf("role-%s", id), types.RoleSpecV6{})
		require.NoError(b, err)
		editRole(role, id)
		roles = append(roles, role)
	}

	// create user, role, and client
	username := "user"

	user, err := CreateUser(ctx, srv.Auth(), username, roles...)
	require.NoError(b, err)
	user.SetTraits(map[string][]string{
		"group": {"users"},
		"email": {"test@example.com"},
	})
	user, err = srv.Auth().UpsertUser(ctx, user)
	require.NoError(b, err)
	identity := TestUser(user.GetName())
	clt, err := srv.NewClient(identity)
	require.NoError(b, err)

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		var resources []types.ResourceWithLabels
		req := proto.ListResourcesRequest{
			ResourceType: types.KindNode,
			Namespace:    apidefaults.Namespace,
			Limit:        1_000,
		}
		for {
			rsp, err := clt.ListResources(ctx, req)
			require.NoError(b, err)

			resources = append(resources, rsp.Resources...)
			req.StartKey = rsp.NextKey
			if req.StartKey == "" {
				break
			}
		}
		require.Len(b, resources, nodeCount-hiddenNodes)
	}
}

// TestGetAndList_Nodes users can retrieve nodes with various filters
// and with the appropriate permissions.
func TestGetAndList_Nodes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	// Create test nodes.
	for i := 0; i < 10; i++ {
		name := uuid.New().String()
		node, err := types.NewServerWithLabels(
			name,
			types.KindNode,
			types.ServerSpecV2{},
			map[string]string{"name": name},
		)
		require.NoError(t, err)

		_, err = srv.Auth().UpsertNode(ctx, node)
		require.NoError(t, err)
	}

	testNodes, err := srv.Auth().GetNodes(ctx, apidefaults.Namespace)
	require.NoError(t, err)

	// create user, role, and client
	username := "user"
	user, role, err := CreateUserAndRole(srv.Auth(), username, nil, nil)
	require.NoError(t, err)
	identity := TestUser(user.GetName())
	clt, err := srv.NewClient(identity)
	require.NoError(t, err)

	// permit user to list all nodes
	role.SetNodeLabels(types.Allow, types.Labels{types.Wildcard: {types.Wildcard}})
	_, err = srv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)

	// Convert nodes retrieved earlier as types.ResourcesWithLabels
	testResources := make([]types.ResourceWithLabels, len(testNodes))
	for i, node := range testNodes {
		testResources[i] = node
	}

	// listing nodes 0-4 should list first 5 nodes
	resp, err := clt.ListResources(ctx, proto.ListResourcesRequest{
		ResourceType: types.KindNode,
		Namespace:    apidefaults.Namespace,
		Limit:        5,
	})
	require.NoError(t, err)
	require.Len(t, resp.Resources, 5)
	expectedNodes := testResources[:5]
	require.Empty(t, cmp.Diff(expectedNodes, resp.Resources))

	// remove permission for third node
	role.SetNodeLabels(types.Deny, types.Labels{"name": {testResources[3].GetName()}})
	_, err = srv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)

	// listing nodes 0-4 should skip the third node and add the fifth to the end.
	resp, err = clt.ListResources(ctx, proto.ListResourcesRequest{
		ResourceType: types.KindNode,
		Namespace:    apidefaults.Namespace,
		Limit:        5,
	})
	require.NoError(t, err)
	require.Len(t, resp.Resources, 5)
	expectedNodes = append(testResources[:3], testResources[4:6]...)
	require.Empty(t, cmp.Diff(expectedNodes, resp.Resources))

	// Test various filtering.
	baseRequest := proto.ListResourcesRequest{
		ResourceType: types.KindNode,
		Namespace:    apidefaults.Namespace,
		Limit:        int32(len(testResources) + 1),
	}

	// Test label match.
	withLabels := baseRequest
	withLabels.Labels = map[string]string{"name": testResources[0].GetName()}
	resp, err = clt.ListResources(ctx, withLabels)
	require.NoError(t, err)
	require.Len(t, resp.Resources, 1)
	require.Empty(t, cmp.Diff(testResources[0:1], resp.Resources))

	// Test search keywords match.
	withSearchKeywords := baseRequest
	withSearchKeywords.SearchKeywords = []string{"name", testResources[0].GetName()}
	resp, err = clt.ListResources(ctx, withSearchKeywords)
	require.NoError(t, err)
	require.Len(t, resp.Resources, 1)
	require.Empty(t, cmp.Diff(testResources[0:1], resp.Resources))

	// Test expression match.
	withExpression := baseRequest
	withExpression.PredicateExpression = fmt.Sprintf(`labels.name == "%s"`, testResources[0].GetName())
	resp, err = clt.ListResources(ctx, withExpression)
	require.NoError(t, err)
	require.Len(t, resp.Resources, 1)
	require.Empty(t, cmp.Diff(testResources[0:1], resp.Resources))
}

// TestStreamSessionEventsRBAC ensures that session events can not be streamed
// by users who lack the read permission on the session resource.
func TestStreamSessionEventsRBAC(t *testing.T) {
	t.Parallel()

	role, err := types.NewRole("deny-sessions", types.RoleSpecV6{
		Allow: types.RoleConditions{
			NodeLabels: types.Labels{
				"*": []string{types.Wildcard},
			},
		},
		Deny: types.RoleConditions{
			Rules: []types.Rule{
				types.NewRule(types.KindSession, []string{types.VerbRead}),
			},
		},
	})
	require.NoError(t, err)

	srv := newTestTLSServer(t)

	user, err := CreateUser(context.Background(), srv.Auth(), "user", role)
	require.NoError(t, err)

	identity := TestUser(user.GetName())
	clt, err := srv.NewClient(identity)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, errC := clt.StreamSessionEvents(ctx, "foo", 0)
	select {
	case err := <-errC:
		require.ErrorAs(t, err, new(*trace.AccessDeniedError))
	case <-time.After(5 * time.Second):
		require.FailNow(t, "expected access denied error but stream succeeded")
	}
}

// TestStreamSessionEvents_User ensures that when a user streams a session's events, it emits an audit event.
func TestStreamSessionEvents_User(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv := newTestTLSServer(t)

	username := "user"
	user, _, err := CreateUserAndRole(srv.Auth(), username, nil, nil)
	require.NoError(t, err)

	identity := TestUser(user.GetName())
	clt, err := srv.NewClient(identity)
	require.NoError(t, err)

	// ignore the response as we don't want the events or the error (the session will not exist)
	_, _ = clt.StreamSessionEvents(ctx, "44c6cea8-362f-11ea-83aa-125400432324", 0)

	// we need to wait for a short period to ensure the event is returned
	time.Sleep(500 * time.Millisecond)

	searchEvents, _, err := srv.AuthServer.AuditLog.SearchEvents(ctx, events.SearchEventsRequest{
		From:       srv.Clock().Now().Add(-time.Hour),
		To:         srv.Clock().Now().Add(time.Hour),
		EventTypes: []string{events.SessionRecordingAccessEvent},
		Limit:      1,
		Order:      types.EventOrderDescending,
	})
	require.NoError(t, err)

	event := searchEvents[0].(*apievents.SessionRecordingAccess)
	require.Equal(t, username, event.User)
}

// TestStreamSessionEvents_Builtin ensures that when a builtin role streams a session's events, it does not emit
// an audit event.
func TestStreamSessionEvents_Builtin(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv := newTestTLSServer(t)

	identity := TestBuiltin(types.RoleProxy)
	clt, err := srv.NewClient(identity)
	require.NoError(t, err)

	// ignore the response as we don't want the events or the error (the session will not exist)
	_, _ = clt.StreamSessionEvents(ctx, "44c6cea8-362f-11ea-83aa-125400432324", 0)

	// we need to wait for a short period to ensure the event is returned
	time.Sleep(500 * time.Millisecond)

	searchEvents, _, err := srv.AuthServer.AuditLog.SearchEvents(ctx, events.SearchEventsRequest{
		From:       srv.Clock().Now().Add(-time.Hour),
		To:         srv.Clock().Now().Add(time.Hour),
		EventTypes: []string{events.SessionRecordingAccessEvent},
		Limit:      1,
		Order:      types.EventOrderDescending,
	})
	require.NoError(t, err)

	require.Empty(t, searchEvents)
}

// TestStreamSessionEvents ensures that when a user streams a session's events
// a "session recording access" event is emitted.
func TestStreamSessionEvents(t *testing.T) {
	t.Parallel()

	srv := newTestTLSServer(t)

	username := "user"
	user, _, err := CreateUserAndRole(srv.Auth(), username, nil, nil)
	require.NoError(t, err)

	identity := TestUser(user.GetName())
	clt, err := srv.NewClient(identity)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// ignore the response as we don't want the events or the error (the session will not exist)
	clt.StreamSessionEvents(ctx, session.ID("44c6cea8-362f-11ea-83aa-125400432324"), 0)

	// we need to wait for a short period to ensure the event is returned
	time.Sleep(500 * time.Millisecond)
	searchEvents, _, err := srv.AuthServer.AuditLog.SearchEvents(ctx, events.SearchEventsRequest{
		From:       srv.Clock().Now().Add(-time.Hour),
		To:         srv.Clock().Now().Add(time.Hour),
		EventTypes: []string{events.SessionRecordingAccessEvent},
		Limit:      1,
		Order:      types.EventOrderDescending,
	})
	require.NoError(t, err)

	event := searchEvents[0].(*apievents.SessionRecordingAccess)
	require.Equal(t, username, event.User)
}

// TestStreamSessionEvents ensures that when a user streams a session's events
// a "session recording access" event is emitted with the correct session type.
func TestStreamSessionEvents_SessionType(t *testing.T) {
	t.Parallel()

	authServerConfig := TestAuthServerConfig{
		Dir:   t.TempDir(),
		Clock: clockwork.NewFakeClockAt(time.Now().Round(time.Second).UTC()),
	}
	require.NoError(t, authServerConfig.CheckAndSetDefaults())

	uploader := eventstest.NewMemoryUploader()
	localLog, err := events.NewAuditLog(events.AuditLogConfig{
		DataDir:       authServerConfig.Dir,
		ServerID:      authServerConfig.ClusterName,
		Clock:         authServerConfig.Clock,
		UploadHandler: uploader,
	})
	require.NoError(t, err)
	authServerConfig.AuditLog = localLog

	as, err := NewTestAuthServer(authServerConfig)
	require.NoError(t, err)

	srv, err := as.NewTestTLSServer()
	require.NoError(t, err)
	t.Cleanup(func() { srv.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	username := "user"
	user, _, err := CreateUserAndRole(srv.Auth(), username, []string{}, nil)
	require.NoError(t, err)

	identity := TestUser(user.GetName())
	clt, err := srv.NewClient(identity)
	require.NoError(t, err)
	sessionID := session.NewID()

	streamer, err := events.NewProtoStreamer(events.ProtoStreamerConfig{
		Uploader: uploader,
	})
	require.NoError(t, err)
	stream, err := streamer.CreateAuditStream(ctx, sessionID)
	require.NoError(t, err)
	// The event is not required to pass through the auth server, we only need
	// the upload to be present.
	require.NoError(t, stream.RecordEvent(ctx, eventstest.PrepareEvent(&apievents.DatabaseSessionStart{
		Metadata: apievents.Metadata{
			Type: events.DatabaseSessionStartEvent,
			Code: events.DatabaseSessionStartCode,
		},
		SessionMetadata: apievents.SessionMetadata{
			SessionID: sessionID.String(),
		},
	})))
	require.NoError(t, stream.Complete(ctx))

	accessedFormat := teleport.PTY
	clt.StreamSessionEvents(metadata.WithSessionRecordingFormatContext(ctx, accessedFormat), sessionID, 0)

	// Perform the listing an eventually loop to ensure the event is emitted.
	var searchEvents []apievents.AuditEvent
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		var err error
		searchEvents, _, err = srv.AuthServer.AuditLog.SearchEvents(ctx, events.SearchEventsRequest{
			From:       srv.Clock().Now().Add(-time.Hour),
			To:         srv.Clock().Now().Add(time.Hour),
			EventTypes: []string{events.SessionRecordingAccessEvent},
			Limit:      1,
			Order:      types.EventOrderDescending,
		})
		assert.NoError(t, err)
		assert.Len(t, searchEvents, 1, "expected one event but got %d", len(searchEvents))
	}, 5*time.Second, 200*time.Millisecond)

	event := searchEvents[0].(*apievents.SessionRecordingAccess)
	require.Equal(t, username, event.User)
	require.Equal(t, string(types.DatabaseSessionKind), event.SessionType)
	require.Equal(t, accessedFormat, event.Format)
}

// TestAPILockedOut tests Auth API when there are locks involved.
func TestAPILockedOut(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	// Create user, role and client.
	user, role, err := CreateUserAndRole(srv.Auth(), "test-user", nil, nil)
	require.NoError(t, err)
	clt, err := srv.NewClient(TestUser(user.GetName()))
	require.NoError(t, err)

	// Prepare an operation requiring authorization.
	testOp := func() error {
		_, err := clt.GetUser(ctx, user.GetName(), false)
		return err
	}

	// With no locks, the operation should pass with no error.
	require.NoError(t, testOp())

	// With a lock targeting the user, the operation should be denied.
	lock, err := types.NewLock("user-lock", types.LockSpecV2{
		Target: types.LockTarget{User: user.GetName()},
	})
	require.NoError(t, err)
	require.NoError(t, srv.Auth().UpsertLock(ctx, lock))
	require.Eventually(t, func() bool { return trace.IsAccessDenied(testOp()) }, time.Second, time.Second/10)

	// Delete the lock.
	require.NoError(t, srv.Auth().DeleteLock(ctx, lock.GetName()))
	require.Eventually(t, func() bool { return testOp() == nil }, time.Second, time.Second/10)

	// Create a new lock targeting the user's role.
	roleLock, err := types.NewLock("role-lock", types.LockSpecV2{
		Target: types.LockTarget{Role: role.GetName()},
	})
	require.NoError(t, err)
	require.NoError(t, srv.Auth().UpsertLock(ctx, roleLock))
	require.Eventually(t, func() bool { return trace.IsAccessDenied(testOp()) }, time.Second, time.Second/10)
}

func serverWithAllowRules(t *testing.T, srv *TestAuthServer, allowRules []types.Rule) *ServerWithRoles {
	username := "test-user"
	ctx := context.Background()
	_, role, err := CreateUserAndRoleWithoutRoles(srv.AuthServer, username, nil)
	require.NoError(t, err)
	role.SetRules(types.Allow, allowRules)
	_, err = srv.AuthServer.UpsertRole(ctx, role)
	require.NoError(t, err)

	localUser := authz.LocalUser{Username: username, Identity: tlsca.Identity{Username: username}}
	authContext, err := authz.ContextForLocalUser(ctx, localUser, srv.AuthServer.Services, srv.ClusterName, true /* disableDeviceAuthz */)
	require.NoError(t, err)
	authContext.AdminActionAuthState = authz.AdminActionAuthMFAVerified

	return &ServerWithRoles{
		authServer: srv.AuthServer,
		alog:       srv.AuditLog,
		context:    *authContext,
	}
}

// TestDatabasesCRUDRBAC verifies RBAC is applied to database CRUD methods.
func TestDatabasesCRUDRBAC(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	// Setup a couple of users:
	// - "dev" only has access to databases with labels env=dev
	// - "admin" has access to all databases
	dev, devRole, err := CreateUserAndRole(srv.Auth(), "dev", nil, nil)
	require.NoError(t, err)
	devRole.SetDatabaseLabels(types.Allow, types.Labels{"env": {"dev"}})
	_, err = srv.Auth().UpsertRole(ctx, devRole)
	require.NoError(t, err)
	devClt, err := srv.NewClient(TestUser(dev.GetName()))
	require.NoError(t, err)

	admin, adminRole, err := CreateUserAndRole(srv.Auth(), "admin", nil, nil)
	require.NoError(t, err)
	adminRole.SetDatabaseLabels(types.Allow, types.Labels{types.Wildcard: {types.Wildcard}})
	_, err = srv.Auth().UpsertRole(ctx, adminRole)
	require.NoError(t, err)
	adminClt, err := srv.NewClient(TestUser(admin.GetName()))
	require.NoError(t, err)

	// Prepare a couple of database resources.
	devDatabase, err := types.NewDatabaseV3(types.Metadata{
		Name:   "dev",
		Labels: map[string]string{"env": "dev", types.OriginLabel: types.OriginDynamic},
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5432",
	})
	require.NoError(t, err)
	adminDatabase, err := types.NewDatabaseV3(types.Metadata{
		Name:   "admin",
		Labels: map[string]string{"env": "prod", types.OriginLabel: types.OriginDynamic},
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolMySQL,
		URI:      "localhost:3306",
	})
	require.NoError(t, err)

	// Dev shouldn't be able to create prod database...
	err = devClt.CreateDatabase(ctx, adminDatabase)
	require.True(t, trace.IsAccessDenied(err))

	// ... but can create dev database.
	err = devClt.CreateDatabase(ctx, devDatabase)
	require.NoError(t, err)

	// Admin can create prod database.
	err = adminClt.CreateDatabase(ctx, adminDatabase)
	require.NoError(t, err)

	// Dev shouldn't be able to update prod database...
	err = devClt.UpdateDatabase(ctx, adminDatabase)
	require.True(t, trace.IsAccessDenied(err))

	// ... but can update dev database.
	err = devClt.UpdateDatabase(ctx, devDatabase)
	require.NoError(t, err)

	// Dev shouldn't be able to update labels on the prod database.
	adminDatabase.SetStaticLabels(map[string]string{"env": "dev", types.OriginLabel: types.OriginDynamic})
	err = devClt.UpdateDatabase(ctx, adminDatabase)
	require.True(t, trace.IsAccessDenied(err))
	adminDatabase.SetStaticLabels(map[string]string{"env": "prod", types.OriginLabel: types.OriginDynamic}) // Reset.

	// Dev shouldn't be able to get prod database...
	_, err = devClt.GetDatabase(ctx, adminDatabase.GetName())
	require.True(t, trace.IsAccessDenied(err))

	// ... but can get dev database.
	db, err := devClt.GetDatabase(ctx, devDatabase.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(devDatabase, db,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	// Admin can get both databases.
	db, err = adminClt.GetDatabase(ctx, adminDatabase.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(adminDatabase, db,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))
	db, err = adminClt.GetDatabase(ctx, devDatabase.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(devDatabase, db,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	// When listing databases, dev should only see one.
	dbs, err := devClt.GetDatabases(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.Database{devDatabase}, dbs,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	// Admin should see both.
	dbs, err = adminClt.GetDatabases(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.Database{adminDatabase, devDatabase}, dbs,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	// Dev shouldn't be able to delete dev database...
	err = devClt.DeleteDatabase(ctx, adminDatabase.GetName())
	require.True(t, trace.IsAccessDenied(err))

	// ... but can delete dev database.
	err = devClt.DeleteDatabase(ctx, devDatabase.GetName())
	require.NoError(t, err)

	// Admin should be able to delete admin database.
	err = adminClt.DeleteDatabase(ctx, adminDatabase.GetName())
	require.NoError(t, err)

	// Create both databases again to test "delete all" functionality.
	require.NoError(t, devClt.CreateDatabase(ctx, devDatabase))
	require.NoError(t, adminClt.CreateDatabase(ctx, adminDatabase))

	// Dev should only be able to delete dev database.
	err = devClt.DeleteAllDatabases(ctx)
	require.NoError(t, err)
	mustGetDatabases(t, adminClt, []types.Database{adminDatabase})

	// Admin should be able to delete all.
	err = adminClt.DeleteAllDatabases(ctx)
	require.NoError(t, err)
	mustGetDatabases(t, adminClt, nil)

	t.Run("discovery service", func(t *testing.T) {
		t.Cleanup(func() {
			require.NoError(t, adminClt.DeleteAllDatabases(ctx))
		})

		// Prepare discovery service client.
		discoveryClt, err := srv.NewClient(TestBuiltin(types.RoleDiscovery))
		require.NoError(t, err)

		cloudDatabase, err := types.NewDatabaseV3(types.Metadata{
			Name:   "cloud1",
			Labels: map[string]string{"env": "prod", types.OriginLabel: types.OriginCloud},
		}, types.DatabaseSpecV3{
			Protocol: defaults.ProtocolMySQL,
			URI:      "localhost:3306",
		})
		require.NoError(t, err)

		// Create a non-cloud database.
		require.NoError(t, adminClt.CreateDatabase(ctx, adminDatabase))
		mustGetDatabases(t, adminClt, []types.Database{adminDatabase})

		t.Run("cannot create non-cloud database", func(t *testing.T) {
			require.True(t, trace.IsAccessDenied(discoveryClt.CreateDatabase(ctx, devDatabase)))
			require.True(t, trace.IsAccessDenied(discoveryClt.UpdateDatabase(ctx, adminDatabase)))
		})
		t.Run("cannot create database with dynamic labels", func(t *testing.T) {
			cloudDatabaseWithDynamicLabels, err := types.NewDatabaseV3(types.Metadata{
				Name:   "cloud2",
				Labels: map[string]string{"env": "prod", types.OriginLabel: types.OriginCloud},
			}, types.DatabaseSpecV3{
				Protocol: defaults.ProtocolMySQL,
				URI:      "localhost:3306",
				DynamicLabels: map[string]types.CommandLabelV2{
					"hostname": {
						Period:  types.Duration(time.Hour),
						Command: []string{"hostname"},
					},
				},
			})
			require.NoError(t, err)
			require.True(t, trace.IsAccessDenied(discoveryClt.CreateDatabase(ctx, cloudDatabaseWithDynamicLabels)))
		})
		t.Run("can create cloud database", func(t *testing.T) {
			require.NoError(t, discoveryClt.CreateDatabase(ctx, cloudDatabase))
			require.NoError(t, discoveryClt.UpdateDatabase(ctx, cloudDatabase))
		})
		t.Run("can get only cloud database", func(t *testing.T) {
			mustGetDatabases(t, discoveryClt, []types.Database{cloudDatabase})
		})
		t.Run("can delete only cloud database", func(t *testing.T) {
			require.NoError(t, discoveryClt.DeleteAllDatabases(ctx))
			mustGetDatabases(t, discoveryClt, nil)
			mustGetDatabases(t, adminClt, []types.Database{adminDatabase})
		})
	})
}

func mustGetDatabases(t *testing.T, client *authclient.Client, wantDatabases []types.Database) {
	t.Helper()

	actualDatabases, err := client.GetDatabases(context.Background())
	require.NoError(t, err)

	require.Empty(t, cmp.Diff(wantDatabases, actualDatabases,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
		cmpopts.EquateEmpty(),
	))
}

func TestKubernetesClusterCRUD_DiscoveryService(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	discoveryClt, err := srv.NewClient(TestBuiltin(types.RoleDiscovery))
	require.NoError(t, err)

	eksCluster, err := common.NewKubeClusterFromAWSEKS(
		"eks-cluster1",
		"arn:aws:eks:eu-west-1:accountID:cluster/cluster1",
		nil,
	)
	require.NoError(t, err)
	eksCluster.SetOrigin(types.OriginCloud)

	// Discovery service must not have access to non-cloud cluster (cluster
	// without "cloud" origin label).
	nonCloudCluster, err := types.NewKubernetesClusterV3(
		types.Metadata{
			Name: "non-cloud",
		},
		types.KubernetesClusterSpecV3{},
	)
	require.NoError(t, err)
	require.NoError(t, srv.Auth().CreateKubernetesCluster(ctx, nonCloudCluster))

	// Discovery service cannot create cluster with dynamic labels.
	clusterWithDynamicLabels, err := common.NewKubeClusterFromAWSEKS(
		"eks-cluster2",
		"arn:aws:eks:eu-west-1:accountID:cluster/cluster2",
		nil,
	)
	require.NoError(t, err)
	clusterWithDynamicLabels.SetOrigin(types.OriginCloud)
	clusterWithDynamicLabels.SetDynamicLabels(map[string]types.CommandLabel{
		"hostname": &types.CommandLabelV2{
			Period:  types.Duration(time.Hour),
			Command: []string{"hostname"},
		},
	})

	t.Run("Create", func(t *testing.T) {
		require.NoError(t, discoveryClt.CreateKubernetesCluster(ctx, eksCluster))
		require.True(t, trace.IsAccessDenied(discoveryClt.CreateKubernetesCluster(ctx, nonCloudCluster)))
		require.True(t, trace.IsAccessDenied(discoveryClt.CreateKubernetesCluster(ctx, clusterWithDynamicLabels)))
	})
	t.Run("Read", func(t *testing.T) {
		clusters, err := discoveryClt.GetKubernetesClusters(ctx)
		require.NoError(t, err)
		require.Empty(t, cmp.Diff([]types.KubeCluster{eksCluster}, clusters, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
	})
	t.Run("Update", func(t *testing.T) {
		require.NoError(t, discoveryClt.UpdateKubernetesCluster(ctx, eksCluster))
		require.True(t, trace.IsAccessDenied(discoveryClt.UpdateKubernetesCluster(ctx, nonCloudCluster)))
	})
	t.Run("Delete", func(t *testing.T) {
		require.NoError(t, discoveryClt.DeleteAllKubernetesClusters(ctx))
		clusters, err := discoveryClt.GetKubernetesClusters(ctx)
		require.NoError(t, err)
		require.Empty(t, clusters)

		// Discovery service cannot delete non-cloud clusters.
		clusters, err = srv.Auth().GetKubernetesClusters(ctx)
		require.NoError(t, err)
		require.Len(t, clusters, 1)
	})
}

func TestGetAndList_DatabaseServers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	// Create test databases.
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("db-%d", i)
		database, err := types.NewDatabaseV3(
			types.Metadata{
				Name:   name,
				Labels: map[string]string{"name": name},
			},
			types.DatabaseSpecV3{
				Protocol: defaults.ProtocolPostgres,
				URI:      "example.com",
			},
		)
		require.NoError(t, err)
		db, err := types.NewDatabaseServerV3(types.Metadata{
			Name:   name,
			Labels: map[string]string{"name": name},
		}, types.DatabaseServerSpecV3{
			Database: database,
			Hostname: "host",
			HostID:   "hostid",
		})
		require.NoError(t, err)

		_, err = srv.Auth().UpsertDatabaseServer(ctx, db)
		require.NoError(t, err)
	}

	testServers, err := srv.Auth().GetDatabaseServers(ctx, apidefaults.Namespace)
	require.NoError(t, err)

	testResources := make([]types.ResourceWithLabels, len(testServers))
	for i, server := range testServers {
		testResources[i] = server
	}

	// create user, role, and client
	username := "user"
	user, role, err := CreateUserAndRole(srv.Auth(), username, nil, nil)
	require.NoError(t, err)
	identity := TestUser(user.GetName())
	clt, err := srv.NewClient(identity)
	require.NoError(t, err)

	listRequest := proto.ListResourcesRequest{
		Namespace: apidefaults.Namespace,
		// Guarantee that the list will all the servers.
		Limit:        int32(len(testServers) + 1),
		ResourceType: types.KindDatabaseServer,
	}

	// permit user to get the first database
	role.SetDatabaseLabels(types.Allow, types.Labels{"name": {testServers[0].GetName()}})
	_, err = srv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)
	servers, err := clt.GetDatabaseServers(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Len(t, servers, 1)
	require.Empty(t, cmp.Diff(testServers[0:1], servers))
	resp, err := clt.ListResources(ctx, listRequest)
	require.NoError(t, err)
	require.Len(t, resp.Resources, 1)
	require.Empty(t, cmp.Diff(testResources[0:1], resp.Resources))

	// permit user to get all databases
	role.SetDatabaseLabels(types.Allow, types.Labels{types.Wildcard: {types.Wildcard}})
	_, err = srv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)
	servers, err = clt.GetDatabaseServers(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.EqualValues(t, len(testServers), len(servers))
	require.Empty(t, cmp.Diff(testServers, servers))
	resp, err = clt.ListResources(ctx, listRequest)
	require.NoError(t, err)
	require.Len(t, resp.Resources, len(testResources))
	require.Empty(t, cmp.Diff(testResources, resp.Resources))

	// Test various filtering.
	baseRequest := proto.ListResourcesRequest{
		Namespace:    apidefaults.Namespace,
		Limit:        int32(len(testServers) + 1),
		ResourceType: types.KindDatabaseServer,
	}

	// list only database with label
	withLabels := baseRequest
	withLabels.Labels = map[string]string{"name": testServers[0].GetName()}
	resp, err = clt.ListResources(ctx, withLabels)
	require.NoError(t, err)
	require.Len(t, resp.Resources, 1)
	require.Empty(t, cmp.Diff(testResources[0:1], resp.Resources))

	// Test search keywords match.
	withSearchKeywords := baseRequest
	withSearchKeywords.SearchKeywords = []string{"name", testServers[0].GetName()}
	resp, err = clt.ListResources(ctx, withSearchKeywords)
	require.NoError(t, err)
	require.Len(t, resp.Resources, 1)
	require.Empty(t, cmp.Diff(testResources[0:1], resp.Resources))

	// Test expression match.
	withExpression := baseRequest
	withExpression.PredicateExpression = fmt.Sprintf(`labels.name == "%s"`, testServers[0].GetName())
	resp, err = clt.ListResources(ctx, withExpression)
	require.NoError(t, err)
	require.Len(t, resp.Resources, 1)
	require.Empty(t, cmp.Diff(testResources[0:1], resp.Resources))

	// deny user to get the first database
	role.SetDatabaseLabels(types.Deny, types.Labels{"name": {testServers[0].GetName()}})
	_, err = srv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)
	servers, err = clt.GetDatabaseServers(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.EqualValues(t, len(testServers[1:]), len(servers))
	require.Empty(t, cmp.Diff(testServers[1:], servers))
	resp, err = clt.ListResources(ctx, listRequest)
	require.NoError(t, err)
	require.Len(t, resp.Resources, len(testResources[1:]))
	require.Empty(t, cmp.Diff(testResources[1:], resp.Resources))

	// deny user to get all databases
	role.SetDatabaseLabels(types.Deny, types.Labels{types.Wildcard: {types.Wildcard}})
	_, err = srv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)
	servers, err = clt.GetDatabaseServers(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Empty(t, servers)
	resp, err = clt.ListResources(ctx, listRequest)
	require.NoError(t, err)
	require.Empty(t, resp.Resources)
}

// TestGetAndList_ApplicationServers verifies RBAC and filtering is applied when fetching app servers.
func TestGetAndList_ApplicationServers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	// Create test app servers.
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("app-%v", i)
		app, err := types.NewAppV3(types.Metadata{
			Name:   name,
			Labels: map[string]string{"name": name},
		},
			types.AppSpecV3{URI: "localhost"})
		require.NoError(t, err)
		server, err := types.NewAppServerV3FromApp(app, "host", "hostid")
		require.NoError(t, err)

		_, err = srv.Auth().UpsertApplicationServer(ctx, server)
		require.NoError(t, err)
	}

	testServers, err := srv.Auth().GetApplicationServers(ctx, apidefaults.Namespace)
	require.NoError(t, err)

	testResources := make([]types.ResourceWithLabels, len(testServers))
	for i, server := range testServers {
		testResources[i] = server
	}

	listRequest := proto.ListResourcesRequest{
		Namespace: apidefaults.Namespace,
		// Guarantee that the list will all the servers.
		Limit:        int32(len(testServers) + 1),
		ResourceType: types.KindAppServer,
	}

	// create user, role, and client
	username := "user"
	user, role, err := CreateUserAndRole(srv.Auth(), username, nil, nil)
	require.NoError(t, err)
	identity := TestUser(user.GetName())
	clt, err := srv.NewClient(identity)
	require.NoError(t, err)

	// permit user to get the first app
	role.SetAppLabels(types.Allow, types.Labels{"name": {testServers[0].GetName()}})
	_, err = srv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)
	servers, err := clt.GetApplicationServers(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Len(t, servers, 1)
	require.Empty(t, cmp.Diff(testServers[0:1], servers))
	resp, err := clt.ListResources(ctx, listRequest)
	require.NoError(t, err)
	require.Len(t, resp.Resources, 1)
	require.Empty(t, cmp.Diff(testResources[0:1], resp.Resources))

	// permit user to get all apps
	role.SetAppLabels(types.Allow, types.Labels{types.Wildcard: {types.Wildcard}})
	_, err = srv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)
	servers, err = clt.GetApplicationServers(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.EqualValues(t, len(testServers), len(servers))
	require.Empty(t, cmp.Diff(testServers, servers))
	resp, err = clt.ListResources(ctx, listRequest)
	require.NoError(t, err)
	require.Len(t, resp.Resources, len(testResources))
	require.Empty(t, cmp.Diff(testResources, resp.Resources))

	// Test various filtering.
	baseRequest := proto.ListResourcesRequest{
		Namespace:    apidefaults.Namespace,
		Limit:        int32(len(testServers) + 1),
		ResourceType: types.KindAppServer,
	}

	// list only application with label
	withLabels := baseRequest
	withLabels.Labels = map[string]string{"name": testServers[0].GetName()}
	resp, err = clt.ListResources(ctx, withLabels)
	require.NoError(t, err)
	require.Len(t, resp.Resources, 1)
	require.Empty(t, cmp.Diff(testResources[0:1], resp.Resources))

	// Test search keywords match.
	withSearchKeywords := baseRequest
	withSearchKeywords.SearchKeywords = []string{"name", testServers[0].GetName()}
	resp, err = clt.ListResources(ctx, withSearchKeywords)
	require.NoError(t, err)
	require.Len(t, resp.Resources, 1)
	require.Empty(t, cmp.Diff(testResources[0:1], resp.Resources))

	// Test expression match.
	withExpression := baseRequest
	withExpression.PredicateExpression = fmt.Sprintf(`labels.name == "%s"`, testServers[0].GetName())
	resp, err = clt.ListResources(ctx, withExpression)
	require.NoError(t, err)
	require.Len(t, resp.Resources, 1)
	require.Empty(t, cmp.Diff(testResources[0:1], resp.Resources))

	// deny user to get the first app
	role.SetAppLabels(types.Deny, types.Labels{"name": {testServers[0].GetName()}})
	_, err = srv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)
	servers, err = clt.GetApplicationServers(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.EqualValues(t, len(testServers[1:]), len(servers))
	require.Empty(t, cmp.Diff(testServers[1:], servers))
	resp, err = clt.ListResources(ctx, listRequest)
	require.NoError(t, err)
	require.Len(t, resp.Resources, len(testResources[1:]))
	require.Empty(t, cmp.Diff(testResources[1:], resp.Resources))

	// deny user to get all apps
	role.SetAppLabels(types.Deny, types.Labels{types.Wildcard: {types.Wildcard}})
	_, err = srv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)
	servers, err = clt.GetApplicationServers(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Empty(t, servers)
	resp, err = clt.ListResources(ctx, listRequest)
	require.NoError(t, err)
	require.Empty(t, resp.Resources)
}

// TestGetAndList_AppServersAndSAMLIdPServiceProviders verifies RBAC and filtering is applied when fetching App Servers and SAML IdP Service Providers.
// DELETE IN 17.0
func TestGetAndList_AppServersAndSAMLIdPServiceProviders(t *testing.T) {
	ctx := context.Background()
	srv := newTestTLSServer(t)

	// Set license to enterprise in order to be able to list SAML IdP Service Providers.
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
	})

	// Create test app servers and SAML IdP Service Providers.
	for i := 0; i < 6; i++ {
		// Alternate between creating AppServers and SAMLIdPServiceProviders
		if i%2 == 0 {
			name := fmt.Sprintf("app-server-%v", i)
			app, err := types.NewAppV3(types.Metadata{
				Name:   name,
				Labels: map[string]string{"name": name},
			},
				types.AppSpecV3{URI: "localhost"})
			require.NoError(t, err)
			server, err := types.NewAppServerV3FromApp(app, "host", "hostid")
			server.Spec.Version = types.V3
			require.NoError(t, err)

			_, err = srv.Auth().UpsertApplicationServer(ctx, server)
			require.NoError(t, err)
		} else {
			name := fmt.Sprintf("saml-app-%v", i)
			sp, err := types.NewSAMLIdPServiceProvider(types.Metadata{
				Name:      name,
				Namespace: apidefaults.Namespace,
			}, types.SAMLIdPServiceProviderSpecV1{
				ACSURL:   fmt.Sprintf("https://entity-id-%v", i),
				EntityID: fmt.Sprintf("entity-id-%v", i),
			})
			require.NoError(t, err)
			err = srv.Auth().CreateSAMLIdPServiceProvider(ctx, sp)
			require.NoError(t, err)
		}
	}

	testAppServers, err := srv.Auth().GetApplicationServers(ctx, apidefaults.Namespace)
	require.NoError(t, err)

	testServiceProviders, _, err := srv.Auth().ListSAMLIdPServiceProviders(ctx, 0, "")
	require.NoError(t, err)

	numResources := len(testAppServers) + len(testServiceProviders)

	testResources := make([]types.ResourceWithLabels, numResources)
	for i, server := range testAppServers {
		testResources[i] = createAppServerOrSPFromAppServer(server)
	}

	for i, sp := range testServiceProviders {
		testResources[i+len(testAppServers)] = createAppServerOrSPFromSP(sp)
	}

	listRequest := proto.ListResourcesRequest{
		Namespace: apidefaults.Namespace,
		// Guarantee that the list will have all the app servers and IdP service providers.
		Limit:        int32(numResources + 1),
		ResourceType: types.KindAppOrSAMLIdPServiceProvider,
	}

	// create user, role, and client
	username := "user"
	user, role, err := CreateUserAndRole(srv.Auth(), username, nil, nil)
	require.NoError(t, err)
	identity := TestUser(user.GetName())
	clt, err := srv.NewClient(identity)
	require.NoError(t, err)

	// permit user to get the first app
	listRequestAppsOnly := listRequest
	listRequestAppsOnly.SearchKeywords = []string{"app-server"}
	role.SetAppLabels(types.Allow, types.Labels{"name": {testAppServers[0].GetName()}})
	_, err = srv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)
	servers, err := clt.GetApplicationServers(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Len(t, servers, 1)
	require.Empty(t, cmp.Diff(testAppServers[0:1], servers))
	resp, err := clt.ListResources(ctx, listRequestAppsOnly)
	require.NoError(t, err)
	require.Len(t, resp.Resources, 1)
	require.Empty(t, cmp.Diff(testResources[0:1], resp.Resources))

	// Permit user to get all apps and saml idp service providers.
	role.SetAppLabels(types.Allow, types.Labels{types.Wildcard: {types.Wildcard}})

	_, err = srv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)

	// Test getting all apps and SAML IdP service providers.
	resp, err = clt.ListResources(ctx, listRequest)
	require.NoError(t, err)
	require.Len(t, resp.Resources, len(testResources))
	require.Empty(t, cmp.Diff(testResources, resp.Resources))
	// Test various filtering.
	baseRequest := proto.ListResourcesRequest{
		Namespace:    apidefaults.Namespace,
		Limit:        int32(numResources + 1),
		ResourceType: types.KindAppOrSAMLIdPServiceProvider,
	}

	// list only application with label
	withLabels := baseRequest
	withLabels.Labels = map[string]string{"name": testAppServers[0].GetName()}
	resp, err = clt.ListResources(ctx, withLabels)
	require.NoError(t, err)
	require.Len(t, resp.Resources, 1)
	require.Empty(t, cmp.Diff(testResources[0:1], resp.Resources))

	// Test search keywords match for app servers.
	withSearchKeywords := baseRequest
	withSearchKeywords.SearchKeywords = []string{"app-server", testAppServers[0].GetName()}
	resp, err = clt.ListResources(ctx, withSearchKeywords)
	require.NoError(t, err)
	require.Len(t, resp.Resources, 1)
	require.Empty(t, cmp.Diff(testResources[0:1], resp.Resources))

	// Test search keywords match for saml idp service providers servers.
	withSearchKeywords.SearchKeywords = []string{"saml-app", testServiceProviders[0].GetName()}
	resp, err = clt.ListResources(ctx, withSearchKeywords)
	require.NoError(t, err)
	require.Len(t, resp.Resources, 1)
	require.Empty(t, cmp.Diff(testResources[len(testAppServers):len(testAppServers)+1], resp.Resources))

	// Test expression match for app servers.
	withExpression := baseRequest
	withExpression.PredicateExpression = fmt.Sprintf(`search("%s")`, testResources[0].GetName())
	resp, err = clt.ListResources(ctx, withExpression)
	require.NoError(t, err)
	require.Len(t, resp.Resources, 1)
	require.Empty(t, cmp.Diff(testResources[0:1], resp.Resources))

	// deny user to get the first app
	role.SetAppLabels(types.Deny, types.Labels{"name": {testAppServers[0].GetName()}})
	_, err = srv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)
	servers, err = clt.GetApplicationServers(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.EqualValues(t, len(testAppServers[1:]), len(servers))
	require.Empty(t, cmp.Diff(testAppServers[1:], servers))
	resp, err = clt.ListResources(ctx, listRequest)
	require.NoError(t, err)
	require.Len(t, resp.Resources, len(testResources[1:]))
	require.Empty(t, cmp.Diff(testResources[1:], resp.Resources))

	// deny user to get all apps and service providers
	role.SetAppLabels(types.Deny, types.Labels{types.Wildcard: {types.Wildcard}})
	role.SetRules(types.Deny, []types.Rule{
		{
			Resources: []string{types.KindSAMLIdPServiceProvider},
			Verbs:     []string{types.VerbList},
		},
	})
	_, err = srv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)
	servers, err = clt.GetApplicationServers(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Empty(t, servers)
	resp, err = clt.ListResources(ctx, listRequest)
	require.NoError(t, err)
	require.Empty(t, resp.Resources)
}

// TestListSAMLIdPServiceProviderAndListResources verifies
// RBAC and search filters when fetching SAML IdP service providers.
func TestListSAMLIdPServiceProviderAndListResources(t *testing.T) {
	// Set license to enterprise in order to be able to list SAML IdP service providers.
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
	})

	ctx := context.Background()
	srv := newTestTLSServer(t)

	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("saml-app-%v", i)
		sp, err := types.NewSAMLIdPServiceProvider(types.Metadata{
			Name:   name,
			Labels: map[string]string{"name": name},
		}, types.SAMLIdPServiceProviderSpecV1{
			ACSURL:   fmt.Sprintf("https://entity-id-%v", i),
			EntityID: fmt.Sprintf("entity-id-%v", i),
		})
		require.NoError(t, err)
		err = srv.Auth().CreateSAMLIdPServiceProvider(ctx, sp)
		require.NoError(t, err)
	}

	testServiceProviders, _, err := srv.Auth().ListSAMLIdPServiceProviders(ctx, 0, "")
	require.NoError(t, err)

	testResources := make([]types.ResourceWithLabels, len(testServiceProviders))
	for i, sp := range testServiceProviders {
		testResources[i] = sp
	}

	listRequest := proto.ListResourcesRequest{
		Namespace: apidefaults.Namespace,
		// Guarantee that the list will have all SAML IdP service providers.
		Limit:        int32(len(testServiceProviders) + 1),
		ResourceType: types.KindSAMLIdPServiceProvider,
	}

	// create user, role, and client
	username := "user"
	user, role, err := CreateUserAndRole(srv.Auth(), username, nil, nil)
	require.NoError(t, err)
	identity := TestUser(user.GetName())
	clt, err := srv.NewClient(identity)
	require.NoError(t, err)

	// permit user to get the first service provider
	role.SetAppLabels(types.Allow, types.Labels{"name": {testServiceProviders[0].GetName()}})
	_, err = srv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)

	sps, _, err := clt.ListSAMLIdPServiceProviders(ctx, 0, "")
	require.NoError(t, err)
	require.Len(t, sps, 1)
	require.Empty(t, cmp.Diff(testServiceProviders[0:1], sps))
	resp, err := clt.ListResources(ctx, listRequest)
	require.NoError(t, err)
	require.Len(t, resp.Resources, 1)
	require.Empty(t, cmp.Diff(testResources[0:1], resp.Resources))

	// permit user to get all service providers
	role.SetAppLabels(types.Allow, types.Labels{types.Wildcard: {types.Wildcard}})
	_, err = srv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)
	sps, _, err = clt.ListSAMLIdPServiceProviders(ctx, 0, "")
	require.NoError(t, err)
	require.EqualValues(t, len(testServiceProviders), len(sps))
	require.Empty(t, cmp.Diff(testServiceProviders, sps))
	resp, err = clt.ListResources(ctx, listRequest)
	require.NoError(t, err)
	require.Len(t, resp.Resources, len(testResources))
	require.Empty(t, cmp.Diff(testResources, resp.Resources))

	// Test various filtering.
	baseRequest := proto.ListResourcesRequest{
		Namespace:    apidefaults.Namespace,
		Limit:        int32(len(testServiceProviders) + 1),
		ResourceType: types.KindSAMLIdPServiceProvider,
	}

	// list only service providers with label
	withLabels := baseRequest
	withLabels.Labels = map[string]string{"name": testServiceProviders[0].GetName()}
	resp, err = clt.ListResources(ctx, withLabels)
	require.NoError(t, err)
	require.Len(t, resp.Resources, 1)
	require.Empty(t, cmp.Diff(testResources[0:1], resp.Resources))

	// Test search keywords match.
	withSearchKeywords := baseRequest
	withSearchKeywords.SearchKeywords = []string{"name", testServiceProviders[0].GetName()}
	resp, err = clt.ListResources(ctx, withSearchKeywords)
	require.NoError(t, err)
	require.Len(t, resp.Resources, 1)
	require.Empty(t, cmp.Diff(testResources[0:1], resp.Resources))

	// Test expression match.
	withExpression := baseRequest
	withExpression.PredicateExpression = fmt.Sprintf(`labels.name == "%s"`, testServiceProviders[0].GetName())
	resp, err = clt.ListResources(ctx, withExpression)
	require.NoError(t, err)
	require.Len(t, resp.Resources, 1)
	require.Empty(t, cmp.Diff(testResources[0:1], resp.Resources))

	// deny user to get the first service provider
	role.SetAppLabels(types.Deny, types.Labels{"name": {testServiceProviders[0].GetName()}})
	_, err = srv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)
	sps, _, err = clt.ListSAMLIdPServiceProviders(ctx, 0, "")
	require.NoError(t, err)
	require.EqualValues(t, len(testServiceProviders[1:]), len(sps))
	require.Empty(t, cmp.Diff(testServiceProviders[1:], sps))
	resp, err = clt.ListResources(ctx, listRequest)
	require.NoError(t, err)
	require.Len(t, resp.Resources, len(testResources[1:]))
	require.Empty(t, cmp.Diff(testResources[1:], resp.Resources))

	// deny user to get all service providers
	role.SetAppLabels(types.Deny, types.Labels{types.Wildcard: {types.Wildcard}})
	_, err = srv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)
	sps, _, err = clt.ListSAMLIdPServiceProviders(ctx, 0, "")
	require.NoError(t, err)
	require.Empty(t, sps)
	resp, err = clt.ListResources(ctx, listRequest)
	require.NoError(t, err)
	require.Empty(t, resp.Resources)
}

// TestApps verifies RBAC is applied to app resources.
func TestApps(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	// Setup a couple of users:
	// - "dev" only has access to apps with labels env=dev
	// - "admin" has access to all apps
	dev, devRole, err := CreateUserAndRole(srv.Auth(), "dev", nil, nil)
	require.NoError(t, err)
	devRole.SetAppLabels(types.Allow, types.Labels{"env": {"dev"}})
	_, err = srv.Auth().UpsertRole(ctx, devRole)
	require.NoError(t, err)
	devClt, err := srv.NewClient(TestUser(dev.GetName()))
	require.NoError(t, err)

	admin, adminRole, err := CreateUserAndRole(srv.Auth(), "admin", nil, nil)
	require.NoError(t, err)
	adminRole.SetAppLabels(types.Allow, types.Labels{types.Wildcard: {types.Wildcard}})
	_, err = srv.Auth().UpsertRole(ctx, adminRole)
	require.NoError(t, err)
	adminClt, err := srv.NewClient(TestUser(admin.GetName()))
	require.NoError(t, err)

	// Prepare a couple of app resources.
	devApp, err := types.NewAppV3(types.Metadata{
		Name:   "dev",
		Labels: map[string]string{"env": "dev", types.OriginLabel: types.OriginDynamic},
	}, types.AppSpecV3{
		URI: "localhost1",
	})
	require.NoError(t, err)
	adminApp, err := types.NewAppV3(types.Metadata{
		Name:   "admin",
		Labels: map[string]string{"env": "prod", types.OriginLabel: types.OriginDynamic},
	}, types.AppSpecV3{
		URI: "localhost2",
	})
	require.NoError(t, err)

	// Dev shouldn't be able to create prod app...
	err = devClt.CreateApp(ctx, adminApp)
	require.True(t, trace.IsAccessDenied(err))

	// ... but can create dev app.
	err = devClt.CreateApp(ctx, devApp)
	require.NoError(t, err)

	// Admin can create prod app.
	err = adminClt.CreateApp(ctx, adminApp)
	require.NoError(t, err)

	// Dev shouldn't be able to update prod app...
	err = devClt.UpdateApp(ctx, adminApp)
	require.True(t, trace.IsAccessDenied(err))

	// ... but can update dev app.
	err = devClt.UpdateApp(ctx, devApp)
	require.NoError(t, err)

	// Dev shouldn't be able to update labels on the prod app.
	adminApp.SetStaticLabels(map[string]string{"env": "dev", types.OriginLabel: types.OriginDynamic})
	err = devClt.UpdateApp(ctx, adminApp)
	require.True(t, trace.IsAccessDenied(err))
	adminApp.SetStaticLabels(map[string]string{"env": "prod", types.OriginLabel: types.OriginDynamic}) // Reset.

	// Dev shouldn't be able to get prod app...
	_, err = devClt.GetApp(ctx, adminApp.GetName())
	require.True(t, trace.IsAccessDenied(err))

	// ... but can get dev app.
	app, err := devClt.GetApp(ctx, devApp.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(devApp, app,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	// Admin can get both apps.
	app, err = adminClt.GetApp(ctx, adminApp.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(adminApp, app,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))
	app, err = adminClt.GetApp(ctx, devApp.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(devApp, app,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	// When listing apps, dev should only see one.
	apps, err := devClt.GetApps(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.Application{devApp}, apps,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	// Admin should see both.
	apps, err = adminClt.GetApps(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.Application{adminApp, devApp}, apps,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	// Dev shouldn't be able to delete dev app...
	err = devClt.DeleteApp(ctx, adminApp.GetName())
	require.True(t, trace.IsAccessDenied(err))

	// ... but can delete dev app.
	err = devClt.DeleteApp(ctx, devApp.GetName())
	require.NoError(t, err)

	// Admin should be able to delete admin app.
	err = adminClt.DeleteApp(ctx, adminApp.GetName())
	require.NoError(t, err)

	// Create both apps again to test "delete all" functionality.
	require.NoError(t, devClt.CreateApp(ctx, devApp))
	require.NoError(t, adminClt.CreateApp(ctx, adminApp))

	// Dev should only be able to delete dev app.
	err = devClt.DeleteAllApps(ctx)
	require.NoError(t, err)
	apps, err = adminClt.GetApps(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.Application{adminApp}, apps,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	// Admin should be able to delete all.
	err = adminClt.DeleteAllApps(ctx)
	require.NoError(t, err)
	apps, err = adminClt.GetApps(ctx)
	require.NoError(t, err)
	require.Empty(t, apps)
}

// TestReplaceRemoteLocksRBAC verifies that only a remote proxy may replace the
// remote locks associated with its cluster.
func TestReplaceRemoteLocksRBAC(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv, err := NewTestAuthServer(TestAuthServerConfig{Dir: t.TempDir()})
	require.NoError(t, err)

	user, _, err := CreateUserAndRole(srv.AuthServer, "test-user", []string{}, nil)
	require.NoError(t, err)

	targetCluster := "cluster"
	tests := []struct {
		desc     string
		identity TestIdentity
		checkErr func(error) bool
	}{
		{
			desc:     "users may not replace remote locks",
			identity: TestUser(user.GetName()),
			checkErr: trace.IsAccessDenied,
		},
		{
			desc:     "local proxy may not replace remote locks",
			identity: TestBuiltin(types.RoleProxy),
			checkErr: trace.IsAccessDenied,
		},
		{
			desc:     "remote proxy of a non-target cluster may not replace the target's remote locks",
			identity: TestRemoteBuiltin(types.RoleProxy, "non-"+targetCluster),
			checkErr: trace.IsAccessDenied,
		},
		{
			desc:     "remote proxy of the target cluster may replace its remote locks",
			identity: TestRemoteBuiltin(types.RoleProxy, targetCluster),
			checkErr: func(err error) bool { return err == nil },
		},
	}

	lock, err := types.NewLock("test-lock", types.LockSpecV2{Target: types.LockTarget{User: "test-user"}})
	require.NoError(t, err)

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			authContext, err := srv.Authorizer.Authorize(authz.ContextWithUser(ctx, test.identity.I))
			require.NoError(t, err)

			s := &ServerWithRoles{
				authServer: srv.AuthServer,
				alog:       srv.AuditLog,
				context:    *authContext,
			}

			err = s.ReplaceRemoteLocks(ctx, targetCluster, []types.Lock{lock})
			require.True(t, test.checkErr(err), trace.DebugReport(err))
		})
	}
}

// TestIsMFARequired_databaseProtocols tests the MFA requirement logic per
// database protocol where different role matchers are used.
func TestIsMFARequired_databaseProtocols(t *testing.T) {
	const (
		databaseName = "test-database"
		userName     = "test-username"
	)

	type modifyRoleFunc func(role types.Role)
	tests := []struct {
		name           string
		modifyRoleFunc modifyRoleFunc
		dbProtocol     string
		req            *proto.IsMFARequiredRequest
		want           proto.MFARequired
	}{
		{
			name:       "RequireSessionMFA on MySQL protocol doesn't match database name",
			dbProtocol: defaults.ProtocolMySQL,
			req: &proto.IsMFARequiredRequest{
				Target: &proto.IsMFARequiredRequest_Database{
					Database: &proto.RouteToDatabase{
						ServiceName: databaseName,
						Protocol:    defaults.ProtocolMySQL,
						Username:    userName,
						Database:    "example",
					},
				},
			},
			modifyRoleFunc: func(role types.Role) {
				roleOpt := role.GetOptions()
				roleOpt.RequireMFAType = types.RequireMFAType_SESSION
				role.SetOptions(roleOpt)

				role.SetDatabaseUsers(types.Allow, []string{types.Wildcard})
				role.SetDatabaseLabels(types.Allow, types.Labels{types.Wildcard: {types.Wildcard}})
				role.SetDatabaseNames(types.Allow, nil)
			},
			want: proto.MFARequired_MFA_REQUIRED_YES,
		},
		{
			name:       "RequireSessionMFA off",
			dbProtocol: defaults.ProtocolMySQL,
			req: &proto.IsMFARequiredRequest{
				Target: &proto.IsMFARequiredRequest_Database{
					Database: &proto.RouteToDatabase{
						ServiceName: databaseName,
						Protocol:    defaults.ProtocolMySQL,
						Username:    userName,
						Database:    "example",
					},
				},
			},
			modifyRoleFunc: func(role types.Role) {
				roleOpt := role.GetOptions()
				roleOpt.RequireMFAType = types.RequireMFAType_OFF
				role.SetOptions(roleOpt)

				role.SetDatabaseUsers(types.Allow, []string{types.Wildcard})
				role.SetDatabaseLabels(types.Allow, types.Labels{types.Wildcard: {types.Wildcard}})
				role.SetDatabaseNames(types.Allow, nil)
			},
			want: proto.MFARequired_MFA_REQUIRED_NO,
		},
		{
			name:       "RequireSessionMFA on Postgres protocol database name doesn't match",
			dbProtocol: defaults.ProtocolPostgres,
			req: &proto.IsMFARequiredRequest{
				Target: &proto.IsMFARequiredRequest_Database{
					Database: &proto.RouteToDatabase{
						ServiceName: databaseName,
						Protocol:    defaults.ProtocolPostgres,
						Username:    userName,
						Database:    "example",
					},
				},
			},
			modifyRoleFunc: func(role types.Role) {
				roleOpt := role.GetOptions()
				roleOpt.RequireMFAType = types.RequireMFAType_SESSION
				role.SetOptions(roleOpt)

				role.SetDatabaseUsers(types.Allow, []string{types.Wildcard})
				role.SetDatabaseLabels(types.Allow, types.Labels{types.Wildcard: {types.Wildcard}})
				role.SetDatabaseNames(types.Allow, nil)
			},
			want: proto.MFARequired_MFA_REQUIRED_NO,
		},
		{
			name:       "RequireSessionMFA on Postgres protocol database name matches",
			dbProtocol: defaults.ProtocolPostgres,
			req: &proto.IsMFARequiredRequest{
				Target: &proto.IsMFARequiredRequest_Database{
					Database: &proto.RouteToDatabase{
						ServiceName: databaseName,
						Protocol:    defaults.ProtocolPostgres,
						Username:    userName,
						Database:    "example",
					},
				},
			},
			modifyRoleFunc: func(role types.Role) {
				roleOpt := role.GetOptions()
				roleOpt.RequireMFAType = types.RequireMFAType_SESSION
				role.SetOptions(roleOpt)

				role.SetDatabaseUsers(types.Allow, []string{types.Wildcard})
				role.SetDatabaseLabels(types.Allow, types.Labels{types.Wildcard: {types.Wildcard}})
				role.SetDatabaseNames(types.Allow, []string{"example"})
			},
			want: proto.MFARequired_MFA_REQUIRED_YES,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			srv := newTestTLSServer(t)

			// Enable MFA support.
			authPref, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
				Type:         constants.Local,
				SecondFactor: constants.SecondFactorOptional,
				Webauthn: &types.Webauthn{
					RPID: "teleport",
				},
			})
			require.NoError(t, err)
			_, err = srv.Auth().UpsertAuthPreference(ctx, authPref)
			require.NoError(t, err)

			db, err := types.NewDatabaseV3(
				types.Metadata{
					Name: databaseName,
				},
				types.DatabaseSpecV3{
					Protocol: tc.dbProtocol,
					URI:      "example.com",
				},
			)
			require.NoError(t, err)

			database, err := types.NewDatabaseServerV3(
				types.Metadata{
					Name: databaseName,
					Labels: map[string]string{
						"env": "dev",
					},
				},
				types.DatabaseServerSpecV3{
					Database: db,
					Hostname: "host",
					HostID:   "hostID",
				},
			)
			require.NoError(t, err)

			_, err = srv.Auth().UpsertDatabaseServer(ctx, database)
			require.NoError(t, err)

			user, role, err := CreateUserAndRole(srv.Auth(), userName, []string{"test-role"}, nil)
			require.NoError(t, err)

			if tc.modifyRoleFunc != nil {
				tc.modifyRoleFunc(role)
			}
			_, err = srv.Auth().UpsertRole(ctx, role)
			require.NoError(t, err)

			cl, err := srv.NewClient(TestUser(user.GetName()))
			require.NoError(t, err)

			resp, err := cl.IsMFARequired(ctx, tc.req)
			require.NoError(t, err)
			assert.Equal(t, tc.want, resp.MFARequired, "MFARequired mismatch")
			assert.Equal(t, MFARequiredToBool(tc.want), resp.Required, "Required mismatch")
		})
	}
}

// TestKindClusterConfig verifies that types.KindClusterConfig can be used
// as an alternative privilege to provide access to cluster configuration
// resources.
func TestKindClusterConfig(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv, err := NewTestAuthServer(TestAuthServerConfig{Dir: t.TempDir()})
	require.NoError(t, err)

	getClusterConfigResources := func(ctx context.Context, user types.User) []error {
		authContext, err := srv.Authorizer.Authorize(authz.ContextWithUser(ctx, TestUser(user.GetName()).I))
		require.NoError(t, err, trace.DebugReport(err))
		s := &ServerWithRoles{
			authServer: srv.AuthServer,
			alog:       srv.AuditLog,
			context:    *authContext,
		}
		_, err1 := s.GetClusterAuditConfig(ctx)
		_, err2 := s.GetClusterNetworkingConfig(ctx)
		_, err3 := s.GetSessionRecordingConfig(ctx)
		return []error{err1, err2, err3}
	}

	t.Run("without KindClusterConfig privilege", func(t *testing.T) {
		user, err := CreateUser(ctx, srv.AuthServer, "test-user")
		require.NoError(t, err)
		for _, err := range getClusterConfigResources(ctx, user) {
			require.Error(t, err)
			require.True(t, trace.IsAccessDenied(err))
		}
	})

	t.Run("with KindClusterConfig privilege", func(t *testing.T) {
		role, err := types.NewRole("test-role", types.RoleSpecV6{
			Allow: types.RoleConditions{
				Rules: []types.Rule{
					types.NewRule(types.KindClusterConfig, []string{types.VerbRead}),
				},
			},
		})
		require.NoError(t, err)
		user, err := CreateUser(ctx, srv.AuthServer, "test-user", role)
		require.NoError(t, err)
		for _, err := range getClusterConfigResources(ctx, user) {
			require.NoError(t, err)
		}
	})
}

func TestGetAndList_KubernetesServers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	// Create test kube servers.
	for i := 0; i < 5; i++ {
		// insert legacy kube servers
		name := uuid.NewString()
		cluster, err := types.NewKubernetesClusterV3(
			types.Metadata{
				Name: name, Labels: map[string]string{"name": name},
			},
			types.KubernetesClusterSpecV3{},
		)
		require.NoError(t, err)

		kubeServer, err := types.NewKubernetesServerV3(
			types.Metadata{
				Name: name, Labels: map[string]string{"name": name},
			},

			types.KubernetesServerSpecV3{
				HostID:   name,
				Hostname: "test",
				Cluster:  cluster,
			},
		)
		require.NoError(t, err)

		_, err = srv.Auth().UpsertKubernetesServer(ctx, kubeServer)
		require.NoError(t, err)
	}

	testServers, err := srv.Auth().GetKubernetesServers(ctx)
	require.NoError(t, err)
	require.Len(t, testServers, 5)

	testResources := make([]types.ResourceWithLabels, len(testServers))
	for i, server := range testServers {
		testResources[i] = server
	}

	// create user, role, and client
	username := "user"
	user, role, err := CreateUserAndRole(srv.Auth(), username, nil, nil)
	require.NoError(t, err)
	identity := TestUser(user.GetName())
	clt, err := srv.NewClient(identity)
	require.NoError(t, err)

	listRequest := proto.ListResourcesRequest{
		Namespace: apidefaults.Namespace,
		// Guarantee that the list will all the servers.
		Limit:        int32(len(testServers) + 1),
		ResourceType: types.KindKubeServer,
	}

	// permit user to get all kubernetes service
	role.SetKubernetesLabels(types.Allow, types.Labels{types.Wildcard: {types.Wildcard}})
	_, err = srv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)
	servers, err := clt.GetKubernetesServers(ctx)
	require.NoError(t, err)
	require.Len(t, testServers, len(testServers))
	require.Empty(t, cmp.Diff(testServers, servers))
	resp, err := clt.ListResources(ctx, listRequest)
	require.NoError(t, err)
	require.Len(t, resp.Resources, len(testResources))
	require.Empty(t, cmp.Diff(testResources, resp.Resources))

	// Test various filtering.
	baseRequest := proto.ListResourcesRequest{
		Namespace:    apidefaults.Namespace,
		Limit:        int32(len(testServers) + 1),
		ResourceType: types.KindKubeServer,
	}

	// Test label match.
	withLabels := baseRequest
	withLabels.Labels = map[string]string{"name": testServers[0].GetName()}
	resp, err = clt.ListResources(ctx, withLabels)
	require.NoError(t, err)
	require.Len(t, resp.Resources, 1)
	require.Empty(t, cmp.Diff(testResources[0:1], resp.Resources))

	// Test search keywords match.
	withSearchKeywords := baseRequest
	withSearchKeywords.SearchKeywords = []string{"name", testServers[0].GetName()}
	resp, err = clt.ListResources(ctx, withSearchKeywords)
	require.NoError(t, err)
	require.Len(t, resp.Resources, 1)
	require.Empty(t, cmp.Diff(testResources[0:1], resp.Resources))

	// Test expression match.
	withExpression := baseRequest
	withExpression.PredicateExpression = fmt.Sprintf(`labels.name == "%s"`, testServers[0].GetName())
	resp, err = clt.ListResources(ctx, withExpression)
	require.NoError(t, err)
	require.Len(t, resp.Resources, 1)
	require.Empty(t, cmp.Diff(testResources[0:1], resp.Resources))

	// deny user to get the first kubernetes service
	role.SetKubernetesLabels(types.Deny, types.Labels{"name": {testServers[0].GetName()}})
	_, err = srv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)
	servers, err = clt.GetKubernetesServers(ctx)
	require.NoError(t, err)
	require.Len(t, servers, len(testServers)-1)
	require.Empty(t, cmp.Diff(testServers[1:], servers))
	resp, err = clt.ListResources(ctx, listRequest)
	require.NoError(t, err)
	require.Len(t, resp.Resources, len(testResources)-1)
	require.Empty(t, cmp.Diff(testResources[1:], resp.Resources))

	// deny user to get all databases
	role.SetKubernetesLabels(types.Deny, types.Labels{types.Wildcard: {types.Wildcard}})
	_, err = srv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)
	servers, err = clt.GetKubernetesServers(ctx)
	require.NoError(t, err)
	require.Empty(t, servers)
	resp, err = clt.ListResources(ctx, listRequest)
	require.NoError(t, err)
	require.Empty(t, resp.Resources)
}

func TestListDatabaseServices(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	numInitialResources := 5

	// Create test Database Services.
	for i := 0; i < numInitialResources; i++ {
		name := uuid.NewString()
		s, err := types.NewDatabaseServiceV1(types.Metadata{
			Name: name,
		}, types.DatabaseServiceSpecV1{
			ResourceMatchers: []*types.DatabaseResourceMatcher{
				{
					Labels: &types.Labels{
						"env": []string{name},
					},
				},
			},
		})
		require.NoError(t, err)

		_, err = srv.Auth().UpsertDatabaseService(ctx, s)
		require.NoError(t, err)
	}

	listServicesResp, err := srv.Auth().ListResources(ctx,
		proto.ListResourcesRequest{
			ResourceType: types.KindDatabaseService,
			Limit:        apidefaults.DefaultChunkSize,
		},
	)
	require.NoError(t, err)
	databaseServices, err := types.ResourcesWithLabels(listServicesResp.Resources).AsDatabaseServices()
	require.NoError(t, err)

	testResources := make([]types.ResourceWithLabels, len(databaseServices))
	for i, server := range databaseServices {
		testResources[i] = server
	}

	// Create user, role, and client
	username := "user"
	user, role, err := CreateUserAndRole(srv.Auth(), username, nil, nil)
	require.NoError(t, err)
	identity := TestUser(user.GetName())
	clt, err := srv.NewClient(identity)
	require.NoError(t, err)

	// User is not allowed to list DatabseServices
	_, err = clt.ListResources(ctx,
		proto.ListResourcesRequest{
			ResourceType: types.KindDatabaseService,
			Limit:        apidefaults.DefaultChunkSize,
		},
	)
	require.True(t, trace.IsAccessDenied(err), "expected access denied because role does not allow Read operations")

	// Change the user's role to allow them to list DatabaseServices
	currentAllowRules := role.GetRules(types.Allow)
	role.SetRules(types.Allow, append(currentAllowRules, types.NewRule(types.KindDatabaseService, services.RO())))
	role.SetDatabaseServiceLabels(types.Allow, types.Labels{types.Wildcard: []string{types.Wildcard}})
	_, err = srv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)

	listServicesResp, err = clt.ListResources(ctx,
		proto.ListResourcesRequest{
			ResourceType: types.KindDatabaseService,
			Limit:        apidefaults.DefaultChunkSize,
		},
	)
	require.NoError(t, err)
	usersViewDBServices, err := types.ResourcesWithLabels(listServicesResp.Resources).AsDatabaseServices()
	require.NoError(t, err)
	require.Len(t, usersViewDBServices, numInitialResources)

	require.Empty(t, cmp.Diff(databaseServices, usersViewDBServices))

	// User is not allowed to Upsert DatabaseServices
	extraDatabaseService, err := types.NewDatabaseServiceV1(types.Metadata{
		Name: "extra",
	}, types.DatabaseServiceSpecV1{
		ResourceMatchers: []*types.DatabaseResourceMatcher{
			{
				Labels: &types.Labels{
					"env": []string{"extra"},
				},
			},
		},
	})
	require.NoError(t, err)
	_, err = clt.UpsertDatabaseService(ctx, extraDatabaseService)
	require.True(t, trace.IsAccessDenied(err), "expected access denied because role does not allow Create/Update operations")

	currentAllowRules = role.GetRules(types.Allow)
	role.SetRules(types.Allow, append(currentAllowRules, types.NewRule(types.KindDatabaseService, services.RW())))
	_, err = srv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)

	_, err = clt.UpsertDatabaseService(ctx, extraDatabaseService)
	require.NoError(t, err)

	listServicesResp, err = clt.ListResources(ctx,
		proto.ListResourcesRequest{
			ResourceType: types.KindDatabaseService,
			Limit:        apidefaults.DefaultChunkSize,
		},
	)
	require.NoError(t, err)
	usersViewDBServices, err = types.ResourcesWithLabels(listServicesResp.Resources).AsDatabaseServices()
	require.NoError(t, err)
	require.Len(t, usersViewDBServices, numInitialResources+1)

	// User can also delete a single or multiple DatabaseServices because they have RW permissions now
	err = clt.DeleteDatabaseService(ctx, extraDatabaseService.GetName())
	require.NoError(t, err)

	listServicesResp, err = clt.ListResources(ctx,
		proto.ListResourcesRequest{
			ResourceType: types.KindDatabaseService,
			Limit:        apidefaults.DefaultChunkSize,
		},
	)
	require.NoError(t, err)
	usersViewDBServices, err = types.ResourcesWithLabels(listServicesResp.Resources).AsDatabaseServices()
	require.NoError(t, err)
	require.Len(t, usersViewDBServices, numInitialResources)

	// After removing all resources, we should have 0 resources being returned.
	err = clt.DeleteAllDatabaseServices(ctx)
	require.NoError(t, err)

	listServicesResp, err = clt.ListResources(ctx,
		proto.ListResourcesRequest{
			ResourceType: types.KindDatabaseService,
			Limit:        apidefaults.DefaultChunkSize,
		},
	)
	require.NoError(t, err)
	usersViewDBServices, err = types.ResourcesWithLabels(listServicesResp.Resources).AsDatabaseServices()
	require.NoError(t, err)
	require.Empty(t, usersViewDBServices)
}

func TestListResources_NeedTotalCountFlag(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	// Create test nodes.
	for i := 0; i < 3; i++ {
		name := uuid.New().String()
		node, err := types.NewServerWithLabels(
			name,
			types.KindNode,
			types.ServerSpecV2{},
			map[string]string{"name": name},
		)
		require.NoError(t, err)

		_, err = srv.Auth().UpsertNode(ctx, node)
		require.NoError(t, err)
	}

	testNodes, err := srv.Auth().GetNodes(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Len(t, testNodes, 3)

	// create user and client
	user, _, err := CreateUserAndRole(srv.Auth(), "user", nil, nil)
	require.NoError(t, err)
	clt, err := srv.NewClient(TestUser(user.GetName()))
	require.NoError(t, err)

	// Total returned.
	resp, err := clt.ListResources(ctx, proto.ListResourcesRequest{
		ResourceType:   types.KindNode,
		Limit:          2,
		NeedTotalCount: true,
	})
	require.NoError(t, err)
	require.Len(t, resp.Resources, 2)
	require.NotEmpty(t, resp.NextKey)
	require.Len(t, testNodes, resp.TotalCount)

	// No total.
	resp, err = clt.ListResources(ctx, proto.ListResourcesRequest{
		ResourceType: types.KindNode,
		Limit:        2,
	})
	require.NoError(t, err)
	require.Len(t, resp.Resources, 2)
	require.NotEmpty(t, resp.NextKey)
	require.Empty(t, resp.TotalCount)
}

func TestListResources_SearchAsRoles(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	// Create test nodes.
	const numTestNodes = 3
	for i := 0; i < numTestNodes; i++ {
		name := fmt.Sprintf("node%d", i)
		node, err := types.NewServerWithLabels(
			name,
			types.KindNode,
			types.ServerSpecV2{},
			map[string]string{"name": name},
		)
		require.NoError(t, err)

		_, err = srv.Auth().UpsertNode(ctx, node)
		require.NoError(t, err)
	}

	testNodes, err := srv.Auth().GetNodes(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Len(t, testNodes, numTestNodes)

	// create user and client
	requester, role, err := CreateUserAndRole(srv.Auth(), "requester", []string{"requester"}, nil)
	require.NoError(t, err)

	// only allow user to see first node
	role.SetNodeLabels(types.Allow, types.Labels{"name": {testNodes[0].GetName()}})

	// create a new role which can see second node
	searchAsRole := services.RoleForUser(requester)
	searchAsRole.SetName("test_search_role")
	searchAsRole.SetNodeLabels(types.Allow, types.Labels{"name": {testNodes[1].GetName()}})
	searchAsRole.SetLogins(types.Allow, []string{"requester"})
	_, err = srv.Auth().UpsertRole(ctx, searchAsRole)
	require.NoError(t, err)

	// create a third role which can see the third node
	previewAsRole := services.RoleForUser(requester)
	previewAsRole.SetName("test_preview_role")
	previewAsRole.SetNodeLabels(types.Allow, types.Labels{"name": {testNodes[2].GetName()}})
	previewAsRole.SetLogins(types.Allow, []string{"requester"})
	_, err = srv.Auth().UpsertRole(ctx, previewAsRole)
	require.NoError(t, err)

	role.SetSearchAsRoles(types.Allow, []string{searchAsRole.GetName()})
	role.SetPreviewAsRoles(types.Allow, []string{previewAsRole.GetName()})
	_, err = srv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)

	requesterClt, err := srv.NewClient(TestUser(requester.GetName()))
	require.NoError(t, err)

	// create another user that can see all nodes but has no search_as_roles or
	// preview_as_roles
	admin, _, err := CreateUserAndRole(srv.Auth(), "admin", []string{"admin"}, nil)
	require.NoError(t, err)
	adminClt, err := srv.NewClient(TestUser(admin.GetName()))
	require.NoError(t, err)

	for _, tc := range []struct {
		desc                   string
		clt                    *authclient.Client
		requestOpt             func(*proto.ListResourcesRequest)
		expectNodes            []string
		expectSearchEvent      bool
		expectSearchEventRoles []string
	}{
		{
			desc:        "no search",
			clt:         requesterClt,
			expectNodes: []string{testNodes[0].GetName()},
		},
		{
			desc: "search as roles",
			clt:  requesterClt,
			requestOpt: func(req *proto.ListResourcesRequest) {
				req.UseSearchAsRoles = true
			},
			expectNodes:            []string{testNodes[0].GetName(), testNodes[1].GetName()},
			expectSearchEventRoles: []string{role.GetName(), searchAsRole.GetName()},
		},
		{
			desc: "preview as roles",
			clt:  requesterClt,
			requestOpt: func(req *proto.ListResourcesRequest) {
				req.UsePreviewAsRoles = true
			},
			expectNodes:            []string{testNodes[0].GetName(), testNodes[2].GetName()},
			expectSearchEventRoles: []string{role.GetName(), previewAsRole.GetName()},
		},
		{
			desc: "both",
			clt:  requesterClt,
			requestOpt: func(req *proto.ListResourcesRequest) {
				req.UseSearchAsRoles = true
				req.UsePreviewAsRoles = true
			},
			expectNodes:            []string{testNodes[0].GetName(), testNodes[1].GetName(), testNodes[2].GetName()},
			expectSearchEventRoles: []string{role.GetName(), searchAsRole.GetName(), previewAsRole.GetName()},
		},
		{
			// this tests the case where the request includes UseSearchAsRoles
			// and UsePreviewAsRoles, but the user has none
			desc: "no extra roles",
			clt:  adminClt,
			requestOpt: func(req *proto.ListResourcesRequest) {
				req.UseSearchAsRoles = true
				req.UsePreviewAsRoles = true
			},
			expectNodes: []string{testNodes[0].GetName(), testNodes[1].GetName(), testNodes[2].GetName()},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			req := proto.ListResourcesRequest{
				ResourceType: types.KindNode,
				Limit:        int32(len(testNodes)),
			}
			if tc.requestOpt != nil {
				tc.requestOpt(&req)
			}
			resp, err := tc.clt.ListResources(ctx, req)
			require.NoError(t, err)
			require.Len(t, resp.Resources, len(tc.expectNodes))
			var gotNodes []string
			for _, node := range resp.Resources {
				gotNodes = append(gotNodes, node.GetName())
			}
			require.ElementsMatch(t, tc.expectNodes, gotNodes)
		})
	}
}

// TestListResources_WithLogins will generate multiple resources
// and get them with login information.
func TestListResources_WithLogins(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t, withCacheEnabled(true))

	require.Eventually(t, func() bool {
		return srv.Auth().UnifiedResourceCache.IsInitialized()
	}, 5*time.Second, 200*time.Millisecond, "unified resource watcher never initialized")

	for i := 0; i < 5; i++ {
		name := uuid.New().String()
		node, err := types.NewServerWithLabels(
			name,
			types.KindNode,
			types.ServerSpecV2{},
			map[string]string{"name": name},
		)
		require.NoError(t, err)

		_, err = srv.Auth().UpsertNode(ctx, node)
		require.NoError(t, err)

		db, err := types.NewDatabaseServerV3(types.Metadata{
			Name: name,
		}, types.DatabaseServerSpecV3{
			HostID:   "_",
			Hostname: "_",
			Database: &types.DatabaseV3{
				Metadata: types.Metadata{
					Name: fmt.Sprintf("name-%d", i),
				},
				Spec: types.DatabaseSpecV3{
					Protocol: "_",
					URI:      "_",
				},
			},
		})
		require.NoError(t, err)

		_, err = srv.Auth().UpsertDatabaseServer(ctx, db)
		require.NoError(t, err)

		desktop, err := types.NewWindowsDesktopV3(name, nil, types.WindowsDesktopSpecV3{
			HostID: strconv.Itoa(i),
			Addr:   "1.2.3.4",
		})
		require.NoError(t, err)

		require.NoError(t, srv.Auth().UpsertWindowsDesktop(ctx, desktop))

		awsApp, err := types.NewAppServerV3(types.Metadata{Name: name}, types.AppServerSpecV3{
			HostID:   "_",
			Hostname: "_",
			App: &types.AppV3{
				Metadata: types.Metadata{Name: fmt.Sprintf("name-%d", i)},
				Spec: types.AppSpecV3{
					URI: "https://console.aws.amazon.com/ec2/v2/home",
				},
			},
		})
		require.NoError(t, err)

		_, err = srv.Auth().UpsertApplicationServer(ctx, awsApp)
		require.NoError(t, err)
	}

	// create user and client
	logins := []string{"llama", "fish"}
	user, role, err := CreateUserAndRole(srv.Auth(), "user", logins, nil)
	require.NoError(t, err)
	role.SetWindowsDesktopLabels(types.Allow, types.Labels{types.Wildcard: []string{types.Wildcard}})
	role.SetWindowsLogins(types.Allow, logins)
	role.SetAppLabels(types.Allow, types.Labels{types.Wildcard: []string{types.Wildcard}})
	role.SetAWSRoleARNs(types.Allow, logins)
	_, err = srv.Auth().UpdateRole(ctx, role)
	require.NoError(t, err)

	clt, err := srv.NewClient(TestUser(user.GetName()))
	require.NoError(t, err)

	t.Run("with fake pagination", func(t *testing.T) {
		for _, resourceType := range []string{types.KindNode, types.KindWindowsDesktop, types.KindDatabaseServer, types.KindAppServer} {
			var results []*types.EnrichedResource
			var start string

			for len(results) != 5 {
				resp, err := apiclient.GetEnrichedResourcePage(ctx, clt, &proto.ListResourcesRequest{
					ResourceType:  resourceType,
					Limit:         2,
					IncludeLogins: true,
					SortBy:        types.SortBy{IsDesc: true, Field: types.ResourceMetadataName},
					StartKey:      start,
				})
				require.NoError(t, err)

				results = append(results, resp.Resources...)
				start = resp.NextKey
			}

			// Check that only server, desktop, and app server resources contain the expected logins
			for _, resource := range results {
				switch resource.ResourceWithLabels.(type) {
				case types.Server, types.WindowsDesktop, types.AppServer:
					require.Empty(t, cmp.Diff(resource.Logins, logins, cmpopts.SortSlices(func(a, b string) bool {
						return strings.Compare(a, b) < 0
					})), "mismatch on expected logins list for resource %T", resource.ResourceWithLabels)
				default:
					require.Empty(t, resource.Logins, "expected resource %T to get empty list of logins but got %s", resource.ResourceWithLabels, resource.Logins)
				}
			}
		}
	})

	t.Run("without fake pagination", func(t *testing.T) {
		for _, resourceType := range []string{types.KindNode, types.KindWindowsDesktop, types.KindDatabaseServer, types.KindAppServer} {
			var results []*types.EnrichedResource
			var start string

			for len(results) != 5 {
				resp, err := apiclient.GetEnrichedResourcePage(ctx, clt, &proto.ListResourcesRequest{
					ResourceType:  resourceType,
					Limit:         2,
					IncludeLogins: true,
					StartKey:      start,
				})
				require.NoError(t, err)

				results = append(results, resp.Resources...)
				start = resp.NextKey
			}

			// Check that only server, desktop, and app server resources contain the expected logins
			for _, resource := range results {
				switch resource.ResourceWithLabels.(type) {
				case types.Server, types.WindowsDesktop, types.AppServer:
					require.Empty(t, cmp.Diff(resource.Logins, logins, cmpopts.SortSlices(func(a, b string) bool {
						return strings.Compare(a, b) < 0
					})), "mismatch on expected logins list for resource %T", resource.ResourceWithLabels)
				default:
					require.Empty(t, resource.Logins, "expected resource %T to get empty list of logins but got %s", resource.ResourceWithLabels, resource.Logins)
				}
			}
		}
	})
}

func TestGetAndList_WindowsDesktops(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	// Create test desktops.
	for i := 0; i < 5; i++ {
		name := uuid.New().String()
		desktop, err := types.NewWindowsDesktopV3(name, map[string]string{"name": name},
			types.WindowsDesktopSpecV3{Addr: "_", HostID: "_"})
		require.NoError(t, err)
		require.NoError(t, srv.Auth().UpsertWindowsDesktop(ctx, desktop))
	}

	// Test all has been upserted.
	testDesktops, err := srv.Auth().GetWindowsDesktops(ctx, types.WindowsDesktopFilter{})
	require.NoError(t, err)
	require.Len(t, testDesktops, 5)

	testResources := types.WindowsDesktops(testDesktops).AsResources()

	// Create user, role, and client.
	username := "user"
	user, role, err := CreateUserAndRole(srv.Auth(), username, nil, nil)
	require.NoError(t, err)
	identity := TestUser(user.GetName())
	clt, err := srv.NewClient(identity)
	require.NoError(t, err)

	// Base request.
	listRequest := proto.ListResourcesRequest{
		ResourceType: types.KindWindowsDesktop,
		Limit:        int32(len(testDesktops) + 1),
	}

	// Permit user to get the first desktop.
	role.SetWindowsDesktopLabels(types.Allow, types.Labels{"name": {testDesktops[0].GetName()}})
	_, err = srv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)

	desktops, err := clt.GetWindowsDesktops(ctx, types.WindowsDesktopFilter{})
	require.NoError(t, err)
	require.Len(t, desktops, 1)
	require.Empty(t, cmp.Diff(testDesktops[0:1], desktops))

	resp, err := clt.ListResources(ctx, listRequest)
	require.NoError(t, err)
	require.Len(t, resp.Resources, 1)
	require.Empty(t, cmp.Diff(testResources[0:1], resp.Resources))
	require.Empty(t, resp.NextKey)
	require.Empty(t, resp.TotalCount)

	// Permit user to get all desktops.
	role.SetWindowsDesktopLabels(types.Allow, types.Labels{types.Wildcard: {types.Wildcard}})
	_, err = srv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)
	desktops, err = clt.GetWindowsDesktops(ctx, types.WindowsDesktopFilter{})
	require.NoError(t, err)
	require.EqualValues(t, len(testDesktops), len(desktops))
	require.Empty(t, cmp.Diff(testDesktops, desktops))

	resp, err = clt.ListResources(ctx, listRequest)
	require.NoError(t, err)
	require.Len(t, resp.Resources, len(testResources))
	require.Empty(t, cmp.Diff(testResources, resp.Resources))
	require.Empty(t, resp.NextKey)
	require.Empty(t, resp.TotalCount)

	// Test sorting is supported.
	withSort := listRequest
	withSort.SortBy = types.SortBy{IsDesc: true, Field: types.ResourceMetadataName}
	resp, err = clt.ListResources(ctx, withSort)
	require.NoError(t, err)
	require.Len(t, resp.Resources, len(testResources))
	desktops, err = types.ResourcesWithLabels(resp.Resources).AsWindowsDesktops()
	require.NoError(t, err)
	names, err := types.WindowsDesktops(desktops).GetFieldVals(types.ResourceMetadataName)
	require.NoError(t, err)
	require.IsDecreasing(t, names)

	// Filter by labels.
	withLabels := listRequest
	withLabels.Labels = map[string]string{"name": testDesktops[0].GetName()}
	resp, err = clt.ListResources(ctx, withLabels)
	require.NoError(t, err)
	require.Len(t, resp.Resources, 1)
	require.Empty(t, cmp.Diff(testResources[0:1], resp.Resources))
	require.Empty(t, resp.NextKey)
	require.Empty(t, resp.TotalCount)

	// Test search keywords match.
	withSearchKeywords := listRequest
	withSearchKeywords.SearchKeywords = []string{"name", testDesktops[0].GetName()}
	resp, err = clt.ListResources(ctx, withSearchKeywords)
	require.NoError(t, err)
	require.Len(t, resp.Resources, 1)
	require.Empty(t, cmp.Diff(testResources[0:1], resp.Resources))

	// Test predicate match.
	withExpression := listRequest
	withExpression.PredicateExpression = fmt.Sprintf(`labels.name == "%s"`, testDesktops[0].GetName())
	resp, err = clt.ListResources(ctx, withExpression)
	require.NoError(t, err)
	require.Len(t, resp.Resources, 1)
	require.Empty(t, cmp.Diff(testResources[0:1], resp.Resources))

	// Deny user to get the first desktop.
	role.SetWindowsDesktopLabels(types.Deny, types.Labels{"name": {testDesktops[0].GetName()}})
	_, err = srv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)

	desktops, err = clt.GetWindowsDesktops(ctx, types.WindowsDesktopFilter{})
	require.NoError(t, err)
	require.EqualValues(t, len(testDesktops[1:]), len(desktops))
	require.Empty(t, cmp.Diff(testDesktops[1:], desktops))

	resp, err = clt.ListResources(ctx, listRequest)
	require.NoError(t, err)
	require.Len(t, resp.Resources, len(testResources[1:]))
	require.Empty(t, cmp.Diff(testResources[1:], resp.Resources))

	// Deny user all desktops.
	role.SetWindowsDesktopLabels(types.Deny, types.Labels{types.Wildcard: {types.Wildcard}})
	_, err = srv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)

	desktops, err = clt.GetWindowsDesktops(ctx, types.WindowsDesktopFilter{})
	require.NoError(t, err)
	require.Empty(t, desktops)
	require.Empty(t, cmp.Diff([]types.WindowsDesktop{}, desktops))

	resp, err = clt.ListResources(ctx, listRequest)
	require.NoError(t, err)
	require.Empty(t, resp.Resources)
}

func TestListResources_KindKubernetesCluster(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv, err := NewTestAuthServer(TestAuthServerConfig{Dir: t.TempDir()})
	require.NoError(t, err)

	authContext, err := srv.Authorizer.Authorize(authz.ContextWithUser(ctx, TestBuiltin(types.RoleProxy).I))
	require.NoError(t, err)

	s := &ServerWithRoles{
		authServer: srv.AuthServer,
		alog:       srv.AuditLog,
		context:    *authContext,
	}

	testNames := []string{"a", "b", "c", "d"}

	// Add a kube service with 3 clusters.
	createKubeServer(t, s, []string{"d", "b", "a"}, "host1")

	// Add a kube service with 2 clusters.
	// Includes a duplicate cluster name to test deduplicate.
	createKubeServer(t, s, []string{"a", "c"}, "host2")

	// Test upsert.
	kubeServers, err := s.GetKubernetesServers(ctx)
	require.NoError(t, err)
	require.Len(t, kubeServers, 5)

	t.Run("fetch all", func(t *testing.T) {
		t.Parallel()

		res, err := s.ListResources(ctx, proto.ListResourcesRequest{
			ResourceType: types.KindKubernetesCluster,
			Limit:        10,
		})
		require.NoError(t, err)
		require.Len(t, res.Resources, len(testNames))
		require.Empty(t, res.NextKey)
		// There is 2 kube services, but 4 unique clusters.
		require.Equal(t, 4, res.TotalCount)

		clusters, err := types.ResourcesWithLabels(res.Resources).AsKubeClusters()
		require.NoError(t, err)
		names, err := types.KubeClusters(clusters).GetFieldVals(types.ResourceMetadataName)
		require.NoError(t, err)
		require.ElementsMatch(t, names, testNames)
	})

	t.Run("start keys", func(t *testing.T) {
		t.Parallel()

		// First fetch.
		res, err := s.ListResources(ctx, proto.ListResourcesRequest{
			ResourceType: types.KindKubernetesCluster,
			Limit:        1,
		})
		require.NoError(t, err)
		require.Len(t, res.Resources, 1)
		require.Equal(t, kubeServers[1].GetCluster().GetName(), res.NextKey)

		// Second fetch.
		res, err = s.ListResources(ctx, proto.ListResourcesRequest{
			ResourceType: types.KindKubernetesCluster,
			Limit:        1,
			StartKey:     res.NextKey,
		})
		require.NoError(t, err)
		require.Len(t, res.Resources, 1)
		require.Equal(t, kubeServers[2].GetCluster().GetName(), res.NextKey)
	})

	t.Run("fetch with sort and total count", func(t *testing.T) {
		t.Parallel()
		res, err := s.ListResources(ctx, proto.ListResourcesRequest{
			ResourceType: types.KindKubernetesCluster,
			Limit:        10,
			SortBy: types.SortBy{
				IsDesc: true,
				Field:  types.ResourceMetadataName,
			},
			NeedTotalCount: true,
		})
		require.NoError(t, err)
		require.Empty(t, res.NextKey)
		require.Len(t, res.Resources, len(testNames))
		require.Len(t, testNames, res.TotalCount)

		clusters, err := types.ResourcesWithLabels(res.Resources).AsKubeClusters()
		require.NoError(t, err)
		names, err := types.KubeClusters(clusters).GetFieldVals(types.ResourceMetadataName)
		require.NoError(t, err)
		require.IsDecreasing(t, names)
	})
}

func createKubeServer(t *testing.T, s *ServerWithRoles, clusterNames []string, hostID string) {
	for _, clusterName := range clusterNames {
		kubeCluster, err := types.NewKubernetesClusterV3(types.Metadata{
			Name: clusterName,
		}, types.KubernetesClusterSpecV3{})
		require.NoError(t, err)
		kubeServer, err := types.NewKubernetesServerV3FromCluster(kubeCluster, hostID, hostID)
		require.NoError(t, err)
		_, err = s.UpsertKubernetesServer(context.Background(), kubeServer)
		require.NoError(t, err)
	}
}

func TestListResources_KindUserGroup(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv, err := NewTestAuthServer(TestAuthServerConfig{Dir: t.TempDir()})
	require.NoError(t, err)

	role, err := types.NewRole("test-role", types.RoleSpecV6{
		Allow: types.RoleConditions{
			GroupLabels: types.Labels{
				"label": []string{"value"},
			},
			Rules: []types.Rule{
				{
					Resources: []string{types.KindUserGroup},
					Verbs:     []string{types.VerbCreate, types.VerbRead, types.VerbList},
				},
			},
		},
	})
	require.NoError(t, err)
	_, err = srv.AuthServer.UpsertRole(ctx, role)
	require.NoError(t, err)

	user, err := types.NewUser("test-user")
	require.NoError(t, err)
	user.AddRole(role.GetName())
	user, err = srv.AuthServer.UpsertUser(ctx, user)
	require.NoError(t, err)

	// Create the admin context so that we can create all the user groups we need.
	authContext, err := srv.Authorizer.Authorize(authz.ContextWithUser(ctx, TestBuiltin(types.RoleAdmin).I))
	require.NoError(t, err)

	s := &ServerWithRoles{
		authServer: srv.AuthServer,
		alog:       srv.AuditLog,
		context:    *authContext,
	}

	// Add user groups.
	testUg1 := createUserGroup(t, s, "c", map[string]string{"label": "value"})
	testUg2 := createUserGroup(t, s, "a", map[string]string{"label": "value"})
	testUg3 := createUserGroup(t, s, "b", map[string]string{"label": "value"})

	// This user group should never should up because the user doesn't have group label access to it.
	_ = createUserGroup(t, s, "d", map[string]string{"inaccessible": "value"})

	authContext, err = srv.Authorizer.Authorize(authz.ContextWithUser(ctx, TestUser(user.GetName()).I))
	require.NoError(t, err)

	s = &ServerWithRoles{
		authServer: srv.AuthServer,
		alog:       srv.AuditLog,
		context:    *authContext,
	}

	// Test create.
	userGroups, _, err := s.ListUserGroups(ctx, 0, "")
	require.NoError(t, err)
	require.Len(t, userGroups, 3)

	t.Run("fetch all", func(t *testing.T) {
		t.Parallel()

		res, err := s.ListResources(ctx, proto.ListResourcesRequest{
			ResourceType: types.KindUserGroup,
			Limit:        10,
		})
		require.NoError(t, err)
		require.Len(t, res.Resources, 3)
		require.Empty(t, res.NextKey)
		require.Equal(t, 0, res.TotalCount) // TotalCount is 0 because this is not using fake pagination.

		userGroups, err := types.ResourcesWithLabels(res.Resources).AsUserGroups()
		require.NoError(t, err)
		slices.SortFunc(userGroups, func(a, b types.UserGroup) int {
			return strings.Compare(a.GetName(), b.GetName())
		})
		require.Empty(t, cmp.Diff([]types.UserGroup{testUg2, testUg3, testUg1}, userGroups, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
	})

	t.Run("start keys", func(t *testing.T) {
		t.Parallel()

		// First fetch.
		res, err := s.ListResources(ctx, proto.ListResourcesRequest{
			ResourceType: types.KindUserGroup,
			Limit:        1,
		})
		require.NoError(t, err)
		require.Len(t, res.Resources, 1)
		require.Equal(t, testUg3.GetName(), res.NextKey)

		// Second fetch.
		res, err = s.ListResources(ctx, proto.ListResourcesRequest{
			ResourceType: types.KindUserGroup,
			Limit:        1,
			StartKey:     res.NextKey,
		})
		require.NoError(t, err)
		require.Len(t, res.Resources, 1)
		require.Equal(t, testUg1.GetName(), res.NextKey)
	})

	t.Run("fetch with sort and total count", func(t *testing.T) {
		t.Parallel()
		res, err := s.ListResources(ctx, proto.ListResourcesRequest{
			ResourceType: types.KindUserGroup,
			Limit:        10,
			SortBy: types.SortBy{
				IsDesc: true,
				Field:  types.ResourceMetadataName,
			},
			NeedTotalCount: true,
		})
		require.NoError(t, err)
		require.Empty(t, res.NextKey)
		require.Len(t, res.Resources, 3)
		require.Equal(t, 3, res.TotalCount)

		userGroups, err := types.ResourcesWithLabels(res.Resources).AsUserGroups()
		require.NoError(t, err)
		names := make([]string, 3)
		for i, userGroup := range userGroups {
			names[i] = userGroup.GetName()
		}
		require.IsDecreasing(t, names)
	})
}

func createUserGroup(t *testing.T, s *ServerWithRoles, name string, labels map[string]string) types.UserGroup {
	userGroup, err := types.NewUserGroup(types.Metadata{
		Name:   name,
		Labels: labels,
	}, types.UserGroupSpecV1{})
	require.NoError(t, err)
	err = s.CreateUserGroup(context.Background(), userGroup)
	require.NoError(t, err)
	return userGroup
}

func TestDeleteUserAppSessions(t *testing.T) {
	ctx := context.Background()

	srv := newTestTLSServer(t)
	t.Cleanup(func() { srv.Close() })

	// Generates a new user client.
	userClient := func(username string) *authclient.Client {
		user, _, err := CreateUserAndRole(srv.Auth(), username, nil, nil)
		require.NoError(t, err)
		identity := TestUser(user.GetName())
		clt, err := srv.NewClient(identity)
		require.NoError(t, err)
		return clt
	}

	// Register users.
	aliceClt := userClient("alice")
	bobClt := userClient("bob")

	// Register multiple applications.
	applications := []struct {
		name       string
		publicAddr string
	}{
		{name: "panel", publicAddr: "panel.example.com"},
		{name: "admin", publicAddr: "admin.example.com"},
		{name: "metrics", publicAddr: "metrics.example.com"},
	}

	// Register and create a session for each application.
	for _, application := range applications {
		// Register an application.
		app, err := types.NewAppV3(types.Metadata{
			Name: application.name,
		}, types.AppSpecV3{
			URI:        "localhost",
			PublicAddr: application.publicAddr,
		})
		require.NoError(t, err)
		server, err := types.NewAppServerV3FromApp(app, "host", uuid.New().String())
		require.NoError(t, err)
		_, err = srv.Auth().UpsertApplicationServer(ctx, server)
		require.NoError(t, err)

		// Create a session for alice.
		_, err = aliceClt.CreateAppSession(ctx, &proto.CreateAppSessionRequest{
			Username:    "alice",
			PublicAddr:  application.publicAddr,
			ClusterName: "localhost",
		})
		require.NoError(t, err)

		// Create a session for bob.
		_, err = bobClt.CreateAppSession(ctx, &proto.CreateAppSessionRequest{
			Username:    "bob",
			PublicAddr:  application.publicAddr,
			ClusterName: "localhost",
		})
		require.NoError(t, err)
	}

	// Ensure the correct number of sessions.
	sessions, nextKey, err := srv.Auth().ListAppSessions(ctx, 10, "", "")
	require.NoError(t, err)
	require.Empty(t, nextKey)
	require.Len(t, sessions, 6)

	// Try to delete other user app sessions.
	err = aliceClt.DeleteUserAppSessions(ctx, &proto.DeleteUserAppSessionsRequest{Username: "bob"})
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))

	err = bobClt.DeleteUserAppSessions(ctx, &proto.DeleteUserAppSessionsRequest{Username: "alice"})
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))

	// Delete alice sessions.
	err = aliceClt.DeleteUserAppSessions(ctx, &proto.DeleteUserAppSessionsRequest{Username: "alice"})
	require.NoError(t, err)

	// Check if only bob's sessions are left.
	sessions, nextKey, err = srv.Auth().ListAppSessions(ctx, 10, "", "bob")
	require.NoError(t, err)
	require.Empty(t, nextKey)
	require.Len(t, sessions, 3)
	for _, session := range sessions {
		require.Equal(t, "bob", session.GetUser())
	}

	sessions, nextKey, err = srv.Auth().ListAppSessions(ctx, 10, "", "alice")
	require.NoError(t, err)
	require.Empty(t, sessions)
	require.Empty(t, nextKey)

	// Delete bob sessions.
	err = bobClt.DeleteUserAppSessions(ctx, &proto.DeleteUserAppSessionsRequest{Username: "bob"})
	require.NoError(t, err)

	// No sessions left.
	sessions, nextKey, err = srv.Auth().ListAppSessions(ctx, 10, "", "")
	require.NoError(t, err)
	require.Empty(t, sessions)
	require.Empty(t, nextKey)
}

func TestListResources_SortAndDeduplicate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	// Create user, role, and client.
	username := "user"
	user, role, err := CreateUserAndRole(srv.Auth(), username, nil, nil)
	require.NoError(t, err)
	identity := TestUser(user.GetName())
	clt, err := srv.NewClient(identity)
	require.NoError(t, err)

	// Permit user to get all resources.
	role.SetWindowsDesktopLabels(types.Allow, types.Labels{types.Wildcard: {types.Wildcard}})
	_, err = srv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)

	// Define some resource names for testing.
	names := []string{"d", "b", "d", "a", "a", "b"}
	uniqueNames := []string{"a", "b", "d"}

	tests := []struct {
		name            string
		kind            string
		insertResources func()
		wantNames       []string
	}{
		{
			name: "KindDatabaseServer",
			kind: types.KindDatabaseServer,
			insertResources: func() {
				for i := 0; i < len(names); i++ {
					db, err := types.NewDatabaseServerV3(types.Metadata{
						Name: fmt.Sprintf("name-%v", i),
					}, types.DatabaseServerSpecV3{
						HostID:   "_",
						Hostname: "_",
						Database: &types.DatabaseV3{
							Metadata: types.Metadata{
								Name: names[i],
							},
							Spec: types.DatabaseSpecV3{
								Protocol: "_",
								URI:      "_",
							},
						},
					})
					require.NoError(t, err)
					_, err = srv.Auth().UpsertDatabaseServer(ctx, db)
					require.NoError(t, err)
				}
			},
		},
		{
			name: "KindAppServer",
			kind: types.KindAppServer,
			insertResources: func() {
				for i := 0; i < len(names); i++ {
					server, err := types.NewAppServerV3(types.Metadata{
						Name: fmt.Sprintf("name-%v", i),
					}, types.AppServerSpecV3{
						HostID: "_",
						App:    &types.AppV3{Metadata: types.Metadata{Name: names[i]}, Spec: types.AppSpecV3{URI: "_"}},
					})
					require.NoError(t, err)
					_, err = srv.Auth().UpsertApplicationServer(ctx, server)
					require.NoError(t, err)
				}
			},
		},
		{
			name: "KindWindowsDesktop",
			kind: types.KindWindowsDesktop,
			insertResources: func() {
				for i := 0; i < len(names); i++ {
					desktop, err := types.NewWindowsDesktopV3(names[i], nil, types.WindowsDesktopSpecV3{
						Addr:   "_",
						HostID: fmt.Sprintf("name-%v", i),
					})
					require.NoError(t, err)
					require.NoError(t, srv.Auth().UpsertWindowsDesktop(ctx, desktop))
				}
			},
		},
		{
			name: "KindKubernetesCluster",
			kind: types.KindKubernetesCluster,
			insertResources: func() {
				for i := 0; i < len(names); i++ {

					kube, err := types.NewKubernetesClusterV3(types.Metadata{
						Name: names[i],
					}, types.KubernetesClusterSpecV3{})
					require.NoError(t, err)
					server, err := types.NewKubernetesServerV3FromCluster(kube, fmt.Sprintf("name-%v", i), fmt.Sprintf("name-%v", i))
					require.NoError(t, err)
					_, err = srv.Auth().UpsertKubernetesServer(ctx, server)
					require.NoError(t, err)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.insertResources()

			// Fetch all resources
			fetchedResources := make([]types.ResourceWithLabels, 0, len(uniqueNames))
			resp, err := clt.ListResources(ctx, proto.ListResourcesRequest{
				ResourceType:   tc.kind,
				NeedTotalCount: true,
				Limit:          2,
				SortBy:         types.SortBy{Field: types.ResourceMetadataName, IsDesc: true},
			})
			require.NoError(t, err)
			require.Len(t, resp.Resources, 2)
			require.Len(t, uniqueNames, resp.TotalCount)
			fetchedResources = append(fetchedResources, resp.Resources...)

			resp, err = clt.ListResources(ctx, proto.ListResourcesRequest{
				ResourceType:   tc.kind,
				NeedTotalCount: true,
				StartKey:       resp.NextKey,
				Limit:          2,
				SortBy:         types.SortBy{Field: types.ResourceMetadataName, IsDesc: true},
			})
			require.NoError(t, err)
			require.Len(t, resp.Resources, 1)
			require.Len(t, uniqueNames, resp.TotalCount)
			fetchedResources = append(fetchedResources, resp.Resources...)

			r := types.ResourcesWithLabels(fetchedResources)
			var extractedErr error
			var extractedNames []string

			switch tc.kind {
			case types.KindDatabaseServer:
				s, err := r.AsDatabaseServers()
				require.NoError(t, err)
				extractedNames, extractedErr = types.DatabaseServers(s).GetFieldVals(types.ResourceMetadataName)

			case types.KindAppServer:
				s, err := r.AsAppServers()
				require.NoError(t, err)
				extractedNames, extractedErr = types.AppServers(s).GetFieldVals(types.ResourceMetadataName)

			case types.KindWindowsDesktop:
				s, err := r.AsWindowsDesktops()
				require.NoError(t, err)
				extractedNames, extractedErr = types.WindowsDesktops(s).GetFieldVals(types.ResourceMetadataName)

			default:
				s, err := r.AsKubeClusters()
				require.NoError(t, err)
				require.Len(t, s, 3)
				extractedNames, extractedErr = types.KubeClusters(s).GetFieldVals(types.ResourceMetadataName)
			}

			require.NoError(t, extractedErr)
			require.ElementsMatch(t, uniqueNames, extractedNames)
			require.IsDecreasing(t, extractedNames)
		})
	}
}

func TestListResources_WithRoles(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)
	const nodePerPool = 3

	// inserts a pool nodes with different labels
	insertNodes := func(ctx context.Context, t *testing.T, srv *Server, nodeCount int, labels map[string]string) {
		for i := 0; i < nodeCount; i++ {
			name := uuid.NewString()
			addr := fmt.Sprintf("node-%s.example.com", name)

			node := &types.ServerV2{
				Kind:    types.KindNode,
				Version: types.V2,
				Metadata: types.Metadata{
					Name:      name,
					Namespace: apidefaults.Namespace,
					Labels:    labels,
				},
				Spec: types.ServerSpecV2{
					Addr: addr,
				},
			}

			_, err := srv.UpsertNode(ctx, node)
			require.NoError(t, err)
		}
	}

	// creates roles that deny the given labels
	createRole := func(ctx context.Context, t *testing.T, srv *Server, name string, labels map[string]apiutils.Strings) {
		role, err := types.NewRole(name, types.RoleSpecV6{
			Allow: types.RoleConditions{
				NodeLabels: types.Labels{
					"*": []string{types.Wildcard},
				},
			},
			Deny: types.RoleConditions{
				NodeLabels: labels,
			},
		})
		require.NoError(t, err)
		_, err = srv.UpsertRole(ctx, role)
		require.NoError(t, err)
	}

	// the pool from which nodes and roles are created from
	pool := map[string]struct {
		denied map[string]apiutils.Strings
		labels map[string]string
	}{
		"other": {
			denied: nil,
			labels: map[string]string{
				"other": "123",
			},
		},
		"a": {
			denied: map[string]apiutils.Strings{
				"pool": {"a"},
			},
			labels: map[string]string{
				"pool": "a",
			},
		},
		"b": {
			denied: map[string]apiutils.Strings{
				"pool": {"b"},
			},
			labels: map[string]string{
				"pool": "b",
			},
		},
		"c": {
			denied: map[string]apiutils.Strings{
				"pool": {"c"},
			},
			labels: map[string]string{
				"pool": "c",
			},
		},
		"d": {
			denied: map[string]apiutils.Strings{
				"pool": {"d"},
			},
			labels: map[string]string{
				"pool": "d",
			},
		},
		"e": {
			denied: map[string]apiutils.Strings{
				"pool": {"e"},
			},
			labels: map[string]string{
				"pool": "e",
			},
		},
	}

	// create the nodes and role
	for name, data := range pool {
		insertNodes(ctx, t, srv.Auth(), nodePerPool, data.labels)
		createRole(ctx, t, srv.Auth(), name, data.denied)
	}

	nodeCount := len(pool) * nodePerPool

	cases := []struct {
		name     string
		roles    []string
		expected int
	}{
		{
			name:     "all allowed",
			roles:    []string{"other"},
			expected: nodeCount,
		},
		{
			name:     "role a",
			roles:    []string{"a"},
			expected: nodeCount - nodePerPool,
		},
		{
			name:     "role a,b",
			roles:    []string{"a", "b"},
			expected: nodeCount - (2 * nodePerPool),
		},
		{
			name:     "role a,b,c",
			roles:    []string{"a", "b", "c"},
			expected: nodeCount - (3 * nodePerPool),
		},
		{
			name:     "role a,b,c,d",
			roles:    []string{"a", "b", "c", "d"},
			expected: nodeCount - (4 * nodePerPool),
		},
		{
			name:     "role a,b,c,d,e",
			roles:    []string{"a", "b", "c", "d", "e"},
			expected: nodeCount - (5 * nodePerPool),
		},
	}

	// ensure that a user can see the correct number of resources for their role(s)
	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			user, err := types.NewUser(uuid.NewString())
			require.NoError(t, err)

			for _, role := range tt.roles {
				user.AddRole(role)
			}

			user, err = srv.Auth().UpsertUser(ctx, user)
			require.NoError(t, err)

			for _, needTotal := range []bool{true, false} {
				total := needTotal
				t.Run(fmt.Sprintf("needTotal=%t", total), func(t *testing.T) {
					t.Parallel()

					clt, err := srv.NewClient(TestUser(user.GetName()))
					require.NoError(t, err)

					var resp *types.ListResourcesResponse
					var nodes []types.ResourceWithLabels
					for {
						key := ""
						if resp != nil {
							key = resp.NextKey
						}

						resp, err = clt.ListResources(ctx, proto.ListResourcesRequest{
							ResourceType:   types.KindNode,
							StartKey:       key,
							Limit:          nodePerPool,
							NeedTotalCount: total,
						})
						require.NoError(t, err)

						nodes = append(nodes, resp.Resources...)

						if resp.NextKey == "" {
							break
						}
					}

					require.Len(t, nodes, tt.expected)
				})
			}
		})
	}
}

// TestListUnifiedResources_WithLogins will generate multiple resources
// and retrieve them with login information.
func TestListUnifiedResources_WithLogins(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t, withCacheEnabled(true))

	require.Eventually(t, func() bool {
		return srv.Auth().UnifiedResourceCache.IsInitialized()
	}, 5*time.Second, 200*time.Millisecond, "unified resource watcher never initialized")

	// create user and client
	logins := []string{"llama", "fish"}
	user, _, err := CreateUserAndRole(srv.Auth(), "user", nil /*mutated with role mutator*/, nil,
		WithRoleMutator(
			func(role types.Role) {
				role.SetLogins(types.Allow, logins)
				role.SetWindowsLogins(types.Allow, logins)
				role.SetWindowsDesktopLabels(types.Allow, types.Labels{types.Wildcard: []string{types.Wildcard}})
				role.SetAWSRoleARNs(types.Allow, logins)
			}),
	)
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		name := uuid.New().String()
		node, err := types.NewServerWithLabels(
			name,
			types.KindNode,
			types.ServerSpecV2{},
			map[string]string{"name": name},
		)
		require.NoError(t, err)

		_, err = srv.Auth().UpsertNode(ctx, node)
		require.NoError(t, err)

		db, err := types.NewDatabaseServerV3(types.Metadata{
			Name: name,
		}, types.DatabaseServerSpecV3{
			HostID:   "_",
			Hostname: "_",
			Database: &types.DatabaseV3{
				Metadata: types.Metadata{
					Name: fmt.Sprintf("name-%d", i),
				},
				Spec: types.DatabaseSpecV3{
					Protocol: "_",
					URI:      "_",
				},
			},
		})
		require.NoError(t, err)

		_, err = srv.Auth().UpsertDatabaseServer(ctx, db)
		require.NoError(t, err)

		desktop, err := types.NewWindowsDesktopV3(name, nil, types.WindowsDesktopSpecV3{
			HostID: strconv.Itoa(i),
			Addr:   "1.2.3.4",
		})
		require.NoError(t, err)

		require.NoError(t, srv.Auth().UpsertWindowsDesktop(ctx, desktop))

		awsApp, err := types.NewAppServerV3(types.Metadata{Name: name}, types.AppServerSpecV3{
			HostID:   strconv.Itoa(i),
			Hostname: "example.com",
			App: &types.AppV3{
				Metadata: types.Metadata{Name: fmt.Sprintf("name-%d", i)},
				Spec: types.AppSpecV3{
					URI: "https://console.aws.amazon.com/ec2/v2/home",
				},
			},
		})
		require.NoError(t, err)

		_, err = srv.Auth().UpsertApplicationServer(ctx, awsApp)
		require.NoError(t, err)
	}

	clt, err := srv.NewClient(TestUser(user.GetName()))
	require.NoError(t, err)

	resultsC := make(chan []*proto.PaginatedResource, 1)

	// Immediately listing resources can be problematic, given that not all were
	// necessarily replicated in the unified resources cache. To cover this
	// scenario, perform multiple attempts (if necessary) to read the complete
	// list of resources.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		var start string
		var results []*proto.PaginatedResource
		for {
			resp, err := clt.ListUnifiedResources(ctx, &proto.ListUnifiedResourcesRequest{
				Limit:         5,
				IncludeLogins: true,
				SortBy:        types.SortBy{IsDesc: true, Field: types.ResourceMetadataName},
				StartKey:      start,
			})
			if !assert.NoError(t, err) {
				return
			}

			results = append(results, resp.Resources...)
			start = resp.NextKey
			if start == "" {
				break
			}
		}

		// Note: this number should be updated in case we add more resources to
		// the setup loop.
		if !assert.Len(t, results, 20) {
			return
		}
		resultsC <- results
	}, 10*time.Second, 100*time.Millisecond, "unable to list all resources")

	results := <-resultsC
	// Check that only server, desktop, and app server resources contain the expected logins
	expectPrincipals := func(resource *proto.PaginatedResource) bool {
		isAWSConsoleApp := resource.GetAppServer() != nil && resource.GetAppServer().GetApp().IsAWSConsole()
		return resource.GetNode() != nil || resource.GetWindowsDesktop() != nil || isAWSConsoleApp
	}

	for _, resource := range results {
		if expectPrincipals(resource) {
			require.Empty(t, cmp.Diff(resource.Logins, logins, cmpopts.SortSlices(func(a, b string) bool {
				return strings.Compare(a, b) < 0
			})), "mismatch on expected logins list for resource %T", resource.Resource)
			continue
		}

		require.Empty(t, resource.Logins, "expected resource %T to get empty list of logins but got %s", resource.Resource, resource.Logins)
	}
}

func TestListUnifiedResources_IncludeRequestable(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	// Create test nodes.
	const numTestNodes = 3
	for i := 0; i < numTestNodes; i++ {
		name := fmt.Sprintf("node%d", i)
		node, err := types.NewServerWithLabels(
			name,
			types.KindNode,
			types.ServerSpecV2{},
			map[string]string{"name": name},
		)
		require.NoError(t, err)

		_, err = srv.Auth().UpsertNode(ctx, node)
		require.NoError(t, err)
	}

	testNodes, err := srv.Auth().GetNodes(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Len(t, testNodes, numTestNodes)

	// create user and client
	requester, role, err := CreateUserAndRole(srv.Auth(), "requester", []string{"requester"}, nil)
	require.NoError(t, err)

	// only allow user to see first node
	role.SetNodeLabels(types.Allow, types.Labels{"name": {testNodes[0].GetName()}})

	// create a new role which can see second node
	searchAsRole := services.RoleForUser(requester)
	searchAsRole.SetName("test_search_role")
	searchAsRole.SetNodeLabels(types.Allow, types.Labels{"name": {testNodes[1].GetName()}})
	searchAsRole.SetLogins(types.Allow, []string{"requester"})
	_, err = srv.Auth().UpsertRole(ctx, searchAsRole)
	require.NoError(t, err)

	role.SetSearchAsRoles(types.Allow, []string{searchAsRole.GetName()})
	_, err = srv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)

	requesterClt, err := srv.NewClient(TestUser(requester.GetName()))
	require.NoError(t, err)

	type expected struct {
		name        string
		requestable bool
	}

	for _, tc := range []struct {
		desc              string
		clt               *authclient.Client
		requestOpt        func(*proto.ListUnifiedResourcesRequest)
		expectedResources []expected
	}{
		{
			desc:              "no search",
			clt:               requesterClt,
			expectedResources: []expected{{name: testNodes[0].GetName(), requestable: false}},
		},
		{
			desc: "search as roles without requestable",
			clt:  requesterClt,
			requestOpt: func(req *proto.ListUnifiedResourcesRequest) {
				req.UseSearchAsRoles = true
			},
			expectedResources: []expected{
				{name: testNodes[0].GetName(), requestable: false},
				{name: testNodes[1].GetName(), requestable: false},
			},
		},
		{
			desc: "search as roles with requestable",
			clt:  requesterClt,
			requestOpt: func(req *proto.ListUnifiedResourcesRequest) {
				req.IncludeRequestable = true
				req.UseSearchAsRoles = true
			},
			expectedResources: []expected{
				{name: testNodes[0].GetName(), requestable: false},
				{name: testNodes[1].GetName(), requestable: true},
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			req := proto.ListUnifiedResourcesRequest{
				SortBy: types.SortBy{Field: "name", IsDesc: false},
				Limit:  int32(len(testNodes)),
			}
			if tc.requestOpt != nil {
				tc.requestOpt(&req)
			}
			resp, err := tc.clt.ListUnifiedResources(ctx, &req)
			require.NoError(t, err)
			require.Len(t, resp.Resources, len(tc.expectedResources))
			var resources []expected
			for _, resource := range resp.Resources {
				resources = append(resources, expected{name: resource.GetNode().GetName(), requestable: resource.RequiresRequest})
			}
			require.ElementsMatch(t, tc.expectedResources, resources)
		})
	}
}

// TestListUnifiedResources_KindsFilter will generate multiple resources
// and filter for only one kind.
func TestListUnifiedResources_KindsFilter(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t, withCacheEnabled(true))

	require.Eventually(t, func() bool {
		return srv.Auth().UnifiedResourceCache.IsInitialized()
	}, 5*time.Second, 200*time.Millisecond, "unified resource watcher never initialized")

	for i := 0; i < 5; i++ {
		name := uuid.New().String()
		node, err := types.NewServerWithLabels(
			name,
			types.KindNode,
			types.ServerSpecV2{},
			map[string]string{"name": name},
		)
		require.NoError(t, err)

		_, err = srv.Auth().UpsertNode(ctx, node)
		require.NoError(t, err)
		db, err := types.NewDatabaseServerV3(types.Metadata{
			Name: name,
		}, types.DatabaseServerSpecV3{
			HostID:   "_",
			Hostname: "_",
			Database: &types.DatabaseV3{
				Metadata: types.Metadata{
					Name: fmt.Sprintf("name-%d", i),
				},
				Spec: types.DatabaseSpecV3{
					Protocol: "_",
					URI:      "_",
				},
			},
		})
		require.NoError(t, err)
		_, err = srv.Auth().UpsertDatabaseServer(ctx, db)
		require.NoError(t, err)
	}

	// create user and client
	user, _, err := CreateUserAndRole(srv.Auth(), "user", nil, nil)
	require.NoError(t, err)
	clt, err := srv.NewClient(TestUser(user.GetName()))
	require.NoError(t, err)

	var resp *proto.ListUnifiedResourcesResponse
	inlineEventually(t, func() bool {
		var err error
		resp, err = clt.ListUnifiedResources(ctx, &proto.ListUnifiedResourcesRequest{
			Kinds:  []string{types.KindDatabase},
			Limit:  5,
			SortBy: types.SortBy{IsDesc: true, Field: types.ResourceMetadataName},
		})
		require.NoError(t, err)
		return len(resp.Resources) == 5
	}, time.Second, time.Second/10)

	// Check that all resources are of type KindDatabaseServer
	for _, resource := range resp.Resources {
		r := resource.GetDatabaseServer()
		require.Equal(t, types.KindDatabaseServer, r.GetKind())
	}

	// Check for invalid sort error message
	_, err = clt.ListUnifiedResources(ctx, &proto.ListUnifiedResourcesRequest{
		Kinds:  []string{types.KindDatabase},
		Limit:  5,
		SortBy: types.SortBy{},
	})
	require.ErrorContains(t, err, "sort field is required")
}

func TestListUnifiedResources_WithPinnedResources(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t, withCacheEnabled(true))

	require.Eventually(t, func() bool {
		return srv.Auth().UnifiedResourceCache.IsInitialized()
	}, 5*time.Second, 200*time.Millisecond, "unified resource watcher never initialized")

	names := []string{"tifa", "cloud", "aerith", "baret", "cid", "tifa2"}
	for _, name := range names {

		// add nodes
		node, err := types.NewServerWithLabels(
			name,
			types.KindNode,
			types.ServerSpecV2{
				Hostname: name,
			},
			map[string]string{"name": name},
		)
		require.NoError(t, err)

		_, err = srv.Auth().UpsertNode(ctx, node)
		require.NoError(t, err)
	}

	// create user, role, and client
	username := "theuser"
	user, _, err := CreateUserAndRole(srv.Auth(), username, nil, nil)
	require.NoError(t, err)
	identity := TestUser(user.GetName())

	// pin a resource
	pinned := &userpreferencesv1.PinnedResourcesUserPreferences{
		ResourceIds: []string{"tifa/tifa/node"},
	}
	clusterPrefs := &userpreferencesv1.ClusterUserPreferences{
		PinnedResources: pinned,
	}

	req := &userpreferencesv1.UpsertUserPreferencesRequest{
		Preferences: &userpreferencesv1.UserPreferences{
			ClusterPreferences: clusterPrefs,
		},
	}
	err = srv.Auth().UpsertUserPreferences(ctx, username, req.Preferences)
	require.NoError(t, err)

	clt, err := srv.NewClient(identity)
	require.NoError(t, err)

	var resp *proto.ListUnifiedResourcesResponse
	inlineEventually(t, func() bool {
		var err error
		resp, err = clt.ListUnifiedResources(ctx, &proto.ListUnifiedResourcesRequest{
			PinnedOnly: true,
		})
		require.NoError(t, err)
		return len(resp.Resources) == 1
	}, time.Second*5, time.Millisecond*200)

	require.Empty(t, resp.NextKey)
	// Check that our returned resource is the pinned resource
	require.Equal(t, "tifa", resp.Resources[0].GetNode().GetHostname())
}

// TestListUnifiedResources_WithSearch will generate multiple resources
// and filter by a search query
func TestListUnifiedResources_WithSearch(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t, withCacheEnabled(true))

	require.Eventually(t, func() bool {
		return srv.Auth().UnifiedResourceCache.IsInitialized()
	}, 5*time.Second, 200*time.Millisecond, "unified resource watcher never initialized")

	names := []string{"vivi", "cloud", "aerith", "barret", "cid", "vivi2"}
	for i := 0; i < 6; i++ {
		name := names[i]
		node, err := types.NewServerWithLabels(
			name,
			types.KindNode,
			types.ServerSpecV2{
				Hostname: name,
			},
			map[string]string{"name": name},
		)
		require.NoError(t, err)

		_, err = srv.Auth().UpsertNode(ctx, node)
		require.NoError(t, err)
	}

	inlineEventually(t, func() bool {
		testNodes, err := srv.Auth().GetNodes(ctx, apidefaults.Namespace)
		require.NoError(t, err)
		return len(testNodes) == 6
	}, time.Second*5, time.Millisecond*200)

	sp := &types.SAMLIdPServiceProviderV1{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name: "example",
			},
		},
		Spec: types.SAMLIdPServiceProviderSpecV1{
			ACSURL:   "https://example.com/acs",
			EntityID: "https://example.com",
		},
	}
	require.NoError(t, srv.Auth().CreateSAMLIdPServiceProvider(ctx, sp))

	// create user and client
	user, _, err := CreateUserAndRole(srv.Auth(), "user", nil, nil)
	require.NoError(t, err)
	clt, err := srv.NewClient(TestUser(user.GetName()))
	require.NoError(t, err)

	var resp *proto.ListUnifiedResourcesResponse
	inlineEventually(t, func() bool {
		var err error
		resp, err = clt.ListUnifiedResources(ctx, &proto.ListUnifiedResourcesRequest{
			SearchKeywords: []string{"example"},
			Limit:          10,
			SortBy:         types.SortBy{IsDesc: true, Field: types.ResourceMetadataName},
		})
		require.NoError(t, err)
		return len(resp.Resources) == 1
	}, time.Second*5, time.Millisecond*200)

	require.Empty(t, resp.NextKey)

	// Check that our returned resource has the correct name
	for _, resource := range resp.Resources {
		r := resource.GetSAMLIdPServiceProvider()
		require.True(t, strings.Contains(r.GetName(), "example"))
	}
}

// TestListUnifiedResources_MixedAccess will generate multiple resources
// and only return the kinds the user has access to
func TestListUnifiedResources_MixedAccess(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t, withCacheEnabled(true))

	require.Eventually(t, func() bool {
		return srv.Auth().UnifiedResourceCache.IsInitialized()
	}, 5*time.Second, 200*time.Millisecond, "unified resource watcher never initialized")

	names := []string{"tifa", "cloud", "aerith", "baret", "cid", "tifa2"}
	for i := 0; i < 6; i++ {
		name := names[i]

		// add nodes
		node, err := types.NewServerWithLabels(
			name,
			types.KindNode,
			types.ServerSpecV2{
				Hostname: name,
			},
			map[string]string{"name": "mylabel"},
		)
		require.NoError(t, err)

		_, err = srv.Auth().UpsertNode(ctx, node)
		require.NoError(t, err)

		// add dbs
		db, err := types.NewDatabaseServerV3(types.Metadata{
			Name: name,
		}, types.DatabaseServerSpecV3{
			HostID:   "_",
			Hostname: "_",
			Database: &types.DatabaseV3{
				Metadata: types.Metadata{
					Name: fmt.Sprintf("name-%d", i),
				},
				Spec: types.DatabaseSpecV3{
					Protocol: "_",
					URI:      "_",
				},
			},
		})
		require.NoError(t, err)
		_, err = srv.Auth().UpsertDatabaseServer(ctx, db)
		require.NoError(t, err)

		// add desktops
		desktop, err := types.NewWindowsDesktopV3(name, nil,
			types.WindowsDesktopSpecV3{Addr: "_", HostID: "_"})
		require.NoError(t, err)
		require.NoError(t, srv.Auth().UpsertWindowsDesktop(ctx, desktop))
	}

	inlineEventually(t, func() bool {
		testNodes, err := srv.Auth().GetNodes(ctx, apidefaults.Namespace)
		require.NoError(t, err)
		return len(testNodes) == 6
	}, time.Second*5, time.Millisecond*200)

	inlineEventually(t, func() bool {
		testDbs, err := srv.Auth().GetDatabaseServers(ctx, apidefaults.Namespace)
		require.NoError(t, err)
		return len(testDbs) == 6
	}, time.Second*5, time.Millisecond*200)

	inlineEventually(t, func() bool {
		testDesktops, err := srv.Auth().GetWindowsDesktops(ctx, types.WindowsDesktopFilter{})
		require.NoError(t, err)
		return len(testDesktops) == 6
	}, time.Second*5, time.Millisecond*200)

	// create user, role, and client
	username := "user"
	user, role, err := CreateUserAndRole(srv.Auth(), username, nil, nil)
	require.NoError(t, err)

	role.SetNodeLabels(types.Allow, types.Labels{"*": {"*"}})
	role.SetDatabaseLabels(types.Allow, types.Labels{"*": {"*"}})
	role.SetWindowsDesktopLabels(types.Allow, types.Labels{"*": {"*"}})
	_, err = srv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)
	// remove permission from nodes by labels
	role.SetNodeLabels(types.Deny, types.Labels{"name": {"mylabel"}})
	// remove permission from desktops by rule
	denyRules := []types.Rule{{
		Resources: []string{types.KindWindowsDesktop},
		Verbs:     []string{types.VerbList, types.VerbRead},
	}}
	role.SetRules(types.Deny, denyRules)
	_, err = srv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)
	// require.NoError(t, err)
	identity := TestUser(user.GetName())
	clt, err := srv.NewClient(identity)
	require.NoError(t, err)

	// ensure updated roles have propagated to auth cache
	flushCache(t, srv.Auth())

	resp, err := clt.ListUnifiedResources(ctx, &proto.ListUnifiedResourcesRequest{
		Limit:  20,
		SortBy: types.SortBy{IsDesc: true, Field: types.ResourceMetadataName},
	})
	require.NoError(t, err)
	require.Len(t, resp.Resources, 6)
	require.Empty(t, resp.NextKey)

	// only receive databases because nodes are denied with labels and desktops are denied with a verb rule
	for _, resource := range resp.Resources {
		r := resource.GetDatabaseServer()
		require.Equal(t, types.KindDatabaseServer, r.GetKind())
	}

	// Update the roles to prevent access to any resource kinds.
	role.SetRules(types.Deny, []types.Rule{{Resources: services.UnifiedResourceKinds, Verbs: []string{types.VerbList, types.VerbRead}}})
	_, err = srv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)

	// ensure updated roles have propagated to auth cache
	flushCache(t, srv.Auth())

	// Get a new client to test with the new roles.
	clt, err = srv.NewClient(identity)
	require.NoError(t, err)

	// Validate that an error is returned when no kinds are requested.
	resp, err = clt.ListUnifiedResources(ctx, &proto.ListUnifiedResourcesRequest{
		Limit:  20,
		SortBy: types.SortBy{IsDesc: true, Field: types.ResourceMetadataName},
	})

	require.True(t, trace.IsAccessDenied(err), "Expected Access Denied, got %v", err)
	require.Nil(t, resp)

	// Validate that an error is returned when a subset of kinds are requested.
	resp, err = clt.ListUnifiedResources(ctx, &proto.ListUnifiedResourcesRequest{
		Limit:  20,
		SortBy: types.SortBy{IsDesc: true, Field: types.ResourceMetadataName},
		Kinds:  []string{types.KindNode, types.KindDatabaseServer},
	})
	require.True(t, trace.IsAccessDenied(err))
	require.Nil(t, resp)
}

// TestListUnifiedResources_WithPredicate will return resources that match the
// predicate expression
func TestListUnifiedResources_WithPredicate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t, withCacheEnabled(true))

	require.Eventually(t, func() bool {
		return srv.Auth().UnifiedResourceCache.IsInitialized()
	}, 5*time.Second, 200*time.Millisecond, "unified resource watcher never initialized")

	names := []string{"tifa", "cloud", "aerith", "baret", "cid", "tifa2"}
	for i := 0; i < 6; i++ {
		name := names[i]

		// add nodes
		node, err := types.NewServerWithLabels(
			name,
			types.KindNode,
			types.ServerSpecV2{
				Hostname: name,
			},
			map[string]string{"name": name},
		)
		require.NoError(t, err)

		_, err = srv.Auth().UpsertNode(ctx, node)
		require.NoError(t, err)
	}

	inlineEventually(t, func() bool {
		testNodes, err := srv.Auth().GetNodes(ctx, apidefaults.Namespace)
		require.NoError(t, err)
		return len(testNodes) == 6
	}, time.Second*5, time.Millisecond*200)

	// create user, role, and client
	username := "theuser"
	user, _, err := CreateUserAndRole(srv.Auth(), username, nil, nil)
	require.NoError(t, err)
	identity := TestUser(user.GetName())
	clt, err := srv.NewClient(identity)
	require.NoError(t, err)

	resp, err := clt.ListUnifiedResources(ctx, &proto.ListUnifiedResourcesRequest{
		PredicateExpression: `labels.name == "tifa"`,
		Limit:               10,
		SortBy:              types.SortBy{IsDesc: true, Field: types.ResourceMetadataName},
	})
	require.NoError(t, err)
	require.Len(t, resp.Resources, 1)
	require.Empty(t, resp.NextKey)

	// fail with bad predicate expression
	_, err = clt.ListUnifiedResources(ctx, &proto.ListUnifiedResourcesRequest{
		PredicateExpression: `labels.name == "tifa`,
		Limit:               10,
		SortBy:              types.SortBy{IsDesc: true, Field: types.ResourceMetadataName},
	})
	require.Error(t, err)
}

func withAccountAssignment(condition types.RoleConditionType, accountID, permissionSet string) CreateUserAndRoleOption {
	return WithRoleMutator(func(role types.Role) {
		r := role.(*types.RoleV6)
		cond := &r.Spec.Deny
		if condition == types.Allow {
			cond = &r.Spec.Allow
		}
		cond.AccountAssignments = append(
			cond.AccountAssignments,
			types.IdentityCenterAccountAssignment{
				Account:       accountID,
				PermissionSet: permissionSet,
			})
	})
}

func TestUnifiedResources_IdentityCenter(t *testing.T) {
	const (
		validAccountID        = "11111111"
		validPermissionSetARN = "some:ps:arn"
	)

	ctx := context.Background()
	srv := newTestTLSServer(t, withCacheEnabled(true))

	require.Eventually(t, func() bool {
		return srv.Auth().UnifiedResourceCache.IsInitialized()
	}, 5*time.Second, 200*time.Millisecond, "unified resource watcher never initialized")

	allowByGenericKind := []types.Rule{
		types.NewRule(types.KindIdentityCenter, services.RO()),
	}

	// adds a Rule ALLOW condition for the valid account ID and Permission set
	// pair
	withMatchingAccountAssignment := withAccountAssignment(types.Allow,
		validAccountID, validPermissionSetARN)

	testCases := []struct {
		name string
		kind string
		init func(*testing.T)
	}{
		{
			name: "account",
			kind: types.KindIdentityCenterAccount,
			init: func(subtestT *testing.T) {
				acct, err := srv.Auth().CreateIdentityCenterAccount(ctx, services.IdentityCenterAccount{
					Account: &identitycenterv1.Account{
						Kind:    types.KindIdentityCenterAccount,
						Version: types.V1,
						Metadata: &headerv1.Metadata{
							Name: "test-account",
							Labels: map[string]string{
								types.OriginLabel: apicommon.OriginAWSIdentityCenter,
							},
						},
						Spec: &identitycenterv1.AccountSpec{
							Id:   validAccountID,
							Arn:  "some:account:arn",
							Name: "Test Account",
						},
					},
				})
				require.NoError(subtestT, err)
				subtestT.Cleanup(func() {
					srv.Auth().DeleteIdentityCenterAccount(ctx,
						services.IdentityCenterAccountID(acct.GetMetadata().GetName()))
				})

				inlineEventually(subtestT,
					func() bool {
						accounts, _, err := srv.Auth().ListIdentityCenterAccounts(
							ctx, 100, &pagination.PageRequestToken{})
						require.NoError(t, err)
						return len(accounts) == 1
					},
					5*time.Second, 200*time.Millisecond,
					"Target resource missing from cache")
			},
		},
		{
			name: "account assignment",
			kind: types.KindIdentityCenterAccountAssignment,
			init: func(subtestT *testing.T) {
				asmt, err := srv.Auth().CreateAccountAssignment(ctx, services.IdentityCenterAccountAssignment{
					AccountAssignment: &identitycenterv1.AccountAssignment{
						Kind:    types.KindIdentityCenterAccountAssignment,
						Version: types.V1,
						Metadata: &headerv1.Metadata{
							Name: "test-account",
							Labels: map[string]string{
								types.OriginLabel: apicommon.OriginAWSIdentityCenter,
							},
						},
						Spec: &identitycenterv1.AccountAssignmentSpec{
							AccountId: validAccountID,
							Display:   "Test Account Assignment",
							PermissionSet: &identitycenterv1.PermissionSetInfo{
								Arn:          validPermissionSetARN,
								Name:         "Test permission set",
								AssignmentId: "Test Assignment on Test Account",
							},
						},
					},
				})
				require.NoError(subtestT, err)
				subtestT.Cleanup(func() {
					srv.Auth().DeleteAccountAssignment(ctx,
						services.IdentityCenterAccountAssignmentID(asmt.GetMetadata().GetName()))
				})

				inlineEventually(subtestT,
					func() bool {
						testAssignments, _, err := srv.Auth().ListAccountAssignments(
							ctx, 100, &pagination.PageRequestToken{})
						require.NoError(t, err)
						return len(testAssignments) == 1
					},
					5*time.Second, 200*time.Millisecond,
					"Target resource missing from cache")
			},
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			test.init(t)

			allowBySpecificKind := []types.Rule{
				types.NewRule(test.kind, services.RO()),
			}

			t.Run("no access", func(t *testing.T) {
				userNoAccess, _, err := CreateUserAndRole(srv.Auth(), "no-access", nil, nil)
				require.NoError(t, err)

				identity := TestUser(userNoAccess.GetName())
				clt, err := srv.NewClient(identity)
				require.NoError(t, err)
				defer clt.Close()

				resp, err := clt.ListResources(ctx, proto.ListResourcesRequest{
					ResourceType: test.kind,
					Labels: map[string]string{
						types.OriginLabel: apicommon.OriginAWSIdentityCenter,
					},
				})
				require.NoError(t, err)
				require.Empty(t, resp.Resources)
			})

			t.Run("no access via no matching account condition ", func(t *testing.T) {
				userNoAccess, _, err := CreateUserAndRole(srv.Auth(), "no-access-account-mismatch", nil,
					allowByGenericKind,
					withAccountAssignment(types.Allow, "22222222", validPermissionSetARN))
				require.NoError(t, err)

				identity := TestUser(userNoAccess.GetName())
				clt, err := srv.NewClient(identity)
				require.NoError(t, err)
				defer clt.Close()

				resp, err := clt.ListResources(ctx, proto.ListResourcesRequest{
					ResourceType: test.kind,
					Labels: map[string]string{
						types.OriginLabel: apicommon.OriginAWSIdentityCenter,
					},
				})
				require.NoError(t, err)
				require.Empty(t, resp.Resources)
			})

			t.Run("access denied by account deny condition", func(t *testing.T) {
				userNoAccess, _, err := CreateUserAndRole(srv.Auth(), "no-access-account-mismatch", nil,
					allowBySpecificKind,
					withMatchingAccountAssignment,
					withAccountAssignment(types.Deny, validAccountID, "*"))
				require.NoError(t, err)

				identity := TestUser(userNoAccess.GetName())
				clt, err := srv.NewClient(identity)
				require.NoError(t, err)
				defer clt.Close()

				resp, err := clt.ListResources(ctx, proto.ListResourcesRequest{
					ResourceType: test.kind,
					Labels: map[string]string{
						types.OriginLabel: apicommon.OriginAWSIdentityCenter,
					},
				})
				require.NoError(t, err)
				require.Empty(t, resp.Resources)
			})

			t.Run("access via generic kind", func(t *testing.T) {
				user, _, err := CreateUserAndRole(srv.Auth(), "read-generic", nil,
					allowByGenericKind,
					withMatchingAccountAssignment)
				require.NoError(t, err)

				identity := TestUser(user.GetName())
				clt, err := srv.NewClient(identity)
				require.NoError(t, err)
				defer clt.Close()

				resp, err := clt.ListResources(ctx, proto.ListResourcesRequest{
					ResourceType: test.kind,
					Labels: map[string]string{
						types.OriginLabel: apicommon.OriginAWSIdentityCenter,
					},
				})
				require.NoError(t, err)
				require.Len(t, resp.Resources, 1)
			})

			t.Run("access via specific kind", func(t *testing.T) {
				user, _, err := CreateUserAndRole(srv.Auth(), "read-specific", nil,
					allowBySpecificKind,
					withMatchingAccountAssignment)
				require.NoError(t, err)

				identity := TestUser(user.GetName())
				clt, err := srv.NewClient(identity)
				require.NoError(t, err)
				defer clt.Close()

				resp, err := clt.ListResources(ctx, proto.ListResourcesRequest{
					ResourceType: test.kind,
				})
				require.NoError(t, err)
				require.Len(t, resp.Resources, 1)
			})

			t.Run("access denied via specific kind beats allow via generic kind", func(t *testing.T) {
				user, _, err := CreateUserAndRole(srv.Auth(), "specific-beats-generic", nil,
					allowByGenericKind,
					withMatchingAccountAssignment,
					WithRoleMutator(func(r types.Role) {
						r.SetRules(types.Deny, []types.Rule{
							types.NewRule(test.kind, services.RO()),
						})
					}))
				require.NoError(t, err)

				identity := TestUser(user.GetName())
				clt, err := srv.NewClient(identity)
				require.NoError(t, err)
				defer clt.Close()

				_, err = clt.ListResources(ctx, proto.ListResourcesRequest{
					ResourceType: test.kind,
				})
				require.True(t, trace.IsAccessDenied(err),
					"Expected Access Denied, got %v", err)
			})

			// The tests below this point are only applicable to Identity Center
			// Account assignments
			if test.kind == types.KindIdentityCenterAccount {
				return
			}

			// Asserts that a role ALLOW condition with a matching Account ID but
			// nonmatching PermissionSet ARN does not allow access
			t.Run("no access via no matching allow permission set condition", func(t *testing.T) {
				userNoAccess, _, err := CreateUserAndRole(srv.Auth(), "no-access-allow-ps-mismatch", nil,
					allowByGenericKind,
					withAccountAssignment(types.Allow, validAccountID, "some:other:ps:arn"))
				require.NoError(t, err)

				identity := TestUser(userNoAccess.GetName())
				clt, err := srv.NewClient(identity)
				require.NoError(t, err)
				defer clt.Close()

				resp, err := clt.ListResources(ctx, proto.ListResourcesRequest{
					ResourceType: test.kind,
					Labels: map[string]string{
						types.OriginLabel: apicommon.OriginAWSIdentityCenter,
					},
				})
				require.NoError(t, err)
				require.Empty(t, resp.Resources)
			})

			// Asserts that a role DENY condition with a matching Account ID but
			// nonmatching PermissionSet ARN does not block access
			t.Run("access via no matching deny permission set condition", func(t *testing.T) {
				userNoAccess, _, err := CreateUserAndRole(srv.Auth(), "access-deny-ps-mismatch", nil,
					allowByGenericKind,
					withMatchingAccountAssignment,
					withAccountAssignment(types.Deny, "*", "some:other:ps"))
				require.NoError(t, err)

				identity := TestUser(userNoAccess.GetName())
				clt, err := srv.NewClient(identity)
				require.NoError(t, err)
				defer clt.Close()

				resp, err := clt.ListResources(ctx, proto.ListResourcesRequest{
					ResourceType: test.kind,
					Labels: map[string]string{
						types.OriginLabel: apicommon.OriginAWSIdentityCenter,
					},
				})
				require.NoError(t, err)
				require.Len(t, resp.Resources, 1)
			})
		})
	}
}

func BenchmarkListUnifiedResourcesFilter(b *testing.B) {
	const nodeCount = 150_000
	const roleCount = 32

	ctx := context.Background()
	srv := newTestTLSServer(b)

	var ids []string
	for i := 0; i < roleCount; i++ {
		ids = append(ids, uuid.New().String())
	}

	ids[0] = "hidden"

	var hiddenNodes int
	// Create test nodes.
	for i := 0; i < nodeCount; i++ {
		name := uuid.New().String()
		id := ids[i%len(ids)]
		if id == "hidden" {
			hiddenNodes++
		}

		labels := map[string]string{
			"kEy":   id,
			"grouP": "useRs",
		}

		if i == 10 {
			labels["ip"] = "10.20.30.40"
			labels["ADDRESS"] = "10.20.30.41"
			labels["food"] = "POTATO"
		}

		node, err := types.NewServerWithLabels(
			name,
			types.KindNode,
			types.ServerSpecV2{},
			labels,
		)
		require.NoError(b, err)

		_, err = srv.Auth().UpsertNode(ctx, node)
		require.NoError(b, err)
	}

	b.Run("labels", func(b *testing.B) {
		benchmarkListUnifiedResources(
			b, ctx,
			1,
			srv,
			ids,
			func(role types.Role, id string) {
				role.SetNodeLabels(types.Allow, types.Labels{types.Wildcard: []string{types.Wildcard}})
			},
			func(req *proto.ListUnifiedResourcesRequest) {
				req.Labels = map[string]string{"ip": "10.20.30.40"}
			},
		)
	})
	b.Run("predicate path", func(b *testing.B) {
		benchmarkListUnifiedResources(
			b, ctx,
			1,
			srv,
			ids,
			func(role types.Role, id string) {
				role.SetNodeLabels(types.Allow, types.Labels{types.Wildcard: []string{types.Wildcard}})
			},
			func(req *proto.ListUnifiedResourcesRequest) {
				req.PredicateExpression = `labels.ip == "10.20.30.40"`
			},
		)
	})
	b.Run("predicate index", func(b *testing.B) {
		benchmarkListUnifiedResources(
			b, ctx,
			1,
			srv,
			ids,
			func(role types.Role, id string) {
				role.SetNodeLabels(types.Allow, types.Labels{types.Wildcard: []string{types.Wildcard}})
			},
			func(req *proto.ListUnifiedResourcesRequest) {
				req.PredicateExpression = `labels["ip"] == "10.20.30.40"`
			},
		)
	})
	b.Run("search lower", func(b *testing.B) {
		benchmarkListUnifiedResources(
			b, ctx,
			1,
			srv,
			ids,
			func(role types.Role, id string) {
				role.SetNodeLabels(types.Allow, types.Labels{types.Wildcard: []string{types.Wildcard}})
			},
			func(req *proto.ListUnifiedResourcesRequest) {
				req.SearchKeywords = []string{"10.20.30.40"}
			},
		)
	})
	b.Run("search upper", func(b *testing.B) {
		benchmarkListUnifiedResources(
			b, ctx,
			1,
			srv,
			ids,
			func(role types.Role, id string) {
				role.SetNodeLabels(types.Allow, types.Labels{types.Wildcard: []string{types.Wildcard}})
			},
			func(req *proto.ListUnifiedResourcesRequest) {
				req.SearchKeywords = []string{"POTATO"}
			},
		)
	})
}

// go test ./lib/auth -bench=BenchmarkListUnifiedResources -run=^$ -v -benchtime 1x
// goos: darwin
// goarch: arm64
// pkg: github.com/gravitational/teleport/lib/auth
// BenchmarkListUnifiedResources
// BenchmarkListUnifiedResources/simple_labels
// BenchmarkListUnifiedResources/simple_labels-10                 1         653696459 ns/op        480570296 B/op   8241706 allocs/op
// PASS
// ok      github.com/gravitational/teleport/lib/auth      2.878s
func BenchmarkListUnifiedResources(b *testing.B) {
	const nodeCount = 150_000
	const roleCount = 32

	ctx := context.Background()
	srv := newTestTLSServer(b)

	var ids []string
	for i := 0; i < roleCount; i++ {
		ids = append(ids, uuid.New().String())
	}

	ids[0] = "hidden"

	var hiddenNodes int
	// Create test nodes.
	for i := 0; i < nodeCount; i++ {
		name := uuid.New().String()
		id := ids[i%len(ids)]
		if id == "hidden" {
			hiddenNodes++
		}
		node, err := types.NewServerWithLabels(
			name,
			types.KindNode,
			types.ServerSpecV2{},
			map[string]string{
				"key":   id,
				"group": "users",
			},
		)
		require.NoError(b, err)

		_, err = srv.Auth().UpsertNode(ctx, node)
		require.NoError(b, err)
	}

	for _, tc := range []struct {
		desc     string
		editRole func(types.Role, string)
	}{
		{
			desc: "simple labels",
			editRole: func(r types.Role, id string) {
				if id == "hidden" {
					r.SetNodeLabels(types.Deny, types.Labels{"key": {id}})
				} else {
					r.SetNodeLabels(types.Allow, types.Labels{"key": {id}})
				}
			},
		},
	} {
		b.Run(tc.desc, func(b *testing.B) {
			benchmarkListUnifiedResources(
				b, ctx,
				nodeCount-hiddenNodes,
				srv,
				ids,
				tc.editRole,
				func(req *proto.ListUnifiedResourcesRequest) {},
			)
		})
	}
}

func benchmarkListUnifiedResources(
	b *testing.B, ctx context.Context,
	expectedCount int,
	srv *TestTLSServer,
	ids []string,
	editRole func(r types.Role, id string),
	editReq func(req *proto.ListUnifiedResourcesRequest),
) {
	var roles []types.Role
	for _, id := range ids {
		role, err := types.NewRole(fmt.Sprintf("role-%s", id), types.RoleSpecV6{})
		require.NoError(b, err)
		editRole(role, id)
		roles = append(roles, role)
	}

	// create user, role, and client
	username := "user"

	user, err := CreateUser(ctx, srv.Auth(), username, roles...)
	require.NoError(b, err)
	user.SetTraits(map[string][]string{
		"group": {"users"},
		"email": {"test@example.com"},
	})
	user, err = srv.Auth().UpsertUser(ctx, user)
	require.NoError(b, err)
	identity := TestUser(user.GetName())
	clt, err := srv.NewClient(identity)
	require.NoError(b, err)

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		var resources []*proto.PaginatedResource
		req := &proto.ListUnifiedResourcesRequest{
			SortBy: types.SortBy{IsDesc: false, Field: types.ResourceMetadataName},
			Limit:  1_000,
		}

		editReq(req)

		for {
			rsp, err := clt.ListUnifiedResources(ctx, req)
			require.NoError(b, err)

			resources = append(resources, rsp.Resources...)
			req.StartKey = rsp.NextKey
			if req.StartKey == "" {
				break
			}
		}
		require.Len(b, resources, expectedCount)
	}
}

// TestGenerateHostCert attempts to generate host certificates using various
// RBAC rules
func TestGenerateHostCert(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	clusterName := srv.ClusterName()

	_, pub, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)

	noError := func(err error) bool {
		return err == nil
	}

	for _, test := range []struct {
		desc       string
		principals []string
		skipRule   bool
		where      string
		deny       bool
		denyWhere  string
		expect     func(error) bool
	}{
		{
			desc:       "disallowed",
			skipRule:   true,
			principals: []string{"foo.example.com"},
			expect:     trace.IsAccessDenied,
		},
		{
			desc:       "denied",
			deny:       true,
			principals: []string{"foo.example.com"},
			expect:     trace.IsAccessDenied,
		},
		{
			desc:       "allowed",
			principals: []string{"foo.example.com"},
			expect:     noError,
		},
		{
			desc:       "allowed-subset",
			principals: []string{"foo.example.com"},
			where:      `is_subset(host_cert.principals, "foo.example.com", "bar.example.com")`,
			expect:     noError,
		},
		{
			desc:       "disallowed-subset",
			principals: []string{"baz.example.com"},
			where:      `is_subset(host_cert.principals, "foo.example.com", "bar.example.com")`,
			expect:     trace.IsAccessDenied,
		},
		{
			desc:       "allowed-all-equal",
			principals: []string{"foo.example.com"},
			where:      `all_equal(host_cert.principals, "foo.example.com")`,
			expect:     noError,
		},
		{
			desc:       "disallowed-all-equal",
			principals: []string{"bar.example.com"},
			where:      `all_equal(host_cert.principals, "foo.example.com")`,
			expect:     trace.IsAccessDenied,
		},
		{
			desc:       "allowed-all-end-with",
			principals: []string{"node.foo.example.com"},
			where:      `all_end_with(host_cert.principals, ".foo.example.com")`,
			expect:     noError,
		},
		{
			desc:       "disallowed-all-end-with",
			principals: []string{"node.bar.example.com"},
			where:      `all_end_with(host_cert.principals, ".foo.example.com")`,
			expect:     trace.IsAccessDenied,
		},
		{
			desc:       "allowed-complex",
			principals: []string{"foo.example.com"},
			where:      `all_end_with(host_cert.principals, ".example.com")`,
			denyWhere:  `is_subset(host_cert.principals, "bar.example.com", "baz.example.com")`,
			expect:     noError,
		},
		{
			desc:       "disallowed-complex",
			principals: []string{"bar.example.com"},
			where:      `all_end_with(host_cert.principals, ".example.com")`,
			denyWhere:  `is_subset(host_cert.principals, "bar.example.com", "baz.example.com")`,
			expect:     trace.IsAccessDenied,
		},
		{
			desc:       "allowed-multiple",
			principals: []string{"bar.example.com", "foo.example.com"},
			where:      `is_subset(host_cert.principals, "foo.example.com", "bar.example.com")`,
			expect:     noError,
		},
		{
			desc:       "disallowed-multiple",
			principals: []string{"foo.example.com", "bar.example.com", "baz.example.com"},
			where:      `is_subset(host_cert.principals, "foo.example.com", "bar.example.com")`,
			expect:     trace.IsAccessDenied,
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			rules := []types.Rule{}
			if !test.skipRule {
				rules = append(rules, types.Rule{
					Resources: []string{types.KindHostCert},
					Verbs:     []string{types.VerbCreate},
					Where:     test.where,
				})
			}

			denyRules := []types.Rule{}
			if test.deny || test.denyWhere != "" {
				denyRules = append(denyRules, types.Rule{
					Resources: []string{types.KindHostCert},
					Verbs:     []string{types.VerbCreate},
					Where:     test.denyWhere,
				})
			}

			role, err := CreateRole(ctx, srv.Auth(), test.desc, types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: rules},
				Deny:  types.RoleConditions{Rules: denyRules},
			})
			require.NoError(t, err)

			user, err := CreateUser(ctx, srv.Auth(), test.desc, role)
			require.NoError(t, err)

			client, err := srv.NewClient(TestUser(user.GetName()))
			require.NoError(t, err)

			// Try by calling new gRPC endpoint directly
			_, err = client.TrustClient().GenerateHostCert(ctx, &trustpb.GenerateHostCertRequest{
				Key:         pub,
				HostId:      "",
				NodeName:    "",
				Principals:  test.principals,
				ClusterName: clusterName,
				Role:        string(types.RoleNode),
				Ttl:         durationpb.New(0),
			})
			require.True(t, test.expect(err))
		})
	}
}

// TestLocalServiceRolesHavePermissionsForUploaderService verifies that all of Teleport's
// builtin roles have permissions to execute the calls required by the uploader service.
// This is because only one uploader service runs per Teleport process, and it will use
// the first available identity.
func TestLocalServiceRolesHavePermissionsForUploaderService(t *testing.T) {
	srv, err := NewTestAuthServer(TestAuthServerConfig{Dir: t.TempDir()})
	require.NoError(t, err, trace.DebugReport(err))

	roles := types.LocalServiceMappings()
	for _, role := range roles {
		// RoleMDM and AccessGraphPlugin services don't create events by themselves, instead they rely on
		// Auth to issue events.
		if role == types.RoleAuth || role == types.RoleMDM || role == types.RoleAccessGraphPlugin {
			continue
		}

		t.Run(role.String(), func(t *testing.T) {
			ctx := context.Background()
			identity := TestIdentity{
				I: authz.BuiltinRole{
					Role:                  types.RoleInstance,
					AdditionalSystemRoles: []types.SystemRole{role},
					Username:              string(types.RoleInstance),
				},
			}

			authContext, err := srv.Authorizer.Authorize(authz.ContextWithUser(ctx, identity.I))
			require.NoError(t, err)

			s := &ServerWithRoles{
				authServer: srv.AuthServer,
				alog:       srv.AuditLog,
				context:    *authContext,
			}

			t.Run("GetSessionTracker", func(t *testing.T) {
				sid := session.NewID()
				tracker, err := s.CreateSessionTracker(ctx, &types.SessionTrackerV1{
					ResourceHeader: types.ResourceHeader{
						Metadata: types.Metadata{
							Name: sid.String(),
						},
					},
					Spec: types.SessionTrackerSpecV1{
						SessionID: sid.String(),
					},
				})
				require.NoError(t, err)

				_, err = s.GetSessionTracker(ctx, tracker.GetSessionID())
				require.NoError(t, err)
			})

			t.Run("EmitAuditEvent", func(t *testing.T) {
				err := s.EmitAuditEvent(ctx, &apievents.UserLogin{
					Metadata: apievents.Metadata{
						Type: events.UserLoginEvent,
						Code: events.UserLocalLoginFailureCode,
					},
					Method: events.LoginMethodClientCert,
					Status: apievents.Status{Success: true},
				})
				require.NoError(t, err)
			})

			t.Run("StreamSessionEvents", func(t *testing.T) {
				// swap out the audit log with a discard log because we don't care if
				// the streaming actually succeeds, we just want to make sure RBAC checks
				// pass and allow us to enter the audit log code
				originalLog := s.alog
				t.Cleanup(func() { s.alog = originalLog })
				s.alog = events.NewDiscardAuditLog()

				eventC, errC := s.StreamSessionEvents(ctx, "foo", 0)
				select {
				case err := <-errC:
					require.NoError(t, err)
				default:
					// drain eventC to prevent goroutine leak
					for range eventC {
					}
				}
			})

			t.Run("CreateAuditStream", func(t *testing.T) {
				stream, err := s.CreateAuditStream(ctx, session.ID("streamer"))
				require.NoError(t, err)
				require.NoError(t, stream.Close(ctx))
			})

			t.Run("ResumeAuditStream", func(t *testing.T) {
				stream, err := s.ResumeAuditStream(ctx, session.ID("streamer"), "upload")
				require.NoError(t, err)
				require.NoError(t, stream.Close(ctx))
			})
		})
	}
}

func TestGetActiveSessionTrackers(t *testing.T) {
	t.Parallel()

	type activeSessionsTestCase struct {
		name        string
		makeRole    func() (types.Role, error)
		makeTracker func(testUser types.User) (types.SessionTracker, error)
		extraSetup  func(*testing.T, *TestTLSServer)

		checkSessionTrackers require.ValueAssertionFunc
	}

	for _, tc := range []activeSessionsTestCase{
		{
			name: "simple-access",
			makeRole: func() (types.Role, error) {
				return types.NewRole("foo", types.RoleSpecV6{
					Allow: types.RoleConditions{
						Rules: []types.Rule{{
							Resources: []string{types.KindSessionTracker},
							Verbs:     []string{types.VerbList, types.VerbRead},
						}},
					},
				})
			},
			makeTracker: func(testUser types.User) (types.SessionTracker, error) {
				return types.NewSessionTracker(types.SessionTrackerSpecV1{
					SessionID: "1",
					Kind:      string(types.SSHSessionKind),
				})
			},
			checkSessionTrackers: require.NotEmpty,
		},
		{
			name: "no-access-rule",
			makeRole: func() (types.Role, error) {
				return types.NewRole("foo", types.RoleSpecV6{})
			},
			makeTracker: func(testUser types.User) (types.SessionTracker, error) {
				return types.NewSessionTracker(types.SessionTrackerSpecV1{
					SessionID: "1",
					Kind:      string(types.SSHSessionKind),
				})
			},
			checkSessionTrackers: require.Empty,
		},
		{
			name: "access-with-match-expression",
			makeRole: func() (types.Role, error) {
				return types.NewRole("foo", types.RoleSpecV6{
					Allow: types.RoleConditions{
						Rules: []types.Rule{{
							Resources: []string{types.KindSessionTracker},
							Verbs:     []string{types.VerbList, types.VerbRead},
							Where:     "equals(session_tracker.session_id, \"1\")",
						}},
					},
				})
			},
			makeTracker: func(testUser types.User) (types.SessionTracker, error) {
				return types.NewSessionTracker(types.SessionTrackerSpecV1{
					SessionID: "1",
					Kind:      string(types.SSHSessionKind),
				})
			},
			checkSessionTrackers: require.NotEmpty,
		},
		{
			name: "no-access-with-match-expression",
			makeRole: func() (types.Role, error) {
				return types.NewRole("foo", types.RoleSpecV6{
					Allow: types.RoleConditions{
						Rules: []types.Rule{{
							Resources: []string{types.KindSessionTracker},
							Verbs:     []string{types.VerbList, types.VerbRead},
							Where:     "equals(session_tracker.session_id, \"1\")",
						}},
					},
				})
			},
			makeTracker: func(testUser types.User) (types.SessionTracker, error) {
				return types.NewSessionTracker(types.SessionTrackerSpecV1{
					SessionID: "2",
					Kind:      string(types.KubernetesSessionKind),
				})
			},
			checkSessionTrackers: require.Empty,
		},
		{
			name: "filter-bug-v3-role",
			makeRole: func() (types.Role, error) {
				return types.NewRoleWithVersion("dev", types.V3, types.RoleSpecV6{
					Allow: types.RoleConditions{
						AppLabels:        types.Labels{"*": []string{"*"}},
						DatabaseLabels:   types.Labels{"*": []string{"*"}},
						KubernetesLabels: types.Labels{"*": []string{"*"}},
						KubernetesResources: []types.KubernetesResource{
							{Kind: types.KindKubePod, Name: "*", Namespace: "*", Verbs: []string{"*"}},
						},
						NodeLabels:           types.Labels{"*": []string{"*"}},
						NodeLabelsExpression: `contains(user.spec.traits["cluster_ids"], labels["cluster_id"]) || contains(user.spec.traits["sub"], labels["owner"])`,
						Logins:               []string{"{{external.sub}}"},
						WindowsDesktopLabels: types.Labels{"cluster_id": []string{"{{external.cluster_ids}}"}},
						WindowsDesktopLogins: []string{"{{external.sub}}", "{{external.windows_logins}}"},
					},
					Deny: types.RoleConditions{
						Rules: []types.Rule{
							{
								Resources: []string{types.KindDatabaseServer, types.KindAppServer, types.KindSession, types.KindSSHSession, types.KindKubeService, types.KindSessionTracker},
								Verbs:     []string{"list", "read"},
							},
						},
					},
				})
			},
			makeTracker: func(testUser types.User) (types.SessionTracker, error) {
				return types.NewSessionTracker(types.SessionTrackerSpecV1{
					SessionID: "1",
					Kind:      string(types.SSHSessionKind),
				})
			},
			checkSessionTrackers: require.Empty,
		},
		{
			name: "explicit-deny-wins", // so long as the user doesn't have join permissions
			makeRole: func() (types.Role, error) {
				return types.NewRole("foo", types.RoleSpecV6{
					Allow: types.RoleConditions{
						Rules: []types.Rule{{
							Resources: []string{types.KindSessionTracker},
							Verbs:     []string{types.VerbList, types.VerbRead},
						}},
					},
					Deny: types.RoleConditions{
						Rules: []types.Rule{{
							Resources: []string{types.KindSessionTracker},
							Verbs:     []string{types.VerbList, types.VerbRead},
						}},
					},
				})
			},
			makeTracker: func(testUser types.User) (types.SessionTracker, error) {
				return types.NewSessionTracker(types.SessionTrackerSpecV1{
					SessionID: "1",
					Kind:      string(types.SSHSessionKind),
				})
			},
			checkSessionTrackers: require.Empty,
		},
		{
			// an explicit deny on session_tracker still allows listing
			// for sessions that the user can join
			name: "explicit-deny-can-join",
			makeRole: func() (types.Role, error) {
				return types.NewRole("observe-sessions", types.RoleSpecV6{
					Allow: types.RoleConditions{
						JoinSessions: []*types.SessionJoinPolicy{
							{
								Name:  "observe-kube-sessions",
								Kinds: []string{string(types.KubernetesSessionKind)},
								Modes: []string{string(types.SessionObserverMode)},
								Roles: []string{"access"},
							},
						},
					},
					Deny: types.RoleConditions{
						Rules: []types.Rule{{
							Resources: []string{types.KindSessionTracker},
							Verbs:     []string{types.VerbList, types.VerbRead},
						}},
					},
				})
			},
			extraSetup: func(t *testing.T, srv *TestTLSServer) {
				originator, err := types.NewUser("session-originator")
				require.NoError(t, err)

				originator.AddRole("access")
				_, err = srv.Auth().UpsertUser(context.Background(), originator)
				require.NoError(t, err)
			},
			makeTracker: func(testUser types.User) (types.SessionTracker, error) {
				return types.NewSessionTracker(types.SessionTrackerSpecV1{
					SessionID: "1",
					Kind:      string(types.KubernetesSessionKind),
					HostUser:  "session-originator",
					HostPolicies: []*types.SessionTrackerPolicySet{
						{Name: "access"},
					},
				})
			},
			checkSessionTrackers: require.NotEmpty,
		},
		{
			// user who can join SSH sessions should not be able to list
			// kubernetes sessions
			name: "no-access-wrong-kind",
			makeRole: func() (types.Role, error) {
				return types.NewRole("observe-sessions", types.RoleSpecV6{
					Allow: types.RoleConditions{
						JoinSessions: []*types.SessionJoinPolicy{
							{
								Name:  "observe-ssh-sessions",
								Kinds: []string{string(types.SSHSessionKind)},
								Modes: []string{string(types.SessionObserverMode)},
								Roles: []string{"access"},
							},
						},
					},
					Deny: types.RoleConditions{
						Rules: []types.Rule{{
							Resources: []string{types.KindSessionTracker},
							Verbs:     []string{types.VerbList, types.VerbRead},
						}},
					},
				})
			},
			extraSetup: func(t *testing.T, srv *TestTLSServer) {
				originator, err := types.NewUser("session-originator")
				require.NoError(t, err)

				originator.AddRole("access")
				_, err = srv.Auth().UpsertUser(context.Background(), originator)
				require.NoError(t, err)
			},
			makeTracker: func(testUser types.User) (types.SessionTracker, error) {
				return types.NewSessionTracker(types.SessionTrackerSpecV1{
					SessionID: "1",
					Kind:      string(types.KubernetesSessionKind),
					HostUser:  "session-originator",
					HostPolicies: []*types.SessionTrackerPolicySet{
						{Name: "access"},
					},
				})
			},
			checkSessionTrackers: require.Empty,
		},
		{
			// Test RFD 45 logic: an exception for the legacy ssh_session resource.
			// (Explicit deny wins, even when the user can join the session)
			name: "rfd-45-legacy-rbac",
			makeRole: func() (types.Role, error) {
				return types.NewRole("observe-sessions", types.RoleSpecV6{
					Allow: types.RoleConditions{
						JoinSessions: []*types.SessionJoinPolicy{
							{
								Name:  "observe-ssh-sessions",
								Kinds: []string{string(types.SSHSessionKind)},
								Modes: []string{string(types.SessionObserverMode)},
								Roles: []string{"access"},
							},
						},
					},
					Deny: types.RoleConditions{
						Rules: []types.Rule{{
							Resources: []string{types.KindSSHSession},
							Verbs:     []string{types.VerbList, types.VerbRead},
						}},
					},
				})
			},
			extraSetup: func(t *testing.T, srv *TestTLSServer) {
				originator, err := types.NewUser("session-originator")
				require.NoError(t, err)

				originator.AddRole("access")
				_, err = srv.Auth().UpsertUser(context.Background(), originator)
				require.NoError(t, err)
			},
			makeTracker: func(testUser types.User) (types.SessionTracker, error) {
				return types.NewSessionTracker(types.SessionTrackerSpecV1{
					SessionID: "1",
					Kind:      string(types.SSHSessionKind),
					HostUser:  "session-originator",
					HostPolicies: []*types.SessionTrackerPolicySet{
						{Name: "access"},
					},
				})
			},
			checkSessionTrackers: require.Empty,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			srv := newTestTLSServer(t)

			role, err := tc.makeRole()
			require.NoError(t, err)

			_, err = srv.Auth().CreateRole(ctx, role)
			require.NoError(t, err)

			user, err := types.NewUser(uuid.NewString())
			require.NoError(t, err)

			user.AddRole(role.GetName())
			user, err = srv.Auth().UpsertUser(ctx, user)
			require.NoError(t, err)

			if tc.extraSetup != nil {
				tc.extraSetup(t, srv)
			}

			tracker, err := tc.makeTracker(user)
			require.NoError(t, err)

			_, err = srv.Auth().CreateSessionTracker(ctx, tracker)
			require.NoError(t, err)

			clt, err := srv.NewClient(TestUser(user.GetName()))
			require.NoError(t, err)

			found, err := clt.GetActiveSessionTrackers(ctx)
			require.NoError(t, err)

			tc.checkSessionTrackers(t, found)
		})
	}
}

func TestListReleasesPermissions(t *testing.T) {
	ctx := context.Background()
	srv := newTestTLSServer(t)

	tt := []struct {
		Name         string
		Role         types.RoleSpecV6
		ErrAssertion require.BoolAssertionFunc
	}{
		{
			Name: "no permission error if user has allow rule to list downloads",
			Role: types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: []types.Rule{{
					Resources: []string{types.KindDownload},
					Verbs:     []string{types.VerbList},
				}}},
			},
			ErrAssertion: require.False,
		},
		{
			Name: "permission error if user deny allow rule to list downloads",
			Role: types.RoleSpecV6{
				Deny: types.RoleConditions{Rules: []types.Rule{{
					Resources: []string{types.KindDownload},
					Verbs:     []string{types.VerbList},
				}}},
			},
			ErrAssertion: require.True,
		},
		{
			Name: "permission error if user has no rules regarding downloads",
			Role: types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: []types.Rule{}},
			},
			ErrAssertion: require.True,
		},
	}

	for _, tc := range tt {
		t.Run(tc.Name, func(t *testing.T) {
			role, err := CreateRole(ctx, srv.Auth(), "test-role", tc.Role)
			require.NoError(t, err)

			user, err := CreateUser(ctx, srv.Auth(), "test-user", role)
			require.NoError(t, err)

			client, err := srv.NewClient(TestUser(user.GetName()))
			require.NoError(t, err)

			_, err = client.ListReleases(ctx)
			tc.ErrAssertion(t, trace.IsAccessDenied(err))
		})
	}
}

func TestGetLicensePermissions(t *testing.T) {
	ctx := context.Background()
	srv := newTestTLSServer(t)

	tt := []struct {
		Name         string
		Role         types.RoleSpecV6
		ErrAssertion require.BoolAssertionFunc
	}{
		{
			Name: "no permission error if user has allow rule to read license",
			Role: types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: []types.Rule{{
					Resources: []string{types.KindLicense},
					Verbs:     []string{types.VerbRead},
				}}},
			},
			ErrAssertion: require.False,
		},
		{
			Name: "permission error if user deny allow rule to read license",
			Role: types.RoleSpecV6{
				Deny: types.RoleConditions{Rules: []types.Rule{{
					Resources: []string{types.KindLicense},
					Verbs:     []string{types.VerbRead},
				}}},
			},
			ErrAssertion: require.True,
		},
		{
			Name: "permission error if user has no rules regarding license",
			Role: types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: []types.Rule{}},
			},
			ErrAssertion: require.True,
		},
	}

	for _, tc := range tt {
		t.Run(tc.Name, func(t *testing.T) {
			role, err := CreateRole(ctx, srv.Auth(), "test-role", tc.Role)
			require.NoError(t, err)

			user, err := CreateUser(ctx, srv.Auth(), "test-user", role)
			require.NoError(t, err)

			client, err := srv.NewClient(TestUser(user.GetName()))
			require.NoError(t, err)

			_, err = client.GetLicense(ctx)
			tc.ErrAssertion(t, trace.IsAccessDenied(err))
		})
	}
}

const errSAMLAppLabelsDenied = "access to saml_idp_service_provider denied"

func TestCreateSAMLIdPServiceProvider(t *testing.T) {
	ctx := context.Background()
	srv := newTestTLSServer(t)

	const errCreateVerbDenied = `access denied to perform action "create"`

	tt := []struct {
		name         string
		metadata     types.Metadata
		spec         types.SAMLIdPServiceProviderSpecV1
		allowRule    types.RoleConditions
		denyRule     types.RoleConditions
		eventCode    string
		errAssertion require.ErrorAssertionFunc
	}{
		{
			name: "with VerbCreate and wildcard app_labels",
			metadata: types.Metadata{
				Name: "sp1",
			},
			spec: types.SAMLIdPServiceProviderSpecV1{
				EntityDescriptor: newEntityDescriptor("sp1"),
				EntityID:         "sp1",
			},
			allowRule:    samlIdPRoleCondition(types.Labels{"*": []string{"*"}}, types.VerbCreate),
			eventCode:    events.SAMLIdPServiceProviderCreateCode,
			errAssertion: require.NoError,
		},
		{
			name: "with VerbCreate and a matching app_labels",
			metadata: types.Metadata{
				Name: "sp2",
				Labels: map[string]string{
					"env": "dev",
				},
			},
			spec: types.SAMLIdPServiceProviderSpecV1{
				EntityDescriptor: newEntityDescriptor("sp2"),
				EntityID:         "sp2",
			},
			allowRule:    samlIdPRoleCondition(types.Labels{"env": []string{"dev"}}, types.VerbCreate),
			eventCode:    events.SAMLIdPServiceProviderCreateCode,
			errAssertion: require.NoError,
		},
		{
			name: "without VerbCreate but a matching app_labels",
			metadata: types.Metadata{
				Name: "sp1",
				Labels: map[string]string{
					"env": "dev",
				},
			},
			spec: types.SAMLIdPServiceProviderSpecV1{
				EntityDescriptor: newEntityDescriptor("sp1"),
				EntityID:         "sp1",
			},
			allowRule: samlIdPRoleCondition(types.Labels{"env": []string{"dev"}}),
			eventCode: events.SAMLIdPServiceProviderCreateFailureCode,
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, errCreateVerbDenied)
			},
		},
		{
			name: "with VerbCreate but a non-matching app_labels",
			metadata: types.Metadata{
				Name: "sp1",
				Labels: map[string]string{
					"env": "prod",
				},
			},
			spec: types.SAMLIdPServiceProviderSpecV1{
				EntityDescriptor: newEntityDescriptor("sp1"),
				EntityID:         "sp1",
			},
			allowRule: samlIdPRoleCondition(types.Labels{"env": []string{"dev"}}, types.VerbCreate),
			eventCode: events.SAMLIdPServiceProviderCreateFailureCode,
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, errSAMLAppLabelsDenied)
			},
		},
		{
			name: "with allow VerbCreate and deny VerbCreate",
			metadata: types.Metadata{
				Name: "sp1",
				Labels: map[string]string{
					"env": "dev",
				},
			},
			spec: types.SAMLIdPServiceProviderSpecV1{
				EntityDescriptor: newEntityDescriptor("sp1"),
				EntityID:         "sp1",
			},
			allowRule: samlIdPRoleCondition(types.Labels{"env": []string{"dev"}}, types.VerbCreate),
			denyRule:  samlIdPRoleCondition(types.Labels{"env": []string{"dev"}}, types.VerbCreate),
			eventCode: events.SAMLIdPServiceProviderCreateFailureCode,
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, errCreateVerbDenied)
			},
		},
		{
			name: "with VerbCreate but a deny app_labels",
			metadata: types.Metadata{
				Name: "sp1",
				Labels: map[string]string{
					"env": "prod",
				},
			},
			spec: types.SAMLIdPServiceProviderSpecV1{
				EntityDescriptor: newEntityDescriptor("sp1"),
				EntityID:         "sp1",
			},
			allowRule: samlIdPRoleCondition(types.Labels{"env": []string{"dev"}}, types.VerbCreate),
			denyRule:  samlIdPRoleCondition(types.Labels{"env": []string{"dev"}}),
			eventCode: events.SAMLIdPServiceProviderCreateFailureCode,
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, errSAMLAppLabelsDenied)
			},
		},
		{
			name: "without any permissions",
			metadata: types.Metadata{
				Name: "sp1",
			},
			spec: types.SAMLIdPServiceProviderSpecV1{
				EntityDescriptor: newEntityDescriptor("sp1"),
				EntityID:         "sp1",
			},
			eventCode: events.SAMLIdPServiceProviderCreateFailureCode,
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, errCreateVerbDenied)
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			user := createSAMLIdPTestUser(t, srv.Auth(), types.RoleSpecV6{Allow: tc.allowRule, Deny: tc.denyRule})
			client, err := srv.NewClient(TestUser(user))
			require.NoError(t, err)
			t.Cleanup(func() {
				require.NoError(t, client.Close())
			})

			sp, err := types.NewSAMLIdPServiceProvider(tc.metadata, tc.spec)
			require.NoError(t, err)
			modifyAndWaitForEvent(t, tc.errAssertion, srv, tc.eventCode, func() error {
				return client.CreateSAMLIdPServiceProvider(ctx, sp)
			})
		})
	}
}

func TestUpdateSAMLIdPServiceProvider(t *testing.T) {
	ctx := context.Background()
	srv := newTestTLSServer(t)

	const errUpdateVerbDenied = `access denied to perform action "update"`

	sp := &types.SAMLIdPServiceProviderV1{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name: "sp1",
				Labels: map[string]string{
					"env": "dev",
				},
			},
		},
		Spec: types.SAMLIdPServiceProviderSpecV1{
			EntityDescriptor: newEntityDescriptor("sp1"),
			EntityID:         "sp1",
		},
	}
	require.NoError(t, srv.Auth().CreateSAMLIdPServiceProvider(ctx, sp))

	tt := []struct {
		name         string
		metadata     types.Metadata
		spec         types.SAMLIdPServiceProviderSpecV1
		allowRule    types.RoleConditions
		denyRule     types.RoleConditions
		eventCode    string
		errAssertion require.ErrorAssertionFunc
	}{
		{
			name: "with VerbUpdate and wildcard app_labels",
			metadata: types.Metadata{
				Name: "sp1",
				Labels: map[string]string{
					"env": "dev",
				},
			},
			spec: types.SAMLIdPServiceProviderSpecV1{
				EntityDescriptor: newEntityDescriptor("sp2"),
				EntityID:         "sp2",
			},
			allowRule:    samlIdPRoleCondition(types.Labels{"*": []string{"*"}}, types.VerbUpdate),
			eventCode:    events.SAMLIdPServiceProviderUpdateCode,
			errAssertion: require.NoError,
		},
		{
			name: "with VerbUpdate and a matching app_labels",
			metadata: types.Metadata{
				Name: "sp1",
				Labels: map[string]string{
					"env": "dev",
				},
			},
			spec: types.SAMLIdPServiceProviderSpecV1{
				EntityDescriptor: newEntityDescriptor("sp3"),
				EntityID:         "sp3",
			},
			allowRule:    samlIdPRoleCondition(types.Labels{"env": []string{"dev"}}, types.VerbUpdate),
			eventCode:    events.SAMLIdPServiceProviderUpdateCode,
			errAssertion: require.NoError,
		},
		{
			name: "without VerbUpdate but a matching app_labels",
			metadata: types.Metadata{
				Name: "sp1",
				Labels: map[string]string{
					"env": "dev",
				},
			},
			spec: types.SAMLIdPServiceProviderSpecV1{
				EntityDescriptor: newEntityDescriptor("sp2"),
				EntityID:         "sp2",
			},
			allowRule: samlIdPRoleCondition(types.Labels{"env": []string{"dev"}}),
			eventCode: events.SAMLIdPServiceProviderUpdateFailureCode,
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, errUpdateVerbDenied)
			},
		},
		{
			name: "with VerbUpdate but a non-matching app_labels",
			metadata: types.Metadata{
				Name: "sp1",
				Labels: map[string]string{
					"env": "prod",
				},
			},
			spec: types.SAMLIdPServiceProviderSpecV1{
				EntityDescriptor: newEntityDescriptor("sp2"),
				EntityID:         "sp2",
			},
			allowRule: samlIdPRoleCondition(types.Labels{"env": []string{"dev"}}, types.VerbUpdate),
			eventCode: events.SAMLIdPServiceProviderUpdateFailureCode,
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, errSAMLAppLabelsDenied)
			},
		},
		{
			name: "with allow VerbUpdate and deny VerbUpdate",
			metadata: types.Metadata{
				Name: "sp1",
				Labels: map[string]string{
					"env": "dev",
				},
			},
			spec: types.SAMLIdPServiceProviderSpecV1{
				EntityDescriptor: newEntityDescriptor("sp1"),
				EntityID:         "sp1",
			},
			allowRule: samlIdPRoleCondition(types.Labels{"env": []string{"dev"}}, types.VerbUpdate),
			denyRule:  samlIdPRoleCondition(types.Labels{}, types.VerbUpdate),
			eventCode: events.SAMLIdPServiceProviderUpdateFailureCode,
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, errUpdateVerbDenied)
			},
		},
		{
			name: "with VerbUpdate but a deny app_labels",
			metadata: types.Metadata{
				Name: "sp1",
				Labels: map[string]string{
					"env": "prod",
				},
			},
			spec: types.SAMLIdPServiceProviderSpecV1{
				EntityDescriptor: newEntityDescriptor("sp1"),
				EntityID:         "sp1",
			},
			allowRule: samlIdPRoleCondition(types.Labels{}, types.VerbUpdate),
			denyRule:  samlIdPRoleCondition(types.Labels{"env": []string{"prod"}}),
			eventCode: events.SAMLIdPServiceProviderUpdateFailureCode,
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, errSAMLAppLabelsDenied)
			},
		},
		{
			name: "without any permissions",
			metadata: types.Metadata{
				Name: "sp1",
			},
			spec: types.SAMLIdPServiceProviderSpecV1{
				EntityDescriptor: newEntityDescriptor("sp1"),
				EntityID:         "sp1",
			},
			eventCode: events.SAMLIdPServiceProviderUpdateFailureCode,
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, errUpdateVerbDenied)
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			user := createSAMLIdPTestUser(t, srv.Auth(), types.RoleSpecV6{Allow: tc.allowRule, Deny: tc.denyRule})

			client, err := srv.NewClient(TestUser(user))
			require.NoError(t, err)
			t.Cleanup(func() {
				require.NoError(t, client.Close())
			})

			sp, err := types.NewSAMLIdPServiceProvider(tc.metadata, tc.spec)
			require.NoError(t, err)

			modifyAndWaitForEvent(t, tc.errAssertion, srv, tc.eventCode, func() error {
				return client.UpdateSAMLIdPServiceProvider(ctx, sp)
			})
		})
	}
}

func TestCreateSAMLIdPServiceProviderInvalidInputs(t *testing.T) {
	ctx := context.Background()
	srv := newTestTLSServer(t)
	user := createSAMLIdPTestUser(t, srv.Auth(), types.RoleSpecV6{Allow: samlIdPRoleCondition(types.Labels{"*": []string{"*"}}, types.VerbCreate)})
	client, err := srv.NewClient(TestUser(user))
	require.NoError(t, err)

	tests := []struct {
		name             string
		entityDescriptor string
		entityID         string
		acsURL           string
		relayState       string
		errAssertion     require.ErrorAssertionFunc
	}{
		{
			name:     "missing url scheme in acs input",
			entityID: "sp",
			acsURL:   "sp",
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "invalid scheme")
			},
		},
		{
			name:             "missing url scheme for acs in ed",
			entityDescriptor: services.NewSAMLTestSPMetadata("sp", "sp"),
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "invalid url scheme")
			},
		},
		{
			name:     "http url scheme in acs",
			entityID: "sp",
			acsURL:   "http://sp",
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "invalid scheme")
			},
		},
		{
			name:             "http url scheme for acs in ed",
			entityDescriptor: services.NewSAMLTestSPMetadata("sp", "http://sp"),
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "unsupported ACS bindings")
			},
		},
		{
			name:     "unsupported scheme in acs",
			entityID: "sp",
			acsURL:   "gopher://sp",
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "invalid scheme")
			},
		},
		{
			name:             "unsupported scheme for acs in ed",
			entityDescriptor: services.NewSAMLTestSPMetadata("sp", "gopher://sp"),
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "invalid url scheme")
			},
		},
		{
			name:     "invalid character in acs",
			entityID: "sp",
			acsURL:   "https://sp>",
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "unsupported character")
			},
		},
		{
			name:             "invalid character in acs in ed",
			entityDescriptor: services.NewSAMLTestSPMetadata("sp", "https://sp>"),
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "unsupported ACS bindings")
			},
		},
		{
			name:       "invalid character in relay state",
			entityID:   "sp",
			acsURL:     "https://sp",
			relayState: "default_state<b",
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "unsupported character")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sp, err := types.NewSAMLIdPServiceProvider(types.Metadata{
				Name: "test",
			}, types.SAMLIdPServiceProviderSpecV1{
				EntityDescriptor: test.entityDescriptor,
				EntityID:         test.entityID,
				ACSURL:           test.acsURL,
				RelayState:       test.relayState,
			})
			require.NoError(t, err)

			err = client.CreateSAMLIdPServiceProvider(ctx, sp)
			test.errAssertion(t, err)
		})
	}
}

func TestUpdateSAMLIdPServiceProviderInvalidInputs(t *testing.T) {
	ctx := context.Background()
	srv := newTestTLSServer(t)
	user := createSAMLIdPTestUser(t, srv.Auth(), types.RoleSpecV6{Allow: samlIdPRoleCondition(types.Labels{"*": []string{"*"}}, types.VerbCreate, types.VerbUpdate)})
	client, err := srv.NewClient(TestUser(user))
	require.NoError(t, err)

	sp, err := types.NewSAMLIdPServiceProvider(types.Metadata{
		Name: "sp",
	}, types.SAMLIdPServiceProviderSpecV1{
		EntityDescriptor: services.NewSAMLTestSPMetadata("https://sp", "https://sp"),
	})
	require.NoError(t, err)

	err = client.CreateSAMLIdPServiceProvider(ctx, sp)
	require.NoError(t, err)

	tests := []struct {
		name             string
		entityDescriptor string
		entityID         string
		acsURL           string
		relayState       string
		errAssertion     require.ErrorAssertionFunc
	}{
		{
			name:             "missing url scheme for acs in ed",
			entityDescriptor: services.NewSAMLTestSPMetadata("https://sp", "sp"),
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "invalid url scheme")
			},
		},
		{
			name:             "http url scheme for acs in ed",
			entityDescriptor: services.NewSAMLTestSPMetadata("https://sp", "http://sp"),
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "unsupported ACS bindings")
			},
		},
		{
			name:             "unsupported scheme for acs in ed",
			entityDescriptor: services.NewSAMLTestSPMetadata("https://sp", "gopher://sp"),
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "invalid url scheme")
			},
		},
		{
			name:             "invalid character in acs in ed",
			entityDescriptor: services.NewSAMLTestSPMetadata("https://sp", "https://sp>"),
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "unsupported ACS bindings")
			},
		},
		{
			name:             "invalid character in relay state",
			entityDescriptor: services.NewSAMLTestSPMetadata("https://sp", "https://sp"),
			relayState:       "default_state<b",
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "unsupported character")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sp, err := types.NewSAMLIdPServiceProvider(types.Metadata{
				Name: "sp",
			}, types.SAMLIdPServiceProviderSpecV1{
				EntityDescriptor: test.entityDescriptor,
				RelayState:       test.relayState,
			})
			require.NoError(t, err)

			err = client.UpdateSAMLIdPServiceProvider(ctx, sp)
			test.errAssertion(t, err)
		})
	}
}

func TestDeleteSAMLIdPServiceProvider(t *testing.T) {
	ctx := context.Background()
	srv := newTestTLSServer(t)

	const errDeleteVerbDenied = `access denied to perform action "delete"`

	sp := &types.SAMLIdPServiceProviderV1{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name: "sp1",
				Labels: map[string]string{
					"env": "dev",
				},
			},
		},
		Spec: types.SAMLIdPServiceProviderSpecV1{
			EntityDescriptor: newEntityDescriptor("sp1"),
			EntityID:         "sp1",
		},
	}
	require.NoError(t, srv.Auth().CreateSAMLIdPServiceProvider(ctx, sp))

	tt := []struct {
		name         string
		spName       string
		allowRule    types.RoleConditions
		denyRule     types.RoleConditions
		eventCode    string
		errAssertion require.ErrorAssertionFunc
	}{
		{
			name:      "without VerbDelete but a matching app_labels",
			spName:    "sp1",
			allowRule: samlIdPRoleCondition(types.Labels{"env": []string{"dev"}}),
			eventCode: events.SAMLIdPServiceProviderDeleteFailureCode,
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, errDeleteVerbDenied)
			},
		},
		{
			name:      "with VerbDelete but a non-matching app_labels",
			spName:    "sp1",
			allowRule: samlIdPRoleCondition(types.Labels{"env": []string{"prod"}}, types.VerbDelete),
			eventCode: events.SAMLIdPServiceProviderDeleteFailureCode,
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, errSAMLAppLabelsDenied)
			},
		},
		{
			name:      "with allow VerbDelete and deny VerbDelete",
			spName:    "sp1",
			allowRule: samlIdPRoleCondition(types.Labels{"env": []string{"dev"}}, types.VerbDelete),
			denyRule:  samlIdPRoleCondition(types.Labels{}, types.VerbDelete),
			eventCode: events.SAMLIdPServiceProviderDeleteFailureCode,
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, errDeleteVerbDenied)
			},
		},
		{
			name:      "with VerbDelete but a deny app_labels",
			spName:    "sp1",
			allowRule: samlIdPRoleCondition(types.Labels{"env": []string{"dev"}}, types.VerbDelete),
			denyRule:  samlIdPRoleCondition(types.Labels{"env": []string{"dev"}}),
			eventCode: events.SAMLIdPServiceProviderDeleteFailureCode,
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, errSAMLAppLabelsDenied)
			},
		},
		{
			name:      "without any permissions",
			spName:    "sp1",
			eventCode: events.SAMLIdPServiceProviderDeleteFailureCode,
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, errDeleteVerbDenied)
			},
		},
		{
			name:         "with VerbDelete and a matching app_labels",
			spName:       "sp1",
			allowRule:    samlIdPRoleCondition(types.Labels{"env": []string{"dev"}}, types.VerbDelete),
			eventCode:    events.SAMLIdPServiceProviderDeleteCode,
			errAssertion: require.NoError,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			user := createSAMLIdPTestUser(t, srv.Auth(), types.RoleSpecV6{Allow: tc.allowRule, Deny: tc.denyRule})

			client, err := srv.NewClient(TestUser(user))
			require.NoError(t, err)
			t.Cleanup(func() {
				require.NoError(t, client.Close())
			})

			modifyAndWaitForEvent(t, tc.errAssertion, srv, tc.eventCode, func() error {
				return client.DeleteSAMLIdPServiceProvider(ctx, tc.spName)
			})
		})
	}
}

func TestDeleteAllSAMLIdPServiceProviders(t *testing.T) {
	ctx := context.Background()
	srv := newTestTLSServer(t)

	const errDeleteVerbDenied = `access denied to perform action "delete"`

	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("saml-app-%v", i)
		sp, err := types.NewSAMLIdPServiceProvider(types.Metadata{
			Name:   name,
			Labels: map[string]string{"env": "dev"},
		}, types.SAMLIdPServiceProviderSpecV1{
			EntityDescriptor: newEntityDescriptor(fmt.Sprintf("entity-id-%v", i)),
			EntityID:         fmt.Sprintf("entity-id-%v", i),
		})
		require.NoError(t, err)
		err = srv.Auth().CreateSAMLIdPServiceProvider(ctx, sp)
		require.NoError(t, err)
	}

	tt := []struct {
		name         string
		allowRule    types.RoleConditions
		denyRule     types.RoleConditions
		eventCode    string
		errAssertion require.ErrorAssertionFunc
	}{
		{
			name:      "without VerbDelete but a matching app_labels",
			allowRule: samlIdPRoleCondition(types.Labels{"env": []string{"dev"}}),
			eventCode: events.SAMLIdPServiceProviderDeleteAllFailureCode,
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, errDeleteVerbDenied)
			},
		},
		{
			name:      "with VerbDelete but a non-matching app_labels",
			allowRule: samlIdPRoleCondition(types.Labels{}, types.VerbDelete),
			eventCode: events.SAMLIdPServiceProviderDeleteAllFailureCode,
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, errSAMLAppLabelsDenied)
			},
		},
		{
			name:      "with allow VerbDelete and deny VerbDelete",
			allowRule: samlIdPRoleCondition(types.Labels{"env": []string{"dev"}}, types.VerbDelete),
			denyRule:  samlIdPRoleCondition(types.Labels{"env": []string{"dev"}}, types.VerbDelete),
			eventCode: events.SAMLIdPServiceProviderDeleteAllFailureCode,
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, errDeleteVerbDenied)
			},
		},
		{
			name:      "with VerbDelete but a deny app_labels",
			allowRule: samlIdPRoleCondition(types.Labels{"env": []string{"dev"}}, types.VerbDelete),
			denyRule:  samlIdPRoleCondition(types.Labels{"env": []string{"dev"}}),
			eventCode: events.SAMLIdPServiceProviderDeleteAllFailureCode,
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, errSAMLAppLabelsDenied)
			},
		},
		{
			name:      "without any permissions",
			eventCode: events.SAMLIdPServiceProviderDeleteAllFailureCode,
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, errDeleteVerbDenied)
			},
		},
		{
			name:         "with VerbDelete and a matching app_labels",
			allowRule:    samlIdPRoleCondition(types.Labels{"env": []string{"dev"}}, types.VerbDelete),
			eventCode:    events.SAMLIdPServiceProviderDeleteAllCode,
			errAssertion: require.NoError,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			user := createSAMLIdPTestUser(t, srv.Auth(), types.RoleSpecV6{Allow: tc.allowRule, Deny: tc.denyRule})

			client, err := srv.NewClient(TestUser(user))
			require.NoError(t, err)
			t.Cleanup(func() {
				require.NoError(t, client.Close())
			})

			modifyAndWaitForEvent(t, tc.errAssertion, srv, tc.eventCode, func() error {
				return client.DeleteAllSAMLIdPServiceProviders(ctx)
			})
		})
	}
}

func createSAMLIdPTestUser(t *testing.T, server *Server, userRole types.RoleSpecV6) string {
	ctx := context.Background()

	role, err := CreateRole(ctx, server, "test-empty", userRole)
	require.NoError(t, err)

	user, err := CreateUser(ctx, server, "test-user", role)
	require.NoError(t, err)

	return user.GetName()
}

func samlIdPRoleCondition(label types.Labels, verb ...string) (rc types.RoleConditions) {
	if label != nil {
		rc.AppLabels = label
	}
	if verb != nil {
		rc.Rules = []types.Rule{
			{
				Resources: []string{types.KindSAMLIdPServiceProvider},
				Verbs:     verb,
			},
		}
	}
	return
}

// modifyAndWaitForEvent performs the function fn() and then waits for the given event.
func modifyAndWaitForEvent(t *testing.T, errFn require.ErrorAssertionFunc, srv *TestTLSServer, eventCode string, fn func() error) apievents.AuditEvent {
	// Make sure we ignore events after consuming this one.
	defer func() {
		srv.AuthServer.AuthServer.emitter = events.NewDiscardEmitter()
	}()
	chanEmitter := eventstest.NewChannelEmitter(1)
	srv.AuthServer.AuthServer.emitter = chanEmitter
	err := fn()
	errFn(t, err)
	select {
	case event := <-chanEmitter.C():
		require.Equal(t, eventCode, event.GetCode())
		return event
	case <-time.After(5 * time.Second):
		require.Fail(t, "timeout waiting for update event")
	}
	return nil
}

// newTestHeadlessAuthn returns the headless authentication resource
// used across headless authentication tests.
func newTestHeadlessAuthn(t *testing.T, user string, clock clockwork.Clock) *types.HeadlessAuthentication {
	sshKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.Ed25519)
	require.NoError(t, err)
	sshPub, err := ssh.NewPublicKey(sshKey.Public())
	require.NoError(t, err)
	sshPubKey := ssh.MarshalAuthorizedKey(sshPub)

	tlsKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)
	tlsPubKey, err := keys.MarshalPublicKey(tlsKey.Public())
	require.NoError(t, err)

	headlessID := services.NewHeadlessAuthenticationID(sshPubKey)
	headlessAuthn := &types.HeadlessAuthentication{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name: headlessID,
			},
		},
		User:            user,
		SshPublicKey:    sshPubKey,
		TlsPublicKey:    tlsPubKey,
		ClientIpAddress: "0.0.0.0",
	}
	headlessAuthn.SetExpiry(clock.Now().Add(time.Minute))

	err = headlessAuthn.CheckAndSetDefaults()
	require.NoError(t, err)

	return headlessAuthn
}

func TestGetHeadlessAuthentication(t *testing.T) {
	ctx := context.Background()
	username := "teleport-user"
	otherUsername := "other-user"

	srv := newTestTLSServer(t)
	_, _, err := CreateUserAndRole(srv.Auth(), username, nil, nil)
	require.NoError(t, err)
	_, _, err = CreateUserAndRole(srv.Auth(), otherUsername, nil, nil)
	require.NoError(t, err)

	assertTimeout := func(t require.TestingT, err error, i ...interface{}) {
		require.Error(t, err)
		require.ErrorContains(t, err, context.DeadlineExceeded.Error(), "expected context deadline error but got: %v", err)
	}

	assertAccessDenied := func(t require.TestingT, err error, i ...interface{}) {
		require.Error(t, err)
		require.True(t, trace.IsAccessDenied(err), "expected access denied error but got: %v", err)
	}

	for _, tc := range []struct {
		name                  string
		headlessID            string
		identity              TestIdentity
		assertError           require.ErrorAssertionFunc
		expectedHeadlessAuthn *types.HeadlessAuthentication
	}{
		{
			name:        "OK same user",
			identity:    TestUser(username),
			assertError: require.NoError,
		}, {
			name:        "NOK not found",
			headlessID:  uuid.NewString(),
			identity:    TestUser(username),
			assertError: assertTimeout,
		}, {
			name:        "NOK different user",
			identity:    TestUser(otherUsername),
			assertError: assertTimeout,
		}, {
			name:        "NOK admin",
			identity:    TestAdmin(),
			assertError: assertAccessDenied,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// create headless authn
			headlessAuthn := newTestHeadlessAuthn(t, username, srv.Auth().clock)
			err := srv.Auth().UpsertHeadlessAuthentication(ctx, headlessAuthn)
			require.NoError(t, err)
			client, err := srv.NewClient(tc.identity)
			require.NoError(t, err)

			ctx, cancel := context.WithTimeout(ctx, time.Second)
			defer cancel()

			// default to same headlessAuthn
			if tc.headlessID == "" {
				tc.headlessID = headlessAuthn.GetName()
			}

			retrievedHeadlessAuthn, err := client.GetHeadlessAuthentication(ctx, tc.headlessID)
			tc.assertError(t, err)
			if err == nil {
				require.Equal(t, headlessAuthn, retrievedHeadlessAuthn)
			}
		})
	}
}

func TestUpdateHeadlessAuthenticationState(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	otherUsername := "other-user"

	srv := newTestTLSServer(t)
	mockEmitter := &eventstest.MockRecorderEmitter{}
	srv.Auth().emitter = mockEmitter
	mfa := configureForMFA(t, srv)

	_, _, err := CreateUserAndRole(srv.Auth(), otherUsername, nil, nil)
	require.NoError(t, err)

	assertNotFound := func(t require.TestingT, err error, i ...interface{}) {
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err), "expected not found error but got: %v", err)
	}

	assertAccessDenied := func(t require.TestingT, err error, i ...interface{}) {
		require.Error(t, err)
		require.True(t, trace.IsAccessDenied(err), "expected access denied error but got: %v", err)
	}

	for _, tc := range []struct {
		name string
		// defaults to the mfa identity tied to the headless authentication created
		identity TestIdentity
		// defaults to id of the headless authentication created
		headlessID   string
		state        types.HeadlessAuthenticationState
		withMFA      bool
		assertError  require.ErrorAssertionFunc
		assertEvents func(*testing.T, *eventstest.MockRecorderEmitter)
	}{
		{
			name:        "OK same user denied",
			state:       types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_DENIED,
			assertError: require.NoError,
			assertEvents: func(t *testing.T, emitter *eventstest.MockRecorderEmitter) {
				require.Len(t, emitter.Events(), 1)
				require.Equal(t, events.UserHeadlessLoginRejectedCode, emitter.LastEvent().GetCode())
			},
		}, {
			name:        "OK same user approved with mfa",
			state:       types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_APPROVED,
			withMFA:     true,
			assertError: require.NoError,
			assertEvents: func(t *testing.T, emitter *eventstest.MockRecorderEmitter) {
				require.Len(t, emitter.Events(), 2)
				require.Equal(t, events.ValidateMFAAuthResponseCode, emitter.Events()[0].GetCode())
				require.Equal(t, events.UserHeadlessLoginApprovedCode, emitter.Events()[1].GetCode())
			},
		}, {
			name:        "NOK same user approved without mfa",
			state:       types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_APPROVED,
			withMFA:     false,
			assertError: assertAccessDenied,
			assertEvents: func(t *testing.T, emitter *eventstest.MockRecorderEmitter) {
				require.Len(t, emitter.Events(), 2)
				require.Equal(t, events.ValidateMFAAuthResponseFailureCode, emitter.Events()[0].GetCode())
				require.Equal(t, events.UserHeadlessLoginApprovedFailureCode, emitter.Events()[1].GetCode())
			},
		}, {
			name:        "NOK not found",
			headlessID:  uuid.NewString(),
			state:       types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_DENIED,
			assertError: assertNotFound,
		}, {
			name:        "NOK different user not found",
			state:       types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_DENIED,
			identity:    TestUser(otherUsername),
			assertError: assertNotFound,
		}, {
			name:        "NOK different user approved",
			state:       types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_APPROVED,
			identity:    TestUser(otherUsername),
			assertError: assertNotFound,
		}, {
			name:        "NOK admin denied",
			state:       types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_DENIED,
			identity:    TestAdmin(),
			assertError: assertAccessDenied,
		}, {
			name:        "NOK admin approved",
			state:       types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_APPROVED,
			identity:    TestAdmin(),
			assertError: assertAccessDenied,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// create headless authn
			headlessAuthn := newTestHeadlessAuthn(t, mfa.User, srv.Auth().clock)
			headlessAuthn.State = types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_PENDING
			err := srv.Auth().UpsertHeadlessAuthentication(ctx, headlessAuthn)
			require.NoError(t, err)

			// default to mfa user
			if tc.identity.I == nil {
				tc.identity = TestUser(mfa.User)
			}

			client, err := srv.NewClient(tc.identity)
			require.NoError(t, err)

			// default to failed mfa challenge response
			resp := &proto.MFAAuthenticateResponse{
				Response: &proto.MFAAuthenticateResponse_Webauthn{
					Webauthn: &wanpb.CredentialAssertionResponse{
						Type: "bad response",
					},
				},
			}

			if tc.withMFA {
				client, err := srv.NewClient(TestUser(mfa.User))
				require.NoError(t, err)

				challenge, err := client.CreateAuthenticateChallenge(ctx, &proto.CreateAuthenticateChallengeRequest{
					Request: &proto.CreateAuthenticateChallengeRequest_ContextUser{},
					ChallengeExtensions: &mfav1.ChallengeExtensions{
						Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_HEADLESS_LOGIN,
					},
				})
				require.NoError(t, err)

				resp, err = mfa.WebDev.SolveAuthn(challenge)
				require.NoError(t, err)
			}

			// default to same headlessAuthn
			if tc.headlessID == "" {
				tc.headlessID = headlessAuthn.GetName()
			}

			ctx, cancel := context.WithTimeout(ctx, time.Second)
			defer cancel()

			mockEmitter.Reset()
			err = client.UpdateHeadlessAuthenticationState(ctx, tc.headlessID, tc.state, resp)
			tc.assertError(t, err)

			if tc.assertEvents != nil {
				tc.assertEvents(t, mockEmitter)
			} else {
				require.Empty(t, mockEmitter.Events())
			}
		})
	}
}

func TestGenerateCertAuthorityCRL(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv, err := NewTestAuthServer(TestAuthServerConfig{Dir: t.TempDir()})
	require.NoError(t, err)

	// Create a test user.
	_, err = CreateUser(ctx, srv.AuthServer.Services, "username")
	require.NoError(t, err)

	for _, tc := range []struct {
		desc      string
		identity  TestIdentity
		assertErr require.ErrorAssertionFunc
	}{
		{
			desc:      "AdminRole",
			identity:  TestAdmin(),
			assertErr: require.NoError,
		},
		{
			desc:      "User",
			identity:  TestUser("username"),
			assertErr: require.NoError,
		},
		{
			desc:      "WindowsDesktopService",
			identity:  TestBuiltin(types.RoleWindowsDesktop),
			assertErr: require.NoError,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			authContext, err := srv.Authorizer.Authorize(authz.ContextWithUser(ctx, tc.identity.I))
			require.NoError(t, err)

			s := &ServerWithRoles{
				authServer: srv.AuthServer,
				alog:       srv.AuditLog,
				context:    *authContext,
			}

			_, err = s.GenerateCertAuthorityCRL(ctx, types.UserCA)
			tc.assertErr(t, err)
		})
	}
}

func TestCreateSnowflakeSession(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)
	alice, bob, admin := createSessionTestUsers(t, srv.Auth())

	tests := map[string]struct {
		identity  TestIdentity
		assertErr require.ErrorAssertionFunc
	}{
		"as db service": {
			identity:  TestBuiltin(types.RoleDatabase),
			assertErr: require.NoError,
		},
		"as session user": {
			identity:  TestUser(alice),
			assertErr: require.NoError,
		},
		"as other user": {
			identity:  TestUser(bob),
			assertErr: require.Error,
		},
		"as admin user": {
			identity:  TestUser(admin),
			assertErr: require.NoError,
		},
	}
	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			client, err := srv.NewClient(test.identity)
			require.NoError(t, err)
			_, err = client.CreateSnowflakeSession(ctx, types.CreateSnowflakeSessionRequest{
				Username:     alice,
				TokenTTL:     time.Minute * 15,
				SessionToken: "test-token-123",
			})
			test.assertErr(t, err)
		})
	}
}

func TestGetSnowflakeSession(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)
	alice, bob, admin := createSessionTestUsers(t, srv.Auth())
	dbClient, err := srv.NewClient(TestBuiltin(types.RoleDatabase))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	// setup a session to get, for user "alice".
	sess, err := dbClient.CreateSnowflakeSession(ctx, types.CreateSnowflakeSessionRequest{
		Username:     alice,
		TokenTTL:     time.Minute * 15,
		SessionToken: "abc123",
	})
	require.NoError(t, err)

	tests := map[string]struct {
		identity  TestIdentity
		assertErr require.ErrorAssertionFunc
	}{
		"as db service": {
			identity:  TestBuiltin(types.RoleDatabase),
			assertErr: require.NoError,
		},
		"as session user": {
			identity:  TestUser(alice),
			assertErr: require.NoError,
		},
		"as other user": {
			identity:  TestUser(bob),
			assertErr: require.Error,
		},
		"as admin user": {
			identity:  TestUser(admin),
			assertErr: require.NoError,
		},
	}

	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			client, err := srv.NewClient(test.identity)
			require.NoError(t, err)
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			_, err = client.GetSnowflakeSession(ctx, types.GetSnowflakeSessionRequest{
				SessionID: sess.GetName(),
			})
			test.assertErr(t, err)
		})
	}
}

func TestGetSnowflakeSessions(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)
	alice, _, admin := createSessionTestUsers(t, srv.Auth())

	tests := map[string]struct {
		identity  TestIdentity
		assertErr require.ErrorAssertionFunc
	}{
		"as db service": {
			identity:  TestBuiltin(types.RoleDatabase),
			assertErr: require.NoError,
		},
		"as user": {
			identity:  TestUser(alice),
			assertErr: require.Error,
		},
		"as admin": {
			identity:  TestUser(admin),
			assertErr: require.NoError,
		},
	}

	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			client, err := srv.NewClient(test.identity)
			require.NoError(t, err)
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			_, err = client.GetSnowflakeSessions(ctx)
			test.assertErr(t, err)
		})
	}
}

func TestDeleteSnowflakeSession(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)
	alice, bob, admin := createSessionTestUsers(t, srv.Auth())
	tests := map[string]struct {
		identity  TestIdentity
		assertErr require.ErrorAssertionFunc
	}{
		"as db service": {
			identity:  TestBuiltin(types.RoleDatabase),
			assertErr: require.NoError,
		},
		"as session user": {
			identity:  TestUser(alice),
			assertErr: require.NoError,
		},
		"as other user": {
			identity:  TestUser(bob),
			assertErr: require.Error,
		},
		"as admin user": {
			identity:  TestUser(admin),
			assertErr: require.NoError,
		},
	}

	dbClient, err := srv.NewClient(TestBuiltin(types.RoleDatabase))
	require.NoError(t, err)
	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			sess, err := dbClient.CreateSnowflakeSession(ctx, types.CreateSnowflakeSessionRequest{
				Username:     alice,
				TokenTTL:     time.Minute * 15,
				SessionToken: "abc123",
			})
			require.NoError(t, err)
			client, err := srv.NewClient(test.identity)
			require.NoError(t, err)
			err = client.DeleteSnowflakeSession(ctx, types.DeleteSnowflakeSessionRequest{
				SessionID: sess.GetName(),
			})
			test.assertErr(t, err)
		})
	}
}

func TestDeleteAllSnowflakeSessions(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)
	alice, _, admin := createSessionTestUsers(t, srv.Auth())

	tests := map[string]struct {
		identity  TestIdentity
		assertErr require.ErrorAssertionFunc
	}{
		"as db service": {
			identity:  TestBuiltin(types.RoleDatabase),
			assertErr: require.NoError,
		},
		"as user": {
			identity:  TestUser(alice),
			assertErr: require.Error,
		},
		"as admin user": {
			identity:  TestUser(admin),
			assertErr: require.NoError,
		},
	}

	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			client, err := srv.NewClient(test.identity)
			require.NoError(t, err)
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			err = client.DeleteAllSnowflakeSessions(ctx)
			test.assertErr(t, err)
		})
	}
}

func TestCreateSAMLIdPSession(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)
	alice, bob, admin := createSessionTestUsers(t, srv.Auth())

	tests := map[string]struct {
		identity  TestIdentity
		assertErr require.ErrorAssertionFunc
	}{
		"as proxy user": {
			identity:  TestBuiltin(types.RoleProxy),
			assertErr: require.NoError,
		},
		"as session user": {
			identity:  TestUser(alice),
			assertErr: require.Error,
		},
		"as other user": {
			identity:  TestUser(bob),
			assertErr: require.Error,
		},
		"as admin user": {
			identity:  TestUser(admin),
			assertErr: require.Error,
		},
	}
	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			client, err := srv.NewClient(test.identity)
			require.NoError(t, err)
			_, err = client.CreateSAMLIdPSession(ctx, types.CreateSAMLIdPSessionRequest{
				SessionID:   "test",
				Username:    alice,
				SAMLSession: &types.SAMLSessionData{},
			})
			test.assertErr(t, err)
		})
	}
}

func TestGetSAMLIdPSession(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)
	alice, bob, admin := createSessionTestUsers(t, srv.Auth())

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	sess, err := srv.Auth().CreateSAMLIdPSession(ctx, types.CreateSAMLIdPSessionRequest{
		SessionID:   "test",
		Username:    alice,
		SAMLSession: &types.SAMLSessionData{},
	})
	require.NoError(t, err)

	tests := map[string]struct {
		identity  TestIdentity
		assertErr require.ErrorAssertionFunc
	}{
		"as proxy service": {
			identity:  TestBuiltin(types.RoleProxy),
			assertErr: require.NoError,
		},
		"as session user": {
			identity:  TestUser(alice),
			assertErr: require.Error,
		},
		"as other user": {
			identity:  TestUser(bob),
			assertErr: require.Error,
		},
		"as admin user": {
			identity:  TestUser(admin),
			assertErr: require.NoError,
		},
	}

	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			client, err := srv.NewClient(test.identity)
			require.NoError(t, err)
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			_, err = client.GetSAMLIdPSession(ctx, types.GetSAMLIdPSessionRequest{
				SessionID: sess.GetName(),
			})
			test.assertErr(t, err)
		})
	}
}

func TestListSAMLIdPSessions(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)
	alice, _, admin := createSessionTestUsers(t, srv.Auth())

	tests := map[string]struct {
		identity  TestIdentity
		assertErr require.ErrorAssertionFunc
	}{
		"as proxy service": {
			identity:  TestBuiltin(types.RoleProxy),
			assertErr: require.NoError,
		},
		"as user": {
			identity:  TestUser(alice),
			assertErr: require.Error,
		},
		"as admin": {
			identity:  TestUser(admin),
			assertErr: require.NoError,
		},
	}

	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			client, err := srv.NewClient(test.identity)
			require.NoError(t, err)
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			_, _, err = client.ListSAMLIdPSessions(ctx, 0, "", "")
			test.assertErr(t, err)
		})
	}
}

func TestDeleteSAMLIdPSession(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)
	alice, bob, admin := createSessionTestUsers(t, srv.Auth())
	tests := map[string]struct {
		identity  TestIdentity
		assertErr require.ErrorAssertionFunc
	}{
		"as proxy service": {
			identity:  TestBuiltin(types.RoleProxy),
			assertErr: require.NoError,
		},
		"as session user": {
			identity:  TestUser(alice),
			assertErr: require.NoError,
		},
		"as other user": {
			identity:  TestUser(bob),
			assertErr: require.Error,
		},
		"as admin user": {
			identity:  TestUser(admin),
			assertErr: require.NoError,
		},
	}

	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			sess, err := srv.Auth().CreateSAMLIdPSession(ctx, types.CreateSAMLIdPSessionRequest{
				SessionID:   uuid.NewString(),
				Username:    alice,
				SAMLSession: &types.SAMLSessionData{},
			})
			require.NoError(t, err)
			client, err := srv.NewClient(test.identity)
			require.NoError(t, err)
			err = client.DeleteSAMLIdPSession(ctx, types.DeleteSAMLIdPSessionRequest{
				SessionID: sess.GetName(),
			})
			test.assertErr(t, err)
		})
	}
}

func TestDeleteAllSAMLIdPSessions(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)
	alice, _, admin := createSessionTestUsers(t, srv.Auth())

	tests := map[string]struct {
		identity  TestIdentity
		assertErr require.ErrorAssertionFunc
	}{
		"as proxy service": {
			identity:  TestBuiltin(types.RoleProxy),
			assertErr: require.NoError,
		},
		"as user": {
			identity:  TestUser(alice),
			assertErr: require.Error,
		},
		"as admin user": {
			identity:  TestUser(admin),
			assertErr: require.NoError,
		},
	}

	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			client, err := srv.NewClient(test.identity)
			require.NoError(t, err)
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			err = client.DeleteAllSAMLIdPSessions(ctx)
			test.assertErr(t, err)
		})
	}
}

// Create test users for web session CRUD authz tests.
func createSessionTestUsers(t *testing.T, authServer *Server) (string, string, string) {
	t.Helper()
	// create alice and bob who have no permissions.
	_, _, err := CreateUserAndRole(authServer, "alice", nil, []types.Rule{})
	require.NoError(t, err)
	_, _, err = CreateUserAndRole(authServer, "bob", nil, []types.Rule{})
	require.NoError(t, err)
	// create "admin" who has read/write on users and web sessions.
	_, _, err = CreateUserAndRole(authServer, "admin", nil, []types.Rule{
		types.NewRule(types.KindUser, services.RW()),
		types.NewRule(types.KindWebSession, services.RW()),
	})
	require.NoError(t, err)
	return "alice", "bob", "admin"
}

func TestCreateAccessRequest(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	srv := newTestTLSServer(t)
	clock := srv.Clock()
	alice, bob, admin := createSessionTestUsers(t, srv.Auth())

	searchRole, err := types.NewRole("searchRole", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Request: &types.AccessRequestConditions{
				Roles:         []string{"requestRole"},
				SearchAsRoles: []string{"requestRole"},
			},
		},
	})
	require.NoError(t, err)

	requestRole, err := types.NewRole("requestRole", types.RoleSpecV6{
		Allow: types.RoleConditions{
			GroupLabels: types.Labels{
				types.Wildcard: []string{types.Wildcard},
			},
			NodeLabels: types.Labels{
				types.Wildcard: []string{types.Wildcard},
			},
		},
	})
	require.NoError(t, err)

	nodeAllowedByRequestRole, err := types.NewServerWithLabels(
		"test-node",
		types.KindNode,
		types.ServerSpecV2{},
		map[string]string{"any-key": "any-val"},
	)
	require.NoError(t, err)

	_, err = srv.Auth().UpsertNode(ctx, nodeAllowedByRequestRole)
	require.NoError(t, err)
	_, err = srv.Auth().CreateRole(ctx, requestRole)
	require.NoError(t, err)
	_, err = srv.Auth().CreateRole(ctx, searchRole)
	require.NoError(t, err)

	user, err := srv.Auth().GetUser(ctx, alice, true)
	require.NoError(t, err)

	user.AddRole(searchRole.GetName())
	_, err = srv.Auth().UpsertUser(ctx, user)
	require.NoError(t, err)

	userGroup1, err := types.NewUserGroup(types.Metadata{
		Name: "user-group1",
	}, types.UserGroupSpecV1{
		Applications: []string{"app1", "app2", "app3"},
	})
	require.NoError(t, err)
	require.NoError(t, srv.Auth().CreateUserGroup(ctx, userGroup1))

	userGroup2, err := types.NewUserGroup(types.Metadata{
		Name: "user-group2",
	}, types.UserGroupSpecV1{})
	require.NoError(t, err)
	require.NoError(t, srv.Auth().CreateUserGroup(ctx, userGroup2))

	userGroup3, err := types.NewUserGroup(types.Metadata{
		Name: "user-group3",
	}, types.UserGroupSpecV1{
		Applications: []string{"app1", "app4", "app5"},
	})
	require.NoError(t, err)
	require.NoError(t, srv.Auth().CreateUserGroup(ctx, userGroup3))

	tests := []struct {
		name             string
		user             string
		accessRequest    types.AccessRequest
		errAssertionFunc require.ErrorAssertionFunc
		expected         types.AccessRequest
	}{
		{
			name: "user creates own pending access request",
			user: alice,
			accessRequest: mustAccessRequest(t, alice, types.RequestState_PENDING, clock.Now(), clock.Now().Add(time.Hour),
				[]string{requestRole.GetName()}, []types.ResourceID{
					mustResourceID(srv.ClusterName(), nodeAllowedByRequestRole.GetKind(), nodeAllowedByRequestRole.GetName()),
				}),
			errAssertionFunc: require.NoError,
			expected: mustAccessRequest(t, alice, types.RequestState_PENDING, clock.Now(), clock.Now().Add(time.Hour),
				[]string{requestRole.GetName()}, []types.ResourceID{
					mustResourceID(srv.ClusterName(), nodeAllowedByRequestRole.GetKind(), nodeAllowedByRequestRole.GetName()),
				}),
		},
		{
			name: "admin creates a request for alice",
			user: admin,
			accessRequest: mustAccessRequest(t, alice, types.RequestState_PENDING, clock.Now(), clock.Now().Add(time.Hour),
				[]string{requestRole.GetName()}, []types.ResourceID{
					mustResourceID(srv.ClusterName(), types.KindUserGroup, userGroup1.GetName()),
				}),
			errAssertionFunc: require.NoError,
			expected: mustAccessRequest(t, alice, types.RequestState_PENDING, clock.Now(), clock.Now().Add(time.Hour),
				[]string{requestRole.GetName()}, []types.ResourceID{
					mustResourceID(srv.ClusterName(), types.KindUserGroup, userGroup1.GetName()),
					mustResourceID(srv.ClusterName(), types.KindApp, userGroup1.GetApplications()[0]),
					mustResourceID(srv.ClusterName(), types.KindApp, userGroup1.GetApplications()[1]),
					mustResourceID(srv.ClusterName(), types.KindApp, userGroup1.GetApplications()[2]),
				}),
		},
		{
			name: "bob fails to create a request for alice",
			user: bob,
			accessRequest: mustAccessRequest(t, alice, types.RequestState_PENDING, clock.Now(), clock.Now().Add(time.Hour),
				[]string{requestRole.GetName()}, []types.ResourceID{
					mustResourceID(srv.ClusterName(), types.KindUserGroup, userGroup1.GetName()),
				}),
			errAssertionFunc: require.Error,
		},
		{
			name: "user creates own pending access request with user group needing app expansion",
			user: alice,
			accessRequest: mustAccessRequest(t, alice, types.RequestState_PENDING, clock.Now(), clock.Now().Add(time.Hour),
				[]string{requestRole.GetName()}, []types.ResourceID{
					mustResourceID(srv.ClusterName(), nodeAllowedByRequestRole.GetKind(), nodeAllowedByRequestRole.GetName()),
					mustResourceID(srv.ClusterName(), types.KindUserGroup, userGroup1.GetName()),
					mustResourceID(srv.ClusterName(), types.KindApp, "app1"),
					mustResourceID(srv.ClusterName(), types.KindUserGroup, userGroup2.GetName()),
					mustResourceID(srv.ClusterName(), types.KindUserGroup, userGroup3.GetName()),
				}),
			errAssertionFunc: require.NoError,
			expected: mustAccessRequest(t, alice, types.RequestState_PENDING, clock.Now(), clock.Now().Add(time.Hour),
				[]string{requestRole.GetName()}, []types.ResourceID{
					mustResourceID(srv.ClusterName(), nodeAllowedByRequestRole.GetKind(), nodeAllowedByRequestRole.GetName()),
					mustResourceID(srv.ClusterName(), types.KindUserGroup, userGroup1.GetName()),
					mustResourceID(srv.ClusterName(), types.KindApp, "app1"),
					mustResourceID(srv.ClusterName(), types.KindUserGroup, userGroup2.GetName()),
					mustResourceID(srv.ClusterName(), types.KindUserGroup, userGroup3.GetName()),
					mustResourceID(srv.ClusterName(), types.KindApp, "app2"),
					mustResourceID(srv.ClusterName(), types.KindApp, "app3"),
					mustResourceID(srv.ClusterName(), types.KindApp, "app4"),
					mustResourceID(srv.ClusterName(), types.KindApp, "app5"),
				}),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Make sure there are no access requests before we do anything. We'll clear out
			// each time to save on the complexity of setting up the auth server and dependent
			// users and roles.
			ctx := context.Background()
			require.NoError(t, srv.Auth().DeleteAllAccessRequests(ctx))

			client, err := srv.NewClient(TestUser(test.user))
			require.NoError(t, err)

			req, err := client.CreateAccessRequestV2(ctx, test.accessRequest)
			test.errAssertionFunc(t, err)

			if err != nil {
				require.Nil(t, test.expected, "erroring test-cases should not assert expectations (this is a bug)")
				return
			}

			// id should be regenerated server-side
			require.NotEqual(t, test.accessRequest.GetName(), req.GetName())

			accessRequests, err := srv.Auth().GetAccessRequests(ctx, types.AccessRequestFilter{
				ID: req.GetName(),
			})
			require.NoError(t, err)

			if test.expected == nil {
				require.Empty(t, accessRequests)
				return
			}

			require.Len(t, accessRequests, 1)

			// We have to ignore the name here, as it's auto-generated by the underlying access request
			// logic.
			require.Empty(t, cmp.Diff(test.expected, accessRequests[0],
				cmpopts.IgnoreFields(types.Metadata{}, "Name", "Revision"),
				cmpopts.IgnoreFields(types.AccessRequestSpecV3{}),
			))
		})
	}
}

func TestAccessRequestNonGreedyAnnotations(t *testing.T) {
	t.Parallel()

	userTraits := map[string][]string{
		"email": {"tester@example.com"},
	}

	paymentsRequester, err := types.NewRole("payments-requester", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Request: &types.AccessRequestConditions{
				Annotations: map[string][]string{
					"services":     {"payments"},
					"requesting":   {"role"},
					"requested-by": {"{{email.local(external.email)}}"},
				},
				Roles: []string{"payments-access"},
			},
		},
	})
	require.NoError(t, err)

	paymentsResourceRequester, err := types.NewRole("payments-resource-requester", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Request: &types.AccessRequestConditions{
				Annotations: map[string][]string{
					"services":     {"payments"},
					"requesting":   {"resources"},
					"requested-by": {"{{email.local(external.email)}}"},
				},
				SearchAsRoles: []string{"payments-access"},
			},
		},
	})
	require.NoError(t, err)

	paymentsAccess, err := types.NewRole("payments-access", types.RoleSpecV6{
		Allow: types.RoleConditions{
			NodeLabels: types.Labels{"service": []string{"payments"}},
			Request: &types.AccessRequestConditions{
				Annotations: map[string][]string{
					"never-get-this": {"true"},
				},
			},
		},
	})
	require.NoError(t, err)

	identityRequester, err := types.NewRole("identity-requester", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Request: &types.AccessRequestConditions{
				Annotations: map[string][]string{
					"services":     {"identity"},
					"requesting":   {"role"},
					"requested-by": {"{{email.local(external.email)}}"},
				},
				Roles: []string{"identity-access"},
			},
		},
	})
	require.NoError(t, err)

	identityResourceRequester, err := types.NewRole("identity-resource-requester", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Request: &types.AccessRequestConditions{
				Annotations: map[string][]string{
					"services":     {"identity"},
					"requesting":   {"resources"},
					"requested-by": {"{{email.local(external.email)}}"},
				},
				SearchAsRoles: []string{"identity-access"},
			},
		},
	})
	require.NoError(t, err)

	identityAccess, err := types.NewRole("identity-access", types.RoleSpecV6{
		Allow: types.RoleConditions{
			NodeLabels: types.Labels{"service": []string{"identity"}},
			Request: &types.AccessRequestConditions{
				Annotations: map[string][]string{
					"never-get-this": {"true"},
				},
			},
		},
	})
	require.NoError(t, err)

	anyResourceRequester, err := types.NewRole("any-requester", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Request: &types.AccessRequestConditions{
				Annotations: map[string][]string{
					"any-requester": {"true"},
					"requested-by":  {"{{email.local(external.email)}}"},
				},
				SearchAsRoles: []string{"identity-access", "payments-access"},
				Roles:         []string{"identity-access", "payments-access"},
			},
		},
	})
	require.NoError(t, err)

	globRequester, err := types.NewRole("glob-requester", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Request: &types.AccessRequestConditions{
				Annotations: map[string][]string{
					"glob-requester": {"true"},
					"requested-by":   {"{{email.local(external.email)}}"},
				},
				Roles: []string{"*"},
			},
		},
	})
	require.NoError(t, err)

	reRequester, err := types.NewRole("re-requester", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Request: &types.AccessRequestConditions{
				Annotations: map[string][]string{
					"re-requester": {"true"},
					"requested-by": {"{{email.local(external.email)}}"},
				},
				Roles: []string{"identity-*", "^payments-acces.$"},
			},
		},
	})
	require.NoError(t, err)

	// This role denies the services: identity annotation
	denyIdentityService, err := types.NewRole("deny-identity-service", types.RoleSpecV6{
		Deny: types.RoleConditions{
			Request: &types.AccessRequestConditions{
				Annotations: map[string][]string{
					"services": {"identity"},
				},
			},
		},
	})
	require.NoError(t, err)

	// This role allows roles and annotations based on claims.
	claimsRequester, err := types.NewRole("claims-requester", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Request: &types.AccessRequestConditions{
				ClaimsToRoles: []types.ClaimMapping{
					{
						Claim: "email",
						Value: "tester@example.com",
						Roles: []string{"identity-access"},
					},
				},
				Annotations: map[string][]string{
					"services":           {"identity"},
					"requested-by":       {"{{email.local(external.email)}}"},
					"should-be-excluded": {"true"},
				},
			},
		},
		Deny: types.RoleConditions{
			Request: &types.AccessRequestConditions{
				Annotations: map[string][]string{
					"should-be-excluded": {"true"},
				},
			},
		},
	})
	require.NoError(t, err)

	roles := []types.Role{
		paymentsRequester, paymentsResourceRequester, paymentsAccess,
		identityRequester, identityResourceRequester, identityAccess,
		anyResourceRequester, globRequester, reRequester,
		denyIdentityService, claimsRequester,
	}

	paymentsServer, err := types.NewServer("server-payments", types.KindNode, types.ServerSpecV2{})
	require.NoError(t, err)
	paymentsServer.SetStaticLabels(map[string]string{"service": "payments"})

	idServer, err := types.NewServerWithLabels(
		"server-identity",
		types.KindNode,
		types.ServerSpecV2{},
		map[string]string{"service": "identity"},
	)
	require.NoError(t, err)

	ctx := context.Background()
	srv := newTestTLSServer(t)
	for _, role := range roles {
		_, err := srv.Auth().CreateRole(ctx, role)
		require.NoError(t, err)
	}

	for _, server := range []types.Server{paymentsServer, idServer} {
		_, err := srv.Auth().UpsertNode(ctx, server)
		require.NoError(t, err)
	}

	for _, tc := range []struct {
		name                 string
		roles                []string
		requestedRoles       []string
		requestedResourceIDs []string
		expectedAnnotations  map[string][]string
		errfn                require.ErrorAssertionFunc
	}{
		{
			name:           "payments-requester requests role, receives payment annotations",
			roles:          []string{"payments-requester"},
			requestedRoles: []string{"payments-access"},
			expectedAnnotations: map[string][]string{
				"services":     {"payments"},
				"requesting":   {"role"},
				"requested-by": {"tester"},
			},
		},
		{
			name:                 "payments-resource-requester requests resource, receives payment annotations",
			roles:                []string{"payments-resource-requester"},
			requestedRoles:       []string{"payments-access"},
			requestedResourceIDs: []string{"server-payments"},
			expectedAnnotations: map[string][]string{
				"services":     {"payments"},
				"requesting":   {"resources"},
				"requested-by": {"tester"},
			},
		},
		{
			name:           "payments-requester requests identity role, receives error",
			roles:          []string{"payments-requester"},
			requestedRoles: []string{"identity-access"},
			errfn:          require.Error,
		},
		{
			name:                 "payments-resource-requester requests identity resource, receives error",
			roles:                []string{"payments-resource-requester"},
			requestedRoles:       []string{"identity-access"},
			requestedResourceIDs: []string{"server-identity"},
			errfn:                require.Error,
		},
		{
			name:           "identity-requester requests role, receives identity annotations",
			roles:          []string{"identity-requester"},
			requestedRoles: []string{"identity-access"},
			expectedAnnotations: map[string][]string{
				"services":     {"identity"},
				"requesting":   {"role"},
				"requested-by": {"tester"},
			},
		},
		{
			name:                 "identity-resource-requester requests resource, receives identity annotations",
			roles:                []string{"identity-resource-requester"},
			requestedRoles:       []string{"identity-access"},
			requestedResourceIDs: []string{"server-identity"},
			expectedAnnotations: map[string][]string{
				"services":     {"identity"},
				"requesting":   {"resources"},
				"requested-by": {"tester"},
			},
		},
		{
			name:           "identity-requester requests paymen role, receives error",
			roles:          []string{"identity-requester"},
			requestedRoles: []string{"payments-access"},
			errfn:          require.Error,
		},
		{
			name:                 "identity-resource-requester requests payment resource, receives error",
			roles:                []string{"identity-resource-requester"},
			requestedRoles:       []string{"payment-access"},
			requestedResourceIDs: []string{"server-identity"},
			errfn:                require.Error,
		},
		{
			name:           "any-requester requests role, receives annotations",
			roles:          []string{"any-requester"},
			requestedRoles: []string{"payments-access"},
			expectedAnnotations: map[string][]string{
				"any-requester": {"true"},
				"requested-by":  {"tester"},
			},
		},
		{
			name:                 "any-requester requests role, receives annotations",
			roles:                []string{"any-requester"},
			requestedRoles:       []string{"payments-access"},
			requestedResourceIDs: []string{"server-payments"},
			expectedAnnotations: map[string][]string{
				"any-requester": {"true"},
				"requested-by":  {"tester"},
			},
		},
		{
			name:           "both payments and identity-requester requests payments role, receives payments annotations",
			roles:          []string{"identity-requester", "payments-requester"},
			requestedRoles: []string{"payments-access"},
			expectedAnnotations: map[string][]string{
				"requesting":   {"role"},
				"services":     {"payments"},
				"requested-by": {"tester"},
			},
		},
		{
			name: "all requester roles, requests payments role, receives payments and any annotations",
			roles: []string{
				"identity-requester", "payments-requester",
				"identity-resource-requester", "payments-resource-requester",
				"any-requester",
			},
			requestedRoles: []string{"payments-access"},
			expectedAnnotations: map[string][]string{
				"requesting":    {"role"},
				"services":      {"payments"},
				"any-requester": {"true"},
				"requested-by":  {"tester"},
			},
		},
		{
			name: "all requester roles, requests payments resource, receives payments and any annotations",
			roles: []string{
				"identity-requester", "payments-requester",
				"identity-resource-requester", "payments-resource-requester",
				"any-requester",
			},
			requestedRoles:       []string{"payments-access"},
			requestedResourceIDs: []string{"server-payments"},
			expectedAnnotations: map[string][]string{
				"requesting":    {"resources"},
				"services":      {"payments"},
				"any-requester": {"true"},
				"requested-by":  {"tester"},
			},
		},
		{
			name:           "glob-requester requests payments role, receives annotations",
			roles:          []string{"glob-requester"},
			requestedRoles: []string{"payments-access"},
			expectedAnnotations: map[string][]string{
				"glob-requester": {"true"},
				"requested-by":   {"tester"},
			},
		},
		{
			name:           "glob-requester requests identity role, receives annotations",
			roles:          []string{"glob-requester"},
			requestedRoles: []string{"identity-access"},
			expectedAnnotations: map[string][]string{
				"glob-requester": {"true"},
				"requested-by":   {"tester"},
			},
		},
		{
			name:           "re-requester requests both roles, receives annotations",
			roles:          []string{"re-requester"},
			requestedRoles: []string{"identity-access", "payments-access"},
			expectedAnnotations: map[string][]string{
				"re-requester": {"true"},
				"requested-by": {"tester"},
			},
		},
		{
			name:           "re-requester requests payments role, receives annotations",
			roles:          []string{"re-requester"},
			requestedRoles: []string{"payments-access"},
			expectedAnnotations: map[string][]string{
				"re-requester": {"true"},
				"requested-by": {"tester"},
			},
		},
		{
			name:           "deny identity services annotation",
			roles:          []string{"identity-requester", "payments-requester", "deny-identity-service"},
			requestedRoles: []string{"identity-access", "payments-access"},
			expectedAnnotations: map[string][]string{
				"requesting":   {"role"},
				"services":     {"payments"},
				"requested-by": {"tester"},
			},
		},
		{
			name:           "annotations based on claims",
			roles:          []string{"claims-requester"},
			requestedRoles: []string{"identity-access"},
			expectedAnnotations: map[string][]string{
				"services":     {"identity"},
				"requested-by": {"tester"},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			user, err := types.NewUser("requester")
			require.NoError(t, err)
			user.SetRoles(tc.roles)
			user.SetTraits(userTraits)
			_, err = srv.Auth().UpsertUser(ctx, user)
			require.NoError(t, err)

			var req types.AccessRequest
			if len(tc.requestedResourceIDs) == 0 {
				req, err = types.NewAccessRequest(uuid.NewString(), user.GetName(), tc.requestedRoles...)
			} else {
				var resourceIds []types.ResourceID
				for _, id := range tc.requestedResourceIDs {
					resourceIds = append(resourceIds, types.ResourceID{
						ClusterName: srv.ClusterName(),
						Kind:        types.KindNode,
						Name:        id,
					})
				}
				req, err = types.NewAccessRequestWithResources(uuid.NewString(), user.GetName(), tc.requestedRoles, resourceIds)
			}
			require.NoError(t, err)

			client, err := srv.NewClient(TestUser(user.GetName()))
			require.NoError(t, err)
			res, err := client.CreateAccessRequestV2(ctx, req)
			if tc.errfn == nil {
				require.NoError(t, err)
				require.Equal(t, tc.expectedAnnotations, res.GetSystemAnnotations())
			} else {
				tc.errfn(t, err)
			}
		})
	}
}

func mustAccessRequest(t *testing.T, user string, state types.RequestState, created, expires time.Time, roles []string, resourceIDs []types.ResourceID) types.AccessRequest {
	t.Helper()

	accessRequest, err := types.NewAccessRequest(uuid.NewString(), user, roles...)
	require.NoError(t, err)

	accessRequest.SetRequestedResourceIDs(resourceIDs)
	accessRequest.SetState(state)
	accessRequest.SetCreationTime(created)
	accessRequest.SetExpiry(expires)
	accessRequest.SetAccessExpiry(expires)
	accessRequest.SetMaxDuration(expires)
	accessRequest.SetSessionTLL(expires)
	accessRequest.SetThresholds([]types.AccessReviewThreshold{{Name: "default", Approve: 1, Deny: 1}})
	accessRequest.SetRoleThresholdMapping(map[string]types.ThresholdIndexSets{
		"requestRole": {
			Sets: []types.ThresholdIndexSet{
				{Indexes: []uint32{0}},
			},
		},
	})

	return accessRequest
}

func mustResourceID(clusterName, kind, name string) types.ResourceID {
	return types.ResourceID{
		ClusterName: clusterName,
		Kind:        kind,
		Name:        name,
	}
}

func TestWatchHeadlessAuthentications_usersCanOnlyWatchThemselves(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)
	alice, bob, admin := createSessionTestUsers(t, srv.Auth())

	// For each user, prepare 4 different headless authentications with the varying states.
	// These will be created during each test, and the watcher will return a subset of the
	// collected events based on the test's filter.
	var headlessAuthns []*types.HeadlessAuthentication
	var headlessEvents []types.Event
	for _, username := range []string{alice, bob} {
		for _, state := range []types.HeadlessAuthenticationState{
			types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_UNSPECIFIED,
			types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_PENDING,
			types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_DENIED,
			types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_APPROVED,
		} {
			ha, err := types.NewHeadlessAuthentication(username, uuid.NewString(), srv.Clock().Now().Add(time.Minute))
			require.NoError(t, err)
			ha.State = state
			headlessAuthns = append(headlessAuthns, ha)
			headlessEvents = append(headlessEvents, types.Event{
				Type:     types.OpPut,
				Resource: ha,
			})
		}
	}
	aliceEvents := headlessEvents[:4]
	bobEvents := headlessEvents[4:]

	testCases := []*struct {
		name             string
		identity         TestIdentity
		filter           types.HeadlessAuthenticationFilter
		expectWatchError string
		expectEvents     []types.Event
		watcher          types.Watcher
	}{
		{
			name:             "NOK non local users cannot watch headless authentications",
			identity:         TestAdmin(),
			expectWatchError: "non-local user roles cannot watch headless authentications",
		},
		{
			name:             "NOK must filter for username",
			identity:         TestUser(admin),
			filter:           types.HeadlessAuthenticationFilter{},
			expectWatchError: "user cannot watch headless authentications without a filter for their username",
		},
		{
			name:     "NOK alice cannot filter for username=bob",
			identity: TestUser(alice),
			filter: types.HeadlessAuthenticationFilter{
				Username: bob,
			},
			expectWatchError: "user \"alice\" cannot watch headless authentications of \"bob\"",
		},
		{
			name:     "OK alice can filter for username=alice",
			identity: TestUser(alice),
			filter: types.HeadlessAuthenticationFilter{
				Username: alice,
			},
			expectEvents: aliceEvents,
		},
		{
			name:     "OK bob can filter for username=bob",
			identity: TestUser(bob),
			filter: types.HeadlessAuthenticationFilter{
				Username: bob,
			},
			expectEvents: bobEvents,
		},
		{
			name:     "OK alice can filter for pending requests",
			identity: TestUser(alice),
			filter: types.HeadlessAuthenticationFilter{
				Username: alice,
				State:    types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_PENDING,
			},
			expectEvents: []types.Event{aliceEvents[types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_PENDING]},
		},
		{
			name:     "OK alice can filter for a specific request",
			identity: TestUser(alice),
			filter: types.HeadlessAuthenticationFilter{
				Username: alice,
				Name:     headlessAuthns[2].GetName(),
			},
			expectEvents: aliceEvents[2:3],
		},
	}

	// Initialize headless watcher for each test cases.
	t.Run("init_watchers", func(t *testing.T) {
		for _, tc := range testCases {
			tc := tc

			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				client, err := srv.NewClient(tc.identity)
				require.NoError(t, err)

				watcher, err := client.NewWatcher(ctx, types.Watch{
					Kinds: []types.WatchKind{
						{
							Kind:   types.KindHeadlessAuthentication,
							Filter: tc.filter.IntoMap(),
						},
					},
				})
				require.NoError(t, err)

				select {
				case event := <-watcher.Events():
					require.Equal(t, types.OpInit, event.Type, "Expected watcher init event but got %v", event)
				case <-time.After(time.Second):
					t.Fatal("Failed to receive watcher init event before timeout")
				case <-watcher.Done():
					if tc.expectWatchError != "" {
						require.True(t, trace.IsAccessDenied(watcher.Error()), "Expected access denied error but got %v", err)
						require.ErrorContains(t, watcher.Error(), tc.expectWatchError)
						return
					}
					t.Fatalf("Watcher unexpectedly closed with error: %v", watcher.Error())
				}

				// If the watcher was intitialized successfully, attach it to the
				// test case for the watch_events portion of the test.
				tc.watcher = watcher
			})
		}
	})

	// Upsert headless requests.
	for _, ha := range headlessAuthns {
		err := srv.Auth().UpsertHeadlessAuthentication(ctx, ha)
		require.NoError(t, err)
	}

	t.Run("watch_events", func(t *testing.T) {
		// Check that each watcher captured the expected events.
		for _, tc := range testCases {
			tc := tc

			// watcher was not initialized for this test case, skip.
			if tc.watcher == nil {
				continue
			}

			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				var events []types.Event
			loop:
				for {
					select {
					case event := <-tc.watcher.Events():
						events = append(events, event)
					case <-time.After(500 * time.Millisecond):
						break loop
					case <-tc.watcher.Done():
						t.Fatalf("Watcher unexpectedly closed with error: %v", tc.watcher.Error())
					}
				}
				require.Equal(t, tc.expectEvents, events)
			})
		}
	})
}

// createAppServerOrSPFromAppServer returns a AppServerOrSAMLIdPServiceProvider given an AppServer.
//
//nolint:staticcheck // SA1019. TODO(sshah) DELETE IN 17.0
func createAppServerOrSPFromAppServer(appServer types.AppServer) types.AppServerOrSAMLIdPServiceProvider {
	appServerOrSP := &types.AppServerOrSAMLIdPServiceProviderV1{
		Resource: &types.AppServerOrSAMLIdPServiceProviderV1_AppServer{
			AppServer: appServer.(*types.AppServerV3),
		},
	}

	return appServerOrSP
}

// createAppServerOrSPFromApp returns a AppServerOrSAMLIdPServiceProvider given a SAMLIdPServiceProvider.
//
//nolint:staticcheck // SA1019. TODO(sshah) DELETE IN 17.0
func createAppServerOrSPFromSP(sp types.SAMLIdPServiceProvider) types.AppServerOrSAMLIdPServiceProvider {
	appServerOrSP := &types.AppServerOrSAMLIdPServiceProviderV1{
		Resource: &types.AppServerOrSAMLIdPServiceProviderV1_SAMLIdPServiceProvider{
			SAMLIdPServiceProvider: sp.(*types.SAMLIdPServiceProviderV1),
		},
	}

	return appServerOrSP
}

func TestKubeKeepAliveServer(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)
	domainName, err := srv.Auth().GetDomainName()
	require.NoError(t, err)

	tests := map[string]struct {
		builtInRole types.SystemRole
		assertErr   require.ErrorAssertionFunc
	}{
		"as kube service": {
			builtInRole: types.RoleKube,
			assertErr:   require.NoError,
		},
		"as legacy proxy service": {
			builtInRole: types.RoleProxy,
			assertErr:   require.NoError,
		},
		"as database service": {
			builtInRole: types.RoleDatabase,
			assertErr:   require.Error,
		},
	}
	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			hostID := uuid.New().String()
			// Create a kubernetes cluster.
			kube, err := types.NewKubernetesClusterV3(
				types.Metadata{
					Name:      "kube",
					Namespace: apidefaults.Namespace,
				},
				types.KubernetesClusterSpecV3{},
			)
			require.NoError(t, err)
			// Create a kubernetes server.
			// If the built-in role is proxy, the server name should be
			// kube-proxy_service
			serverName := "kube"
			if test.builtInRole == types.RoleProxy {
				serverName += teleport.KubeLegacyProxySuffix
			}
			kubeServer, err := types.NewKubernetesServerV3(
				types.Metadata{
					Name:      serverName,
					Namespace: apidefaults.Namespace,
				},
				types.KubernetesServerSpecV3{
					Cluster: kube,
					HostID:  hostID,
				},
			)
			require.NoError(t, err)
			// Upsert the kubernetes server into the backend.
			_, err = srv.Auth().UpsertKubernetesServer(context.Background(), kubeServer)
			require.NoError(t, err)

			// Create a built-in role.
			authContext, err := authz.ContextForBuiltinRole(
				authz.BuiltinRole{
					Role:     test.builtInRole,
					Username: fmt.Sprintf("%s.%s", hostID, domainName),
				},
				types.DefaultSessionRecordingConfig(),
			)
			require.NoError(t, err)

			// Create a server with the built-in role.
			srv := ServerWithRoles{
				authServer: srv.Auth(),
				context:    *authContext,
			}
			// Keep alive the server.
			err = srv.KeepAliveServer(context.Background(),
				types.KeepAlive{
					Type:      types.KeepAlive_KUBERNETES,
					Expires:   time.Now().Add(5 * time.Minute),
					Name:      serverName,
					Namespace: apidefaults.Namespace,
					HostID:    hostID,
				},
			)
			test.assertErr(t, err)
		},
		)
	}
}

// inlineEventually is equivalent to require.Eventually except that it runs the provided function directly
// instead of in a background goroutine, making it safe to fail the test from within the closure.
func inlineEventually(t *testing.T, cond func() bool, waitFor time.Duration, tick time.Duration, msgAndArgs ...interface{}) {
	t.Helper()

	timer := time.NewTimer(waitFor)
	defer timer.Stop()

	ticker := time.NewTicker(tick)
	defer ticker.Stop()

	for {
		select {
		case <-timer.C:
			require.FailNow(t, "condition never satisfied", msgAndArgs...)
		case <-ticker.C:
			if cond() {
				return
			}
		}
	}
}

func TestIsMFARequired_AdminAction(t *testing.T) {
	for _, tt := range []struct {
		name                 string
		adminActionAuthState authz.AdminActionAuthState
		expectResp           *proto.IsMFARequiredResponse
	}{
		{
			name:                 "unauthorized",
			adminActionAuthState: authz.AdminActionAuthUnauthorized,
			expectResp: &proto.IsMFARequiredResponse{
				Required:    true,
				MFARequired: proto.MFARequired_MFA_REQUIRED_YES,
			},
		}, {
			name:                 "not required",
			adminActionAuthState: authz.AdminActionAuthNotRequired,
			expectResp: &proto.IsMFARequiredResponse{
				Required:    false,
				MFARequired: proto.MFARequired_MFA_REQUIRED_NO,
			},
		}, {
			name:                 "mfa verified",
			adminActionAuthState: authz.AdminActionAuthMFAVerified,
			expectResp: &proto.IsMFARequiredResponse{
				Required:    true,
				MFARequired: proto.MFARequired_MFA_REQUIRED_YES,
			},
		}, {
			name:                 "mfa verified with reuse",
			adminActionAuthState: authz.AdminActionAuthMFAVerifiedWithReuse,
			expectResp: &proto.IsMFARequiredResponse{
				Required:    true,
				MFARequired: proto.MFARequired_MFA_REQUIRED_YES,
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := ServerWithRoles{
				context: authz.Context{
					AdminActionAuthState: tt.adminActionAuthState,
				},
			}
			resp, err := server.IsMFARequired(context.Background(), &proto.IsMFARequiredRequest{
				Target: &proto.IsMFARequiredRequest_AdminAction{},
			})
			require.NoError(t, err)
			require.Equal(t, tt.expectResp, resp)
		})
	}
}

func TestCloudDefaultPasswordless(t *testing.T) {
	tt := []struct {
		name                     string
		cloud                    bool
		withPresetUsers          bool
		qtyPreexistingUsers      int
		expectedDefaultConnector string
	}{
		{
			name:                     "First Cloud user should set cluster to passwordless when presets exist",
			cloud:                    true,
			withPresetUsers:          true,
			qtyPreexistingUsers:      0,
			expectedDefaultConnector: constants.PasswordlessConnector,
		},
		{
			name:                     "First Cloud user should set cluster to passwordless when presets don't exist",
			cloud:                    true,
			withPresetUsers:          false,
			qtyPreexistingUsers:      0,
			expectedDefaultConnector: constants.PasswordlessConnector,
		},
		{
			name:                     "Second Cloud user should not set cluster to passwordless when presets exist",
			cloud:                    true,
			withPresetUsers:          true,
			qtyPreexistingUsers:      1,
			expectedDefaultConnector: "",
		},
		{
			name:                     "Second Cloud user should not set cluster to passwordless when presets don't exist",
			cloud:                    true,
			withPresetUsers:          false,
			qtyPreexistingUsers:      1,
			expectedDefaultConnector: "",
		},
		{
			name:                     "First non-Cloud user should not set cluster to passwordless",
			cloud:                    false,
			withPresetUsers:          false,
			qtyPreexistingUsers:      0,
			expectedDefaultConnector: "",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// create new server
			ctx := context.Background()
			srv := newTestTLSServer(t)

			modules.SetTestModules(t, &modules.TestModules{
				TestBuildType: modules.BuildEnterprise,
				TestFeatures: modules.Features{
					Cloud: tc.cloud,
				},
			})

			// set cluster Webauthn
			authPreference, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
				Type:         constants.Local,
				SecondFactor: constants.SecondFactorWebauthn,
				Webauthn: &types.Webauthn{
					RPID: "localhost",
				},
			})
			require.NoError(t, err)
			_, err = srv.Auth().UpsertAuthPreference(ctx, authPreference)
			require.NoError(t, err)

			// the test server doesn't create the preset users, so we call createPresetUsers manually
			if tc.withPresetUsers {
				createPresetUsers(ctx, srv.Auth())
			}

			// create preexisting users
			for i := 0; i < tc.qtyPreexistingUsers; i += 1 {
				_, _, err = CreateUserAndRole(srv.Auth(), fmt.Sprintf("testuser-%d", i), nil /* allowedLogins */, nil /* allowRules */)
				require.NoError(t, err)
			}

			// create the user that might have the auth method changed to passwordless with 2FA
			// since CreateResetPasswordToken requires MFA verification.
			u, err := createUserWithSecondFactors(srv)
			require.NoError(t, err)

			// add permission to edit users, necessary to change auth method to passwordless
			user, err := srv.Auth().GetUser(ctx, u.username, false /* withSecrets */)
			require.NoError(t, err)
			// createUserWithSecondFactors creates a role for the user already, so we modify it with extra perms
			roleName := user.GetRoles()[0]
			role, err := srv.Auth().GetRole(ctx, roleName)
			require.NoError(t, err)
			role.SetRules(types.Allow, []types.Rule{
				types.NewRule(types.KindUser, services.RW()),
			})
			_, err = srv.Auth().UpsertRole(ctx, role)
			require.NoError(t, err)

			client, err := srv.NewClient(TestUser(user.GetName()))
			require.NoError(t, err)

			// setup to change authentication method to passkey
			resetToken, err := client.CreateResetPasswordToken(ctx, authclient.CreateUserTokenRequest{
				Name: user.GetName(),
			})
			require.NoError(t, err)
			token := resetToken.GetName()

			registerChal, err := client.CreateRegisterChallenge(ctx, &proto.CreateRegisterChallengeRequest{
				TokenID:     token,
				DeviceType:  proto.DeviceType_DEVICE_TYPE_WEBAUTHN,
				DeviceUsage: proto.DeviceUsage_DEVICE_USAGE_PASSWORDLESS,
			})
			require.NoError(t, err)

			_, registerSolved, err := NewTestDeviceFromChallenge(registerChal, WithPasswordless())
			require.NoError(t, err)

			_, err = client.ChangeUserAuthentication(ctx, &proto.ChangeUserAuthenticationRequest{
				TokenID:                token,
				NewMFARegisterResponse: registerSolved,
			})
			require.NoError(t, err)

			authPreferences, err := client.GetAuthPreference(ctx)
			require.NoError(t, err)

			// assert that the auth preference matches the expected
			require.Equal(t, tc.expectedDefaultConnector, authPreferences.GetConnectorName())
		})
	}
}

func TestRoleRequestReasonModeValidation(t *testing.T) {
	ctx := context.Background()
	srv := newTestTLSServer(t)

	s := newTestServerWithRoles(t, srv.AuthServer, types.RoleAdmin)

	testCases := []struct {
		desc          string
		allow         types.AccessRequestConditions
		deny          types.AccessRequestConditions
		expectedError error
	}{
		{
			desc: "Reason mode can be omitted",
			allow: types.AccessRequestConditions{
				Roles: []string{"requestable-test-role"},
			},
			expectedError: nil,
		},
		{
			desc: "Reason mode can be empty",
			allow: types.AccessRequestConditions{
				Roles: []string{"requestable-test-role"},
				Reason: &types.AccessRequestConditionsReason{
					Mode: "",
				},
			},
			expectedError: nil,
		},
		{
			desc: "Reason mode can be required in allow condition",
			allow: types.AccessRequestConditions{
				Roles: []string{"requestable-test-role"},
				Reason: &types.AccessRequestConditionsReason{
					Mode: types.RequestReasonModeRequired,
				},
			},
			expectedError: nil,
		},
		{
			desc: "Reason mode can be optional in allow condition",
			allow: types.AccessRequestConditions{
				Roles: []string{"requestable-test-role"},
				Reason: &types.AccessRequestConditionsReason{
					Mode: types.RequestReasonModeOptional,
				},
			},
			expectedError: nil,
		},
		{
			desc: "Reason mode can be empty",
			allow: types.AccessRequestConditions{
				Roles: []string{"requestable-test-role"},
				Reason: &types.AccessRequestConditionsReason{
					Mode: "",
				},
			},
			expectedError: nil,
		},
		{
			desc: "Reason mode cannot be set to any other value",
			allow: types.AccessRequestConditions{
				Roles: []string{"requestable-test-role"},
				Reason: &types.AccessRequestConditionsReason{
					Mode: "other-value",
				},
			},
			expectedError: trace.BadParameter(`unrecognized request reason mode "other-value", must be one of: [required optional]`),
		},
		{
			desc: "Reason mode cannot be set deny condition",
			deny: types.AccessRequestConditions{
				Roles: []string{"requestable-test-role"},
				Reason: &types.AccessRequestConditionsReason{
					Mode: types.RequestReasonModeOptional,
				},
			},
			expectedError: trace.BadParameter("request reason mode can be provided only for allow rules"),
		},
	}

	for i, tt := range testCases {
		t.Run(tt.desc, func(t *testing.T) {
			var err error

			createRole := newRole(t, fmt.Sprintf("test-create-role-%d", i), nil, types.RoleConditions{}, types.RoleConditions{})
			updateRole := newRole(t, fmt.Sprintf("test-update-role-%d", i), nil, types.RoleConditions{}, types.RoleConditions{})
			upsertRole := newRole(t, fmt.Sprintf("test-upsert-role-%d", i), nil, types.RoleConditions{}, types.RoleConditions{})

			createRole.SetAccessRequestConditions(types.Allow, tt.allow)
			createRole.SetAccessRequestConditions(types.Deny, tt.deny)
			_, err = s.CreateRole(ctx, createRole)
			require.ErrorIs(t, err, tt.expectedError)

			updateRole, err = s.CreateRole(ctx, updateRole)
			require.NoError(t, err)
			updateRole.SetAccessRequestConditions(types.Allow, tt.allow)
			updateRole.SetAccessRequestConditions(types.Deny, tt.deny)
			_, err = s.UpdateRole(ctx, updateRole)
			require.ErrorIs(t, err, tt.expectedError)

			upsertRole, err = s.CreateRole(ctx, upsertRole)
			require.NoError(t, err)
			upsertRole.SetAccessRequestConditions(types.Allow, tt.allow)
			upsertRole.SetAccessRequestConditions(types.Deny, tt.deny)
			_, err = s.UpsertRole(ctx, upsertRole)
			require.ErrorIs(t, err, tt.expectedError)
		})
	}
}

func testUserName(testName string) string {
	return strings.ReplaceAll(testName, " ", "_")
}

func TestFilterIdentityCenterPermissionSets(t *testing.T) {
	const (
		allAccessRoleName      = "all-access"
		accountID              = "1234567890"
		permissionSetArnPrefix = "aws:awn:test:permission:set:"
	)

	// GIVEN a test cluster...
	ctx := context.Background()
	srv := newTestTLSServer(t)
	s := newTestServerWithRoles(t, srv.AuthServer, types.RoleAdmin)

	// GIVEN an Identity Center Account with some associated Permission Set
	// resources
	permissionSets := []*identitycenterv1.PermissionSetInfo{
		{
			Name:         "PS One",
			Arn:          permissionSetArnPrefix + "one",
			AssignmentId: accountID + "-" + "ps_one",
		},
		{
			Name:         "PS Two",
			Arn:          permissionSetArnPrefix + "two",
			AssignmentId: accountID + "-" + "ps_two",
		},
		{
			Name:         "PS Three",
			Arn:          permissionSetArnPrefix + "ps_three",
			AssignmentId: accountID + "-" + "ps_three",
		},
	}

	_, err := s.authServer.CreateIdentityCenterAccount(ctx,
		services.IdentityCenterAccount{
			Account: &identitycenterv1.Account{
				Kind:    types.KindIdentityCenterAccount,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: accountID,
					Labels: map[string]string{
						types.OriginLabel: apicommon.OriginAWSIdentityCenter,
					},
				},
				Spec: &identitycenterv1.AccountSpec{
					Id:                accountID,
					Arn:               "aws:arn:test:account",
					Name:              "Test Account",
					Description:       "An account for testing",
					PermissionSetInfo: permissionSets,
				},
			},
		})
	require.NoError(t, err)

	// GIVEN a role that allows access to all permission sets on the target
	// Identity Center account
	roleAccessAll, err := types.NewRole(allAccessRoleName, types.RoleSpecV6{
		Allow: types.RoleConditions{
			AccountAssignments: []types.IdentityCenterAccountAssignment{
				{
					Account:       accountID,
					PermissionSet: types.Wildcard,
				},
			},
		},
	})
	require.NoError(t, err, "Constructing role should succeed")
	_, err = srv.Auth().CreateRole(ctx, roleAccessAll)
	require.NoError(t, err, "Cretaing role should succeed")

	withRequesterRole := WithRoleMutator(func(role types.Role) {
		r := role.(*types.RoleV6)
		r.Spec.Allow.Request = &types.AccessRequestConditions{
			SearchAsRoles: []string{allAccessRoleName},
		}
	})

	// EXPECT that the IC Account has made it to the cache
	inlineEventually(t,
		func() bool {
			testAssignments, _, err := srv.Auth().ListIdentityCenterAccounts(
				ctx, 100, &pagination.PageRequestToken{})
			require.NoError(t, err)
			return len(testAssignments) == 1
		},
		5*time.Second, 200*time.Millisecond,
		"Target resource missing from cache")

	testCases := []struct {
		name                   string
		roleModifiers          []CreateUserAndRoleOption
		includeRequestable     bool
		expectedPSs            []*types.IdentityCenterPermissionSet
		expectedRequireRequest require.BoolAssertionFunc
	}{
		{
			name: "basic access",
			roleModifiers: []CreateUserAndRoleOption{
				withAccountAssignment(types.Allow, accountID, permissionSets[0].Arn),
				withAccountAssignment(types.Allow, accountID, permissionSets[1].Arn),
			},
			expectedPSs: []*types.IdentityCenterPermissionSet{
				paginatedAppPermissionSet(permissionSets[0]),
				paginatedAppPermissionSet(permissionSets[1]),
			},
			expectedRequireRequest: require.False,
		},
		{
			name: "ignore search as roles when disabled",
			roleModifiers: []CreateUserAndRoleOption{
				withAccountAssignment(types.Allow, accountID, permissionSets[1].Arn),
				withRequesterRole,
			},
			includeRequestable: false,
			expectedPSs: []*types.IdentityCenterPermissionSet{
				paginatedAppPermissionSet(permissionSets[1]),
			},
			expectedRequireRequest: require.False,
		},
		{
			name: "requestable access",
			roleModifiers: []CreateUserAndRoleOption{
				withAccountAssignment(types.Allow, accountID, permissionSets[1].Arn),
				withRequesterRole,
			},
			includeRequestable: true,
			expectedPSs: []*types.IdentityCenterPermissionSet{
				paginatedAppPermissionSet(permissionSets[0]),
				paginatedAppPermissionSet(permissionSets[1]),
				paginatedAppPermissionSet(permissionSets[2]),
			},
			expectedRequireRequest: require.True,
		},
		{
			name: "no access",
			roleModifiers: []CreateUserAndRoleOption{
				withAccountAssignment(types.Allow, accountID, "some-non-existent-ps"),
			},
			expectedRequireRequest: require.False,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			// GIVEN a user who has a role that allows a test-defined level of
			// Identity Center access
			user, _, err := CreateUserAndRole(srv.Auth(), testUserName(test.name),
				nil, nil, test.roleModifiers...)
			require.NoError(t, err)

			// GIVEN an auth client using the above user
			identity := TestUser(user.GetName())
			clt, err := srv.NewClient(identity)
			require.NoError(t, err)
			t.Cleanup(func() { clt.Close() })

			// WHEN I list the unified resources, with a filter specifically for
			// the account resource defined above...
			resp, err := clt.ListUnifiedResources(ctx, &proto.ListUnifiedResourcesRequest{
				Kinds: []string{types.KindApp},
				Labels: map[string]string{
					types.OriginLabel: apicommon.OriginAWSIdentityCenter,
				},
				UseSearchAsRoles:   test.includeRequestable,
				IncludeRequestable: test.includeRequestable,
				IncludeLogins:      true,
				SortBy:             types.SortBy{IsDesc: true, Field: types.ResourceMetadataName},
			})

			// EXPECT that the listing succeeds and returns a single resource
			require.NoError(t, err)
			require.Len(t, resp.Resources, 1, "Must return exactly one resource")

			// EXPECT that the contained resource has the test-defined value for
			// the RequiresRequest flag
			resource := resp.Resources[0]
			test.expectedRequireRequest(t, resource.RequiresRequest)

			// EXPECT that the returned resource is an App
			appServer := resp.Resources[0].GetAppServer()
			require.NotNil(t, appServer, "Expected resource to be an app")
			app := appServer.GetApp()

			// EXPECT that the app PermissionSets are filtered to the test-defined
			// list
			require.ElementsMatch(t,
				test.expectedPSs, app.GetIdentityCenter().PermissionSets)
		})
	}
}

func paginatedAppPermissionSet(src *identitycenterv1.PermissionSetInfo) *types.IdentityCenterPermissionSet {
	return &types.IdentityCenterPermissionSet{
		ARN:          src.Arn,
		Name:         src.Name,
		AssignmentID: src.AssignmentId,
	}
}
