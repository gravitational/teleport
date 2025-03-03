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

	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func (s *TerraformSuiteEnterprise) TestSAMLIdPServiceProvider() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	checkDestroyed := func(state *terraform.State) error {
		_, err := s.client.GetSAMLIdPServiceProvider(ctx, "test")
		if trace.IsNotFound(err) {
			return nil
		}
		return err
	}

	name := "teleport_saml_idp_service_provider.test"

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkDestroyed,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("saml_idp_service_provider_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "saml_idp_service_provider"),
					resource.TestCheckResourceAttr(name, "spec.entity_id", "iamshowcase"),
					resource.TestCheckResourceAttr(name, "spec.acs_url", "https://sptest.iamshowcase.com/acs"),
				),
			},
			{
				Config:   s.getFixture("saml_idp_service_provider_0_create.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("saml_idp_service_provider_0_create_with_entitydescriptor.tf"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "saml_idp_service_provider"),
					resource.TestCheckResourceAttr(name, "spec.entity_descriptor", `<md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" xmlns:ds="http://www.w3.org/2000/09/xmldsig#" entityID="IAMShowcase" validUntil="2025-12-09T09:13:31.006Z">
<md:SPSSODescriptor AuthnRequestsSigned="false" WantAssertionsSigned="true" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
<md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified</md:NameIDFormat>
<md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress</md:NameIDFormat>
<md:AssertionConsumerService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://sptest.iamshowcase.com/acs" index="0" isDefault="true"/>
</md:SPSSODescriptor>
</md:EntityDescriptor>
`),
				),
			},
			{
				Config:   s.getFixture("saml_idp_service_provider_0_create_with_entitydescriptor.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("saml_idp_service_provider_1_update.tf"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "saml_idp_service_provider"),
					resource.TestCheckResourceAttr(name, "spec.entity_descriptor", `<md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" xmlns:ds="http://www.w3.org/2000/09/xmldsig#" entityID="IAMShowcase" validUntil="2025-12-09T09:13:31.006Z">
<md:SPSSODescriptor AuthnRequestsSigned="false" WantAssertionsSigned="true" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
<md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified</md:NameIDFormat>
<md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress</md:NameIDFormat>
<md:AssertionConsumerService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://sptest.iamshowcase.com/acs" index="0" isDefault="true"/>
</md:SPSSODescriptor>
</md:EntityDescriptor>
`),
				),
			},
			{
				Config:   s.getFixture("saml_idp_service_provider_1_update.tf"),
				PlanOnly: true,
			},
		},
	})
}
