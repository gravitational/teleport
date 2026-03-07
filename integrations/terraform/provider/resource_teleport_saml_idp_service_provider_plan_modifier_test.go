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
