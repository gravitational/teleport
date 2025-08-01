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

package auth_test

import (
	"cmp"
	"context"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	gocmp "github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/gravitational/license"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	notificationsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/notifications/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/installers"
	"github.com/gravitational/teleport/api/types/trait"
	"github.com/gravitational/teleport/api/types/userloginstate"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/auth/keystore"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/cryptosuites/cryptosuitestest"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/sshca"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

type testPack struct {
	bk             backend.Backend
	versionStorage auth.VersionStorage
	clusterName    types.ClusterName
	a              *auth.Server
	mockEmitter    *eventstest.MockRecorderEmitter
}

func newTestPack(
	ctx context.Context, dataDir string, opts ...auth.ServerOption,
) (testPack, error) {
	var (
		p   testPack
		err error
	)
	p.bk, err = memory.New(memory.Config{})
	if err != nil {
		return p, trace.Wrap(err)
	}
	p.clusterName, err = services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: "test.localhost",
	})
	if err != nil {
		return p, trace.Wrap(err)
	}

	p.versionStorage = authtest.NewFakeTeleportVersion()

	identityService, err := local.NewTestIdentityService(p.bk)
	if err != nil {
		return p, trace.Wrap(err)
	}

	p.mockEmitter = &eventstest.MockRecorderEmitter{}
	authConfig := &auth.InitConfig{
		DataDir:        dataDir,
		Backend:        p.bk,
		VersionStorage: p.versionStorage,
		ClusterName:    p.clusterName,
		Authority:      testauthority.New(),
		Emitter:        p.mockEmitter,
		// This uses lower bcrypt costs for faster tests.
		Identity:               identityService,
		SkipPeriodicOperations: true,
	}
	p.a, err = auth.NewServer(authConfig, opts...)
	if err != nil {
		return p, trace.Wrap(err)
	}

	// set lock watcher
	lockWatcher, err := services.NewLockWatcher(ctx, services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentAuth,
			Client:    p.a,
		},
	})
	if err != nil {
		return p, trace.Wrap(err)
	}
	p.a.SetLockWatcher(lockWatcher)

	urc, err := services.NewUnifiedResourceCache(ctx, services.UnifiedResourceCacheConfig{
		Clock: p.a.GetClock(),
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentAuth,
			Client:    p.a,
		},
		ResourceGetter: p.a,
	})
	if err != nil {
		return p, trace.Wrap(err)
	}

	p.a.SetUnifiedResourcesCache(urc)

	// set cluster name
	if err := p.a.SetClusterName(p.clusterName); err != nil {
		return p, trace.Wrap(err)
	}

	// set static tokens
	staticTokens, err := types.NewStaticTokens(types.StaticTokensSpecV2{
		StaticTokens: []types.ProvisionTokenV1{},
	})
	if err != nil {
		return p, trace.Wrap(err)
	}
	err = p.a.SetStaticTokens(staticTokens)
	if err != nil {
		return p, trace.Wrap(err)
	}

	authPreference, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOff,
	})
	if err != nil {
		return p, trace.Wrap(err)
	}
	if _, err = p.a.UpsertAuthPreference(ctx, authPreference); err != nil {
		return p, trace.Wrap(err)
	}
	if err := p.a.SetClusterAuditConfig(ctx, types.DefaultClusterAuditConfig()); err != nil {
		return p, trace.Wrap(err)
	}
	if _, err := p.a.UpsertClusterNetworkingConfig(ctx, types.DefaultClusterNetworkingConfig()); err != nil {
		return p, trace.Wrap(err)
	}
	if _, err := p.a.UpsertSessionRecordingConfig(ctx, types.DefaultSessionRecordingConfig()); err != nil {
		return p, trace.Wrap(err)
	}

	if err := p.a.UpsertCertAuthority(ctx, authtest.NewTestCA(types.UserCA, p.clusterName.GetClusterName())); err != nil {
		return p, trace.Wrap(err)
	}
	if err := p.a.UpsertCertAuthority(ctx, authtest.NewTestCA(types.HostCA, p.clusterName.GetClusterName())); err != nil {
		return p, trace.Wrap(err)
	}
	if err := p.a.UpsertCertAuthority(ctx, authtest.NewTestCA(types.OpenSSHCA, p.clusterName.GetClusterName())); err != nil {
		return p, trace.Wrap(err)
	}

	return p, nil
}

func newAuthSuite(t *testing.T) *testPack {
	s, err := newTestPack(context.Background(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		if s.bk != nil {
			s.bk.Close()
		}
	})

	return &s
}

func TestMain(m *testing.M) {
	logtest.InitLogger(testing.Verbose)

	ctx, cancel := context.WithCancel(context.Background())
	cryptosuitestest.PrecomputeRSAKeys(ctx)
	modules.SetInsecureTestMode(true)
	exitCode := m.Run()
	cancel()
	os.Exit(exitCode)
}

func TestSessions(t *testing.T) {
	t.Parallel()
	s := newAuthSuite(t)

	ctx := context.Background()

	user := "user1"
	pass := []byte("abcdef123456")

	_, _, err := authtest.CreateUserAndRole(s.a, user, []string{user}, nil)
	require.NoError(t, err)

	err = s.a.UpsertPassword(user, pass)
	require.NoError(t, err)

	authPref, err := s.a.GetAuthPreference(ctx)
	require.NoError(t, err)

	for _, tc := range []struct {
		desc                string
		suite               types.SignatureAlgorithmSuite
		expectSSHPubKeyType string
		expectTLSPubKeyAlgo x509.PublicKeyAlgorithm
		expectKeysToMatch   bool
	}{
		{
			desc:                "unspecified",
			suite:               types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_UNSPECIFIED,
			expectSSHPubKeyType: "ssh-rsa-cert-v01@openssh.com",
			expectTLSPubKeyAlgo: x509.RSA,
			expectKeysToMatch:   true,
		},
		{
			desc:                "legacy",
			suite:               types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_LEGACY,
			expectSSHPubKeyType: "ssh-rsa-cert-v01@openssh.com",
			expectTLSPubKeyAlgo: x509.RSA,
			expectKeysToMatch:   true,
		},
		{
			desc:                "balanced-v1",
			suite:               types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_BALANCED_V1,
			expectSSHPubKeyType: "ssh-ed25519-cert-v01@openssh.com",
			expectTLSPubKeyAlgo: x509.ECDSA,
		},
		{
			desc:                "fips-v1",
			suite:               types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_FIPS_V1,
			expectSSHPubKeyType: "ecdsa-sha2-nistp256-cert-v01@openssh.com",
			expectTLSPubKeyAlgo: x509.ECDSA,
		},
		{
			desc:                "hsm-v1",
			suite:               types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_HSM_V1,
			expectSSHPubKeyType: "ssh-ed25519-cert-v01@openssh.com",
			expectTLSPubKeyAlgo: x509.ECDSA,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			authPref.SetSignatureAlgorithmSuite(tc.suite)
			_, err := s.a.UpsertAuthPreference(ctx, authPref)
			require.NoError(t, err)

			ws, err := s.a.AuthenticateWebUser(ctx, authclient.AuthenticateUserRequest{
				Username: user,
				Pass:     &authclient.PassCreds{Password: pass},
			})
			require.NoError(t, err)
			require.NotNil(t, ws)

			if tc.expectKeysToMatch {
				assert.Equal(t, ws.GetSSHPriv(), ws.GetTLSPriv())
			} else {
				assert.NotEqual(t, ws.GetSSHPriv(), ws.GetTLSPriv())
			}

			pub, _, _, _, err := ssh.ParseAuthorizedKey(ws.GetPub())
			require.NoError(t, err)
			assert.Equal(t, tc.expectSSHPubKeyType, pub.Type())

			tlsCert, _ := parseX509PEMAndIdentity(t, ws.GetTLSCert())
			assert.Equal(t, tc.expectTLSPubKeyAlgo, tlsCert.PublicKeyAlgorithm)

			// GetWebSessionInfo and make sure it matches, with private keys removed.
			out, err := s.a.GetWebSessionInfo(ctx, user, ws.GetName())
			require.NoError(t, err)
			assert.Empty(t, out.GetSSHPriv())
			assert.Empty(t, out.GetTLSPriv())
			assert.Empty(t, gocmp.Diff(ws, out,
				cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
				cmpopts.IgnoreFields(types.WebSessionSpecV2{}, "Priv", "TLSPriv")))

			err = s.a.WebSessions().Delete(ctx, types.DeleteWebSessionRequest{
				User:      user,
				SessionID: ws.GetName(),
			})
			require.NoError(t, err)

			_, err = s.a.GetWebSession(ctx, types.GetWebSessionRequest{
				User:      user,
				SessionID: ws.GetName(),
			})
			assert.True(t, trace.IsNotFound(err), "%#v", err)
		})
	}
}

func TestAuthenticateWebUser_deviceWebToken(t *testing.T) {
	t.Parallel()
	s := newAuthSuite(t)

	authServer := s.a

	const user = "llama"
	const pass = "supersecretpassword!!1!"

	// Prepare user and password.
	// 2nd factors are not important for this test.
	_, _, err := authtest.CreateUserAndRole(authServer, user, []string{user}, nil /* allowRules */)
	require.NoError(t, err, "CreateUserAndRole failed")
	require.NoError(t,
		authServer.UpsertPassword(user, []byte(pass)),
		"UpsertPassword failed")

	const remoteIP = "40.89.244.232"
	const remoteAddr = remoteIP + ":4242"

	makeTokenSuccess := func(ctx context.Context, dwt *devicepb.DeviceWebToken) (*devicepb.DeviceWebToken, error) {
		switch {
		case dwt == nil:
			return nil, errors.New("dtw parameter is nil")
		case dwt.WebSessionId == "":
			return nil, errors.New("dwt.WebSessionId is empty")
		case dwt.BrowserUserAgent == "":
			return nil, errors.New("dwt.BrowserUserAgent is empty")
		case dwt.BrowserIp != remoteIP:
			return nil, errors.New("dwt.BrowserUserAgent is empty")
		case dwt.User != user:
			return nil, errors.New("dwt.User mismatch")
		}

		return &devicepb.DeviceWebToken{
			Id:    "this is an opaque ID",
			Token: "this is an opaque token",
		}, nil
	}

	makeTokenError := func(context.Context, *devicepb.DeviceWebToken) (*devicepb.DeviceWebToken, error) {
		return nil, errors.New("something bad happened")
	}

	// Use the BrowserUserAgent as a request key of sorts and save the
	// WebSessionID corresponding to each user agent.
	// This lets us use a single CreateDeviceWebTokenFunc and run all subtests in
	// parallel.
	// Only possible because the BrowserUserAgent is not interpreted by the fakes
	// used here.
	var uaToSessionID sync.Map // map[string]string
	authServer.SetCreateDeviceWebTokenFunc(func(ctx context.Context, dwt *devicepb.DeviceWebToken) (*devicepb.DeviceWebToken, error) {
		if dwt == nil {
			return nil, errors.New("dwt required")
		}
		uaToSessionID.Store(dwt.BrowserUserAgent, dwt.GetWebSessionId())

		if strings.HasPrefix(dwt.BrowserUserAgent, "fail") {
			return makeTokenError(ctx, dwt)
		}

		return makeTokenSuccess(ctx, dwt)
	})

	ctx := context.Background()
	makeReq := func(ua string) *authclient.AuthenticateUserRequest {
		return &authclient.AuthenticateUserRequest{
			Username: user,
			Pass: &authclient.PassCreds{
				Password: []byte(pass),
			},
			ClientMetadata: &authclient.ForwardedClientMetadata{
				UserAgent:  ua,
				RemoteAddr: remoteAddr,
			},
		}
	}

	tests := []struct {
		name      string
		req       *authclient.AuthenticateUserRequest
		wantToken bool
	}{
		{
			name:      "success",
			req:       makeReq("success1"),
			wantToken: true,
		},
		{
			name: "CreateDeviceWebToken fails",
			req:  makeReq("fail1"), // "fail" UA causes token creation to fail.
		},
		{
			name: "empty ClientMetadata.UserAgent skips token creation",
			req:  makeReq(""),
		},
		{
			name: "nil ClientMetadata skips token creation",
			req: func() *authclient.AuthenticateUserRequest {
				req := makeReq("")
				req.ClientMetadata = nil
				return req
			}(),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			req := test.req
			webSession, err := authServer.AuthenticateWebUser(ctx, *req)
			// AuthenticateWebUser is never expected to fail in this test.
			// Either a DeviceWebToken exists in the response, or it doesn't, but the
			// method itself always works.
			require.NoError(t, err, "AuthenticateWebUser failed unexpectedly")

			// Device requirement not calculated for OSS.
			assert.Equal(t,
				types.TrustedDeviceRequirement_TRUSTED_DEVICE_REQUIREMENT_UNSPECIFIED,
				webSession.GetTrustedDeviceRequirement(),
				"WebSession.TrustedDeviceRequirement mismatch",
			)

			// Verify the token itself.
			deviceToken := webSession.GetDeviceWebToken()
			if !test.wantToken {
				assert.Nil(t, deviceToken, "WebSession.GetDeviceWebToken is not nil")
				return
			}
			require.NotNil(t, deviceToken, "WebSession.GetDeviceWebToken is nil")
			assert.NotEmpty(t, deviceToken.Id, "DeviceWebToken.Id is empty")
			assert.NotEmpty(t, deviceToken.Token, "DeviceWebToken.Token is empty")

			// Verify the WebSessionId sent to CreateDeviceWebTokenFunc.
			var gotSessionID string
			if val, ok := uaToSessionID.Load(req.ClientMetadata.UserAgent); ok {
				gotSessionID = val.(string)
			} else {
				t.Fatal("WebSessionID not captured by test")
			}
			assert.Equal(t, webSession.GetName(), gotSessionID, "Captured DeviceWebToken.WebSessionId mismatch")
		})
	}
}

func TestAuthenticateWebUser_trustedDeviceRequirement(t *testing.T) {
	// Can't t.Parallel because of modules.SetTestModules.
	modulestest.SetTestModules(t, modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
	})

	s := newAuthSuite(t)
	authServer := s.a
	ctx := context.Background()

	const user1 = "llama"
	const pass1 = "supersecretpassword!!1!"
	const user2 = "alpaca"
	const pass2 = "supersecretpassword!!2!"

	// Create the require-trusted-device role.
	rtdRole := services.NewPresetRequireTrustedDeviceRole()
	require.NotNil(t, rtdRole, "require-trusted-device role is nil, are the modules set to Enterprise?")
	_, err := authServer.UpsertRole(ctx, rtdRole)
	require.NoError(t, err, "UpsertRole(%q) failed", rtdRole.GetName())

	// Create users.
	for _, u := range []struct {
		user, pass string
		roles      []string
	}{
		{user: user1, pass: pass1},
		{user: user2, pass: pass2, roles: []string{rtdRole.GetName()}},
	} {
		user, _, err := authtest.CreateUserAndRole(
			authServer,
			u.user,
			append(u.roles, u.user),
			nil, /* allowRules */
		)
		require.NoError(t, err, "CreateUserAndRole(%q) failed", u.user)
		require.NoError(t,
			authServer.UpsertPassword(u.user, []byte(u.pass)),
			"UpsertPassword(%q) failed", u.user)

		// Add extra roles to the user.
		if len(u.roles) > 0 {
			user.SetRoles(append(user.GetRoles(), u.roles...))
			_, err = authServer.Services.UpsertUser(ctx, user)
			require.NoError(t, err, "UpsertUser(%q) failed", u.user)
		}
	}

	const userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"
	const remoteIP = "40.89.244.232"
	const remoteAddr = remoteIP + ":4242"

	makeValidReq := func(user, pass string) *authclient.AuthenticateUserRequest {
		return &authclient.AuthenticateUserRequest{
			Username: user,
			Pass: &authclient.PassCreds{
				Password: []byte(pass),
			},
			ClientMetadata: &authclient.ForwardedClientMetadata{
				UserAgent:  userAgent,
				RemoteAddr: remoteAddr,
			},
		}
	}

	t.Run("required by cluster", func(t *testing.T) {
		// Set cluster mode to required.
		ap, err := authServer.GetAuthPreference(ctx)
		require.NoError(t, err, "GetAuthPreference failed")
		ap.SetDeviceTrust(&types.DeviceTrust{
			Mode: constants.DeviceTrustModeRequired,
		})
		_, err = authServer.UpsertAuthPreference(ctx, ap)
		require.NoError(t, err, "UpsertAuthPreference failed")

		// Reset mode after testing.
		t.Cleanup(func() {
			ap.GetDeviceTrust().Mode = ""
			_, err = authServer.UpsertAuthPreference(ctx, ap)
			assert.NoError(t, err, "cleanup: UpsertAuthPreference failed")
		})

		webSession, err := authServer.AuthenticateWebUser(ctx, *makeValidReq(user1, pass1))
		require.NoError(t, err, "AuthenticateWebUser failed unexpectedly")
		assert.Equal(t,
			types.TrustedDeviceRequirement_TRUSTED_DEVICE_REQUIREMENT_REQUIRED,
			webSession.GetTrustedDeviceRequirement(),
			"WebSession.TrustedDeviceRequirement mismatch",
		)
	})

	tests := []struct {
		name string
		req  *authclient.AuthenticateUserRequest
		want types.TrustedDeviceRequirement
	}{
		{
			name: "not required",
			req:  makeValidReq(user1, pass1),
			want: types.TrustedDeviceRequirement_TRUSTED_DEVICE_REQUIREMENT_NOT_REQUIRED,
		},
		{
			name: "required by role",
			req:  makeValidReq(user2, pass2), // user changed
			want: types.TrustedDeviceRequirement_TRUSTED_DEVICE_REQUIREMENT_REQUIRED,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			webSession, err := authServer.AuthenticateWebUser(ctx, *test.req)
			require.NoError(t, err, "AuthenticateWebUser failed unexpectedly")
			assert.Equal(t,
				test.want,
				webSession.GetTrustedDeviceRequirement(),
				"WebSession.TrustedDeviceRequirement mismatch",
			)
		})
	}
}

