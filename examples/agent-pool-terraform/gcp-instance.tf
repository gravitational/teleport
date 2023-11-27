resource "google_compute_instance" "teleport_agent" {
  count = var.cloud == "gcp" ? var.agent_count : 0
  name  = "teleport-agent-${count.index}"
  zone  = var.gcp_zone

  boot_disk {
    initialize_params {
      image = "family/ubuntu-2204-lts"
    }
  }

  network_interface {
    subnetwork = var.subnet_id
  }

  machine_type = "e2-standard-2"

  metadata_startup_script = templatefile("./userdata", {
    token                 = teleport_provision_token.agent[count.index].metadata.name
    proxy_service_address = var.proxy_service_address
    teleport_version      = var.teleport_version
  })
}
