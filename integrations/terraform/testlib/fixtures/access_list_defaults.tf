resource "teleport_access_list" "defaults" {
  header = {
    version = "v1"
    metadata = {
      name = "defaults"
    }
  }
  spec = {
    title = "Hello"
    owners = [
      {
        name = "gru"
      }
    ]
  }
}
