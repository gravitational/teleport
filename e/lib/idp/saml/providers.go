package saml

import (
	"net/http"

	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

func (s *Service) GetSession(w http.ResponseWriter, r *http.Request, req *saml.IdpAuthnRequest) *saml.Session {
	session, err := s.getSession(w, r)
	if err != nil {
		s.log.WithError(err).Error("Failed to get session.")
		s.writeError(w, trace.ErrorToCode(err))
	}

	// Getting metadata for the audit event.
	user, userErr := getUsernameFromCtx(r.Context())
	if userErr != nil {
		s.log.Warnf("error getting username from context: %v", userErr)
	}

	var sessionID string
	if session != nil {
		sessionID = session.ID
	}

	var entityID string
	if req != nil && req.ServiceProviderMetadata != nil {
		entityID = req.ServiceProviderMetadata.EntityID
	}

	// If the entity ID is still empty, try to retrieve it from the context.
	// This will happen during an IdP initiated SSO flow.
	if entityID == "" {
		var ctxErr error
		entityID, ctxErr = getSPEntityIDFromCtx(r.Context())
		if ctxErr != nil {
			s.log.Debugf("error getting service provider entity ID from the context, continuing: %v", err)
		}
	}

	s.emitAuthAttemptEvent(r.Context(), user, sessionID, entityID, "", err)

	return session
}

// GetSession is an implementation of crewjam's ServiceProviderProvider which injects Teleport native
// properties into the resulting session. This has been largely adapted from crewjam/saml's implementation.
func (s *Service) getSession(w http.ResponseWriter, r *http.Request) (*saml.Session, error) {
	identity, err := getIdentityFromCtx(r.Context())
	if err != nil {
		s.log.Debugf("error getting identity from context: %v", err)
		return nil, trace.AccessDenied("access denied")
	}

	existingSession, err := s.getExistingSession(r, identity)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if existingSession != nil {
		return existingSession, nil
	}

	session, err := s.createSession(r, identity)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// MaxAge is in seconds, so we'll calculate the delta between now and the expire time.
	maxAge := identity.Expires.Sub(s.clock.Now())
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    session.ID,
		MaxAge:   int(maxAge.Seconds()),
		HttpOnly: true,
		Secure:   true,
		Path:     "/",
	})
	return session, nil
}

// GetServiceProvider will return service providers from the API.
func (s *Service) GetServiceProvider(r *http.Request, serviceProviderID string) (*saml.EntityDescriptor, error) {
	var nextToken string

	for {
		var sps []types.SAMLIdPServiceProvider
		var err error
		sps, nextToken, err = s.accessPoint.ListSAMLIdPServiceProviders(r.Context(), 0, nextToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// Search for the service provider with a matching entity ID.
		for _, sp := range sps {
			if sp.GetEntityID() == serviceProviderID {
				if err := validateAssertionConsumerServices(sp); err != nil {
					return nil, trace.Wrap(err)
				}

				ed, err := samlsp.ParseMetadata([]byte(sp.GetEntityDescriptor()))
				if err != nil {
					return nil, trace.Wrap(err)
				}

				return ed, nil
			}
		}

		if nextToken == "" {
			break
		}
	}

	// Getting metadata for the audit event.
	user, err := getUsernameFromCtx(r.Context())
	if err != nil {
		s.log.Warnf("error getting username from context: %v", err)
	}

	err = trace.NotFound("could not find service provider")
	s.emitAuthAttemptEvent(r.Context(), user, "", serviceProviderID, "", err)

	return nil, err
}
