terraform {
  required_version = ">= 1.0"

  required_providers {
    teleport = {
      source  = "terraform.releases.teleport.dev/gravitational/teleport"
      version = "~> 18.0"
    }
  }
}

locals {
  proxy_public_addr = "teleport.example.com:443"
}

provider "teleport" {
  addr = local.proxy_public_addr
}

# Example: Short-term access list for production server access
# Members get a requester role for on-demand access, owners review requests
module "production_server_access" {
  source = "github.com/gravitational/teleport//examples/terraform/access-list-preset"

  access_list_name        = "production-server-access"
  access_list_title       = "Production Server Access"
  access_list_description = "Request-based access to production servers"

  # short-term: Members get requester role to request access
  preset_type = "short-term"

  # Pass existing role names (these roles must already exist in Teleport)
  access_roles = ["web-server-access", "app-server-access"]

  audit = {
    recurrence = {
      frequency    = "1month"
      day_of_month = 1
    }
  }

  owners = [
    {
      name        = "devops-team"
    }
  ]

  members = [
    {
      name   = "charlie@example.com"
    },
    {
      name   = "diana@example.com"
    }
  ]
}

output "access_list" {
  value = {
    name         = module.production_server_access.access_list_name
  }
}
