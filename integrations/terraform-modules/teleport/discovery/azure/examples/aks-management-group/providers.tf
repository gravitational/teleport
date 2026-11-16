# Both configurations use the same Azure authentication credentials.
# The default subscription hosts discovery and the public Kubernetes Service.
provider "azurerm" {
  subscription_id = local.deployment_subscription_id
  features {}
}

provider "azurerm" {
  alias           = "private"
  subscription_id = local.private_subscription_id
  features {}
}

provider "teleport" {
  addr         = var.teleport_proxy_addr
  profile_name = replace(var.teleport_proxy_addr, "/:[0-9]+.*/", "")
}
