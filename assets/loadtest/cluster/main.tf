provider "google" {
  project     = var.project
  region      = var.region
  zone        = var.zone
}

terraform {
  required_version = ">= 0.12"
}


data "google_compute_network" "default" {
  name = var.network
}


resource "google_container_cluster" "loadtest" {
  name     = var.cluster_name
  location = var.region

  # clusters are always created with a default node pool,
  # so specify the smallest allowed node count and then
  # immediately delete it.
  remove_default_node_pool = true
  initial_node_count       = 1
}

resource "google_container_node_pool" "loadtest" {
  name     = var.cluster_name
  cluster  = google_container_cluster.loadtest.name
  location = google_container_cluster.loadtest.location

  node_count = var.nodes_per_zone

  node_locations = var.node_locations

  node_config {
    machine_type = "n1-standard-8"

    metadata = {
      disable-legacy-endpoints = "true"
    }

    oauth_scopes = [
      "https://www.googleapis.com/auth/logging.write",
      "https://www.googleapis.com/auth/monitoring",
    ]
  }
}
