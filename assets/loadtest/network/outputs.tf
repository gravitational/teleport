output "proxy_ip" {
  description = "The static proxy ip address"
  value       = google_compute_address.proxy_ip.address
}