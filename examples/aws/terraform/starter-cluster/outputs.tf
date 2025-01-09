output "instance_ip_public" {
  description = "Public IP address of the Teleport cluster instance"
  value       = aws_instance.cluster.public_ip
}

output "cluster_name" {
  description = "Configured name for the Teleport cluster"
  value       = var.cluster_name
}

output "cluster_web_address" {
  description = "Web address to access the Teleport cluster"
  value       = "https://${var.use_acm ? aws_route53_record.cluster_acm[0].name : aws_route53_record.cluster[0].fqdn}"
}

output "key_name" {
  description = "Name of the key pair used for SSH access to instances"
  value       = aws_instance.cluster.key_name
}
