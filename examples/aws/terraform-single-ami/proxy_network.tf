// Proxy is deployed in public subnet to receive
// traffic from Network load balancers.
resource "aws_security_group" "proxy" {
  name   = "${var.cluster_name}-proxy"
  vpc_id = "${local.vpc_id}"
  tags {
    TeleportCluster = "${var.cluster_name}"
  }
}

// SSH emergency access via bastion only
resource "aws_security_group_rule" "proxy_ingress_allow_ssh" {
  type              = "ingress"
  from_port         = 22
  to_port           = 22
  protocol          = "tcp"
  security_group_id = "${aws_security_group.proxy.id}"
  source_security_group_id = "${aws_security_group.bastion.id}"
}

// Ingress proxy traffic is allowed from all ports
resource "aws_security_group_rule" "proxy_ingress_allow_proxy" {
  type              = "ingress"
  from_port         = 3023
  to_port           = 3023
  protocol          = "tcp"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = "${aws_security_group.proxy.id}"
}

// Ingress traffic to web port 3080 is allowed from all directions
resource "aws_security_group_rule" "proxy_ingress_allow_web" {
  type              = "ingress"
  from_port         = 3080
  to_port           = 3080
  protocol          = "tcp"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = "${aws_security_group.proxy.id}"
}

// Egress traffic is allowed everywhere
resource "aws_security_group_rule" "proxy_egress_allow_all_traffic" {
  type              = "egress"
  from_port         = 0
  to_port           = 0
  protocol          = "-1"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = "${aws_security_group.proxy.id}"
}

// Load balancer for proxy server
resource "aws_lb" "proxy" {
  name            = "${var.cluster_name}-proxy"
  internal        = false
  subnets         = ["${aws_subnet.public.*.id}"]
  load_balancer_type = "network"
  idle_timeout    = 3600

  tags {
    TeleportCluster = "${var.cluster_name}"
  }
}

// Proxy is for SSH proxy - jumphost target endpoint.
resource "aws_lb_target_group" "proxy_proxy" {
  name     = "${var.cluster_name}-proxy-proxy"
  port     = 3023
  vpc_id   = "${aws_vpc.teleport.id}"
  protocol = "TCP"
}

resource "aws_lb_listener" "proxy_proxy" {
  load_balancer_arn = "${aws_lb.proxy.arn}"
  port              = "3023"
  protocol = "TCP"

  default_action {
    target_group_arn = "${aws_lb_target_group.proxy_proxy.arn}"
    type             = "forward"
  }
}

// This is address used for remote clusters to connect to and the users
// accessing web UI.
resource "aws_lb_target_group" "proxy_web" {
  name     = "${var.cluster_name}-proxy-web"
  port     = 3080
  vpc_id   = "${aws_vpc.teleport.id}"
  protocol = "TCP"
}

resource "aws_lb_listener" "proxy_web" {
  load_balancer_arn = "${aws_lb.proxy.arn}"
  port              = "443"
  protocol          = "TCP"

  default_action {
    target_group_arn = "${aws_lb_target_group.proxy_web.arn}"
    type             = "forward"
  }
}

// This is a small hack to expose grafana over web port 8443
// feel free to remove it or replace with something else
resource "aws_lb_target_group" "proxy_grafana" {
  name     = "${var.cluster_name}-proxy-grafana"
  port     = 8443
  vpc_id   = "${aws_vpc.teleport.id}"
  protocol = "TCP"
}

resource "aws_lb_listener" "proxy_grafana" {
  load_balancer_arn = "${aws_lb.proxy.arn}"
  port              = "8443"
  protocol          = "TCP"

  default_action {
    target_group_arn = "${aws_lb_target_group.proxy_grafana.arn}"
    type             = "forward"
  }
}

