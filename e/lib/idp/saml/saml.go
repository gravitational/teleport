package saml

import (
	"context"
	"net/http"
	"net/url"

	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	dsig "github.com/russellhaering/goxmldsig"
	"github.com/sirupsen/logrus"

	samlidppb "github.com/gravitational/teleport/api/gen/proto/go/teleport/samlidp/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
)

type Config struct {
	// Log is the logger
	Log *logrus.Entry
	// Clock is a clock for time-related operations
	Clock clockwork.Clock
	// Client is the non-cached client to use for the SAML IdP
	Client IdPAuthClient
	// AccessPoint is the cached client to use for the SAML IdP
	AccessPoint IdPAccessPoint
	// Authorizer is the authorizer to use for the SAML IdP
	Authorizer authz.Authorizer
	// BaseURL is the base URL for the SAML IdP
	BaseURL string
	// Emitter emits audit events.
	Emitter apievents.Emitter
}

// Check makes sure the SAML identity provider service configuration is valid.
func (c *Config) Check() error {
	if c.Log == nil {
		c.Log = logrus.WithField(trace.Component, teleport.ComponentSAMLIdP)
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	if c.Client == nil {
		return trace.BadParameter("client is missing")
	}
	if c.AccessPoint == nil {
		return trace.BadParameter("access point is missing")
	}
	if c.Authorizer == nil {
		return trace.BadParameter("authorizer is missing")
	}
	if c.BaseURL == "" {
		return trace.BadParameter("base URL is missing")
	}
	if c.Emitter == nil {
		return trace.BadParameter("emitter is missing")
	}
	return nil
}

// IdPAuthClient is a subset of auth.Client for the SAML IdP to use.
//
//nolint:revive // Because we want this to be IdP.
type IdPAuthClient interface {
	services.AuthorityGetter
	services.SAMLIdPServiceProviders
	services.SAMLIdPSession
	services.RoleGetter

	// SAMLIdPClient is the client for the SAML IdP specific utility functions.
	SAMLIdPClient() samlidppb.SAMLIdPServiceClient

	// CreateSAMLIdPSession creates a SAML IdP. SAML IdP sessions represent
	// sessions created by the SAML identity provider.
	CreateSAMLIdPSession(context.Context, types.CreateSAMLIdPSessionRequest) (types.WebSession, error)

	// GetDomainName returns auth server cluster name
	GetDomainName(ctx context.Context) (string, error)
}

// IdPAccessPoint is a caching access point for the SAML IdP to use.
//
//nolint:revive // Because we want this to be IdP.
type IdPAccessPoint interface {
	services.RoleGetter

	// GetAuthPreference gets types.AuthPreference from the backend.
	GetAuthPreference(context.Context) (types.AuthPreference, error)

	// GetSAMLIdPServiceProvider returns the specified SAML IdP service provider resources.
	GetSAMLIdPServiceProvider(ctx context.Context, name string) (types.SAMLIdPServiceProvider, error)

	// ListSAMLIdPServiceProviders returns a paginated list of all SAML IdP service provider resources.
	ListSAMLIdPServiceProviders(context.Context, int, string) ([]types.SAMLIdPServiceProvider, string, error)

	// GetSAMLIdPSession gets a SAML IdP session.
	GetSAMLIdPSession(context.Context, types.GetSAMLIdPSessionRequest) (types.WebSession, error)
}

// Service is the SAML identity provider service.
type Service struct {
	log         *logrus.Entry
	clock       clockwork.Clock
	authorizer  authz.Authorizer
	idpHandler  http.Handler
	client      IdPAuthClient
	accessPoint IdPAccessPoint
	emitter     apievents.Emitter

	signatureMethod string
	domainName      string
	metadataURL     url.URL
	ssoURL          url.URL
}

// New creates a new SAML identity provider service.
func New(ctx context.Context, cfg Config) (*Service, error) {
	if err := cfg.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	domainName, err := cfg.Client.GetDomainName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	parsedURL, err := url.Parse(cfg.BaseURL)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// If the port is pointing to 443, strip the port. This will make
	// the resulting entity ID a little cleaner as it won't always have
	// the port present.
	if parsedURL.Scheme == "https" && parsedURL.Port() == "443" {
		parsedURL.Host = parsedURL.Hostname()
	}

	// The base path should have the SAMLIdPRoute as its beginning path.
	parsedURL.Path = IdPRoute

	service := &Service{
		log:             cfg.Log,
		clock:           cfg.Clock,
		authorizer:      cfg.Authorizer,
		client:          cfg.Client,
		accessPoint:     cfg.AccessPoint,
		emitter:         cfg.Emitter,
		domainName:      domainName,
		signatureMethod: dsig.RSASHA256SignatureMethod,
		metadataURL:     *parsedURL.JoinPath("metadata"),
		ssoURL:          *parsedURL.JoinPath("sso"),
	}

	service.idpHandler, err = service.initRouter()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return service, nil
}

// createIdP creates a saml.IdentityProvider with the most recent certificate baked into it
// this will ensure that IdP always has the most recent SAMLIDPCA certificate. This is done
// to account for certificate rotation.
//
//nolint:revive // Because we want this to be IdP.
func (s *Service) createIdP(ctx context.Context) (saml.IdentityProvider, error) {
	// Use SAML IdP CA cert for the IdP.
	ca, err := s.client.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.SAMLIDPCA,
		DomainName: s.domainName,
	}, false)
	if err != nil {
		return saml.IdentityProvider{}, trace.Wrap(err)
	}

	// TODO: list all of the signing keys when/if crewjam/saml supports it
	tlsKeys := ca.GetTrustedTLSKeyPairs()
	if len(tlsKeys) == 0 {
		return saml.IdentityProvider{}, trace.BadParameter("no trusted TLS key pairs found")
	}

	cert, err := tlsca.ParseCertificatePEM(tlsKeys[0].Cert)
	if err != nil {
		return saml.IdentityProvider{}, trace.Wrap(err)
	}

	return saml.IdentityProvider{
		Logger:                  s.log,
		Certificate:             cert,
		SignatureMethod:         s.signatureMethod,
		SessionProvider:         s,
		ServiceProviderProvider: s,
		AssertionMaker:          s,
		MetadataURL:             s.metadataURL,
		SSOURL:                  s.ssoURL,
	}, nil
}

