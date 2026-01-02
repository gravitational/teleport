resource "teleport_role" "kube_verbs" {
  metadata = {
    name = "kube_verbs"
  }

  spec = {
    allow = {
      logins = ["onev8"]
      kubernetes_resources = [
        {
          kind      = "pods"
          name      = "*"
          namespace = "myns"
          verbs     = ["get", "watch", "list"]
        }
      ]
    }
  }

  version = "v8"
}
