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

package services

import (
	"context"
	"net/url"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// SAMLIdpServiceProviderGetter defines interface for fetching SAMLIdPServiceProvider resources.
type SAMLIdpServiceProviderGetter interface {
	ListSAMLIdPServiceProviders(ctx context.Context, pageSize int, nextKey string) ([]types.SAMLIdPServiceProvider, string, error)
}

// SAMLIdPServiceProviders defines an interface for managing SAML IdP service providers.
type SAMLIdPServiceProviders interface {
	SAMLIdpServiceProviderGetter
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
			copy.SetRevision("")
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
		if cfg.Revision != "" {
			s.SetRevision(cfg.Revision)
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

// ValidateAssertionConsumerServicesEndpoint ensures that the Assertion Consumer Service location
// is a valid HTTPS endpoint.
func ValidateAssertionConsumerServicesEndpoint(acs string) error {
	endpoint, err := url.Parse(acs)
	switch {
	case err != nil:
		return trace.Wrap(err)
	case endpoint.Scheme != "https":
		return trace.BadParameter("the assertion consumer services location must be an https endpoint")
	}

	return nil
}
