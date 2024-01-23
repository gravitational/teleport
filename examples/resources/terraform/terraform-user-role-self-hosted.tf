terraform {
  required_providers {
    teleport = {
      source  = "terraform.releases.teleport.dev/gravitational/teleport"
      version = "~> (=teleport.major_version=).0"
    }
  }
}

provider "teleport" {
  # Update addr to point to Teleport Auth/Proxy
  # addr              = "auth.example.com:3025"
  addr               = "proxy.example.com:443"
  identity_file_path = "terraform-identity"
}

resource "teleport_role" "terraform-test" {
  version = "v7"
  metadata = {
    name        = "terraform-test"
    description = "Terraform test role"
    labels = {
      example = "yes"
    }
  }

  spec = {
    options = {
      forward_agent           = false
      max_session_ttl         = "30m"
      port_forwarding         = false
      client_idle_timeout     = "1h"
      disconnect_expired_cert = true
      permit_x11_forwarding   = false
      request_access          = "denied"
    }

    allow = {
      logins = ["this-user-does-not-exist"]

      rules = [
        {
          resources = ["user", "role"]
          verbs     = ["list"]
        }
      ]

      request = {
        roles = ["example"]
        claims_to_roles = [
          {
            claim = "example"
            value = "example"
            roles = ["example"]
          }
        ]
      }

      node_labels = {
        key    = ["example"]
        alabel = ["with", "multiple", "values"]
      }
    }

    deny = {
      logins = ["anonymous"]
    }
  }
}

resource "teleport_user" "terraform-test" {
  version = "v2"
  metadata = {
    name        = "terraform-test"
    description = "Test terraform user"
    expires     = "2022-10-12T07:20:50Z"

    labels = {
      test = "true"
    }
  }

  spec = {
    roles = ["terraform-test"]
  }
}
