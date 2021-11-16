output "cluster_name" {
    description = "The gke cluster name"
    value       = google_container_cluster.loadtest.name
}