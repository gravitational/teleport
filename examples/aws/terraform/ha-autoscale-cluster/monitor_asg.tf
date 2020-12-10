// Monitor is an example of influxdb + grafana deployment
// Grafana is available on port 8443
// Internal influxdb HTTP collector service listens on port 8086

// letsencrypt
resource "aws_autoscaling_group" "monitor" {
  name                      = "${var.cluster_name}-monitor"
  max_size                  = 1
  min_size                  = 1
  health_check_grace_period = 300
  health_check_type         = "EC2"
  desired_capacity          = 1
  force_delete              = false
  launch_configuration      = aws_launch_configuration.monitor.name
  vpc_zone_identifier       = [local.public_subnet_ids[0]]

  // Auto scaling group is associated with internal load balancer for metrics ingestion
  // and proxy load balancer for grafana
  target_group_arns = [aws_lb_target_group.proxy_grafana[0].arn, aws_lb_target_group.monitor.arn]
  count             = var.use_acm ? 0 : 1

  tag {
    key                 = "TeleportCluster"
    value               = var.cluster_name
    propagate_at_launch = true
  }

  tag {
    key                 = "TeleportRole"
    value               = "monitor"
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
resource "aws_autoscaling_group" "monitor_acm" {
  name                      = "${var.cluster_name}-monitor"
  max_size                  = 1
  min_size                  = 1
  health_check_grace_period = 300
  health_check_type         = "EC2"
  desired_capacity          = 1
  force_delete              = false
  launch_configuration      = aws_launch_configuration.monitor.name
  vpc_zone_identifier       = [local.public_subnet_ids[0]]

  // Auto scaling group is associated with internal load balancer for metrics ingestion
  // and proxy load balancer for grafana
  target_group_arns = [aws_lb_target_group.proxy_grafana_acm[0].arn, aws_lb_target_group.monitor.arn]
  count             = var.use_acm ? 1 : 0

  tag {
    key                 = "TeleportCluster"
    value               = var.cluster_name
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

resource "aws_launch_configuration" "monitor" {
  lifecycle {
    create_before_destroy = true
  }
  name_prefix                 = "${var.cluster_name}-monitor-"
  image_id                    = var.ami_id
  instance_type               = var.monitor_instance_type
  user_data                   = templatefile(
    "${path.module}/monitor-user-data.tpl",
    {
      region           = data.aws_region.current.name
      cluster_name     = var.cluster_name
      influxdb_version = var.influxdb_version
      grafana_version  = var.grafana_version
      telegraf_version = var.telegraf_version
      s3_bucket        = var.s3_bucket_name
      domain_name      = var.route53_domain
      use_acm          = var.use_acm
    }
  )
  key_name                    = var.key_name
  ebs_optimized               = true
  associate_public_ip_address = true
  security_groups             = [aws_security_group.monitor.id]
  iam_instance_profile        = aws_iam_instance_profile.monitor.id
}

// Monitors support traffic comming from internal cluster subnets and expose 8443 for grafana
resource "aws_security_group" "monitor" {
  name   = "${var.cluster_name}-monitor"
  vpc_id = local.vpc_id
  tags = {
    TeleportCluster = var.cluster_name
  }
}

// SSH access via bastion only
resource "aws_security_group_rule" "monitor_ingress_allow_ssh" {
  type                     = "ingress"
  from_port                = 22
  to_port                  = 22
  protocol                 = "tcp"
  security_group_id        = aws_security_group.monitor.id
  source_security_group_id = aws_security_group.bastion.id
}

// Ingress traffic to SSL port 8443 is allowed from everywhere (letsencrypt)
resource "aws_security_group_rule" "monitor_ingress_allow_web" {
  type              = "ingress"
  from_port         = 8443
  to_port           = 8443
  protocol          = "tcp"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.monitor.id
  count             = var.use_acm ? 0 : 1
}

// Ingress traffic to non-SSL port 8444 is allowed from everywhere (ACM)
resource "aws_security_group_rule" "monitor_ingress_allow_web_acm" {
  type              = "ingress"
  from_port         = 8444
  to_port           = 8444
  protocol          = "tcp"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.monitor.id
  count             = var.use_acm ? 1 : 0
}

// Influxdb metrics collector traffic is limited to internal VPC CIDR
// We use CIDR here because traffic arriving from NLB is not marked with security group
resource "aws_security_group_rule" "monitor_collector_ingress_allow_vpc_cidr_traffic" {
  type              = "ingress"
  from_port         = 8086
  to_port           = 8086
  protocol          = "tcp"
  cidr_blocks       = [var.vpc_cidr]
  security_group_id = aws_security_group.monitor.id
}

// All egress traffic is allowed
resource "aws_security_group_rule" "monitor_egress_allow_all_traffic" {
  type              = "egress"
  from_port         = 0
  to_port           = 0
  protocol          = "-1"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.monitor.id
}

// Network load balancer for influxdb collector
// Notice that in this case it is in the single subnet
// because network load balancers only distriute traffic
// in the same AZ, and this example does not have HA InfluxDB setup
resource "aws_lb" "monitor" {
  name               = "${var.cluster_name}-monitor"
  internal           = true
  subnets            = [local.public_subnet_ids[0]]
  load_balancer_type = "network"
  idle_timeout       = 3600

  tags = {
    TeleportCluster = var.cluster_name
  }
}

// Target group is associated with monitor instance
resource "aws_lb_target_group" "monitor" {
  name     = "${var.cluster_name}-monitor"
  port     = 8086
  vpc_id   = aws_vpc.teleport.id
  protocol = "TCP"
}

// 8086 is monitor metrics collector
resource "aws_lb_listener" "monitor" {
  load_balancer_arn = aws_lb.monitor.arn
  port              = "8086"
  protocol          = "TCP"

  default_action {
    target_group_arn = aws_lb_target_group.monitor.arn
    type             = "forward"
  }
}

