resource "teleport_role" "env_access" {
  version = "v7"
  metadata = {
    name        = "${var.env_label}_access"
    description = "Can access infrastructure with label ${var.env_label}"
    labels = {
      env = var.env_label
    }
  }

  spec = {
    allow = {
      aws_role_arns          = lookup(var.principals, "aws_role_arns", [])
      azure_identities       = lookup(var.principals, "azure_identities", [])
      db_names               = lookup(var.principals, "db_names", [])
      db_users               = lookup(var.principals, "db_users", [])
      gcp_service_accounts   = lookup(var.principals, "gcp_service_accounts", [])
      kubernetes_groups      = lookup(var.principals, "kubernetes_groups", [])
      kubernetes_users       = lookup(var.principals, "kubernetes_users", [])
      logins                 = lookup(var.principals, "logins", [])
      windows_desktop_logins = lookup(var.principals, "windows_desktop_logins", [])

      request = {
        roles           = var.request_roles
        search_as_roles = var.request_roles
        thresholds = [{
          approve = 1
          deny    = 1
          filter  = "!equals(request.reason, \"\")"
        }]
      }

      app_labels = {
        env = [var.env_label]
      }

      db_labels = {
        env = [var.env_label]
      }

      node_labels = {
        env = [var.env_label]
      }

      kubernetes_labels = {
        env = [var.env_label]
      }

      windows_desktop_labels = {
        env = [var.env_label]
      }
    }
  }
}

output "role_name" {
  value = teleport_role.env_access.metadata.name
}

