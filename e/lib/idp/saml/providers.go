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
	"net/http"

	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

func (s *Service) GetSession(w http.ResponseWriter, r *http.Request, _ *saml.IdpAuthnRequest) *saml.Session {
	session, err := s.getSession(w, r)
	if err != nil {
		s.log.WithError(err).Error("Failed to get session.")
		s.writeError(w, trace.ErrorToCode(err))
		return nil
	}

	return session
}

// GetSession is an implementation of crewjam's ServiceProviderProvider which injects Teleport native
// properties into the resulting session. This has been largely adapted from crewjam/saml's implementation.
func (s *Service) getSession(w http.ResponseWriter, r *http.Request) (*saml.Session, error) {
	identity, err := s.getIdentityFromCtx(r.Context())
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

	return nil, trace.NotFound("could not find service provider %s", serviceProviderID)
}
