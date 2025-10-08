output "cert_path" {
  description = "Path to the generated certificate file"
  value       = "${var.target_dir}/teleport.pem"
  depends_on  = [null_resource.generate_certs]
}

output "key_path" {
  description = "Path to the generated key file"
  value       = "${var.target_dir}/teleport-key.pem"
  depends_on  = [null_resource.generate_certs]
}

output "root_ca_path" {
  description = "Path to the root CA certificate"
  value       = "${var.target_dir}/rootCA.pem"
  depends_on  = [null_resource.copy_root_ca]
}

output "cert_content" {
  description = "Content of the generated certificate"
  value       = data.local_file.cert.content
  sensitive   = true
}

output "key_content" {
  description = "Content of the generated key"
  value       = data.local_file.key.content
  sensitive   = true
}

output "root_ca_content" {
  description = "Content of the root CA certificate"
  value       = data.local_file.root_ca.content
  sensitive   = true
}
