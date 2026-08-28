
resource "teleport_role" "kube_resources_v8" {
  metadata = {
    name = "kube_resources_v8"
  }

  spec = {
    allow = {
      logins = ["onev8"]

      kubernetes_labels = {
        env = ["dev", "prod"]
      }

      kubernetes_resources = [
        {
          kind      = "pods"
          name      = "*"
          namespace = "myns"
          verbs     = ["get"]
        },
        {
          kind      = "deployments"
          api_group = "apps"
          name      = "*"
          namespace = "myns"
          verbs     = ["get"]
        }
      ]
    }
  }

  version = "v8"
}
