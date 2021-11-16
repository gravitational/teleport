provider "google" {
  project     = var.project
  region      = var.region
  zone        = var.zone
}

resource "google_compute_address" "proxy_ip" {
  name         = "proxy-ip"
  address_type = "EXTERNAL"
  network_tier = "PREMIUM"
}

resource "google_compute_address" "grafana_ip" {
  name         = "grafana-ip"
  address_type = "EXTERNAL"
  network_tier = "PREMIUM"
}
