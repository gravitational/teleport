# Teleport OIDC connector
# 
# Please note that OIDC connector will work in Enterprise version only. Check the setup docs:
# https://goteleport.com/docs/enterprise/sso/oidc/

variable "oidc_secret" {}

resource "teleport_oidc_connector" "example" {
  metadata = {
    name = "example"
    labels = {
      test = "yes"
    }
  }

  spec = {
    client_id = "client"
    client_secret = var.oidc_secret

    claims_to_roles = [{
      claim = "test"
      roles = ["terraform"]
    }]

    redirect_url = ["https://example.com/redirect"]
  }
}
