package auth_test

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v3"
	josejwt "github.com/go-jose/go-jose/v3/jwt"
	"github.com/gogo/protobuf/proto"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zitadel/oidc/v3/example/server/storage"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"golang.org/x/oauth2"

	"github.com/gravitational/teleport/api/constants"
	loginrulepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/common"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/api/utils/keys"
	eauth "github.com/gravitational/teleport/e/lib/auth"
	"github.com/gravitational/teleport/e/lib/auth/oidctest"
	"github.com/gravitational/teleport/e/lib/licensefile"
	"github.com/gravitational/teleport/e/lib/loginrule"
	loginrulestorage "github.com/gravitational/teleport/e/lib/loginrule/storage"
	emodules "github.com/gravitational/teleport/e/tool/modules"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/authtest"
	authority "github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/clocki"
)

type oidcSuiteOpts struct {
	clock             clocki.FakeClock
	pkceMode          string
	requestObjectMode constants.OIDCRequestObjectMode
	signerFactory     eauth.JWTSignerFactory
	licenseChecker    eauth.LicenseChecker
}

func overridePKCEMode(mode string) func(*oidcSuiteOpts) {
	return func(opts *oidcSuiteOpts) {
		opts.pkceMode = mode
	}
}

func overrideClock(clock clocki.FakeClock) func(*oidcSuiteOpts) {
	return func(opts *oidcSuiteOpts) {
		opts.clock = clock
	}
}

func withLicenseChecker(m eauth.LicenseChecker) func(*oidcSuiteOpts) {
	return func(opts *oidcSuiteOpts) {
		opts.licenseChecker = m
	}
}

// alwaysValidLicense is an [eauth.LicenseChecker] that reports the license as never disabled.
type alwaysValidLicense struct{}

func (alwaysValidLicense) IsDisabled() bool { return false }

// disabledLicenseChecker always reports the license as disabled.
type disabledLicenseChecker struct{}

func (d *disabledLicenseChecker) IsDisabled() bool { return true }

type OIDCSuite struct {
	authServer       *auth.Server
	backend          backend.Backend
	clock            clocki.FakeClock
	oidcService      *eauth.OIDCAuthService
	emitter          *eventstest.MockRecorderEmitter
	idpServer        *oidctest.IdPServer
	connector        *types.OIDCConnectorV3
	oidcIDPPublicKey crypto.PublicKey
}

func newECDSAKey(clusterName string) (*jwt.Key, crypto.Signer, error) {
	ecdsaKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	jwtKey, err := jwt.New(&jwt.Config{
		PublicKey:   ecdsaKey.Public(),
		PrivateKey:  ecdsaKey,
		ClusterName: clusterName,
	})
	if err != nil {
		return nil, nil, err
	}
	return jwtKey, ecdsaKey, err
}

func newOIDCSuite(t *testing.T, idpServer *oidctest.IdPServer, opts ...func(*oidcSuiteOpts)) *OIDCSuite {
	ctx := context.Background()

	o := oidcSuiteOpts{
		clock:             clockwork.NewFakeClock(),
		pkceMode:          "disabled",
		requestObjectMode: constants.OIDCRequestObjectModeUnknown,
		licenseChecker:    alwaysValidLicense{},
	}

	for _, opt := range opts {
		opt(&o)
	}

	var err error
	bk, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   o.clock,
	})
	require.NoError(t, err)

	clusterName, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: "me.localhost",
	})
	require.NoError(t, err)

	keygen, err := authority.NewKeygen(modules.BuildEnterprise, o.clock.Now)
	require.NoError(t, err)

	authConfig := &auth.InitConfig{
		VersionStorage:         authtest.NewFakeTeleportVersion(),
		ClusterName:            clusterName,
		Backend:                bk,
		Authority:              keygen,
		SkipPeriodicOperations: true,
		Clock:                  o.clock,
		HostUUID:               uuid.NewString(),
	}
	authServer, err := auth.NewServer(authConfig)
	require.NoError(t, err)

	emitter := &eventstest.MockRecorderEmitter{}

	connector := &types.OIDCConnectorV3{
		Kind:    types.KindOIDCConnector,
		Version: types.V3,
		Metadata: types.Metadata{
			Name: "test-connector",
		},
		Spec: types.OIDCConnectorSpecV3{
			IssuerURL:    idpServer.URL,
			ClientID:     "test",
			ClientSecret: "secret",
			Provider:     "test",
			PKCEMode:     o.pkceMode,
			Display:      "test",
			Scope:        []string{oidc.ScopeOpenID, oidc.ScopeEmail, oidc.ScopeProfile},
			ClaimsToRoles: []types.ClaimMapping{
				{
					Claim: "groups",
					Value: "access",
					Roles: []string{"access"},
				},
			},
			RedirectURLs: wrappers.Strings{
				idpServer.URL + "/proxy/oidc/callback",
			},
			RequestObjectMode: string(o.requestObjectMode),
		},
	}

	c, err := authServer.CreateOIDCConnector(ctx, connector)
	require.NoError(t, err)

	require.NoError(t, authServer.SetClusterName(
		&types.ClusterNameV2{
			Kind:     types.KindClusterName,
			Version:  types.V2,
			Metadata: types.Metadata{},
			Spec: types.ClusterNameSpecV2{
				ClusterName: "test.example.com",
				ClusterID:   "test",
			},
		},
	))

	_, err = authServer.CreateClusterNetworkingConfig(ctx, types.DefaultClusterNetworkingConfig())
	require.NoError(t, err)

	_, err = authServer.CreateAuthPreference(ctx, types.DefaultAuthPreference())
	require.NoError(t, err)

	lockWatcher, err := services.NewLockWatcher(ctx, services.LockWatcherConfig{
		LockGetter: authServer,
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Client:    authServer,
			Component: "test",
		},
	})
	require.NoError(t, err)

	authServer.SetLockWatcher(lockWatcher)

	var keySet types.CAKeySet
	sshKeyPair, err := authServer.GetKeyStore().NewSSHKeyPair(ctx, cryptosuites.UserCASSH)
	require.NoError(t, err)
	keySet.SSH = append(keySet.SSH, sshKeyPair)

	tlsKeyPair, err := authServer.GetKeyStore().NewTLSKeyPair(ctx, "test.example.com", cryptosuites.UserCASSH)
	require.NoError(t, err)
	keySet.TLS = append(keySet.TLS, tlsKeyPair)

	ca, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        types.UserCA,
		ClusterName: "test.example.com",
		ActiveKeys:  keySet,
	})
	require.NoError(t, err)

	err = authServer.CreateCertAuthority(ctx, ca)
	require.NoError(t, err)

	jwtKeyPair, err := authServer.GetKeyStore().NewJWTKeyPair(ctx, cryptosuites.OIDCIdPCAJWT)
	require.NoError(t, err)

	oidcIDPPublicKey, err := keys.ParsePublicKey(jwtKeyPair.PublicKey)
	require.NoError(t, err)

	jwtCA, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        types.OIDCIdPCA,
		ClusterName: "test.example.com",
		ActiveKeys: types.CAKeySet{
			JWT: []*types.JWTKeyPair{
				jwtKeyPair,
			},
		},
	})
	require.NoError(t, err)

	err = authServer.CreateCertAuthority(ctx, jwtCA)
	require.NoError(t, err)

	oidcService, err := eauth.NewOIDCAuthService(&eauth.OIDCAuthServiceConfig{
		Auth:           authServer,
		Emitter:        emitter,
		Client:         idpServer.Client(),
		SignerFactory:  o.signerFactory,
		LicenseChecker: o.licenseChecker,
	})
	require.NoError(t, err)
	authServer.SetOIDCService(oidcService)

	_, err = authServer.CreateRole(ctx, services.NewPresetAccessRole())
	require.NoError(t, err)

	return &OIDCSuite{
		authServer:       authServer,
		backend:          bk,
		clock:            o.clock,
		oidcService:      oidcService,
		emitter:          emitter,
		idpServer:        idpServer,
		connector:        c.(*types.OIDCConnectorV3),
		oidcIDPPublicKey: oidcIDPPublicKey,
	}
}

func (s *OIDCSuite) authenticateUser(ctx context.Context, user string, req types.OIDCAuthRequest) (string, *authclient.OIDCAuthResponse, error) {
	authRequest, err := s.oidcService.CreateOIDCAuthRequest(ctx, req)
	if err != nil {
		return "", nil, err
	}

	codeChallenge := oauth2.S256ChallengeFromVerifier(req.PkceVerifier)
	_, err = s.idpServer.Authorize(
		ctx,
		&oidc.AuthRequest{
			Scopes:              []string{oidc.ScopeOpenID, oidc.ScopeEmail, oidc.ScopeProfile},
			ClientID:            s.connector.GetClientID(),
			RedirectURI:         s.connector.GetRedirectURLs()[0],
			State:               authRequest.StateToken,
			Display:             "none",
			CodeChallenge:       codeChallenge,
			CodeChallengeMethod: "S256",
		},
		user,
	)
	if err != nil {
		return "", nil, err
	}

	resp, err := s.oidcService.ValidateOIDCAuthCallback(ctx, url.Values{
		"code":  []string{"test"},
		"state": []string{authRequest.StateToken},
	})
	if err != nil {
		return "", nil, err
	}

	return authRequest.StateToken, resp, nil
}

