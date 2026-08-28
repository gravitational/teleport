resource "kubernetes_namespace_v1" "agents" {
  metadata {
    name = local.agents_namespace
  }
}

resource "kubernetes_network_policy_v1" "agents" {
  metadata {
    name      = "restrict-instance-metadata"
    namespace = kubernetes_namespace_v1.agents.metadata.0.name
  }

  spec {
    pod_selector {}
    policy_types = ["Egress"]
    egress {
      to {
        ip_block {
          cidr   = "0.0.0.0/0"
          except = ["169.254.169.254/32"]
        }
      }
    }
  }
}
