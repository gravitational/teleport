# mkcert module - generates local certificates using mkcert

# Check if mkcert is installed
resource "null_resource" "check_mkcert" {
  provisioner "local-exec" {
    command = <<-EOT
      if ! command -v mkcert &> /dev/null; then
        echo "Error: mkcert is not installed. Please install it first:"
        echo "  macOS: brew install mkcert"
        echo "  Linux: see https://github.com/FiloSottile/mkcert#installation"
        exit 1
      fi
    EOT
  }

  # Force re-run if address changes
  triggers = {
    address = var.address
  }
}

# Install mkcert root CA
resource "null_resource" "mkcert_install" {
  depends_on = [null_resource.check_mkcert]

  provisioner "local-exec" {
    command = "mkcert -install > /dev/null 2>&1 || true"
  }

  # Only run once per system
  triggers = {
    always_run = timestamp()
  }
}

# Generate certificates
resource "null_resource" "generate_certs" {
  depends_on = [null_resource.mkcert_install]

  provisioner "local-exec" {
    command = <<-EOT
      mkdir -p "${var.target_dir}"
      mkcert -cert-file "${var.target_dir}/teleport.pem" \
             -key-file "${var.target_dir}/teleport-key.pem" \
             "${var.address}" localhost 127.0.0.1 ::1
    EOT
  }

  # Re-run if address or target_dir changes
  triggers = {
    address    = var.address
    target_dir = var.target_dir
  }
}

# Get mkcert CA root directory
data "external" "mkcert_caroot" {
  depends_on = [null_resource.mkcert_install]

  program = ["bash", "-c", "echo '{\"caroot\":\"'$(mkcert -CAROOT)'\"}'"]
}

# Copy root CA certificate
resource "null_resource" "copy_root_ca" {
  depends_on = [null_resource.generate_certs, data.external.mkcert_caroot]

  provisioner "local-exec" {
    command = <<-EOT
      cp "${data.external.mkcert_caroot.result.caroot}/rootCA.pem" "${var.target_dir}/rootCA.pem"
    EOT
  }

  triggers = {
    target_dir = var.target_dir
    caroot     = data.external.mkcert_caroot.result.caroot
  }
}

# Read generated files to ensure they exist
data "local_file" "cert" {
  depends_on = [null_resource.generate_certs]
  filename   = "${var.target_dir}/teleport.pem"
}

data "local_file" "key" {
  depends_on = [null_resource.generate_certs]
  filename   = "${var.target_dir}/teleport-key.pem"
}

data "local_file" "root_ca" {
  depends_on = [null_resource.copy_root_ca]
  filename   = "${var.target_dir}/rootCA.pem"
}
