resource "teleport_static_host_user" "test" {
  version = "v2"
  metadata = {
    name = "test"
  }
  spec = {
    matchers = [
      {
        node_labels = [
          {
            name   = "baz"
            values = ["quux"]
          }
        ]
        groups = ["baz", "quux"]
      }
    ]
  }
}