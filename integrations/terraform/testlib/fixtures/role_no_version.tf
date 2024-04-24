resource "teleport_role" "test" {
  metadata = {
    name = "test"
  }

  spec = {
    allow = {
      logins = ["anonymous"]
    }
  }
}
