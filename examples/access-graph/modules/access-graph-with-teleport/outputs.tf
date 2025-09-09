output "instructions" {
  description = "Deployment instructions for configured environments"
  value       = <<-EOT

    ========================================
    LOCAL DEPLOYMENT INSTRUCTIONS
    ========================================

    1. Check /etc/hosts for required entries:
       ${var.teleport != null ? coalesce(var.teleport.address, "${var.name}.local") : "${var.name}.local"} localhost

       If missing, append with:
         $ echo "127.0.0.1 ${var.teleport != null ? coalesce(var.teleport.address, "${var.name}.local") : "${var.name}.local"}" | sudo tee -a /etc/hosts

    2. Navigate to output directory:
         $ cd ${var.local_deployment.target_dir}

    3. Start services:
         $ docker compose up -d

    4. Create first Teleport user, follow printed link to complete user setup:
         $ docker compose exec teleport tctl users add --roles=access,editor --logins=root USERNAME
    EOT
}

output "local_config_path" {
  description = "Path to local deployment directory"
  value       = var.local_deployment != null ? var.local_deployment.target_dir : null
}
