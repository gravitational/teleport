resource "teleport_integration" "aws_oidc" {
  version  = "v1"
  sub_kind = "aws-oidc"
  metadata = {
    name        = "aws-oidc"
    description = "Test integration"
    labels = {
      example = "yes"
    }
  }

  spec = {
    aws_oidc = {
      audience = "aws-identity-center"
      role_arn = "arn:aws:iam::123456789012:role/test-role-updated"
    }
  }
}
