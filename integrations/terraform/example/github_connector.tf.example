# Terraform Github connector

variable "github_secret" {}

resource "teleport_github_connector" "github" {
  # This section tells Terraform that role example must be created before the GitHub connector
  depends_on = [
    teleport_role.example
  ]

  metadata = {
     name = "example"
     labels = {
       example = "yes"
     }
  }
  
  spec = {
    client_id = "client"
    client_secret = var.github_secret

    teams_to_roles = [{
       organization = "gravitational"
       team = "devs"
       roles = ["example"]
    }]
  }
}
