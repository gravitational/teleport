// Proxy auto scaling group is for Teleport proxies
// set up in the public subnet. This is the only group of servers that are
// accepting traffic from the internet.

// Let's Encrypt
resource "aws_autoscaling_group" "proxy" {
  name                      = "${var.cluster_name}-proxy"
  max_size                  = 5
  min_size                  = length(local.azs)
  health_check_grace_period = 300
  health_check_type         = "EC2"
  desired_capacity          = length(local.azs)
  force_delete              = false
  vpc_zone_identifier       = aws_subnet.public.*.id

  launch_template {
    name    = aws_launch_template.proxy.name
    version = "$Latest"
  }

  // Auto scaling group is associated with load balancer
  // When TLS routing is enabled, we only expose the web TLS listener
  target_group_arns = var.use_tls_routing ? [aws_lb_target_group.proxy_web[0].arn] : [
    aws_lb_target_group.proxy_proxy[0].arn,
    aws_lb_target_group.proxy_tunnel[0].arn,
    aws_lb_target_group.proxy_web[0].arn,
    aws_lb_target_group.proxy_kube[0].arn,
    aws_lb_target_group.proxy_mysql.arn,
    aws_lb_target_group.proxy_postgres.arn,
    aws_lb_target_group.proxy_mongodb.arn,
  ]

  count = var.use_acm ? 0 : 1

  tag {
    key                 = "TeleportCluster"
    value               = var.cluster_name
    propagate_at_launch = true
  }

  tag {
    key                 = "TeleportRole"
    value               = "proxy"
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

// ACM
resource "aws_autoscaling_group" "proxy_acm" {
  name                      = "${var.cluster_name}-proxy"
  max_size                  = 5
  min_size                  = length(local.azs)
  health_check_grace_period = 300
  health_check_type         = "EC2"
  desired_capacity          = length(local.azs)
  force_delete              = false
  vpc_zone_identifier       = aws_subnet.public.*.id

  launch_template {
    name    = aws_launch_template.proxy.name
    version = "$Latest"
  }

  // Auto scaling group is associated with load balancer
  // When TLS routing is enabled, we only expose the web TLS listener
  target_group_arns = var.use_tls_routing ? [aws_lb_target_group.proxy_web_acm[0].arn] : [
    aws_lb_target_group.proxy_proxy[0].arn,
    aws_lb_target_group.proxy_tunnel[0].arn,
    aws_lb_target_group.proxy_web_acm[0].arn,
    aws_lb_target_group.proxy_kube[0].arn,
    aws_lb_target_group.proxy_mysql.arn,
    aws_lb_target_group.proxy_postgres.arn,
    aws_lb_target_group.proxy_mongodb.arn,
  ]

  count = var.use_acm ? 1 : 0

  tag {
    key                 = "TeleportCluster"
    value               = var.cluster_name
    propagate_at_launch = true
  }

  tag {
    key                 = "TeleportRole"
    value               = "proxy"
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

// Needs to have a public IP
// tfsec:ignore:aws-ec2-no-public-ip
resource "aws_launch_template" "proxy" {
  lifecycle {
    create_before_destroy = true
  }

  name_prefix   = "${var.cluster_name}-proxy-"
  image_id      = data.aws_ami.base.id
  instance_type = var.proxy_instance_type
  user_data = base64encode(templatefile(
    "${path.module}/proxy-user-data.tpl",
    {
      region                   = data.aws_region.current.name
      cluster_name             = var.cluster_name
      auth_server_addr         = aws_lb.auth.dns_name
      proxy_server_lb_addr     = var.use_acm ? aws_lb.proxy_acm[0].dns_name : aws_lb.proxy[0].dns_name
      proxy_server_nlb_alias   = var.route53_domain_acm_nlb_alias
      email                    = var.email
      domain_name              = var.route53_domain
      s3_bucket                = var.s3_bucket_name
      enable_mongodb_listener  = var.enable_mongodb_listener
      enable_mysql_listener    = var.enable_mysql_listener
      enable_postgres_listener = var.enable_postgres_listener
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
    associate_public_ip_address = true
    security_groups             = [aws_security_group.proxy.id]
  }

  iam_instance_profile {
    name = aws_iam_instance_profile.proxy.id
  }
}