func TestAuthenticateSSHUser(t *testing.T) {
	t.Parallel()
	s := newAuthSuite(t)

	ctx := context.Background()

	// Register the leaf cluster.
	leaf, err := types.NewRemoteCluster("leaf.localhost")
	require.NoError(t, err)
	_, err = s.a.CreateRemoteCluster(ctx, leaf)
	require.NoError(t, err)

	user := "user1"
	pass := []byte("abcdef123456")

	// Try to login as an unknown user.
	_, err = s.a.AuthenticateSSHUser(ctx, authclient.AuthenticateSSHRequest{
		AuthenticateUserRequest: authclient.AuthenticateUserRequest{
			Username:     user,
			Pass:         &authclient.PassCreds{Password: pass},
			SSHPublicKey: []byte(sshPubKey),
			TLSPublicKey: []byte(tlsPubKey),
		},
	})
	var accessDeniedErr *trace.AccessDeniedError
	require.ErrorAs(t, err, &accessDeniedErr)

	// Create the user.
	_, role, err := authtest.CreateUserAndRole(s.a, user, []string{user}, nil)
	require.NoError(t, err)
	err = s.a.UpsertPassword(user, pass)
	require.NoError(t, err)
	// Give the role some k8s principals too.
	role.SetKubeUsers(types.Allow, []string{user})
	role.SetKubeGroups(types.Allow, []string{"system:masters"})

	role, err = s.a.UpsertRole(ctx, role)
	require.NoError(t, err)

	// Login to the root cluster.
	resp, err := s.a.AuthenticateSSHUser(ctx, authclient.AuthenticateSSHRequest{
		AuthenticateUserRequest: authclient.AuthenticateUserRequest{
			Username:     user,
			Pass:         &authclient.PassCreds{Password: pass},
			SSHPublicKey: []byte(sshPubKey),
			TLSPublicKey: []byte(tlsPubKey),
		},
		TTL:            time.Hour,
		RouteToCluster: s.clusterName.GetClusterName(),
	})
	require.NoError(t, err)
	require.Equal(t, user, resp.Username)
	// Verify the public key and principals in SSH cert.
	inSSHPub, _, _, _, err := ssh.ParseAuthorizedKey([]byte(sshPubKey))
	require.NoError(t, err)
	gotSSHCert, err := sshutils.ParseCertificate(resp.Cert)
	require.NoError(t, err)
	require.Equal(t, inSSHPub, gotSSHCert.Key)
	require.Equal(t, []string{user, teleport.SSHSessionJoinPrincipal}, gotSSHCert.ValidPrincipals)
	// Verify the public key and Subject in TLS cert.
	inTLSPub, err := keys.ParsePublicKey([]byte(tlsPubKey))
	require.NoError(t, err)
	gotTLSCert, err := tlsca.ParseCertificatePEM(resp.TLSCert)
	require.NoError(t, err)
	require.Equal(t, gotTLSCert.PublicKey, inTLSPub)
	wantID := tlsca.Identity{
		Username:         user,
		Groups:           []string{role.GetName()},
		Principals:       []string{user, teleport.SSHSessionJoinPrincipal},
		KubernetesUsers:  []string{user},
		KubernetesGroups: []string{"system:masters"},
		Expires:          gotTLSCert.NotAfter,
		RouteToCluster:   s.clusterName.GetClusterName(),
		TeleportCluster:  s.clusterName.GetClusterName(),
		PrivateKeyPolicy: keys.PrivateKeyPolicyNone,
		UserType:         "local",
	}
	gotID, err := tlsca.FromSubject(gotTLSCert.Subject, gotTLSCert.NotAfter)
	require.NoError(t, err)
	require.Equal(t, wantID, *gotID)

	// Login to the leaf cluster.
	resp, err = s.a.AuthenticateSSHUser(ctx, authclient.AuthenticateSSHRequest{
		AuthenticateUserRequest: authclient.AuthenticateUserRequest{
			Username:     user,
			Pass:         &authclient.PassCreds{Password: pass},
			SSHPublicKey: []byte(sshPubKey),
			TLSPublicKey: []byte(tlsPubKey),
		},
		TTL:               time.Hour,
		RouteToCluster:    "leaf.localhost",
		KubernetesCluster: "leaf-kube-cluster",
	})
	require.NoError(t, err)
	// Verify the TLS cert has the correct RouteToCluster set.
	gotTLSCert, err = tlsca.ParseCertificatePEM(resp.TLSCert)
	require.NoError(t, err)
	wantID = tlsca.Identity{
		Username:         user,
		Groups:           []string{role.GetName()},
		Principals:       []string{user, teleport.SSHSessionJoinPrincipal},
		KubernetesUsers:  []string{user},
		KubernetesGroups: []string{"system:masters"},
		// It's OK to use a non-existent kube cluster for leaf teleport
		// clusters. The leaf is responsible for validating those.
		KubernetesCluster: "leaf-kube-cluster",
		Expires:           gotTLSCert.NotAfter,
		RouteToCluster:    "leaf.localhost",
		TeleportCluster:   s.clusterName.GetClusterName(),
		PrivateKeyPolicy:  keys.PrivateKeyPolicyNone,
		UserType:          "local",
	}
	gotID, err = tlsca.FromSubject(gotTLSCert.Subject, gotTLSCert.NotAfter)
	require.NoError(t, err)
	require.Equal(t, wantID, *gotID)

	kubeCluster, err := types.NewKubernetesClusterV3(
		types.Metadata{
			Name: "root-kube-cluster",
		},
		types.KubernetesClusterSpecV3{},
	)
	require.NoError(t, err)

	kubeServer, err := types.NewKubernetesServerV3FromCluster(kubeCluster, "host", "uuid")
	require.NoError(t, err)
	_, err = s.a.UpsertKubernetesServer(ctx, kubeServer)
	require.NoError(t, err)

	// Wait for cache propagation of the kubernetes resources before proceeding with the tests.
	require.Eventually(t, func() bool {
		for ks := range s.a.UnifiedResourceCache.KubernetesServers(ctx, services.UnifiedResourcesIterateParams{}) {
			if ks.GetCluster().GetName() == kubeCluster.GetName() {
				return true
			}
		}
		return false
	}, 10*time.Second, 100*time.Millisecond)

	// Login specifying a valid kube cluster. It should appear in the TLS cert.
	resp, err = s.a.AuthenticateSSHUser(ctx, authclient.AuthenticateSSHRequest{
		AuthenticateUserRequest: authclient.AuthenticateUserRequest{
			Username:     user,
			Pass:         &authclient.PassCreds{Password: pass},
			SSHPublicKey: []byte(sshPubKey),
			TLSPublicKey: []byte(tlsPubKey),
		},
		TTL:               time.Hour,
		RouteToCluster:    s.clusterName.GetClusterName(),
		KubernetesCluster: "root-kube-cluster",
	})
	require.NoError(t, err)
	require.Equal(t, resp.Username, user)
	gotTLSCert, err = tlsca.ParseCertificatePEM(resp.TLSCert)
	require.NoError(t, err)
	wantID = tlsca.Identity{
		Username:          user,
		Groups:            []string{role.GetName()},
		Principals:        []string{user, teleport.SSHSessionJoinPrincipal},
		KubernetesUsers:   []string{user},
		KubernetesGroups:  []string{"system:masters"},
		KubernetesCluster: "root-kube-cluster",
		Expires:           gotTLSCert.NotAfter,
		RouteToCluster:    s.clusterName.GetClusterName(),
		TeleportCluster:   s.clusterName.GetClusterName(),
		PrivateKeyPolicy:  keys.PrivateKeyPolicyNone,
		UserType:          "local",
	}
	gotID, err = tlsca.FromSubject(gotTLSCert.Subject, gotTLSCert.NotAfter)
	require.NoError(t, err)
	require.Equal(t, wantID, *gotID)

	// Login without specifying kube cluster. Kube cluster in the certificate should be empty.
	resp, err = s.a.AuthenticateSSHUser(ctx, authclient.AuthenticateSSHRequest{
		AuthenticateUserRequest: authclient.AuthenticateUserRequest{
			Username:     user,
			Pass:         &authclient.PassCreds{Password: pass},
			SSHPublicKey: []byte(sshPubKey),
			TLSPublicKey: []byte(tlsPubKey),
		},
		TTL:            time.Hour,
		RouteToCluster: s.clusterName.GetClusterName(),
		// Intentionally empty, auth server should default to a registered
		// kubernetes cluster.
		KubernetesCluster: "",
	})
	require.NoError(t, err)
	require.Equal(t, user, resp.Username)
	gotTLSCert, err = tlsca.ParseCertificatePEM(resp.TLSCert)
	require.NoError(t, err)
	wantID = tlsca.Identity{
		Username:         user,
		Groups:           []string{role.GetName()},
		Principals:       []string{user, teleport.SSHSessionJoinPrincipal},
		KubernetesUsers:  []string{user},
		KubernetesGroups: []string{"system:masters"},
		Expires:          gotTLSCert.NotAfter,
		RouteToCluster:   s.clusterName.GetClusterName(),
		TeleportCluster:  s.clusterName.GetClusterName(),
		PrivateKeyPolicy: keys.PrivateKeyPolicyNone,
		UserType:         "local",
	}
	gotID, err = tlsca.FromSubject(gotTLSCert.Subject, gotTLSCert.NotAfter)
	require.NoError(t, err)
	require.Equal(t, wantID, *gotID)

	// Login specifying a valid kube cluster. It should appear in the TLS cert.
	resp, err = s.a.AuthenticateSSHUser(ctx, authclient.AuthenticateSSHRequest{
		AuthenticateUserRequest: authclient.AuthenticateUserRequest{
			Username:     user,
			Pass:         &authclient.PassCreds{Password: pass},
			SSHPublicKey: []byte(sshPubKey),
			TLSPublicKey: []byte(tlsPubKey),
		},
		TTL:               time.Hour,
		RouteToCluster:    s.clusterName.GetClusterName(),
		KubernetesCluster: "root-kube-cluster",
	})
	require.NoError(t, err)
	require.Equal(t, user, resp.Username)
	gotTLSCert, err = tlsca.ParseCertificatePEM(resp.TLSCert)
	require.NoError(t, err)
	wantID = tlsca.Identity{
		Username:          user,
		Groups:            []string{role.GetName()},
		Principals:        []string{user, teleport.SSHSessionJoinPrincipal},
		KubernetesUsers:   []string{user},
		KubernetesGroups:  []string{"system:masters"},
		KubernetesCluster: "root-kube-cluster",
		Expires:           gotTLSCert.NotAfter,
		RouteToCluster:    s.clusterName.GetClusterName(),
		TeleportCluster:   s.clusterName.GetClusterName(),
		PrivateKeyPolicy:  keys.PrivateKeyPolicyNone,
		UserType:          "local",
	}
	gotID, err = tlsca.FromSubject(gotTLSCert.Subject, gotTLSCert.NotAfter)
	require.NoError(t, err)
	require.Equal(t, wantID, *gotID)

	// Login without specifying kube cluster. Kube cluster in the certificate should be empty.
	resp, err = s.a.AuthenticateSSHUser(ctx, authclient.AuthenticateSSHRequest{
		AuthenticateUserRequest: authclient.AuthenticateUserRequest{
			Username:     user,
			Pass:         &authclient.PassCreds{Password: pass},
			SSHPublicKey: []byte(sshPubKey),
			TLSPublicKey: []byte(tlsPubKey),
		},
		TTL:            time.Hour,
		RouteToCluster: s.clusterName.GetClusterName(),
		// Intentionally empty, auth server should default to a registered
		// kubernetes cluster.
		KubernetesCluster: "",
	})
	require.NoError(t, err)
	require.Equal(t, user, resp.Username)
	gotTLSCert, err = tlsca.ParseCertificatePEM(resp.TLSCert)
	require.NoError(t, err)
	wantID = tlsca.Identity{
		Username:         user,
		Groups:           []string{role.GetName()},
		Principals:       []string{user, teleport.SSHSessionJoinPrincipal},
		KubernetesUsers:  []string{user},
		KubernetesGroups: []string{"system:masters"},
		Expires:          gotTLSCert.NotAfter,
		RouteToCluster:   s.clusterName.GetClusterName(),
		TeleportCluster:  s.clusterName.GetClusterName(),
		PrivateKeyPolicy: keys.PrivateKeyPolicyNone,
		UserType:         "local",
	}
	gotID, err = tlsca.FromSubject(gotTLSCert.Subject, gotTLSCert.NotAfter)
	require.NoError(t, err)
	require.Equal(t, wantID, *gotID)

	// Login specifying an invalid kube cluster. This should fail.
	_, err = s.a.AuthenticateSSHUser(ctx, authclient.AuthenticateSSHRequest{
		AuthenticateUserRequest: authclient.AuthenticateUserRequest{
			Username:     user,
			Pass:         &authclient.PassCreds{Password: pass},
			SSHPublicKey: []byte(sshPubKey),
			TLSPublicKey: []byte(tlsPubKey),
		},
		TTL:               time.Hour,
		RouteToCluster:    s.clusterName.GetClusterName(),
		KubernetesCluster: "invalid-kube-cluster",
	})
	require.Error(t, err)
}

func TestAuthenticateUser_mfaDeviceLocked(t *testing.T) {
	t.Parallel()

	testServer := newTestTLSServer(t)
	authServer := testServer.Auth()

	ctx := context.Background()
	const user = "llama"
	const pass = "supersecret!!1!one"

	// Configure auth preferences.
	authPref, err := authServer.GetAuthPreference(ctx)
	require.NoError(t, err, "GetAuthPreference")
	authPref.SetSecondFactors(types.SecondFactorType_SECOND_FACTOR_TYPE_WEBAUTHN)
	authPref.SetWebauthn(&types.Webauthn{
		RPID: "localhost",
	})
	_, err = authServer.UpdateAuthPreference(ctx, authPref)
	require.NoError(t, err, "UpdateAuthPreference")

	// Prepare user, password and MFA device.
	_, _, err = authtest.CreateUserAndRole(authServer, user, []string{user}, nil /* allowRules */)
	require.NoError(t, err, "CreateUserAndRole")
	require.NoError(t,
		authServer.UpsertPassword(user, []byte(pass)),
		"UpsertPassword")

	userClient, err := testServer.NewClient(authtest.TestUser(user))
	require.NoError(t, err, "NewClient")

	// OTP devices would work for this test too.
	dev1, err := authtest.RegisterTestDevice(ctx, userClient, "dev1", proto.DeviceType_DEVICE_TYPE_WEBAUTHN, nil /* authenticator */)
	require.NoError(t, err, "RegisterTestDevice")
	dev2, err := authtest.RegisterTestDevice(ctx, userClient, "dev2", proto.DeviceType_DEVICE_TYPE_WEBAUTHN, dev1 /* authenticator */)
	require.NoError(t, err, "RegisterTestDevice")

	// Users initially authenticate via Proxy, as there isn't a userClient before
	// authn.
	proxyClient, err := testServer.NewClient(authtest.TestBuiltin(types.RoleProxy))
	require.NoError(t, err, "NewClient")

	authenticateSSH := func(dev *authtest.Device) (*authclient.SSHLoginResponse, error) {
		chal, err := proxyClient.CreateAuthenticateChallenge(ctx, &proto.CreateAuthenticateChallengeRequest{
			Request: &proto.CreateAuthenticateChallengeRequest_UserCredentials{
				UserCredentials: &proto.UserCredentials{
					Username: user,
					Password: []byte(pass),
				},
			},
		})
		if err != nil {
			return nil, fmt.Errorf("create challenge: %w", err)
		}

		chalResp, err := dev.SolveAuthn(chal)
		if err != nil {
			return nil, fmt.Errorf("solve challenge: %w", err)
		}

		return proxyClient.AuthenticateSSHUser(ctx, authclient.AuthenticateSSHRequest{
			AuthenticateUserRequest: authclient.AuthenticateUserRequest{
				Username:     user,
				SSHPublicKey: []byte(sshPubKey),
				TLSPublicKey: []byte(tlsPubKey),
				Pass: &authclient.PassCreds{
					Password: []byte(pass),
				},
				Webauthn: wantypes.CredentialAssertionResponseFromProto(chalResp.GetWebauthn()),
			},
			TTL: 1 * time.Hour,
		})
	}

	// Lock dev1.
	const lockMessage = "device locked for testing"
	lock, err := types.NewLock("dev1-lock", types.LockSpecV2{
		Target: types.LockTarget{
			MFADevice: dev1.MFA.Id,
		},
		Message: lockMessage,
	})
	require.NoError(t, err, "NewLock")
	require.NoError(t,
		userClient.UpsertLock(ctx, lock),
		"UpsertLock")

	t.Run("locked device", func(t *testing.T) {
		_, err := authenticateSSH(dev1)
		assert.ErrorContains(t, err, lockMessage)
	})

	t.Run("unlocked device", func(t *testing.T) {
		_, err := authenticateSSH(dev2)
		assert.NoError(t, err, "authenticateSSH failed unexpectedly")
	})

	t.Run("locked device password change", func(t *testing.T) {
		chal, err := userClient.CreateAuthenticateChallenge(ctx, &proto.CreateAuthenticateChallengeRequest{
			Request: &proto.CreateAuthenticateChallengeRequest_ContextUser{
				ContextUser: &proto.ContextUser{},
			},
			ChallengeExtensions: &mfav1.ChallengeExtensions{
				Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_CHANGE_PASSWORD,
			},
		})
		require.NoError(t, err, "CreateAuthenticateChallenge")

		// dev1 is still locked.
		chalResp, err := dev1.SolveAuthn(chal)
		require.NoError(t, err, "SolveAuthn")

		assert.ErrorContains(t,
			userClient.ChangePassword(ctx, &proto.ChangePasswordRequest{
				User:        user,
				OldPassword: []byte(pass),
				NewPassword: []byte("evenmoresecret!!1!ONE"),
				Webauthn:    chalResp.GetWebauthn(),
			}),
			lockMessage)
	})
}

func TestUserLock(t *testing.T) {
	t.Parallel()
	s := newAuthSuite(t)
	ctx := context.Background()

	username := "user1"
	pass := []byte("abcdef123456")

	_, err := s.a.AuthenticateWebUser(ctx, authclient.AuthenticateUserRequest{
		Username: username,
		Pass:     &authclient.PassCreds{Password: pass},
	})
	require.Error(t, err)

	_, _, err = authtest.CreateUserAndRole(s.a, username, []string{username}, nil)
	require.NoError(t, err)

	err = s.a.UpsertPassword(username, pass)
	require.NoError(t, err)

	// successful log in
	ws, err := s.a.AuthenticateWebUser(ctx, authclient.AuthenticateUserRequest{
		Username: username,
		Pass:     &authclient.PassCreds{Password: pass},
	})
	require.NoError(t, err)
	require.NotNil(t, ws)

	fakeClock := clockwork.NewFakeClock()
	s.a.SetClock(fakeClock)

	for i := 0; i <= defaults.MaxLoginAttempts; i++ {
		_, err = s.a.AuthenticateWebUser(ctx, authclient.AuthenticateUserRequest{
			Username: username,
			Pass:     &authclient.PassCreds{Password: []byte("wrong password")},
		})
		require.Error(t, err)
	}

	user, err := s.a.GetUser(ctx, username, false)
	require.NoError(t, err)
	require.True(t, user.GetStatus().IsLocked)

	// advance time and make sure we can login again
	fakeClock.Advance(defaults.AccountLockInterval + time.Second)

	_, err = s.a.AuthenticateWebUser(ctx, authclient.AuthenticateUserRequest{
		Username: username,
		Pass:     &authclient.PassCreds{Password: pass},
	})
	require.NoError(t, err)
}

func TestAuth_SetStaticTokens(t *testing.T) {
	t.Parallel()
	s := newAuthSuite(t)
	ctx := context.Background()

	roles := types.SystemRoles{types.RoleProxy}
	st, err := types.NewStaticTokens(types.StaticTokensSpecV2{
		StaticTokens: []types.ProvisionTokenV1{{
			Token:   "static-token-value",
			Roles:   roles,
			Expires: time.Unix(0, 0).UTC(),
		}},
	})
	require.NoError(t, err)
	err = s.a.SetStaticTokens(st)
	require.NoError(t, err)
	token, err := s.a.ValidateToken(ctx, "static-token-value")
	require.NoError(t, err)
	fetchesRoles := token.GetRoles()
	require.Equal(t, fetchesRoles, roles)
}

type tokenCreatorAndDeleter interface {
	CreateToken(ctx context.Context, token types.ProvisionToken) error
	DeleteToken(ctx context.Context, token string) error
}

func generateTestToken(
	ctx context.Context,
	t *testing.T,
	roles types.SystemRoles,
	expires time.Time,
	auth tokenCreatorAndDeleter,
) string {
	t.Helper()
	token, err := utils.CryptoRandomHex(defaults.TokenLenBytes)
	require.NoError(t, err)

	pt, err := types.NewProvisionToken(token, roles, expires)
	require.NoError(t, err)
	require.NoError(t, auth.CreateToken(ctx, pt))

	return token
}

func TestBadTokens(t *testing.T) {
	t.Parallel()
	s := newAuthSuite(t)

	ctx := context.Background()
	// empty
	_, err := s.a.ValidateToken(ctx, "")
	require.Error(t, err)

	// garbage
	_, err = s.a.ValidateToken(ctx, "bla bla")
	require.Error(t, err)

	// tampered
	tok := generateTestToken(
		ctx, t,
		types.SystemRoles{types.RoleNode},
		time.Time{},
		s.a,
	)
	tampered := string(tok[0]+1) + tok[1:]
	_, err = s.a.ValidateToken(ctx, tampered)
	require.Error(t, err)
}

// TestLocalControlStream verifies that local control stream behaves as expected.
func TestLocalControlStream(t *testing.T) {
	const serverID = "test-server"

	t.Parallel()
	ctx := t.Context()

	s := newAuthSuite(t)

	stream := s.a.MakeLocalInventoryControlStream()
	defer stream.Close()

	err := stream.Send(ctx, &proto.UpstreamInventoryHello{
		ServerID: serverID,
	})
	require.NoError(t, err)

	select {
	case msg := <-stream.Recv():
		_, ok := msg.(*proto.DownstreamInventoryHello)
		require.True(t, ok)
	case <-stream.Done():
		t.Fatalf("stream closed unexpectedly: %v", stream.Error())
	case <-time.After(time.Second * 10):
		t.Fatal("timeout waiting for downstream hello")
	}

	// wait for control stream to get inserted into the controller (happens after
	// hello exchange is finished).
	require.Eventually(t, func() bool {
		_, ok := s.a.Inventory().GetControlStream(serverID)
		return ok
	}, time.Second*5, time.Millisecond*200)

	// try performing a normal operation against the control stream to double-check that it is healthy
	go s.a.PingInventory(ctx, proto.InventoryPingRequest{
		ServerID: serverID,
	})

	select {
	case msg := <-stream.Recv():
		require.IsType(t, *new(*proto.DownstreamInventoryPing), msg)
	case <-stream.Done():
		t.Fatalf("stream closed unexpectedly: %v", stream.Error())
	case <-time.After(time.Second * 10):
		t.Fatal("timeout waiting for downstream hello")
	}
}

func TestUpdateConfig(t *testing.T) {
	t.Parallel()
	s := newAuthSuite(t)
	ctx := context.Background()

	cn, err := s.a.GetClusterName(ctx)
	require.NoError(t, err)
	require.Equal(t, cn.GetClusterName(), s.clusterName.GetClusterName())
	st, err := s.a.GetStaticTokens()
	require.NoError(t, err)
	require.Empty(t, st.GetStaticTokens())

	// try and set cluster name, this should fail because you can only set the
	// cluster name once
	clusterName, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: "foo.localhost",
	})
	require.NoError(t, err)
	// use same backend but start a new auth server with different config.
	authConfig := &auth.InitConfig{
		ClusterName:            clusterName,
		Backend:                s.bk,
		VersionStorage:         s.versionStorage,
		Authority:              testauthority.New(),
		SkipPeriodicOperations: true,
	}
	authServer, err := auth.NewServer(authConfig)
	require.NoError(t, err)

	err = authServer.SetClusterName(clusterName)
	require.Error(t, err)
	// try and set static tokens, this should be successful because the last
	// one to upsert tokens wins
	staticTokens, err := types.NewStaticTokens(types.StaticTokensSpecV2{
		StaticTokens: []types.ProvisionTokenV1{{
			Token: "bar",
			Roles: types.SystemRoles{types.SystemRole("baz")},
		}},
	})
	require.NoError(t, err)
	err = authServer.SetStaticTokens(staticTokens)
	require.NoError(t, err)

	// check first auth server and make sure it returns the correct values
	// (original cluster name, new static tokens)
	cn, err = s.a.GetClusterName(ctx)
	require.NoError(t, err)
	require.Equal(t, cn.GetClusterName(), s.clusterName.GetClusterName())
	st, err = s.a.GetStaticTokens()
	require.NoError(t, err)
	require.Equal(t, st.GetStaticTokens(), types.ProvisionTokensFromStatic([]types.ProvisionTokenV1{{
		Token: "bar",
		Roles: types.SystemRoles{types.SystemRole("baz")},
	}}))

	// check second auth server and make sure it also has the correct values
	// new static tokens
	st, err = authServer.GetStaticTokens()
	require.NoError(t, err)
	require.Equal(t, st.GetStaticTokens(), types.ProvisionTokensFromStatic([]types.ProvisionTokenV1{{
		Token: "bar",
		Roles: types.SystemRoles{types.SystemRole("baz")},
	}}))
}

func TestTrustedClusterCRUDEventEmitted(t *testing.T) {
	t.Parallel()
	s := newAuthSuite(t)

	clientAddr := &net.TCPAddr{IP: net.IPv4(10, 255, 0, 0)}
	ctx := authz.ContextWithClientSrcAddr(context.Background(), clientAddr)
	s.a.SetEmitter(s.mockEmitter)

	// set up existing cluster to bypass switch cases that
	// makes a network request when creating new clusters
	tc, err := types.NewTrustedCluster("test", types.TrustedClusterSpecV2{
		Enabled:              true,
		Roles:                []string{"a"},
		ReverseTunnelAddress: "b",
	})
	require.NoError(t, err)
	// use the UpsertTrustedCluster in Uncached as we just want the resource in
	// the backend, we don't want to actually connect
	_, err = s.a.Services.UpsertTrustedCluster(ctx, tc)
	require.NoError(t, err)

	require.NoError(t, s.a.UpsertCertAuthority(ctx, authtest.NewTestCA(types.UserCA, "test")))
	require.NoError(t, s.a.UpsertCertAuthority(ctx, authtest.NewTestCA(types.HostCA, "test")))

	err = s.a.CreateReverseTunnel(ctx, tc)
	require.NoError(t, err)

	// test create event for switch case: when tc exists but enabled is false
	tc.SetEnabled(false)

	_, err = s.a.UpsertTrustedClusterV2(ctx, tc)
	require.NoError(t, err)
	require.Equal(t, events.TrustedClusterCreateEvent, s.mockEmitter.LastEvent().GetType())
	createEvt := s.mockEmitter.LastEvent().(*apievents.TrustedClusterCreate)
	require.Equal(t, clientAddr.String(), createEvt.ConnectionMetadata.RemoteAddr)
	s.mockEmitter.Reset()

	// test create event for switch case: when tc exists but enabled is true
	tc.SetEnabled(true)

	_, err = s.a.UpsertTrustedClusterV2(ctx, tc)
	require.NoError(t, err)
	require.Equal(t, events.TrustedClusterCreateEvent, s.mockEmitter.LastEvent().GetType())
	createEvt = s.mockEmitter.LastEvent().(*apievents.TrustedClusterCreate)
	require.Equal(t, clientAddr.String(), createEvt.ConnectionMetadata.RemoteAddr)
	s.mockEmitter.Reset()

	// test delete event
	err = s.a.DeleteTrustedCluster(ctx, "test")
	require.NoError(t, err)
	require.Equal(t, events.TrustedClusterDeleteEvent, s.mockEmitter.LastEvent().GetType())
	deleteEvt := s.mockEmitter.LastEvent().(*apievents.TrustedClusterDelete)
	require.Equal(t, clientAddr.String(), deleteEvt.ConnectionMetadata.RemoteAddr)
}

