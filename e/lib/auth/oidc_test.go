package auth_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zitadel/oidc/v3/example/server/storage"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
	"golang.org/x/oauth2"

	"github.com/gravitational/teleport/api/constants"
	loginrulepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/wrappers"
	eauth "github.com/gravitational/teleport/e/lib/auth"
	"github.com/gravitational/teleport/e/lib/loginrule"
	loginrulestorage "github.com/gravitational/teleport/e/lib/loginrule/storage"
	"github.com/gravitational/teleport/entitlements"
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
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/clocki"
)

// user wraps a [storage.User] with additional claims.
type user struct {
	*storage.User
	Claims map[string]any
}

type userStore struct {
	users map[string]user
}

// ExampleClientID is only used in the example server
func (u userStore) ExampleClientID() string {
	return "service"
}

func (u userStore) GetUserByID(id string) *storage.User {
	return u.users[id].User
}

func (u userStore) GetUserByUsername(username string) *storage.User {
	for _, user := range u.users {
		if user.Username == username {
			return user.User
		}
	}
	return nil
}

func newUserStore(users []user) userStore {
	store := userStore{users: make(map[string]user)}
	for _, u := range users {
		store.users[u.ID] = u
	}

	return store
}

// store wraps [storage.Storage] for the sole purpose of injecting
// additional claims that it does not support into the user info
// and auth requests.
type store struct {
	*storage.Storage
	userStore userStore
}

func (s store) SetUserinfoFromToken(ctx context.Context, userinfo *oidc.UserInfo, tokenID, subject, origin string) error {
	if err := s.Storage.SetUserinfoFromToken(ctx, userinfo, tokenID, subject, origin); err != nil {
		return trace.Wrap(err)
	}

	// Inject any custom claims that may have
	// been specified for the user.
	u, ok := s.userStore.users[subject]
	if !ok || len(u.Claims) == 0 {
		return nil
	}

	if userinfo.Claims == nil {
		userinfo.Claims = map[string]any{}
	}

	for k, v := range u.Claims {
		if _, ok := userinfo.Claims[k]; !ok {
			userinfo.Claims[k] = v
		}
	}

	return nil
}

type authRequest struct {
	authTime time.Time
	acr      string
	op.AuthRequest
}

func (a authRequest) GetACR() string {
	return a.acr
}

func (a authRequest) GetAuthTime() time.Time {
	return a.authTime
}

func (s store) augmentAuthRequest(req op.AuthRequest) op.AuthRequest {
	storageReq, ok := req.(*storage.AuthRequest)
	if !ok {
		return req
	}

	// Inject any custom claims that may have
	// been specified for the user.
	u, ok := s.userStore.users[storageReq.UserID]
	if !ok || len(u.Claims) == 0 {
		return req
	}

	augmentedReq := authRequest{
		AuthRequest: req,
	}

	if acr, ok := u.Claims["acr"]; ok {
		switch v := acr.(type) {
		case string:
			augmentedReq.acr = v
		}
	}

	if authTime, ok := u.Claims["auth_time"]; ok {
		augmentedReq.authTime = time.Unix(int64(authTime.(float64)), 0)
	}

	return augmentedReq
}

