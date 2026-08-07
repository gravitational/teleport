resource "kubernetes_namespace_v1" "monitoring" {
  metadata {
    name = local.monitoring_namespace
  }
}

resource "kubernetes_config_map_v1" "monitoring_teleport_dashboard" {
  metadata {
    name = "teleport-dashboard"
    labels = {
      "grafana_dashboard" = "1"
    }
    namespace = one(kubernetes_namespace_v1.monitoring.metadata).name
  }

  binary_data = {
    "teleport-dashboard.json" = filebase64("${path.module}/../../../examples/grafana/teleport-dashboard.json")
  }
}

resource "helm_release" "monitoring" {
  name = local.monitoring_release

  reset_values = true
  max_history  = 10

  chart      = "kube-prometheus-stack"
  repository = "https://prometheus-community.github.io/helm-charts"

  namespace = one(kubernetes_namespace_v1.monitoring.metadata).name
  wait      = true

  values = [jsonencode({
    "grafana" = {
      "grafana.ini" = {
        "auth.anonymous" = {
          "enabled"  = true
          "org_name" = "Main Org."
          "org_role" = "Admin"
        }
      }
    }
    "prometheus" = {
      "prometheusSpec" = {
        "enableAdminAPI" = true
        "scrapeInterval" = "15s"
        "retention"      = "30d"
        "resources" = {
          "requests" = {
            "memory" = "16Gi"
            "cpu"    = "4"
          }
          "limits" = {
            "memory" = "16Gi"
          }
        }
        "storageSpec" = {
          "volumeClaimTemplate" = {
            "spec" = {
              "accessModes" = ["ReadWriteOnce"]
              "resources" = {
                requests = {
                  "storage" = "50Gi"
                }
              }
            }
          }
        }
        "podMonitorSelectorNilUsesHelmValues"     = false
        "serviceMonitorSelectorNilUsesHelmValues" = false
      }
    }
  })]

  depends_on = [
    kubernetes_config_map_v1.monitoring_teleport_dashboard,
  ]
}
