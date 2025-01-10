resource "teleport_role" "kube_verbs" {
  metadata = {
    name = "kube_verbs"
  }

  spec = {
    allow = {
      logins = ["onev6"]
      kubernetes_resources = [
        {
          kind      = "pod"
          name      = "*"
          namespace = "myns"
          verbs     = ["get", "watch", "list"]
        }
      ]
    }
  }

  version = "v7"
}
