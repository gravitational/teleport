resource "terraform_data" "userdata" {
  count = var.agent_count
  input = {
    userdata = templatefile("${path.module}/userdata", {
      extra_labels          = yamlencode(var.agent_labels)
      proxy_service_address = var.proxy_service_address
      teleport_edition      = var.teleport_edition
      teleport_version      = var.teleport_version
      token                 = teleport_provision_token.agent[count.index].metadata.name
    })
  }
}

output "userdata_scripts" {
  value       = terraform_data.userdata[*].output.userdata
  description = "User data script to run on agent instances."
}
