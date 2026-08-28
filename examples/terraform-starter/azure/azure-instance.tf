locals {
  username = "admin_temp"
}

resource "azurerm_network_interface" "teleport_agent" {
  count               = length(var.userdata_scripts)
  name                = "teleport-agent-ni-${count.index}"
  location            = var.region
  resource_group_name = var.azure_resource_group
  ip_configuration {
    name                          = "teleport_agent_ip_config"
    subnet_id                     = var.subnet_id
    private_ip_address_allocation = "Dynamic"
    public_ip_address_id          = var.insecure_direct_access ? azurerm_public_ip.agent[count.index].id : ""
  }
}

resource "azurerm_public_ip" "agent" {
  count               = var.insecure_direct_access ? length(var.userdata_scripts) : 0
  name                = "agentIP-${count.index}"
  resource_group_name = var.azure_resource_group
  location            = var.region
  allocation_method   = "Static"
}

resource "azurerm_virtual_machine" "teleport_agent" {
  count = length(var.userdata_scripts)
  name  = "teleport-agent-${count.index}"

  location            = var.region
  resource_group_name = var.azure_resource_group

  network_interface_ids = [azurerm_network_interface.teleport_agent[count.index].id]
  os_profile_linux_config {
    disable_password_authentication = true
    ssh_keys {
      key_data = file(var.public_key_path)
      // The only allowed path. See:
      // https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/virtual_machine
      path = "/home/${local.username}/.ssh/authorized_keys"
    }
  }

  os_profile {
    computer_name  = "teleport-agent-${count.index}"
    admin_username = local.username
    custom_data    = var.userdata_scripts[count.index]
  }

  vm_size = "Standard_B2s"

  storage_os_disk {
    name              = "teleport-agent-disk-${count.index}"
    create_option     = "FromImage"
    managed_disk_type = "Standard_LRS"
  }

  storage_image_reference {
    publisher = "Canonical"
    offer     = "0001-com-ubuntu-server-jammy"
    sku       = "22_04-lts"
    version   = "latest"
  }
}
