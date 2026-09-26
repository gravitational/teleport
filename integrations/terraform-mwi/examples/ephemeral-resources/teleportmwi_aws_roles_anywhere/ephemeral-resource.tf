// https://goteleport.com/docs/machine-workload-identity/workload-identity/aws-roles-anywhere/
// Configure the Workload Identity and AWS Roles Anywhere trust relationship
// before using this resource. The identity that the teleportmwi provider assumes must have permission to use the Workload Identity resource. 
ephemeral "teleportmwi_aws_roles_anywhere" "aws_role" {
  selector = {
    name = "my-workload-identity"
  }
  role_arn         = "arn:aws:iam::123456789012:role/my-role"
  profile_arn      = "arn:aws:rolesanywhere:us-east-1:123456789012:profile/00000000-0000-0000-0000-000000000000"
  trust_anchor_arn = "arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/00000000-0000-0000-0000-000000000000"

  // Defaults to 1 hour. Credentials must remain valid for the entire operation;
  // AWS profile and IAM role settings also constrain the session duration.
  session_duration = "1h"
}

// https://registry.terraform.io/providers/hashicorp/aws/latest/docs
provider "aws" {
  region     = "us-west-2"
  access_key = ephemeral.teleportmwi_aws_roles_anywhere.aws_role.output.access_key
  secret_key = ephemeral.teleportmwi_aws_roles_anywhere.aws_role.output.secret_key
  token      = ephemeral.teleportmwi_aws_roles_anywhere.aws_role.output.token
}

data "aws_caller_identity" "current" {}
