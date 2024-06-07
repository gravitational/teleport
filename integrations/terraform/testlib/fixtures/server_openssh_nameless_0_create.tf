resource "teleport_server" "test" {
  version  = "v2"
  sub_kind = "openssh"
  spec = {
    addr     = "127.0.0.1:22"
    hostname = "test.local"
  }
}