func (s *OIDCSuite) authenticateUserWithMFA(ctx context.Context, user string, sd *services.SSOMFASessionData, req types.OIDCAuthRequest) (*authclient.OIDCAuthResponse, error) {
	authRequest, err := s.oidcService.CreateOIDCAuthRequestForMFA(ctx, req)
	if err != nil {
		return nil, err
	}

	sd.RequestID = authRequest.StateToken
	if err := s.authServer.UpsertMFASessionData(ctx, sd); err != nil {
		return nil, err
	}

	connectorCopy := *s.connector
	if err := connectorCopy.WithMFASettings(); err != nil {
		return nil, err
	}

	_, err = s.idpServer.Authorize(
		ctx,
		&oidc.AuthRequest{
			Scopes:      []string{oidc.ScopeOpenID, oidc.ScopeEmail, oidc.ScopeProfile},
			ClientID:    connectorCopy.GetClientID(),
			RedirectURI: connectorCopy.GetRedirectURLs()[0],
			State:       authRequest.StateToken,
			Display:     "none",
		},
		user,
	)
	if err != nil {
		return nil, err
	}

	resp, err := s.oidcService.ValidateOIDCAuthCallback(ctx, url.Values{
		"code":  []string{"test"},
		"state": []string{authRequest.StateToken},
	})
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func TestCreateOIDCAuthRequest(t *testing.T) {
	t.Parallel()
	idp, err := oidctest.NewIdPServer()
	require.NoError(t, err)
	t.Cleanup(idp.Close)

	suite := newOIDCSuite(t, idp)

	tests := []struct {
		name      string
		req       types.OIDCAuthRequest
		assertion func(t *testing.T, req *types.OIDCAuthRequest, err error)
	}{
		{
			name: "test flow",
			req: types.OIDCAuthRequest{
				ConnectorID:   "test-flow-connector",
				SSOTestFlow:   true,
				ConnectorSpec: &suite.connector.Spec,
			},
			assertion: func(t *testing.T, req *types.OIDCAuthRequest, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, req.RedirectURL)
			},
		},
		{
			name: "test flow custom client redirect",
			req: types.OIDCAuthRequest{
				ConnectorID:       "test-flow-connector",
				SSOTestFlow:       true,
				ConnectorSpec:     &suite.connector.Spec,
				ClientRedirectURL: "http://test.example.com/callback?secret_key=test",
			},
			assertion: func(t *testing.T, req *types.OIDCAuthRequest, err error) {
				require.ErrorContains(t, err, "custom client redirect URLs are not allowed in SSO test")
				require.Nil(t, req)
			},
		},
		{
			name: "oidc login",
			req: types.OIDCAuthRequest{
				ConnectorID: suite.connector.GetName(),
			},
			assertion: func(t *testing.T, req *types.OIDCAuthRequest, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, req.RedirectURL)
			},
		},
		{
			name: "invalid connector",
			req: types.OIDCAuthRequest{
				ConnectorID: "fake-connector",
			},
			assertion: func(t *testing.T, req *types.OIDCAuthRequest, err error) {
				require.ErrorContains(t, err, "OpenID connector 'fake-connector' is not configured")
				require.Nil(t, req)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resp, err := suite.oidcService.CreateOIDCAuthRequest(context.Background(), test.req)
			test.assertion(t, resp, err)
		})
	}
}

func TestValidateOIDCAuthCallback(t *testing.T) {
	t.Parallel()

	idp, err := oidctest.NewIdPServer()
	require.NoError(t, err)
	t.Cleanup(idp.Close)

	suite := newOIDCSuite(t, idp, overridePKCEMode("enabled"))

	ctx := t.Context()

	// Constrain the TTL so role applies.
	const accessRoleMaxSessionTTL = 30 * time.Second
	accessRole, err := suite.authServer.GetRole(ctx, "access")
	require.NoError(t, err)

	accessRoleOptions := accessRole.GetOptions()
	accessRoleOptions.MaxSessionTTL = types.Duration(accessRoleMaxSessionTTL)
	accessRole.SetOptions(accessRoleOptions)

	_, err = suite.authServer.UpdateRole(ctx, accessRole)
	require.NoError(t, err)

	tests := []struct {
		name           string
		userID         string
		createSession  bool
		testFlow       bool
		noCodeVerifier bool
		q              url.Values
		assertion      func(t *testing.T, resp *authclient.OIDCAuthResponse, err error)
		setup          func(t *testing.T, suite *OIDCSuite)
	}{
		{
			name:   "successful authentication",
			userID: "id1",
			assertion: func(t *testing.T, resp *authclient.OIDCAuthResponse, err error) {
				require.NoError(t, err)
				require.NotNil(t, resp)
				require.Nil(t, resp.Session)

				u, err := suite.authServer.GetUser(context.Background(), "test-user@example.com", false)
				require.NoError(t, err)
				require.NotNil(t, u)
				require.NotNil(t, u.GetCreatedBy())
				require.Equal(t, suite.connector.GetName(), u.GetCreatedBy().Connector.ID)
			},
		},
		{
			name:     "test flow",
			userID:   "id3",
			testFlow: true,
			assertion: func(t *testing.T, resp *authclient.OIDCAuthResponse, err error) {
				require.NoError(t, err)
				require.NotNil(t, resp)
				require.Nil(t, resp.Session)

				u, err := suite.authServer.GetUser(context.Background(), "test-user3@example.com", false)
				require.True(t, trace.IsNotFound(err))
				require.Nil(t, u)
			},
		},
		{
			name:          "successful authentication with session",
			userID:        "id3",
			createSession: true,
			assertion: func(t *testing.T, resp *authclient.OIDCAuthResponse, err error) {
				require.NoError(t, err)
				require.NotNil(t, resp)
				require.NotNil(t, resp.Session)

				u, err := suite.authServer.GetUser(context.Background(), "test-user3@example.com", false)
				require.NoError(t, err)
				require.NotNil(t, u)
				require.NotNil(t, u.GetCreatedBy())
				require.Equal(t, suite.connector.GetName(), u.GetCreatedBy().Connector.ID)
			},
		},
		{
			name:   "email not verified",
			userID: "id2",
			assertion: func(t *testing.T, resp *authclient.OIDCAuthResponse, err error) {
				require.ErrorContains(t, err, "email not verified by OIDC provider")
				require.Nil(t, resp)
				_, err = suite.authServer.GetUser(ctx, "test-user2@example.com", false)
				require.ErrorAs(t, err, new(*trace.NotFoundError))
			},
		},
		{
			name:          "no claims to roles without access list roles",
			userID:        "no-groups",
			createSession: true,
			assertion: func(t *testing.T, resp *authclient.OIDCAuthResponse, err error) {
				require.ErrorIs(t, err, eauth.ErrOIDCNoRoles)
				require.Nil(t, resp)
				// User isn't persisted.
				_, err = suite.authServer.GetUser(ctx, "test-user2@example.com", false)
				require.ErrorAs(t, err, new(*trace.NotFoundError))
				_, err = suite.authServer.GetUserLoginState(ctx, "test-user2@example.com")
				require.ErrorAs(t, err, new(*trace.NotFoundError))
			},
		},
		{
			name:          "no claims to roles without access list roles",
			userID:        "no-groups",
			createSession: true,
			testFlow:      true,
			assertion: func(t *testing.T, resp *authclient.OIDCAuthResponse, err error) {
				require.ErrorIs(t, err, eauth.ErrOIDCNoRoles)
				require.Nil(t, resp)
			},
		},
		{
			name:          "no claims to roles with access list roles",
			userID:        "no-groups",
			createSession: true,
			setup: func(t *testing.T, suite *OIDCSuite) {
				ctx := t.Context()

				username := "test-user4@example.com"

				acl, err := accesslist.NewAccessList(
					header.Metadata{Name: "oidc-test-access-list"},
					accesslist.Spec{
						Title:              "title",
						Owners:             []accesslist.Owner{{Name: "test-user5"}},
						Audit:              accesslist.Audit{NextAuditDate: time.Now().Add(24 * time.Hour)},
						MembershipRequires: accesslist.Requires{},
						OwnershipRequires:  accesslist.Requires{},
						Grants:             accesslist.Grants{Roles: []string{"access"}},
					},
				)
				require.NoError(t, err)

				member, err := accesslist.NewAccessListMember(
					header.Metadata{Name: username},
					accesslist.AccessListMemberSpec{
						AccessList: acl.GetName(),
						Name:       username,
						Joined:     time.Now(),
						Expires:    time.Now().Add(24 * time.Hour),
						Reason:     "test",
						AddedBy:    "test-user5",
					},
				)
				require.NoError(t, err)

				_, _, err = suite.authServer.UpsertAccessListWithMembers(
					ctx,
					acl,
					[]*accesslist.AccessListMember{member},
				)
				require.NoError(t, err)

				t.Cleanup(func() {
					suite.authServer.DeleteAccessList(ctx, acl.GetName())
				})
			},
			assertion: func(t *testing.T, resp *authclient.OIDCAuthResponse, err error) {
				require.NoError(t, err)
				require.NotNil(t, resp)
				require.WithinDuration(t, suite.clock.Now().Add(accessRoleMaxSessionTTL), resp.Session.Expiry(), 5*time.Second)
			},
		},
		{
			name:           "errors with no pkce code verifier if pkce enabled",
			userID:         "id3",
			noCodeVerifier: true,
			assertion: func(t *testing.T, resp *authclient.OIDCAuthResponse, err error) {
				require.ErrorContains(t, err, "pkce code verifier must not be empty")
				require.Nil(t, resp)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()

			codeVerifier := "123"

			req := types.OIDCAuthRequest{
				ConnectorID:      suite.connector.GetName(),
				CreateWebSession: test.createSession,
				CheckUser:        true,
				PkceVerifier:     codeVerifier,
				SSOTestFlow:      test.testFlow,
			}
			if test.testFlow {
				req.ConnectorSpec = &suite.connector.Spec
			}
			if test.noCodeVerifier {
				req.PkceVerifier = ""
			}

			if test.setup != nil {
				test.setup(t, suite)
			}

			_, resp, err := suite.authenticateUser(ctx, test.userID, req)
			test.assertion(t, resp, err)
		})
	}
}

