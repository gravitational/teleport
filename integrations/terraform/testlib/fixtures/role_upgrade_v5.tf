resource "teleport_role" "upgrade" {
  metadata = {
    name = "upgrade"
  }

  spec = {
    allow = {
      logins = ["onev5"]
      kubernetes_labels = {
        env = ["dev", "prod"]
      }
    }
  }

  version = "v5"
}
