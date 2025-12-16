# Terraform module for creating Teleport Access Lists with preset configurations
# This module creates:
# - Reviewer role (reviewer-acl-preset-{name}) - allows reviewing access requests
# - Requester role (requester-acl-preset-{name}) - allows requesting access
# - Access list with appropriate grants based on preset type
#

locals {
  role_suffix = "acl-preset-${var.access_list_name}"

  access_role_names = var.access_roles

  reviewer_role_name  = "reviewer-${local.role_suffix}"
  requester_role_name = "requester-${local.role_suffix}"

  preset_label = {
    "teleport.internal/access-list-preset" = var.preset_type
  }

  membership_kind_map = {
    "user" = 1
    "list" = 2
  }
}

# Create reviewer role that allows reviewing access requests
resource "teleport_role" "reviewer" {
  version = "v7"

  metadata = {
    name        = local.reviewer_role_name
    description = "Role created by Teleport. Do not edit."
    labels      = local.preset_label
  }

  spec = {
    allow = {
      review_requests = {
        preview_as_roles = local.access_role_names
        roles            = local.access_role_names
      }
    }
  }

}

# Create requester role that allows requesting access
resource "teleport_role" "requester" {
  version = "v7"

  metadata = {
    name        = local.requester_role_name
    description = "Role created by Teleport. Do not edit."
    labels      = local.preset_label
  }

  spec = {
    allow = {
      request = {
        search_as_roles = local.access_role_names
      }
    }
  }
}

# Create the access list with grants based on preset type
resource "teleport_access_list" "main" {
  header = {
    version = "v1"
    metadata = {
      name        = var.access_list_name
      description = var.access_list_description
      labels      = local.preset_label
    }
  }

  spec = merge(
    {
      title       = var.access_list_title
      description = var.access_list_description

      # Grants configuration based on preset type:
      # - long-term: Members get access roles directly, owners get reviewer role
      # - short-term: Members get requester role, owners get reviewer role
      grants = {
        roles = var.preset_type == "long-term" ? local.access_role_names : [local.requester_role_name]
      }

      owner_grants = {
        roles = [local.reviewer_role_name]
      }
      owners = var.owners
    },
    var.audit != null ? {
      audit = var.audit
    } : {},
    var.membership_requires != null ? {
      membership_requires = var.membership_requires
    } : {},
    var.ownership_requires != null ? {
      ownership_requires = var.ownership_requires
    } : {}
  )

  depends_on = [
    teleport_role.reviewer,
    teleport_role.requester
  ]
}

resource "teleport_access_list_member" "members" {
  count = length(var.members)

  header = {
    version = "v1"
    metadata = {
      name    = var.members[count.index].name
      expires = var.members[count.index].expires
    }
  }

  spec = merge(
    {
      access_list = teleport_access_list.main.id
      membership_kind = local.membership_kind_map[var.members[count.index].membership_kind]
    },
    var.members[count.index].joined != null ? { joined = var.members[count.index].joined } : {},
    var.members[count.index].expires != null ? { expires = var.members[count.index].expires } : {},
    var.members[count.index].reason != null ? { reason = var.members[count.index].reason } : {},
    var.members[count.index].added_by != null ? { added_by = var.members[count.index].added_by } : {}
  )

  depends_on = [teleport_access_list.main]
}
