resource "teleport_role" "splitbrain" {
  version = "v7"
  metadata = {
    name = "splitbrain"
  }

  spec = {
    allow = {
      logins = ["one"]
    }
  }
}
