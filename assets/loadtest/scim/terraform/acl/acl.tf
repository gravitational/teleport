terraform {
  required_providers {
    teleport = {
      source  = "terraform.releases.teleport.dev/gravitational/teleport"
      version = "~> 18.0"
    }
  }
}

provider "teleport" {
  addr = var.proxy_addr
}

locals {
  acl_spec = {
    type = "scim"
    grants = {
      roles  = ["requester"]
      traits = []
    }
    owners              = [{ name = var.acl_owner }]
    membership_requires = { roles = [] }
    ownership_requires  = { roles = [] }
    audit = {
      recurrence = {
        frequency    = 3
        day_of_month = 15
      }
    }
  }
}

# Load-test groups: k6-group-0 .. k6-group-<acl_count-1>.
resource "teleport_access_list" "k6_group" {
  count = var.acl_count

  header = {
    version  = "v1"
    metadata = { name = "k6-group-${count.index}" }
  }
  spec = merge(local.acl_spec, { title = "k6-group-${count.index}" })
}
