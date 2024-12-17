// Node auto scaling group supports multiple
// teleport nodes joining the cluster,
// Setup for demo/testing purposes.
resource "aws_autoscaling_group" "node" {
  name                      = "${var.cluster_name}-node"
  max_size                  = 1000
  min_size                  = 1
  health_check_grace_period = 300
  health_check_type         = "EC2"
  desired_capacity          = 1
  force_delete              = false
  vpc_zone_identifier       = aws_subnet.node.*.id

  launch_template {
    id      = aws_launch_template.node.id
    version = aws_launch_template.node.latest_version
  }

  tag {
    key                 = "TeleportCluster"
    value               = var.cluster_name
    propagate_at_launch = true
  }

  tag {
    key                 = "TeleportRole"
    value               = "node"
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
    for_each = var.enable_node_asg_instance_refresh ? [1] : []
    content {
      strategy = "Rolling"
      preferences {
        auto_rollback          = true
        min_healthy_percentage = 50
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

resource "aws_launch_template" "node" {
  lifecycle {
    create_before_destroy = true
  }

  name_prefix   = "${var.cluster_name}-node-"
  image_id      = data.aws_ami.base.id
  instance_type = var.node_instance_type
  user_data = base64encode(templatefile(
    "${path.module}/node-user-data.tpl",
    {
      region           = var.region
      cluster_name     = var.cluster_name
      auth_server_addr = aws_lb.auth.dns_name
      use_acm          = var.use_acm
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

  key_name = var.key_name

  network_interfaces {
    associate_public_ip_address = false
    security_groups             = [aws_security_group.node.id]
  }

  iam_instance_profile {
    name = aws_iam_instance_profile.node.name
  }

  dynamic "tag_specifications" {
    for_each = ["instance", "volume", "network-interface"]
    content {
      resource_type = tag_specifications.value
      tags          = data.aws_default_tags.this.tags
    }
  }
}
