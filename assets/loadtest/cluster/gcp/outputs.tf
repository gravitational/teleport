output "cluster_name" {
  description = "The gke cluster name"
  value       = google_container_cluster.loadtest.name
}

output "project" {
  description = "The project that the cluster was created in"
  value       = var.project
}