// emitAuthAttemptEvent will emit an auth attempt event to the audit log.
func (s *Service) emitAuthAttemptEvent(ctx context.Context, user, sessionID, entityID, shortcut string, sourceErr error) {
	success := true
	var errorMsg string
	if sourceErr != nil {
		success = false
		errorMsg = sourceErr.Error()
	}

	event := &apievents.SAMLIdPAuthAttempt{
		Metadata: apievents.Metadata{
			Type: events.SAMLIdPAuthAttemptEvent,
			Code: events.SAMLIdPAuthAttemptCode,
		},
		UserMetadata: apievents.UserMetadata{
			User: user,
		},
		SessionMetadata: apievents.SessionMetadata{
			SessionID: sessionID,
		},
		Status: apievents.Status{
			Success: success,
			Error:   errorMsg,
		},
		SAMLIdPServiceProviderMetadata: apievents.SAMLIdPServiceProviderMetadata{
			ServiceProviderEntityID: entityID,
			ServiceProviderShortcut: shortcut,
		},
	}

	if emitErr := s.emitter.EmitAuditEvent(ctx, event); emitErr != nil {
		s.log.WithError(emitErr).Warnf("Failed to emit SAML IdP auth attempt event: %v", event)
	}
}

// validateAssertionConsumerServices ensures that all AssertionConsumerServices defined in the
// [saml.EntityDescriptor] are valid HTTPS endpoints.
func validateAssertionConsumerServices(sp types.SAMLIdPServiceProvider) error {
	ed, err := samlsp.ParseMetadata([]byte(sp.GetEntityDescriptor()))
	if err != nil {
		return trace.Wrap(err)
	}

	for _, descriptor := range ed.SPSSODescriptors {
		for _, acs := range descriptor.AssertionConsumerServices {
			endpoint, err := url.Parse(acs.Location)
			switch {
			case err != nil:
				return trace.Wrap(err)
			case endpoint.Scheme != "https":
				return trace.BadParameter("the assertion consumer services location must be an http or https endpoint")
			}
		}
	}

	return nil
}
