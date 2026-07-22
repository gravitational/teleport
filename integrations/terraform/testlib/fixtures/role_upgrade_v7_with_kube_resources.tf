
resource "teleport_role" "upgrade" {
  metadata = {
    name = "upgrade"
  }

  spec = {
    allow = {
      logins = ["onev7"]

      kubernetes_labels = {
        env = ["dev", "prod"]
      }

      kubernetes_resources = [
        {
          kind      = "deployment"
          name      = "*"
          namespace = "myns"
          verbs     = ["get"]
        }
      ]
    }
  }

  version = "v7"
}
