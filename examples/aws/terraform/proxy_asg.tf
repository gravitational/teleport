// Proxy auto scaling group is for Teleport proxies
// set up in the public subnet. This is the only group of servers that are
// accepting traffic from the internet.

// letsencrypt
resource "aws_autoscaling_group" "proxy" {
  name                      = "${var.cluster_name}-proxy"
  max_size                  = 5
  min_size                  = "${length(local.azs)}"
  health_check_grace_period = 300
  health_check_type         = "EC2"
  desired_capacity          = "${length(local.azs)}"
  force_delete              = false
  launch_configuration      = "${aws_launch_configuration.proxy.name}"
  vpc_zone_identifier       = ["${aws_subnet.public.*.id}"]
  // Auto scaling group is associated with load balancer
  target_group_arns         = ["${aws_lb_target_group.proxy_proxy.arn}", "${aws_lb_target_group.proxy_web.arn}"]
  count                     = "${var.use_acm ? 0 : 1}"

  tag {
    key =  "TeleportCluster"
    value = "${var.cluster_name}"
    propagate_at_launch = true
  }

  tag {
    key =  "TeleportRole"
    value = "proxy"
    propagate_at_launch = true
  }

  // external autoscale algos can modify these values,
  // so ignore changes to them
  lifecycle {
    ignore_changes = ["desired_capacity", "max_size", "min_size"]
  }
}

// ACM
resource "aws_autoscaling_group" "proxy_acm" {
  name                      = "${var.cluster_name}-proxy"
  max_size                  = 5
  min_size                  = "${length(local.azs)}"
  health_check_grace_period = 300
  health_check_type         = "EC2"
  desired_capacity          = "${length(local.azs)}"
  force_delete              = false
  launch_configuration      = "${aws_launch_configuration.proxy.name}"
  vpc_zone_identifier       = ["${aws_subnet.public.*.id}"]
  // Auto scaling group is associated with load balancer
  target_group_arns         = ["${aws_lb_target_group.proxy_proxy.arn}", "${aws_lb_target_group.proxy_tunnel_acm.arn}", "${aws_lb_target_group.proxy_web_acm.arn}"]
  count                     = "${var.use_acm ? 1 : 0}"

  tag {
    key =  "TeleportCluster"
    value = "${var.cluster_name}"
    propagate_at_launch = true
  }

  tag {
    key =  "TeleportRole"
    value = "proxy"
    propagate_at_launch = true
  }

  // external autoscale algos can modify these values,
  // so ignore changes to them
  lifecycle {
    ignore_changes = ["desired_capacity", "max_size", "min_size"]
  }
}

data "template_file" "proxy_user_data" {
  template = "${file("proxy-user-data.tpl")}"

  vars {
    region = "${var.region}"
    cluster_name = "${var.cluster_name}"
    auth_server_addr = "${aws_lb.auth.dns_name}"
    proxy_server_lb_addr = "${aws_lb.proxy.dns_name}"
    influxdb_addr = "http://${aws_lb.monitor.dns_name}:8086"
    email = "${var.email}"
    domain_name = "${var.route53_domain}"
    s3_bucket = "${var.s3_bucket_name}"
    telegraf_version = "${var.telegraf_version}"
    teleport_uid = "${var.teleport_uid}"
    use_acm = "${var.use_acm}"
  }
}

resource "aws_launch_configuration" "proxy" {
  lifecycle {
    create_before_destroy = true
  }
  name_prefix                 = "${var.cluster_name}-proxy-"
  image_id                    = "${data.aws_ami.base.id}"
  instance_type               = "${var.proxy_instance_type}"
  user_data                   = "${data.template_file.proxy_user_data.rendered}"
  key_name                    = "${var.key_name}"
  ebs_optimized               = true
  associate_public_ip_address = true
  security_groups             = ["${aws_security_group.proxy.id}"]
  iam_instance_profile        = "${aws_iam_instance_profile.proxy.id}"
}
