// Proxy is deployed in public subnet to receive
// traffic from Network load balancers.

// Proxy SG for instances behind network LB
resource "aws_security_group" "proxy" {
  name   = "${var.cluster_name}-proxy"
  vpc_id = local.vpc_id
  tags = {
    TeleportCluster = var.cluster_name
  }
}

// Proxy SG for application LB (ACM)
resource "aws_security_group" "proxy_acm" {
  name   = "${var.cluster_name}-proxy-acm"
  vpc_id = local.vpc_id
  count  = var.use_acm ? 1 : 0
  tags = {
    TeleportCluster = var.cluster_name
  }
}

// SSH emergency access via bastion only
resource "aws_security_group_rule" "proxy_ingress_allow_ssh" {
  type                     = "ingress"
  from_port                = 22
  to_port                  = 22
  protocol                 = "tcp"
  security_group_id        = aws_security_group.proxy.id
  source_security_group_id = aws_security_group.bastion.id
}

// Ingress traffic to web port 443 is allowed from all directions (ACM)
resource "aws_security_group_rule" "proxy_ingress_allow_web_acm" {
  type              = "ingress"
  from_port         = 443
  to_port           = 443
  protocol          = "tcp"
  cidr_blocks       = var.allowed_proxy_ingress_cidr_blocks
  security_group_id = aws_security_group.proxy_acm[0].id
  count             = var.use_acm ? 1 : 0
}

// Ingress proxy traffic is allowed from all ports
resource "aws_security_group_rule" "proxy_ingress_allow_proxy" {
  type              = "ingress"
  from_port         = 3023
  to_port           = 3023
  protocol          = "tcp"
  cidr_blocks       = var.allowed_proxy_ingress_cidr_blocks
  security_group_id = aws_security_group.proxy.id
}

// Ingress traffic to tunnel port 3024 is allowed from all directions (ACM)
resource "aws_security_group_rule" "proxy_ingress_allow_tunnel" {
  type              = "ingress"
  from_port         = 3024
  to_port           = 3024
  protocol          = "tcp"
  cidr_blocks       = var.allowed_proxy_ingress_cidr_blocks
  security_group_id = aws_security_group.proxy.id
  count             = var.use_acm ? 1 : 0
}

// Ingress traffic to web port 3026 is allowed from all directions
resource "aws_security_group_rule" "proxy_ingress_allow_kube" {
  type              = "ingress"
  from_port         = 3026
  to_port           = 3026
  protocol          = "tcp"
  cidr_blocks       = var.allowed_proxy_ingress_cidr_blocks
  security_group_id = aws_security_group.proxy.id
}

// Permit inbound to Teleport mysql services
resource "aws_security_group_rule" "proxy_ingress_allow_mysql" {
  type              = "ingress"
  from_port         = 3036
  to_port           = 3036
  protocol          = "tcp"
  cidr_blocks       = var.allowed_proxy_ingress_cidr_blocks
  security_group_id = aws_security_group.proxy.id
}
// Permit inbound to Teleport postgres services
resource "aws_security_group_rule" "proxy_ingress_allow_postgres" {
  type              = "ingress"
  from_port         = 5432
  to_port           = 5432
  protocol          = "tcp"
  cidr_blocks       = var.allowed_proxy_ingress_cidr_blocks
  security_group_id = aws_security_group.proxy.id
}

// Permit inbound to Teleport mongodb services
resource "aws_security_group_rule" "cluster_ingress_mongodb" {
  type              = "ingress"
  from_port         = 27017
  to_port           = 27017
  protocol          = "tcp"
  cidr_blocks       = var.allowed_proxy_ingress_cidr_blocks
  security_group_id = aws_security_group.proxy.id
}

// Ingress traffic to web port 3080 is allowed from all directions
resource "aws_security_group_rule" "proxy_ingress_allow_web" {
  type              = "ingress"
  from_port         = 3080
  to_port           = 3080
  protocol          = "tcp"
  cidr_blocks       = var.allowed_proxy_ingress_cidr_blocks
  security_group_id = aws_security_group.proxy.id
}

