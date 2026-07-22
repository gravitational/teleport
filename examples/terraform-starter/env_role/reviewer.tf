locals {
  can_review_roles = join(", ", var.request_roles)
}

resource "teleport_role" "env_access_reviewer" {
  version = "v7"
  count   = length(var.request_roles) > 0 ? 1 : 0
  metadata = {
    name        = "${local.can_review_roles}_reviewer"
    description = "Can review Access Requests for: ${local.can_review_roles}"
  }

  spec = {
    allow = {
      review_requests = {
        roles = var.request_roles
      }
    }
  }
}

output "reviewer_role_names" {
  value = teleport_role.env_access_reviewer[*].metadata.name
}
