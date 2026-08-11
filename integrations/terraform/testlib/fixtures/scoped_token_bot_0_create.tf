resource "teleport_scoped_token" "test" {
  version = "v1"
  metadata = {
    name = "test-bot-scoped-token"
  }
  scope = "/staging/aa"
  spec = {
    bot         = "/staging/aa::server-enroller"
    join_method = "kubernetes"
    kubernetes = {
      allow = [
        { service_account_name = "pod-sa", service_account_namespace = "default" }
      ]
      oidc = {
        issuer = "https://container.googleapis.com/v1/projects/my-project/locations/us-central1-a/clusters/my-cluster-name"
      }
      type = "oidc"
    }
    roles      = ["Bot"]
    usage_mode = "bot"
  }
}