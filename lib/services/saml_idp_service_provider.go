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
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"strings"

	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"
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
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch sp := serviceProvider.(type) {
	case *types.SAMLIdPServiceProviderV1:
		if err := sp.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return utils.FastMarshal(maybeResetProtoRevision(cfg.PreserveRevision, sp))
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
			return nil, trace.BadParameter("%s", err)
		}
		if err := s.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
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

	return trace.Wrap(validateAssertionConsumerServicesEndpoint(acs.Location))
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
					slog.WarnContext(context.Background(), "AssertionConsumerService binding for entity is invalid and will be ignored",
						"entity_id", ed.EntityID,
						"error", err,
					)
				}
				return true
			}

			return false
		})

		originalCount += len(ed.SPSSODescriptors[i].AssertionConsumerServices)
		filteredCount += len(filtered)

		ed.SPSSODescriptors[i].AssertionConsumerServices = filtered
	}

	if filteredCount != originalCount {
		return trace.BadParameter("Entity descriptor for entity %q contains unsupported AssertionConsumerService binding or location", ed.EntityID)
	}

	return nil
}

// invalidSAMLIdPACSURLChars contains low hanging HTML tag characters that are more
// commonly used in xss payload. This is not a comprehensive list but is only
// meant to increase the ost of xss payload.
const invalidSAMLIdPACSURLChars = `<>"!;`

// SAMLACSInputFilteringThreshold defines level of strictness for entity descriptor filtering.
type SAMLACSInputFilteringThreshold string

const (
	// SAMLACSInputStrictFilter indicates ValidateAndFilterEntityDescriptor to return an error on
	// any instance of unsupported ACS value.
	SAMLACSInputStrictFilter SAMLACSInputFilteringThreshold = "SAMLACSInputStrictFilter"
	// SAMLACSInputPermissiveFilter indicates ValidateAndFilterEntityDescriptor to ignore an error on
	// any instance of unsupported ACS value.
	SAMLACSInputPermissiveFilter SAMLACSInputFilteringThreshold = "SAMLACSInputPermissiveFilter"
)

// ValidateAndFilterEntityDescriptor validates entity id and ACS value. It specifically:
//   - checks for a valid entity descriptor XML format.
//   - checks for a matching entity ID field in both the entity_id field and entity ID contained in the value of
//     entity_descriptor field.
//   - performs filtering on the Assertion Consumer service (ACS) binding format or its location URL endpoint.
//     filterThreshold dictates if ValidateAndFilterEntityDescriptor should return or ignore error on filtering result.
func ValidateAndFilterEntityDescriptor(sp types.SAMLIdPServiceProvider, filterThreshold SAMLACSInputFilteringThreshold) error {
	edOriginal, err := samlsp.ParseMetadata([]byte(sp.GetEntityDescriptor()))
	if err != nil {
		return trace.BadParameter("invalid entity descriptor for SAML IdP Service Provider %q: %v", sp.GetEntityID(), err)
	}

	if edOriginal.EntityID != sp.GetEntityID() {
		return trace.BadParameter("entity ID parsed from the entity descriptor does not match the entity ID in the SAML IdP service provider object")
	}

	if err := FilterSAMLEntityDescriptor(edOriginal, false /* quiet */); err != nil {
		if filterThreshold == SAMLACSInputStrictFilter {
			return trace.BadParameter("Entity descriptor for SAML IdP Service Provider %q contains unsupported ACS bindings: %v", sp.GetEntityID(), err)
		}
	}

	return nil
}

// validateAssertionConsumerServicesEndpoint ensures that the Assertion Consumer Service location
// is a valid HTTPS endpoint.
func validateAssertionConsumerServicesEndpoint(acs string) error {
	endpoint, err := url.Parse(acs)
	switch {
	case err != nil:
		return trace.BadParameter("acs location endpoint %q could not be parsed: %v", acs, err)
	case endpoint.Scheme != "https":
		return trace.BadParameter("invalid scheme %q in acs location endpoint %q (must be 'https')", endpoint.Scheme, acs)
	}

	if strings.ContainsAny(acs, invalidSAMLIdPACSURLChars) {
		return trace.BadParameter("acs location endpoint contains an unsupported character")
	}
	return nil
}

// ValidateSAMLIdPACSURLAndRelayStateInputs performs validation on SAML IdP Service Provider
// ACS URL and Relay State fields.
func ValidateSAMLIdPACSURLAndRelayStateInputs(sp types.SAMLIdPServiceProvider) error {
	if sp.GetACSURL() != "" {
		if err := validateAssertionConsumerServicesEndpoint(sp.GetACSURL()); err != nil {
			return trace.Wrap(err)
		}
	}

	if strings.ContainsAny(sp.GetRelayState(), invalidSAMLIdPACSURLChars) {
		return trace.BadParameter("relay state contains an unsupported character")
	}

	return nil
}

// NewSAMLTestSPMetadata creates a new entity descriptor for tests.
func NewSAMLTestSPMetadata(entityID, acsURL string) string {
	return fmt.Sprintf(samlTestSPMetadata, entityID, acsURL)
}

// samlTestSPMetadata mimics metadata format generated by saml.ServiceProvider.Metadata()
const samlTestSPMetadata = `<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" validUntil="2023-12-09T23:43:58.16Z" entityID="%s">
<SPSSODescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" validUntil="2023-12-09T23:43:58.16Z" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol" AuthnRequestsSigned="false" WantAssertionsSigned="true">
  <NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified</NameIDFormat>
  <AssertionConsumerService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="%s" index="1"></AssertionConsumerService>
</SPSSODescriptor>
</EntityDescriptor>
`
