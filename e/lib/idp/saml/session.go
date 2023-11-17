package saml

import (
	"net/http"

	"github.com/crewjam/saml"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// sessionCookieName is the name of the session cookie set by and read by the IdP.
	sessionCookieName = "__Host-saml_session"
)

// getExistingSession will get an existing session if possible.
func (s *Service) getExistingSession(r *http.Request, identity *tlsca.Identity) (*saml.Session, error) {
	// Look for the session ID from the session cookie.
	sessionCookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return nil, trace.NotFound("session cookie not found")
	}
	webSession, err := s.accessPoint.GetSAMLIdPSession(r.Context(), types.GetSAMLIdPSessionRequest{
		SessionID: sessionCookie.Value,
	})
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.AccessDenied("user %s attempted to access a non-existent SAML IdP session", identity.Username)
		}
		return nil, trace.Wrap(err)
	}

	// Make sure this session belongs to this user.
	if webSession.GetUser() != identity.Username {
		return nil, trace.AccessDenied("user %s attempted to access a SAML IdP session that belonged to %s", identity.Username, webSession.GetUser())
	}

	if s.clock.Now().Before(webSession.Expiry()) {
		return webSessionToSAMLSession(webSession)
	}

	return nil, trace.NotFound("session not found")
}

// createSession will create a new SAML session.
func (s *Service) createSession(r *http.Request, identity *tlsca.Identity) (*saml.Session, error) {
	// Create random strings for the ID and the index. This code is adapted from the
	// example crewjam/samlidp.
	idHex, err := utils.CryptoRandomHex(32)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	indexHex, err := utils.CryptoRandomHex(32)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	session := &saml.Session{
		ID:         idHex,
		NameID:     identity.Username,
		CreateTime: s.clock.Now(),
		ExpireTime: identity.Expires,
		Index:      indexHex,
		UserName:   identity.Username,
		Groups:     identity.Groups,
	}
	webSession, err := samlSessionToWebSession(session)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if _, err := s.client.CreateSAMLIdPSession(r.Context(), types.CreateSAMLIdPSessionRequest{
		SessionID:   session.ID,
		Username:    identity.Username,
		SAMLSession: webSession.GetSAMLSession(),
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	return session, nil
}

// webSessionToSAMLSession converts a WebSession into a *saml.Session object.
func webSessionToSAMLSession(internalSession types.WebSession) (*saml.Session, error) {
	if internalSession.GetSAMLSession() == nil {
		return nil, trace.BadParameter("the internal session has no SAML session data")
	}

	data := internalSession.GetSAMLSession()

	session := &saml.Session{
		ID:         data.ID,
		CreateTime: data.CreateTime,
		ExpireTime: data.ExpireTime,
		Index:      data.Index,

		NameID:       data.NameID,
		NameIDFormat: data.NameIDFormat,
		SubjectID:    data.SubjectID,

		UserName:              data.UserName,
		UserEmail:             data.UserEmail,
		UserCommonName:        data.UserCommonName,
		UserSurname:           data.UserSurname,
		UserGivenName:         data.UserGivenName,
		UserScopedAffiliation: data.UserScopedAffiliation,
	}

	session.Groups = append(session.Groups, data.Groups...)

	for _, internalAttribute := range data.CustomAttributes {
		var values []saml.AttributeValue
		for _, internalValue := range internalAttribute.Values {
			var nameID *saml.NameID
			if internalValue.NameID != nil {
				nameID = &saml.NameID{
					NameQualifier:   internalValue.NameID.NameQualifier,
					SPNameQualifier: internalValue.NameID.SPNameQualifier,
					Format:          internalValue.NameID.Format,
					SPProvidedID:    internalValue.NameID.SPProvidedID,
					Value:           internalValue.NameID.Value,
				}
			}
			value := saml.AttributeValue{
				Type:   internalValue.Type,
				Value:  internalValue.Value,
				NameID: nameID,
			}

			values = append(values, value)
		}
		attribute := saml.Attribute{
			FriendlyName: internalAttribute.FriendlyName,
			Name:         internalAttribute.Name,
			NameFormat:   internalAttribute.NameFormat,
			Values:       values,
		}

		session.CustomAttributes = append(session.CustomAttributes, attribute)
	}

	return session, nil
}

// samlSessionToWebSession converts a *saml.Session object into a WebSession.
func samlSessionToWebSession(session *saml.Session) (types.WebSession, error) {
	if session == nil {
		return nil, trace.BadParameter("the session is nil")
	}

	data := &types.SAMLSessionData{
		ID:         session.ID,
		CreateTime: session.CreateTime,
		ExpireTime: session.ExpireTime,
		Index:      session.Index,

		NameID:       session.NameID,
		NameIDFormat: session.NameIDFormat,
		SubjectID:    session.SubjectID,

		UserName:              session.UserName,
		UserEmail:             session.UserEmail,
		UserCommonName:        session.UserCommonName,
		UserSurname:           session.UserSurname,
		UserGivenName:         session.UserGivenName,
		UserScopedAffiliation: session.UserScopedAffiliation,
	}

	data.Groups = append(data.Groups, session.Groups...)

	for _, attribute := range session.CustomAttributes {
		var values []*types.SAMLAttributeValue
		for _, value := range attribute.Values {
			var nameID *types.SAMLNameID
			if value.NameID != nil {
				nameID = &types.SAMLNameID{
					NameQualifier:   value.NameID.NameQualifier,
					SPNameQualifier: value.NameID.SPNameQualifier,
					Format:          value.NameID.Format,
					SPProvidedID:    value.NameID.SPProvidedID,
					Value:           value.NameID.Value,
				}
			}
			value := &types.SAMLAttributeValue{
				Type:   value.Type,
				Value:  value.Value,
				NameID: nameID,
			}

			values = append(values, value)
		}
		internalAttribute := types.SAMLAttribute{
			FriendlyName: attribute.FriendlyName,
			Name:         attribute.Name,
			NameFormat:   attribute.NameFormat,
			Values:       values,
		}

		data.CustomAttributes = append(data.CustomAttributes, &internalAttribute)
	}

	internalSession, err := types.NewWebSession(
		session.ID, types.KindSAMLIdPSession,
		types.WebSessionSpecV2{
			User:        session.UserName,
			Expires:     session.ExpireTime,
			SAMLSession: data,
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return internalSession, nil
}
