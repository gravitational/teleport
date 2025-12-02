resource "teleport_integration" "aws_oidc" {
  version  = "v1"
  sub_kind = "aws-oidc"
  metadata = {
    name        = "example"
    description = "AWS OIDC integration"
    labels = {
      env = "dev"
    }
  }

  spec = {
    aws_oidc = {
      role_arn = "arn:aws:iam::123456789012:role/example-role-name"
    }
  }
}

resource "teleport_integration" "azure_oidc" {
  version  = "v1"
  sub_kind = "azure-oidc"
  metadata = {
    name        = "azure-oidc"
    description = "Example Azure OIDC integration"
    labels = {
      env = "dev"
    }
  }

  spec = {
    azure_oidc = {
      // Azure Entra ID tenant ID
      tenant_id = "a1b2c3d4-f2e4-97a8-9abc-1234567890ab"
      // Azure enterprise application client ID
      client_id = "7f12e3b5-6789-4abc-def0-112233445566"
    }
  }
}

resource "teleport_integration" "aws_roles_anywhere" {
  version  = "v1"
  sub_kind = "aws-ra"
  metadata = {
    name        = "aws-ra"
    description = "Example AWS Roles Anywhere integration"
    labels = {
      env = "dev"
    }
  }

  spec = {
    aws_ra = {
      profile_sync_config = {
        // sync AWS profiles as Teleport applications
        enabled                           = true
        profile_accepts_role_session_name = false
        profile_arn                       = "arn:aws:rolesanywhere:us-east-1:123456789012:profile/<random-uuid>"
        role_arn                          = "arn:aws:iam::123456789012:role/example-role-name"
        // only sync AWS profiles as Teleport applications if the profile name matches
        profile_name_filters = [
          "teleport-*",    // supports globs
          "^teleport-.*$", // and regex if the string is enclosed in regex anchors
        ]
      }
      trust_anchor_arn = "arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/<random-uuid>"
    }
  }
}