func TestOIDCUserCreation(t *testing.T) {
	t.Parallel()

	idp, err := oidctest.NewIdPServer()
	require.NoError(t, err)
	t.Cleanup(idp.Close)

	suite := newOIDCSuite(t, idp)
	ctx := context.Background()

	_, _, err = suite.authenticateUser(ctx, "id1", types.OIDCAuthRequest{
		ConnectorID: suite.connector.GetName(),
		CheckUser:   true,
		CertTTL:     time.Minute,
	})
	require.NoError(t, err)

	u, err := suite.authServer.GetUser(ctx, "test-user@example.com", false)
	require.NoError(t, err)

	_, _, err = suite.authenticateUser(ctx, "id1", types.OIDCAuthRequest{
		ConnectorID: suite.connector.GetName(),
		CheckUser:   true,
		CertTTL:     time.Minute,
	})
	require.NoError(t, err)

	u2, err := suite.authServer.GetUser(ctx, "test-user@example.com", false)
	require.NoError(t, err)

	require.NotEqual(t, u.GetRevision(), u2.GetRevision())
	require.Equal(t, u.GetName(), u2.GetName())

	// Advance time 2 minutes, the user should be gone.
	suite.clock.Advance(2 * time.Minute)
	_, err = suite.authServer.GetUser(ctx, "test-user@example.com", false)
	require.Error(t, err)
}

// TestAccessListOnlyRolesConstrainsExpiry verifies that user authenticating with
// only Access List roles has expiry constrained by Access List expiry.
func TestAccessListOnlyRolesConstrainsExpiry(t *testing.T) {
	t.Parallel()

	idp, err := oidctest.NewIdPServer()
	require.NoError(t, err)
	t.Cleanup(idp.Close)

	suite := newOIDCSuite(t, idp)
	ctx := context.Background()

	acl, err := accesslist.NewAccessList(
		header.Metadata{Name: "oidc-test-access-list"},
		accesslist.Spec{
			Title:              "title",
			Owners:             []accesslist.Owner{{Name: "test-user5"}},
			Audit:              accesslist.Audit{NextAuditDate: time.Now().Add(24 * time.Hour)},
			MembershipRequires: accesslist.Requires{},
			OwnershipRequires:  accesslist.Requires{},
			Grants:             accesslist.Grants{Roles: []string{"access"}},
		},
	)
	require.NoError(t, err)

	member, err := accesslist.NewAccessListMember(
		header.Metadata{Name: "test-user4@example.com"},
		accesslist.AccessListMemberSpec{
			AccessList: acl.GetName(),
			Name:       "test-user4@example.com",
			Joined:     time.Now(),
			Expires:    time.Now().Add(24 * time.Hour),
			Reason:     "test",
			AddedBy:    "test-user5",
		},
	)
	require.NoError(t, err)

	_, _, err = suite.authServer.UpsertAccessListWithMembers(
		ctx,
		acl,
		[]*accesslist.AccessListMember{member},
	)
	require.NoError(t, err)

	// Update access role with 30s max TTL.
	accessRole, err := suite.authServer.GetRole(ctx, "access")
	require.NoError(t, err)
	options := accessRole.GetOptions()
	options.MaxSessionTTL = types.Duration(30 * time.Second)
	accessRole.SetOptions(options)
	_, err = suite.authServer.UpdateRole(ctx, accessRole)
	require.NoError(t, err)

	_, _, err = suite.authenticateUser(ctx, "no-groups", types.OIDCAuthRequest{
		ConnectorID: suite.connector.GetName(),
		CheckUser:   true,
		CertTTL:     10 * time.Minute,
	})
	require.NoError(t, err)

	u, err := suite.authServer.GetUser(ctx, "test-user4@example.com", false)
	require.NoError(t, err)

	_, _, err = suite.authenticateUser(ctx, "no-groups", types.OIDCAuthRequest{
		ConnectorID: suite.connector.GetName(),
		CheckUser:   true,
		CertTTL:     time.Minute,
	})
	require.NoError(t, err)

	u2, err := suite.authServer.GetUser(ctx, "test-user4@example.com", false)
	require.NoError(t, err)

	require.NotEqual(t, u.GetRevision(), u2.GetRevision())
	require.Equal(t, u.GetName(), u2.GetName())
	// Constrained down to 30s from Access List role.
	require.WithinDuration(t, suite.clock.Now().Add(30*time.Second), u2.Expiry(), 5*time.Second)

	// Advance time 2 minutes, the user should be gone.
	suite.clock.Advance(2 * time.Minute)
	_, err = suite.authServer.GetUser(ctx, "test-user4@example.com", false)
	require.Error(t, err)
}

// TestUserInfoBlockHTTP ensures that an insecure userinfo endpoint is
// not consulted for additional claims. For these users, the only claims
// consumed are the ones provided already within the token.
func TestOIDCBlockHTTPUserInfo(t *testing.T) {
	t.Parallel()

	idp, err := oidctest.NewIdPServer(oidctest.WithAllowInsecure())
	require.NoError(t, err)
	t.Cleanup(idp.Close)

	suite := newOIDCSuite(t, idp)

	ctx := context.Background()

	_, _, err = suite.authenticateUser(ctx, "id1", types.OIDCAuthRequest{
		ConnectorID: suite.connector.GetName(),
		CheckUser:   true,
		CertTTL:     time.Minute,
	})

	// The role mapping claims for the user are only populated from the information retrieved
	// via the user info endpoint. This validates that when the user info endpoint
	// is insecure that we do not enrich the user and the appropriate error is returned.
	require.ErrorIs(t, err, eauth.ErrOIDCNoRoles)
}

// TestUserInfoBadStatus asserts that a 4xx response from userinfo results
// in claims only being take from the id token.
func TestUserInfoBadStatus(t *testing.T) {
	t.Parallel()

	// When userinfo requests fail, we do not enrich the user and appropriate error is returned.
	userinfoError := func(t require.TestingT, err error, i ...any) {
		require.ErrorIs(t, err, eauth.ErrOIDCNoRoles, i...)
	}

	tests := []struct {
		name           string
		userId         string
		userEmail      string
		statusCode     int
		expectedGroups []string
		assertion      require.ErrorAssertionFunc
	}{
		{
			name:       http.StatusText(http.StatusInternalServerError),
			userId:     "id1",
			statusCode: http.StatusInternalServerError,
			assertion:  require.Error,
		},
		{
			name:       http.StatusText(http.StatusBadRequest),
			userId:     "id1",
			statusCode: http.StatusBadRequest,
			assertion:  userinfoError,
		},
		{
			name:       http.StatusText(http.StatusUnauthorized),
			userId:     "id1",
			statusCode: http.StatusUnauthorized,
			assertion:  userinfoError,
		},
		{
			name:       http.StatusText(http.StatusForbidden),
			userId:     "id1",
			statusCode: http.StatusForbidden,
			assertion:  userinfoError,
		},
		{
			name:       http.StatusText(http.StatusMethodNotAllowed),
			userId:     "id1",
			statusCode: http.StatusMethodNotAllowed,
			assertion:  userinfoError,
		},
		{
			name:           "existing token fallback",
			userId:         "user-with-custom-claim",
			userEmail:      "user-with-custom-claim@example.com",
			statusCode:     http.StatusBadRequest,
			expectedGroups: []string{"access"},
			assertion:      require.NoError,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			idp, err := oidctest.NewIdPServer(oidctest.WithProxyOP(
				func(h http.Handler) http.Handler {
					return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if strings.Contains(r.URL.Path, "userinfo") {
							w.WriteHeader(test.statusCode)
							return
						}

						h.ServeHTTP(w, r)
					})
				},
			))
			require.NoError(t, err)
			t.Cleanup(idp.Close)

			suite := newOIDCSuite(t, idp)

			ctx := context.Background()
			suite.connector.Spec.AllowUnverifiedEmail = true
			_, err = suite.authServer.UpdateOIDCConnector(ctx, suite.connector)
			require.NoError(t, err)

			_, _, err = suite.authenticateUser(ctx, test.userId, types.OIDCAuthRequest{
				ConnectorID: suite.connector.GetName(),
				CheckUser:   true,
			})
			test.assertion(t, err)

			if len(test.expectedGroups) > 0 {
				user, err := suite.authServer.Services.GetUser(t.Context(), test.userEmail, false)
				require.NoError(t, err)
				require.ElementsMatch(t, user.GetTraits()["groups"], test.expectedGroups)
			}
		})
	}
}

