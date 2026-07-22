resource "teleport_role" "test" {
  version = "v8"
  metadata = {
    name = "test"
  }

  spec = {
    allow = {
      logins = ["anonymous"]
    }
  }
}