func TestGithubConnectorCRUDEventsEmitted(t *testing.T) {
	t.Parallel()
	s := newAuthSuite(t)

	clientAddr := &net.TCPAddr{IP: net.IPv4(10, 255, 0, 0)}
	ctx := authz.ContextWithClientSrcAddr(context.Background(), clientAddr)
	github, err := types.NewGithubConnector("test", types.GithubConnectorSpecV3{
		TeamsToRoles: []types.TeamRolesMapping{
			{
				Organization: "octocats",
				Team:         "dummy",
				Roles:        []string{"dummy"},
			},
		},
	})
	// test github create event
	require.NoError(t, err)
	github, err = auth.CreateGithubConnector(ctx, s.a, github)
	require.NoError(t, err)
	require.IsType(t, &apievents.GithubConnectorCreate{}, s.mockEmitter.LastEvent())
	require.Equal(t, events.GithubConnectorCreatedEvent, s.mockEmitter.LastEvent().GetType())
	createEvt := s.mockEmitter.LastEvent().(*apievents.GithubConnectorCreate)
	require.Equal(t, clientAddr.String(), createEvt.ConnectionMetadata.RemoteAddr)
	s.mockEmitter.Reset()

	// test github update event
	github.SetDisplay("llama")
	github, err = auth.UpdateGithubConnector(ctx, s.a, github)
	require.NoError(t, err)
	require.IsType(t, &apievents.GithubConnectorUpdate{}, s.mockEmitter.LastEvent())
	require.Equal(t, events.GithubConnectorUpdatedEvent, s.mockEmitter.LastEvent().GetType())
	updateEvt := s.mockEmitter.LastEvent().(*apievents.GithubConnectorUpdate)
	require.Equal(t, clientAddr.String(), updateEvt.ConnectionMetadata.RemoteAddr)
	s.mockEmitter.Reset()

	// test github upsert event
	github.SetDisplay("alpaca")
	upserted, err := auth.UpsertGithubConnector(ctx, s.a, github)
	require.NoError(t, err)
	require.NotNil(t, upserted)
	require.IsType(t, &apievents.GithubConnectorCreate{}, s.mockEmitter.LastEvent())
	require.Equal(t, events.GithubConnectorCreatedEvent, s.mockEmitter.LastEvent().GetType())
	createEvt = s.mockEmitter.LastEvent().(*apievents.GithubConnectorCreate)
	require.Equal(t, clientAddr.String(), createEvt.ConnectionMetadata.RemoteAddr)
	s.mockEmitter.Reset()

	// test github delete event
	err = auth.DeleteGithubConnector(ctx, s.a, "test")
	require.NoError(t, err)
	require.IsType(t, &apievents.GithubConnectorDelete{}, s.mockEmitter.LastEvent())
	require.Equal(t, events.GithubConnectorDeletedEvent, s.mockEmitter.LastEvent().GetType())
	deleteEvt := s.mockEmitter.LastEvent().(*apievents.GithubConnectorDelete)
	require.Equal(t, clientAddr.String(), deleteEvt.ConnectionMetadata.RemoteAddr)
}

func TestOIDCConnectorCRUDEventsEmitted(t *testing.T) {
	t.Parallel()
	s := newAuthSuite(t)

	ctx := context.Background()
	oidc, err := types.NewOIDCConnector("test", types.OIDCConnectorSpecV3{
		ClientID:     "a",
		ClientSecret: "b",
		ClaimsToRoles: []types.ClaimMapping{
			{
				Claim: "dummy",
				Value: "dummy",
				Roles: []string{"dummy"},
			},
		},
		RedirectURLs: []string{"https://proxy.example.com/v1/webapi/oidc/callback"},
	})
	require.NoError(t, err)

	// test oidc create event
	oidc, err = s.a.CreateOIDCConnector(ctx, oidc)
	require.NoError(t, err)
	require.IsType(t, &apievents.OIDCConnectorCreate{}, s.mockEmitter.LastEvent())
	require.Equal(t, events.OIDCConnectorCreatedEvent, s.mockEmitter.LastEvent().GetType())
	s.mockEmitter.Reset()

	// test oidc update event
	oidc.SetDisplay("llama")
	oidc, err = s.a.UpdateOIDCConnector(ctx, oidc)
	require.NoError(t, err)
	require.IsType(t, &apievents.OIDCConnectorUpdate{}, s.mockEmitter.LastEvent())
	require.Equal(t, events.OIDCConnectorUpdatedEvent, s.mockEmitter.LastEvent().GetType())
	s.mockEmitter.Reset()

	// test oidc upsert event
	oidc.SetDisplay("alpaca")
	upserted, err := s.a.UpsertOIDCConnector(ctx, oidc)
	require.NoError(t, err)
	require.NotNil(t, upserted)
	require.IsType(t, &apievents.OIDCConnectorCreate{}, s.mockEmitter.LastEvent())
	require.Equal(t, events.OIDCConnectorCreatedEvent, s.mockEmitter.LastEvent().GetType())
	s.mockEmitter.Reset()

	// test oidc delete event
	err = s.a.DeleteOIDCConnector(ctx, "test")
	require.NoError(t, err)
	require.IsType(t, &apievents.OIDCConnectorDelete{}, s.mockEmitter.LastEvent())
	require.Equal(t, events.OIDCConnectorDeletedEvent, s.mockEmitter.LastEvent().GetType())
}

func TestSAMLConnectorCRUDEventsEmitted(t *testing.T) {
	t.Parallel()
	s := newAuthSuite(t)

	ctx := context.Background()
	// generate a certificate that makes ParseCertificatePEM happy, copied from ca_test.go
	ca, err := tlsca.FromKeys([]byte(fixtures.TLSCACertPEM), []byte(fixtures.TLSCAKeyPEM))
	require.NoError(t, err)

	privateKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)

	testClock := clockwork.NewFakeClock()
	certBytes, err := ca.GenerateCertificate(tlsca.CertificateRequest{
		Clock:     testClock,
		PublicKey: privateKey.Public(),
		Subject:   pkix.Name{CommonName: "test"},
		NotAfter:  testClock.Now().Add(time.Hour),
	})
	require.NoError(t, err)

	// SAML connector validation requires the roles in mappings exist.
	role, err := types.NewRole("dummy", types.RoleSpecV6{})
	require.NoError(t, err)
	role, err = s.a.CreateRole(ctx, role)
	require.NoError(t, err)

	saml, err := types.NewSAMLConnector("test", types.SAMLConnectorSpecV2{
		AssertionConsumerService: "a",
		Issuer:                   "b",
		SSO:                      "https://example.com",
		AttributesToRoles: []types.AttributeMapping{
			{
				Name:  "dummy",
				Value: "dummy",
				Roles: []string{role.GetName()},
			},
		},
		Cert: string(certBytes),
	})
	require.NoError(t, err)

	// test saml create
	saml, err = s.a.CreateSAMLConnector(ctx, saml)
	require.NoError(t, err)
	require.IsType(t, &apievents.SAMLConnectorCreate{}, s.mockEmitter.LastEvent())
	require.Equal(t, events.SAMLConnectorCreatedEvent, s.mockEmitter.LastEvent().GetType())
	s.mockEmitter.Reset()

	// test saml update event
	saml.SetDisplay("llama")
	saml, err = s.a.UpdateSAMLConnector(ctx, saml)
	require.NoError(t, err)
	require.IsType(t, &apievents.SAMLConnectorUpdate{}, s.mockEmitter.LastEvent())
	require.Equal(t, events.SAMLConnectorUpdatedEvent, s.mockEmitter.LastEvent().GetType())
	s.mockEmitter.Reset()

	// test saml upsert event
	saml.SetDisplay("alapaca")
	_, err = s.a.UpsertSAMLConnector(ctx, saml)
	require.NoError(t, err)
	require.IsType(t, &apievents.SAMLConnectorCreate{}, s.mockEmitter.LastEvent())
	require.Equal(t, events.SAMLConnectorCreatedEvent, s.mockEmitter.LastEvent().GetType())
	s.mockEmitter.Reset()

	// test saml delete event
	err = s.a.DeleteSAMLConnector(ctx, "test")
	require.NoError(t, err)
	require.IsType(t, &apievents.SAMLConnectorDelete{}, s.mockEmitter.LastEvent())
	require.Equal(t, events.SAMLConnectorDeletedEvent, s.mockEmitter.LastEvent().GetType())
}

func TestEmitSSOLoginFailureEvent(t *testing.T) {
	mockE := &eventstest.MockRecorderEmitter{}

	auth.EmitSSOLoginFailureEvent(context.Background(), mockE, "test", trace.BadParameter("some error"), false)

	expectedLoginFailure := &apievents.UserLogin{
		Metadata: apievents.Metadata{
			Type: events.UserLoginEvent,
			Code: events.UserSSOLoginFailureCode,
		},
		Method: "test",
		Status: apievents.Status{
			Success:     false,
			Error:       "some error",
			UserMessage: "some error",
		},
	}
	require.Equal(t, expectedLoginFailure, mockE.LastEvent())

	auth.EmitSSOLoginFailureEvent(context.Background(), mockE, "test", trace.BadParameter("some error"), true)

	expectedTestFailure := &apievents.UserLogin{
		Metadata: apievents.Metadata{
			Type: events.UserLoginEvent,
			Code: events.UserSSOTestFlowLoginFailureCode,
		},
		Method: "test",
		Status: apievents.Status{
			Success:     false,
			Error:       "some error",
			UserMessage: "some error",
		},
	}
	require.Equal(t, expectedTestFailure, mockE.LastEvent())
}

func TestServer_AugmentContextUserCertificates(t *testing.T) {
	t.Parallel()

	testServer := newTestTLSServer(t)
	authServer := testServer.Auth()
	emitter := &eventstest.MockRecorderEmitter{}
	authServer.SetEmitter(emitter)
	ctx := context.Background()

	const username = "llama"
	const pass = "secret!!1!!!"

	// Use a >1 list of principals.
	// This is enough to cause ordering issues between the TLS and SSH principal
	// lists, which caused a bug in the device trust preview.
	principals := []string{"login0", username, "-teleport-internal-join"}

	// Prepare the user to test with.
	_, _, err := authtest.CreateUserAndRole(authServer, username, principals, nil)
	require.NoError(t, err, "CreateUserAndRole failed")
	require.NoError(t,
		authServer.UpsertPassword(username, []byte(pass)),
		"UpsertPassword failed")

	// Authenticate and create certificates.
	authResp, err := authServer.AuthenticateSSHUser(ctx, authclient.AuthenticateSSHRequest{
		AuthenticateUserRequest: authclient.AuthenticateUserRequest{
			Username: username,
			Pass: &authclient.PassCreds{
				Password: []byte(pass),
			},
			SSHPublicKey: []byte(sshPubKey),
			TLSPublicKey: []byte(tlsPubKey),
		},
		TTL: 1 * time.Hour,
	})
	require.NoError(t, err, "AuthenticateSSHUser failed")

	const devID = "deviceid1"
	const devTag = "devicetag1"
	const devCred = "devicecred1"

	advanceClock := func(d time.Duration) {
		if fc, ok := testServer.Clock().(*clockwork.FakeClock); ok {
			fc.Advance(d)
		}
	}

	tests := []struct {
		name           string
		x509PEM        []byte
		opts           *auth.AugmentUserCertificateOpts
		wantSSHCert    bool
		assertX509Cert func(t *testing.T, c *x509.Certificate)
		assertSSHCert  func(t *testing.T, c *ssh.Certificate)
	}{
		{
			name:    "device extensions",
			x509PEM: authResp.TLSCert,
			opts: &auth.AugmentUserCertificateOpts{
				SSHAuthorizedKey:         authResp.Cert,
				SSHKeySatisfiedChallenge: true,
				DeviceExtensions: &auth.DeviceExtensions{
					DeviceID:     devID,
					AssetTag:     devTag,
					CredentialID: devCred,
				},
			},
			wantSSHCert: true,
			assertX509Cert: func(t *testing.T, c *x509.Certificate) {
				id, err := tlsca.FromSubject(c.Subject, c.NotAfter)
				require.NoError(t, err, "FromSubject failed")
				assert.Equal(t, devID, id.DeviceExtensions.DeviceID, "DeviceID mismatch")
				assert.Equal(t, devTag, id.DeviceExtensions.AssetTag, "AssetTag mismatch")
				assert.Equal(t, devCred, id.DeviceExtensions.CredentialID, "CredentialID mismatch")
			},
			assertSSHCert: func(t *testing.T, c *ssh.Certificate) {
				assert.Equal(t, devID, c.Extensions[teleport.CertExtensionDeviceID], "DeviceID mismatch")
				assert.Equal(t, devTag, c.Extensions[teleport.CertExtensionDeviceAssetTag], "AssetTag mismatch")
				assert.Equal(t, devCred, c.Extensions[teleport.CertExtensionDeviceCredentialID], "CredentialID mismatch")
			},
		},
		{
			name:    "augment without SSH",
			x509PEM: authResp.TLSCert,
			opts: &auth.AugmentUserCertificateOpts{
				DeviceExtensions: &auth.DeviceExtensions{
					DeviceID:     devID,
					AssetTag:     devTag,
					CredentialID: devCred,
				},
			},
			// Nothing to assert, we are just looking for the absence of errors here.
			assertX509Cert: func(t *testing.T, c *x509.Certificate) {},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			xCert, identity := parseX509PEMAndIdentity(t, test.x509PEM)

			// Prepare ctx and authz.Context.
			ctx = authz.ContextWithUserCertificate(ctx, xCert)
			ctx = authz.ContextWithUser(ctx, authz.LocalUser{
				Username: username,
				Identity: *identity,
			})
			authCtx, err := testServer.APIConfig.Authorizer.Authorize(ctx)
			require.NoError(t, err, "Authorize failed")

			// Advance time before issuing new certs. This makes timestamp checks
			// effective under fake clocks.
			// 1m is enough to make tests fail if the timestamps aren't correct.
			advanceClock(1 * time.Minute)
			validAfter := testServer.Clock().Now().UTC().Add(-61 * time.Second)

			// Test!
			certs, err := authServer.AugmentContextUserCertificates(ctx, authCtx, test.opts)
			require.NoError(t, err, "AugmentContextUserCertificates failed")
			require.NotNil(t, certs, "AugmentContextUserCertificates returned nil certs")

			// Assert X.509 certificate.
			newXCert, _ := parseX509PEMAndIdentity(t, certs.TLS)
			test.assertX509Cert(t, newXCert)
			assert.True(t,
				validAfter.Before(newXCert.NotBefore),
				"got newXCert.NotBefore = %v, want > %v", newXCert.NotBefore, validAfter)
			assert.Equal(t, xCert.NotAfter, newXCert.NotAfter, "newXCert.NotAfter mismatch")

			// Assert SSH certificate.
			if test.wantSSHCert && len(certs.SSH) == 0 {
				t.Errorf("AugmentContextUserCertificates returned no SSH certificate")
			} else if !test.wantSSHCert {
				return
			}
			newSSHCert, err := sshutils.ParseCertificate(certs.SSH)
			require.NoError(t, err, "ParseCertificate failed")
			test.assertSSHCert(t, newSSHCert)
			assert.Less(t, uint64(validAfter.Unix()), newSSHCert.ValidAfter,
				"got newSSHCert.ValidAfter = %v, want > %v", newSSHCert.ValidAfter, validAfter.Unix())
			assert.Equal(t, uint64(xCert.NotAfter.Unix()), newSSHCert.ValidBefore, "newSSHCert.ValidBefore mismatch")
		})

		// Assert audit events.
		lastEvent := emitter.LastEvent()
		require.NotNil(t, lastEvent, "emitter.LastEvent() is nil")
		// Assert the code, that is a good enough proxy for other fields.
		assert.Equal(t, events.CertificateCreateEvent, lastEvent.GetType(), "lastEvent type mismatch")
		// Assert event DeviceExtensions.
		certEvent, ok := lastEvent.(*apievents.CertificateCreate)
		if assert.True(t, ok, "lastEvent is not an apievents.CertificateCreate, got %T", lastEvent) {
			got := certEvent.Identity.DeviceExtensions
			want := &apievents.DeviceExtensions{
				DeviceId:     test.opts.DeviceExtensions.DeviceID,
				AssetTag:     test.opts.DeviceExtensions.AssetTag,
				CredentialId: test.opts.DeviceExtensions.CredentialID,
			}
			if diff := gocmp.Diff(want, got); diff != "" {
				t.Errorf("certEvent.Identity.DeviceExtensions mismatch (-want +got)\n%s", diff)
			}
		}
	}
}

