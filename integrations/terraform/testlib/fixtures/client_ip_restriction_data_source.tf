resource "teleport_client_ip_restriction" "test" {
  spec = {
    allowed_cidrs = ["10.0.0.0/8"]
  }
}

data "teleport_client_ip_restriction" "test" {
  version = "v1"
  kind    = "client_ip_restriction"
  spec = {
    allowed_cidrs = ["10.0.0.0/8"]
  }
  depends_on = [teleport_client_ip_restriction.test]
}
