// Autoscaling group for Teleport Authentication servers.
// Auth servers are most privileged in terms of IAM roles
// as they are allowed to publish to SSM parameter store,
// write certificates to encrypted S3 bucket.
resource "aws_autoscaling_group" "auth" {
  name                      = "${var.cluster_name}-auth"
  max_size                  = 5
  min_size                  = length(local.azs)
  health_check_grace_period = 300
  health_check_type         = "EC2"
  desired_capacity          = length(local.azs)
  force_delete              = false
  vpc_zone_identifier       = aws_subnet.auth.*.id

  launch_template {
    name    = aws_launch_template.auth.name
    version = aws_launch_template.auth.latest_version
  }

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

  dynamic "tag" {
    for_each = data.aws_default_tags.this.tags
    content {
      key                 = tag.key
      value               = tag.value
      propagate_at_launch = true
    }
  }

  dynamic "instance_refresh" {
    for_each = var.enable_auth_asg_instance_refresh ? [1] : []
    content {
      strategy = "Rolling"
      preferences {
        auto_rollback          = false
        min_healthy_percentage = 0
      }
    }
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

resource "aws_launch_template" "auth" {
  lifecycle {
    create_before_destroy = true
  }
  name_prefix   = "${var.cluster_name}-auth-"
  image_id      = data.aws_ami.base.id
  instance_type = var.auth_instance_type
  user_data = base64encode(templatefile(
    "${path.module}/auth-user-data.tpl",
    {
      region                   = var.region
      locks_table_name         = aws_dynamodb_table.locks.name
      auth_server_addr         = aws_lb.auth.dns_name
      teleport_auth_type       = var.teleport_auth_type
      cluster_name             = var.cluster_name
      dynamo_table_name        = aws_dynamodb_table.teleport.name
      dynamo_events_table_name = aws_dynamodb_table.teleport_events.name
      email                    = var.email
      domain_name              = var.route53_domain
      s3_bucket                = var.s3_bucket_name
      license_path             = var.license_path
      teleport_uid             = var.teleport_uid
      use_acm                  = var.use_acm
      use_tls_routing          = var.use_tls_routing
    }
  ))

  metadata_options {
    http_tokens   = "required"
    http_endpoint = "enabled"

  }

  block_device_mappings {
    device_name = "/dev/xvda"
    ebs {
      delete_on_termination = true
      encrypted             = true
      iops                  = 3000
      throughput            = 125
      volume_type           = "gp3"
    }
  }

  key_name      = var.key_name
  ebs_optimized = true

  network_interfaces {
    associate_public_ip_address = false
    security_groups             = [aws_security_group.auth.id]
  }

  iam_instance_profile {
    name = aws_iam_instance_profile.auth.name
  }

  dynamic "tag_specifications" {
    for_each = ["instance", "volume", "network-interface"]
    content {
      resource_type = tag_specifications.value
      tags          = data.aws_default_tags.this.tags
    }
  }
}
