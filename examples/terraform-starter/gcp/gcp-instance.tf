locals {
  // Google Cloud provides public IP addresses to instances when the
  // network_interface block includes an empty access_config, so use a dynamic
  // block to enable a public IP based on the insecure_direct_access input.
  access_configs = var.insecure_direct_access ? [{}] : []
}

resource "google_compute_instance" "teleport_agent" {
  count = length(var.userdata_scripts)
  name  = "teleport-agent-${count.index}"
  zone  = var.gcp_zone

  // Initialize the instance tags to an empty map to prevent errors when the
  // Teleport SSH Service fetches them.
  params {
    resource_manager_tags = {}
  }

  boot_disk {
    initialize_params {
      image = "family/ubuntu-2204-lts"
    }
  }

  network_interface {
    subnetwork = var.subnet_id

    // If the user enables insecure direct access, allocate a public IP to the
    // instance.
    dynamic "access_config" {
      for_each = local.access_configs
      content {}
    }
  }

  machine_type = "e2-standard-2"

  metadata_startup_script = var.userdata_scripts[count.index]
}
