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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
)

// SAMLIdPServiceProviderService manages IdP service providers in the Backend
type SAMLIdPServiceProviderService struct {
	backend.Backend
}

// NewSAMLIdPServiceProviderService creates a new SAMLIdPServiceProviderService.
func NewSAMLIdPServiceProviderService(backend backend.Backend) *SAMLIdPServiceProviderService {
	return &SAMLIdPServiceProviderService{Backend: backend}
}

// GetSAMLIdPServiceProviders returns all SAML IdP service provider resources.
func (s *SAMLIdPServiceProviderService) GetSAMLIdPServiceProviders(ctx context.Context) ([]types.SAMLIdPServiceProvider, error) {
	startKey := backend.Key(samlIDPServiceProviderPrefix)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	serviceProviders := make([]types.SAMLIdPServiceProvider, len(result.Items))
	for i, item := range result.Items {
		sp, err := services.UnmarshalSAMLIdPServiceProvider(item.Value,
			services.WithResourceID(item.ID), services.WithExpires(item.Expires))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		serviceProviders[i] = sp
	}
	return serviceProviders, nil
}

// GetSAMLIdPServiceProvider returns the specified SAML IdP service provider resource.
func (s *SAMLIdPServiceProviderService) GetSAMLIdPServiceProvider(ctx context.Context, name string) (types.SAMLIdPServiceProvider, error) {
	item, err := s.Get(ctx, backend.Key(samlIDPServiceProviderPrefix, name))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("SAML IdP service provider %q doesn't exist", name)
		}
		return nil, trace.Wrap(err)
	}
	sp, err := services.UnmarshalSAMLIdPServiceProvider(item.Value,
		services.WithResourceID(item.ID), services.WithExpires(item.Expires))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sp, nil
}

// CreateSAMLIdPServiceProvider creates a new SAML IdP service provider resource.
func (s *SAMLIdPServiceProviderService) CreateSAMLIdPServiceProvider(ctx context.Context, sp types.SAMLIdPServiceProvider) error {
	if err := sp.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	value, err := services.MarshalSAMLIdPServiceProvider(sp)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key(samlIDPServiceProviderPrefix, sp.GetName()),
		Value:   value,
		Expires: sp.Expiry(),
		ID:      sp.GetResourceID(),
	}
	_, err = s.Create(ctx, item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// UpdateSAMLIdPServiceProvider updates an existing SAML IdP service provider resource.
func (s *SAMLIdPServiceProviderService) UpdateSAMLIdPServiceProvider(ctx context.Context, sp types.SAMLIdPServiceProvider) error {
	if err := sp.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	value, err := services.MarshalSAMLIdPServiceProvider(sp)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key(samlIDPServiceProviderPrefix, sp.GetName()),
		Value:   value,
		Expires: sp.Expiry(),
		ID:      sp.GetResourceID(),
	}
	_, err = s.Update(ctx, item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteSAMLIdPServiceProvider removes the specified SAML IdP service provider resource.
func (s *SAMLIdPServiceProviderService) DeleteSAMLIdPServiceProvider(ctx context.Context, name string) error {
	err := s.Delete(ctx, backend.Key(samlIDPServiceProviderPrefix, name))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("SAML IdP service provider %q doesn't exist", name)
		}
		return trace.Wrap(err)
	}
	return nil
}

// DeleteAllSAMLIdPServiceProviders removes all SAML IdP service provider resources.
func (s *SAMLIdPServiceProviderService) DeleteAllSAMLIdPServiceProviders(ctx context.Context) error {
	startKey := backend.Key(samlIDPServiceProviderPrefix)
	err := s.DeleteRange(ctx, startKey, backend.RangeEnd(startKey))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

const (
	samlIDPServiceProviderPrefix = "saml_idp_service_provider"
)