func (s store) CreateAuthRequest(ctx context.Context, authReq *oidc.AuthRequest, userID string) (op.AuthRequest, error) {
	req, err := s.Storage.CreateAuthRequest(ctx, authReq, userID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return s.augmentAuthRequest(req), nil

}

func (s store) AuthRequestByCode(ctx context.Context, code string) (op.AuthRequest, error) {
	req, err := s.Storage.AuthRequestByCode(ctx, code)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return s.augmentAuthRequest(req), nil
}

func (s store) AuthRequestByID(ctx context.Context, id string) (op.AuthRequest, error) {
	req, err := s.Storage.AuthRequestByID(ctx, id)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return s.augmentAuthRequest(req), nil
}

type oidcSuiteOpts struct {
	users    []user
	insecure bool
	license  eauth.License
	clock    clocki.FakeClock
	proxy    func(http.Handler) http.Handler
	pkceMode string
}

func overridePKCEMode(mode string) func(*oidcSuiteOpts) {
	return func(opts *oidcSuiteOpts) {
		opts.pkceMode = mode
	}
}

func insecureOIDCSuite() func(*oidcSuiteOpts) {
	return func(opts *oidcSuiteOpts) {
		opts.insecure = true
	}
}

func overrideUsers(users []user) func(*oidcSuiteOpts) {
	return func(opts *oidcSuiteOpts) {
		opts.users = users
	}
}

func overrideLicense(license eauth.License) func(*oidcSuiteOpts) {
	return func(opts *oidcSuiteOpts) {
		opts.license = license
	}
}

func overrideClock(clock clocki.FakeClock) func(*oidcSuiteOpts) {
	return func(opts *oidcSuiteOpts) {
		opts.clock = clock
	}
}

func proxyOP(p func(http.Handler) http.Handler) func(*oidcSuiteOpts) {
	return func(opts *oidcSuiteOpts) {
		opts.proxy = p
	}
}

type OIDCSuite struct {
	authServer  *auth.Server
	backend     backend.Backend
	clock       clocki.FakeClock
	oidcService *eauth.OIDCAuthService
	emitter     *eventstest.MockRecorderEmitter
	idpServer   *httptest.Server
	connector   *types.OIDCConnectorV3
	store       *store
}

func newOIDCSuite(t *testing.T, opts ...func(*oidcSuiteOpts)) *OIDCSuite {
	ctx := context.Background()

	o := oidcSuiteOpts{
		license:  eauth.ValidLicense{},
		clock:    clockwork.NewFakeClock(),
		proxy:    func(h http.Handler) http.Handler { return h },
		pkceMode: "disabled",
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

	authConfig := &auth.InitConfig{
		VersionStorage:         authtest.NewFakeTeleportVersion(),
		ClusterName:            clusterName,
		Backend:                bk,
		Authority:              authority.New(),
		SkipPeriodicOperations: true,
		Clock:                  o.clock,
	}
	authServer, err := auth.NewServer(authConfig)
	require.NoError(t, err)

	emitter := &eventstest.MockRecorderEmitter{}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	s := &httptest.Server{
		Listener: ln,
		Config:   &http.Server{},
	}
	t.Cleanup(s.Close)

	defaultUsers := []user{
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
				"groups": []string{"access"},
			},
		},
		{
			User: &storage.User{
				ID:        "id2",
				Username:  "test-user2",
				Password:  "verysecure",
				FirstName: "Test",
				LastName:  "User2",
				Email:     "test-user2@example.com",
			},
			Claims: map[string]any{
				"groups": []string{"access"},
			},
		},
		{
			User: &storage.User{
				ID:            "id3",
				Username:      "test-user3",
				Password:      "verysecure",
				FirstName:     "Test",
				LastName:      "User3",
				Email:         "test-user3@example.com",
				EmailVerified: true,
			},
			Claims: map[string]any{
				"groups": []string{"access"},
			},
		},
		{
			User: &storage.User{
				ID:            "no-groups",
				Username:      "test-user4",
				Password:      "verysecure",
				FirstName:     "Test",
				LastName:      "User4",
				Email:         "test-user4@example.com",
				EmailVerified: true,
			},
			Claims: map[string]any{
				"animal": []string{"llama"},
			},
		},
	}

	if len(o.users) > 0 {
		defaultUsers = o.users
	}

	defaultClients := map[string]*storage.Client{
		"test": storage.WebClient("test", "secret", "http://example.com"),
	}

	usersStore := newUserStore(defaultUsers)
	opStore := &store{
		userStore: usersStore,
		Storage: storage.NewStorageWithClients(
			usersStore,
			defaultClients,
		),
	}
	oidcKeySet := &op.OpenIDKeySet{Storage: opStore}

	provider, err := op.NewProvider(
		&op.Config{
			CryptoKey:                sha256.Sum256([]byte("test")),
			DefaultLogoutRedirectURI: "/logged-out",
			CodeMethodS256:           true,
			AuthMethodPost:           true,
			AuthMethodPrivateKeyJWT:  true,
			GrantTypeRefreshToken:    true,
			RequestObjectSupported:   true,
			SupportedClaims:          op.DefaultSupportedClaims,
			DeviceAuthorization: op.DeviceAuthorizationConfig{
				Lifetime:     5 * time.Minute,
				PollInterval: 5 * time.Second,
				UserFormPath: "/device",
				UserCode:     op.UserCodeBase20,
			},
		},
		opStore,
		func(insecure bool) (op.IssuerFromRequest, error) {
			return func(r *http.Request) string {
				return s.URL
			}, nil
		},
		op.WithAllowInsecure(),
		op.WithAccessTokenKeySet(oidcKeySet),
		op.WithIDTokenHintKeySet(oidcKeySet),
	)
	require.NoError(t, err)

	s.Config.Handler = o.proxy(op.RegisterLegacyServer(op.NewLegacyServer(provider, *op.DefaultEndpoints), op.AuthorizeCallbackHandler(provider)))

	if o.insecure {
		s.Start()
	} else {
		s.StartTLS()
	}

	connector := &types.OIDCConnectorV3{
		Kind:    types.KindOIDCConnector,
		Version: types.V3,
		Metadata: types.Metadata{
			Name: "test-connector",
		},
		Spec: types.OIDCConnectorSpecV3{
			IssuerURL:    s.URL,
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
				s.URL + "/proxy/oidc/callback",
			},
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

	oidcService, err := eauth.NewOIDCAuthService(&eauth.OIDCAuthServiceConfig{
		Auth:    authServer,
		License: o.license,
		Emitter: emitter,
		Client:  s.Client(),
	})
	require.NoError(t, err)
	authServer.SetOIDCService(oidcService)

	_, err = authServer.CreateRole(ctx, services.NewPresetAccessRole())
	require.NoError(t, err)

	return &OIDCSuite{
		authServer:  authServer,
		backend:     bk,
		clock:       o.clock,
		oidcService: oidcService,
		emitter:     emitter,
		idpServer:   s,
		connector:   c.(*types.OIDCConnectorV3),
		store:       opStore,
	}
}

func (s *OIDCSuite) authenticateUser(ctx context.Context, user string, req types.OIDCAuthRequest) (string, *authclient.OIDCAuthResponse, error) {
	authRequest, err := s.oidcService.CreateOIDCAuthRequest(ctx, req)
	if err != nil {
		return "", nil, err
	}

	codeChallenge := oauth2.S256ChallengeFromVerifier(req.PkceVerifier)
	request, err := s.store.CreateAuthRequest(
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

	if err := s.store.SaveAuthCode(ctx, request.GetID(), "test"); err != nil {
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
	if err := s.authServer.UpsertSSOMFASessionData(ctx, sd); err != nil {
		return nil, err
	}

	request, err := s.store.CreateAuthRequest(
		ctx,
		&oidc.AuthRequest{
			Scopes:      []string{oidc.ScopeOpenID, oidc.ScopeEmail, oidc.ScopeProfile},
			ClientID:    s.connector.GetClientID(),
			RedirectURI: s.connector.GetRedirectURLs()[0],
			State:       authRequest.StateToken,
			Display:     "none",
		},
		user,
	)
	if err != nil {
		return nil, err
	}

	if err := s.store.SaveAuthCode(ctx, request.GetID(), "test"); err != nil {
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
	suite := newOIDCSuite(t)

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
	suite := newOIDCSuite(t, overridePKCEMode("enabled"))

	tests := []struct {
		name           string
		userID         string
		createSession  bool
		testFlow       bool
		noCodeVerifier bool
		q              url.Values
		assertion      func(t *testing.T, resp *authclient.OIDCAuthResponse, err error)
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
			},
		},
		{
			name:   "no claims to roles",
			userID: "no-groups",
			assertion: func(t *testing.T, resp *authclient.OIDCAuthResponse, err error) {
				require.ErrorContains(t, err, "No roles mapped from claims")
				require.Nil(t, resp)
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

			_, resp, err := suite.authenticateUser(ctx, test.userID, req)
			test.assertion(t, resp, err)
		})
	}
}

func TestOIDCUserCreation(t *testing.T) {
	t.Parallel()
	suite := newOIDCSuite(t)
	ctx := context.Background()

	_, _, err := suite.authenticateUser(ctx, "id1", types.OIDCAuthRequest{
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

// TestUserInfoBlockHTTP ensures that an insecure userinfo endpoint returns
// trace.NotFound similar to an invalid userinfo endpoint. For these users,
// all claim information is already within the token and additional claim
// information does not need to be fetched.
func TestOIDCBlockHTTPUserInfo(t *testing.T) {
	t.Parallel()

	suite := newOIDCSuite(t, insecureOIDCSuite())

	ctx := context.Background()

	_, _, err := suite.authenticateUser(ctx, "id1", types.OIDCAuthRequest{
		ConnectorID: suite.connector.GetName(),
		CheckUser:   true,
		CertTTL:     time.Minute,
	})
	// The email_verified claim for the user is only populated from the information retrieved
	// via the user info endpoint. This validates that when the user info endpoint
	// is insecure that we do not enrich the user and the appropriate error is returned.
	require.ErrorContains(t, err, "email not verified by OIDC provider")

	suite.connector.Spec.AllowUnverifiedEmail = true
	_, err = suite.authServer.UpdateOIDCConnector(ctx, suite.connector)
	require.NoError(t, err)

	_, _, err = suite.authenticateUser(ctx, "id1", types.OIDCAuthRequest{
		ConnectorID: suite.connector.GetName(),
		CheckUser:   true,
		CertTTL:     time.Minute,
	})

	// The role mapping claims for the user are only populated from the information retrieved
	// via the user info endpoint. This validates that when the user info endpoint
	// is insecure that we do not enrich the user and the appropriate error is returned.
	require.ErrorContains(t, err, "No roles mapped from claims")
}

// TestUserInfoBadStatus asserts that a 4xx response from userinfo results
// in claims only being take from the id token.
func TestUserInfoBadStatus(t *testing.T) {
	t.Parallel()

	// Tests provide no claims in the id token that are mapped to roles.
	// When userinfo requests fail, but do not abort authentication, it's
	// expected that the error returned indicates no roles were mapped from
	// the limited claims.
	mappingError := require.ErrorAssertionFunc(func(t require.TestingT, err error, i ...any) {
		require.ErrorContains(t, err, "No roles mapped from claims", i...)
	})

	tests := []struct {
		name       string
		statusCode int
		assertion  require.ErrorAssertionFunc
	}{
		{
			name:       http.StatusText(http.StatusInternalServerError),
			statusCode: http.StatusInternalServerError,
			assertion:  require.Error,
		},
		{
			name:       http.StatusText(http.StatusBadRequest),
			statusCode: http.StatusBadRequest,
			assertion:  mappingError,
		},
		{
			name:       http.StatusText(http.StatusUnauthorized),
			statusCode: http.StatusUnauthorized,
			assertion:  mappingError,
		}, {
			name:       http.StatusText(http.StatusForbidden),
			statusCode: http.StatusForbidden,
			assertion:  mappingError,
		}, {
			name:       http.StatusText(http.StatusMethodNotAllowed),
			statusCode: http.StatusMethodNotAllowed,
			assertion:  mappingError,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			suite := newOIDCSuite(t, proxyOP(func(h http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if strings.Contains(r.URL.Path, "userinfo") {
						w.WriteHeader(test.statusCode)
						return
					}

					h.ServeHTTP(w, r)

				})
			}))

			ctx := context.Background()
			suite.connector.Spec.AllowUnverifiedEmail = true
			_, err := suite.authServer.UpdateOIDCConnector(ctx, suite.connector)
			require.NoError(t, err)

			_, _, err = suite.authenticateUser(ctx, "id1", types.OIDCAuthRequest{
				ConnectorID: suite.connector.GetName(),
				CheckUser:   true,
			})
			test.assertion(t, err)
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
		user            user
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
			user: user{
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
			user: user{
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
			user: user{
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

			suite := newOIDCSuite(t, overrideUsers([]user{
				tc.user,
			}))
			suite.connector.Spec.ClaimsToRoles = tc.claimsToRoles
			_, err := suite.authServer.UpdateOIDCConnector(ctx, suite.connector)
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
	rule := &loginrulepb.LoginRule{
		Metadata: &types.Metadata{
			Name: "testrule",
		},
		TraitsMap: make(map[string]*wrappers.StringValues),
	}
	for trait, values := range traitsMap {
		rule.TraitsMap[trait] = &wrappers.StringValues{
			Values: values,
		}
	}
	_, err := ruleStorage.CreateLoginRule(ctx, rule)
	require.NoError(t, err)
}

func TestEmailVerifiedClaim(t *testing.T) {
	t.Parallel()

	users := []user{
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

	suite := newOIDCSuite(t, overrideUsers(users))
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
			name:      "email not verified in user",
			userID:    users[1].ID,
			assertion: unverifiedErrorAssertion,
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

	users := []user{
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

	suite := newOIDCSuite(t, overrideUsers(users))
	ctx := context.Background()

	suite.connector.Spec.UsernameClaim = usernameClaim
	_, err := suite.authServer.UpdateOIDCConnector(ctx, suite.connector)
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

	suite := newOIDCSuite(t)
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
			suite := newOIDCSuite(t, overrideUsers([]user{
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
			}))

			suite.connector.Spec.Provider = test.provider
			suite.connector.Spec.ACR = test.acrValue

			ctx := context.Background()
			_, err := suite.authServer.UpdateOIDCConnector(ctx, suite.connector)
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
		license     eauth.License
		expectError bool
	}{
		{
			name:    "valid license",
			license: eauth.ValidLicense{},
		},
		{
			name:        "disabled license",
			license:     eauth.DisabledLicense{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			suite := newOIDCSuite(t, overrideLicense(tt.license))

			req := types.OIDCAuthRequest{ConnectorID: suite.connector.GetName(), Type: constants.OIDC}
			_, err := suite.oidcService.CreateOIDCAuthRequest(ctx, req)
			if tt.expectError {
				require.Error(t, err)
				require.True(t, trace.IsAccessDenied(err), "expected access denied, got: %v", err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateOIDCResponseMFA(t *testing.T) {
	modulestest.SetTestModules(t, modulestest.Modules{
		TestFeatures: modules.Features{Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
			entitlements.OIDC: {Enabled: true},
		}},
	})

	clock := clockwork.NewFakeClock()
	suite := newOIDCSuite(t,
		overrideClock(clock),
		overrideUsers([]user{
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

	suite.connector.Spec.MFASettings = &types.OIDCConnectorMFASettings{
		Enabled:      true,
		ClientId:     suite.connector.GetClientID(),
		ClientSecret: suite.connector.GetClientSecret(),
	}

	ctx := context.Background()
	_, err := suite.authServer.UpdateOIDCConnector(ctx, suite.connector)
	require.NoError(t, err)

	for _, tt := range []struct {
		name              string
		mutateSessionData func(sd *services.SSOMFASessionData)
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
				sd, err := suite.authServer.GetSSOMFASessionData(ctx, token)
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

	suite := newOIDCSuite(t)
	suite.connector.Spec.IssuerURL = srv.URL

	ctx := context.Background()
	_, err := suite.authServer.UpdateOIDCConnector(ctx, suite.connector)
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

	suite := newOIDCSuite(t)

	mux := http.NewServeMux()
	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)

	u, err := url.Parse(srv.URL)
	require.NoError(t, err)

	params := u.Query()
	params.Add("a", "b")
	params.Add("c", "d")
	u.RawQuery = params.Encode()

	requestedURL := make(chan *url.URL, 1)
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
