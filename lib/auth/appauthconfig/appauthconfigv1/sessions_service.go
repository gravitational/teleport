// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package appauthconfigv1

import (
	"cmp"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/gravitational/trace"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/gravitational/teleport"
	appauthconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/appauthconfig/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// jwtMaxIssuedAtAfter is the amount of time that Teleport will still accept
	// a JWT token after it was issued.
	jwtMaxIssuedAtAfter = 30 * time.Minute
	// defaultUsernameClaim is the default JWT claim used as username.
	defaultUsernameClaim = "email"
)

// CreateAppSessionForAppAuthRequest defines the request params for `CreateAppSessionForAppAuth`.
type CreateAppSessionForAppAuthRequest struct {
	// ClusterName is cluster within which the application is running.
	ClusterName string
	// Username is the identity of the user requesting the session.
	Username string
	// LoginIP is an observed IP of the client, it will be embedded into certificates.
	LoginIP string
	// Roles optionally lists additional user roles
	Roles []string
	// Traits optionally lists role traits
	Traits map[string][]string
	// TTL is the session validity period.
	TTL time.Duration
	// SuggestedSessionID is the session ID suggested by the requester.
	SuggestedSessionID string
	// AppName is the name of the app.
	AppName string
	// AppURI is the URI of the app. This is the internal endpoint where the application is running and isn't user-facing.
	AppURI string
	// AppPublicAddr is the application public address.
	AppPublicAddr string
}

// AppSessionCreator creates new app sessions.
type AppSessionCreator interface {
	// CreateAppSessionForAppAuth creates a new session for app auth requests.
	CreateAppSessionForAppAuth(ctx context.Context, req *CreateAppSessionForAppAuthRequest) (types.WebSession, error)
}

// Service implements the teleport.appauthconfig.v1.AppAuthConfigSessionsServiceServer
// gRPC API.
type SessionsService struct {
	appauthconfigv1.UnimplementedAppAuthConfigSessionsServiceServer

	authorizer authz.Authorizer
	cache      services.AppAuthConfigReader
	emitter    apievents.Emitter
	sessions   AppSessionCreator
	httpClient *http.Client
	logger     *slog.Logger
	userGetter services.UserOrLoginStateGetter
}

// SessionsServiceConfig holds configuration options for [SessionsService].
type SessionsServiceConfig struct {
	// Authorizer used by the service.
	Authorizer authz.Authorizer
	// Reader is the cache used to store app auth config resources.
	Reader services.AppAuthConfigReader
	// Emitter is an audit event emitter.
	Emitter apievents.Emitter
	// Logger is the slog logger.
	Logger *slog.Logger
	// SessionCreator is used to create app sessions.
	SessionsCreator AppSessionCreator
	// UserGetter is used to retrieve the user.
	UserGetter services.UserOrLoginStateGetter
	// HTTPClient is a http client used to make external HTTP requests.
	HTTPClient *http.Client
}

// NewService creates a new instance of [SessionsService].
func NewSessionsService(cfg SessionsServiceConfig) (*SessionsService, error) {
	switch {
	case cfg.Authorizer == nil:
		return nil, trace.BadParameter("authorizer is required for app auth config sessions service")
	case cfg.Reader == nil:
		return nil, trace.BadParameter("cache is required for app auth config sessions service")
	case cfg.Emitter == nil:
		return nil, trace.BadParameter("emitter is required for app auth config sessions service")
	case cfg.HTTPClient == nil:
		transport, err := defaults.Transport()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		cfg.HTTPClient = &http.Client{
			Transport: otelhttp.NewTransport(transport),
			Timeout:   defaults.HTTPRequestTimeout,
		}
	case cfg.UserGetter == nil:
		return nil, trace.BadParameter("user getter is required for app auth config sessions service")
	}

	return &SessionsService{
		authorizer: cfg.Authorizer,
		cache:      cfg.Reader,
		emitter:    cfg.Emitter,
		sessions:   cfg.SessionsCreator,
		httpClient: cfg.HTTPClient,
		userGetter: cfg.UserGetter,
		logger:     cmp.Or(cfg.Logger, slog.Default()),
	}, nil
}

