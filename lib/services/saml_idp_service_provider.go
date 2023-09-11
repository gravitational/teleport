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

package services

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// SAMLIdPServiceProvider defines an interface for managing SAML IdP service providers.
type SAMLIdPServiceProviders interface {
	// ListSAMLIdPServiceProviders returns a paginated list of all SAML IdP service provider resources.
	ListSAMLIdPServiceProviders(context.Context, int, string) ([]types.SAMLIdPServiceProvider, string, error)
	// GetSAMLIdPServiceProvider returns the specified SAML IdP service provider resources.
	GetSAMLIdPServiceProvider(ctx context.Context, name string) (types.SAMLIdPServiceProvider, error)
	// CreateSAMLIdPServiceProvider creates a new SAML IdP service provider resource.
	CreateSAMLIdPServiceProvider(context.Context, types.SAMLIdPServiceProvider) error
	// UpdateSAMLIdPServiceProvider updates an existing SAML IdP service provider resource.
	UpdateSAMLIdPServiceProvider(context.Context, types.SAMLIdPServiceProvider) error
	// DeleteSAMLIdPServiceProvider removes the specified SAML IdP service provider resource.
	DeleteSAMLIdPServiceProvider(ctx context.Context, name string) error
	// DeleteAllSAMLIdPServiceProviders removes all SAML IdP service providers.
	DeleteAllSAMLIdPServiceProviders(context.Context) error
}

// MarshalSAMLIdPServiceProvider marshals the SAMLIdPServiceProvider resource to JSON.
func MarshalSAMLIdPServiceProvider(serviceProvider types.SAMLIdPServiceProvider, opts ...MarshalOption) ([]byte, error) {
	if err := serviceProvider.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch sp := serviceProvider.(type) {
	case *types.SAMLIdPServiceProviderV1:
		if !cfg.PreserveResourceID {
			copy := *sp
			copy.SetResourceID(0)
			sp = &copy
		}
		return utils.FastMarshal(sp)
	default:
		return nil, trace.BadParameter("unsupported SAML IdP service provider resource %T", sp)
	}
}

// UnmarshalSAMLIdPServiceProvider unmarshals SAMLIdPServiceProvider resource from JSON.
func UnmarshalSAMLIdPServiceProvider(data []byte, opts ...MarshalOption) (types.SAMLIdPServiceProvider, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing SAML IdP service provider data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var h types.ResourceHeader
	if err := utils.FastUnmarshal(data, &h); err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case types.V1:
		var s types.SAMLIdPServiceProviderV1
		if err := utils.FastUnmarshal(data, &s); err != nil {
			return nil, trace.BadParameter(err.Error())
		}
		if err := s.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.ID != 0 {
			s.SetResourceID(cfg.ID)
		}
		if !cfg.Expires.IsZero() {
			s.SetExpiry(cfg.Expires)
		}
		return &s, nil
	}
	return nil, trace.BadParameter("unsupported SAML IdP service provider resource version %q", h.Version)
}

// GenerateIdPServiceProviderFromFields takes `name` and `entityDescriptor` fields and returns a SAMLIdPServiceProvider.
func GenerateIdPServiceProviderFromFields(name string, entityDescriptor string) (types.SAMLIdPServiceProvider, error) {
	if len(name) == 0 {
		return nil, trace.BadParameter("missing name")
	}
	if len(entityDescriptor) == 0 {
		return nil, trace.BadParameter("missing entity descriptor")
	}

	var s types.SAMLIdPServiceProviderV1
	s.SetName(name)
	s.SetEntityDescriptor(entityDescriptor)
	if err := s.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &s, nil
}