func TestServer_AugmentContextUserCertificates_errors(t *testing.T) {
	t.Parallel()

	testServer := newTestTLSServer(t)
	authServer := testServer.Auth()
	ctx := context.Background()

	const pass1 = "secret!!1!!!"
	const pass2 = "secret!!2!!!"
	const pass3 = "secret!!3!!!"

	// Prepare a few distinct users.
	user1, _, err := authtest.CreateUserAndRole(authServer, "llama", []string{"llama"}, nil)
	require.NoError(t, err, "CreateUserAndRole failed")
	require.NoError(t,
		authServer.UpsertPassword(user1.GetName(), []byte(pass1)),
		"UpsertPassword failed")

	user2, _, err := authtest.CreateUserAndRole(authServer, "alpaca", []string{"alpaca"}, nil)
	require.NoError(t, err, "CreateUserAndRole failed")
	require.NoError(t,
		authServer.UpsertPassword(user2.GetName(), []byte(pass2)),
		"UpsertPassword failed")

	user3, _, err := authtest.CreateUserAndRole(authServer, "camel", []string{"camel"}, nil)
	require.NoError(t, err, "CreateUserAndRole failed")
	require.NoError(t,
		authServer.UpsertPassword(user3.GetName(), []byte(pass3)),
		"UpsertPassword failed")

	// authenticate authenticates the specified user, creating a new key pair, a
	// new pair of certificates, and parsing all relevant responses.
	authenticate := func(t *testing.T, user, pass string) (tlsRaw, sshRaw []byte, xCert *x509.Certificate, sshCert *ssh.Certificate, identity *tlsca.Identity) {
		sshKey, tlsKey, err := cryptosuites.GenerateUserSSHAndTLSKey(ctx, cryptosuites.GetCurrentSuiteFromAuthPreference(authServer))
		require.NoError(t, err)

		sshPublicKey, err := ssh.NewPublicKey(sshKey.Public())
		require.NoError(t, err)
		tlsPublicKeyPEM, err := keys.MarshalPublicKey(tlsKey.Public())
		require.NoError(t, err)

		authResp, err := authServer.AuthenticateSSHUser(ctx, authclient.AuthenticateSSHRequest{
			AuthenticateUserRequest: authclient.AuthenticateUserRequest{
				Username: user,
				Pass: &authclient.PassCreds{
					Password: []byte(pass),
				},
				SSHPublicKey: ssh.MarshalAuthorizedKey(sshPublicKey),
				TLSPublicKey: tlsPublicKeyPEM,
			},
			TTL: 1 * time.Hour,
		})
		require.NoError(t, err, "AuthenticateSSHUser(%q) failed", user)

		xCert, identity = parseX509PEMAndIdentity(t, authResp.TLSCert)
		// parseX509PEMAndIdentity reports errors via t.

		sshCert, err = sshutils.ParseCertificate(authResp.Cert)
		require.NoError(t, err, "ParseCertificate failed")

		return authResp.TLSCert, authResp.Cert, xCert, sshCert, identity
	}

	// Authenticate.
	// user1 covers most of the tests.
	// user2 is mainly used to test mismatched certificates against user1.
	// user3 is used to test user locks.
	_, sshRaw1, xCert1, sshCert1, identity1 := authenticate(t, user1.GetName(), pass1)
	_, sshRaw2, xCert2, _, _ := authenticate(t, user2.GetName(), pass2)
	_, _, xCert3, _, identity3 := authenticate(t, user3.GetName(), pass3)

	// sshRaw11 is identical to sshRaw1, except it is backed by a different
	// key pair.
	_, sshRaw11, _, _, _ := authenticate(t, user1.GetName(), pass1)

	// wrongKey is used to represent an invalid/unknown CA.
	wrongKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err, "GenerateKey failed")

	// Build an invalid version of xCert1 (signed using wrongKey).
	userCA, err := authServer.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.UserCA,
		DomainName: testServer.ClusterName(),
	}, true /* loadKeys */)
	require.NoError(t, err, "GetCertAuthority failed")
	caXPEM := userCA.GetActiveKeys().TLS[0].Cert

	caXCert, _ := parseX509PEMAndIdentity(t, caXPEM)
	caXCert.PublicKey = wrongKey.Public()
	wrongXCert1DER, err := x509.CreateCertificate(rand.Reader, xCert1, caXCert, xCert1.PublicKey, wrongKey)
	require.NoError(t, err, "CreateCertificate failed")
	wrongXCert1, err := x509.ParseCertificate(wrongXCert1DER)
	require.NoError(t, err, "ParseCertificate failed")

	// Build an invalid version of sshCert1 (signed using wrongKey).
	sshSigner, err := ssh.NewSignerFromKey(wrongKey)
	require.NoError(t, err, "NewSignerFromKey failed")
	wrongSSHCert1, err := sshutils.ParseCertificate(sshRaw1)
	require.NoError(t, err, "ParseCertificate failed")
	require.NoError(t, wrongSSHCert1.SignCert(rand.Reader, sshSigner), "SignCert failed")
	wrongSSHRaw1 := ssh.MarshalAuthorizedKey(wrongSSHCert1)

	// Issue augmented certs for user1.
	// Used to test that re-issue of augmented certs is not allowed.
	ctxFromAuthorize := testServer.APIConfig.Authorizer.Authorize
	aCtx := authz.ContextWithUserCertificate(context.Background(), xCert1)
	aCtx = authz.ContextWithUser(aCtx, authz.LocalUser{
		Username: identity1.Username,
		Identity: *identity1,
	})
	aaCtx, err := ctxFromAuthorize(aCtx)
	require.NoError(t, err, "ctxFromAuthorize failed")
	augResp, err := authServer.AugmentContextUserCertificates(aCtx, aaCtx, &auth.AugmentUserCertificateOpts{
		SSHAuthorizedKey:         sshRaw1,
		SSHKeySatisfiedChallenge: true,
		DeviceExtensions: &auth.DeviceExtensions{
			DeviceID:     "device1",
			AssetTag:     "tag1",
			CredentialID: "credential1",
		},
	})
	require.NoError(t, err, "AugmentContextUserCertificates failed")
	augCert1, augIdentity1 := parseX509PEMAndIdentity(t, augResp.TLS)
	augSSHRaw1 := augResp.SSH

	// signAndMarshalSSH is used to create variations of SSH certificates signed
	// by the Teleport CA.
	signAndMarshalSSH := func(t *testing.T, c *ssh.Certificate) (sshRaw []byte) {
		signer, err := authServer.GetKeyStore().GetSSHSigner(ctx, userCA)
		require.NoError(t, err, "GetSSHSigner failed")

		err = c.SignCert(rand.Reader, signer)
		require.NoError(t, err, "SignCert failed")

		return ssh.MarshalAuthorizedKey(c)
	}

	baseOpts := &auth.AugmentUserCertificateOpts{
		DeviceExtensions: &auth.DeviceExtensions{
			DeviceID:     "deviceid1",
			AssetTag:     "devicetag1",
			CredentialID: "credentialid1",
		},
		SSHKeySatisfiedChallenge: true,
	}
	optsFromBase := func(_ *testing.T) *auth.AugmentUserCertificateOpts { return baseOpts }

	tests := []struct {
		name     string
		x509Cert *x509.Certificate
		identity *tlsca.Identity
		// createAuthCtx defaults to ctxFromAuthorize.
		createAuthCtx func(ctx context.Context) (*authz.Context, error)
		// createOpts defaults to optsFromBase.
		createOpts func(t *testing.T) *auth.AugmentUserCertificateOpts
		wantErr    string
	}{
		// Simple input validation errors.
		{
			name:          "authCtx nil",
			x509Cert:      xCert1,
			identity:      identity1,
			createAuthCtx: func(ctx context.Context) (*authz.Context, error) { return nil, nil },
			wantErr:       "authCtx",
		},
		{
			name:       "opts nil",
			x509Cert:   xCert1,
			identity:   identity1,
			createOpts: func(_ *testing.T) *auth.AugmentUserCertificateOpts { return nil },
			wantErr:    "opts",
		},
		{
			name:     "opts missing extensions",
			x509Cert: xCert1,
			identity: identity1,
			createOpts: func(_ *testing.T) *auth.AugmentUserCertificateOpts {
				cp := *baseOpts
				cp.DeviceExtensions = nil
				return &cp
			},
			wantErr: "at least one opts extension",
		},

		// DeviceExtensions.
		{
			name:     "opts.DeviceExtensions.DeviceID empty",
			x509Cert: xCert1,
			identity: identity1,
			createOpts: func(_ *testing.T) *auth.AugmentUserCertificateOpts {
				cp := *baseOpts
				cp.DeviceExtensions = &auth.DeviceExtensions{
					DeviceID:     "",
					AssetTag:     "asset1",
					CredentialID: "credential1",
				}
				return &cp
			},
			wantErr: "DeviceID",
		},
		{
			name:     "opts.DeviceExtensions.AssetTag empty",
			x509Cert: xCert1,
			identity: identity1,
			createOpts: func(_ *testing.T) *auth.AugmentUserCertificateOpts {
				cp := *baseOpts
				cp.DeviceExtensions = &auth.DeviceExtensions{
					DeviceID:     "id1",
					AssetTag:     "",
					CredentialID: "credential1",
				}
				return &cp
			},
			wantErr: "AssetTag",
		},
		{
			name:     "opts.DeviceExtensions.CredentialID empty",
			x509Cert: xCert1,
			identity: identity1,
			createOpts: func(_ *testing.T) *auth.AugmentUserCertificateOpts {
				cp := *baseOpts
				cp.DeviceExtensions = &auth.DeviceExtensions{
					DeviceID:     "id1",
					AssetTag:     "asset1",
					CredentialID: "",
				}
				return &cp
			},
			wantErr: "CredentialID",
		},

		// Identity and certificate mismatch scenarios.
		{
			name:     "x509/identity mismatch",
			x509Cert: xCert2, // should be xCert1
			identity: identity1,
			wantErr:  "x509 user mismatch",
		},
		{
			name:     "SSH challenge not satisfied and x509/SSH public key mismatch",
			x509Cert: xCert1,
			identity: identity1,
			createOpts: func(_ *testing.T) *auth.AugmentUserCertificateOpts {
				cp := *baseOpts
				cp.SSHAuthorizedKey = sshRaw11 // should be sshRaw1
				cp.SSHKeySatisfiedChallenge = false
				return &cp
			},
			wantErr: "public key mismatch",
		},
		{
			name:     "SSH/identity mismatch",
			x509Cert: xCert1,
			identity: identity1,
			createOpts: func(_ *testing.T) *auth.AugmentUserCertificateOpts {
				cp := *baseOpts
				cp.SSHAuthorizedKey = sshRaw2 // should be sshRaw1
				return &cp
			},
			wantErr: "SSH user mismatch",
		},
		{
			name:     "SSH/identity principals mismatch",
			x509Cert: xCert1,
			identity: identity1,
			createOpts: func(t *testing.T) *auth.AugmentUserCertificateOpts {
				changedPrincipals := *sshCert1
				changedPrincipals.ValidPrincipals = append(changedPrincipals.ValidPrincipals, "camel")
				sshRaw := signAndMarshalSSH(t, &changedPrincipals)

				cp := *baseOpts
				cp.SSHAuthorizedKey = sshRaw
				return &cp
			},
			wantErr: "principals mismatch",
		},
		{
			name:     "SSH cert type mismatch",
			x509Cert: xCert1,
			identity: identity1,
			createOpts: func(t *testing.T) *auth.AugmentUserCertificateOpts {
				changedType := *sshCert1
				changedType.CertType = ssh.HostCert // shouldn't happen!
				sshRaw := signAndMarshalSSH(t, &changedType)

				cp := *baseOpts
				cp.SSHAuthorizedKey = sshRaw
				return &cp
			},
			wantErr: "cert type mismatch",
		},

		// Invalid certificates.
		{
			name:     "x509 cert unknown authority",
			x509Cert: wrongXCert1, // signed by a different CA
			identity: identity1,
			wantErr:  "unknown authority",
		},
		{
			name:     "SSH cert unknown authority",
			x509Cert: xCert1,
			identity: identity1,
			createOpts: func(_ *testing.T) *auth.AugmentUserCertificateOpts {
				cp := *baseOpts
				cp.SSHAuthorizedKey = wrongSSHRaw1 // signed by a different CA
				return &cp
			},
			wantErr: "unknown authority",
		},
		{
			name:     "SSH cert expired",
			x509Cert: xCert1,
			identity: identity1,
			createOpts: func(t *testing.T) *auth.AugmentUserCertificateOpts {
				// Fake a 1h TTL, expired cert.
				now := testServer.Clock().Now()
				after := now.Add(-1 * time.Hour)
				before := now.Add(-1 * time.Minute)

				expiredCert := *sshCert1
				expiredCert.ValidAfter = uint64(after.Unix())
				expiredCert.ValidBefore = uint64(before.Unix())
				sshRaw := signAndMarshalSSH(t, &expiredCert)

				cp := *baseOpts
				cp.SSHAuthorizedKey = sshRaw
				return &cp
			},
			wantErr: "cert has expired",
		},

		// Certificates with existing extensions are not reissued.
		{
			name:     "x509 cert with device extensions not reissued",
			x509Cert: augCert1,     // already has extensions
			identity: augIdentity1, // already has extensions
			wantErr:  "extensions already present",
		},
		{
			name:     "SSH cert with device extensions not reissued",
			x509Cert: xCert1,
			identity: identity1,
			createOpts: func(_ *testing.T) *auth.AugmentUserCertificateOpts {
				cp := *baseOpts
				cp.SSHAuthorizedKey = augSSHRaw1 // already has extensions.
				return &cp
			},
			wantErr: "extensions already present",
		},

		// Locks.
		{
			name:     "locked user",
			x509Cert: xCert3,
			identity: identity3, // user3 locked below.
			createAuthCtx: func(ctx context.Context) (*authz.Context, error) {
				// Authorize user3...
				authCtx, err := ctxFromAuthorize(ctx)
				if err != nil {
					return nil, err
				}

				lockTarget := types.LockTarget{
					User: user3.GetName(),
				}
				watcher, err := authServer.SubscribeToLockTarget(ctx, lockTarget)
				if err != nil {
					return nil, err
				}
				defer watcher.Close()

				// ...and lock them right after.
				user3Lock, err := types.NewLock("user3-lock", types.LockSpecV2{
					Target:  lockTarget,
					Message: "user locked",
				})
				if err != nil {
					return nil, err
				}
				if err := authServer.UpsertLock(ctx, user3Lock); err != nil {
					return nil, err
				}

				// Wait for the lock to propagate.
				<-watcher.Events()
				return authCtx, nil
			},
			wantErr: "user locked",
		},
		{
			name:     "locked device",
			x509Cert: xCert1,
			identity: identity1, // device locked below.
			createOpts: func(t *testing.T) *auth.AugmentUserCertificateOpts {
				opts := &auth.AugmentUserCertificateOpts{
					DeviceExtensions: &auth.DeviceExtensions{
						DeviceID:     "bad-device-1",
						AssetTag:     "bad-device-tag",
						CredentialID: "bad-device-credential",
					},
				}

				// Create a target matching the device device.
				lockTarget := types.LockTarget{
					Device: opts.DeviceExtensions.DeviceID,
				}
				watcher, err := authServer.SubscribeToLockTarget(ctx, lockTarget)
				require.NoError(t, err, "Subscribe failed")
				defer watcher.Close()

				// Lock the device before returning opts.
				lock, err := types.NewLock("bad-device-lock", types.LockSpecV2{
					Target:  lockTarget,
					Message: "device locked",
				})
				require.NoError(t, err, "NewLock failed")
				require.NoError(t,
					authServer.UpsertLock(ctx, lock),
					"NewLock failed")

				// Wait for the lock to propagate.
				<-watcher.Events()
				return opts
			},
			wantErr: "device locked",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.createAuthCtx == nil {
				test.createAuthCtx = ctxFromAuthorize
			}
			if test.createOpts == nil {
				test.createOpts = optsFromBase
			}

			// Prepare ctx and authz.Context.
			ctx = authz.ContextWithUserCertificate(ctx, test.x509Cert)
			ctx = authz.ContextWithUser(ctx, authz.LocalUser{
				Username: test.identity.Username,
				Identity: *test.identity,
			})
			authCtx, err := test.createAuthCtx(ctx)
			require.NoError(t, err, "createAuthCtx failed")

			// Test!
			_, err = authServer.AugmentContextUserCertificates(ctx, authCtx, test.createOpts(t))
			assert.ErrorContains(t, err, test.wantErr, "AugmentContextUserCertificates error mismatch")
		})
	}
}

func TestServer_AugmentWebSessionCertificates(t *testing.T) {
	t.Parallel()

	testServer := newTestTLSServer(t)
	authServer := testServer.Auth()
	ctx := context.Background()

	userData := setupUserForAugmentWebSessionCertificatesTest(t, testServer)

	// Safe to reuse, user-independent.
	deviceExts := &auth.DeviceExtensions{
		DeviceID:     "my-device-id",
		AssetTag:     "my-device-asset-tag",
		CredentialID: "my-device-credential-id",
	}

	assertSSHCert := func(t *testing.T, sshCert []byte) {
		cert, err := sshutils.ParseCertificate(sshCert)
		require.NoError(t, err, "ParseCertificate")

		// Not empty is good enough here, other tests assert this deeply.
		assert.NotEmpty(t, cert.Extensions[teleport.CertExtensionDeviceID], "DeviceID empty")
		assert.NotEmpty(t, cert.Extensions[teleport.CertExtensionDeviceAssetTag], "AssetTag empty")
		assert.NotEmpty(t, cert.Extensions[teleport.CertExtensionDeviceCredentialID], "CredentialID empty")
	}

	assertX509Cert := func(t *testing.T, x509PEM []byte) {
		_, identity := parseX509PEMAndIdentity(t, x509PEM)

		// Not empty is good enough here, other tests assert this deeply.
		assert.NotEmpty(t, identity.DeviceExtensions.DeviceID, "DeviceID empty")
		assert.NotEmpty(t, identity.DeviceExtensions.AssetTag, "AssetTag empty")
		assert.NotEmpty(t, identity.DeviceExtensions.CredentialID, "CredentialID empty")
	}

	t.Run("ok", func(t *testing.T) {
		t.Parallel() // Get the errors suite going asap.

		opts := &auth.AugmentWebSessionCertificatesOpts{
			WebSessionID:     userData.webSessionID,
			User:             userData.user,
			DeviceExtensions: deviceExts,
		}
		err := authServer.AugmentWebSessionCertificates(ctx, opts)
		require.NoError(t, err, "AugmentWebSessionCertificates")

		// Verify WebSession certificates.
		webSession, err := authServer.WebSessions().Get(ctx, types.GetWebSessionRequest{
			User:      userData.user,
			SessionID: userData.webSessionID,
		})
		require.NoError(t, err, "WebSessions().Get() failed: %v", err)
		assertSSHCert(t, webSession.GetPub())
		assertX509Cert(t, webSession.GetTLSCert())
		assert.True(t, webSession.GetHasDeviceExtensions(), "webSesssion.GetHasDeviceExtensions() mismatch")

		// Scenario requires augmented certs to work.
		t.Run("cannot re-augment the same session", func(t *testing.T) {
			err := authServer.AugmentWebSessionCertificates(ctx, opts)
			const wantErr = "extensions already present"
			assert.ErrorContains(t, err, wantErr, "AugmentWebSessionCertificates error mismatch")
		})
	})

	user2Data := setupUserForAugmentWebSessionCertificatesTest(t, testServer)
	user2Opts := &auth.AugmentWebSessionCertificatesOpts{
		WebSessionID:     user2Data.webSessionID,
		User:             user2Data.user,
		DeviceExtensions: deviceExts,
	}

	t.Run("errors", func(t *testing.T) {
		tests := []struct {
			name      string
			opts      *auth.AugmentWebSessionCertificatesOpts
			wantErr   string
			assertErr func(error) bool // defaults to trace.IsBadParameter
		}{
			{
				name:    "opts nil",
				wantErr: "opts required",
			},
			{
				name: "opts.WebSessionID is empty",
				opts: func() *auth.AugmentWebSessionCertificatesOpts {
					opts := *user2Opts
					opts.WebSessionID = ""
					return &opts
				}(),
				wantErr: "WebSessionID required",
			},
			{
				name: "opts.User is empty",
				opts: func() *auth.AugmentWebSessionCertificatesOpts {
					opts := *user2Opts
					opts.User = ""
					return &opts
				}(),
				wantErr: "User required",
			},
			{
				name: "opts.DeviceExtensions nil",
				opts: func() *auth.AugmentWebSessionCertificatesOpts {
					opts := *user2Opts
					opts.DeviceExtensions = nil
					return &opts
				}(),
				wantErr: "at least one opts extension",
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				err := authServer.AugmentWebSessionCertificates(ctx, test.opts)
				assert.ErrorContains(t, err, test.wantErr, "AugmentWebSessionCertificates error mismatch")

				assertErr := test.assertErr
				if assertErr == nil {
					assertErr = trace.IsBadParameter
				}
				assert.True(t,
					assertErr(err),
					"AugmentWebSessionCertificates: assertErr failed: err=%v (%T)", err, trace.Unwrap(err))
			})
		}
	})
}

func TestServer_ExtendWebSession_deviceExtensions(t *testing.T) {
	t.Parallel()

	testServer := newTestTLSServer(t)
	authServer := testServer.Auth()
	ctx := context.Background()

	userData := setupUserForAugmentWebSessionCertificatesTest(t, testServer)

	deviceExts := &auth.DeviceExtensions{
		DeviceID:     "my-device-id",
		AssetTag:     "my-device-asset-tag",
		CredentialID: "my-device-credential-id",
	}

	// Augment the user's session, then later extend it.
	err := authServer.AugmentWebSessionCertificates(ctx, &auth.AugmentWebSessionCertificatesOpts{
		WebSessionID:     userData.webSessionID,
		User:             userData.user,
		DeviceExtensions: deviceExts,
	})
	require.NoError(t, err, "AugmentWebSessionCertificates() failed")

	// Retrieve augmented session and parse its identity.
	webSession, err := authServer.WebSessions().Get(ctx, types.GetWebSessionRequest{
		User:      userData.user,
		SessionID: userData.webSessionID,
	})
	require.NoError(t, err, "WebSessions().Get() failed")

	_, sessionIdentity := parseX509PEMAndIdentity(t, webSession.GetTLSCert())

	parseSSHDeviceExtensions := func(t *testing.T, sshCert []byte) tlsca.DeviceExtensions {
		cert, err := sshutils.ParseCertificate(sshCert)
		require.NoError(t, err, "ParseCertificate")

		return tlsca.DeviceExtensions{
			DeviceID:     cert.Extensions[teleport.CertExtensionDeviceID],
			AssetTag:     cert.Extensions[teleport.CertExtensionDeviceAssetTag],
			CredentialID: cert.Extensions[teleport.CertExtensionDeviceCredentialID],
		}
	}

	t.Run("ok", func(t *testing.T) {
		newSession, err := authServer.ExtendWebSession(ctx, authclient.WebSessionReq{
			User:          webSession.GetUser(),
			PrevSessionID: webSession.GetName(),
		}, *sessionIdentity)
		require.NoError(t, err, "ExtendWebSession() failed")

		// Assert extensions flag.
		assert.True(t, newSession.GetHasDeviceExtensions(), "newSession.GetHasDeviceExtensions() mismatch")

		// Assert TLS extensions.
		_, newIdentity := parseX509PEMAndIdentity(t, newSession.GetTLSCert())
		wantExts := tlsca.DeviceExtensions(*deviceExts)
		if diff := gocmp.Diff(wantExts, newIdentity.DeviceExtensions); diff != "" {
			t.Errorf("newSession.TLSCert DeviceExtensions mismatch (-want +got)\n%s", diff)
		}

		// Assert SSH extensions.
		if diff := gocmp.Diff(wantExts, parseSSHDeviceExtensions(t, newSession.GetPub())); diff != "" {
			t.Errorf("newSession.Pub DeviceExtensions mismatch (-want +got)\n%s", diff)
		}
	})
}

type augmentUserData struct {
	user         string
	pass         []byte
	pubKey       []byte // SSH "AuthorizedKey" format
	webSessionID string
}

func setupUserForAugmentWebSessionCertificatesTest(t *testing.T, testServer *authtest.TLSServer) *augmentUserData {
	authServer := testServer.Auth()
	ctx := context.Background()

	user := &augmentUserData{
		user: "llama_" + uuid.NewString(),
		pass: []byte("passwordforllamaA1!"),
	}

	// Create user and assign a password.
	_, _, err := authtest.CreateUserAndRole(authServer, user.user, []string{user.user}, nil /* allowRules */)
	require.NoError(t, err, "CreateUserAndRole")
	require.NoError(t,
		authServer.UpsertPassword(user.user, user.pass),
		"UpsertPassword",
	)

	user.pubKey = []byte(sshPubKey)

	// Prepare a WebSession to be augmented.
	authnReq := authclient.AuthenticateUserRequest{
		Username:     user.user,
		SSHPublicKey: user.pubKey,
		TLSPublicKey: []byte(tlsPubKey),
		Pass: &authclient.PassCreds{
			Password: user.pass,
		},
	}
	session, err := authServer.AuthenticateWebUser(ctx, authnReq)
	require.NoError(t, err, "AuthenticateWebUser")
	user.webSessionID = session.GetName()

	return user
}

func TestGenerateUserCertIPPinning(t *testing.T) {
	modulestest.SetTestModules(t, modulestest.Modules{TestBuildType: modules.BuildEnterprise})

	s := newAuthSuite(t)

	ctx := context.Background()

	const pinnedUser = "pinnedUser"
	const unpinnedUser = "unpinnedUser"
	pass := []byte("abcdef123456")

	// Create the user without IP pinning
	_, _, err := authtest.CreateUserAndRole(s.a, unpinnedUser, []string{unpinnedUser}, nil)
	require.NoError(t, err)
	err = s.a.UpsertPassword(unpinnedUser, pass)
	require.NoError(t, err)

	// Create the user with IP pinning enabled
	_, pinnedRole, err := authtest.CreateUserAndRole(s.a, pinnedUser, []string{pinnedUser}, nil)
	require.NoError(t, err)
	err = s.a.UpsertPassword(pinnedUser, pass)
	require.NoError(t, err)
	options := pinnedRole.GetOptions()
	options.PinSourceIP = true
	pinnedRole.SetOptions(options)

	_, err = s.a.UpsertRole(ctx, pinnedRole)
	require.NoError(t, err)

	findTLSLoginIP := func(names []pkix.AttributeTypeAndValue) any {
		for _, name := range names {
			if name.Type.Equal(tlsca.LoginIPASN1ExtensionOID) {
				return name.Value
			}
		}
		return nil
	}

	findTLSPinnedIP := func(names []pkix.AttributeTypeAndValue) any {
		for _, name := range names {
			if name.Type.Equal(tlsca.PinnedIPASN1ExtensionOID) {
				return name.Value
			}
		}
		return nil
	}

	testCases := []struct {
		desc       string
		user       string
		loginIP    string
		wantPinned bool
	}{
		{desc: "no client ip, not pinned", user: unpinnedUser, loginIP: "", wantPinned: false},
		{desc: "client ip, not  pinned", user: unpinnedUser, loginIP: "1.2.3.4", wantPinned: false},
		{desc: "client ip, pinned", user: pinnedUser, loginIP: "1.2.3.4", wantPinned: true},
		{desc: "no client ip, pinned", user: pinnedUser, loginIP: "", wantPinned: true},
	}

	baseAuthRequest := authclient.AuthenticateSSHRequest{
		AuthenticateUserRequest: authclient.AuthenticateUserRequest{
			Pass:         &authclient.PassCreds{Password: pass},
			SSHPublicKey: []byte(sshPubKey),
			TLSPublicKey: []byte(tlsPubKey),
		},
		TTL:            time.Hour,
		RouteToCluster: s.clusterName.GetClusterName(),
	}

	for _, tt := range testCases {
		t.Run(tt.desc, func(t *testing.T) {
			authRequest := baseAuthRequest
			authRequest.AuthenticateUserRequest.Username = tt.user
			if tt.loginIP != "" {
				authRequest.ClientMetadata = &authclient.ForwardedClientMetadata{
					RemoteAddr: tt.loginIP,
				}
			}
			resp, err := s.a.AuthenticateSSHUser(ctx, authRequest)
			if tt.wantPinned && tt.loginIP == "" {
				require.ErrorContains(t, err, "source IP pinning is enabled but client IP is unknown")
				return
			}
			require.NoError(t, err)
			require.Equal(t, resp.Username, tt.user)

			sshCert, err := sshutils.ParseCertificate(resp.Cert)
			require.NoError(t, err)

			tlsCert, err := tlsca.ParseCertificatePEM(resp.TLSCert)
			require.NoError(t, err)

			tlsLoginIP := findTLSLoginIP(tlsCert.Subject.Names)
			tlsPinnedIP := findTLSPinnedIP(tlsCert.Subject.Names)
			sshLoginIP, sshLoginIPOK := sshCert.Extensions[teleport.CertExtensionLoginIP]
			sshCriticalAddress, sshCriticalAddressOK := sshCert.CriticalOptions["source-address"]

			if tt.loginIP != "" {
				require.NotNil(t, tlsLoginIP, "client IP not found on TLS cert")
				require.Equal(t, tlsLoginIP, tt.loginIP, "TLS LoginIP mismatch")

				require.True(t, sshLoginIPOK, "SSH LoginIP extension not present")
				require.Equal(t, tt.loginIP, sshLoginIP, "SSH LoginIP mismatch")
			} else {
				require.Nil(t, tlsLoginIP, "client IP unexpectedly found on TLS cert")

				require.False(t, sshLoginIPOK, "client IP unexpectedly found on SSH cert")
			}

			if tt.wantPinned {
				require.NotNil(t, tlsPinnedIP, "pinned IP not found on TLS cert")
				require.Equal(t, tt.loginIP, tlsPinnedIP, "pinned IP on TLS cert mismatch")

				require.True(t, sshCriticalAddressOK, "source address not found on SSH cert")
				require.Equal(t, tt.loginIP+"/32", sshCriticalAddress, "SSH source address mismatch")
			} else {
				require.Nil(t, tlsPinnedIP, "pinned IP unexpectedly found on TLS cert")

				require.False(t, sshCriticalAddressOK, "source address unexpectedly found on SSH cert")
			}
		})
	}
}

func parseX509PEMAndIdentity(t *testing.T, rawPEM []byte) (*x509.Certificate, *tlsca.Identity) {
	b, _ := pem.Decode(rawPEM)
	require.NotNil(t, b, "Decode failed")

	cert, err := x509.ParseCertificate(b.Bytes)
	require.NoError(t, err, "ParseCertificate failed: %v", err)

	identity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
	require.NoError(t, err, "FromSubject failed: %v", err)

	return cert, identity
}

