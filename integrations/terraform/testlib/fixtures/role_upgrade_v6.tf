resource "teleport_role" "upgrade" {
  metadata = {
    name = "upgrade"
  }

  spec = {
    allow = {
      logins = ["onev6"]
      kubernetes_labels = {
        env = ["dev", "prod"]
      }
    }
  }

  version = "v6"
}
