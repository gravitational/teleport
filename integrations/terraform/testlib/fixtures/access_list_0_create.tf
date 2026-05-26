resource "teleport_access_list" "test" {
  header = {
    version = "v1"
    metadata = {
      name = "test"
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
