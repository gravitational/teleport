# Validate that access_graph and teleport are not null
resource "null_resource" "validate_required_vars" {
  lifecycle {
    precondition {
      condition     = var.access_graph != null
      error_message = "The 'access_graph' variable must be provided and cannot be null."
    }
    precondition {
      condition     = var.teleport != null
      error_message = "The 'teleport' variable must be provided and cannot be null."
    }
    precondition {
      condition     = var.teleport.license_pem_path != ""
      error_message = "The 'teleport.license_pem_path' must not be empty. Please provide a valid path to the Teleport license file."
    }
    precondition {
      condition     = var.local_deployment != null
      error_message = "Local deployment must be configured."
    }
  }
}

# Validate that SSH-enabled Teleport uses a debug image with busybox
resource "null_resource" "validate_teleport_ssh_image" {
  lifecycle {
    precondition {
      condition     = !var.teleport.enable_ssh_service || can(regex("-debug:", var.teleport.image))
      error_message = "SSH service requires a debug Teleport image with busybox. Use an image with '-debug' suffix (e.g., 'public.ecr.aws/gravitational/teleport-ent-distroless-debug:18.1.6'). Current image: ${var.teleport.image}"
    }
  }
}
