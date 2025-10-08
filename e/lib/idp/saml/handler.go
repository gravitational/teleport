package saml

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"strings"

	"github.com/crewjam/saml"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	dtauthz "github.com/gravitational/teleport/lib/devicetrust/authz"
	"github.com/gravitational/teleport/lib/services"
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

// errUnauthorized is returned for errors related to user session.
// RBAC related errors should directly use trace.AccessDenied.
var errUnauthorized = &trace.AccessDeniedError{Message: "unauthorized"}

// ServeHTTP serves the IdP endpoints.
func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	handler := s.handler()

	if handler == nil {
		s.writeError(w, trace.NotFound("idp not enabled"))
		return
	}

	r.URL.Path = strings.TrimPrefix(r.URL.Path, IdPRoute)
	handler.ServeHTTP(w, r)
}

// initRouter initializes the HTTP router for the SAML IdP endpoints.
func (s *Service) initRouter() (*httprouter.Router, error) {
	router := httprouter.New()

	router.GET("/metadata", s.withHighLimiter(s.handleMetadata))
	router.GET("/metadata-values", s.withHighLimiter(s.handleMetadataValues))
	router.GET("/sso", s.withAuthCtx(s.handleSSO))
	router.POST("/sso", s.withAuthCtx(s.handleSSO))

	// Handle logins.
	router.GET("/login/:shortcut", s.withAuthCtx(s.handleIdPInitiatedLogin))
	router.GET("/login/:shortcut/*urlsuffix", s.withAuthCtx(s.handleIdPInitiatedLogin))
	router.POST("/login/:shortcut", s.withAuthCtx(s.handleIdPInitiatedLogin))
	router.POST("/login/:shortcut/*urlsuffix", s.withAuthCtx(s.handleIdPInitiatedLogin))

	return router, nil
}

// withHighLimiter limits request based on WithHighLimiter from lib/web.
func (h *Service) withHighLimiter(fn httprouter.Handle) httprouter.Handle {
	return h.highLimiter(func(w http.ResponseWriter, r *http.Request, p httprouter.Params) (any, error) {
		fn(w, r, p)
		return nil, nil
	})
}

// withAuthCtx checks for a valid user session.
func (s *Service) withAuthCtx(fn httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		identity, err := s.validateSession(r)
		if err != nil {
			if !trace.IsAccessDenied(err) { // access denied are expected
				s.logger.ErrorContext(r.Context(), "Error authorizing user for SAML IdP", "error", err)
			}

			var username string
			if identity != nil {
				username = identity.Username
			}
			s.emitAuthAttemptEvent(r.Context(), username, "", "", err)
			s.writeError(w, trace.NewAggregate(err, errUnauthorized))
			return
		}
		fn(w, r.WithContext(ctxWithIdentity(r.Context(), identity)), p)
	}
}

