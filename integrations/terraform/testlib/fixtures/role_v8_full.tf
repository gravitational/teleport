# Teleport Role resource with v8 features

resource "teleport_role" "v8_example" {
  version = "v8"
  metadata = {
    name        = "v8-example"
    description = "Example v8 Role with api_group"
    labels = {
      example = "v8"
    }
  }

  spec = {
    options = {
      forward_agent           = false
      max_session_ttl         = "30h"
      client_idle_timeout     = "1h"
      disconnect_expired_cert = true
      permit_x11_forwarding   = false
      request_access          = "optional"
    }

    allow = {
      logins = ["root", "ubuntu"]

      kubernetes_labels = {
        "*" = ["*"]
      }

      # v8 requires api_group for kubernetes_resources
      kubernetes_resources = [
        {
          kind      = "pods"
          api_group = ""
          name      = "*"
          namespace = "*"
          verbs     = ["*"]
        },
        {
          kind      = "deployments"
          api_group = "apps"
          name      = "*"
          namespace = "production"
          verbs     = ["get", "list", "watch"]
        },
        {
          kind      = "clusterroles"
          api_group = "rbac.authorization.k8s.io"
          name      = "*"
          namespace = ""
          verbs     = ["get", "list"]
        }
      ]

      node_labels = {
        env = ["prod", "staging"]
      }
    }

    deny = {
      logins = ["guest"]
    }
  }
}

