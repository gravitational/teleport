resource "teleport_role" "upgrade" {
  metadata = {
    name = "upgrade"
  }

  spec = {
    allow = {
      logins = ["onev8"]
      kubernetes_labels = {
        env = ["dev", "prod"]
      }
      kubernetes_resources = [{
        kind      = "pods"
        api_group = ""
        name      = "*"
        namespace = "*"
        verbs     = ["*"]
      }]
    }
  }

  version = "v8"
}

