data "azurerm_dns_zone" "dns_zone" {
  name                = var.dns_zone
  resource_group_name = var.dns_zone_rg
}

resource "azurerm_user_assigned_identity" "dns_identity" {
  name                = "${azurerm_resource_group.rg.name}-dns"
  resource_group_name = azurerm_resource_group.rg.name
  location            = azurerm_resource_group.rg.location
}

resource "azurerm_role_assignment" "dns_identity" {
  scope                = data.azurerm_dns_zone.dns_zone.id
  role_definition_name = "DNS Zone Contributor"
  principal_id         = azurerm_user_assigned_identity.dns_identity.principal_id

  skip_service_principal_aad_check = true
}

resource "azurerm_federated_identity_credential" "dns_identity" {
  name = "${azurerm_user_assigned_identity.dns_identity.name}-${azurerm_kubernetes_cluster.kube_cluster.name}"

  parent_id           = azurerm_user_assigned_identity.dns_identity.id
  resource_group_name = azurerm_user_assigned_identity.dns_identity.resource_group_name

  audience = ["api://AzureADTokenExchange"]

  issuer  = azurerm_kubernetes_cluster.kube_cluster.oidc_issuer_url
  subject = "system:serviceaccount:${local.certmanager_namespace}:${local.certmanager_release}"
}

resource "helm_release" "certmanager" {
  name = local.certmanager_release

  chart      = "cert-manager"
  repository = "https://charts.jetstack.io"

  namespace        = local.certmanager_namespace
  create_namespace = true
  wait             = true

  values = [jsonencode({
    "installCRDs" = true
    "podLabels" = {
      "azure.workload.identity/use" : "true"
    }
  })]
}

resource "kubectl_manifest" "issuer" {
  yaml_body = jsonencode({
    "apiVersion" = "cert-manager.io/v1"
    "kind"       = "ClusterIssuer"
    "metadata" = {
      "name" = local.clusterissuer
    }
    "spec" = {
      "acme" = {
        "server" = "https://acme-v02.api.letsencrypt.org/directory"
        "privateKeySecretRef" = {
          "name" = local.clusterissuer
        }
        "solvers" = [{
          "selector" = {
            "dnsZones" = [data.azurerm_dns_zone.dns_zone.name]
          }
          "dns01" = { "azureDNS" = {
            "subscriptionID"    = data.azurerm_client_config.current.subscription_id
            "resourceGroupName" = data.azurerm_dns_zone.dns_zone.resource_group_name
            "hostedZoneName"    = data.azurerm_dns_zone.dns_zone.name
            "managedIdentity" = {
              "clientID" = azurerm_user_assigned_identity.dns_identity.client_id
            }
          } }
        }]
      }
    }
  })

  depends_on = [
    # ClusterIssuer CRD
    helm_release.certmanager,
    # can use dns_identity
    azurerm_federated_identity_credential.dns_identity,
    # dns_identity can use DNS zone
    azurerm_role_assignment.dns_identity,
  ]
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