func TestMergeUserInfoClaims(t *testing.T) {
	t.Parallel()

	type userInfo struct {
		status int
		resp   *oidc.UserInfo
	}

	tests := []struct {
		name             string
		userInfo         userInfo
		expectedGroups   []string
		errAssertionFunc require.ErrorAssertionFunc
	}{
		{
			name: "userinfo overrides groups claim",
			userInfo: userInfo{
				status: http.StatusOK,
				resp: &oidc.UserInfo{
					Subject: "id1", // this user wont get "groups" claim in id_token.
					UserInfoProfile: oidc.UserInfoProfile{
						Name: "test-user",
					},
					UserInfoEmail: oidc.UserInfoEmail{
						Email:         "test-user@example.com",
						EmailVerified: true,
					},
					Address: &oidc.UserInfoAddress{},
					Claims: map[string]any{
						"groups": []string{"access", "custom-group"},
					},
				},
			},
			// final groups claim should match values returned from userinfo.
			expectedGroups:   []string{"access", "custom-group"},
			errAssertionFunc: require.NoError,
		},

		{
			name: "userinfo does not override if claim key exists",
			userInfo: userInfo{
				status: http.StatusOK,
				resp: &oidc.UserInfo{
					Subject: "user-with-custom-claim", // this user gets "groups" claim in id_token.
					UserInfoProfile: oidc.UserInfoProfile{
						Name: "user-with-custom-claim",
					},
					UserInfoEmail: oidc.UserInfoEmail{
						Email:         "user-with-custom-claim@example.com",
						EmailVerified: true,
					},
					Address: &oidc.UserInfoAddress{},
					Claims: map[string]any{
						"groups": []string{"no-access", "custom-group"},
					},
				},
			},
			// final groups claim should match values initially returned in id_token.
			expectedGroups:   []string{"access"},
			errAssertionFunc: require.NoError,
		},
		{
			name: "userinfo does not override if claim key exists",
			userInfo: userInfo{
				status: http.StatusOK,
				resp: &oidc.UserInfo{
					Subject: "user-with-empty-group-claim", // this user gets "groups" claim with empty value in id_token
					UserInfoProfile: oidc.UserInfoProfile{
						Name: "user-with-empty-group-claim",
					},
					UserInfoEmail: oidc.UserInfoEmail{
						Email:         "user-with-empty-group-claim@example.com",
						EmailVerified: true,
					},
					Address: &oidc.UserInfoAddress{},
					Claims: map[string]any{
						"groups": []string{"access"},
					},
				},
			},
			// since this user had zero groups, it should result in error, despite having values
			// returned from the userinfo endpoint.
			errAssertionFunc: func(tt require.TestingT, err error, i ...any) {
				require.ErrorIs(t, err, eauth.ErrOIDCNoRoles)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			uinfoHandler := oidctest.WithProxyOP(func(h http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if strings.Contains(r.URL.Path, "userinfo") {
						w.WriteHeader(test.userInfo.status)
						w.Header().Set("Content-Type", "application/json")
						if err := json.NewEncoder(w).Encode(test.userInfo.resp); err != nil {
							http.Error(w, err.Error(), http.StatusInternalServerError)
						}
						return
					}
					h.ServeHTTP(w, r)
				})
			})

			idp, err := oidctest.NewIdPServer(uinfoHandler)
			require.NoError(t, err)
			t.Cleanup(idp.Close)

			suite := newOIDCSuite(t, idp)

			_, _, err = suite.authenticateUser(t.Context(), test.userInfo.resp.Subject, types.OIDCAuthRequest{
				ConnectorID: suite.connector.GetName(),
				CheckUser:   true,
			})
			test.errAssertionFunc(t, err)

			// Only expected if suite.authenticateUser passes.
			if len(test.expectedGroups) > 0 {
				user, err := suite.authServer.Services.GetUser(t.Context(), test.userInfo.resp.Email, false)
				require.NoError(t, err)
				require.ElementsMatch(t, user.GetTraits()["groups"], test.expectedGroups)
			}
		})
	}
}

func TestSSODiagnostic(t *testing.T) {
	t.Parallel()

	var loginHookCounter atomic.Int32
	var loginHook auth.LoginHook = func(context.Context, types.User) error {
		loginHookCounter.Add(1)
		return nil
	}

	tests := []struct {
		name            string
		claimsToRoles   []types.ClaimMapping
		user            oidctest.User
		traitsMap       map[string][]string
		expectRoles     []string
		expectGroups    []string
		expectTraits    map[string]string
		wantValidateErr error
		loginHooks      []auth.LoginHook
	}{
		{
			name: "success",
			claimsToRoles: []types.ClaimMapping{
				{
					Claim: "groups",
					Value: "idp-admin",
					Roles: []string{"access"},
				},
			},
			user: oidctest.User{
				User: &storage.User{
					ID:            "00001234abcd",
					Username:      "test-user",
					Password:      "verysecure",
					FirstName:     "Test",
					LastName:      "User",
					Email:         "superuser@example.com",
					EmailVerified: true,
					IsAdmin:       true,
				},
				Claims: map[string]any{
					"groups": []string{"everyone", "idp-admin", "idp-dev"},
					"exp":    1652091713.0,
				},
			},
			expectRoles:  []string{"access"},
			expectGroups: []string{"everyone", "idp-admin", "idp-dev"},
			expectTraits: map[string]string{
				"email":              "superuser@example.com",
				"sub":                "00001234abcd",
				"aud":                "test",
				"azp":                "test",
				"family_name":        "User",
				"given_name":         "Test",
				"name":               "Test User",
				"preferred_username": "test-user",
				"client_id":          "test",
			},
			loginHooks: []auth.LoginHook{
				loginHook,
				loginHook,
			},
		},
		{
			name: "fail to map claims to roles",
			claimsToRoles: []types.ClaimMapping{
				{
					Claim: "groups",
					Value: "nonexistant",
					Roles: []string{"access"},
				},
			},
			user: oidctest.User{
				User: &storage.User{
					ID:            "00001234abcd",
					Username:      "test-user",
					Password:      "verysecure",
					FirstName:     "Test",
					LastName:      "User",
					Email:         "superuser@example.com",
					EmailVerified: true,
					IsAdmin:       true,
				},
				Claims: map[string]any{
					"groups": []string{"everyone", "idp-admin", "idp-dev"},
					"exp":    1652091713.0,
				},
			},
			wantValidateErr: eauth.ErrOIDCNoRoles,
		},
		{
			// Test that login rules can influence mapped roles.
			name: "login rules",
			claimsToRoles: []types.ClaimMapping{
				{
					Claim: "groups",
					Value: "rule-access",
					Roles: []string{"access"},
				},
			},
			user: oidctest.User{
				User: &storage.User{
					ID:            "00001234abcd",
					Username:      "test-user",
					Password:      "verysecure",
					FirstName:     "Test",
					LastName:      "User",
					Email:         "superuser@example.com",
					EmailVerified: true,
				},
				Claims: map[string]any{
					"groups": []string{"everyone", "idp-admin", "idp-dev"},
					"exp":    1652091713.0,
				},
			},
			traitsMap: map[string][]string{
				"email": {"external.email"},
				"groups": {
					`ifelse(external.groups.contains("idp-admin"),
						external.groups.add("rule-access"),
						external.groups)`,
				},
			},
			expectRoles:  []string{"access"},
			expectGroups: []string{"everyone", "idp-admin", "idp-dev", "rule-access"},
			expectTraits: map[string]string{
				"email": "superuser@example.com",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			idp, err := oidctest.NewIdPServer(oidctest.WithUsers([]oidctest.User{tc.user}))
			require.NoError(t, err)
			t.Cleanup(idp.Close)

			suite := newOIDCSuite(t, idp)
			suite.connector.Spec.ClaimsToRoles = tc.claimsToRoles
			_, err = suite.authServer.UpdateOIDCConnector(ctx, suite.connector)
			require.NoError(t, err)

			loginHookCounter.Store(0)
			for _, hook := range tc.loginHooks {
				suite.authServer.RegisterLoginHook(hook)
			}

			var expectLoginRules []string
			if len(tc.traitsMap) > 0 {
				installLoginRule(ctx, t, suite.authServer, suite.backend, tc.traitsMap)
				expectLoginRules = append(expectLoginRules, "testrule")
			}

			suite.emitter.Reset()

			addr := utils.MustParseAddr("1.1.1.1:42")
			oidcRequest := types.OIDCAuthRequest{
				ConnectorID:   "-sso-test-okta",
				Type:          constants.OIDC,
				CertTTL:       defaults.OIDCAuthRequestTTL,
				SSOTestFlow:   true,
				ConnectorSpec: &suite.connector.Spec,
				ClientLoginIP: addr.String(),
			}

			reqID, resp, err := suite.authenticateUser(ctx, tc.user.ID, oidcRequest)
			if tc.wantValidateErr != nil {
				require.ErrorIs(t, err, tc.wantValidateErr)
				return
			}

			require.Len(t, tc.loginHooks, int(loginHookCounter.Load()))

			require.NoError(t, err)
			require.NotNil(t, resp)
			require.Empty(t, cmp.Diff(
				&authclient.OIDCAuthResponse{
					Username: "superuser@example.com",
					Identity: types.ExternalIdentity{
						ConnectorID: "-sso-test-okta",
						Username:    "superuser@example.com",
					},
				},
				resp,
				cmpopts.IgnoreFields(authclient.OIDCAuthResponse{}, "Req")),
			)
			require.NotNil(t, suite.emitter.LastEvent())
			require.Equal(t, events.UserLoginEvent, suite.emitter.LastEvent().GetType())
			require.IsType(t, &apievents.UserLogin{}, suite.emitter.LastEvent())
			loginEvt := suite.emitter.LastEvent().(*apievents.UserLogin)
			require.Equal(t, addr.String(), loginEvt.ConnectionMetadata.RemoteAddr)

			diagInfo, err := suite.authServer.GetSSODiagnosticInfo(ctx, types.KindOIDC, reqID)
			require.NoError(t, err)

			diff := cmp.Diff(
				diagInfo,
				&types.SSODiagnosticInfo{
					TestFlow: true,
					Success:  true,
					CreateUserParams: &types.CreateUserParams{
						ConnectorName: "-sso-test-okta",
						Username:      "superuser@example.com",
						Logins:        nil,
						KubeGroups:    nil,
						KubeUsers:     nil,
						Roles:         tc.expectRoles,
						SessionTTL:    600000000000,
					},
					OIDCClaimsToRoles:         tc.claimsToRoles,
					OIDCClaimsToRolesWarnings: nil,
					OIDCClaims:                tc.user.Claims,
					OIDCIdentity: &types.OIDCIdentity{
						ID:        "00001234abcd",
						Name:      "Test User",
						Email:     "superuser@example.com",
						ExpiresAt: diagInfo.OIDCIdentity.ExpiresAt,
					},
					OIDCConnectorTraitMapping: []types.TraitMapping{
						{
							Trait: tc.claimsToRoles[0].Claim,
							Value: tc.claimsToRoles[0].Value,
							Roles: tc.claimsToRoles[0].Roles,
						},
					},
					AppliedLoginRules: expectLoginRules,
				},
				cmpopts.SortSlices(func(a, b string) bool { return a < b }),
				cmpopts.IgnoreFields(types.CreateUserParams{}, "Traits"),
				cmpopts.IgnoreFields(types.SSODiagnosticInfo{}, "OIDCClaims", "OIDCTraitsFromClaims"),
			)
			require.Empty(t, diff, "diagnostic info does not match expected")

			assert.Empty(t, cmp.Diff(tc.expectGroups, diagInfo.CreateUserParams.Traits["groups"],
				cmpopts.SortSlices(func(a string, b string) bool {
					return a < b
				})))

			claimGroups := diagInfo.OIDCClaims["groups"].([]any)
			groups := make([]string, 0, len(claimGroups))
			for _, g := range claimGroups {
				groups = append(groups, g.(string))
			}
			assert.Empty(t, cmp.Diff(tc.user.Claims["groups"], groups,
				cmpopts.SortSlices(func(a string, b string) bool {
					return a < b
				})))
			assert.Empty(t, cmp.Diff(
				tc.expectGroups, diagInfo.OIDCTraitsFromClaims["groups"],
				cmpopts.SortSlices(func(a string, b string) bool {
					return a < b
				})))

			for traitName, traitValue := range tc.expectTraits {
				v, ok := diagInfo.CreateUserParams.Traits[traitName]
				assert.True(t, ok)
				assert.Equal(t, []string{traitValue}, v)

				claim, ok := diagInfo.OIDCClaims[traitName]
				assert.True(t, ok)
				switch claim.(type) {
				case string:
					assert.Equal(t, traitValue, claim)
				case []any:
					assert.Equal(t, []any{traitValue}, claim)
				}

				trait, ok := diagInfo.OIDCTraitsFromClaims[traitName]
				assert.True(t, ok)
				assert.Equal(t, []string{traitValue}, trait)
			}

			require.Equal(t, len(tc.loginHooks), int(loginHookCounter.Load()))
		})
	}
}

