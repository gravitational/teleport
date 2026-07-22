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
        node_labels_expression = "labels.foo == \"bar\""
        groups                 = ["foo", "bar"]
        sudoers                = ["abcd1234"]
        uid                    = 1234
        gid                    = 1234
        default_shell          = "/bin/bash"
      }
    ]
  }
}