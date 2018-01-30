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
  launch_configuration      = "${aws_launch_configuration.node.name}"
  vpc_zone_identifier       = ["${aws_subnet.node.*.id}"]

  tag {
    key =  "TeleportCluster"
    value = "${var.cluster_name}"
    propagate_at_launch = true
  }

  tag {
    key =  "TeleportRole"
    value = "node"
    propagate_at_launch = true
  }

  // external autoscale algos can modify these values,
  // so ignore changes to them
  lifecycle {
    ignore_changes = ["desired_capacity", "max_size", "min_size"]
  }
}

data "template_file" "node_user_data" {
  template = "${file("node-user-data.tpl")}"

  vars {
    region = "${var.region}"
    cluster_name = "${var.cluster_name}"
    teleport_version = "${var.teleport_version}"
    telegraf_version = "${var.telegraf_version}"
    auth_server_addr = "${aws_lb.auth.dns_name}:3025"
    influxdb_addr = "http://${aws_lb.monitor.dns_name}:8086"
  }
}

resource "aws_launch_configuration" "node" {
  lifecycle {
    create_before_destroy = true
  }
  name_prefix                 = "${var.cluster_name}-node-"
  image_id                    = "${data.aws_ami.base.id}"
  instance_type               = "${var.node_instance_type}"
  user_data                   = "${data.template_file.node_user_data.rendered}"
  key_name                    = "${var.key_name}"
  associate_public_ip_address = false
  security_groups             = ["${aws_security_group.node.id}"]
  iam_instance_profile        = "${aws_iam_instance_profile.node.id}"
}
