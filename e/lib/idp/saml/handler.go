/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package saml

import (
	"context"
	"net/http"
	"strings"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/tlsca"
)

type idpContextKey string

const (
	// IdPRoute is the route to the IdP on the server.
	//
	//nolint:revive // Because we want this to be IdP.
	IdPRoute = "/enterprise/saml-idp"

	identityContextKey idpContextKey = "saml-idp-identity"
)

// ServeHTTP serves the IdP endpoints.
func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	handler := s.handler()

	if handler == nil {
		s.writeError(w, http.StatusNotFound)
		return
	}

	r.URL.Path = strings.TrimPrefix(r.URL.Path, IdPRoute)
	handler.ServeHTTP(w, r)
}

// initRouter initializes the HTTP router for the SAML IdP endpoints.
func (s *Service) initRouter() (*httprouter.Router, error) {
	router := httprouter.New()

	router.GET("/metadata", s.withAuthCtx(s.handleMetadata))
	router.GET("/sso", s.withAuthCtx(s.handleSSO))
	router.POST("/sso", s.withAuthCtx(s.handleSSO))

	// Handle logins.
	router.GET("/login/:shortcut", s.withAuthCtx(s.handleIdPInitiatedLogin))
	router.GET("/login/:shortcut/*urlsuffix", s.withAuthCtx(s.handleIdPInitiatedLogin))
	router.POST("/login/:shortcut", s.withAuthCtx(s.handleIdPInitiatedLogin))
	router.POST("/login/:shortcut/*urlsuffix", s.withAuthCtx(s.handleIdPInitiatedLogin))

	return router, nil
}

func (s *Service) withAuthCtx(fn httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		identity, err := s.authorize(r.Context())
		if err != nil {
			if !trace.IsAccessDenied(err) { // access denied are expected
				s.log.Errorf("error authorizing user for SAML IdP: %s", err.Error())
			}
			s.writeError(w, http.StatusNotFound)
			return
		}
		fn(w, r.WithContext(s.ctxWithIdentity(r.Context(), identity)), p)
	}
}

func (s *Service) authorize(ctx context.Context) (*tlsca.Identity, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authPref, err := s.accessPoint.GetAuthPreference(ctx)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	// If the auth preference is not found, the CheckAccessToSAMLIdP function will handle it.
	if err := authCtx.Checker.CheckAccessToSAMLIdP(authPref); err != nil {
		return nil, trace.Wrap(err)
	}

	// Only allow local users to use the SAML IdP.
	var identity tlsca.Identity
	switch user := authCtx.Identity.(type) {
	case auth.LocalUser:
		identity = user.GetIdentity()
	default:
		return nil, trace.BadParameter("unsupported user type: %T", user)
	}

	// If the user's cert is expired, return immediately.
	if s.clock.Now().After(identity.Expires) {
		return nil, trace.AccessDenied("identity is expired")
	}

	return &identity, nil
}

// handleMetadata handles metadata requests.
func (s *Service) handleMetadata(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	s.idpMutex.RLock()
	defer s.idpMutex.RUnlock()
	s.idp.ServeMetadata(w, r) // The saml.IdentityProvider does the response handling here.
}

// handleSSO handles SSO requests.
func (s *Service) handleSSO(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	s.idpMutex.RLock()
	defer s.idpMutex.RUnlock()
	s.idp.ServeSSO(w, r) // The saml.IdentityProvider does the response handling here.
}

// handleIdPInitiatedLogin will handle IdP initiated logins for a service provider.
//
//nolint:revive // Because we want this to be IdP.
func (s *Service) handleIdPInitiatedLogin(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	shortcutName := p.ByName("shortcut")
	if shortcutName == "" {
		s.log.Error("IdP initiated SSO is missing the shortcut parameter")
		s.writeError(w, http.StatusBadRequest)
		return
	}

	sp, err := s.accessPoint.GetSAMLIdPServiceProvider(r.Context(), shortcutName)
	if err != nil {
		s.log.Errorf("Unable to find service provider %s: %v", shortcutName, err)
		s.writeError(w, http.StatusInternalServerError)
		return
	}

	// TODO (mdwn): Implement configurable relay state.
	s.idpMutex.RLock()
	defer s.idpMutex.RUnlock()
	// The saml.IdentityProvider does the response handling here.
	s.idp.ServeIDPInitiated(w, r, sp.GetEntityID(), "" /* empty relay state for now */)
}

// handler returns the HTTP handler for the identity provider.
func (s *Service) handler() http.Handler {
	s.idpMutex.RLock()
	defer s.idpMutex.RUnlock()
	return s.idpHandler
}

// writeError writes an error response.
func (s *Service) writeError(w http.ResponseWriter, code int) {
	http.Error(w, http.StatusText(code), code)
}

// ctxWithIdentity will return the context with the given identity.
func (s *Service) ctxWithIdentity(ctx context.Context, identity *tlsca.Identity) context.Context {
	return context.WithValue(ctx, identityContextKey, identity)
}

// getIdentityFromCtx will return the identity from the given context.
func (s *Service) getIdentityFromCtx(ctx context.Context) (*tlsca.Identity, error) {
	identity, ok := ctx.Value(identityContextKey).(*tlsca.Identity)
	if !ok {
		s.log.Debugf("identity is not the expected type, got %T", identity)
		return nil, trace.NotFound("identity not found")
	}
	return identity, nil
}
