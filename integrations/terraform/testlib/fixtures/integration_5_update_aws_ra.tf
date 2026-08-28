resource "teleport_integration" "aws_ra" {
  version  = "v1"
  sub_kind = "aws-ra"
  metadata = {
    name        = "aws-ra"
    description = "Test integration"
    labels = {
      example = "yes"
    }
  }

  spec = {
    aws_ra = {
      profile_sync_config = {
        enabled                           = false
        profile_accepts_role_session_name = false
        profile_arn                       = "arn:aws:rolesanywhere:us-east-1:123456789012:profile/updated"
        profile_name_filters              = ["*updated*", "^.*$"]
        role_arn                          = "arn:aws:iam::123456789012:role/updated"
      }
      trust_anchor_arn = "arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/updated"
    }
  }
}
