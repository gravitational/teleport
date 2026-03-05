data "teleport_github_connector" "test" {
  kind    = "github"
  version = "v3"
  metadata = {
    name = "test"
  }
  spec = {
    client_id     = ""
    client_secret = ""
  }
}