func contextWithGRPCClientUserAgent(ctx context.Context, userAgent string) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		md = make(metadata.MD)
	}
	md["user-agent"] = append(md["user-agent"], userAgent)
	return metadata.NewIncomingContext(ctx, md)
}

func TestGenerateUserCertWithCertExtension(t *testing.T) {
	t.Parallel()
	ctx := contextWithGRPCClientUserAgent(context.Background(), "test-user-agent/1.0")
	p, err := newTestPack(ctx, t.TempDir())
	require.NoError(t, err)

	user, role, err := authtest.CreateUserAndRole(p.a, "test-user", []string{}, nil)
	require.NoError(t, err)

	extension := types.CertExtension{
		Name:  "abc",
		Value: "cde",
		Type:  types.CertExtensionType_SSH,
		Mode:  types.CertExtensionMode_EXTENSION,
	}
	options := role.GetOptions()
	options.CertExtensions = []*types.CertExtension{&extension}
	role.SetOptions(options)
	_, err = p.a.UpsertRole(ctx, role)
	require.NoError(t, err)

	_, sshPubKey, _, tlsPubKey := newSSHAndTLSKeyPairs(t)

	sshCert, _, err := p.a.GenerateUserTestCertsWithContext(ctx, auth.GenerateUserTestCertsRequest{
		SSHPubKey: sshPubKey,
		TLSPubKey: tlsPubKey,
		Username:  user.GetName(),
	})
	require.NoError(t, err)

	key, err := sshutils.ParseCertificate(sshCert)
	require.NoError(t, err)

	val, ok := key.Extensions[extension.Name]
	require.True(t, ok)
	require.Equal(t, extension.Value, val)

	// Validate audit event.
	lastEvent := p.mockEmitter.LastEvent()
	require.IsType(t, &apievents.CertificateCreate{}, lastEvent)
	require.Empty(t, gocmp.Diff(
		&apievents.CertificateCreate{
			Metadata: apievents.Metadata{
				Type: events.CertificateCreateEvent,
				Code: events.CertificateCreateCode,
			},
			Identity: &apievents.Identity{
				User:             "test-user",
				Roles:            []string{"user:test-user"},
				RouteToCluster:   "test.localhost",
				TeleportCluster:  "test.localhost",
				PrivateKeyPolicy: "none",
			},
			CertificateType: events.CertificateTypeUser,
			ClientMetadata: apievents.ClientMetadata{
				UserAgent: "test-user-agent/1.0",
			},
			CertificateAuthority: &apievents.CertificateAuthority{
				Type:   "user",
				Domain: "test.localhost",
			},
		},
		lastEvent.(*apievents.CertificateCreate),
		cmpopts.IgnoreFields(apievents.Identity{}, "Logins", "Expires"),
		cmpopts.IgnoreFields(apievents.CertificateAuthority{}, "SubjectKeyID"),
	))
}

func TestGenerateOpenSSHCert(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	p, err := newTestPack(ctx, t.TempDir())
	require.NoError(t, err)

	// create keypair and sign with OpenSSH CA
	logins := []string{"login1", "login2"}
	u, r, err := authtest.CreateUserAndRole(p.a, "test-user", logins, nil)
	require.NoError(t, err)

	user, ok := u.(*types.UserV2)
	require.True(t, ok)
	role, ok := r.(*types.RoleV6)
	require.True(t, ok)

	priv, err := cryptosuites.GeneratePrivateKeyWithAlgorithm(cryptosuites.Ed25519)
	require.NoError(t, err)

	reply, err := p.a.GenerateOpenSSHCert(ctx, &proto.OpenSSHCertRequest{
		User:      user,
		Roles:     []*types.RoleV6{role},
		PublicKey: priv.MarshalSSHPublicKey(),
		TTL:       proto.Duration(time.Hour),
		Cluster:   p.clusterName.GetClusterName(),
	})
	require.NoError(t, err)

	// verify that returned cert is signed with OpenSSH CA
	signedCert, err := sshutils.ParseCertificate(reply.Cert)
	require.NoError(t, err)

	ca, err := p.a.GetCertAuthority(
		ctx,
		types.CertAuthID{
			Type:       types.OpenSSHCA,
			DomainName: p.clusterName.GetClusterName(),
		},
		false,
	)
	require.NoError(t, err)

	keys := ca.GetActiveKeys().SSH
	require.NotEmpty(t, keys)
	caPubkey, _, _, _, err := ssh.ParseAuthorizedKey(keys[0].PublicKey)
	require.NoError(t, err)

	require.Equal(t, caPubkey.Marshal(), signedCert.SignatureKey.Marshal())

	// verify that user's logins are present in cert
	logins = append(logins, teleport.SSHSessionJoinPrincipal)
	require.Equal(t, logins, signedCert.ValidPrincipals)
}

func TestGenerateUserCertWithLocks(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	p, err := newTestPack(ctx, t.TempDir())
	require.NoError(t, err)

	user, _, err := authtest.CreateUserAndRole(p.a, "test-user", []string{}, nil)
	require.NoError(t, err)

	const mfaID = "test-mfa-id"
	const requestID = "test-access-request"
	const deviceID = "deviceid1"

	_, sshPubKey, _, tlsPubKey := newSSHAndTLSKeyPairs(t)

	certReq := auth.GenerateUserTestCertsRequest{
		SSHPubKey:      sshPubKey,
		TLSPubKey:      tlsPubKey,
		Username:       user.GetName(),
		ActiveRequests: []string{requestID},
		MFAVerified:    mfaID,
		DeviceExtensions: auth.DeviceExtensions{
			DeviceID:     deviceID,
			AssetTag:     "assettag1",
			CredentialID: "credentialid1",
		},
	}

	_, _, err = p.a.GenerateUserTestCertsWithContext(ctx, certReq)
	require.NoError(t, err)

	testTargets := append(
		[]types.LockTarget{
			{User: user.GetName()},
			{MFADevice: mfaID},
			{AccessRequest: requestID},
			{Device: deviceID},
		},
		services.RolesToLockTargets(user.GetRoles())...,
	)
	for _, target := range testTargets {
		t.Run(fmt.Sprintf("lock targeting %v", target), func(t *testing.T) {
			lockWatch, err := p.a.SubscribeToLockTarget(ctx, target)
			require.NoError(t, err)
			defer lockWatch.Close()
			lock, err := types.NewLock("test-lock", types.LockSpecV2{Target: target})
			require.NoError(t, err)

			require.NoError(t, p.a.UpsertLock(ctx, lock))
			select {
			case event := <-lockWatch.Events():
				require.Equal(t, types.OpPut, event.Type)
				require.Empty(t, resourceDiff(event.Resource, lock))
			case <-lockWatch.Done():
				t.Fatal("Watcher has unexpectedly exited.")
			case <-time.After(2 * time.Second):
				t.Fatal("Timeout waiting for lock update.")
			}

			_, _, err = p.a.GenerateUserTestCertsWithContext(ctx, certReq)
			require.Error(t, err)
			require.EqualError(t, err, services.LockInForceAccessDenied(lock).Error())
		})
	}
}

func TestGenerateHostCertWithLocks(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	p, err := newTestPack(ctx, t.TempDir())
	require.NoError(t, err)

	hostID := uuid.New().String()
	keygen := testauthority.New()
	_, pub, err := keygen.GenerateKeyPair()
	require.NoError(t, err)
	_, err = p.a.GenerateHostCert(ctx, pub, hostID, "test-node", []string{},
		p.clusterName.GetClusterName(), types.RoleNode, time.Minute)
	require.NoError(t, err)

	target := types.LockTarget{ServerID: hostID}
	lockWatch, err := p.a.SubscribeToLockTarget(ctx, target)
	require.NoError(t, err)
	defer lockWatch.Close()
	lock, err := types.NewLock("test-lock", types.LockSpecV2{Target: target})
	require.NoError(t, err)

	require.NoError(t, p.a.UpsertLock(ctx, lock))
	select {
	case event := <-lockWatch.Events():
		require.Equal(t, types.OpPut, event.Type)
		require.Empty(t, resourceDiff(event.Resource, lock))
	case <-lockWatch.Done():
		t.Fatal("Watcher has unexpectedly exited.")
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for lock update.")
	}
	_, err = p.a.GenerateHostCert(ctx, pub, hostID, "test-node", []string{}, p.clusterName.GetClusterName(), types.RoleNode, time.Minute)
	require.Error(t, err)
	require.EqualError(t, err, services.LockInForceAccessDenied(lock).Error())

	// Locks targeting server IDs should apply to other system roles.
	_, err = p.a.GenerateHostCert(ctx, pub, hostID, "test-proxy", []string{}, p.clusterName.GetClusterName(), types.RoleProxy, time.Minute)
	require.Error(t, err)
}

func TestGenerateUserCertWithUserLoginState(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	p, err := newTestPack(ctx, t.TempDir())
	require.NoError(t, err)

	user, role, err := authtest.CreateUserAndRole(p.a, "test-user", []string{}, nil)
	require.NoError(t, err)

	_, sshPubKey, _, tlsPubKey := newSSHAndTLSKeyPairs(t)

	// Generate cert with no user login state.
	certReq := auth.GenerateUserTestCertsRequest{
		SSHPubKey: sshPubKey,
		TLSPubKey: tlsPubKey,
		Username:  user.GetName(),
	}

	rawSSHCert, _, err := p.a.GenerateUserTestCertsWithContext(ctx, certReq)
	require.NoError(t, err)

	sshCert, err := sshutils.ParseCertificate(rawSSHCert)
	require.NoError(t, err)

	sshIdent, err := sshca.DecodeIdentity(sshCert)
	require.NoError(t, err)

	require.Equal(t, []string{role.GetName()}, sshIdent.Roles)

	traits := wrappers.Traits{}
	err = wrappers.UnmarshalTraits([]byte(sshCert.Extensions[teleport.CertExtensionTeleportTraits]), &traits)
	require.NoError(t, err)
	require.Empty(t, traits)

	uls, err := userloginstate.New(
		header.Metadata{
			Name: user.GetName(),
		},
		userloginstate.Spec{
			Roles: []string{
				role.GetName(), // We'll try to grant a duplicate role, which should be deduplicated.
				"uls-role1",
				"uls-role2",
			},
			Traits: trait.Traits{
				"uls-trait1": []string{"value1", "value2"},
				"uls-trait2": []string{"value3", "value4"},
			},
		},
	)
	require.NoError(t, err)
	_, err = p.a.UpsertUserLoginState(ctx, uls)
	require.NoError(t, err)

	ulsRole1, err := types.NewRole("uls-role1", types.RoleSpecV6{})
	require.NoError(t, err)
	ulsRole2, err := types.NewRole("uls-role2", types.RoleSpecV6{})
	require.NoError(t, err)

	_, err = p.a.UpsertRole(ctx, ulsRole1)
	require.NoError(t, err)
	_, err = p.a.UpsertRole(ctx, ulsRole2)
	require.NoError(t, err)

	certReq = auth.GenerateUserTestCertsRequest{
		SSHPubKey: sshPubKey,
		TLSPubKey: tlsPubKey,
		Username:  user.GetName(),
	}

	rawSSHCert, _, err = p.a.GenerateUserTestCertsWithContext(ctx, certReq)
	require.NoError(t, err)

	sshCert, err = sshutils.ParseCertificate(rawSSHCert)
	require.NoError(t, err)

	sshIdent, err = sshca.DecodeIdentity(sshCert)
	require.NoError(t, err)
	require.Equal(t, []string{role.GetName(), "uls-role1", "uls-role2"}, sshIdent.Roles)

	traits = wrappers.Traits{}
	err = wrappers.UnmarshalTraits([]byte(sshCert.Extensions[teleport.CertExtensionTeleportTraits]), &traits)
	require.NoError(t, err)
	require.Equal(t, map[string][]string{
		"uls-trait1": {"value1", "value2"},
		"uls-trait2": {"value3", "value4"},
	}, map[string][]string(traits))
}

func TestGenerateUserCertWithHardwareKeySupport(t *testing.T) {
	ctx := context.Background()
	p, err := newTestPack(ctx, t.TempDir())
	require.NoError(t, err)

	user, _, err := authtest.CreateUserAndRole(p.a, "test-user", []string{}, nil)
	require.NoError(t, err)
	user.SetTraits(map[string][]string{
		// add in other random serial numbers to test comparison logic.
		"hardware_key_serial_numbers": {"other1", "other2,12345678,other3"},
		// custom trait name
		"known_yubikeys": {"13572468"},
	})
	_, err = p.a.UpdateUser(ctx, user)
	require.NoError(t, err)

	_, sshPubKey, _, tlsPubKey := newSSHAndTLSKeyPairs(t)

	certReq := auth.GenerateUserTestCertsRequest{
		SSHPubKey: sshPubKey,
		TLSPubKey: tlsPubKey,
		Username:  user.GetName(),
	}

	for _, tt := range []struct {
		name                string
		cap                 types.AuthPreferenceSpecV2
		mockAttestationData *keys.AttestationData
		assertErr           require.ErrorAssertionFunc
	}{
		{
			name: "private key policy satified",
			cap: types.AuthPreferenceSpecV2{
				RequireMFAType: types.RequireMFAType_HARDWARE_KEY_TOUCH,
			},
			mockAttestationData: &keys.AttestationData{
				PrivateKeyPolicy: keys.PrivateKeyPolicyHardwareKeyTouch,
			},
			assertErr: require.NoError,
		}, {
			name: "no attestation data",
			cap: types.AuthPreferenceSpecV2{
				RequireMFAType: types.RequireMFAType_HARDWARE_KEY_TOUCH,
			},
			assertErr: func(t require.TestingT, err error, i ...any) {
				require.Error(t, err, "expected private key policy error but got %v", err)
				require.True(t, keys.IsPrivateKeyPolicyError(err), "expected private key policy error but got %v", err)
			},
		}, {
			name: "private key policy not satisfied",
			cap: types.AuthPreferenceSpecV2{
				RequireMFAType: types.RequireMFAType_HARDWARE_KEY_TOUCH,
			},
			mockAttestationData: &keys.AttestationData{
				PrivateKeyPolicy: keys.PrivateKeyPolicyHardwareKey,
				SerialNumber:     12345678,
			},
			assertErr: func(t require.TestingT, err error, i ...any) {
				require.Error(t, err, "expected private key policy error but got %v", err)
				require.True(t, keys.IsPrivateKeyPolicyError(err), "expected private key policy error but got %v", err)
			},
		}, {
			name: "known hardware key",
			cap: types.AuthPreferenceSpecV2{
				RequireMFAType: types.RequireMFAType_HARDWARE_KEY_TOUCH,
				HardwareKey: &types.HardwareKey{
					SerialNumberValidation: &types.HardwareKeySerialNumberValidation{
						Enabled: true,
					},
				},
			},
			mockAttestationData: &keys.AttestationData{
				PrivateKeyPolicy: keys.PrivateKeyPolicyHardwareKeyTouch,
				SerialNumber:     12345678,
			},
			assertErr: require.NoError,
		}, {
			name: "partial serial number is unknown",
			cap: types.AuthPreferenceSpecV2{
				RequireMFAType: types.RequireMFAType_HARDWARE_KEY_TOUCH,
				HardwareKey: &types.HardwareKey{
					SerialNumberValidation: &types.HardwareKeySerialNumberValidation{
						Enabled: true,
					},
				},
			},
			mockAttestationData: &keys.AttestationData{
				PrivateKeyPolicy: keys.PrivateKeyPolicyHardwareKeyTouch,
				SerialNumber:     1234,
			},
			assertErr: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsBadParameter(err), "expected bad parameter error but got %v", err)
				require.ErrorContains(t, err, "unknown hardware key")
			},
		}, {
			name: "known hardware key custom trait name",
			cap: types.AuthPreferenceSpecV2{
				RequireMFAType: types.RequireMFAType_HARDWARE_KEY_TOUCH,
				HardwareKey: &types.HardwareKey{
					SerialNumberValidation: &types.HardwareKeySerialNumberValidation{
						Enabled:               true,
						SerialNumberTraitName: "known_yubikeys",
					},
				},
			},
			mockAttestationData: &keys.AttestationData{
				PrivateKeyPolicy: keys.PrivateKeyPolicyHardwareKeyTouch,
				SerialNumber:     13572468,
			},
			assertErr: require.NoError,
		}, {
			name: "unknown hardware key",
			cap: types.AuthPreferenceSpecV2{
				RequireMFAType: types.RequireMFAType_HARDWARE_KEY_TOUCH,
				HardwareKey: &types.HardwareKey{
					SerialNumberValidation: &types.HardwareKeySerialNumberValidation{
						Enabled: true,
					},
				},
			},
			mockAttestationData: &keys.AttestationData{
				PrivateKeyPolicy: keys.PrivateKeyPolicyHardwareKeyTouch,
				SerialNumber:     87654321,
			},
			assertErr: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsBadParameter(err), "expected bad parameter error but got %v", err)
				require.ErrorContains(t, err, "unknown hardware key")
			},
		}, {
			name: "no known hardware keys",
			cap: types.AuthPreferenceSpecV2{
				RequireMFAType: types.RequireMFAType_HARDWARE_KEY_TOUCH,
				HardwareKey: &types.HardwareKey{
					SerialNumberValidation: &types.HardwareKeySerialNumberValidation{
						Enabled:               true,
						SerialNumberTraitName: "none",
					},
				},
			},
			mockAttestationData: &keys.AttestationData{
				PrivateKeyPolicy: keys.PrivateKeyPolicyHardwareKeyTouch,
				SerialNumber:     12345678,
			},
			assertErr: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsBadParameter(err), "expected bad parameter error but got %v", err)
				require.ErrorContains(t, err, "no known hardware keys")
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			modulestest.SetTestModules(t, modulestest.Modules{
				MockAttestationData: tt.mockAttestationData,
			})

			authPref, err := types.NewAuthPreference(tt.cap)
			require.NoError(t, err)
			_, err = p.a.UpsertAuthPreference(ctx, authPref)
			require.NoError(t, err)

			_, _, err = p.a.GenerateUserTestCertsWithContext(ctx, certReq)
			tt.assertErr(t, err)
		})
	}
}

func TestGenerateKubernetesUserCert(t *testing.T) {
	ctx := context.Background()
	p, err := newTestPack(ctx, t.TempDir())
	require.NoError(t, err)

	user, _, err := authtest.CreateUserAndRole(p.a, "test-user", []string{}, nil)
	require.NoError(t, err)

	rc, err := types.NewRemoteCluster("leaf")
	require.NoError(t, err)
	_, err = p.a.CreateRemoteCluster(ctx, rc)
	require.NoError(t, err)

	kubeCluster, err := types.NewKubernetesClusterV3(types.Metadata{Name: "kube-cluster"}, types.KubernetesClusterSpecV3{})
	require.NoError(t, err)
	kubeServer, err := types.NewKubernetesServerV3FromCluster(kubeCluster, "foo", "1")
	require.NoError(t, err)
	_, err = p.a.UpsertKubernetesServer(ctx, kubeServer)
	require.NoError(t, err)

	// Wait for cache propagation of the kubernetes resources before proceeding with the tests.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		gotNames := map[string]struct{}{}
		for ks, err := range p.a.UnifiedResourceCache.KubernetesServers(ctx, services.UnifiedResourcesIterateParams{}) {
			if !assert.NoError(t, err) {
				return
			}

			gotNames[ks.GetCluster().GetName()] = struct{}{}
		}
		assert.Contains(t, gotNames, kubeCluster.GetName(), "missing kube cluster")
	}, 15*time.Second, 100*time.Millisecond)

	_, sshPubKey, _, tlsPubKey := newSSHAndTLSKeyPairs(t)

	for _, tt := range []struct {
		name              string
		teleportCluster   string
		kubernetesCluster string
		assertErr         require.ErrorAssertionFunc
	}{
		{
			name:              "leaf clusters not validated",
			teleportCluster:   "leaf",
			kubernetesCluster: "foo",
			assertErr:         require.NoError,
		},
		{
			name:              "kubernetes cluster not registered",
			teleportCluster:   p.clusterName.GetClusterName(),
			kubernetesCluster: "foo",
			assertErr:         require.Error,
		},
		{
			name:              "kubernetes cluster registered",
			teleportCluster:   p.clusterName.GetClusterName(),
			kubernetesCluster: kubeCluster.GetName(),
			assertErr:         require.NoError,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			certReq := auth.GenerateUserTestCertsRequest{
				SSHPubKey:         sshPubKey,
				TLSPubKey:         tlsPubKey,
				Username:          user.GetName(),
				RouteToCluster:    tt.teleportCluster,
				KubernetesCluster: tt.kubernetesCluster,
			}

			_, _, err = p.a.GenerateUserTestCertsWithContext(ctx, certReq)
			tt.assertErr(t, err)
		})
	}
}

func TestNewWebSession(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	p, err := newTestPack(ctx, t.TempDir())
	require.NoError(t, err)

	// Set a web idle timeout.
	duration := time.Duration(5) * time.Minute
	cfg := types.DefaultClusterNetworkingConfig()
	cfg.SetWebIdleTimeout(duration)
	_, err = p.a.UpsertClusterNetworkingConfig(ctx, cfg)
	require.NoError(t, err)

	// Create a user.
	user, _, err := authtest.CreateUserAndRole(p.a, "test-user", []string{"test-role"}, nil)
	require.NoError(t, err)

	// Create a new web session.
	req := auth.NewWebSessionRequest{
		User:       user.GetName(),
		Roles:      user.GetRoles(),
		Traits:     user.GetTraits(),
		LoginTime:  p.a.GetClock().Now().UTC(),
		SessionTTL: apidefaults.CertDuration,
	}
	bearerTokenTTL := min(req.SessionTTL, defaults.BearerTokenTTL)

	ws, _, err := p.a.NewWebSession(ctx, req, nil /* opts */)
	require.NoError(t, err)
	require.Equal(t, user.GetName(), ws.GetUser())
	require.Equal(t, duration, ws.GetIdleTimeout())
	require.Equal(t, req.LoginTime, ws.GetLoginTime())
	require.Equal(t, req.LoginTime.UTC().Add(req.SessionTTL), ws.GetExpiryTime())
	require.Equal(t, req.LoginTime.UTC().Add(bearerTokenTTL), ws.GetBearerTokenExpiryTime())
	require.NotEmpty(t, ws.GetBearerToken())
	require.NotEmpty(t, ws.GetSSHPriv())
	require.NotEmpty(t, ws.GetTLSPriv())
	require.NotEmpty(t, ws.GetPub())
	require.NotEmpty(t, ws.GetTLSCert())
}

