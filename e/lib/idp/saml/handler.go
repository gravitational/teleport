package saml

import (
	"context"
	"net/http"
	"strings"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/tlsca"
)

type idpContextKey string

const (
	// IdPRoute is the route to the IdP on the server.
	//
	//nolint:revive // Because we want this to be IdP.
	IdPRoute = "/enterprise/saml-idp"

	identityContextKey   idpContextKey = "saml-idp-identity"
	spEntityIDContextKey idpContextKey = "saml-idp-sp-entity-id"
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

			var user string
			if identity != nil {
				user = identity.Username
			}
			s.emitAuthAttemptEvent(r.Context(), user, "", "", "", err)
			s.writeError(w, http.StatusNotFound)
			return
		}
		fn(w, r.WithContext(ctxWithIdentity(r.Context(), identity)), p)
	}
}

func (s *Service) authorize(ctx context.Context) (*tlsca.Identity, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Only allow local users to use the SAML IdP.
	var identity tlsca.Identity
	switch user := authCtx.Identity.(type) {
	case authz.LocalUser:
		identity = user.GetIdentity()
	default:
		identity = user.GetIdentity()
		return &identity, trace.BadParameter("unsupported user type: %T", user)
	}

	authPref, err := s.accessPoint.GetAuthPreference(ctx)
	if err != nil && !trace.IsNotFound(err) {
		return &identity, trace.Wrap(err)
	}

	// If the auth preference is not found, the CheckAccessToSAMLIdP function will handle it.
	if err := authCtx.Checker.CheckAccessToSAMLIdP(authPref); err != nil {
		return &identity, trace.Wrap(err)
	}

	// If the user's cert is expired, return immediately.
	if s.clock.Now().After(identity.Expires) {
		return &identity, trace.AccessDenied("identity is expired")
	}

	return &identity, nil
}

// handleMetadata handles metadata requests.
func (s *Service) handleMetadata(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	idp, err := s.createIdP(r.Context())
	if err != nil {
		s.log.Errorf("Error creating IdP: %v", err)
		s.writeError(w, http.StatusInternalServerError)
	}
	idp.ServeMetadata(w, r) // The saml.IdentityProvider does the response handling here.
}

// handleSSO handles SSO requests.
func (s *Service) handleSSO(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	idp, err := s.createIdP(r.Context())
	if err != nil {
		s.log.Errorf("Error creating IdP: %v", err)
		s.writeError(w, http.StatusInternalServerError)
	}
	idp.ServeSSO(w, r) // The saml.IdentityProvider does the response handling here.
}

// handleIdPInitiatedLogin will handle IdP initiated logins for a service provider.
//
//nolint:revive // Because we want this to be IdP.
func (s *Service) handleIdPInitiatedLogin(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	user, err := getUsernameFromCtx(r.Context())
	if err != nil {
		s.log.Warnf("Error getting username from context: %v", err)
	}
	shortcutName := p.ByName("shortcut")

	// It looks like this case isn't possible because the httprouter won't let this resolve
	// without the shortcut, but we'll check here just to be sure.
	if shortcutName == "" {
		s.emitAuthAttemptEvent(r.Context(), user, "", "", "", trace.NotFound("shortcut is empty"))
		s.writeError(w, http.StatusInternalServerError)
		return
	}

	sp, err := s.accessPoint.GetSAMLIdPServiceProvider(r.Context(), shortcutName)
	if err != nil {
		s.emitAuthAttemptEvent(r.Context(), user, "", "", shortcutName, err)
		s.writeError(w, http.StatusNotFound)
		return
	}

	if err := validateAssertionConsumerServices(sp); err != nil {
		s.emitAuthAttemptEvent(r.Context(), user, "", "", shortcutName, err)
		s.writeError(w, http.StatusNotFound)
		return
	}

	// embed the entity ID in the context because IdP initiated SSO doesn't put the
	// entity ID into the authnRequest until after the session is retrieved, which means
	// audit event emitted in the session provider doesn't have access to the entity ID.
	r = r.WithContext(ctxWithSPEntityID(r.Context(), sp.GetEntityID()))

	// TODO (mdwn): Implement configurable relay state.
	// The saml.IdentityProvider does the response handling here.
	idp, err := s.createIdP(r.Context())
	if err != nil {
		s.log.Errorf("Error creating IdP: %v", err)
		s.writeError(w, http.StatusInternalServerError)
	}
	idp.ServeIDPInitiated(w, r, sp.GetEntityID(), "" /* empty relay state for now */)
}

// handler returns the HTTP handler for the identity provider.
func (s *Service) handler() http.Handler {
	return s.idpHandler
}

// writeError writes an error response.
func (s *Service) writeError(w http.ResponseWriter, code int) {
	http.Error(w, http.StatusText(code), code)
}

// ctxWithIdentity will return the context with the given identity.
func ctxWithIdentity(ctx context.Context, identity *tlsca.Identity) context.Context {
	return context.WithValue(ctx, identityContextKey, identity)
}

// getIdentityFromCtx will return the identity from the given context.
func getIdentityFromCtx(ctx context.Context) (*tlsca.Identity, error) {
	identity, ok := ctx.Value(identityContextKey).(*tlsca.Identity)
	if !ok {
		return nil, trace.BadParameter("identity is not the expected type, got %T", identity)
	}
	return identity, nil
}

// getUsernameFromCtx will return the username from the given context.
func getUsernameFromCtx(ctx context.Context) (string, error) {
	identity, err := getIdentityFromCtx(ctx)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return identity.Username, nil
}

// ctxWithSPEntityID will return the context with the service provider entity id.
func ctxWithSPEntityID(ctx context.Context, entityID string) context.Context {
	return context.WithValue(ctx, spEntityIDContextKey, entityID)
}

// getSPEntityIDFromCtx will return the service provider entity ID from the given context.
func getSPEntityIDFromCtx(ctx context.Context) (string, error) {
	entityID, ok := ctx.Value(spEntityIDContextKey).(string)
	if !ok {
		return "", trace.NotFound("entityID is not the expected type, got %T", entityID)
	}
	return entityID, nil
}