// Ingress traffic to grafana port 8443 is allowed from all directions (ACM)
resource "aws_security_group_rule" "proxy_ingress_allow_grafana_acm" {
  type              = "ingress"
  from_port         = 8443
  to_port           = 8443
  protocol          = "tcp"
  cidr_blocks       = var.allowed_proxy_ingress_cidr_blocks
  security_group_id = aws_security_group.proxy_acm[0].id
  count             = var.use_acm ? 1 : 0
}

// Egress traffic is allowed everywhere
resource "aws_security_group_rule" "proxy_egress_allow_all_traffic" {
  type              = "egress"
  from_port         = 0
  to_port           = 0
  protocol          = "-1"
  cidr_blocks       = var.allowed_proxy_egress_cidr_blocks
  security_group_id = aws_security_group.proxy.id
}

// Egress traffic is allowed everywhere (ACM)
resource "aws_security_group_rule" "proxy_egress_allow_all_traffic_acm" {
  type              = "egress"
  from_port         = 0
  to_port           = 0
  protocol          = "-1"
  cidr_blocks       = var.allowed_proxy_egress_cidr_blocks
  security_group_id = aws_security_group.proxy_acm[0].id
  count             = var.use_acm ? 1 : 0
}

// Network load balancer for proxy server
resource "aws_lb" "proxy" {
  name                              = "${var.cluster_name}-proxy"
  internal                          = false
  subnets                           = aws_subnet.public.*.id
  load_balancer_type                = "network"
  idle_timeout                      = 3600
  enable_cross_zone_load_balancing  = true

  tags = {
    TeleportCluster = var.cluster_name
  }
}

// Application load balancer for proxy server web interface (using ACM)
resource "aws_lb" "proxy_acm" {
  name               = "${var.cluster_name}-proxy-acm"
  internal           = false
  subnets            = aws_subnet.public.*.id
  load_balancer_type = "application"
  idle_timeout       = 3600
  security_groups    = [aws_security_group.proxy_acm[0].id]
  count              = var.use_acm ? 1 : 0
  tags = {
    TeleportCluster = var.cluster_name
  }
}

// Proxy is for SSH proxy - jumphost target endpoint.
resource "aws_lb_target_group" "proxy_proxy" {
  name     = "${var.cluster_name}-proxy-proxy"
  port     = 3023
  vpc_id   = aws_vpc.teleport.id
  protocol = "TCP"
}

resource "aws_lb_listener" "proxy_proxy" {
  load_balancer_arn = aws_lb.proxy.arn
  port              = "3023"
  protocol          = "TCP"

  default_action {
    target_group_arn = aws_lb_target_group.proxy_proxy.arn
    type             = "forward"
  }
}

// Tunnel endpoint/listener on LB - this is only used with ACM (as
// Teleport web/tunnel multiplexing can be used with Letsencrypt)
resource "aws_lb_target_group" "proxy_tunnel_acm" {
  name     = "${var.cluster_name}-proxy-tunnel"
  port     = 3024
  vpc_id   = aws_vpc.teleport.id
  protocol = "TCP"
  count    = var.use_acm ? 1 : 0
}

resource "aws_lb_listener" "proxy_tunnel_acm" {
  load_balancer_arn = aws_lb.proxy.arn
  port              = "3024"
  protocol          = "TCP"
  count             = var.use_acm ? 1 : 0

  default_action {
    target_group_arn = aws_lb_target_group.proxy_tunnel_acm[0].arn
    type             = "forward"
  }
}

// Proxy is for Kube proxy - jumphost target endpoint.
resource "aws_lb_target_group" "proxy_kube" {
  name     = "${var.cluster_name}-proxy-kube"
  port     = 3026
  vpc_id   = aws_vpc.teleport.id
  protocol = "TCP"
}

resource "aws_lb_listener" "proxy_kube" {
  load_balancer_arn = aws_lb.proxy.arn
  port              = "3026"
  protocol          = "TCP"

  default_action {
    target_group_arn = aws_lb_target_group.proxy_kube.arn
    type             = "forward"
  }
}

// MySQL port
resource "aws_lb_target_group" "proxy_mysql" {
  name     = "${var.cluster_name}-proxy-mysql"
  port     = 3036
  vpc_id   = aws_vpc.teleport.id
  protocol = "TCP"
}

resource "aws_lb_listener" "proxy_mysql" {
  load_balancer_arn = aws_lb.proxy.arn
  port              = "3036"
  protocol          = "TCP"

  default_action {
    target_group_arn = aws_lb_target_group.proxy_mysql.arn
    type             = "forward"
  }
}

