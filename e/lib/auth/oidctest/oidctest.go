package oidctest

import (
	"context"
	"crypto/sha256"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"time"

	"github.com/gravitational/trace"
	"github.com/zitadel/oidc/v3/example/server/storage"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
)

// IdPServer is a test OIDC identity provider backed by an embedded test server.
// Use URL as the connector's issuer URL and Client to obtain an HTTP client that
// trusts the server's certificate.
type IdPServer struct {
	*httptest.Server
	store *store
}

// NewIdPServer creates an IdPServer and applies the provided options.
func NewIdPServer(opts ...func(*IdPServerOpts)) (*IdPServer, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s := &httptest.Server{
		Listener: ln,
		Config:   &http.Server{},
	}

	o := IdPServerOpts{
		proxyOP:       func(h http.Handler) http.Handler { return h },
		allowInsecure: false,
		users:         defaultUsers,
	}

	for _, opt := range opts {
		opt(&o)
	}

	defaultClients := map[string]*storage.Client{
		"test":         storage.WebClient("test", "secret", "http://example.com"),
		"mfa-no-roles": storage.WebClient("mfa-no-roles", "secret", "http://example.com"),
	}

	usersStore := newUserStore(o.users)
	opStore := newOpStore(usersStore, defaultClients)
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
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s.Config.Handler = o.proxyOP(op.RegisterLegacyServer(op.NewLegacyServer(provider, *op.DefaultEndpoints), op.AuthorizeCallbackHandler(provider)))

	if o.allowInsecure {
		s.Start()
	} else {
		s.StartTLS()
	}

	return &IdPServer{Server: s, store: opStore}, nil
}

// Authorize authorizes a request for a user. It hardcodes a "test" authorization code, stores, and returns it.
func (s *IdPServer) Authorize(ctx context.Context, req *oidc.AuthRequest, userID string) (string, error) {
	request, err := s.store.CreateAuthRequest(ctx, req, userID)
	if err != nil {
		return "", trace.Wrap(err)
	}

	// TODO(nixpig): Consider generating a UUID so server can handle multiple concurrent requests.
	const code = "test"
	if err := s.store.SaveAuthCode(ctx, request.GetID(), code); err != nil {
		return "", trace.Wrap(err)
	}

	return code, nil
}

// IdPServerOpts is used to configure the IdP server.
// Values are set using functional options passed to New.
type IdPServerOpts struct {
	users         []User
	allowInsecure bool
	proxyOP       func(http.Handler) http.Handler
}

// WithAllowInsecure allows the use of HTTP on the IdP server.
func WithAllowInsecure() func(*IdPServerOpts) {
	return func(o *IdPServerOpts) {
		o.allowInsecure = true
	}
}

// WithUsers overrides the default users on the IdP server.
func WithUsers(users []User) func(*IdPServerOpts) {
	return func(o *IdPServerOpts) {
		o.users = users
	}
}

// WithProxyOP sets a proxy HTTP hander on the IdP server.
func WithProxyOP(p func(http.Handler) http.Handler) func(*IdPServerOpts) {
	return func(o *IdPServerOpts) {
		o.proxyOP = p
	}
}

// User wraps a [storage.User] with additional claims.
type User struct {
	*storage.User
	Claims map[string]any
}

type userStore struct {
	users map[string]User
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

func newUserStore(users []User) userStore {
	store := userStore{users: make(map[string]User)}
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
	userStore      userStore
	tokenToRequest map[string]op.TokenRequest
}

func newOpStore(userStore userStore, clients map[string]*storage.Client) *store {
	return &store{
		userStore:      userStore,
		Storage:        storage.NewStorageWithClients(userStore, clients),
		tokenToRequest: map[string]op.TokenRequest{},
	}
}

// Override 'CreateAccessToken' so that we can snoop token requests.
func (s store) CreateAccessToken(ctx context.Context, request op.TokenRequest) (string, time.Time, error) {
	tokenID, time, err := s.Storage.CreateAccessToken(ctx, request)
	if err == nil {
		s.tokenToRequest[tokenID] = request
	}
	return tokenID, time, err
}

// SetUserinfoFromRequest is used to override id_token claims.
func (s store) SetUserinfoFromRequest(ctx context.Context, userinfo *oidc.UserInfo, token op.IDTokenRequest, scopes []string) error {
	if err := s.Storage.SetUserinfoFromRequest(ctx, userinfo, token, scopes); err != nil {
		return trace.Wrap(err)
	}

	if u, ok := s.userStore.users[userinfo.Subject]; ok {
		userinfo.Email = u.Email
		userinfo.EmailVerified = oidc.Bool(u.EmailVerified)
	}

	// Special case for testing id_token with claims.
	if userinfo.Subject == "user-with-custom-claim" ||
		userinfo.Subject == "user-with-empty-group-claim" {
		return s.setClaims(userinfo)
	}

	return nil
}

func (s store) SetUserinfoFromToken(ctx context.Context, userinfo *oidc.UserInfo, tokenID, subject, origin string) error {
	if err := s.Storage.SetUserinfoFromToken(ctx, userinfo, tokenID, subject, origin); err != nil {
		return trace.Wrap(err)
	}

	// Special case. The client "mfa-no-roles" should skip
	// custom claims enrichment.
	if request, ok := s.tokenToRequest[tokenID]; ok {
		if slices.Contains(request.GetAudience(), "mfa-no-roles") {
			return nil
		}
	}

	// Inject any custom claims that may have
	// been specified for the user.
	return s.setClaims(userinfo)
}

// setClaims configured in user storage.
func (s store) setClaims(userinfo *oidc.UserInfo) error {
	if userinfo.Claims == nil {
		userinfo.Claims = make(map[string]any)
	}
	u, ok := s.userStore.users[userinfo.Subject]
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

// defaultUsers is the list of users added to the IdPServer when none is provided.
var defaultUsers = []User{
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
			"groups":         []string{"access"},
			"email_verified": false,
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
	{
		User: &storage.User{
			ID:            "user-with-custom-claim",
			Username:      "user-with-custom-claim",
			Password:      "verysecure",
			FirstName:     "Test group",
			LastName:      "User",
			Email:         "user-with-custom-claim@example.com",
			EmailVerified: true,
			IsAdmin:       true,
		},
		Claims: map[string]any{
			"groups": []string{"access"},
			"email":  "user-with-custom-claim@example.com",
		},
	},
	{
		User: &storage.User{
			ID:            "user-with-empty-group-claim",
			Username:      "user-with-empty-group-claim",
			Password:      "verysecure",
			FirstName:     "Test empty",
			LastName:      "User",
			Email:         "user-with-empty-group-claim@example.com",
			EmailVerified: true,
			IsAdmin:       true,
		},
		Claims: map[string]any{
			"groups": []string{},
		},
	},
}
