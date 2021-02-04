//Output providing the bastion public IP, auto scaling group names and how 
//to get the private IPs to jumphost through the bastion. 



output "z_bastion_instance_public_ip" {
  value = {
  for instance in aws_instance.bastion:
  instance.id => format("ip %s %s %s %s",instance.public_ip, ", Add identity file via ssh agent (ssh-add identityfile.pem)\n and you can jump host to servers. \nEx:\nssh -J ec2-user@", instance.public_ip, " ec2-user@<proxy/auth/node ip>") 
}
  description = "AWS Bastion EC2 IP. Add AWS identity file to your ssh agent and you can jump host through the bastion.  ssh -J <bastion-ip> <proxy/auth ip>"
}

output "proxy_autoscaling_groups" {
  value = aws_autoscaling_group.proxy.*.id
}
output "proxy_acm_autoscaling_groups" {
  value = aws_autoscaling_group.proxy_acm.*.id
}
output "auth_autoscaling_groups" {
  value = aws_autoscaling_group.auth.*.id
}
output "node_autoscaling_groups" {
  value = aws_autoscaling_group.node.*.id
}

output "monitor_autoscaling_groups" {
  value = aws_autoscaling_group.monitor.*.id
}
output "retrieving_autoscaling_group_ips" {
  value = format("To get the private IPs of the autoscaling ec2 instances set the ASG_VALUE to a above scaling group.\nEx:\nASG_VALUE=teleportoss-auth\naws ec2 describe-instances     --filters \"Name=tag-value,Values=%s\" --region %s  --query 'Reservations[*].Instances[*].[InstanceId,PrivateIpAddress]' --output text", "$${ASG_VALUE}", var.region)

}