func TestDeleteMFADeviceSync(t *testing.T) {
	t.Parallel()

	testServer := newTestTLSServer(t)
	authServer := testServer.Auth()
	mockEmitter := &eventstest.MockRecorderEmitter{}
	authServer.SetEmitter(mockEmitter)

	ctx := context.Background()

	const username = "llama@goteleport.com"
	_, _, err := authtest.CreateUserAndRole(authServer, username, []string{username}, nil /* allowRules */)
	require.NoError(t, err)

	authPreference, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOptional, // "optional" lets all user devices be deleted.
		Webauthn: &types.Webauthn{
			RPID: "localhost",
		},
	})
	require.NoError(t, err)
	_, err = authServer.UpsertAuthPreference(ctx, authPreference)
	require.NoError(t, err)

	userClient, err := testServer.NewClient(authtest.TestUser(username))
	require.NoError(t, err)

	// webDev1 is used as the authenticator for various checks.
	webDev1, err := authtest.RegisterTestDevice(ctx, userClient, "web1", proto.DeviceType_DEVICE_TYPE_WEBAUTHN, nil /* authenticator */)
	require.NoError(t, err, "RegisterTestDevice(web1)")

	// Insert devices for deletion.
	deviceOpts := []authtest.TestDeviceOpt{authtest.WithTestDeviceClock(testServer.Clock())}
	registerDevice := func(t *testing.T, deviceName string, deviceType proto.DeviceType) *authtest.Device {
		t.Helper()
		testDev, err := authtest.RegisterTestDevice(
			ctx, userClient, deviceName, deviceType, webDev1 /* authenticator */, deviceOpts...)
		require.NoError(t, err, "RegisterTestDevice(%v)", deviceName)
		return testDev
	}
	deleteWeb1 := registerDevice(t, "delete-web1", proto.DeviceType_DEVICE_TYPE_WEBAUTHN)
	deleteWeb2 := registerDevice(t, "delete-web2", proto.DeviceType_DEVICE_TYPE_WEBAUTHN)
	deleteTOTP1 := registerDevice(t, "delete-totp1", proto.DeviceType_DEVICE_TYPE_TOTP)
	deleteTOTP2 := registerDevice(t, "delete-totp2", proto.DeviceType_DEVICE_TYPE_TOTP)

	deleteReqUsingToken := func(tokenReq authclient.CreateUserTokenRequest) func(t *testing.T) *proto.DeleteMFADeviceSyncRequest {
		return func(t *testing.T) *proto.DeleteMFADeviceSyncRequest {
			token, err := authServer.NewUserToken(tokenReq)
			require.NoError(t, err, "newUserToken")

			_, err = authServer.CreateUserToken(ctx, token)
			require.NoError(t, err, "CreateUserToken")

			return &proto.DeleteMFADeviceSyncRequest{
				TokenID: token.GetName(),
			}
		}
	}

	deleteReqUsingChallenge := func(authenticator *authtest.Device) func(t *testing.T) *proto.DeleteMFADeviceSyncRequest {
		return func(t *testing.T) *proto.DeleteMFADeviceSyncRequest {
			authnChal, err := userClient.CreateAuthenticateChallenge(ctx, &proto.CreateAuthenticateChallengeRequest{
				Request: &proto.CreateAuthenticateChallengeRequest_ContextUser{
					ContextUser: &proto.ContextUser{},
				},
				ChallengeExtensions: &mfav1.ChallengeExtensions{
					Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_MANAGE_DEVICES,
				},
			})
			require.NoError(t, err, "CreateAuthenticateChallenge")

			authnSolved, err := authenticator.SolveAuthn(authnChal)
			require.NoError(t, err, "SolveAuthn")

			return &proto.DeleteMFADeviceSyncRequest{
				ExistingMFAResponse: authnSolved,
			}
		}
	}

	tests := []struct {
		name            string
		createDeleteReq func(t *testing.T) *proto.DeleteMFADeviceSyncRequest
		deviceToDelete  string
	}{
		{
			name: "recovery approved token",
			createDeleteReq: deleteReqUsingToken(authclient.CreateUserTokenRequest{
				Name: username,
				TTL:  5 * time.Minute,
				Type: authclient.UserTokenTypeRecoveryApproved,
			}),
			deviceToDelete: deleteWeb1.MFA.GetName(),
		},
		{
			name: "privilege token",
			createDeleteReq: deleteReqUsingToken(authclient.CreateUserTokenRequest{
				Name: username,
				TTL:  5 * time.Minute,
				Type: authclient.UserTokenTypePrivilege,
			}),
			deviceToDelete: deleteTOTP1.MFA.GetName(),
		},
		{
			name:            "Webauthn using challenge",
			createDeleteReq: deleteReqUsingChallenge(webDev1),
			deviceToDelete:  deleteWeb2.MFA.GetName(),
		},
		{
			name:            "TOTP using challenge",
			createDeleteReq: deleteReqUsingChallenge(webDev1),
			deviceToDelete:  deleteTOTP2.MFA.GetName(),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			deleteReq := test.createDeleteReq(t)
			deleteReq.DeviceName = test.deviceToDelete

			// Delete the device.
			mockEmitter.Reset()
			err = userClient.DeleteMFADeviceSync(ctx, deleteReq)
			require.NoError(t, err, "DeleteMFADeviceSync failed")

			// Verify device deletion.
			devs, err := authServer.Services.GetMFADevices(ctx, username, false /* withSecrets */)
			require.NoError(t, err, "GetMFADevices failed")
			for _, dev := range devs {
				if dev.GetName() == test.deviceToDelete {
					t.Errorf("DeleteMFADeviceSync(%q): device not deleted", test.deviceToDelete)
					return
				}
			}

			// Verify deletion event.
			event := mockEmitter.LastEvent()
			assert.Equal(t, events.MFADeviceDeleteEvent, event.GetType(), "event.Type")
			assert.Equal(t, events.MFADeviceDeleteEventCode, event.GetCode(), "event.Code")
			require.IsType(t, &apievents.MFADeviceDelete{}, event, "underlying event type")
			deleteEvent := event.(*apievents.MFADeviceDelete) // asserted above
			assert.Equal(t, username, deleteEvent.User, "event.User")
			assert.Contains(t, deleteEvent.ConnectionMetadata.RemoteAddr, "127.0.0.1", "client remote addr must be localhost")
		})
	}
}

func TestDeleteMFADeviceSync_WithErrors(t *testing.T) {
	t.Parallel()

	testServer := newTestTLSServer(t)
	authServer := testServer.Auth()
	clock := testServer.Clock()
	ctx := context.Background()

	const username = "llama@goteleport.com"
	_, _, err := authtest.CreateUserAndRole(authServer, username, []string{username}, nil)
	require.NoError(t, err)

	const origin = "localhost"
	authPreference, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOptional,
		Webauthn: &types.Webauthn{
			RPID: origin,
		},
	})
	require.NoError(t, err)
	_, err = authServer.UpsertAuthPreference(ctx, authPreference)
	require.NoError(t, err)

	userClient, err := testServer.NewClient(authtest.TestUser(username))
	require.NoError(t, err)

	// Insert a device.
	const devName = "otp"
	_, err = authtest.RegisterTestDevice(
		ctx, userClient, devName, proto.DeviceType_DEVICE_TYPE_TOTP, nil /* authenticator */, authtest.WithTestDeviceClock(clock))
	require.NoError(t, err)

	createReq := func(name string) *proto.DeleteMFADeviceSyncRequest {
		return &proto.DeleteMFADeviceSyncRequest{
			DeviceName: name,
		}
	}

	tests := []struct {
		name         string
		tokenRequest *authclient.CreateUserTokenRequest
		deleteReq    *proto.DeleteMFADeviceSyncRequest
		wantErr      string
		assertErr    func(error) bool
	}{
		{
			name: "token not found",
			deleteReq: &proto.DeleteMFADeviceSyncRequest{
				TokenID:    "unknown-token-id",
				DeviceName: devName,
			},
			wantErr:   "invalid token",
			assertErr: trace.IsAccessDenied,
		},
		{
			name: "invalid token type",
			tokenRequest: &authclient.CreateUserTokenRequest{
				Name: username,
				TTL:  5 * time.Minute,
				Type: "unknown-token-type",
			},
			deleteReq: createReq(devName),
			wantErr:   "invalid token",
			assertErr: trace.IsAccessDenied,
		},
		{
			name: "device not found",
			tokenRequest: &authclient.CreateUserTokenRequest{
				Name: username,
				TTL:  5 * time.Minute,
				Type: authclient.UserTokenTypeRecoveryApproved,
			},
			deleteReq: &proto.DeleteMFADeviceSyncRequest{
				DeviceName: "does-not-exist",
			},
			wantErr:   "does not exist",
			assertErr: trace.IsNotFound,
		},
		{
			name:      "neither token nor challenge provided",
			deleteReq: createReq(devName),
			wantErr:   "either a privilege token or",
			assertErr: trace.IsBadParameter,
		},
		{
			name: "invalid challenge",
			deleteReq: &proto.DeleteMFADeviceSyncRequest{
				DeviceName: devName,
				ExistingMFAResponse: &proto.MFAAuthenticateResponse{
					Response: &proto.MFAAuthenticateResponse_TOTP{
						TOTP: &proto.TOTPResponse{
							Code: "not an OTP code",
						},
					},
				},
			},
			wantErr:   "invalid totp token",
			assertErr: trace.IsAccessDenied,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			deleteReq := test.deleteReq

			if test.tokenRequest != nil {
				token, err := authServer.NewUserToken(*test.tokenRequest)
				require.NoError(t, err)
				_, err = authServer.CreateUserToken(context.Background(), token)
				require.NoError(t, err)

				deleteReq.TokenID = token.GetName()
			}

			err := userClient.DeleteMFADeviceSync(ctx, deleteReq)
			assert.ErrorContains(t, err, test.wantErr, "DeleteMFADeviceSync error mismatch")
			assert.True(t,
				test.assertErr(err),
				"DeleteMFADeviceSync error type assertion failed, got err=%q (%T)", err, trace.Unwrap(err))
		})
	}
}

// TestDeleteMFADeviceSync_lastDevice tests for preventing deletion of last
// device when second factor is required.
func TestDeleteMFADeviceSync_lastDevice(t *testing.T) {
	t.Parallel()

	testServer := newTestTLSServer(t)
	authServer := testServer.Auth()
	ctx := context.Background()

	// Create a user with TOTP and Webauthn.
	userCreds, err := createUserWithSecondFactors(testServer)
	require.NoError(t, err, "createUserWithSecondFactors")

	userClient, err := testServer.NewClient(authtest.TestUser(userCreds.username))
	require.NoError(t, err, "NewClient")
	totpDev := userCreds.totpDev
	webDev := userCreds.webDev

	// Reuse Webauthn config from createUserWithSecondFactors.
	authPref, err := authServer.GetAuthPreference(ctx)
	require.NoError(t, err, "GetAuthPreference")
	webConfig, _ := authPref.GetWebauthn()

	// Define various test helpers.
	setSecondFactor := func(t *testing.T, sf constants.SecondFactorType) {
		authPreference, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
			Type:         constants.Local,
			SecondFactor: sf,
			Webauthn:     webConfig,
		})
		require.NoError(t, err, "NewAuthPreference")
		_, err = authServer.UpsertAuthPreference(ctx, authPreference)
		require.NoError(t, err, "UpsertAuthPreference")
	}

	deleteDevice := func(userClient *authclient.Client, testDev *authtest.Device) error {
		// Issue and solve authn challenge.
		authnChal, err := userClient.CreateAuthenticateChallenge(ctx, &proto.CreateAuthenticateChallengeRequest{
			Request: &proto.CreateAuthenticateChallengeRequest_ContextUser{
				ContextUser: &proto.ContextUser{},
			},
			ChallengeExtensions: &mfav1.ChallengeExtensions{
				Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_MANAGE_DEVICES,
			},
		})
		if err != nil {
			return err
		}
		authnSolved, err := testDev.SolveAuthn(authnChal)
		if err != nil {
			return err
		}

		return userClient.DeleteMFADeviceSync(ctx, &proto.DeleteMFADeviceSyncRequest{
			DeviceName:          testDev.MFA.GetName(),
			ExistingMFAResponse: authnSolved,
		})
	}

	makeTest := func(sf constants.SecondFactorType, deviceToDelete *authtest.Device) func(t *testing.T) {
		return func(t *testing.T) {
			t.Helper()

			setSecondFactor(t, sf)

			// Attempt deletion.
			const wantErr = "cannot delete the last"
			assert.ErrorContains(t,
				deleteDevice(userClient, deviceToDelete),
				wantErr)

			devicesResp, err := userClient.GetMFADevices(ctx, &proto.GetMFADevicesRequest{})
			require.NoError(t, err, "GetMFADevices")
			devName := deviceToDelete.MFA.GetName()
			for _, dev := range devicesResp.Devices {
				if dev.GetName() == devName {
					return // Success, device not deleted.
				}
			}
			t.Errorf("Device %q wrongly deleted", devName)
		}
	}

	// First attempt deletions on specific modes like TOTP and Webauthn.
	// These shouldn't work because we only have one of each device.
	t.Run("second factor otp", makeTest(constants.SecondFactorOTP, totpDev))
	t.Run("second factor webauthn", makeTest(constants.SecondFactorWebauthn, webDev))

	// Make sure only one device is left, then attempt deletion with
	// second_factor=on.
	setSecondFactor(t, constants.SecondFactorOptional)
	require.NoError(t,
		deleteDevice(userClient, webDev),
		"Second-to-last device deletion failed",
	)
	t.Run("second factor on, otp device", makeTest(constants.SecondFactorOn, totpDev))

	// Same as above, but now delete the last Webauthn device.
	webDev, err = authtest.RegisterTestDevice(ctx, userClient, "web1", proto.DeviceType_DEVICE_TYPE_WEBAUTHN, totpDev /* authenticator */)
	require.NoError(t, err, "RegisterTestDevice")
	require.NoError(t,
		deleteDevice(userClient, totpDev),
		"Second-to-last device deletion failed",
	)
	t.Run("second factor on, otp device", makeTest(constants.SecondFactorOn, webDev))
}

func TestAddMFADeviceSync(t *testing.T) {
	t.Parallel()

	testServer := newTestTLSServer(t)
	authServer := testServer.Auth()
	mockEmitter := &eventstest.MockRecorderEmitter{}
	authServer.SetEmitter(mockEmitter)
	clock := authServer.GetClock()
	ctx := context.Background()

	authPreference, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOn,
		Webauthn: &types.Webauthn{
			RPID: "localhost",
		},
	})
	require.NoError(t, err)
	_, err = authServer.UpsertAuthPreference(ctx, authPreference)
	require.NoError(t, err)

	u, err := createUserWithSecondFactors(testServer)
	require.NoError(t, err)

	userClient, err := testServer.NewClient(authtest.TestUser(u.username))
	require.NoError(t, err)

	solveChallengeWithToken := func(
		t *testing.T,
		tokenType string,
		deviceType proto.DeviceType,
		deviceUsage proto.DeviceUsage,
	) (token string, testDev *authtest.Device, registerSolved *proto.MFARegisterResponse) {
		privilegeToken, err := auth.CreatePrivilegeToken(ctx, authServer, u.username, tokenType)
		require.NoError(t, err, "createPrivilegeToken")
		token = privilegeToken.GetName()

		registerChal, err := authServer.CreateRegisterChallenge(ctx, &proto.CreateRegisterChallengeRequest{
			TokenID:     token,
			DeviceType:  deviceType,
			DeviceUsage: deviceUsage,
		})
		require.NoError(t, err, "CreateRegisterChallenge")

		testDev, registerSolved, err = authtest.NewTestDeviceFromChallenge(registerChal, authtest.WithTestDeviceClock(clock))
		require.NoError(t, err, "NewTestDeviceFromChallenge")
		return token, testDev, registerSolved
	}

	solveChallengeWithUser := func(
		t *testing.T,
		deviceType proto.DeviceType,
		deviceUsage proto.DeviceUsage,
	) (*authtest.Device, *proto.MFARegisterResponse) {
		// Create and solve a registration challenge.
		authChal, err := userClient.CreateAuthenticateChallenge(ctx, &proto.CreateAuthenticateChallengeRequest{
			Request: &proto.CreateAuthenticateChallengeRequest_ContextUser{
				ContextUser: &proto.ContextUser{},
			},
			ChallengeExtensions: &mfav1.ChallengeExtensions{
				Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_MANAGE_DEVICES,
			},
		})
		require.NoError(t, err, "CreateAuthenticateChallenge")

		authSolved, err := u.webDev.SolveAuthn(authChal)
		require.NoError(t, err, "SolveAuthn")

		registerChal, err := userClient.CreateRegisterChallenge(ctx, &proto.CreateRegisterChallengeRequest{
			ExistingMFAResponse: authSolved,
			DeviceType:          deviceType,
			DeviceUsage:         deviceUsage,
		})
		require.NoError(t, err, "CreateRegisterChallenge")

		testDev, registerSolved, err := authtest.NewTestDeviceFromChallenge(
			registerChal,
			authtest.WithTestDeviceClock(clock),
		)
		require.NoError(t, err, "NewTestDeviceFromChallenge")
		return testDev, registerSolved
	}

	tests := []struct {
		name       string
		deviceName string
		wantErr    bool
		getReq     func(t *testing.T, deviceName string) *proto.AddMFADeviceSyncRequest
	}{
		{
			name:    "invalid token type",
			wantErr: true,
			getReq: func(t *testing.T, deviceName string) *proto.AddMFADeviceSyncRequest {
				// Obtain a non privilege token.
				token, err := authServer.NewUserToken(authclient.CreateUserTokenRequest{
					Name: u.username,
					TTL:  5 * time.Minute,
					Type: authclient.UserTokenTypeResetPassword,
				})
				require.NoError(t, err)
				_, err = authServer.CreateUserToken(ctx, token)
				require.NoError(t, err)

				return &proto.AddMFADeviceSyncRequest{
					TokenID:       token.GetName(),
					NewDeviceName: deviceName,
				}
			},
		},
		{
			name:       "TOTP device with privilege token",
			deviceName: "new-totp",
			getReq: func(t *testing.T, deviceName string) *proto.AddMFADeviceSyncRequest {
				token, _, registerSolved := solveChallengeWithToken(
					t, authclient.UserTokenTypePrivilege, proto.DeviceType_DEVICE_TYPE_TOTP, proto.DeviceUsage_DEVICE_USAGE_MFA)

				return &proto.AddMFADeviceSyncRequest{
					TokenID:        token,
					NewDeviceName:  deviceName,
					NewMFAResponse: registerSolved,
				}
			},
		},
		{
			name:       "Webauthn device with privilege exception token",
			deviceName: "new-webauthn",
			getReq: func(t *testing.T, deviceName string) *proto.AddMFADeviceSyncRequest {
				token, _, registerSolved := solveChallengeWithToken(
					t, authclient.UserTokenTypePrivilegeException, proto.DeviceType_DEVICE_TYPE_WEBAUTHN, proto.DeviceUsage_DEVICE_USAGE_MFA)

				return &proto.AddMFADeviceSyncRequest{
					TokenID:        token,
					NewDeviceName:  deviceName,
					NewMFAResponse: registerSolved,
				}
			},
		},
		{
			name:       "invalid device name length",
			deviceName: strings.Repeat("A", auth.MFADeviceNameMaxLen+1),
			wantErr:    true,
			getReq: func(t *testing.T, deviceName string) *proto.AddMFADeviceSyncRequest {
				token, _, registerSolved := solveChallengeWithToken(
					t, authclient.UserTokenTypePrivilegeException, proto.DeviceType_DEVICE_TYPE_WEBAUTHN, proto.DeviceUsage_DEVICE_USAGE_MFA)

				return &proto.AddMFADeviceSyncRequest{
					TokenID:        token,
					NewDeviceName:  deviceName,
					NewMFAResponse: registerSolved,
				}
			},
		},
		{
			name:       "WebAuthn with context user",
			deviceName: "context-webauthn1",
			getReq: func(t *testing.T, deviceName string) *proto.AddMFADeviceSyncRequest {
				_, registerSolved := solveChallengeWithUser(
					t,
					proto.DeviceType_DEVICE_TYPE_WEBAUTHN,
					proto.DeviceUsage_DEVICE_USAGE_MFA)

				return &proto.AddMFADeviceSyncRequest{
					ContextUser:    &proto.ContextUser{},
					NewDeviceName:  deviceName,
					NewMFAResponse: registerSolved,
					DeviceUsage:    proto.DeviceUsage_DEVICE_USAGE_MFA,
				}
			},
		},
		{
			name:       "TOTP with context user",
			deviceName: "context-totp1",
			getReq: func(t *testing.T, deviceName string) *proto.AddMFADeviceSyncRequest {
				_, registerSolved := solveChallengeWithUser(
					t,
					proto.DeviceType_DEVICE_TYPE_TOTP,
					proto.DeviceUsage_DEVICE_USAGE_MFA)

				return &proto.AddMFADeviceSyncRequest{
					ContextUser:    &proto.ContextUser{},
					NewDeviceName:  deviceName,
					NewMFAResponse: registerSolved,
					DeviceUsage:    proto.DeviceUsage_DEVICE_USAGE_MFA,
				}
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, err := userClient.AddMFADeviceSync(ctx, tc.getReq(t, tc.deviceName))
			switch {
			case tc.wantErr:
				expectedErr := trace.IsAccessDenied(err) || trace.IsBadParameter(err)
				require.True(t, expectedErr)
			default:
				require.NoError(t, err)
				require.Equal(t, tc.deviceName, res.GetDevice().GetName())

				// Test events emitted.
				event := mockEmitter.LastEvent()
				require.Equal(t, events.MFADeviceAddEvent, event.GetType())
				require.Equal(t, events.MFADeviceAddEventCode, event.GetCode())
				addEvt := event.(*apievents.MFADeviceAdd)
				require.Equal(t, u.username, addEvt.UserMetadata.User)
				assert.Contains(t, addEvt.ConnectionMetadata.RemoteAddr, "127.0.0.1", "client remote addr must be localhost")

				// Check it's been added.
				res, err := userClient.GetMFADevices(ctx, &proto.GetMFADevicesRequest{})
				require.NoError(t, err)

				found := false
				for _, mfa := range res.GetDevices() {
					if mfa.GetName() == tc.deviceName {
						found = true
						break
					}
				}
				require.True(t, found, "MFA device %q not found", tc.deviceName)
			}
		})
	}
}

func TestGetMFADevices_WithToken(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)
	ctx := context.Background()

	authPreference, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOptional,
		Webauthn: &types.Webauthn{
			RPID: "localhost",
		},
	})
	require.NoError(t, err)
	_, err = srv.Auth().UpsertAuthPreference(ctx, authPreference)
	require.NoError(t, err)

	username := "llama@goteleport.com"
	_, _, err = authtest.CreateUserAndRole(srv.Auth(), username, []string{username}, nil)
	require.NoError(t, err)

	clt, err := srv.NewClient(authtest.TestUser(username))
	require.NoError(t, err)
	webDev, err := authtest.RegisterTestDevice(ctx, clt, "web", proto.DeviceType_DEVICE_TYPE_WEBAUTHN, nil /* authenticator */)
	require.NoError(t, err)
	totpDev, err := authtest.RegisterTestDevice(ctx, clt, "otp", proto.DeviceType_DEVICE_TYPE_TOTP, webDev, authtest.WithTestDeviceClock(srv.Clock()))
	require.NoError(t, err)

	tests := []struct {
		name         string
		wantErr      bool
		tokenRequest *authclient.CreateUserTokenRequest
	}{
		{
			name:    "token not found",
			wantErr: true,
		},
		{
			name:    "invalid token type",
			wantErr: true,
			tokenRequest: &authclient.CreateUserTokenRequest{
				Name: username,
				TTL:  5 * time.Minute,
				Type: authclient.UserTokenTypeResetPassword,
			},
		},
		{
			name: "valid token",
			tokenRequest: &authclient.CreateUserTokenRequest{
				Name: username,
				TTL:  5 * time.Minute,
				Type: authclient.UserTokenTypeRecoveryApproved,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tokenID := "test-token-not-found"

			if tc.tokenRequest != nil {
				token, err := srv.Auth().NewUserToken(*tc.tokenRequest)
				require.NoError(t, err)
				_, err = srv.Auth().CreateUserToken(context.Background(), token)
				require.NoError(t, err)

				tokenID = token.GetName()
			}

			res, err := srv.Auth().GetMFADevices(ctx, &proto.GetMFADevicesRequest{
				TokenID: tokenID,
			})

			switch {
			case tc.wantErr:
				require.True(t, trace.IsAccessDenied(err))
			default:
				require.NoError(t, err)
				compareDevices(t, true /* ignoreUpdateAndCounter */, res.GetDevices(), webDev.MFA, totpDev.MFA)
			}
		})
	}
}