func installLoginRule(ctx context.Context, t *testing.T, a *auth.Server, b backend.Backend, traitsMap map[string][]string) {
	// Install login rules plugin.
	ruleStorage := loginrulestorage.New(b)
	evaluator := loginrule.NewEvaluator(ruleStorage)
	a.SetLoginRuleEvaluator(evaluator)

	if len(traitsMap) == 0 {
		return
	}

	// Create login rule and upsert to backend.
	rule := loginrulepb.LoginRule_builder{
		Metadata: &types.Metadata{
			Name: "testrule",
		},
		TraitsMap: make(map[string]*wrappers.StringValues),
	}.Build()
	for trait, values := range traitsMap {
		rule.GetTraitsMap()[trait] = &wrappers.StringValues{
			Values: values,
		}
	}
	_, err := ruleStorage.CreateLoginRule(ctx, rule)
	require.NoError(t, err)
}

func TestEmailVerifiedClaim(t *testing.T) {
	t.Parallel()

	users := []oidctest.User{
		{
			User: &storage.User{
				ID:            "id1",
				Username:      "test-user",
				Password:      "verysecure",
				FirstName:     "Test",
				LastName:      "User",
				Email:         "test-user@example.com",
				EmailVerified: true,
			},
			Claims: map[string]any{
				"groups": []string{"access"},
			},
		},
		{
			User: &storage.User{
				ID:            "id2",
				Username:      "test-user2",
				Password:      "verysecure",
				FirstName:     "Test",
				LastName:      "User",
				Email:         "test-user2@example.com",
				EmailVerified: false,
			},
			Claims: map[string]any{
				"groups": []string{"access"},
			},
		},
		{
			User: &storage.User{
				ID:        "id3",
				Username:  "test-user3",
				Password:  "verysecure",
				FirstName: "Test",
				LastName:  "User",
				Email:     "test-user3@example.com",
			},
			Claims: map[string]any{
				"groups":         []string{"access"},
				"email_verified": "true",
			},
		},
		{
			User: &storage.User{
				ID:        "id4",
				Username:  "test-user4",
				Password:  "verysecure",
				FirstName: "Test",
				LastName:  "User",
				Email:     "test-user4@example.com",
			},
			Claims: map[string]any{
				"groups":         []string{"access"},
				"email_verified": "false",
			},
		},
		{
			User: &storage.User{
				ID:        "id5",
				Username:  "test-user5",
				Password:  "verysecure",
				FirstName: "Test",
				LastName:  "User",
				Email:     "test-user5@example.com",
			},
			Claims: map[string]any{
				"groups":         []string{"access"},
				"email_verified": "random_value",
			},
		},
		{
			User: &storage.User{
				ID:        "id6",
				Username:  "test-user6",
				Password:  "verysecure",
				FirstName: "Test",
				LastName:  "User",
				Email:     "test-user6@example.com",
			},
			Claims: map[string]any{
				"groups":         []string{"access"},
				"email_verified": "",
			},
		},
	}

	idp, err := oidctest.NewIdPServer(oidctest.WithUsers(users))
	require.NoError(t, err)
	t.Cleanup(idp.Close)

	suite := newOIDCSuite(t, idp)
	ctx := context.Background()

	unverifiedErrorAssertion := func(t require.TestingT, err error, i ...any) {
		require.ErrorContains(t, err, "email not verified by OIDC provider")
	}

	tests := []struct {
		name      string
		userID    string
		assertion require.ErrorAssertionFunc
	}{
		{
			name:      "email verified in user",
			userID:    users[0].ID,
			assertion: require.NoError,
		},
		{
			name:      "email verification not provided",
			userID:    users[1].ID,
			assertion: require.NoError,
		},
		{
			name:      "email verified in claims",
			userID:    users[2].ID,
			assertion: require.NoError,
		},
		{
			name:      "email not verified in claims",
			userID:    users[3].ID,
			assertion: unverifiedErrorAssertion,
		},
		{
			name:      "bogus email verified claims",
			userID:    users[4].ID,
			assertion: unverifiedErrorAssertion,
		},
		{
			name:      "empty email verified claims",
			userID:    users[5].ID,
			assertion: require.NoError,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := suite.authenticateUser(ctx, test.userID, types.OIDCAuthRequest{
				ConnectorID: suite.connector.GetName(),
			})

			test.assertion(t, err)
		})
	}
}

// TestUsernameClaim ensures that the `username_claim` field in an OIDC config is handled correctly.
func TestUsernameClaim(t *testing.T) {
	t.Parallel()

	const usernameClaim = "the_username_claim"

	users := []oidctest.User{
		{
			User: &storage.User{
				ID:            "id1",
				Username:      "test-user",
				Password:      "verysecure",
				FirstName:     "Test",
				LastName:      "User",
				Email:         "test-user@example.com",
				EmailVerified: true,
			},
			Claims: map[string]any{
				"groups":      []string{"access"},
				usernameClaim: "USER1",
			},
		},
		{
			User: &storage.User{
				ID:            "id2",
				Username:      "test-user2",
				Password:      "verysecure",
				FirstName:     "Test",
				LastName:      "User",
				Email:         "test-user2@example.com",
				EmailVerified: true,
			},
			Claims: map[string]any{
				"groups": []string{"access"},
			},
		},
	}

	idp, err := oidctest.NewIdPServer(oidctest.WithUsers(users))
	require.NoError(t, err)
	t.Cleanup(idp.Close)

	suite := newOIDCSuite(t, idp)
	ctx := context.Background()

	suite.connector.Spec.UsernameClaim = usernameClaim
	_, err = suite.authServer.UpdateOIDCConnector(ctx, suite.connector)
	require.NoError(t, err)

	tests := []struct {
		name             string
		userID           string
		expectedUsername string
		assertion        require.ErrorAssertionFunc
	}{
		{
			name:             "custom username claim provided",
			userID:           users[0].ID,
			expectedUsername: users[0].Claims[usernameClaim].(string),
			assertion:        require.NoError,
		},
		{
			name:   "no custom username claim provided",
			userID: users[1].ID,
			assertion: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, `The configured username_claim of "the_username_claim" was not received from the IdP`)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := suite.authenticateUser(ctx, test.userID, types.OIDCAuthRequest{
				ConnectorID: suite.connector.GetName(),
			})

			test.assertion(t, err)

			if err != nil {
				return
			}

			u, err := suite.authServer.GetUser(ctx, test.expectedUsername, false)
			require.NoError(t, err)
			require.NotNil(t, u)
		})
	}
}

