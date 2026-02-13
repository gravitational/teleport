resource "teleport_app_auth_config" "test" {
  version = "v1"
  metadata = {
    name        = "test"
    description = "Test app auth config"
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
