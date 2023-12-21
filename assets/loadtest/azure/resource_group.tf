resource "random_pet" "rg" {
  prefix = local.name_prefix
}

resource "azurerm_resource_group" "rg" {
  name     = random_pet.rg.id
  location = var.location
}
