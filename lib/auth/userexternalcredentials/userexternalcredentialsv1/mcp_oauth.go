/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package userexternalcredentialsv1

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/oauth2"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userexternalcredentials/v1"
	"github.com/gravitational/teleport/api/scopes"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/credentialencryption"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
)

type mcpOAuthCredential struct {
	ClientID      string `json:"client_id"`
	ClientSecret  string `json:"client_secret,omitempty"`
	TokenEndpoint string `json:"token_endpoint"`
	AppURI        string `json:"app_uri"`
	// Resource is the RFC 8707 resource indicator if any.
	Resource string        `json:"resource,omitempty"`
	Token    *oauth2.Token `json:"token"`
}

// ExchangeMCPOAuthCode exchanges an authorization code and stores the tokens
// for the caller. If the caller is a beam, we store them for the beam's owner.
func (s *Service) ExchangeMCPOAuthCode(ctx context.Context, req *pb.ExchangeMCPOAuthCodeRequest) (*pb.ExchangeMCPOAuthCodeResponse, error) {
	if req.GetAppName() == "" {
		return nil, trace.BadParameter("missing app_name")
	}
	if req.GetAppUri() == "" {
		return nil, trace.BadParameter("missing app_uri")
	}

	authCtx, err := s.cfg.ScopedAuthorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	identity := authCtx.Identity.GetIdentity()
	owner, err := credentialOwner(&identity)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := s.writeLimiter.RegisterRequest(owner); err != nil {
		return nil, trace.Wrap(err)
	}
	app := scopes.QualifiedName{Name: req.GetAppName(), Scope: req.GetAppScope()}
	if err := s.checkMCPAppAccess(ctx, authCtx, app, req.GetAppUri(), req.GetClientId()); err != nil {
		return nil, trace.Wrap(err)
	}

	cred := &mcpOAuthCredential{
		ClientID:      req.GetClientId(),
		ClientSecret:  req.GetClientSecret(),
		TokenEndpoint: req.GetTokenEndpoint(),
		AppURI:        req.GetAppUri(),
		Resource:      req.GetResource(),
	}
	config := &oauth2.Config{
		ClientID:     cred.ClientID,
		ClientSecret: cred.ClientSecret,
		Endpoint:     oauth2.Endpoint{TokenURL: cred.TokenEndpoint},
		RedirectURL:  req.GetRedirectUri(),
	}
	opts := []oauth2.AuthCodeOption{oauth2.VerifierOption(req.GetCodeVerifier())}
	if req.GetResource() != "" {
		opts = append(opts, oauth2.SetAuthURLParam("resource", req.GetResource()))
	}
	exchangeCtx := context.WithValue(ctx, oauth2.HTTPClient, s.cfg.mcpHTTPClient)
	if cred.Token, err = config.Exchange(exchangeCtx, req.GetCode(), opts...); err != nil {
		s.cfg.Logger.DebugContext(ctx, "Failed to exchange MCP OAuth authorization code",
			"app", app.String(),
			"error", mcpOAuthProviderError(err),
		)
		return nil, trace.BadParameter("failed to exchange the authorization code for MCP server %q", app)
	}

	// a refresh that's already running would overwrite this login with the old one
	err = s.cfg.Backend.RunWhileMCPOAuthCredentialLocked(ctx, owner, app, func(ctx context.Context) error {
		return trace.Wrap(s.putMCPOAuthCredential(ctx, owner, app, cred))
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s.cfg.Logger.DebugContext(ctx, "Stored MCP OAuth credential",
		"owner", owner,
		"initiator", identity.Username,
		"app", app.String(),
	)
	return &pb.ExchangeMCPOAuthCodeResponse{}, nil
}

// checkMCPAppAccess runs before we call the token endpoint because the caller
// chooses that endpoint.
func (s *Service) checkMCPAppAccess(ctx context.Context, authCtx *authz.ScopedContext, sqn scopes.QualifiedName, uri, clientID string) error {
	for server, err := range s.cfg.Cache.RangeApplicationServersWithName(ctx, sqn.Name) {
		if err != nil {
			return trace.Wrap(err)
		}
		// names are only unique within a scope
		if server.GetScope() != sqn.Scope {
			continue
		}
		app := server.GetApp()
		if types.GetMCPServerTransportType(app.GetURI()) != types.MCPTransportHTTP {
			return trace.AccessDenied("no access to MCP server %q", sqn)
		}
		// not checking MFA here, we check it when connecting to the app
		if _, err := authCtx.CheckerContext.Decision(ctx, app.GetScope(), func(checker *services.ScopedAccessChecker) error {
			return trace.Wrap(checker.App().CanAccessApp(app))
		}); err != nil {
			return trace.AccessDenied("no access to MCP server %q", sqn)
		}
		if app.GetURI() != uri {
			return trace.BadParameter("MCP server %q does not have URI %q", sqn, uri)
		}
		// if the app has a client set, users can't pick their own
		if mcp := app.GetMCP(); mcp != nil && mcp.Oauth != nil && mcp.Oauth.ClientId != "" && mcp.Oauth.ClientId != clientID {
			return trace.BadParameter("MCP server %q does not use OAuth client %q", sqn, clientID)
		}
		return nil
	}
	return trace.AccessDenied("no access to MCP server %q", sqn)
}

// GetMCPOAuthAccessToken returns the plaintext access token for the certificate's
// user and the app it's routed to.
func (s *Service) GetMCPOAuthAccessToken(ctx context.Context, req *pb.GetMCPOAuthAccessTokenRequest) (*pb.GetMCPOAuthAccessTokenResponse, error) {
	authCtx, err := s.cfg.ScopedAuthorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !authCtx.HasBuiltinRole(types.RoleProxy) {
		return nil, trace.AccessDenied("only proxies can get MCP OAuth access tokens")
	}
	identity, err := s.verifyUserCertificate(ctx, req.GetUserCertificate())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	app := scopes.QualifiedName{Name: identity.RouteToApp.Name, Scope: identity.RouteToApp.Scope}
	if app.Name == "" {
		return nil, trace.BadParameter("user certificate has no app route")
	}
	owner, err := credentialOwner(identity)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cred, err := s.getMCPOAuthCredential(ctx, owner, app)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if s.needsMCPOAuthRefresh(cred) {
		refreshed, err := s.refreshMCPOAuthCredential(ctx, owner, app)
		switch {
		case err == nil:
			cred = refreshed
		case cred.Token.Expiry.Before(s.cfg.Clock.Now()):
			return nil, trace.Wrap(err)
		default:
			// the token still works, so a failed refresh shouldn't stop the user early
			s.cfg.Logger.WarnContext(ctx, "Failed to refresh MCP OAuth token, using the current one until it expires",
				"owner", owner,
				"app", app.String(),
				"error", err,
			)
		}
	}
	// without this the Proxy would ask us again on every request
	if !cred.Token.Expiry.IsZero() && cred.Token.Expiry.Before(s.cfg.Clock.Now()) {
		return nil, trace.NotFound("the MCP OAuth login for app %q has expired", app)
	}

	resp := pb.GetMCPOAuthAccessTokenResponse_builder{
		AccessToken: cred.Token.AccessToken,
		AppUri:      cred.AppURI,
	}.Build()
	if !cred.Token.Expiry.IsZero() {
		resp.SetExpires(timestamppb.New(cred.Token.Expiry))
	}
	return resp, nil
}

func (s *Service) needsMCPOAuthRefresh(cred *mcpOAuthCredential) bool {
	window := services.MCPOAuthRefreshWindow
	// a token this short-lived would otherwise need a refresh as soon as we get it
	if lifetime := time.Duration(cred.Token.ExpiresIn) * time.Second; lifetime > 0 {
		window = min(window, lifetime/2)
	}
	return cred.Token.RefreshToken != "" &&
		!cred.Token.Expiry.IsZero() &&
		cred.Token.Expiry.Before(s.cfg.Clock.Now().Add(window))
}

// refreshMCPOAuthCredential holds a lock because concurrent refreshes would spend a
// rotating refresh token twice, and the loser's request would get a 401.
func (s *Service) refreshMCPOAuthCredential(ctx context.Context, owner string, app scopes.QualifiedName) (*mcpOAuthCredential, error) {
	var cred *mcpOAuthCredential
	err := s.cfg.Backend.RunWhileMCPOAuthCredentialLocked(ctx, owner, app, func(ctx context.Context) error {
		var err error
		if cred, err = s.getMCPOAuthCredential(ctx, owner, app); err != nil {
			return trace.Wrap(err)
		}
		// someone else can have refreshed it while we waited for the lock
		if !s.needsMCPOAuthRefresh(cred) {
			return nil
		}
		refreshCtx := context.WithValue(ctx, oauth2.HTTPClient, s.mcpRefreshHTTPClient(cred.Resource))
		config := &oauth2.Config{
			ClientID:     cred.ClientID,
			ClientSecret: cred.ClientSecret,
			Endpoint:     oauth2.Endpoint{TokenURL: cred.TokenEndpoint},
		}
		token, err := config.TokenSource(refreshCtx, &oauth2.Token{RefreshToken: cred.Token.RefreshToken}).Token()
		if err != nil {
			s.cfg.Logger.DebugContext(ctx, "Failed to refresh MCP OAuth token",
				"owner", owner,
				"app", app.String(),
				"error", mcpOAuthProviderError(err),
			)
			// the Proxy treats NotFound as no token, so the user gets the app's 401
			// TODO(avatus): delete the rejected credential here once we have a delete RPC
			if retrieveErr, ok := errors.AsType[*oauth2.RetrieveError](err); ok && retrieveErr.ErrorCode == "invalid_grant" {
				return trace.NotFound("the MCP OAuth login for app %q is no longer valid", app)
			}
			return trace.Errorf("failed to refresh the MCP OAuth token for app %q", app)
		}
		cred.Token = token
		if err := s.putMCPOAuthCredential(ctx, owner, app, cred); err != nil {
			return trace.Wrap(err)
		}
		s.cfg.Logger.DebugContext(ctx, "Refreshed MCP OAuth token", "owner", owner, "app", app.String(), "expiry", cred.Token.Expiry)
		return nil
	})
	return cred, trace.Wrap(err)
}

// mcpOAuthProviderError leaves out the response body because it could have anything
// in it.
func mcpOAuthProviderError(err error) string {
	if retrieveErr, ok := errors.AsType[*oauth2.RetrieveError](err); ok {
		return retrieveErr.Response.Status + " " + retrieveErr.ErrorCode
	}
	return err.Error()
}

// oauth2's TokenSource can't send the resource indicator, so we add it here.
func (s *Service) mcpRefreshHTTPClient(resource string) *http.Client {
	if resource == "" {
		return s.cfg.mcpHTTPClient
	}
	client := *s.cfg.mcpHTTPClient
	client.Transport = mcpOAuthResourceTransport{base: client.Transport, resource: resource}
	return &client
}

type mcpOAuthResourceTransport struct {
	base     http.RoundTripper
	resource string
}

func (t mcpOAuthResourceTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	form, err := url.ParseQuery(string(body))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	form.Set("resource", t.resource)
	encoded := form.Encode()
	r = r.Clone(r.Context())
	r.Body = io.NopCloser(strings.NewReader(encoded))
	r.ContentLength = int64(len(encoded))
	return t.base.RoundTrip(r)
}

func (s *Service) getMCPOAuthCredential(ctx context.Context, owner string, app scopes.QualifiedName) (*mcpOAuthCredential, error) {
	encrypted, err := s.cfg.Backend.GetMCPOAuthCredential(ctx, owner, app)
	if trace.IsNotFound(err) {
		return nil, trace.NotFound("no MCP OAuth credential for app %q", app)
	} else if err != nil {
		return nil, trace.Wrap(err)
	}
	// only here so sessions without a login don't use up the limit
	if err := s.decryptLimiter.RegisterRequest(owner); err != nil {
		return nil, trace.Wrap(err)
	}
	payload, err := s.cfg.Encryptor.DecryptPayload(ctx, encrypted)
	if err != nil {
		return nil, trace.Wrap(err, "decrypting MCP OAuth credential")
	}
	// sure, we may have just pulled it by the key, but should check the decrypted payload to ensure
	// it belongs to the user and app we're looking for
	if payload.User != owner || payload.ResourceKind != types.KindApp || payload.ResourceName != app.String() {
		return nil, trace.AccessDenied("MCP OAuth credential does not belong to user %q and app %q", owner, app)
	}
	var cred mcpOAuthCredential
	if err := json.Unmarshal(payload.Credential, &cred); err != nil {
		return nil, trace.Wrap(err)
	}
	if cred.Token == nil {
		return nil, trace.BadParameter("MCP OAuth credential has no token")
	}
	return &cred, nil
}

func (s *Service) putMCPOAuthCredential(ctx context.Context, owner string, app scopes.QualifiedName, cred *mcpOAuthCredential) error {
	plaintext, err := json.Marshal(cred)
	if err != nil {
		return trace.Wrap(err)
	}
	encrypted, err := s.cfg.Encryptor.EncryptPayload(ctx, &credentialencryption.Payload{
		User:         owner,
		ResourceKind: types.KindApp,
		ResourceName: app.String(),
		Credential:   plaintext,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(s.cfg.Backend.UpsertMCPOAuthCredential(ctx, owner, app, encrypted))
}
