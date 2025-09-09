# Internal certificates for Access Graph and PostgreSQL
# This module generates the certificates needed for secure communication

# Generate CA private key
resource "tls_private_key" "ca" {
  algorithm = "RSA"
  rsa_bits  = 4096
}

# Generate internal CA certificate
resource "tls_self_signed_cert" "internal_ca" {
  private_key_pem = tls_private_key.ca.private_key_pem

  subject {
    common_name = "TestCA"
  }

  validity_period_hours = 24 * 1024 # ~2.8 years

  allowed_uses = [
    "digital_signature",
    "cert_signing",
    "key_encipherment",
  ]

  is_ca_certificate = true
}

# Generate server private key
resource "tls_private_key" "server" {
  algorithm = "RSA"
  rsa_bits  = 2048
}

# Generate server certificate signed by CA
resource "tls_cert_request" "server" {
  private_key_pem = tls_private_key.server.private_key_pem

  subject {
    common_name = "localhost"
  }

  dns_names = [
    "access-graph",
    "localhost"
  ]
}

resource "tls_locally_signed_cert" "server" {
  cert_request_pem   = tls_cert_request.server.cert_request_pem
  ca_private_key_pem = tls_private_key.ca.private_key_pem
  ca_cert_pem        = tls_self_signed_cert.internal_ca.cert_pem

  validity_period_hours = 24 * 365 # 1 year

  allowed_uses = [
    "key_encipherment",
    "data_encipherment",
    "server_auth",
  ]
}

# Generate postgres private key
resource "tls_private_key" "postgres" {
  algorithm = "RSA"
  rsa_bits  = 2048
}

# Generate postgres certificate signed by CA
resource "tls_cert_request" "postgres" {
  private_key_pem = tls_private_key.postgres.private_key_pem

  subject {
    common_name = "localhost"
  }

  dns_names = [
    "access-graph",
    "localhost"
  ]
}

resource "tls_locally_signed_cert" "postgres" {
  cert_request_pem   = tls_cert_request.postgres.cert_request_pem
  ca_private_key_pem = tls_private_key.ca.private_key_pem
  ca_cert_pem        = tls_self_signed_cert.internal_ca.cert_pem

  validity_period_hours = 24 * 365 # 1 year

  allowed_uses = [
    "key_encipherment",
    "data_encipherment",
    "server_auth",
  ]
}

# Write certificates to files
resource "local_file" "ca_cert" {
  content         = tls_self_signed_cert.internal_ca.cert_pem
  filename        = "${var.target_dir}/internal-ca.crt"
  file_permission = "0644"
}

resource "local_file" "server_cert" {
  content         = tls_locally_signed_cert.server.cert_pem
  filename        = "${var.target_dir}/server.crt"
  file_permission = "0644"
}

resource "local_sensitive_file" "server_key" {
  content         = tls_private_key.server.private_key_pem
  filename        = "${var.target_dir}/server.key"
  file_permission = "0600"
}

resource "local_file" "postgres_cert" {
  content         = tls_locally_signed_cert.postgres.cert_pem
  filename        = "${var.target_dir}/postgres.crt"
  file_permission = "0644"
}

resource "local_sensitive_file" "postgres_key" {
  content         = tls_private_key.postgres.private_key_pem
  filename        = "${var.target_dir}/postgres.key"
  file_permission = "0600"
}

# Fix permissions on key files (needed for some systems where file_permission doesn't work)
resource "null_resource" "fix_key_permissions" {
  depends_on = [
    local_sensitive_file.server_key,
    local_sensitive_file.postgres_key
  ]

  provisioner "local-exec" {
    command = <<-EOT
      chmod 600 "${var.target_dir}/server.key" "${var.target_dir}/postgres.key"
    EOT
  }

  triggers = {
    server_key_id   = local_sensitive_file.server_key.id
    postgres_key_id = local_sensitive_file.postgres_key.id
  }
}
