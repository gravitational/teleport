output "ca_cert" {
  description = "CA certificate in PEM format"
  value       = tls_self_signed_cert.internal_ca.cert_pem
}

output "server_cert" {
  description = "Server certificate in PEM format"
  value       = tls_locally_signed_cert.server.cert_pem
}

output "server_key" {
  description = "Server private key in PEM format"
  value       = tls_private_key.server.private_key_pem
  sensitive   = true
}

output "postgres_cert" {
  description = "PostgreSQL certificate in PEM format"
  value       = tls_locally_signed_cert.postgres.cert_pem
}

output "postgres_key" {
  description = "PostgreSQL private key in PEM format"
  value       = tls_private_key.postgres.private_key_pem
  sensitive   = true
}

output "postgres_csr" {
  description = "PostgreSQL certificate signing request in PEM format"
  value       = tls_cert_request.postgres.cert_request_pem
}

# File path outputs
output "ca_cert_path" {
  description = "Path to CA certificate file"
  value       = local_file.ca_cert.filename
}

output "server_cert_path" {
  description = "Path to server certificate file"
  value       = local_file.server_cert.filename
}

output "server_key_path" {
  description = "Path to server private key file"
  value       = local_sensitive_file.server_key.filename
}

output "postgres_cert_path" {
  description = "Path to PostgreSQL certificate file"
  value       = local_file.postgres_cert.filename
}

output "postgres_key_path" {
  description = "Path to PostgreSQL private key file"
  value       = local_sensitive_file.postgres_key.filename
}
