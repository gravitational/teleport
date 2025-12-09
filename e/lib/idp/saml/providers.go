package saml

import (
	"context"
	"errors"
	"net/http"

	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/idp/saml/attribute"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

const samlIdpLoginPath = "/web/saml-idp/login?redirect_uri="

// GetSession ensures user session and applies RBAC against service provider resource.
func (s *Service) GetSession(w http.ResponseWriter, r *http.Request, req *saml.IdpAuthnRequest) *saml.Session {
	ctx := r.Context()
	// username for the audit event.
	username, err := getUsernameFromCtx(ctx)
	if err != nil {
		// It isn't expected for missing user identity at this stage so we log error here.
		s.logger.WarnContext(ctx, "Error getting username from context", "error", err)
	}

	entityID := s.getSPEntityID(ctx, req)
	sp, err := s.getServiceProvider(ctx, entityID)
	if err != nil {
		s.emitAuthAttemptEvent(ctx, username, entityID, "", err)
		s.writeError(w, err)
		return nil
	}

	identity, err := s.authorize(r, sp)
	if err != nil {
		if errors.Is(err, services.ErrSessionMFARequired) {
			// redirect user to /web/saml-idp/login to provide mfa and try again.
			redirectURL, err := SSORedirectURL(r, IdPRoute+r.URL.Path)
			if err != nil {
				s.emitAuthAttemptEvent(ctx, username, entityID, "", err)
				s.logger.ErrorContext(ctx, "Failed to process redirect URL for MFA", "error", err)
				s.writeError(w, err)
				return nil
			}
			http.Redirect(w, r, samlIdpLoginPath+redirectURL.String(), http.StatusSeeOther)
			return nil
		}
		s.emitAuthAttemptEvent(ctx, username, entityID, "", err)
		s.logger.DebugContext(ctx, "User not authorized", "error", err)
		s.writeError(w, err)
		return nil
	}

	session, err := s.createSession(identity)
	if err != nil {
		s.emitAuthAttemptEvent(ctx, username, entityID, "", err)
		s.logger.ErrorContext(ctx, "Failed to get session", "error", err)
		s.writeError(w, err)
		return nil
	}

	return session
}

func (s *Service) getSPEntityID(ctx context.Context, req *saml.IdpAuthnRequest) string {
	if req != nil && req.ServiceProviderMetadata != nil && req.ServiceProviderMetadata.EntityID != "" {
		return req.ServiceProviderMetadata.EntityID
	}

	// If the entity ID is still empty, try to retrieve it from the context.
	// This will happen during an IdP initiated SSO flow.
	if entityID, err := getSPEntityIDFromCtx(ctx); err == nil {
		return entityID
	}

	s.logger.DebugContext(ctx, "Failed to get service provider entity ID, continuing")
	return ""
}

// createSession will create a new SAML session.
func (s *Service) createSession(identity *tlsca.Identity) (*saml.Session, error) {
	if s.clock.Now().After(identity.Expires) {
		return nil, trace.AccessDenied("access denied")
	}

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
		CustomAttributes: SamlMappableAttributeToCustomAttribute(attribute.SAMLMappableUserSpec{
			Username: identity.Username,
			Traits:   identity.Traits,
			Roles:    identity.Groups,
		}),
	}
	return session, nil
}

// SamlMappableAttributeToCustomAttribute converts samlMappableUserSpec to saml.Attribute
// which will eventually be added to SAML session custom attributes.
func SamlMappableAttributeToCustomAttribute(userSpec attribute.SAMLMappableUserSpec) []saml.Attribute {
	var customAttributes []saml.Attribute
	for k, v := range userSpec.Traits {
		customAttributes = addAttribute(customAttributes, k, k, v...)
	}
	customAttributes = addAttribute(customAttributes, "roles", "roles", userSpec.Roles...)
	customAttributes = addAttribute(customAttributes, "username", "username", userSpec.Username)
	return customAttributes
}

// GetServiceProvider will return matching service provider based on entity ID.
func (s *Service) GetServiceProvider(r *http.Request, serviceProviderID string) (*saml.EntityDescriptor, error) {
	ctx := r.Context()
	// username for the audit event.
	username, err := getUsernameFromCtx(ctx)
	if err != nil {
		// It isn't expected for missing user identity at this stage so we log error here.
		s.logger.WarnContext(ctx, "Error getting username from context", "error", err)
	}

	sp, err := s.getServiceProvider(ctx, serviceProviderID)
	if err != nil {
		s.emitAuthAttemptEvent(ctx, username, serviceProviderID, "", err)
		return nil, trace.Wrap(err)
	}
	ed, err := samlsp.ParseMetadata([]byte(sp.GetEntityDescriptor()))
	if err != nil {
		s.emitAuthAttemptEvent(ctx, username, serviceProviderID, "", err)
		return nil, trace.Wrap(err)
	}

	return ed, err
}

func (s *Service) getServiceProvider(ctx context.Context, entityID string) (types.SAMLIdPServiceProvider, error) {
	var nextToken string
	for {
		var sps []types.SAMLIdPServiceProvider
		var err error
		sps, nextToken, err = s.accessPoint.ListSAMLIdPServiceProviders(ctx, 0, nextToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// Search for the service provider with a matching entity ID.
		for _, sp := range sps {
			if sp.GetEntityID() == entityID {
				if err := validateAssertionConsumerServices(sp); err != nil {
					return nil, trace.Wrap(err)
				}
				return sp, nil
			}
		}

		if nextToken == "" {
			break
		}
	}

	return nil, trace.NotFound("could not find service provider")
}
