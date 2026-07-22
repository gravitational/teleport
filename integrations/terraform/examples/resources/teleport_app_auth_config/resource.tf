resource "teleport_app_auth_config" "example" {
  version = "v1"
  metadata = {
    name        = "example"
    description = "Example app auth config"
    labels = {
      example               = "yes"
      "teleport.dev/origin" = "dynamic"
    }
  }

  spec = {
    app_labels = [{
      name   = "teleport.internal/app-sub-kind"
      values = ["mcp"]
    }]
    jwt = {
      issuer   = "https://issuer"
      audience = "teleport"
      jwks_url = "https://issuer/.well-known/jwks.json"
    }
  }
}
