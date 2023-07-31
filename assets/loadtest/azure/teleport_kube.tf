locals {
  teleport_release   = "teleport"
  teleport_namespace = "teleport"

  agents_namespace = "agents"
}

resource "kubernetes_namespace_v1" "teleport" {
  metadata {
    name = local.teleport_namespace
    labels = {
      "pod-security.kubernetes.io/enforce" = "baseline"
    }
  }
}

resource "kubernetes_network_policy_v1" "teleport" {
  metadata {
    name      = "restrict-instance-metadata"
    namespace = kubernetes_namespace_v1.teleport.metadata.0.name
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

resource "kubernetes_namespace_v1" "agents" {
  metadata {
    name = local.agents_namespace
  }
}

resource "kubernetes_network_policy_v1" "agents" {
  metadata {
    name      = "restrict-instance-metadata"
    namespace = kubernetes_namespace_v1.agents.metadata[0].name
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

resource "helm_release" "teleport" {
  count = var.deploy_teleport ? 1 : 0

  name = local.teleport_release

  chart      = "teleport-cluster"
  repository = "https://charts.releases.teleport.dev"
  version    = var.teleport_version

  namespace = kubernetes_namespace_v1.teleport.metadata[0].name

  values = [jsonencode({
    "clusterName" = "${var.cluster_prefix}.${data.azurerm_dns_zone.dns_zone.name}"

    "chartMode" = "azure"
    "azure" = {
      "databaseHost"                   = azurerm_postgresql_flexible_server.pgbk.fqdn,
      "databaseUser"                   = azurerm_postgresql_flexible_server_active_directory_administrator.pgbk_teleport.principal_name
      "sessionRecordingStorageAccount" = azurerm_storage_account.azsessions.primary_blob_host
      "clientID"                       = azurerm_user_assigned_identity.teleport_identity.client_id
      "databasePoolMaxConnections"     = 50
    }

    "log" = {
      "format" = "json"
      "level"  = "DEBUG"
    }
    "extraArgs" = ["--debug"]

    "proxyListenerMode" = "multiplex"

    "proxy" = {
      "annotations" = {
        "service" = {
          "service.beta.kubernetes.io/azure-pip-name"                     = azurerm_public_ip.proxy.name
          "service.beta.kubernetes.io/azure-load-balancer-resource-group" = azurerm_public_ip.proxy.resource_group_name
        }
      }
      "service" = {
        "spec" = {
          "externalTrafficPolicy" = "Local"
        }
      }
    }

    "auth" = {
      "teleportConfig" = {
        "kubernetes_service" = {
          "enabled" = false
        }
      }
    }

    "resources" = {
      "limits" = {
        "memory" = "8Gi"
      }
      "requests" = {
        "cpu"    = "4"
        "memory" = "8Gi"
      }
    }

    "authentication" = {
      "secondFactor" = "off"
    }

    "highAvailability" = {
      "replicaCount" = 3
      "certManager" = {
        "enabled"    = true
        "issuerName" = kubectl_manifest.letsencrypt_production.name
        "issuerKind" = kubectl_manifest.letsencrypt_production.kind
      }
    }

    "podSecurityPolicy" = {
      "enabled" = false
    }
    "podMonitor" = {
      "enabled" = true
    }
  })]

  depends_on = [
    # block IDMS
    kubernetes_network_policy_v1.teleport,
    # can use teleport_identity
    azurerm_federated_identity_credential.teleport_identity,
    # postgres is fully configured
    azurerm_postgresql_flexible_server_configuration.pgbk,
    # teleport_identity can use blob storage
    azurerm_role_assignment.teleport_identity,
    # PodMonitor CRD
    helm_release.monitoring,
  ]
}

resource "azurerm_public_ip" "proxy" {
  name                = "teleport-proxy"
  resource_group_name = azurerm_resource_group.rg.name
  location            = azurerm_resource_group.rg.location

  allocation_method = "Static"
  sku               = "Standard"
}

resource "azurerm_role_assignment" "proxy_ip" {
  scope                = azurerm_public_ip.proxy.id
  role_definition_name = "Network Contributor"
  principal_id         = azurerm_kubernetes_cluster.kube_cluster.identity[0].principal_id

  skip_service_principal_aad_check = true
}

resource "azurerm_dns_a_record" "proxy" {
  name                = var.cluster_prefix
  zone_name           = data.azurerm_dns_zone.dns_zone.name
  resource_group_name = data.azurerm_dns_zone.dns_zone.resource_group_name

  ttl                = 300
  target_resource_id = azurerm_public_ip.proxy.id
}

resource "azurerm_dns_a_record" "proxy_wild" {
  name                = "*.${var.cluster_prefix}"
  zone_name           = data.azurerm_dns_zone.dns_zone.name
  resource_group_name = data.azurerm_dns_zone.dns_zone.resource_group_name

  ttl                = 300
  target_resource_id = azurerm_public_ip.proxy.id
}