// Postgres port
resource "aws_lb_target_group" "proxy_postgres" {
  name     = "${var.cluster_name}-proxy-postgres"
  port     = 5432
  vpc_id   = aws_vpc.teleport.id
  protocol = "TCP"
}

resource "aws_lb_listener" "proxy_postgres" {
  load_balancer_arn = aws_lb.proxy.arn
  port              = "5432"
  protocol          = "TCP"

  default_action {
    target_group_arn = aws_lb_target_group.proxy_postgres.arn
    type             = "forward"
  }
}

// MongoDB port
resource "aws_lb_target_group" "proxy_mongodb" {
  name     = "${var.cluster_name}-proxy-mongodb"
  port     = 27017
  vpc_id   = aws_vpc.teleport.id
  protocol = "TCP"
}

resource "aws_lb_listener" "proxy_mongodb" {
  load_balancer_arn = aws_lb.proxy.arn
  port              = "27017"
  protocol          = "TCP"

  default_action {
    target_group_arn = aws_lb_target_group.proxy_mongodb.arn
    type             = "forward"
  }
}

// This is address used for remote clusters to connect to and the users
// accessing web UI.

// Proxy web target group (using letsencrypt)
resource "aws_lb_target_group" "proxy_web" {
  name     = "${var.cluster_name}-proxy-web"
  port     = 3080
  vpc_id   = aws_vpc.teleport.id
  count    = var.use_acm ? 0 : 1
  protocol = "TCP"
}

// Proxy web listener (using letsencrypt)
resource "aws_lb_listener" "proxy_web" {
  load_balancer_arn = aws_lb.proxy.arn
  port              = "443"
  protocol          = "TCP"
  count             = var.use_acm ? 0 : 1

  default_action {
    target_group_arn = aws_lb_target_group.proxy_web[0].arn
    type             = "forward"
  }
}

// Proxy web target group (using ACM)
resource "aws_lb_target_group" "proxy_web_acm" {
  name     = "${var.cluster_name}-proxy-web"
  port     = 3080
  vpc_id   = aws_vpc.teleport.id
  protocol = "HTTPS"
  count    = var.use_acm ? 1 : 0

  health_check {
    path     = "/web/login"
    protocol = "HTTPS"
  }
}

// Proxy web listener (using ACM)
resource "aws_lb_listener" "proxy_web_acm" {
  load_balancer_arn = aws_lb.proxy_acm[0].arn
  port              = "443"
  protocol          = "HTTPS"
  certificate_arn   = aws_acm_certificate_validation.cert[0].certificate_arn
  count             = var.use_acm ? 1 : 0

  default_action {
    target_group_arn = aws_lb_target_group.proxy_web_acm[0].arn
    type             = "forward"
  }
}

// This is a small hack to expose grafana over web port 8443
// feel free to remove it or replace with something else
// letsencrypt
resource "aws_lb_target_group" "proxy_grafana" {
  name     = "${var.cluster_name}-proxy-grafana"
  port     = 8443
  vpc_id   = aws_vpc.teleport.id
  protocol = "TCP"
  count    = var.use_acm ? 0 : 1
}

resource "aws_lb_listener" "proxy_grafana" {
  load_balancer_arn = aws_lb.proxy.arn
  port              = "8443"
  protocol          = "TCP"
  count             = var.use_acm ? 0 : 1

  default_action {
    target_group_arn = aws_lb_target_group.proxy_grafana[0].arn
    type             = "forward"
  }
}

// ACM
resource "aws_lb_target_group" "proxy_grafana_acm" {
  name     = "${var.cluster_name}-proxy-grafana"
  port     = 8444
  vpc_id   = aws_vpc.teleport.id
  protocol = "HTTP"
  count    = var.use_acm ? 1 : 0
}

resource "aws_lb_listener" "proxy_grafana_acm" {
  load_balancer_arn = aws_lb.proxy_acm[0].arn
  port              = "8443"
  protocol          = "HTTPS"
  certificate_arn   = aws_acm_certificate_validation.cert[0].certificate_arn
  count             = var.use_acm ? 1 : 0

  default_action {
    target_group_arn = aws_lb_target_group.proxy_grafana_acm[0].arn
    type             = "forward"
  }
}