func TestGetMFADevices_WithAuth(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	authPreference, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOptional,
		Webauthn: &types.Webauthn{
			RPID: "localhost",
		},
	})
	require.NoError(t, err)
	_, err = srv.Auth().UpsertAuthPreference(ctx, authPreference)
	require.NoError(t, err)

	username := "llama@goteleport.com"
	_, _, err = authtest.CreateUserAndRole(srv.Auth(), username, []string{username}, nil)
	require.NoError(t, err)

	clt, err := srv.NewClient(authtest.TestUser(username))
	require.NoError(t, err)
	webDev, err := authtest.RegisterTestDevice(ctx, clt, "web", proto.DeviceType_DEVICE_TYPE_WEBAUTHN, nil /* authenticator */)
	require.NoError(t, err)
	totpDev, err := authtest.RegisterTestDevice(ctx, clt, "otp", proto.DeviceType_DEVICE_TYPE_TOTP, webDev, authtest.WithTestDeviceClock(srv.Clock()))
	require.NoError(t, err)

	res, err := clt.GetMFADevices(ctx, &proto.GetMFADevicesRequest{})
	require.NoError(t, err)
	compareDevices(t, true /* ignoreUpdateAndCounter */, res.GetDevices(), webDev.MFA, totpDev.MFA)
}

func newTestServices(t *testing.T) auth.Services {
	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)

	configService, err := local.NewClusterConfigurationService(bk)
	require.NoError(t, err)

	identityService, err := local.NewTestIdentityService(bk)
	require.NoError(t, err)

	return auth.Services{
		TrustInternal:                local.NewCAService(bk),
		PresenceInternal:             local.NewPresenceService(bk),
		Provisioner:                  local.NewProvisioningService(bk),
		Identity:                     identityService,
		Access:                       local.NewAccessService(bk),
		DynamicAccessExt:             local.NewDynamicAccessService(bk),
		ClusterConfigurationInternal: configService,
		Events:                       local.NewEventsService(bk),
		AuditLogSessionStreamer:      events.NewDiscardAuditLog(),
	}
}

func compareDevices(t *testing.T, ignoreUpdateAndCounter bool, got []*types.MFADevice, want ...*types.MFADevice) {
	sort.Slice(got, func(i, j int) bool { return got[i].GetName() < got[j].GetName() })
	sort.Slice(want, func(i, j int) bool { return want[i].GetName() < want[j].GetName() })

	// Remove TOTP keys before comparison.
	for _, w := range want {
		totp := w.GetTotp()
		if totp == nil {
			continue
		}
		if totp.Key == "" {
			continue
		}
		key := totp.Key
		// defer in loop on purpose, we want this to run at the end of the function.
		defer func() {
			totp.Key = key
		}()
		totp.Key = ""
	}

	// Ignore LastUsed and SignatureCounter?
	var opts []gocmp.Option
	if ignoreUpdateAndCounter {
		opts = append(opts, gocmp.FilterPath(func(path gocmp.Path) bool {
			p := path.String()
			return p == "LastUsed" || p == "Device.Webauthn.SignatureCounter"
		}, gocmp.Ignore()))
	}

	if diff := gocmp.Diff(want, got, opts...); diff != "" {
		t.Errorf("compareDevices mismatch (-want +got):\n%s", diff)
	}
}

type mockCache struct {
	authclient.Cache

	resources      []types.ResourceWithLabels
	resourcesError error
}

func (m mockCache) ListResources(ctx context.Context, req proto.ListResourcesRequest) (*types.ListResourcesResponse, error) {
	if m.resourcesError != nil {
		return nil, m.resourcesError
	}

	if req.StartKey != "" {
		return nil, nil
	}

	return &types.ListResourcesResponse{Resources: m.resources}, nil
}

func TestFilterResources(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	fail := errors.New("fail")

	const resourceCount = 100
	nodes := make([]types.ResourceWithLabels, 0, resourceCount)

	for range resourceCount {
		s, err := types.NewServer(uuid.NewString(), types.KindNode, types.ServerSpecV2{})
		require.NoError(t, err)
		nodes = append(nodes, s)
	}

	cases := []struct {
		name           string
		limit          int32
		filterFn       func(labels types.ResourceWithLabels) error
		errorAssertion require.ErrorAssertionFunc
		cache          mockCache
	}{
		{
			name:  "ListResources fails",
			cache: mockCache{resourcesError: fail},
			errorAssertion: func(t require.TestingT, err error, i ...any) {
				require.Error(t, err, i...)
				require.ErrorIs(t, err, fail)
			},
		},
		{
			name:           "Done returns no errors",
			cache:          mockCache{resources: nodes},
			errorAssertion: require.NoError,
			filterFn: func(labels types.ResourceWithLabels) error {
				return auth.ErrDone
			},
		},
		{
			name:  "fatal errors are propagated",
			cache: mockCache{resources: nodes},
			errorAssertion: func(t require.TestingT, err error, i ...any) {
				require.Error(t, err, i...)
				require.ErrorIs(t, err, fail)
			},
			filterFn: func(labels types.ResourceWithLabels) error {
				return fail
			},
		},
		{
			name:           "no errors iterates the entire resource set",
			cache:          mockCache{resources: nodes},
			errorAssertion: require.NoError,
			filterFn: func(labels types.ResourceWithLabels) error {
				return nil
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := &auth.Server{Cache: tt.cache}

			err := srv.IterateResources(ctx, proto.ListResourcesRequest{
				ResourceType: types.KindNode,
				Namespace:    apidefaults.Namespace,
				Limit:        tt.limit,
			}, tt.filterFn)
			tt.errorAssertion(t, err)
		})
	}
}

type fakeAuthPreferenceGetter struct{}

func (f *fakeAuthPreferenceGetter) GetAuthPreference(context.Context) (types.AuthPreference, error) {
	return types.DefaultAuthPreference(), nil
}

func TestCAGeneration(t *testing.T) {
	ctx := context.Background()
	const (
		clusterName = "cluster1"
		HostUUID    = "0000-000-000-0000"
	)
	// Cache key for better performance as we don't care about the value being unique.
	privKey, pubKey, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)

	keyStoreManager, err := keystore.NewManager(t.Context(), &servicecfg.KeystoreConfig{}, &keystore.Options{
		ClusterName:          &types.ClusterNameV2{Metadata: types.Metadata{Name: clusterName}},
		AuthPreferenceGetter: &fakeAuthPreferenceGetter{},
		RSAKeyPairSource: func() (priv []byte, pub []byte, err error) {
			return privKey, pubKey, nil
		},
	})
	require.NoError(t, err)

	for _, caType := range types.CertAuthTypes {
		t.Run(string(caType), func(t *testing.T) {
			testKeySet := authtest.NewTestCA(caType, clusterName, privKey).Spec.ActiveKeys
			keySet, err := auth.NewKeySet(ctx, keyStoreManager, types.CertAuthID{Type: caType, DomainName: clusterName})
			require.NoError(t, err)

			// Don't compare values as those are different. Only check if the key is set/not set in both cases.
			require.Equal(t, len(testKeySet.SSH) > 0, len(keySet.SSH) > 0,
				"test CA and production CA have different SSH keys for type %v", caType)
			require.Equal(t, len(testKeySet.TLS) > 0, len(keySet.TLS) > 0,
				"test CA and production CA have different TLS keys for type %v", caType)
			require.Equal(t, len(testKeySet.JWT) > 0, len(keySet.JWT) > 0,
				"test CA and production CA have different JWT keys for type %v", caType)
		})
	}
}

func TestGetLicense(t *testing.T) {
	s := newAuthSuite(t)

	// GetLicense should return error if license is not set
	_, err := s.a.GetLicense(context.Background())
	assert.Error(t, err)

	// GetLicense should return cert and key pem concatenated, when license is set
	l := license.License{
		CertPEM: []byte("cert"),
		KeyPEM:  []byte("key"),
	}
	s.a.SetLicense(&l)

	actual, err := s.a.GetLicense(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("%s%s", l.CertPEM, l.KeyPEM), actual)
}

func TestInstallerCRUD(t *testing.T) {
	t.Parallel()
	s := newAuthSuite(t)
	ctx := context.Background()

	var inst types.Installer
	var err error
	contents := "#! just some script contents"
	inst, err = types.NewInstallerV1(installers.InstallerScriptName, contents)
	require.NoError(t, err)

	require.NoError(t, s.a.SetInstaller(ctx, inst))

	inst, err = s.a.GetInstaller(ctx, installers.InstallerScriptName)
	require.NoError(t, err)
	require.Equal(t, contents, inst.GetScript())

	newContents := "nothing useful here"
	newInstaller, err := types.NewInstallerV1("other-script", newContents)
	require.NoError(t, err)
	require.NoError(t, s.a.SetInstaller(ctx, newInstaller))

	newInst, err := s.a.GetInstaller(ctx, "other-script")
	require.NoError(t, err)
	require.Equal(t, newContents, newInst.GetScript())

	instcoll, err := s.a.GetInstallers(ctx)
	require.NoError(t, err)
	var instScripts []string
	for _, inst := range instcoll {
		instScripts = append(instScripts, inst.GetScript())
	}

	require.ElementsMatch(t,
		[]string{inst.GetScript(), newInst.GetScript()},
		instScripts,
	)

	err = s.a.DeleteInstaller(ctx, installers.InstallerScriptName)
	require.NoError(t, err)

	_, err = s.a.GetInstaller(ctx, installers.InstallerScriptName)
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err))
}

func TestGetTokens(t *testing.T) {
	t.Parallel()
	s := newAuthSuite(t)
	ctx := context.Background()

	_, _, err := authtest.CreateUserAndRole(s.a, "username", []string{"username"}, nil)
	require.NoError(t, err)
	_, err = s.a.CreateResetPasswordToken(ctx, authclient.CreateUserTokenRequest{
		Name: "username",
		TTL:  time.Minute,
		Type: authclient.UserTokenTypeResetPasswordInvite,
	})
	require.NoError(t, err)

	for _, role := range types.LocalServiceMappings() {
		generateTestToken(ctx, t, types.SystemRoles{role}, s.a.GetClock().Now().Add(time.Minute*30), s.a)
	}

	_, err = s.a.GetTokens(ctx)
	require.NoError(t, err)
}

func TestAccessRequestAuditLog(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	p, err := newTestPack(ctx, t.TempDir())
	require.NoError(t, err)

	fakeClock := clockwork.NewFakeClock()
	p.a.Notifications, err = local.NewNotificationsService(p.bk, fakeClock)
	require.NoError(t, err)

	requester, _, _ := createSessionTestUsers(t, p.a)

	paymentsRole, err := types.NewRole("paymentsRole", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Request: &types.AccessRequestConditions{
				Roles: []string{"requestRole"},
				Annotations: map[string][]string{
					"pagerduty_services": {"payments"},
				},
			},
		},
	})
	require.NoError(t, err)

	requestRole, err := types.NewRole("requestRole", types.RoleSpecV6{})
	require.NoError(t, err)

	p.a.CreateRole(ctx, requestRole)
	p.a.CreateRole(ctx, paymentsRole)

	user, err := p.a.GetUser(ctx, requester, true)
	require.NoError(t, err)
	user.AddRole(paymentsRole.GetName())
	_, err = p.a.UpsertUser(ctx, user)
	require.NoError(t, err)

	accessRequest, err := types.NewAccessRequest(uuid.NewString(), requester, "requestRole")
	require.NoError(t, err)
	req, err := p.a.CreateAccessRequestV2(ctx, accessRequest, authtest.TestUser(requester).I.GetIdentity())
	require.NoError(t, err)

	expectedAnnotations, err := apievents.EncodeMapStrings(paymentsRole.GetAccessRequestConditions(types.Allow).Annotations)
	require.NoError(t, err)

	arc, ok := p.mockEmitter.LastEvent().(*apievents.AccessRequestCreate)
	require.True(t, ok)
	require.Equal(t, expectedAnnotations, arc.Annotations)
	require.Equal(t, "PENDING", arc.RequestState)

	err = p.a.SetAccessRequestState(ctx, types.AccessRequestUpdate{
		RequestID: req.GetName(),
		State:     types.RequestState_APPROVED,
	})
	require.NoError(t, err)

	arc, ok = p.mockEmitter.LastEvent().(*apievents.AccessRequestCreate)
	require.True(t, ok)
	require.Equal(t, expectedAnnotations, arc.Annotations)
	require.Equal(t, "APPROVED", arc.RequestState)
}

func testCreateRole(t *testing.T, server *authtest.TLSServer, name string, setup func(*types.RoleSpecV6)) types.Role {
	t.Helper()
	ctx := context.Background()

	spec := types.RoleSpecV6{
		Allow: types.RoleConditions{
			Request: &types.AccessRequestConditions{
				Reason: &types.AccessRequestConditionsReason{},
			},
			ReviewRequests: &types.AccessReviewConditions{},
		},
		Deny: types.RoleConditions{
			Request:        &types.AccessRequestConditions{},
			ReviewRequests: &types.AccessReviewConditions{},
		},
	}
	setup(&spec)

	role, err := types.NewRole(name, spec)
	require.NoError(t, err, "types.NewRole")

	createdRole, err := server.AuthServer.AuthServer.UpsertRole(ctx, role)
	require.NoError(t, err, "AuthServer.UpsertRole")

	return createdRole
}

func testCreateUserWithRoles(t *testing.T, server *authtest.TLSServer, user string, roles ...string) (authtest.TestIdentity, *authclient.Client) {
	t.Helper()
	ctx := context.Background()

	u, err := types.NewUser(user)
	require.NoError(t, err, "types.NewUser")
	u.SetRoles(roles)
	_, err = server.AuthServer.AuthServer.UpsertUser(ctx, u)
	require.NoError(t, err, "AuthServer.UpsertUser")

	identity := authtest.TestUser(user)
	client, err := server.NewClient(identity)
	require.NoError(t, err, "server.NewClient")

	return identity, client
}

func TestAccessRequestNotifications(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	fakeClock := clockwork.NewFakeClock()

	testAuthServer, err := authtest.NewAuthServer(authtest.AuthServerConfig{
		Dir:   t.TempDir(),
		Clock: fakeClock,
	})
	require.NoError(t, err)
	testTLSServer, err := testAuthServer.NewTestTLSServer()
	require.NoError(t, err)

	reviewerUsername := "reviewer"
	requesterUsername := "requester"
	requestRoleName := "requestRole"

	reviewerRole := testCreateRole(t, testTLSServer, reviewerUsername, func(spec *types.RoleSpecV6) {
		spec.Allow.Logins = []string{"user"}
		spec.Allow.ReviewRequests.Roles = []string{"requestRole"}
	})

	requesterRole := testCreateRole(t, testTLSServer, requesterUsername, func(spec *types.RoleSpecV6) {
		spec.Allow.Request.Roles = []string{requestRoleName}
	})

	requestRole := testCreateRole(t, testTLSServer, requestRoleName, func(spec *types.RoleSpecV6) {
		spec.Allow.Request.Roles = []string{requestRoleName}
	})

	reviewer, reviewerClient := testCreateUserWithRoles(t, testTLSServer, reviewerUsername, reviewerRole.GetName())

	requester, _ := testCreateUserWithRoles(t, testTLSServer, requesterUsername, requesterRole.GetName())

	accessRequest, err := types.NewAccessRequest(uuid.NewString(), requester.GetUsername(), requestRole.GetName())
	require.NoError(t, err)
	req, err := testTLSServer.AuthServer.AuthServer.CreateAccessRequestV2(ctx, accessRequest, reviewer.I.GetIdentity())
	require.NoError(t, err)

	// Verify that a global notification was created which matches for users who can review the requestRole.
	globalNotifsResp, _, err := testTLSServer.AuthServer.AuthServer.Notifications.ListGlobalNotifications(ctx, 100, "")
	require.NoError(t, err)
	require.Len(t, globalNotifsResp, 1)
	require.Equal(t, &types.AccessReviewConditions{
		Roles: []string{requestRole.GetName()},
	}, globalNotifsResp[0].GetSpec().GetByPermissions().GetRoleConditions()[0].ReviewRequests)

	// Approve the request
	_, err = reviewerClient.SubmitAccessReview(ctx, types.AccessReviewSubmission{
		RequestID: req.GetName(),
		Review: types.AccessReview{
			ProposedState: types.RequestState_APPROVED,
		},
	})
	require.NoError(t, err)
	// Verify that a user notification was created notifying the requester that their access request was approved.
	userNotifsResp, _, err := testTLSServer.AuthServer.AuthServer.Notifications.ListUserNotifications(ctx, 100, "")
	require.NoError(t, err)
	require.Len(t, userNotifsResp, 1)
	require.Contains(t, userNotifsResp[0].GetMetadata().GetLabels()[types.NotificationTitleLabel], "reviewer approved your access request")

	// Create another access request.
	accessRequest, err = types.NewAccessRequest(uuid.NewString(), requester.GetUsername(), requestRole.GetName())
	require.NoError(t, err)
	req, err = testTLSServer.AuthServer.AuthServer.CreateAccessRequestV2(ctx, accessRequest, authtest.TestUser(requester.GetUsername()).I.GetIdentity())
	require.NoError(t, err)

	// Deny the request.
	_, err = reviewerClient.SubmitAccessReview(ctx, types.AccessReviewSubmission{
		RequestID: req.GetName(),
		Review: types.AccessReview{
			ProposedState: types.RequestState_DENIED,
		},
	})
	require.NoError(t, err)
	// Verify that a user notification was created notifying the requester that their access request was denied.
	userNotifsResp, _, err = testTLSServer.AuthServer.AuthServer.Notifications.ListUserNotifications(ctx, 100, "")
	require.NoError(t, err)
	require.Len(t, userNotifsResp, 2)
	require.Contains(t, userNotifsResp[1].GetMetadata().GetLabels()[types.NotificationTitleLabel], "reviewer denied your access request")
}

func testNewAccessRequest(t *testing.T, user string, roles ...string) types.AccessRequest {
	t.Helper()
	r, err := types.NewAccessRequest(uuid.NewString(), user, roles...)
	require.NoError(t, err, "types.NewAccessRequest")
	return r
}

func TestAccessRequestDryRunEnrichment(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	testAuthServer, err := authtest.NewAuthServer(authtest.AuthServerConfig{
		Dir:   t.TempDir(),
		Clock: clockwork.NewFakeClock(),
	})
	require.NoError(t, err)
	testTLSServer, err := testAuthServer.NewTestTLSServer()
	require.NoError(t, err)

	someRole := testCreateRole(t, testTLSServer, "some-role", func(spec *types.RoleSpecV6) {})

	someRoleRequesterRole := testCreateRole(t, testTLSServer, "some-role-requester", func(spec *types.RoleSpecV6) {
		spec.Allow.Request.Roles = []string{someRole.GetName()}
	})

	someRoleRequesterRoleRequiringReason := testCreateRole(t, testTLSServer, "some-role-requester-requiring-reason", func(spec *types.RoleSpecV6) {
		spec.Allow.Request.Roles = []string{someRole.GetName()}
		spec.Allow.Request.Reason.Mode = types.RequestReasonModeRequired
	})

	globalPromptRole1 := testCreateRole(t, testTLSServer, "prompt-role-1", func(spec *types.RoleSpecV6) {
		spec.Options.RequestPrompt = "test prompt #1"
	})
	globalPromptRole2 := testCreateRole(t, testTLSServer, "prompt-role-2", func(spec *types.RoleSpecV6) {
		spec.Options.RequestPrompt = "test prompt #2"
	})

	t.Run("requesting-role-no-reason-required-no-prompts", func(t *testing.T) {
		requester, requesterClient := testCreateUserWithRoles(t, testTLSServer, "requester",
			someRoleRequesterRole.GetName(),
		)

		dryRunAccessRequest := testNewAccessRequest(t, requester.GetUsername(), someRole.GetName())
		dryRunAccessRequest.SetDryRun(true)

		resp, err := requesterClient.CreateAccessRequestV2(ctx, dryRunAccessRequest)
		require.NoError(t, err)

		require.NotNil(t, resp.GetDryRunEnrichment())
		// check reason mode
		require.Equal(t, types.RequestReasonModeOptional, resp.GetDryRunEnrichment().ReasonMode)
		// check prompts
		require.Empty(t, resp.GetDryRunEnrichment().ReasonPrompts)
	})

	t.Run("requesting-role-reason-required", func(t *testing.T) {
		requester, requesterClient := testCreateUserWithRoles(t, testTLSServer, "requester",
			someRoleRequesterRoleRequiringReason.GetName(),
		)

		dryRunAccessRequest := testNewAccessRequest(t, requester.GetUsername(), someRole.GetName())
		dryRunAccessRequest.SetDryRun(true)

		resp, err := requesterClient.CreateAccessRequestV2(ctx, dryRunAccessRequest)
		require.NoError(t, err)

		require.NotNil(t, resp.GetDryRunEnrichment())
		// check reason mode
		require.Equal(t, types.RequestReasonModeRequired, resp.GetDryRunEnrichment().ReasonMode)
		// check prompts
		require.Empty(t, resp.GetDryRunEnrichment().ReasonPrompts)
	})

	t.Run("requesting-role-multiple-prompts", func(t *testing.T) {
		requester, requesterClient := testCreateUserWithRoles(t, testTLSServer, "requester",
			someRoleRequesterRole.GetName(),
			globalPromptRole1.GetName(),
			globalPromptRole2.GetName(),
		)

		dryRunAccessRequest := testNewAccessRequest(t, requester.GetUsername(), someRole.GetName())
		dryRunAccessRequest.SetDryRun(true)

		resp, err := requesterClient.CreateAccessRequestV2(ctx, dryRunAccessRequest)
		require.NoError(t, err)

		require.NotNil(t, resp.GetDryRunEnrichment())
		// check reason mode
		require.Equal(t, types.RequestReasonModeOptional, resp.GetDryRunEnrichment().ReasonMode)
		// check prompts
		require.Len(t, resp.GetDryRunEnrichment().ReasonPrompts, 2)
		require.Contains(t, resp.GetDryRunEnrichment().ReasonPrompts, globalPromptRole1.GetOptions().RequestPrompt)
		require.Contains(t, resp.GetDryRunEnrichment().ReasonPrompts, globalPromptRole2.GetOptions().RequestPrompt)
	})

	t.Run("requesting-role-reason-required-and-multiple-prompts", func(t *testing.T) {
		requester, requesterClient := testCreateUserWithRoles(t, testTLSServer, "requester",
			someRoleRequesterRole.GetName(),
			someRoleRequesterRoleRequiringReason.GetName(),
			globalPromptRole1.GetName(),
			globalPromptRole2.GetName(),
		)

		dryRunAccessRequest := testNewAccessRequest(t, requester.GetUsername(), someRole.GetName())
		dryRunAccessRequest.SetDryRun(true)

		resp, err := requesterClient.CreateAccessRequestV2(ctx, dryRunAccessRequest)
		require.NoError(t, err)

		require.NotNil(t, resp.GetDryRunEnrichment())
		// check reason mode
		require.Equal(t, types.RequestReasonModeRequired, resp.GetDryRunEnrichment().ReasonMode)
		// check prompts
		require.Len(t, resp.GetDryRunEnrichment().ReasonPrompts, 2)
		require.Contains(t, resp.GetDryRunEnrichment().ReasonPrompts, globalPromptRole1.GetOptions().RequestPrompt)
		require.Contains(t, resp.GetDryRunEnrichment().ReasonPrompts, globalPromptRole2.GetOptions().RequestPrompt)
	})

	t.Run("requesting-role-prompts-sorted-and-duplicated", func(t *testing.T) {
		globalPromptRole1 := testCreateRole(t, testTLSServer, "prompt-role-1", func(spec *types.RoleSpecV6) {
			spec.Options.RequestPrompt = "C test prompt"
		})
		globalPromptRole2 := testCreateRole(t, testTLSServer, "prompt-role-2", func(spec *types.RoleSpecV6) {
			spec.Options.RequestPrompt = "A test prompt"
		})
		globalPromptRole3 := testCreateRole(t, testTLSServer, "prompt-role-3", func(spec *types.RoleSpecV6) {
			spec.Options.RequestPrompt = "B test prompt"
		})
		globalPromptRole4 := testCreateRole(t, testTLSServer, "prompt-role-4", func(spec *types.RoleSpecV6) {
			spec.Options.RequestPrompt = "B test prompt"
		})
		globalPromptRole5 := testCreateRole(t, testTLSServer, "prompt-role-5", func(spec *types.RoleSpecV6) {
			spec.Options.RequestPrompt = "C test prompt"
		})

		requester, requesterClient := testCreateUserWithRoles(t, testTLSServer, "requester",
			someRoleRequesterRole.GetName(),
			globalPromptRole1.GetName(),
			globalPromptRole2.GetName(),
			globalPromptRole3.GetName(),
			globalPromptRole4.GetName(),
			globalPromptRole5.GetName(),
		)

		dryRunAccessRequest := testNewAccessRequest(t, requester.GetUsername(), someRole.GetName())
		dryRunAccessRequest.SetDryRun(true)

		resp, err := requesterClient.CreateAccessRequestV2(ctx, dryRunAccessRequest)
		require.NoError(t, err)

		require.NotNil(t, resp.GetDryRunEnrichment())
		// check prompts
		require.Len(t, resp.GetDryRunEnrichment().ReasonPrompts, 3)
		require.Equal(t, "A test prompt", resp.GetDryRunEnrichment().ReasonPrompts[0])
		require.Equal(t, "B test prompt", resp.GetDryRunEnrichment().ReasonPrompts[1])
		require.Equal(t, "C test prompt", resp.GetDryRunEnrichment().ReasonPrompts[2])
	})
}

