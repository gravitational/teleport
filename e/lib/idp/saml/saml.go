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
	"net/url"
	"sync"

	"github.com/crewjam/saml"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	dsig "github.com/russellhaering/goxmldsig"
	"github.com/sirupsen/logrus"

	samlidppb "github.com/gravitational/teleport/api/gen/proto/go/teleport/samlidp/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
)

const (
	// ComponentSAMLIdP is the component to be used in logs.
	ComponentSAMLIdP = "saml_idp" //nolint:revive // Because we want this to be IdP.
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
		c.Log = logrus.WithField(trace.Component, ComponentSAMLIdP)
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
	idpMutex    sync.RWMutex
	idp         saml.IdentityProvider
	authorizer  authz.Authorizer
	idpHandler  http.Handler
	client      IdPAuthClient
	accessPoint IdPAccessPoint
	emitter     apievents.Emitter
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

	// Use SAML IdP CA cert for the IdP.
	ca, err := cfg.Client.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.SAMLIDPCA,
		DomainName: domainName,
	}, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tlsKeys := ca.GetTrustedTLSKeyPairs()
	if len(tlsKeys) == 0 {
		return nil, trace.BadParameter("no trusted TLS key pairs found")
	}

	cert, err := tlsca.ParseCertificatePEM(tlsKeys[0].Cert)
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
		log:         cfg.Log,
		clock:       cfg.Clock,
		authorizer:  cfg.Authorizer,
		client:      cfg.Client,
		accessPoint: cfg.AccessPoint,
		emitter:     cfg.Emitter,
	}

	service.idp = saml.IdentityProvider{
		Logger:                  cfg.Log,
		Certificate:             cert,
		SignatureMethod:         dsig.RSASHA256SignatureMethod,
		SessionProvider:         service,
		ServiceProviderProvider: service,
		AssertionMaker:          service,
		MetadataURL:             *parsedURL.JoinPath("metadata"),
		SSOURL:                  *parsedURL.JoinPath("sso"),
	}

	service.idpHandler, err = service.initRouter()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return service, nil
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
