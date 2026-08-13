resource "teleport_server" "unscoped" {
  version  = "v2"
  sub_kind = "openssh"
  metadata = {
    name = "test-unscoped"
  }
  spec = {
    addr     = "127.0.0.1:22"
    hostname = "unscoped.local"
  }
}

resource "teleport_server" "scoped" {
  depends_on = [teleport_server.unscoped]

  version  = "v2"
  sub_kind = "openssh"
  scope    = "/foo/bar"
  metadata = {
    name = "test-scoped"
  }
  spec = {
    addr     = "127.0.0.1:2222"
    hostname = "scoped.local"
  }
}
