resource "teleport_server" "test" {
  version  = "v2"
  sub_kind = "openssh"
  metadata = {
    name = "test"
  }
  spec = {
    addr     = "127.0.0.1:22"
    hostname = "test.local"
  }
}
