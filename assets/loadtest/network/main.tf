provider "google" {
  project     = var.project
  region      = var.region
  zone        = var.zone
}

terraform {
  required_version = ">= 0.12"
  backend "gcs" {
    bucket  = "loadtest-tf-state"
    prefix  = "terraform/state"
  }
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