// TestReqMaxAge tests that MaxAge is correctly set in a OIDC authentication request.
func TestReqMaxAge(t *testing.T) {
	t.Parallel()

	idp, err := oidctest.NewIdPServer()
	require.NoError(t, err)
	t.Cleanup(idp.Close)

	suite := newOIDCSuite(t, idp)
	ctx := context.Background()

	tests := []struct {
		name              string
		maxAge            *types.MaxAge
		expectedReqMaxAge string
	}{
		{
			name: "empty",
		},
		{
			name:              "zero",
			maxAge:            &types.MaxAge{Value: types.Duration(0)},
			expectedReqMaxAge: "0",
		},
		{
			name:              "hour",
			maxAge:            &types.MaxAge{Value: types.Duration(time.Hour)},
			expectedReqMaxAge: "3600",
		},
	}

	for _, test := range tests {
		suite.connector.Spec.MaxAge = test.maxAge
		_, err := suite.authServer.UpdateOIDCConnector(ctx, suite.connector)
		require.NoError(t, err)

		oidcRequest := types.OIDCAuthRequest{
			ConnectorID:   "okta-oidc",
			Type:          constants.OIDC,
			CertTTL:       defaults.OIDCAuthRequestTTL,
			SSOTestFlow:   true,
			ConnectorSpec: &suite.connector.Spec,
		}
		request, err := suite.oidcService.CreateOIDCAuthRequest(ctx, oidcRequest)
		require.NoError(t, err)
		require.NotEmpty(t, request.RedirectURL)

		redirURL, err := url.Parse(request.RedirectURL)
		require.NoError(t, err)
		maxAge := redirURL.Query().Get("max_age")
		require.Equal(t, test.expectedReqMaxAge, maxAge)

	}
}

func TestValidateACRValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		acrClaim  any
		acrValue  string
		provider  string
		assertion require.ErrorAssertionFunc
	}{
		{
			name:      "default, acr values match",
			acrClaim:  "foo",
			acrValue:  "foo",
			assertion: require.NoError,
		},
		{
			name:      "default, acr values do not match",
			acrClaim:  "foo",
			acrValue:  "bar",
			assertion: require.Error,
		},
		{
			name: "netiq, acr values match",
			acrClaim: map[string][]string{
				"values": {
					"foo/bar/baz",
				},
			},
			acrValue:  "foo/bar/baz",
			provider:  "netiq",
			assertion: require.NoError,
		},
		{
			name: "netiq, invalid format",
			acrClaim: map[string]string{
				"values": "foo/bar/baz",
			},
			acrValue:  "foo/bar/baz",
			provider:  "netiq",
			assertion: require.Error,
		},
		{
			name: "netiq, invalid value",
			acrClaim: map[string][]string{
				"values": {
					"foo/bar/baz/qux",
				},
			},

			acrValue:  "foo/bar/baz",
			provider:  "netiq",
			assertion: require.Error,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			idp, err := oidctest.NewIdPServer(oidctest.WithUsers(
				[]oidctest.User{
					{
						User: &storage.User{
							ID:            "user1",
							Username:      "test-user",
							Password:      "verysecure",
							FirstName:     "Test",
							LastName:      "User",
							Email:         "test-user@example.com",
							EmailVerified: true,
						},
						Claims: map[string]any{
							"acr":    test.acrClaim,
							"groups": []string{"access"},
						},
					},
				},
			))
			require.NoError(t, err)
			t.Cleanup(idp.Close)

			suite := newOIDCSuite(t, idp)

			suite.connector.Spec.Provider = test.provider
			suite.connector.Spec.ACR = test.acrValue

			ctx := context.Background()
			_, err = suite.authServer.UpdateOIDCConnector(ctx, suite.connector)
			require.NoError(t, err)

			_, _, err = suite.authenticateUser(ctx, "user1", types.OIDCAuthRequest{
				ConnectorID: suite.connector.GetName(),
			})
			test.assertion(t, err)
		})
	}
}

