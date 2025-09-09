module "access_graph_with_teleport" {
  source = "../modules/access-graph-with-teleport"

  # Name for the deployment (used as prefix for resources and nodename)
  name = "root-sample"

  # Local Deployment Configuration
  local_deployment = {
    target_dir = "out" # Directory where config files will be written
  }

  # Access Graph Configuration
  access_graph = {
    # Docker image for Access Graph
    # image = "public.ecr.aws/gravitational/access-graph:1.28.1"
  }

  # Teleport Configuration
  teleport = {
    # Path to Teleport license file (required)
    license_pem_path = "/tmp/license.pem"

    # Docker image for Teleport Enterprise
    image = "public.ecr.aws/gravitational/teleport-ent-distroless-debug:18.1.6"

    # Optional: Custom address for Teleport
    # For local: defaults to ${name}.local (e.g., root-sample.local)
    # address = "custom.local"

    # Optional: Disable SSH service with busybox shell
    # Default: true (SSH service enabled)
    # enable_ssh_service = false
  }
}

output "instructions" {
  value = module.access_graph_with_teleport.instructions
}


terraform {
  required_version = ">= 1.12.1"
}
