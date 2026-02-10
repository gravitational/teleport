
resource "teleport_role" "kube_resources_v7" {
  metadata = {
    name = "kube_resources_v7"
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