func TestOIDCLicense(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		disabled    bool
		expectError bool
	}{
		{
			name: "valid license",
		},
		{
			name:        "disabled license",
			disabled:    true,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			var suiteOpts []func(*oidcSuiteOpts)
			if tt.disabled {
				suiteOpts = append(suiteOpts, withLicenseChecker(&disabledLicenseChecker{}))
			}

			idp, err := oidctest.NewIdPServer()
			require.NoError(t, err)
			t.Cleanup(idp.Close)

			suite := newOIDCSuite(t, idp, suiteOpts...)

			req := types.OIDCAuthRequest{ConnectorID: suite.connector.GetName(), Type: constants.OIDC}
			_, err = suite.oidcService.CreateOIDCAuthRequest(ctx, req)
			if tt.expectError {
				require.Error(t, err)
				require.True(t, trace.IsAccessDenied(err), "expected access denied, got: %v", err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestOIDCLicenseUpdateEnterpriseModules verifies that calling UpdateModules on
// the real *emodules.EnterpriseModules propagates immediately to the OIDC service,
// because both hold a pointer to the same struct.
func TestOIDCLicenseUpdateEnterpriseModules(t *testing.T) {
	t.Parallel()

	// Build an *EnterpriseModules with a far-future expiry so IsDisabled() = false.
	validLic := &types.LicenseV3{}
	validLic.SetExpiry(time.Now().Add(24 * time.Hour))
	validLicenseFile := &licensefile.LicenseFile{License: validLic}
	mod := emodules.NewEnterpriseModules(emodules.EnterpriseModulesConfig{
		License: validLicenseFile,
	})

	idp, err := oidctest.NewIdPServer()
	require.NoError(t, err)
	t.Cleanup(idp.Close)

	suite := newOIDCSuite(t, idp, withLicenseChecker(mod))
	req := types.OIDCAuthRequest{ConnectorID: suite.connector.GetName(), Type: constants.OIDC}
	ctx := context.Background()

	// Initially valid — license has a future expiry.
	_, err = suite.oidcService.CreateOIDCAuthRequest(ctx, req)
	require.NoError(t, err)

	// Expire the license well past the 30-day grace period.
	expiredLic := &types.LicenseV3{}
	expiredLic.SetExpiry(time.Now().Add(-60 * 24 * time.Hour))
	mod.UpdateModules(&licensefile.LicenseFile{License: expiredLic}, modules.Features{})
	_, err = suite.oidcService.CreateOIDCAuthRequest(ctx, req)
	require.True(t, trace.IsAccessDenied(err), "expected access denied after license expired, got: %v", err)

	// Renew the license — the same pointer, so the OIDC service sees the change immediately.
	mod.UpdateModules(validLicenseFile, modules.Features{})
	_, err = suite.oidcService.CreateOIDCAuthRequest(ctx, req)
	require.NoError(t, err, "expected success after license renewed")
}

func TestValidateOIDCResponseMFA(t *testing.T) {
	t.Parallel()
	clock := clockwork.NewFakeClock()
	idp, err := oidctest.NewIdPServer(oidctest.WithUsers([]oidctest.User{
		{
			User: &storage.User{
				ID:            "id1",
				Username:      "test-user",
				Password:      "verysecure",
				FirstName:     "Test",
				LastName:      "User",
				Email:         "test-user@example.com",
				EmailVerified: true,
				IsAdmin:       true,
			},
			Claims: map[string]any{
				"groups":    []string{"access"},
				"auth_time": float64(clock.Now().Unix()),
			},
		},
	}))
	require.NoError(t, err)
	t.Cleanup(idp.Close)

	suite := newOIDCSuite(t, idp, overrideClock(clock))

	suite.connector.Spec.MFASettings = &types.OIDCConnectorMFASettings{
		Enabled:      true,
		ClientId:     suite.connector.GetClientID(),
		ClientSecret: suite.connector.GetClientSecret(),
	}

	ctx := context.Background()
	_, err = suite.authServer.UpdateOIDCConnector(ctx, suite.connector)
	require.NoError(t, err)
	// Login first to ensure user is created
	_, _, err = suite.authenticateUser(ctx, "id1", types.OIDCAuthRequest{
		ConnectorID:      suite.connector.GetName(),
		Type:             constants.OIDC,
		CreateWebSession: true,
		CheckUser:        true,
	})
	require.NoError(t, err)

	for _, tt := range []struct {
		name              string
		mutateSessionData func(sd *services.SSOMFASessionData)
		mutateConnector   func(conn *types.OIDCConnectorV3)
		checkError        assert.ErrorAssertionFunc
		checkResponse     func(t *testing.T, token string, resp *authclient.OIDCAuthResponse)
	}{
		{
			name:       "OK valid MFA session",
			checkError: assert.NoError,
			checkResponse: func(t *testing.T, token string, resp *authclient.OIDCAuthResponse) {
				require.NotEmpty(t, resp)
				assert.NotEmpty(t, resp.MFAToken)

				// MFA session data token should match the response.
				sd, err := suite.authServer.GetMFASessionData(ctx, token)
				assert.NoError(t, err)
				assert.Equal(t, resp.MFAToken, sd.Token)
			},
		},
		{
			name: "NOK username mismatch",
			mutateSessionData: func(sd *services.SSOMFASessionData) {
				sd.Username = "unknown"
			},
			checkError: func(t assert.TestingT, err error, i ...any) bool {
				return assert.True(t, trace.IsAccessDenied(err), "expected access denied error but got %v", err)
			},
		},
		{
			name: "NOK connectorID mismatch",
			mutateSessionData: func(sd *services.SSOMFASessionData) {
				sd.ConnectorID = "unknown"
			},
			checkError: func(t assert.TestingT, err error, i ...any) bool {
				return assert.True(t, trace.IsAccessDenied(err), "expected access denied error but got %v", err)
			},
		},
		{
			name: "NOK connectorType mismatch",
			mutateSessionData: func(sd *services.SSOMFASessionData) {
				sd.ConnectorType = "unknown"
			},
			checkError: func(t assert.TestingT, err error, i ...any) bool {
				return assert.True(t, trace.IsAccessDenied(err), "expected access denied error but got %v", err)
			},
		},
		{
			name: "OK divergent MFA client",
			mutateConnector: func(conn *types.OIDCConnectorV3) {
				// Configure MFA settings to use a separate client from the base OIDC connector.
				// 'mfa-no-roles' is a special client configured to return an ID token with no
				// custom claims. We'll use this role to validate that MFA authentication does
				// not require role mapping, only that the user has a valid MFA session and is
				// currently logged in.
				conn.Spec.MFASettings = &types.OIDCConnectorMFASettings{
					Enabled:      true,
					ClientId:     "mfa-no-roles",
					ClientSecret: "secret",
				}
			},
			checkError: assert.NoError,
			checkResponse: func(t *testing.T, token string, resp *authclient.OIDCAuthResponse) {
				require.NotEmpty(t, resp)
				assert.NotEmpty(t, resp.MFAToken)

				// MFA session data token should match the response.
				sd, err := suite.authServer.GetMFASessionData(ctx, token)
				assert.NoError(t, err)
				assert.Equal(t, resp.MFAToken, sd.Token)
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// Add SSO MFA session data for the oidc auth request. This should result in an MFA token being created.
			sd := &services.SSOMFASessionData{
				Username:      "test-user@example.com",
				ConnectorID:   suite.connector.GetName(),
				ConnectorType: constants.OIDC,
			}
			if tt.mutateSessionData != nil {
				tt.mutateSessionData(sd)
			}

			if tt.mutateConnector != nil {
				tt.mutateConnector(suite.connector)
				_, err := suite.authServer.UpdateOIDCConnector(ctx, suite.connector)
				require.NoError(t, err)
			}

			response, err := suite.authenticateUserWithMFA(ctx, "id1",
				sd,
				types.OIDCAuthRequest{
					ConnectorID:      suite.connector.GetName(),
					Type:             constants.OIDC,
					CreateWebSession: true,
					CheckUser:        true,
				},
			)
			tt.checkError(t, err)

			if tt.checkResponse != nil {
				tt.checkResponse(t, sd.RequestID, response)
			}
		})
	}
}

// TestOIDCRoleMapping verifies basic mapping from OIDC claims to roles.
func TestOIDCRoleMapping(t *testing.T) {
	// create a connector
	oidcConnector, err := types.NewOIDCConnector("example", types.OIDCConnectorSpecV3{
		IssuerURL:     "https://www.exmaple.com",
		ClientID:      "example-client-id",
		ClientSecret:  "example-client-secret",
		Display:       "sign in with example.com",
		Scope:         []string{"foo", "bar"},
		ClaimsToRoles: []types.ClaimMapping{{Claim: "roles", Value: "teleport-user", Roles: []string{"user"}}},
		RedirectURLs:  []string{"https://localhost:3080/v1/webapi/oidc/callback"},
	})
	require.NoError(t, err)

	// create some claims
	claims := map[string]any{
		"roles":     "teleport-user",
		"email":     "foo@example.com",
		"nickname":  "foo",
		"full_name": "foo bar",
	}

	traits := eauth.OIDCClaimsToTraits(claims)
	require.Len(t, traits, 4)

	_, roles := services.TraitsToRoles(oidcConnector.GetTraitMappings(), traits)
	require.Len(t, roles, 1)
	require.Equal(t, "user", roles[0])
}

func TestOIDCClaimsToTraits(t *testing.T) {
	claims := map[string]any{
		"singular_claim": "value",
		"plural_claim":   []string{"value1", "value2", "value3"},
	}

	expectTraits := map[string][]string{
		"singular_claim": {"value"},
		"plural_claim":   {"value1", "value2", "value3"},
	}

	gotTraits := eauth.OIDCClaimsToTraits(claims)
	require.Equal(t, expectTraits, gotTraits)
}

// TestLargePayload verifies that large payloads from
// discovery requests are rejected.
func TestLargePayload(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(strings.Repeat("a", 1024*1024*2))
	})

	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)

	idp, err := oidctest.NewIdPServer()
	require.NoError(t, err)
	t.Cleanup(idp.Close)

	suite := newOIDCSuite(t, idp)
	suite.connector.Spec.IssuerURL = srv.URL

	ctx := context.Background()
	_, err = suite.authServer.UpdateOIDCConnector(ctx, suite.connector)
	require.NoError(t, err)

	_, err = suite.oidcService.CreateOIDCAuthRequest(
		context.Background(),
		types.OIDCAuthRequest{ConnectorID: suite.connector.GetName()},
	)
	require.ErrorContains(t, err, "response exceeds maximum size of")
}

// TestDiscoveryURL verifies that query parameters set on the issuer url
// are passed along to discovery requests.
func TestDiscoveryURL(t *testing.T) {
	t.Parallel()

	idp, err := oidctest.NewIdPServer()
	require.NoError(t, err)
	t.Cleanup(idp.Close)

	suite := newOIDCSuite(t, idp)

	mux := http.NewServeMux()
	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)

	u, err := url.Parse(srv.URL)
	require.NoError(t, err)

	params := u.Query()
	params.Add("a", "b")
	params.Add("c", "d")
	u.RawQuery = params.Encode()

	requestedURL := make(chan *url.URL, 2 /* Discovery endpoint gets called twice */)
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		requestedURL <- r.URL
		fmt.Fprintf(w, `{
		"issuer": "%[1]v",
		"authorization_endpoint": "%[1]v/authz",
		"token_endpoint": "%[1]v/token",
		"jwks_uri": "%[1]v/jwks",
		"userinfo_endpoint": "%[1]v/userinfo",
		"subject_types_supported": ["public"],
		"id_token_signing_alg_values_supported": ["HS256", "RS256"]
}`, u.String())
	})

	connector := proto.Clone(suite.connector).(*types.OIDCConnectorV3)
	connector.Spec.IssuerURL = u.String()

	ctx := context.Background()
	_, err = suite.authServer.UpsertOIDCConnector(ctx, connector)
	require.NoError(t, err)

	_, err = suite.oidcService.CreateOIDCAuthRequest(
		context.Background(),
		types.OIDCAuthRequest{ConnectorID: connector.GetName()},
	)
	require.NoError(t, err)

	select {
	case u := <-requestedURL:
		assert.Equal(t, "b", u.Query().Get("a"))
		assert.Equal(t, "d", u.Query().Get("c"))
	case <-time.After(20 * time.Second):
		t.Fatal("timed out waiting for requested url")
	}
}

