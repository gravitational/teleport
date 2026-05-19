resource "kubernetes_namespace_v1" "teleport" {
  metadata {
    name = local.teleport_namespace
    labels = {
      "pod-security.kubernetes.io/enforce" = "baseline"
    }
  }
}

resource "helm_release" "teleport" {
  count = var.deploy_teleport ? 1 : 0

  name = local.teleport_release

  chart      = "teleport-cluster"
  repository = "https://charts.releases.development.teleport.dev"
  version    = var.teleport_version

  namespace = kubernetes_namespace_v1.teleport.metadata.0.name

  values = [jsonencode({
    "clusterName" = "${var.cluster_prefix}.${data.azurerm_dns_zone.dns_zone.name}"

    "chartMode" = "azure"
    "azure" = {
      "databaseHost"                   = azurerm_postgresql_flexible_server.pgbk.fqdn,
      "databaseUser"                   = azurerm_postgresql_flexible_server_active_directory_administrator.pgbk_teleport.principal_name
      "sessionRecordingStorageAccount" = azurerm_storage_account.azsessions.primary_blob_host
      "clientID"                       = azurerm_user_assigned_identity.teleport_identity.client_id
      "databasePoolMaxConnections"     = 100
    }

    "log" = {
      "format" = "json"
      "level"  = "DEBUG"
    }
    "extraArgs"       = ["--debug"]
    "image"           = "public.ecr.aws/gravitational-staging/teleport-distroless-debug"
    "enterpriseImage" = "public.ecr.aws/gravitational-staging/teleport-ent-distroless-debug"

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
      "secondFactor" = "webauthn"
    }

    "highAvailability" = {
      "replicaCount" = 3
      "certManager" = {
        "enabled"    = true
        "issuerName" = kubectl_manifest.issuer.name
        "issuerKind" = kubectl_manifest.issuer.kind
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
    # can use teleport_identity
    azurerm_federated_identity_credential.teleport_identity,
    # postgres has replication enabled, we can connect to it, and we can log in
    # with teleport_identity
    azurerm_postgresql_flexible_server_configuration.pgbk_wal_level,
    azurerm_postgresql_flexible_server_firewall_rule.pgbk,
    azurerm_postgresql_flexible_server_active_directory_administrator.pgbk_teleport,
    # teleport_identity can use blob storage
    azurerm_role_assignment.azsessions,
    # the PodMonitor CRD is available
    helm_release.monitoring,
    # the public ip is usable by the proxy service
    azurerm_role_assignment.proxy_ip,
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
  principal_id         = azurerm_kubernetes_cluster.kube_cluster.identity.0.principal_id

  skip_service_principal_aad_check = true
}
