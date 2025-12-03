# Certificate Generation Modules

# Generate mkcert certificates for local Teleport deployment
module "mkcert" {
  count  = var.local_deployment != null ? 1 : 0
  source = "../certs/mkcert"

  address    = local.local_teleport_address
  target_dir = "${var.local_deployment.target_dir}/teleport/certs"
}

# For local deployment: generate internal certificates directly to target directory
module "internal_certs_local" {
  count  = var.local_deployment != null ? 1 : 0
  source = "../certs/internal-certs"

  target_dir = "${var.local_deployment.target_dir}/access-graph/certs"
}

# Certificate Paths for Local Deployment
locals {
  # Only the CA certificate path is needed for copying to teleport/certs
  access_graph_ca_source = var.local_deployment != null ? module.internal_certs_local[0].ca_cert_path : ""
}
