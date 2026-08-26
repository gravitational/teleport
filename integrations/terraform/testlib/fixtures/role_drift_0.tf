resource "teleport_role" "splitbrain" {
  version = "v8"
  metadata = {
    name = "splitbrain"
  }

  spec = {
    allow = {
      logins = ["one"]
    }
  }
}