// CreateAppSessionWithJwt implements appauthconfigv1.AppAuthConfigSessionsServiceServer.
func (s *SessionsService) CreateAppSessionWithJWT(ctx context.Context, req *appauthconfigv1.CreateAppSessionWithJWTRequest) (_ *appauthconfigv1.CreateAppSessionWithJWTResponse, err error) {
	defer func() {
		if emitErr := s.emitter.EmitAuditEvent(ctx, newVerifyJWTAuditEvent(ctx, req, "", err)); emitErr != nil {
			s.logger.ErrorContext(ctx, "failed to emit jwt verification audit event", "error", emitErr)
		}
	}()

	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authCtx, string(types.RoleProxy)) {
		return nil, trace.AccessDenied("this request can be only executed by a proxy")
	}

	sid := services.GenerateAppSessionIDFromJWT(req.Jwt)
	if err := validateCreateAppSessionWithJWTRequest(req); err != nil {
		return nil, trace.Wrap(err)
	}

	config, err := s.cache.GetAppAuthConfig(ctx, req.ConfigName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	jwtConfig := config.Spec.GetJwt()
	if jwtConfig == nil {
		return nil, trace.BadParameter("app auth config jwt session can only start with jwt configs")
	}

	jwks, sigs, err := retrieveJWKSAppAuthConfig(jwtConfig, s.httpClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	username, tokenTTL, err := verifyAppAuthJWTToken(req.Jwt, jwks, sigs, jwtConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	user, err := services.GetUserOrLoginState(ctx, s.userGetter, username)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ws, err := s.sessions.CreateAppSessionForAppAuth(ctx, &CreateAppSessionForAppAuthRequest{
		Username:           username,
		ClusterName:        req.App.ClusterName,
		AppName:            req.App.AppName,
		AppURI:             req.App.Uri,
		AppPublicAddr:      req.App.PublicAddr,
		LoginIP:            req.RemoteAddr,
		Roles:              user.GetRoles(),
		Traits:             user.GetTraits(),
		TTL:                time.Until(tokenTTL),
		SuggestedSessionID: sid,
	})
	if err != nil {
		s.logger.WarnContext(ctx, "failed to create a web session from jwt token", "error", err)
		return nil, trace.Wrap(err)
	}

	return &appauthconfigv1.CreateAppSessionWithJWTResponse{Session: ws.(*types.WebSessionV2)}, nil
}

// retrieveJWKSAppAuthConfig retrieves JWKS contents given a app auth JWT
// config.
func retrieveJWKSAppAuthConfig(jwtConfig *appauthconfigv1.AppAuthConfigJWTSpec, httpClient *http.Client) (*jose.JSONWebKeySet, []jose.SignatureAlgorithm, error) {
	var rawJwks []byte
	switch jwksSource := jwtConfig.KeysSource.(type) {
	case *appauthconfigv1.AppAuthConfigJWTSpec_JwksUrl:
		req, err := http.NewRequest(http.MethodGet, jwksSource.JwksUrl, nil)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		defer resp.Body.Close()

		rawJwks, err = utils.ReadAtMost(resp.Body, teleport.MaxHTTPRequestSize)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
	case *appauthconfigv1.AppAuthConfigJWTSpec_StaticJwks:
		rawJwks = []byte(jwksSource.StaticJwks)
	}

	var jwks jose.JSONWebKeySet
	if err := json.Unmarshal(rawJwks, &jwks); err != nil {
		return nil, nil, trace.Wrap(err, "unable to parse JWKS contents")
	}

	sigs := make([]jose.SignatureAlgorithm, len(jwks.Keys))
	for i, jwk := range jwks.Keys {
		sigs[i] = jose.SignatureAlgorithm(jwk.Algorithm)
	}

	if len(sigs) == 0 {
		return nil, nil, trace.BadParameter("empty JWKS contents")
	}

	return &jwks, sigs, nil
}

// verifyAppAuthJWTToken verifies the provided JWT token using app auth config
// and returns the extracted username from JWT claims.
func verifyAppAuthJWTToken(jwtToken string, jwks *jose.JSONWebKeySet, sigs []jose.SignatureAlgorithm, jwtConfig *appauthconfigv1.AppAuthConfigJWTSpec) (string, time.Time, error) {
	parsedJWT, err := jwt.ParseSigned(jwtToken, sigs)
	if err != nil {
		return "", time.Time{}, trace.Wrap(err)
	}

	var (
		claims          jwt.Claims
		remainingClaims map[string]any
	)
	if err := parsedJWT.Claims(jwks, &claims, &remainingClaims); err != nil {
		return "", time.Time{}, trace.Wrap(err)
	}

	expected := jwt.Expected{
		Issuer:      jwtConfig.Issuer,
		AnyAudience: jwt.Audience{jwtConfig.Audience},
		Time:        time.Now(),
	}
	if err := claims.Validate(expected); err != nil {
		return "", time.Time{}, trace.Wrap(err)
	}

	if claims.IssuedAt == nil {
		return "", time.Time{}, trace.BadParameter("token must have 'iat' claim")
	}

	if claims.Expiry == nil {
		return "", time.Time{}, trace.BadParameter("token must have 'exp' claim")
	}

	issuedAt := claims.IssuedAt.Time()
	if time.Since(issuedAt) > jwtMaxIssuedAtAfter {
		return "", time.Time{}, trace.BadParameter("token must be issued recently to be used")
	}

	usernameClaimName := cmp.Or(jwtConfig.UsernameClaim, defaultUsernameClaim)
	usernameClaim, ok := remainingClaims[usernameClaimName]
	if !ok {
		return "", time.Time{}, trace.BadParameter("token must have %q claim", usernameClaimName)
	}

	usernameClaimStr, ok := usernameClaim.(string)
	if !ok {
		return "", time.Time{}, trace.BadParameter("token username claim %q must be of string type", usernameClaimName)
	}

	return usernameClaimStr, claims.Expiry.Time(), nil
}

func validateCreateAppSessionWithJWTRequest(req *appauthconfigv1.CreateAppSessionWithJWTRequest) error {
	switch {
	case req.ConfigName == "":
		return trace.BadParameter("create app session request requires an app auth config name")
	case req.App == nil:
		return trace.BadParameter("create app session request requires app information")
	case req.App.AppName == "":
		return trace.BadParameter("create app session request requires app name")
	case req.App.ClusterName == "":
		return trace.BadParameter("create app session request requires app cluster name")
	case req.App.PublicAddr == "":
		return trace.BadParameter("create app session request requires app public address")
	case req.App.Uri == "":
		return trace.BadParameter("create app session request requires app uri")
	}

	return nil
}