func TestCleanupNotifications(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	fakeClock := clockwork.NewFakeClock()

	authServer, err := authtest.NewAuthServer(authtest.AuthServerConfig{
		Dir:          t.TempDir(),
		Clock:        fakeClock,
		CacheEnabled: true,
	})
	require.NoError(t, err)

	srv, err := authServer.NewTestTLSServer()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, srv.Close()) })

	// Create test users
	users := []string{"user1", "user2"}
	for _, username := range users {
		user, err := types.NewUser(username)
		require.NoError(t, err)
		_, err = srv.Auth().UpsertUser(ctx, user)
		require.NoError(t, err)
	}

	// Create notifications and store their IDs
	type notificationInfo struct {
		id       string
		username string
		isGlobal bool
	}
	var createdNotifications []notificationInfo

	createNotifications := func(username string, count int, expiryDuration time.Duration) {
		for i := range count {
			var id string
			if username != "" {
				notification := newUserNotificationWithExpiry(t, username, fmt.Sprintf("%s-notification-%d", username, i+1), timestamppb.New(fakeClock.Now().Add(expiryDuration)))
				created, err := srv.Auth().CreateUserNotification(ctx, notification)
				require.NoError(t, err)
				id = created.GetMetadata().GetName()
				createdNotifications = append(createdNotifications, notificationInfo{id: id, username: username, isGlobal: false})
			} else {
				notification := newGlobalNotificationWithExpiry(t, fmt.Sprintf("global-notification-%d", i+1), timestamppb.New(fakeClock.Now().Add(expiryDuration)))
				created, err := srv.Auth().CreateGlobalNotification(ctx, notification)
				require.NoError(t, err)
				id = created.GetMetadata().GetName()
				createdNotifications = append(createdNotifications, notificationInfo{id: id, isGlobal: true})
			}
		}
	}

	// Create user notifications
	createNotifications("user1", 2, 5*time.Minute)  // 2 notifications for user1, expiring in 5 minutes
	createNotifications("user1", 2, 10*time.Minute) // 2 notifications for user1, expiring in 10 minutes
	createNotifications("user2", 2, 5*time.Minute)  // 2 notifications for user2, expiring in 5 minutes
	createNotifications("user2", 2, 10*time.Minute) // 2 notifications for user2, expiring in 10 minutes

	// Create global notifications
	createNotifications("", 2, 5*time.Minute)  // 2 global notifications, expiring in 5 minutes
	createNotifications("", 2, 10*time.Minute) // 2 global notifications, expiring in 10 minutes

	// Create notification states for all notifications
	for _, notif := range createdNotifications {
		if !notif.isGlobal {
			_, err := srv.Auth().UpsertUserNotificationState(ctx, notif.username, &notificationsv1.UserNotificationState{
				Spec: &notificationsv1.UserNotificationStateSpec{
					Username:       notif.username,
					NotificationId: notif.id,
				},
				Status: &notificationsv1.UserNotificationStateStatus{
					NotificationState: notificationsv1.NotificationState_NOTIFICATION_STATE_CLICKED,
				},
			})
			require.NoError(t, err)
		} else {
			// For global notifications, create a state for both users
			for _, username := range users {
				_, err := srv.Auth().UpsertUserNotificationState(ctx, username, &notificationsv1.UserNotificationState{
					Spec: &notificationsv1.UserNotificationStateSpec{
						Username:       username,
						NotificationId: notif.id,
					},
					Status: &notificationsv1.UserNotificationStateStatus{
						NotificationState: notificationsv1.NotificationState_NOTIFICATION_STATE_CLICKED,
					},
				})
				require.NoError(t, err)
			}
		}
	}

	verifyNotificationCounts := func(collectT *assert.CollectT, expectedUserNotifsCount, expectedGlobalNotifsCount, expectedStatesCount int) {
		userNotifs, _, err := srv.Auth().Cache.ListUserNotifications(ctx, 20, "")
		assert.NoError(collectT, err)
		assert.Len(collectT, userNotifs, expectedUserNotifsCount)

		globalNotifs, _, err := srv.Auth().Cache.ListGlobalNotifications(ctx, 20, "")
		assert.NoError(collectT, err)
		assert.Len(collectT, globalNotifs, expectedGlobalNotifsCount)

		states, _, err := srv.Auth().ListNotificationStatesForAllUsers(ctx, 20, "")
		assert.NoError(collectT, err)
		assert.Len(collectT, states, expectedStatesCount)
	}

	require.EventuallyWithT(t, func(collectT *assert.CollectT) {
		// Expect 8 user notifications, 4 global notifications, and 16 states.
		verifyNotificationCounts(collectT, 8, 4, 16)
	}, 3*time.Second, 100*time.Millisecond)

	// Advance clock to make half of the notifications expire.
	fakeClock.Advance(7 * time.Minute)
	// Run CleanupNotifications.
	srv.Auth().CleanupNotifications(ctx)

	require.EventuallyWithT(t, func(collectT *assert.CollectT) {
		// Half of each should have been deleted.
		verifyNotificationCounts(collectT, 4, 2, 8)
	}, 3*time.Second, 100*time.Millisecond)

	// Advance clock to make the remaining notifications expire.
	fakeClock.Advance(5 * time.Minute)
	// Run CleanupNotifications again.
	srv.Auth().CleanupNotifications(ctx)

	require.EventuallyWithT(t, func(collectT *assert.CollectT) {
		// No notifications nor states should remain.
		verifyNotificationCounts(collectT, 0, 0, 0)
	}, 3*time.Second, 100*time.Millisecond)
}

func TestCreateAccessListReminderNotifications(t *testing.T) {
	ctx := context.Background()

	modulestest.SetTestModules(t, modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.Identity: {Enabled: true},
			},
		},
	})

	// Setup test auth server
	testServer := newTestTLSServer(t)
	authServer := testServer.Auth()

	testRole, err := types.NewRole("test", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Logins:         []string{"user"},
			ReviewRequests: &types.AccessReviewConditions{},
		},
	})
	require.NoError(t, err)
	_, err = authServer.UpsertRole(ctx, testRole)
	require.NoError(t, err)

	testUsername := "user1"
	user, err := types.NewUser(testUsername)
	require.NoError(t, err)
	user.SetRoles([]string{"test"})
	_, err = authServer.UpsertUser(ctx, user)
	require.NoError(t, err)

	client, err := testServer.NewClient(authtest.TestUser(testUsername))
	require.NoError(t, err)

	// Create access lists with different expiry times
	accessLists := []struct {
		name      string
		dueInDays int
	}{
		// These should trigger a single "due in less than 14 days" notification.
		{name: "al-due-13d", dueInDays: 13},
		{name: "al-due-12d", dueInDays: 12},
		// This should trigger a "due in less than 7 days" notification.
		{name: "al-due-5d", dueInDays: 5},
		// This should trigger a "due in less than 3 days" notification.
		{name: "al-due-2d", dueInDays: 2},
		// This should trigger an "overdue by more than 3 days" notification.
		{name: "al-overdue-4d", dueInDays: -4},
		// This should trigger an "overdue by more than 7 days" notification.
		{name: "al-overdue-10d", dueInDays: -10},
		// This should not trigger a notification.
		{name: "al-due-60d", dueInDays: 60},
		// This should trigger a "due today" notification.
		{name: "al-overdue-today", dueInDays: 0},
	}

	for _, al := range accessLists {
		createAccessList(t, authServer, al.name+"-static",
			withType(accesslist.Static),
			withOwners([]accesslist.Owner{{Name: testUsername}}),
			withNextAuditDate(authServer.GetClock().Now().Add(time.Duration(al.dueInDays)*24*time.Hour)),
		)
		createAccessList(t, authServer, al.name+"-scim",
			withType(accesslist.SCIM),
			withOwners([]accesslist.Owner{{Name: testUsername}}),
			withNextAuditDate(authServer.GetClock().Now().Add(time.Duration(al.dueInDays)*24*time.Hour)),
		)
		createAccessList(t, authServer, al.name,
			withOwners([]accesslist.Owner{{Name: testUsername}}),
			withNextAuditDate(authServer.GetClock().Now().Add(time.Duration(al.dueInDays)*24*time.Hour)),
		)
	}

	// Run CreateAccessListReminderNotifications()
	authServer.CreateAccessListReminderNotifications(ctx)

	// Check notifications
	resp, err := client.ListNotifications(ctx, &notificationsv1.ListNotificationsRequest{
		PageSize: 50,
	})
	require.NoError(t, err)
	require.Len(t, resp.Notifications, 6)

	// Run CreateAccessListReminderNotifications() again to verify no duplicates are created if it's run again.
	authServer.CreateAccessListReminderNotifications(ctx)

	// Check notifications again, counts should remain the same.
	resp, err = client.ListNotifications(ctx, &notificationsv1.ListNotificationsRequest{
		PageSize: 50,
	})
	require.NoError(t, err)
	require.Len(t, resp.Notifications, 6)
}

type createAccessListOptions struct {
	typ           accesslist.Type
	nextAuditDate time.Time
	owners        []accesslist.Owner
}

type createAccessListOpt func(*createAccessListOptions)

func withType(typ accesslist.Type) createAccessListOpt {
	return func(o *createAccessListOptions) {
		o.typ = typ
	}
}

func withNextAuditDate(nextAuditDate time.Time) createAccessListOpt {
	return func(o *createAccessListOptions) {
		o.nextAuditDate = nextAuditDate
	}
}

func withOwners(owners []accesslist.Owner) createAccessListOpt {
	return func(o *createAccessListOptions) {
		o.owners = owners
	}
}

func createAccessList(t *testing.T, authServer *auth.Server, name string, opts ...createAccessListOpt) {
	t.Helper()
	ctx := t.Context()

	options := createAccessListOptions{}
	for _, o := range opts {
		o(&options)
	}

	al, err := accesslist.NewAccessList(header.Metadata{
		Name: name,
	}, accesslist.Spec{
		Type:        options.typ,
		Title:       fmt.Sprintf("Test Access List %s", name),
		Description: fmt.Sprintf("Test Access List %s description", name),
		Owners:      options.owners,
		Audit: accesslist.Audit{
			NextAuditDate: options.nextAuditDate,
			Recurrence:    accesslist.Recurrence{},
			Notifications: accesslist.Notifications{},
		},
		Grants: accesslist.Grants{
			Roles: []string{"grant"},
		},
	})
	require.NoError(t, err)

	_, err = authServer.UpsertAccessList(ctx, al)
	require.NoError(t, err)
}

func TestServer_GetAnonymizationKey(t *testing.T) {
	tests := []struct {
		name        string
		testModules modulestest.Modules
		license     *license.License
		want        string
		errCheck    require.ErrorAssertionFunc
	}{
		{
			name: "returns CloudAnonymizationKey if present",
			testModules: modulestest.Modules{
				TestFeatures: modules.Features{CloudAnonymizationKey: []byte("cloud-key")},
			},
			license: &license.License{
				AnonymizationKey: []byte("license-key"),
			},
			want:     "cloud-key",
			errCheck: require.NoError,
		},
		{
			name:        "Returns license AnonymizationKey if no Cloud Key is present",
			testModules: modulestest.Modules{},
			license: &license.License{
				AnonymizationKey: []byte("license-key"),
			},
			want:     "license-key",
			errCheck: require.NoError,
		},
		{
			name:        "Returns clusterID if no cloud key nor license key is present",
			testModules: modulestest.Modules{},
			license:     &license.License{},
			want:        "cluster-id",
			errCheck:    require.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testAuthServer, err := authtest.NewAuthServer(authtest.AuthServerConfig{
				Dir:       t.TempDir(),
				Clock:     clockwork.NewFakeClock(),
				ClusterID: "cluster-id",
			})
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, testAuthServer.Close()) })

			testTLSServer, err := testAuthServer.NewTestTLSServer()
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, testTLSServer.Close()) })

			modulestest.SetTestModules(t, tt.testModules)

			testTLSServer.AuthServer.AuthServer.SetLicense(tt.license)

			got, err := testTLSServer.AuthServer.AuthServer.GetAnonymizationKey(context.Background())
			tt.errCheck(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func newUserNotificationWithExpiry(t *testing.T, username string, title string, expires *timestamppb.Timestamp) *notificationsv1.Notification {
	t.Helper()

	notification := notificationsv1.Notification{
		SubKind: "test-subkind",
		Spec: &notificationsv1.NotificationSpec{
			Username: username,
		},
		Metadata: &headerv1.Metadata{
			Expires: expires,
			Labels: map[string]string{
				types.NotificationTitleLabel: title,
			},
		},
	}

	return &notification
}

func newGlobalNotificationWithExpiry(t *testing.T, title string, expires *timestamppb.Timestamp) *notificationsv1.GlobalNotification {
	t.Helper()

	notification := notificationsv1.GlobalNotification{
		Spec: &notificationsv1.GlobalNotificationSpec{
			Matcher: &notificationsv1.GlobalNotificationSpec_All{
				All: true,
			},
			Notification: &notificationsv1.Notification{
				SubKind: "test-subkind",
				Spec:    &notificationsv1.NotificationSpec{},
				Metadata: &headerv1.Metadata{
					Expires: expires,
					Labels: map[string]string{
						types.NotificationTitleLabel: title,
					},
				},
			},
		},
	}

	return &notification
}

// TestServerHostnameSanitization tests that persisting servers with
// "invalid" hostnames results in the hostname being sanitized and the
// illegal name being placed in a label.
func TestServerHostnameSanitization(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv, err := authtest.NewAuthServer(authtest.AuthServerConfig{Dir: t.TempDir()})
	require.NoError(t, err)

	cases := []struct {
		name            string
		hostname        string
		addr            string
		invalidHostname bool
		invalidAddr     bool
	}{
		{
			name:     "valid dns hostname",
			hostname: "llama.example.com",
		},
		{
			name:     "valid friendly hostname",
			hostname: "llama",
		},
		{
			name:     "uuid hostname",
			hostname: uuid.NewString(),
		},
		{
			name:     "uuid dns hostname",
			hostname: uuid.NewString() + ".example.com",
		},
		{
			name:     "valid dns hostname with multi-dots",
			hostname: "llama..example.com",
		},
		{
			name:            "empty hostname",
			hostname:        "",
			invalidHostname: true,
		},
		{
			name:            "exceptionally long hostname",
			hostname:        strings.Repeat("a", auth.ServerHostnameMaxLen*2),
			invalidHostname: true,
		},
		{
			name:            "spaces in hostname",
			hostname:        "the quick brown fox jumps over the lazy dog",
			invalidHostname: true,
		},
		{
			name:            "invalid addr",
			hostname:        "..",
			addr:            "..:2345",
			invalidHostname: true,
			invalidAddr:     true,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			for _, subKind := range []string{types.KindNode, types.SubKindOpenSSHNode} {
				t.Run(subKind, func(t *testing.T) {
					server := &types.ServerV2{
						Kind:    types.KindNode,
						SubKind: subKind,
						Metadata: types.Metadata{
							Name: uuid.NewString(),
						},
						Spec: types.ServerSpecV2{
							Hostname: test.hostname,
							Addr:     cmp.Or(test.addr, "abcd:1234"),
						},
					}
					if subKind == types.KindNode {
						server.SubKind = ""
					}

					_, err = srv.AuthServer.UpsertNode(ctx, server)
					require.NoError(t, err)

					replacedValue, _ := server.GetLabel("teleport.internal/invalid-hostname")
					if !test.invalidHostname {
						assert.Equal(t, test.hostname, server.GetHostname())
						assert.Empty(t, replacedValue)
						return
					}

					assert.Equal(t, test.hostname, replacedValue)
					switch subKind {
					case types.SubKindOpenSSHNode:
						host, _, err := net.SplitHostPort(server.GetAddr())
						assert.NoError(t, err)
						if !test.invalidAddr {
							// If the address is valid, then the hostname should be set
							// to the host of the addr field.
							assert.Equal(t, host, server.GetHostname())
						} else {
							// If the address is not valid, then the hostname should be
							// set to a UUID.
							assert.NotEqual(t, host, server.GetHostname())
							assert.NotEqual(t, server.GetName(), server.GetHostname())

							_, err := uuid.Parse(server.GetHostname())
							require.NoError(t, err)
						}
					default:
						assert.Equal(t, server.GetName(), server.GetHostname())
					}
				})
			}
		})
	}
}

func TestValidServerHostname(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		hostname string
		want     bool
	}{
		{
			name:     "valid dns hostname",
			hostname: "llama.example.com",
			want:     true,
		},
		{
			name:     "valid friendly hostname",
			hostname: "llama",
			want:     true,
		},
		{
			name:     "uuid hostname",
			hostname: uuid.NewString(),
			want:     true,
		},
		{
			name:     "valid hostname with multi-dashes",
			hostname: "llama--example.com",
			want:     true,
		},
		{
			name:     "valid hostname with multi-dots",
			hostname: "llama..example.com",
			want:     true,
		},
		{
			name:     "valid hostname with numbers",
			hostname: "llama9",
			want:     true,
		},
		{
			name:     "hostname with invalid characters",
			hostname: "llama?!$",
			want:     false,
		},
		{
			name:     "super long hostname",
			hostname: strings.Repeat("a", auth.ServerHostnameMaxLen*2),
			want:     false,
		},
		{
			name:     "hostname with spaces",
			hostname: "the quick brown fox jumps over the lazy dog",
			want:     false,
		},
		{
			name:     "hostname with ;",
			hostname: "llama;example.com",
			want:     false,
		},
		{
			name:     "empty hostname",
			hostname: "",
			want:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := auth.ValidServerHostname(tt.hostname)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestCreateAuthPreference(t *testing.T) {
	cases := []struct {
		name       string
		modules    *modulestest.Modules
		preference func(p types.AuthPreference)
		assertion  func(t *testing.T, created types.AuthPreference, err error)
	}{
		{
			name: "creation prevented when hardware key policy is set in open source",
			preference: func(p types.AuthPreference) {
				pp := p.(*types.AuthPreferenceV2)
				pp.Spec.RequireMFAType = types.RequireMFAType_HARDWARE_KEY_PIN
			},
			assertion: func(t *testing.T, created types.AuthPreference, err error) {
				assert.Nil(t, created)
				require.True(t, trace.IsAccessDenied(err), "got (%v), expected hardware key policy to be rejected in OSS", err)
			},
		},
		{
			name:    "creation allowed when hardware key policy is set in enterprise",
			modules: &modulestest.Modules{TestBuildType: modules.BuildEnterprise},
			preference: func(p types.AuthPreference) {
				pp := p.(*types.AuthPreferenceV2)
				pp.Spec.RequireMFAType = types.RequireMFAType_HARDWARE_KEY_PIN
			},
			assertion: func(t *testing.T, created types.AuthPreference, err error) {
				require.NoError(t, err, "got (%v), expected auth role to create auth mutator", err)
				require.NotNil(t, created)
			},
		},
		{
			name: "creation prevented when hardware key policy is set in open source",
			preference: func(p types.AuthPreference) {
				p.SetDeviceTrust(&types.DeviceTrust{
					Mode: constants.DeviceTrustModeRequired,
				})
			},
			assertion: func(t *testing.T, created types.AuthPreference, err error) {
				assert.Nil(t, created)
				require.True(t, trace.IsBadParameter(err), "got (%v), expected device trust mode conflict to prevent creation", err)
			},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if test.modules != nil {
				modulestest.SetTestModules(t, *test.modules)
			}

			bk, err := memory.New(memory.Config{})
			require.NoError(t, err)

			clusterName, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
				ClusterName: "test.localhost",
			})

			require.NoError(t, err)

			clusterConfigService, err := local.NewClusterConfigurationService(bk)
			require.NoError(t, err)

			server, err := auth.NewServer(&auth.InitConfig{
				DataDir:                t.TempDir(),
				Backend:                bk,
				ClusterName:            clusterName,
				VersionStorage:         authtest.NewFakeTeleportVersion(),
				Authority:              testauthority.New(),
				Emitter:                &eventstest.MockRecorderEmitter{},
				ClusterConfiguration:   clusterConfigService,
				SkipPeriodicOperations: true,
			})
			require.NoError(t, err)

			pref := types.DefaultAuthPreference()
			if test.preference != nil {
				test.preference(pref)
			}

			created, err := server.CreateAuthPreference(context.Background(), pref)
			test.assertion(t, created, err)
		})
	}
}
