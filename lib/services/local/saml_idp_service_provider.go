/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package local

import (
	"context"
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	samlIDPServiceProviderPrefix        = "saml_idp_service_provider"
	samlIDPServiceProviderModifyLock    = "saml_idp_service_provider_modify_lock"
	samlIDPServiceProviderModifyLockTTL = time.Second * 5
	samlIDPServiceProviderMaxPageSize   = 200
)

// SAMLIdPServiceProviderService manages IdP service providers in the Backend.
type SAMLIdPServiceProviderService struct {
	svc        generic.Service[types.SAMLIdPServiceProvider]
	log        logrus.FieldLogger
	httpClient *http.Client
}

// SAMLIdPOption adds optional arguments to NewSAMLIdPServiceProviderService.
type SAMLIdPOption func(*SAMLIdPServiceProviderService)

// WithHTTPClient configures SAMLIdPServiceProviderService with given http client.
func WithHTTPClient(httpClient *http.Client) SAMLIdPOption {
	return func(s *SAMLIdPServiceProviderService) {
		s.httpClient = httpClient
	}
}

// NewSAMLIdPServiceProviderService creates a new SAMLIdPServiceProviderService.
func NewSAMLIdPServiceProviderService(backend backend.Backend, opts ...SAMLIdPOption) (*SAMLIdPServiceProviderService, error) {
	svc, err := generic.NewService(&generic.ServiceConfig[types.SAMLIdPServiceProvider]{
		Backend:       backend,
		PageLimit:     samlIDPServiceProviderMaxPageSize,
		ResourceKind:  types.KindSAMLIdPServiceProvider,
		BackendPrefix: samlIDPServiceProviderPrefix,
		MarshalFunc:   services.MarshalSAMLIdPServiceProvider,
		UnmarshalFunc: services.UnmarshalSAMLIdPServiceProvider,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	samlSPService := &SAMLIdPServiceProviderService{
		svc: *svc,
		log: logrus.WithFields(logrus.Fields{trace.Component: "saml-idp"}),
	}

	for _, opt := range opts {
		opt(samlSPService)
	}

	if samlSPService.httpClient == nil {
		samlSPService.httpClient = &http.Client{
			Timeout: defaults.HTTPRequestTimeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}

	return samlSPService, nil
}

// ListSAMLIdPServiceProviders returns a paginated list of SAML IdP service provider resources.
func (s *SAMLIdPServiceProviderService) ListSAMLIdPServiceProviders(ctx context.Context, pageSize int, pageToken string) ([]types.SAMLIdPServiceProvider, string, error) {
	return s.svc.ListResources(ctx, pageSize, pageToken)
}

// GetSAMLIdPServiceProvider returns the specified SAML IdP service provider resource.
func (s *SAMLIdPServiceProviderService) GetSAMLIdPServiceProvider(ctx context.Context, name string) (types.SAMLIdPServiceProvider, error) {
	return s.svc.GetResource(ctx, name)
}

// CreateSAMLIdPServiceProvider creates a new SAML IdP service provider resource.
func (s *SAMLIdPServiceProviderService) CreateSAMLIdPServiceProvider(ctx context.Context, sp types.SAMLIdPServiceProvider) error {
	if sp.GetEntityDescriptor() == "" {
		if err := s.fetchAndSetEntityDescriptor(sp); err != nil {
			// We aren't interested in checking error type as any occurrence of error mean entity descriptor was not set.
			// But a debug log should be helpful to indicate source of error.
			s.log.Debugf("Failed to fetch entity descriptor from %s. %s.", sp.GetEntityID(), err.Error())

			if err := s.generateAndSetEntityDescriptor(sp); err != nil {
				return trace.BadParameter("could not generate entity descriptor with given entity_id and acs_url.")
			}
		}
	}

	if err := validateSAMLIdPServiceProvider(sp); err != nil {
		return trace.Wrap(err)
	}

	item, err := s.svc.MakeBackendItem(sp, sp.GetName())
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(s.svc.RunWhileLocked(ctx, samlIDPServiceProviderModifyLock, samlIDPServiceProviderModifyLockTTL,
		func(ctx context.Context, backend backend.Backend) error {
			if err := s.ensureEntityIDIsUnique(ctx, sp); err != nil {
				return trace.Wrap(err)
			}

			_, err := backend.Create(ctx, item)
			if trace.IsAlreadyExists(err) {
				return trace.AlreadyExists("%s %q already exists", types.KindSAMLIdPServiceProvider, sp.GetName())
			}
			return trace.Wrap(err)
		}))
}

// UpdateSAMLIdPServiceProvider updates an existing SAML IdP service provider resource.
func (s *SAMLIdPServiceProviderService) UpdateSAMLIdPServiceProvider(ctx context.Context, sp types.SAMLIdPServiceProvider) error {
	if err := validateSAMLIdPServiceProvider(sp); err != nil {
		return trace.Wrap(err)
	}

	item, err := s.svc.MakeBackendItem(sp, sp.GetName())
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(s.svc.RunWhileLocked(ctx, samlIDPServiceProviderModifyLock, samlIDPServiceProviderModifyLockTTL,
		func(ctx context.Context, backend backend.Backend) error {
			if err := s.ensureEntityIDIsUnique(ctx, sp); err != nil {
				return trace.Wrap(err)
			}

			_, err := backend.Update(ctx, item)
			if trace.IsNotFound(err) {
				return trace.NotFound("%s %q doesn't exist", types.KindSAMLIdPServiceProvider, sp.GetName())
			}

			return trace.Wrap(err)
		}))
}

// DeleteSAMLIdPServiceProvider removes the specified SAML IdP service provider resource.
func (s *SAMLIdPServiceProviderService) DeleteSAMLIdPServiceProvider(ctx context.Context, name string) error {
	return s.svc.DeleteResource(ctx, name)
}

// DeleteAllSAMLIdPServiceProviders removes all SAML IdP service provider resources.
func (s *SAMLIdPServiceProviderService) DeleteAllSAMLIdPServiceProviders(ctx context.Context) error {
	return s.svc.DeleteAllResources(ctx)
}

// ensureEntityIDIsUnique makes sure that the entity ID in the service provider doesn't already exist in the backend.
func (s *SAMLIdPServiceProviderService) ensureEntityIDIsUnique(ctx context.Context, sp types.SAMLIdPServiceProvider) error {
	// Make sure no other service provider has the same entity ID.
	var nextToken string
	for {
		var listSps []types.SAMLIdPServiceProvider
		var err error
		listSps, nextToken, err = s.ListSAMLIdPServiceProviders(ctx, samlIDPServiceProviderMaxPageSize, nextToken)

		if err != nil {
			return trace.Wrap(err)
		}

		for _, listSp := range listSps {
			// Only check entity ID duplicates if we're looking at objects other than the one we're trying to validate.
			// This ensures updates will work and that creates will return an already exists error.
			if listSp.GetName() != sp.GetName() && listSp.GetEntityID() == sp.GetEntityID() {
				return trace.BadParameter("%s %q has the same entity ID %q", types.KindSAMLIdPServiceProvider, listSp.GetName(), listSp.GetEntityID())
			}
		}
		if nextToken == "" {
			break
		}
	}

	return nil
}

// validateSAMLIdPServiceProvider ensures that the entity ID in the entity descriptor is the same as the entity ID
// in the [types.SAMLIdPServiceProvider] and that all AssertionConsumerServices defined are valid HTTPS endpoints.
func validateSAMLIdPServiceProvider(sp types.SAMLIdPServiceProvider) error {
	ed, err := samlsp.ParseMetadata([]byte(sp.GetEntityDescriptor()))
	if err != nil {
		switch {
		case errors.Is(err, io.EOF):
			return trace.BadParameter("missing entity descriptor: %s", err.Error())
		default:
			return trace.BadParameter(err.Error())
		}
	}

	if ed.EntityID != sp.GetEntityID() {
		return trace.BadParameter("entity ID parsed from the entity descriptor does not match the entity ID in the SAML IdP service provider object")
	}

	for _, descriptor := range ed.SPSSODescriptors {
		for _, acs := range descriptor.AssertionConsumerServices {
			if err := services.ValidateAssertionConsumerServicesEndpoint(acs.Location); err != nil {
				return trace.Wrap(err)
			}
		}
	}

	return nil
}

// fetchAndSetEntityDescriptor fetches Service Provider entity descriptor (aka SP metadata)
// from remote metadata endpoint (Entity ID) and sets it to sp if the xml format
// is a valid Service Provider metadata format.
func (s *SAMLIdPServiceProviderService) fetchAndSetEntityDescriptor(sp types.SAMLIdPServiceProvider) error {
	if s.httpClient == nil {
		return trace.BadParameter("missing http client")
	}
	resp, err := s.httpClient.Get(sp.GetEntityID())
	if err != nil {
		return trace.Wrap(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return trace.Wrap(trace.ReadError(resp.StatusCode, nil))
	}

	body, err := utils.ReadAtMost(resp.Body, teleport.MaxHTTPResponseSize)
	if err != nil {
		return trace.Wrap(err)
	}

	// parse body to check if its a valid entity descriptor
	_, err = samlsp.ParseMetadata(body)
	if err != nil {
		return trace.Wrap(err)
	}

	sp.SetEntityDescriptor(string(body))
	return nil
}

// generateAndSetEntityDescriptor generates and sets Service Provider entity descriptor
// with ACS URL, Entity ID and unspecified NameID format.
func (s *SAMLIdPServiceProviderService) generateAndSetEntityDescriptor(sp types.SAMLIdPServiceProvider) error {
	s.log.Infof("Generating a default entity_descriptor with entity_id %s and acs_url %s.", sp.GetEntityID(), sp.GetACSURL())

	acsURL, err := url.Parse(sp.GetACSURL())
	if err != nil {
		return trace.Wrap(err)
	}

	newServiceProvider := saml.ServiceProvider{
		EntityID:          sp.GetEntityID(),
		AcsURL:            *acsURL,
		AuthnNameIDFormat: saml.UnspecifiedNameIDFormat,
	}

	entityDescriptor, err := xml.MarshalIndent(newServiceProvider.Metadata(), "", "  ")
	if err != nil {
		return trace.Wrap(err)
	}

	sp.SetEntityDescriptor(string(entityDescriptor))
	return nil
}