func TestAuthorizationRequestObject(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	idp, err := oidctest.NewIdPServer()
	require.NoError(t, err)
	t.Cleanup(idp.Close)

	defaultSuite := newOIDCSuite(t, idp)

	clusterName, err := defaultSuite.authServer.GetClusterName(ctx)
	require.NoError(t, err)

	_, ecdsaSigner, err := newECDSAKey(clusterName.GetClusterID())
	require.NoError(t, err)

	_, err = defaultSuite.authServer.UpdateOIDCConnector(ctx, defaultSuite.connector)
	require.NoError(t, err)

	type createOIDCAuthRequestResults struct {
		req *types.OIDCAuthRequest
		err error
	}

	// Inspect/validate the output of each call
	validateEquivalence := func(key crypto.PublicKey, defaultResult createOIDCAuthRequestResults, jarResult createOIDCAuthRequestResults) {
		// first, ensure both calls succeeded
		require.NoError(t, defaultResult.err)
		require.NoError(t, jarResult.err)

		jarURL, err := url.ParseRequestURI(jarResult.req.RedirectURL)
		require.NoError(t, err)
		// jar url MUST have the client id
		clientID := jarURL.Query().Get("client_id")
		assert.Equal(t, defaultSuite.connector.Spec.ClientID, clientID)
		// and the request object
		requestObject := jarURL.Query().Get("request")
		require.NotEmpty(t, requestObject)

		// Request object should be signed with the provided key
		claimsMap, headers, err := parseAndVerifyOIDCAuthRequestToken(key, requestObject)
		require.NoError(t, err)

		// Every query parameter in the "non-jar" authorization request *should* appear in
		// the claims set of the JWT
		defaultURL, err := url.ParseRequestURI(defaultResult.req.RedirectURL)
		require.NoError(t, err)
		for key := range defaultURL.Query() {
			assert.Contains(t, claimsMap, key, "Claims set is missing the claim '%s'", key)
		}
		// "kid" should also be set on the header
		require.Len(t, headers, 1) /* Single signature only */
		assert.NotEmpty(t, headers[0].KeyID)
	}

	tests := []struct {
		name           string
		input          types.OIDCAuthRequest
		pre            func(*OIDCSuite)
		inspectResults func(createOIDCAuthRequestResults, createOIDCAuthRequestResults)
	}{
		// Test default key factory and RS256 signing.
		{
			name:  "default key factory",
			input: types.OIDCAuthRequest{ConnectorID: defaultSuite.connector.GetName(), StateToken: "somestate"},
			inspectResults: func(defaultResult, jarResult createOIDCAuthRequestResults) {
				validateEquivalence(defaultSuite.oidcIDPPublicKey, defaultResult, jarResult)
			},
		},
		// Test error returned when provider does not support our signing algorithm.
		{
			name: "unsupported signing algorithm",
			pre: func(suite *OIDCSuite) {
				// Rebuild the OIDC service with a new key factory that
				// invokes the default key factory, but swaps the reported algorithm.
				// This way we can validate that signing with ES256 works, but work around
				// the provider's hardcoded RS256 compatibility advertisement
				suite.oidcService, err = eauth.NewOIDCAuthService(
					&eauth.OIDCAuthServiceConfig{
						Auth:           suite.authServer,
						Emitter:        suite.emitter,
						Client:         suite.idpServer.Client(),
						LicenseChecker: alwaysValidLicense{},
						SignerFactory: func(ctx context.Context) (jose.Signer, string, error) {
							jwtSigner, err := joseSignerFromCrypto(ecdsaSigner)
							return jwtSigner, "ES256", err
						},
					},
				)
				require.NoError(t, err)
			},
			input: types.OIDCAuthRequest{ConnectorID: defaultSuite.connector.GetName(), StateToken: "somestate"},
			inspectResults: func(defaultResult, jarResult createOIDCAuthRequestResults) {
				// Should fail because the provider only supports signing with RS256
				// and we provided an ECDSA key
				require.Error(t, jarResult.err)
				// Should succeed since no request object is being used
				require.NoError(t, defaultResult.err)
			},
		},
		// Test signing with ECDSA key.
		{
			name: "ES256 signing algorithm",
			pre: func(suite *OIDCSuite) {
				// Rebuild the OIDC service with a new key factory that returns
				// an ECDSA JWT key that we've generated for this test suite
				// Validates that signing with ECDSA keys works as well
				suite.oidcService, err = eauth.NewOIDCAuthService(
					&eauth.OIDCAuthServiceConfig{
						Auth:           suite.authServer,
						Emitter:        suite.emitter,
						Client:         suite.idpServer.Client(),
						LicenseChecker: alwaysValidLicense{},
						SignerFactory: func(ctx context.Context) (jose.Signer, string, error) {
							jwtSigner, err := joseSignerFromCrypto(ecdsaSigner)
							return jwtSigner, "RS256", err
						},
					},
				)
				require.NoError(t, err)
			},
			input: types.OIDCAuthRequest{ConnectorID: defaultSuite.connector.GetName(), StateToken: "somestate"},
			inspectResults: func(defaultResult, jarResult createOIDCAuthRequestResults) {
				validateEquivalence(ecdsaSigner.Public(), defaultResult, jarResult)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.pre != nil {
				tt.pre(defaultSuite)
			}

			var defaultResult createOIDCAuthRequestResults
			var jarResult createOIDCAuthRequestResults

			// Create an OIDCAuthRequest with request objects disabled
			defaultSuite.connector.SetRequestObjectMode(constants.OIDCRequestObjectModeNone)
			_, err := defaultSuite.authServer.UpdateOIDCConnector(ctx, defaultSuite.connector)
			require.NoError(t, err)
			defaultResult.req, defaultResult.err = defaultSuite.oidcService.CreateOIDCAuthRequest(ctx, tt.input)

			// Now re-enable request objects and generate a request with the same input
			defaultSuite.connector.SetRequestObjectMode(constants.OIDCRequestObjectModeSigned)
			_, err = defaultSuite.authServer.UpdateOIDCConnector(ctx, defaultSuite.connector)
			require.NoError(t, err)
			jarResult.req, jarResult.err = defaultSuite.oidcService.CreateOIDCAuthRequest(ctx, tt.input)

			// Both calls should produce equivalent requests, but the latter simply pokes all of the
			// request parameters into the claims set of a JWT
			tt.inspectResults(defaultResult, jarResult)
		})
	}
}

func TestOIDCSSOUpdateSCIMUser(t *testing.T) {
	t.Parallel()

	idp, err := oidctest.NewIdPServer()
	require.NoError(t, err)
	t.Cleanup(idp.Close)

	suite := newOIDCSuite(t, idp)
	ctx := context.Background()

	testUser, err := types.NewUser("test-user@example.com")
	require.NoError(t, err)

	testUser, err = suite.authServer.CreateUser(ctx, testUser)
	require.NoError(t, err)

	// Case 1: Local (non-OIDC) user exists with the same username.
	// We don't allow for username collisions between a local user and an OIDC identity
	// OIDC login flow must fail until the conflict is resolved.
	t.Run("OIDC flow should fail when local user is present", func(t *testing.T) {
		_, _, err = suite.authenticateUser(ctx, "id1", types.OIDCAuthRequest{
			ConnectorID: suite.connector.GetName(),
			CheckUser:   true,
			CertTTL:     time.Minute,
		})
		require.True(t, trace.IsAlreadyExists(err))
		require.ErrorContains(t, err, "Either change email in OIDC identity or remove local user and try again")
	})

	// Case 2: SCIM-managed user exists, but it was created via a different connector.
	// We don't allow logging in via OIDC using a different connector when an SCIM user
	// with the same username is already managed by SCIM provisioning
	t.Run("OIDC flow should fail when SCIM user belongs to a different connector", func(t *testing.T) {
		testUser.SetOrigin(common.OriginSCIM)
		testUser.SetCreatedBy(types.CreatedBy{
			Connector: &types.ConnectorRef{
				ID: "scim-integration-conn",
			},
		})
		testUser, err = suite.authServer.UpdateUser(ctx, testUser)
		require.NoError(t, err)

		_, _, err = suite.authenticateUser(ctx, "id1", types.OIDCAuthRequest{
			ConnectorID: suite.connector.GetName(),
			CheckUser:   true,
			CertTTL:     time.Minute,
		})
		require.True(t, trace.IsAlreadyExists(err))
		require.ErrorContains(t, err, "SCIM-managed user with the same username already exists")
	})

	// Case 3: SCIM-managed user exists and was created via the SAME connector.
	// OIDC login is allowed and should update only roles/traits, not SCIM-managed user metadata.
	t.Run("OIDC flow should succeed when SCIM user belongs to the same connector", func(t *testing.T) {
		testUser.SetOrigin(common.OriginSCIM)
		testUser.SetCreatedBy(types.CreatedBy{
			Connector: &types.ConnectorRef{
				ID:   suite.connector.GetName(),
				Type: types.KindOIDC,
			},
		})
		testUser, err = suite.authServer.UpdateUser(ctx, testUser)
		require.NoError(t, err)

		_, _, err = suite.authenticateUser(ctx, "id1", types.OIDCAuthRequest{
			ConnectorID: suite.connector.GetName(),
			CheckUser:   true,
			CertTTL:     time.Minute,
		})
		require.NoError(t, err)

		testUser, err = suite.authServer.GetUser(ctx, "test-user@example.com", false)
		require.NoError(t, err)

		// OIDC login flow should with persistent SCIM users should only update user roles/traits
		require.Equal(t, []string{"access"}, testUser.GetRoles())
		// OIDC login flow should not overwrite president SCIM user properties
		require.Equal(t, common.OriginSCIM, testUser.Origin())
		require.Nil(t, testUser.GetMetadata().Expires)
	})
}

// joseSignerFromCrypto creates a jose.Signer from a crypto.Signer
func joseSignerFromCrypto(signer crypto.Signer) (jose.Signer, error) {
	joseKey, err := jwt.SigningKeyFromPrivateKey(signer)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	kid, err := jwt.KeyID(signer.Public())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	joseSigner, err := jose.NewSigner(joseKey, (&jose.SignerOptions{}).WithHeader("kid", kid))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return joseSigner, nil
}

// parseAndVerifyOIDCAuthRequestToken parses the JWT and verifies the signature. It does not
// perform any validation of claims, but returns them for further inspection.
func parseAndVerifyOIDCAuthRequestToken(public crypto.PublicKey, rawToken string) (map[string]any, []jose.Header, error) {
	// Parse the token.
	tok, err := josejwt.ParseSigned(rawToken)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	out := map[string]any{}
	// Validate the signature on the JWT token.
	err = tok.Claims(public, &out)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return out, tok.Headers, nil
}
