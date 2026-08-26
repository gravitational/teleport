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
            name   = "foo"
            values = ["bar"]
          }
        ]
        groups = ["foo", "bar"]
      }
    ]
  }
}