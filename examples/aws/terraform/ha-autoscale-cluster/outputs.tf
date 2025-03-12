output "bastion_ip_public" {
  description = "Public IP address of the SSH bastion server for accessing auth/proxy/node instances"
  value       = aws_instance.bastion[0].public_ip
}

data "aws_instances" "auth_servers" {
  instance_tags = {
    TeleportCluster = var.cluster_name
    TeleportRole    = "auth"
  }

  depends_on = [aws_autoscaling_group.auth]
}

output "auth_instance_private_ips" {
  description = "Private IPs of Teleport cluster auth servers"
  value       = data.aws_instances.auth_servers.private_ips
}

data "aws_instances" "proxy_servers" {
  instance_tags = {
    TeleportCluster = var.cluster_name
    TeleportRole    = "proxy"
  }

  depends_on = [aws_autoscaling_group.proxy, aws_autoscaling_group.proxy_acm]
}

output "proxy_instance_private_ips" {
  description = "Private IPs of Teleport cluster proxy servers"
  value       = data.aws_instances.proxy_servers.private_ips
}

output "cluster_name" {
  description = "Configured name for the Teleport cluster"
  value       = var.cluster_name
}

output "cluster_web_address" {
  description = "Web address to access the Teleport cluster"
  value       = "https://${var.use_acm ? aws_route53_record.proxy_acm[0].name : aws_route53_record.proxy[0].fqdn}"
}

output "key_name" {
  description = "Name of the key pair used for SSH access to instances"
  value       = aws_instance.bastion[0].key_name
}
