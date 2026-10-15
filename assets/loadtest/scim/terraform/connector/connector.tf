terraform {
  required_providers {
    teleport = {
      source  = "terraform.releases.teleport.dev/gravitational/teleport"
      version = "~> 18.0"
    }
  }
}

provider "teleport" {
  addr = var.proxy_addr
}

resource "teleport_saml_connector" "idp" {
  version = "v2"

  metadata = {
    name = var.connector_name
  }

  spec = {
    # Connector role mapping.
    attributes_to_roles = [{
      name  = "groups"
      roles = ["requester"]
      value = "Everyone"
    }]

    # The ACS URL is derived from the proxy address.
    acs               = "https://${var.proxy_addr}/v1/webapi/saml/acs/${var.connector_name}"
    entity_descriptor = file(var.entity_descriptor_file)
  }
}