func (s *Service) validateSession(r *http.Request) (*tlsca.Identity, error) {
	authCtx, err := s.authorizer.Authorize(r.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	identity, err := s.identityFromAuthCtx(authCtx)
	if err != nil {
		return identity, trace.Wrap(err)
	}

	return identity, nil
}

func (s *Service) identityFromAuthCtx(authCtx *authz.Context) (*tlsca.Identity, error) {
	var identity tlsca.Identity
	// Only allow local users to use the SAML IdP.
	switch user := authCtx.Identity.(type) {
	case authz.LocalUser:
		identity = user.GetIdentity()
	default:
		identity = user.GetIdentity()
		return &identity, trace.BadParameter("unsupported user type: %T", user)
	}

	if s.clock.Now().After(identity.Expires) {
		return &identity, trace.AccessDenied("identity is expired")
	}

	return &identity, nil
}

// authorize applies RBAC to service provider resource.
// A valid user session is expected before calling this method.
// Returns user identity in any case in order to provide username
// for audit logging at the call site.
func (s *Service) authorize(r *http.Request, sp types.SAMLIdPServiceProvider) (*tlsca.Identity, error) {
	authCtx, err := s.authorizer.Authorize(r.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	identity, err := s.identityFromAuthCtx(authCtx)
	if err != nil {
		return nil, trace.NewAggregate(err, errUnauthorized)
	}

	authPref, err := s.accessPoint.GetAuthPreference(r.Context())
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	accessState := authCtx.Checker.GetAccessState(authPref)
	if r.URL.Query().Get(Webauthn.String()) != "" || r.URL.Query().Get(MFAResponse.String()) != "" {
		// For now, authorize the user on the assumption that the provided MFA
		// Response is valid. It will be passed to the Auth Server for verification
		// before the final assertion is signed.
		accessState.MFAVerified = true
	}
	accessState.EnableDeviceVerification = true
	accessState.DeviceVerified = dtauthz.IsTLSDeviceVerified(&identity.DeviceExtensions)

	if err := authCtx.CheckAccessToKind(
		types.KindSAMLIdPServiceProvider,
		types.VerbRead,
		types.VerbList,
	); err != nil {
		return nil, trace.Wrap(err)
	}
	// CheckAccessToSAMLIdP checks for session MFA and a role option
	// that enables access to IdP (legacy SAML IdP RBAC).
	if err := authCtx.Checker.CheckAccessToSAMLIdP(sp, authPref, accessState); err != nil {
		return nil, trace.Wrap(err)
	}

	return identity, nil
}

// handleMetadata handles metadata requests. The response is handled by IdP, which serves
// metadata file.
func (s *Service) handleMetadata(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	idp, err := s.createIdP(r.Context())
	if err != nil {
		s.logger.ErrorContext(r.Context(), "Error creating IdP", "error", err)
		s.writeError(w, err)
	}
	idp.ServeMetadata(w, r) // The saml.IdentityProvider does the response handling here.
}

type idpMetadataValues struct {
	EntityID string `json:"entityID"`
	SSOURL   string `json:"ssoURL"`
	X509PEM  string `json:"x509PEM"`
}

// handleMetadataValues returns IdP entity ID, SSO URL and PEM encoded certificate values.
// While handleMetadata serves a whole metadata file, handleMetadataValues is used to display metadata
// values in the Teleport Web UI.
func (s *Service) handleMetadataValues(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	idp, err := s.createIdP(r.Context())
	if err != nil {
		s.logger.ErrorContext(r.Context(), "Error creating IdP", "error", err)
		s.writeError(w, err)
	}
	ed := idp.Metadata()

	var metadata idpMetadataValues
	metadata.EntityID = ed.EntityID
	for _, desc := range ed.IDPSSODescriptors {
		for _, ssoService := range desc.SingleSignOnServices {
			if ssoService.Binding == "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" {
				metadata.SSOURL = ssoService.Location
			}

		}
	}

	for _, desc := range ed.IDPSSODescriptors {
		for _, keys := range desc.KeyDescriptors {
			if keys.Use == "signing" {
				b64EncodedCert := keys.KeyInfo.X509Data.X509Certificates[0].Data
				rawCert, err := base64.StdEncoding.DecodeString(b64EncodedCert)
				if err != nil {
					s.logger.ErrorContext(r.Context(), "Error decoding IdP certificate", "error", err)
					s.writeError(w, err)
				}
				certPEM := pem.EncodeToMemory(&pem.Block{
					Type:  "CERTIFICATE",
					Bytes: rawCert,
				})
				metadata.X509PEM = string(certPEM)
			}
		}
	}

	resp, err := json.Marshal(metadata)
	if err != nil {
		s.writeError(w, err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(resp)
}

// handleSSO handles SSO requests.
func (s *Service) handleSSO(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	idp, err := s.createIdP(r.Context())
	if err != nil {
		s.logger.ErrorContext(r.Context(), "Error creating IdP", "error", err)
		s.writeError(w, err)
	}
	idp.ServeSSO(w, r) // The saml.IdentityProvider does the response handling here.
}

// handleIdPInitiatedLogin will handle IdP initiated logins for a service provider.
//
//nolint:revive // Because we want this to be IdP.
func (s *Service) handleIdPInitiatedLogin(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	user, err := getUsernameFromCtx(r.Context())
	if err != nil {
		s.logger.WarnContext(r.Context(), "Error getting username from context", "error", err)
	}
	shortcutName := p.ByName("shortcut")

	// It looks like this case isn't possible because the httprouter won't let this resolve
	// without the shortcut, but we'll check here just to be sure.
	if shortcutName == "" {
		err := trace.NotFound("shortcut (SAML app name) is empty")
		s.emitAuthAttemptEvent(r.Context(), user, "", "", err)
		s.writeError(w, err)
		return
	}

	sp, err := s.accessPoint.GetSAMLIdPServiceProvider(r.Context(), shortcutName)
	if err != nil {
		s.emitAuthAttemptEvent(r.Context(), user, "", shortcutName, err)
		s.writeError(w, trace.NotFound("service provider not found"))
		return
	}

	if err := validateAssertionConsumerServices(sp); err != nil {
		s.emitAuthAttemptEvent(r.Context(), user, "", shortcutName, err)
		s.writeError(w, err)
		return
	}

	// embed the entity ID in the context because IdP initiated SSO doesn't put the
	// entity ID into the authnRequest until after the session is retrieved, which means
	// audit event emitted in the session provider doesn't have access to the entity ID.
	r = r.WithContext(ctxWithSPEntityID(r.Context(), sp.GetEntityID()))

	// The saml.IdentityProvider does the response handling here.
	idp, err := s.createIdP(r.Context())
	if err != nil {
		s.logger.ErrorContext(r.Context(), "Error creating IdP", "error", err)
		s.writeError(w, err)
	}
	idp.ServeIDPInitiated(w, r, sp.GetEntityID(), sp.GetRelayState())
}

// Write writes SAML authentication response with security headers and HTML POST form.
func (s *Service) Write(w http.ResponseWriter, authnRequest *saml.IdpAuthnRequest) error {
	authnForm, err := authnRequest.PostBinding()
	if err != nil {
		return trace.Wrap(err)
	}
	if err := WriteSAMLPOSTFormWithHeaders(w, POSTFormData{
		URL:                  authnForm.URL,
		SAMLAuthnMessageType: SAMLResponse,
		SAMLAuthnMessage:     authnForm.SAMLResponse,
		RelayState:           authnForm.RelayState,
	}); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// handler returns the HTTP handler for the identity provider.
func (s *Service) handler() http.Handler {
	return s.idpHandler
}

// writeError writes an error response.
func (s *Service) writeError(w http.ResponseWriter, err error) {
	var code int
	var msg string

	switch {
	case errors.Is(err, errUnauthorized):
		code = http.StatusUnauthorized
		msg = http.StatusText(http.StatusUnauthorized)
	case errors.Is(err, services.ErrTrustedDeviceRequired):
		code = http.StatusForbidden
		msg = "Access to this resource requires a Trusted Device."
	case trace.IsNotFound(err) || trace.IsAccessDenied(err):
		// StatusForbidden is used to prevent SAML application enumeration
		// based on not-found or access denied errors.
		code = http.StatusForbidden
		msg = "You do not have access to this resource."
	default:
		code = trace.ErrorToCode(err)
		msg = http.StatusText(code)
	}

	msg = msg + " More details related to this event can be found in the Teleport audit log."

	http.Error(w, msg, code)
}

// ctxWithIdentity will return the context with the given identity.
func ctxWithIdentity(ctx context.Context, identity *tlsca.Identity) context.Context {
	return context.WithValue(ctx, identityContextKey, identity)
}

// getIdentityFromCtx will return the identity from the given context.
func getIdentityFromCtx(ctx context.Context) (*tlsca.Identity, error) {
	identityContextValue := ctx.Value(identityContextKey)
	if identityContextValue == nil {
		return nil, trace.AccessDenied("access denied")
	}

	identity, ok := identityContextValue.(*tlsca.Identity)
	if !ok {
		return nil, trace.BadParameter("identity is not the expected type, got %T", identityContextValue)
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
