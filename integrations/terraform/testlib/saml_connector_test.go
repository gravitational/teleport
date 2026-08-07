/*
Copyright 2015-2021 Gravitational, Inc.

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

package testlib

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync/atomic"

	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func (s *TerraformSuiteEnterprise) TestSAMLConnector() {
	s.T().Skip("TODO(hugoShaka): fix test")
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	checkDestroyed := func(state *terraform.State) error {
		_, err := s.client.GetSAMLConnector(ctx, "test", false)
		if trace.IsNotFound(err) {
			return nil
		}

		return err
	}

	name := "teleport_saml_connector.test"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkDestroyed,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("saml_connector_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "saml"),
					resource.TestCheckResourceAttr(name, "spec.acs", "https://example.com/v1/webapi/saml/acs"),
					resource.TestCheckResourceAttr(name, "spec.attributes_to_roles.0.name", "groups"),
					resource.TestCheckResourceAttr(name, "spec.attributes_to_roles.0.roles.0", "admin"),
					resource.TestCheckResourceAttr(name, "spec.attributes_to_roles.0.value", "okta-admin"),
				),
			},
			{
				Config:   s.getFixture("saml_connector_0_create.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("saml_connector_1_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "saml"),
					resource.TestCheckResourceAttr(name, "spec.acs", "https://example.com/v1/webapi/saml/acs"),
					resource.TestCheckResourceAttr(name, "spec.attributes_to_roles.0.name", "groups"),
					resource.TestCheckResourceAttr(name, "spec.attributes_to_roles.0.roles.0", "admin"),
					resource.TestCheckResourceAttr(name, "spec.attributes_to_roles.0.value", "okta-admin"),
				),
			},
			{
				Config:   s.getFixture("saml_connector_1_update.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteEnterprise) TestImportSAMLConnector() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	metadataAvailable := &atomic.Bool{}
	metadataAvailable.Store(true)
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !metadataAvailable.Load() {
			http.Error(w, "metadata unavailable", http.StatusServiceUnavailable)
			return
		}
		fmt.Fprintln(w, testDescriptor)
	}))
	s.T().Cleanup(httpServer.Close)

	r := "teleport_saml_connector"
	id := "test_import"
	name := r + "." + id

	// Set up role.
	roleName := "rolex"
	rolexSpec, err := types.NewRole(roleName, types.RoleSpecV6{})
	require.NoError(s.T(), err)

	_, err = s.client.UpsertRole(ctx, rolexSpec)
	require.NoError(s.T(), err)

	samlConnector := &types.SAMLConnectorV2{
		Metadata: types.Metadata{
			Name: id,
		},
		Spec: types.SAMLConnectorSpecV2{
			AssertionConsumerService: "https://example.com/v1/webapi/saml/acs",
			EntityDescriptorURL:      httpServer.URL,
			AttributesToRoles: []types.AttributeMapping{
				{
					Name:  "map attrx to rolex",
					Value: "attrx",
					Roles: []string{roleName},
				},
			},
		},
	}

	err = samlConnector.CheckAndSetDefaults()
	require.NoError(s.T(), err)

	_, err = s.client.UpsertSAMLConnector(ctx, samlConnector)
	require.NoError(s.T(), err)
	// Import must use cached metadata instead of fetching the unavailable URL.
	metadataAvailable.Store(false)

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		Steps: []resource.TestStep{
			{
				Config:        s.terraformConfig + "\n" + `resource "` + r + `" "` + id + `" { }`,
				ResourceName:  name,
				ImportState:   true,
				ImportStateId: id,
				ImportStateCheck: func(state []*terraform.InstanceState) error {
					require.Equal(s.T(), "saml", state[0].Attributes["kind"])
					require.Equal(s.T(), "https://example.com/v1/webapi/saml/acs", state[0].Attributes["spec.acs"])
					require.Equal(s.T(), httpServer.URL, state[0].Attributes["spec.entity_descriptor_url"])

					return nil
				},
			},
		},
	})
}

// Verify that Terraform can manage a connector when previously validated metadata URLs become
// unavailable:
//  1. Create a connector while its old metadata URL is available.
//  2. Make the old URL unavailable and update to a reachable new URL. The pre-update read must not
//     fetch metadata, while the write validates the new URL.
//  3. Make both URLs unavailable. Resource refresh, data-source read, and destroy must still work.
func (s *TerraformSuiteEnterprise) TestSAMLConnectorWithEntityDescriptorURL() {
	oldMetadataUnavailable := &atomic.Bool{}
	allMetadataUnavailable := &atomic.Bool{}

	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if allMetadataUnavailable.Load() || (oldMetadataUnavailable.Load() && strings.HasPrefix(r.URL.Path, "/old/")) {
			http.Error(w, "metadata unavailable", http.StatusServiceUnavailable)
			return
		}
		fmt.Fprintln(w, testDescriptor)
	}))
	s.T().Cleanup(httpServer.Close)

	oldMetadataURL := httpServer.URL + "/old"
	newMetadataURL := httpServer.URL + "/new"

	resourceName := "teleport_saml_connector.test"
	dataSourceName := "data.teleport_saml_connector.test"
	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("saml_connector_0_create_with_entitydescriptorurl.tf", oldMetadataURL),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "spec.entity_descriptor_url", oldMetadataURL+"/app/exk4d7tmnz9DEaEw85d7/sso/saml/metadata"),
					func(*terraform.State) error {
						oldMetadataUnavailable.Store(true)
						return nil
					},
				),
			},
			{
				Config: s.getFixture("saml_connector_0_create_with_entitydescriptorurl.tf", newMetadataURL),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "spec.entity_descriptor_url", newMetadataURL+"/app/exk4d7tmnz9DEaEw85d7/sso/saml/metadata"),
					resource.TestCheckResourceAttr(dataSourceName, "metadata.name", "test"),
					resource.TestCheckResourceAttr(dataSourceName, "spec.entity_descriptor_url", newMetadataURL+"/app/exk4d7tmnz9DEaEw85d7/sso/saml/metadata"),
					func(*terraform.State) error {
						allMetadataUnavailable.Store(true)
						return nil
					},
				),
			},
			{
				Config:   s.getFixture("saml_connector_0_create_with_entitydescriptorurl.tf", newMetadataURL),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteEnterprise) TestSAMLConnectorWithoutEntityDescriptor() {
	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		Steps: []resource.TestStep{
			{
				Config:      s.getFixture("saml_connector_0_create_without_entitydescriptor.tf"),
				ExpectError: regexp.MustCompile("AnyOf 'entity_descriptor, entity_descriptor_url' keys must be present"),
			},
		},
	})
}

var testDescriptor = `<?xml version="1.0" encoding="UTF-8"?><md:EntityDescriptor entityID="http://www.okta.com/exk6q03a7zcGFUlm71d7" xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata"><md:IDPSSODescriptor WantAuthnRequestsSigned="false" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol"><md:KeyDescriptor use="signing"><ds:KeyInfo xmlns:ds="http://www.w3.org/2000/09/xmldsig#"><ds:X509Data><ds:X509Certificate>MIIDujCCAqKgAwIBAgIGAYXAcx/eMA0GCSqGSIb3DQEBCwUAMIGdMQswCQYDVQQGEwJVUzETMBEG
A1UECAwKQ2FsaWZvcm5pYTEWMBQGA1UEBwwNU2FuIEZyYW5jaXNjbzENMAsGA1UECgwET2t0YTEU
MBIGA1UECwwLU1NPUHJvdmlkZXIxHjAcBgNVBAMMFWdyYXZpdGF0aW9uYWwtcHJldmlldzEcMBoG
CSqGSIb3DQEJARYNaW5mb0Bva3RhLmNvbTAeFw0yMzAxMTcxNTU2MjhaFw0zMzAxMTcxNTU3Mjha
MIGdMQswCQYDVQQGEwJVUzETMBEGA1UECAwKQ2FsaWZvcm5pYTEWMBQGA1UEBwwNU2FuIEZyYW5j
aXNjbzENMAsGA1UECgwET2t0YTEUMBIGA1UECwwLU1NPUHJvdmlkZXIxHjAcBgNVBAMMFWdyYXZp
dGF0aW9uYWwtcHJldmlldzEcMBoGCSqGSIb3DQEJARYNaW5mb0Bva3RhLmNvbTCCASIwDQYJKoZI
hvcNAQEBBQADggEPADCCAQoCggEBALoIr14BqjPFRJGJOsVQ8RWKkhYzdP3foRRLZTQLHlv50afF
UZCgng3HV8n5rFoZgSLEeVYs4+9RIQKctPCYQ9YE+dtQrU6ZkxEixvwvpxtFEfAK1KriQdPjBNxM
EyTr4+khUckbS4cPNPy5/Bjw/TakCQCC5P/7r7GRcesEsuPnOnHPiIOdeEZ+DEpgndf0kwalgKs9
xJWepooOLH9cWtFQXHM15PVbvFxW1fpt6LzXXxiW0UZxGP+744qY92KY9kIyV+gbtH4xOjQVftvc
GIsxdr8pZ9PSDW+Ivx6KaR8zv+Kt6HRg6Svi8CJ8iA6uAPrbw0FxSlwuh0/GCWF2rTkCAwEAATAN
BgkqhkiG9w0BAQsFAAOCAQEAFPtGtJNd2E95wTV5++FLelUaWCwZfChhOyBmNZc4y/aCHEMMApbQ
ZdmVO08rv/E96e8xguURSHNUSctFe3sM/8ZDNVYI4MPt1Tz4PafVmQ0C8qrgNM2/nWlXHih7BXwH
zWWhkLclbcBUszNCRuF9FeIDCOUXTP5qHcnM3cWTci6MWssIHe0uj4pPXFCPPvl6Pe200y6l9coz
lAA/ybt7fxOF+iw1lDHPydVKP1diu0mJMS1jcytW3D9pAD9J23q3N6kT3ZpKBlr0HGQhCmuLNqj/
ENwliNq8+0Ouew4Ww08DP+a1HdK4F1POi4MpJyyeAlaXvhGizIR1d9jV4hlqVQ==</ds:X509Certificate></ds:X509Data></ds:KeyInfo></md:KeyDescriptor><md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress</md:NameIDFormat><md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://gravitational-preview.oktapreview.com/app/gravitational-preview_oktaintegrationtestingsso_1/exk6q03a7zcGFUlm71d7/sso/saml"/><md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://gravitational-preview.oktapreview.com/app/gravitational-preview_oktaintegrationtestingsso_1/exk6q03a7zcGFUlm71d7/sso/saml"/></md:IDPSSODescriptor></md:EntityDescriptor>`
