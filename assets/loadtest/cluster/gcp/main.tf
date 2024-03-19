provider "google" {
  project = var.project
  region  = var.region
  zone    = var.zone
}

terraform {
  required_version = ">= 0.12"
}


#trivy:ignore:AVD-GCP-0047
#trivy:ignore:AVD-GCP-0049
#trivy:ignore:AVD-GCP-0051
#trivy:ignore:AVD-GCP-0056
#trivy:ignore:AVD-GCP-0059
#trivy:ignore:AVD-GCP-0061
resource "google_container_cluster" "loadtest" {
  name     = var.cluster_name
  location = var.region

  # clusters are always created with a default node pool,
  # so specify the smallest allowed node count and then
  # immediately delete it.
  remove_default_node_pool = true
  initial_node_count       = 1
}

#trivy:ignore:AVD-GCP-0048
#trivy:ignore:AVD-GCP-0049
#trivy:ignore:AVD-GCP-0050
#trivy:ignore:AVD-GCP-0054
#trivy:ignore:AVD-GCP-0057
#trivy:ignore:AVD-GCP-0058
#trivy:ignore:AVD-GCP-0063
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
      "https://www.googleapis.com/auth/devstorage.read_only",
    ]
  }
}
