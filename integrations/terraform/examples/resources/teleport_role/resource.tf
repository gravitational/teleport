# Teleport Role resource

resource "teleport_role" "example2" {
  version = "v8"
  metadata = {
    name        = "example2"
    description = "Example Teleport Role"
    expires     = "2022-10-12T07:20:51Z"
    labels = {
      example2 = "yes"
    }
  }

  spec = {
    options = {
      forward_agent   = false
      max_session_ttl = "7m"
      ssh_port_forwarding = {
        remote = {
          enabled = false
        }

        local = {
          enabled = false
        }
      }
      client_idle_timeout     = "1h"
      disconnect_expired_cert = true
      permit_x11_forwarding   = false
      request_access          = "optional"
    }

    allow = {
      logins = ["example2"]

      rules = [{
        resources = ["user", "role"]
        verbs     = ["list"]
      }]

      request = {
        roles = ["example2"]
        claims_to_roles = [{
          claim = "example2"
          value = "example2"
          roles = ["example2"]
        }]
      }

      node_labels = {
        example2 = ["yes"]
      }
    }

    deny = {
      logins = ["anonymous"]
    }
  }
}
