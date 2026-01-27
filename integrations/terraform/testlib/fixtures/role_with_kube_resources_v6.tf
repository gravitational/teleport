resource "teleport_role" "kube_resources_v6" {
  metadata = {
    name = "kube_resources_v6"
  }

  spec = {
    allow = {
      logins = ["onev6"]
      kubernetes_labels = {
        env = ["dev", "prod"]
      }
      kubernetes_resources = [
        {
          kind      = "pod"
          name      = "*"
          namespace = "myns"
        }
      ]
    }
  }

  version = "v6"
}
