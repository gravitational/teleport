// Autoscaling group for Teleport Authentication servers.
// Auth servers are most privileged in terms of IAM roles
// as they are allowed to publish to SSM parameter store,
// write certificates to encrypted S3 bucket.
resource "aws_autoscaling_group" "auth" {
  name                      = "${var.cluster_name}-auth"
  max_size                  = 5
  min_size                  = "${length(local.azs)}"
  health_check_grace_period = 300
  health_check_type         = "EC2"
  desired_capacity          = "${length(local.azs)}"
  force_delete              = false
  launch_configuration      = "${aws_launch_configuration.auth.name}"
  vpc_zone_identifier       = ["${aws_subnet.auth.*.id}"]
  // These are target groups of the auth server network load balancer
  // this autoscaling group is associated with target groups of the NLB
  target_group_arns    = ["${aws_lb_target_group.auth.arn}"]

  tag {
    key =  "TeleportCluster"
    value = "${var.cluster_name}"
    propagate_at_launch = true
  }

  tag {
    key =  "TeleportRole"
    value = "auth"
    propagate_at_launch = true
  }

  // external autoscale algos can modify these values,
  // so ignore changes to them
  lifecycle {
    ignore_changes = ["desired_capacity", "max_size", "min_size"]
  }
}

data "template_file" "auth_user_data" {
  template = "${file("auth-user-data.tpl")}"

  vars {
    region = "${var.region}"
    locks_table_name = "${aws_dynamodb_table.locks.name}"
    cluster_name = "${var.cluster_name}"
    efs_mount_point = "${aws_efs_file_system.auth.id}.efs.${var.region}.amazonaws.com"
    teleport_version = "${var.teleport_version}"
    dynamo_table_name = "${aws_dynamodb_table.teleport.name}"
    email = "${var.email}"
    domain_name = "${var.route53_domain}"
    s3_bucket = "${var.s3_bucket_name}"
    influxdb_addr = "http://${aws_lb.monitor.dns_name}:8086"
    telegraf_version = "${var.telegraf_version}"
    teleport_uid = "${var.teleport_uid}"
  }
}

resource "aws_launch_configuration" "auth" {
  lifecycle {
    create_before_destroy = true
  }
  name_prefix                 = "${var.cluster_name}-auth-"
  image_id                    = "${data.aws_ami.base.id}"
  instance_type               = "${var.auth_instance_type}"
  user_data                   = "${data.template_file.auth_user_data.rendered}"
  key_name                    = "${var.key_name}"
  ebs_optimized               = true
  associate_public_ip_address = false
  security_groups             = ["${aws_security_group.auth.id}"]
  iam_instance_profile        = "${aws_iam_instance_profile.auth.id}"
}
