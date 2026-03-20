terraform {
  required_providers {
    azurerm = {
      source = "hashicorp/azurerm"
    }
    helm = {
      source = "hashicorp/helm"
    }
    kubectl = {
      source = "alekc/kubectl"
    }
    kubernetes = {
      source = "hashicorp/kubernetes"
    }
    random = {
      source = "hashicorp/random"
    }
  }
}

provider "kubernetes" {
  host                   = azurerm_kubernetes_cluster.kube_cluster.kube_config.0.host
  client_certificate     = base64decode(azurerm_kubernetes_cluster.kube_cluster.kube_config.0.client_certificate)
  client_key             = base64decode(azurerm_kubernetes_cluster.kube_cluster.kube_config.0.client_key)
  cluster_ca_certificate = base64decode(azurerm_kubernetes_cluster.kube_cluster.kube_config.0.cluster_ca_certificate)
}

provider "helm" {
  kubernetes = {
    host                   = azurerm_kubernetes_cluster.kube_cluster.kube_config.0.host
    client_certificate     = base64decode(azurerm_kubernetes_cluster.kube_cluster.kube_config.0.client_certificate)
    client_key             = base64decode(azurerm_kubernetes_cluster.kube_cluster.kube_config.0.client_key)
    cluster_ca_certificate = base64decode(azurerm_kubernetes_cluster.kube_cluster.kube_config.0.cluster_ca_certificate)
  }
}

provider "kubectl" {
  host                   = azurerm_kubernetes_cluster.kube_cluster.kube_config.0.host
  client_certificate     = base64decode(azurerm_kubernetes_cluster.kube_cluster.kube_config.0.client_certificate)
  client_key             = base64decode(azurerm_kubernetes_cluster.kube_cluster.kube_config.0.client_key)
  cluster_ca_certificate = base64decode(azurerm_kubernetes_cluster.kube_cluster.kube_config.0.cluster_ca_certificate)
  load_config_file       = false
}

provider "azurerm" {
  subscription_id = var.subscription_id
  features {}
}

data "azurerm_client_config" "current" {}

locals {
  name_prefix       = "loadtest"
  short_name_prefix = "lt"

  certmanager_release   = "cert-manager"
  certmanager_namespace = "cert-manager"
  clusterissuer         = "letsencrypt-production"

  monitoring_release   = "monitoring"
  monitoring_namespace = "monitoring"

  teleport_release   = "teleport"
  teleport_namespace = "teleport"

  agents_namespace = "agents"
}
