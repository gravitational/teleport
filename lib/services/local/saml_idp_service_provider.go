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
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	preset "github.com/gravitational/teleport/api/types/samlsp"
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
	samlIDPServiceName                  = "teleport_saml_idp_service"
)

// SAMLIdPServiceProviderService manages IdP service providers in the Backend.
type SAMLIdPServiceProviderService struct {
	svc generic.Service[types.SAMLIdPServiceProvider]
	// backend is used to spawn Plugins storage service so that
	// it can be queried from the SAML service.
	backend    backend.Backend
	logger     *slog.Logger
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
func NewSAMLIdPServiceProviderService(b backend.Backend, opts ...SAMLIdPOption) (*SAMLIdPServiceProviderService, error) {
	svc, err := generic.NewService(&generic.ServiceConfig[types.SAMLIdPServiceProvider]{
		Backend:       b,
		PageLimit:     samlIDPServiceProviderMaxPageSize,
		ResourceKind:  types.KindSAMLIdPServiceProvider,
		BackendPrefix: backend.NewKey(samlIDPServiceProviderPrefix),
		MarshalFunc:   services.MarshalSAMLIdPServiceProvider,
		UnmarshalFunc: services.UnmarshalSAMLIdPServiceProvider,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	samlSPService := &SAMLIdPServiceProviderService{
		svc:     *svc,
		backend: b,
		logger:  slog.With(teleport.ComponentKey, "saml-idp"),
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
	if err := services.ValidateSAMLIdPACSURLAndRelayStateInputs(sp); err != nil {
		// logging instead of returning an error cause we do not want to break cache writes on a cluster
		// that already has a service provider with unsupported characters/scheme in the acs_url or relay_state.
		s.logger.WarnContext(ctx, "Provided SAML IdP service provided is invalid", "error", err)
	}
	if sp.GetEntityDescriptor() == "" {
		if err := s.configureEntityDescriptorPerPreset(sp); err != nil {
			errMsg := fmt.Errorf("failed to configure entity descriptor with the given entity_id %q and acs_url %q: %w",
				sp.GetEntityID(), sp.GetACSURL(), err)
			s.logger.ErrorContext(ctx, "failed to configure entity descriptor",
				"entity_id", sp.GetEntityID(),
				"acs_url", sp.GetACSURL(),
				"error", err,
			)
			return trace.BadParameter(errMsg.Error())
		}
	}

	// we only verify if the entity ID field in the spec matches with the entity descriptor.
	// filtering is done only for logging purpose.
	if err := services.ValidateAndFilterEntityDescriptor(sp, services.SAMLACSInputPermissiveFilter); err != nil {
		return trace.Wrap(err)
	}

	// embed attribute mapping in entity descriptor
	if err := s.embedAttributeMapping(sp); err != nil {
		return trace.Wrap(err)
	}

	item, err := s.svc.MakeBackendItem(sp, sp.GetName())
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(s.svc.RunWhileLocked(ctx, []string{samlIDPServiceProviderModifyLock}, samlIDPServiceProviderModifyLockTTL,
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
	if err := services.ValidateSAMLIdPACSURLAndRelayStateInputs(sp); err != nil {
		// logging instead of returning an error cause we do not want to break cache writes on a cluster
		// that already has a service provider with unsupported characters/scheme in the acs_url or relay_state.
		s.logger.WarnContext(ctx, "Provided SAML IdP service provided is invalid", "error", err)
	}

	// we only verify if the entity ID field in the spec matches with the entity descriptor.
	// filtering is done only for logging purpose.
	if err := services.ValidateAndFilterEntityDescriptor(sp, services.SAMLACSInputPermissiveFilter); err != nil {
		return trace.Wrap(err)
	}

	// embed attribute mapping in entity descriptor
	if err := s.embedAttributeMapping(sp); err != nil {
		return trace.Wrap(err)
	}

	item, err := s.svc.MakeBackendItem(sp, sp.GetName())
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(s.svc.RunWhileLocked(ctx, []string{samlIDPServiceProviderModifyLock}, samlIDPServiceProviderModifyLockTTL,
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
	if err := spReferencedByAWSICPlugin(ctx, s.backend, name); err != nil {
		return trace.Wrap(err)
	}
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

// configureEntityDescriptorPerPreset configures entity descriptor based on SAML service provider preset.
func (s *SAMLIdPServiceProviderService) configureEntityDescriptorPerPreset(sp types.SAMLIdPServiceProvider) error {
	switch sp.GetPreset() {
	case preset.GCPWorkforce:
		return trace.Wrap(s.generateAndSetEntityDescriptor(sp))
	default:
		// fetchAndSetEntityDescriptor is expected to return error if it fails
		// to fetch a valid entity descriptor.
		if err := s.fetchAndSetEntityDescriptor(sp); err != nil {
			s.logger.DebugContext(context.Background(), "Failed to fetch entity descriptor",
				"entity_id", sp.GetEntityID(),
				"error", err,
			)
			// We aren't interested in checking error type as any occurrence of error
			// mean entity descriptor was not set.
			return trace.Wrap(s.generateAndSetEntityDescriptor(sp))
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
		return trace.Wrap(trace.BadParameter("unexpected response status: %q", resp.StatusCode))
	}

	body, err := utils.ReadAtMost(resp.Body, teleport.MaxHTTPResponseSize)
	if err != nil {
		return trace.Wrap(err)
	}

	// parse body to check if it's a valid entity descriptor
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
	s.logger.InfoContext(context.Background(), "Generating a default entity_descriptor",
		"entity_id", sp.GetEntityID(),
		"acs_url", sp.GetACSURL(),
	)

	acsURL, err := url.Parse(sp.GetACSURL())
	if err != nil {
		return trace.Wrap(err)
	}

	newServiceProvider := saml.ServiceProvider{
		EntityID:          sp.GetEntityID(),
		AcsURL:            *acsURL,
		AuthnNameIDFormat: saml.UnspecifiedNameIDFormat,
	}

	ed := newServiceProvider.Metadata()
	// HTTPArtifactBinding is defined when entity descriptor is generated
	// using crewjam/saml https://github.com/crewjam/saml/blob/main/service_provider.go#L228.
	// But we do not support it, so filter it out below.
	// Error and warnings are swallowed because the descriptor is Teleport generated and
	// users have no control over sanitizing filtered binding.
	services.FilterSAMLEntityDescriptor(ed, true /* quiet */)
	edXMLBytes, err := xml.MarshalIndent(ed, "", "  ")
	if err != nil {
		return trace.Wrap(err)
	}

	sp.SetEntityDescriptor(string(edXMLBytes))
	return nil
}

// embedAttributeMapping embeds attribute mapping input into entity descriptor.
func (s *SAMLIdPServiceProviderService) embedAttributeMapping(sp types.SAMLIdPServiceProvider) error {
	ed, err := samlsp.ParseMetadata([]byte(sp.GetEntityDescriptor()))
	if err != nil {
		return trace.Wrap(err)
	}

	teleportSPSSODescriptorIndex, _ := GetTeleportSPSSODescriptor(ed.SPSSODescriptors)
	switch attrMapLen := len(sp.GetAttributeMapping()); {
	case attrMapLen == 0:
		if teleportSPSSODescriptorIndex == 0 {
			s.logger.DebugContext(context.Background(), "No custom attribute mapping values provided,SAML assertion will default to uid and eduPersonAffiliate",
				"entity_id", sp.GetEntityID(),
			)
			return nil
		} else {
			// delete Teleport SPSSODescriptor
			ed.SPSSODescriptors = append(ed.SPSSODescriptors[:teleportSPSSODescriptorIndex], ed.SPSSODescriptors[teleportSPSSODescriptorIndex+1:]...)
		}
	case attrMapLen > 0:
		if teleportSPSSODescriptorIndex == 0 {
			ed.SPSSODescriptors = append(ed.SPSSODescriptors, genTeleportSPSSODescriptor(sp.GetAttributeMapping()))
		} else {
			// if there is existing SPSSODescriptor with "teleport_saml_idp_service" service name, replace it at
			// existingTeleportSPSSODescriptorIndex to avoid duplication or possible fragmented SPSSODescriptor.
			ed.SPSSODescriptors[teleportSPSSODescriptorIndex] = genTeleportSPSSODescriptor(sp.GetAttributeMapping())
		}
	}

	edWithAttributes, err := xml.MarshalIndent(ed, " ", "    ")
	if err != nil {
		return trace.Wrap(err)
	}

	sp.SetEntityDescriptor(string(edWithAttributes))
	return nil
}

// GetTeleportSPSSODescriptor returns Teleport embedded SPSSODescriptor and its index from a
// list of SPSSODescriptors. The correct SPSSODescriptor is determined by searching for
// AttributeConsumingService element with ServiceNames named teleport_saml_idp_service.
func GetTeleportSPSSODescriptor(spSSODescriptors []saml.SPSSODescriptor) (embeddedSPSSODescriptorIndex int, teleportSPSSODescriptor saml.SPSSODescriptor) {
	for descriptorIndex, descriptor := range spSSODescriptors {
		for _, acs := range descriptor.AttributeConsumingServices {
			for _, serviceName := range acs.ServiceNames {
				if serviceName.Value == samlIDPServiceName {
					return descriptorIndex, spSSODescriptors[descriptorIndex]
				}
			}
		}
	}
	return
}

// genTeleportSPSSODescriptor returns saml.SPSSODescriptor populated with Attribute Consuming Service
// named teleport_saml_idp_service and attributeMapping input (types.SAMLAttributeMapping) converted to
// saml.RequestedAttributes format.
func genTeleportSPSSODescriptor(attributeMapping []*types.SAMLAttributeMapping) saml.SPSSODescriptor {
	var reqs []saml.RequestedAttribute
	for _, v := range attributeMapping {
		reqs = append(reqs, saml.RequestedAttribute{
			Attribute: saml.Attribute{
				FriendlyName: v.Name,
				Name:         v.Name,
				NameFormat:   v.NameFormat,
				Values:       []saml.AttributeValue{{Value: v.Value}},
			},
		})
	}
	return saml.SPSSODescriptor{
		AttributeConsumingServices: []saml.AttributeConsumingService{
			{
				// ServiceNames is hardcoded with value teleport_saml_idp_service to make the descriptor
				// recognizable throughout SAML SSO flow. Attribute mapping should only ever
				// edit SPSSODescriptor containing teleport_saml_idp_service element. Otherwise, we risk
				// overriding SP managed SPSSODescriptor!
				ServiceNames:        []saml.LocalizedName{{Value: samlIDPServiceName}},
				RequestedAttributes: reqs,
			},
		},
	}
}

// spReferencedByAWSICPlugin returns a BadParameter error if the serviceProviderName
// is referenced in the AWS Identity Center plugin.
func spReferencedByAWSICPlugin(ctx context.Context, bk backend.Backend, serviceProviderName string) error {
	pluginService := NewPluginsService(bk)
	plugins, err := pluginService.GetPlugins(ctx, false /* withSecrets */)
	if err != nil {
		return trace.Wrap(err)
	}

	for _, p := range plugins {
		pluginV1, ok := p.(*types.PluginV1)
		if !ok {
			continue
		}
		if pluginV1.GetType() != types.PluginType(types.PluginTypeAWSIdentityCenter) {
			continue
		}
		if awsIC := pluginV1.Spec.GetAwsIc(); awsIC != nil {
			if awsIC.SamlIdpServiceProviderName == serviceProviderName {
				return trace.BadParameter("cannot delete SAML service provider currently referenced by AWS Identity Center integration %q", pluginV1.GetName())
			}
		}
	}

	return nil
}
