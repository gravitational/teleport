/*
Copyright 2026 Gravitational, Inc.

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

package provider

import (
	"strings"
	"testing"
)

func TestReplaceEntityIDInSamlIdPDescriptorRewritesParsedXML(t *testing.T) {
	descriptor := `<?xml version='1.0'?>
<md:EntityDescriptor xmlns:md='urn:oasis:names:tc:SAML:2.0:metadata' entityID='old-id'>
  <md:SPSSODescriptor protocolSupportEnumeration='urn:oasis:names:tc:SAML:2.0:protocol'>
    <md:AssertionConsumerService Binding='urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST' Location='https://example.com/acs' index='0'/>
  </md:SPSSODescriptor>
</md:EntityDescriptor>`

	rewritten, err := replaceEntityIDInSamlIdPDescriptor(descriptor, "new-id")
	if err != nil {
		t.Fatalf("Failed to parse test xml [%s]", descriptor)
	}

	if !strings.Contains(rewritten, `entityID="new-id"`) {
		t.Fatalf("expected rewritten entity ID, got %q", rewritten)
	}
}

func TestReplaceACSURLInSamlIdPDescriptorRewritesParsedXML(t *testing.T) {
	descriptor := `<?xml version='1.0'?>
<md:EntityDescriptor xmlns:md='urn:oasis:names:tc:SAML:2.0:metadata' entityID='id'>
  <md:SPSSODescriptor protocolSupportEnumeration='urn:oasis:names:tc:SAML:2.0:protocol'>
    <md:AssertionConsumerService Binding='urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST' Location='https://example.com/old-acs' index='0'/>
  </md:SPSSODescriptor>
</md:EntityDescriptor>`

	rewritten, err := replaceACSURLInSamlIdPDescriptor(descriptor, "https://example.com/new-acs")
	if err != nil {
		t.Fatalf("Failed to parse test xml [%s]", descriptor)
	}

	if !strings.Contains(rewritten, `Location="https://example.com/new-acs"`) {
		t.Fatalf("expected rewritten ACS URL, got %q", rewritten)
	}
}
