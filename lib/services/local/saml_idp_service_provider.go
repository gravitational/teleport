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

package local

import (
	"context"
	"time"

	"github.com/crewjam/saml/samlsp"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
)

const (
	samlIDPServiceProviderPrefix        = "saml_idp_service_provider"
	samlIDPServiceProviderModifyLock    = "saml_idp_service_provider_modify_lock"
	samlIDPServiceProviderModifyLockTTL = time.Second * 5
	samlIDPServiceProviderMaxPageSize   = 200
)

// SAMLIdPServiceProviderService manages IdP service providers in the Backend.
type SAMLIdPServiceProviderService struct {
	genericResourceService[types.SAMLIdPServiceProvider]
}

// NewSAMLIdPServiceProviderService creates a new SAMLIdPServiceProviderService.
func NewSAMLIdPServiceProviderService(backend backend.Backend) *SAMLIdPServiceProviderService {
	s := &SAMLIdPServiceProviderService{
		genericResourceService: genericResourceService[types.SAMLIdPServiceProvider]{
			backend:       backend,
			limit:         samlIDPServiceProviderMaxPageSize,
			resourceKind:  types.KindSAMLIdPServiceProvider,
			backendPrefix: samlIDPServiceProviderPrefix,
			marshalFunc:   services.MarshalSAMLIdPServiceProvider,
			unmarshalFunc: services.UnmarshalSAMLIdPServiceProvider,
		},
	}

	s.modificationPostCheckValidator = s.ensureEntityDescriptorMatchesEntityID
	s.modificationLockName = samlIDPServiceProviderModifyLock
	s.modificationLockTTL = samlIDPServiceProviderModifyLockTTL
	s.preModifyValidator = s.ensureEntityIDIsUnique

	return s
}

// ListSAMLIdPServiceProviders returns a paginated list of SAML IdP service provider resources.
func (s *SAMLIdPServiceProviderService) ListSAMLIdPServiceProviders(ctx context.Context, pageSize int, pageToken string) ([]types.SAMLIdPServiceProvider, string, error) {
	return s.listResources(ctx, pageSize, pageToken)
}

// GetSAMLIdPServiceProvider returns the specified SAML IdP service provider resource.
func (s *SAMLIdPServiceProviderService) GetSAMLIdPServiceProvider(ctx context.Context, name string) (types.SAMLIdPServiceProvider, error) {
	return s.getResource(ctx, name)
}

// CreateSAMLIdPServiceProvider creates a new SAML IdP service provider resource.
func (s *SAMLIdPServiceProviderService) CreateSAMLIdPServiceProvider(ctx context.Context, sp types.SAMLIdPServiceProvider) error {
	return s.createResource(ctx, sp, sp.GetName())
}

// UpdateSAMLIdPServiceProvider updates an existing SAML IdP service provider resource.
func (s *SAMLIdPServiceProviderService) UpdateSAMLIdPServiceProvider(ctx context.Context, sp types.SAMLIdPServiceProvider) error {
	return s.updateResource(ctx, sp, sp.GetName())
}

// DeleteSAMLIdPServiceProvider removes the specified SAML IdP service provider resource.
func (s *SAMLIdPServiceProviderService) DeleteSAMLIdPServiceProvider(ctx context.Context, name string) error {
	return s.deleteResource(ctx, name)
}

// DeleteAllSAMLIdPServiceProviders removes all SAML IdP service provider resources.
func (s *SAMLIdPServiceProviderService) DeleteAllSAMLIdPServiceProviders(ctx context.Context) error {
	return s.deleteAllResources(ctx)
}

// ensureEntityIDIsUnique makes sure that the entity ID in the service provider doesn't already exist in the backend.
func (s *SAMLIdPServiceProviderService) ensureEntityIDIsUnique(ctx context.Context, sp types.SAMLIdPServiceProvider, _ string) error {
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

// ensureEntityDescriptorMatchesEntityID ensures that the entity ID in the entity descriptor is the same as the entity ID
// in the SAMLIdPServiceProvider object.
func (s *SAMLIdPServiceProviderService) ensureEntityDescriptorMatchesEntityID(sp types.SAMLIdPServiceProvider, _ string) error {
	ed, err := samlsp.ParseMetadata([]byte(sp.GetEntityDescriptor()))
	if err != nil {
		return trace.Wrap(err)
	}

	if ed.EntityID != sp.GetEntityID() {
		return trace.BadParameter("entity ID parsed from the entity descriptor does not match the entity ID in the SAML IdP service provider object")
	}

	return nil
}
