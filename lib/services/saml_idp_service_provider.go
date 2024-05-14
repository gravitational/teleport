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
	"slices"

	"github.com/crewjam/saml"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

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
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch sp := serviceProvider.(type) {
	case *types.SAMLIdPServiceProviderV1:
		if err := sp.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return utils.FastMarshal(maybeResetProtoResourceID(cfg.PreserveResourceID, sp))
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

// supportedACSBindings is the set of AssertionConsumerService bindings that teleport supports.
var supportedACSBindings = map[string]struct{}{
	saml.HTTPPostBinding:     {},
	saml.HTTPRedirectBinding: {},
}

// ValidateAssertionConsumerService checks if a given assertion consumer service is usable by teleport. Note that
// it is permissible for a service provider to include acs endpoints that are not compatible with teleport, so long
// as at least one _is_ compatible.
func ValidateAssertionConsumerService(acs saml.IndexedEndpoint) error {
	if _, ok := supportedACSBindings[acs.Binding]; !ok {
		return trace.BadParameter("unsupported acs binding: %q", acs.Binding)
	}

	if acs.Location == "" {
		return trace.BadParameter("acs location endpoint is missing or could not be decoded for %q binding", acs.Binding)
	}

	return trace.Wrap(ValidateAssertionConsumerServicesEndpoint(acs.Location))
}

// FilterSAMLEntityDescriptor performs a filter in place to remove unsupported and/or insecure fields from
// a saml entity descriptor. Specifically, it removes acs endpoints that are either of an unsupported kind,
// or are using a non-https endpoint. We perform filtering rather than outright rejection because it is generally
// expected that a service provider will successfully support a given ACS so long as they have at least one
// compatible binding.
func FilterSAMLEntityDescriptor(ed *saml.EntityDescriptor, quiet bool) error {
	var originalCount int
	var filteredCount int
	for i := range ed.SPSSODescriptors {
		filtered := slices.DeleteFunc(ed.SPSSODescriptors[i].AssertionConsumerServices, func(acs saml.IndexedEndpoint) bool {
			if err := ValidateAssertionConsumerService(acs); err != nil {
				if !quiet {
					log.Warnf("AssertionConsumerService binding for entity %q is invalid and will be ignored: %v", ed.EntityID, err)
				}
				return true
			}

			return false
		})

		originalCount += len(ed.SPSSODescriptors[i].AssertionConsumerServices)
		filteredCount += len(filtered)

		ed.SPSSODescriptors[i].AssertionConsumerServices = filtered
	}

	if filteredCount == 0 && originalCount != 0 {
		return trace.BadParameter("no AssertionConsumerService bindings for entity %q passed validation", ed.EntityID)
	}

	return nil
}

// ValidateAssertionConsumerServicesEndpoint ensures that the Assertion Consumer Service location
// is a valid HTTPS endpoint.
func ValidateAssertionConsumerServicesEndpoint(acs string) error {
	endpoint, err := url.Parse(acs)
	switch {
	case err != nil:
		return trace.BadParameter("acs location endpoint %q could not be parsed: %v", acs, err)
	case endpoint.Scheme != "https":
		return trace.BadParameter("invalid scheme %q in acs location endpoint %q (must be 'https')", endpoint.Scheme, acs)
	}

	return nil
}
