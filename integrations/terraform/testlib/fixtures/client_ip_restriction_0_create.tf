resource "teleport_client_ip_restriction" "test" {
  spec = {
    allowed_cidrs = ["10.0.0.0/8"]
  }
}
