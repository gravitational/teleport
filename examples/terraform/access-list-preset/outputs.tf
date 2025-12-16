output "access_list_name" {
  description = "Name of the created access list"
  value       = teleport_access_list.main.header.metadata.name
}

output "access_list_id" {
  description = "ID of the created access list"
  value       = teleport_access_list.main.id
}

output "access_role_names" {
  description = "Names of the created access roles"
  value       = local.access_role_names
}

output "reviewer_role_name" {
  description = "Name of the reviewer role"
  value       = local.reviewer_role_name
}

output "requester_role_name" {
  description = "Name of the requester role"
  value       = local.requester_role_name
}

output "preset_type" {
  description = "The preset type used for this access list"
  value       = var.preset_type
}

output "grants" {
  description = "Roles granted to members based on preset type"
  value = {
    member_roles = var.preset_type == "long-term" ? local.access_role_names : [local.requester_role_name]
    owner_roles  = [local.reviewer_role_name]
  }
}
