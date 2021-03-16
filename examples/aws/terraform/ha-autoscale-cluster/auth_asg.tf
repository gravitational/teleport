// Autoscaling group for Teleport Authentication servers.
// Auth servers are most privileged in terms of IAM roles
// as they are allowed to publish to SSM parameter store,
// write certificates to encrypted S3 bucket.
resource "aws_autoscaling_group" "auth" {
  name                      = "${var.cluster_name}-auth"
  max_size                  = 5
  min_size                  = length(var.az_list)
  health_check_grace_period = 300
  health_check_type         = "EC2"
  desired_capacity          = length(var.az_list)
  force_delete              = false
  launch_configuration      = aws_launch_configuration.auth.name
  vpc_zone_identifier       = [for subnet in aws_subnet.auth : subnet.id]

  // These are target groups of the auth server network load balancer
  // this autoscaling group is associated with target groups of the NLB
  target_group_arns = [aws_lb_target_group.auth.arn]

  tag {
    key                 = "TeleportCluster"
    value               = var.cluster_name
    propagate_at_launch = true
  }

  tag {
    key                 = "TeleportRole"
    value               = "auth"
    propagate_at_launch = true
  }

  // external autoscale algos can modify these values,
  // so ignore changes to them
  lifecycle {
    ignore_changes = [
      desired_capacity,
      max_size,
      min_size,
    ]
  }
}

resource "aws_launch_configuration" "auth" {
  lifecycle {
    create_before_destroy = true
  }
  name_prefix                 = "${var.cluster_name}-auth-"
  image_id                    = data.aws_ami.base.id
  instance_type               = var.auth_instance_type
  user_data                   = templatefile(
    "${path.module}/auth-user-data.tpl",
    {
      region                   = data.aws_region.current.name
      locks_table_name         = aws_dynamodb_table.locks.name
      auth_type                = var.auth_type
      auth_server_addr         = aws_lb.auth.dns_name
      cluster_name             = var.cluster_name
      dynamo_table_name        = aws_dynamodb_table.teleport.name
      dynamo_events_table_name = aws_dynamodb_table.teleport_events.name
      email                    = var.email
      domain_name              = var.route53_domain
      s3_bucket                = var.s3_bucket_name
      influxdb_addr            = "http://${aws_lb.monitor.dns_name}:8086"
      license_path             = var.license_path
      telegraf_version         = var.telegraf_version
      use_acm                  = var.use_acm
    }
  )
  key_name                    = var.key_name
  ebs_optimized               = true
  associate_public_ip_address = false
  security_groups             = [aws_security_group.auth.id]
  iam_instance_profile        = aws_iam_instance_profile.auth.id
}

