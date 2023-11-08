locals {
  username = "admin_temp"
}

resource "azurerm_network_interface" "teleport_agent" {
  count               = var.cloud == "azure" ? var.agent_count : 0
  name                = "teleport-agent-ni-${count.index}"
  location            = var.region
  resource_group_name = var.azure_resource_group
  ip_configuration {
    name                          = "teleport_agent_ip_config"
    subnet_id                     = var.subnet_id
    private_ip_address_allocation = "Dynamic"
  }
}

resource "azurerm_virtual_machine" "teleport_agent" {
  count = var.cloud == "azure" ? var.agent_count : 0
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
    custom_data = templatefile("./userdata", {
      token                 = teleport_provision_token.agent[count.index].metadata.name
      proxy_service_address = var.proxy_service_address
      teleport_version      = var.teleport_version
    })
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
