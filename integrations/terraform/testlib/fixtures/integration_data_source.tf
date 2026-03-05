data "teleport_integration" "test" {
  version  = "v1"
  sub_kind = "aws-oidc"
  metadata = {
    name = "aws-oidc"
  }
  spec = {}
}
