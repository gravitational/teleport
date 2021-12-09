output "proxy_ip" {
  description = "The static proxy ip address"
  value       = google_compute_address.proxy_ip.address
}

output "grafana_ip" {
  description = "The static grafana ip address"
  value       = google_compute_address.grafana_ip.address
}