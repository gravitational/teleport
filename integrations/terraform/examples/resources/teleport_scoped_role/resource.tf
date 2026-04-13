# Teleport Scoped Role resource

resource "teleport_scoped_role" "example" {
  version = "v1"
  metadata = {
    name        = "example-scoped-role"
    description = "An example scoped role for managing resources within /staging"

    labels = {
      env = "staging"
    }
  }

  scope = "/staging"

  spec = {
    assignable_scopes = ["/staging/aa", "/staging/bb"]
    rules = [{
      resources = ["scoped_token"]
      verbs     = ["read", "list"]
    }]
    ssh = {
      logins                = ["root", "ubuntu"]
      client_idle_timeout   = "30m"
      forward_agent         = true
      permit_x11_forwarding = true
      file_copy             = true
      max_sessions          = 10
      host_sudoers          = ["ALL=(ALL) NOPASSWD: ALL"]
      labels = [{
        name   = "env"
        values = ["staging", "dev"]
      }]
      host_user_creation = {
        mode   = "keep"
        shell  = "/bin/bash"
        groups = ["sudo", "docker"]
      }
      port_forwarding = {
        local = {
          enabled = true
        }
        remote = {
          enabled = true
        }
      }
    }
  }
}
