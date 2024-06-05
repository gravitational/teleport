resource "teleport_role" "test" {
  version = "v7"
  metadata = {
    name = "test"
  }

  spec = {
    allow = {
      logins = ["anonymous"]
    }
  }
}
