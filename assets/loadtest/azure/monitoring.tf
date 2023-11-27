resource "helm_release" "monitoring" {
  name = local.monitoring_release

  chart      = "kube-prometheus-stack"
  repository = "https://prometheus-community.github.io/helm-charts"

  namespace        = local.monitoring_namespace
  create_namespace = true
  wait             = true

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
}
