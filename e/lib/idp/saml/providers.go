package saml

import (
	"context"
	"net/http"

	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/idp/saml/attribute"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

func (s *Service) GetSession(w http.ResponseWriter, r *http.Request, req *saml.IdpAuthnRequest) *saml.Session {
	sess, err := s.getSession(r.Context(), req)
	if err != nil {
		s.logger.ErrorContext(r.Context(), "Failed to get session", "error", err)
		s.writeError(w, trace.ErrorToCode(err))
	}
	return sess
}

func (s *Service) getSession(ctx context.Context, req *saml.IdpAuthnRequest) (*saml.Session, error) {
	entityID := s.getSPEntityID(ctx, req)

	identity, err := getIdentityFromCtx(ctx)
	if err != nil {
		s.logger.DebugContext(ctx, "error getting identity from context", "error", err)
		s.emitAuthAttemptEvent(ctx, "", entityID, "", err)
		return nil, trace.Wrap(err)
	}

	session, err := s.createSession(identity)
	if err != nil {
		s.emitAuthAttemptEvent(ctx, identity.Username, entityID, "", err)
		return nil, trace.Wrap(err, "failed to create session")
	}

	s.emitAuthAttemptEvent(ctx, identity.Username, entityID, "", err)
	return session, nil
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
	var customAttributes []saml.Attribute = make([]saml.Attribute, 0)
	for k, v := range userSpec.Traits {
		customAttributes = addAttribute(customAttributes, k, k, v...)
	}
	customAttributes = addAttribute(customAttributes, "roles", "roles", userSpec.Roles...)
	customAttributes = addAttribute(customAttributes, "username", "username", userSpec.Username)
	return customAttributes
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
		s.logger.WarnContext(r.Context(), "error getting username from context", "error", err)
	}

	err = trace.NotFound("could not find service provider")
	s.emitAuthAttemptEvent(r.Context(), user, serviceProviderID, "", err)

	return nil, err
}
