resource "teleport_role" "admin" {
  version = "v7"
  metadata = {
    name        = "admin"
    description = "admin role"
    expires     = "2032-12-12T00:00:00Z"
  }

  spec = {
    options = {}
    allow   = {}
  }
}

resource "teleport_saml_connector" "test" {
  version = "v2"
  metadata = {
    name    = "test"
    expires = "2032-10-12T07:20:50Z"
    labels = {
      example = "yes"
    }
  }

  spec = {
    attributes_to_roles = [{
      name  = "groups"
      roles = ["admin"]
      value = "okta-admin"
    }]

    acs               = "https://example.com/v1/webapi/saml/acs"
    entity_descriptor = <<EOT
<md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" entityID="http://www.okta.com/exk1hqp7cwfwMSmWU5d7">
<md:IDPSSODescriptor WantAuthnRequestsSigned="false" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
<md:KeyDescriptor use="signing">
<ds:KeyInfo xmlns:ds="http://www.w3.org/2000/09/xmldsig#">
<ds:X509Data>
<ds:X509Certificate>---</ds:X509Certificate>
</ds:X509Data>
</ds:KeyInfo>
</md:KeyDescriptor>
<md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress</md:NameIDFormat>
<md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://dev-82418781.okta.com/app/dev-82418781_evilmartiansteleportsh_1/exk1hqp7cwfwMSmWU5d7/sso/saml"/>
<md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://dev-82418781.okta.com/app/dev-82418781_evilmartiansteleportsh_1/exk1hqp7cwfwMSmWU5d7/sso/saml"/>
</md:IDPSSODescriptor>
</md:EntityDescriptor>				
EOT
  }
}
