// Proxy is deployed in public subnet to receive
// traffic from Network load balancers.

// Proxy SG for instances behind network LB
resource "aws_security_group" "proxy" {
  name        = "${var.cluster_name}-proxy"
  description = "Proxy SG for instances behind network LB"
  vpc_id      = local.vpc_id
  tags = {
    TeleportCluster = var.cluster_name
  }
}

// Proxy SG for application LB (ACM)
resource "aws_security_group" "proxy_acm" {
  name        = "${var.cluster_name}-proxy-acm"
  description = "Proxy SG for application LB (ACM)"
  vpc_id      = local.vpc_id
  count       = var.use_acm ? 1 : 0
  tags = {
    TeleportCluster = var.cluster_name
  }
}

// SSH emergency access via bastion only
resource "aws_security_group_rule" "proxy_ingress_allow_ssh" {
  description              = "SSH emergency access via bastion only"
  type                     = "ingress"
  from_port                = 22
  to_port                  = 22
  protocol                 = "tcp"
  security_group_id        = aws_security_group.proxy.id
  source_security_group_id = aws_security_group.bastion.id
}

// Ingress traffic to web port 443 is allowed from all directions (ACM)
// tfsec:ignore:aws-ec2-no-public-ingress-sgr
resource "aws_security_group_rule" "proxy_ingress_allow_web_acm" {
  description       = "Ingress traffic to web port 443 is allowed from all directions (ACM)"
  type              = "ingress"
  from_port         = 443
  to_port           = 443
  protocol          = "tcp"
  cidr_blocks       = var.allowed_proxy_ingress_cidr_blocks
  security_group_id = aws_security_group.proxy_acm[0].id
  count             = var.use_acm ? 1 : 0
}

// Ingress proxy traffic is allowed from all ports
// tfsec:ignore:aws-ec2-no-public-ingress-sgr
resource "aws_security_group_rule" "proxy_ingress_allow_proxy" {
  description       = "Ingress proxy traffic is allowed from all ports"
  type              = "ingress"
  from_port         = 3023
  to_port           = 3023
  protocol          = "tcp"
  cidr_blocks       = var.allowed_proxy_ingress_cidr_blocks
  security_group_id = aws_security_group.proxy.id
  count             = var.use_tls_routing ? 0 : 1
}

// Ingress traffic to tunnel port 3024 is allowed from all directions
// tfsec:ignore:aws-ec2-no-public-ingress-sgr
resource "aws_security_group_rule" "proxy_ingress_allow_tunnel" {
  description       = "Ingress traffic to tunnel port 3024 is allowed from all directions"
  type              = "ingress"
  from_port         = 3024
  to_port           = 3024
  protocol          = "tcp"
  cidr_blocks       = var.allowed_proxy_ingress_cidr_blocks
  security_group_id = aws_security_group.proxy.id
  count             = var.use_tls_routing ? 0 : 1
}

// Ingress traffic to web port 3026 is allowed from all directions
// tfsec:ignore:aws-ec2-no-public-ingress-sgr
resource "aws_security_group_rule" "proxy_ingress_allow_kube" {
  description       = "Ingress traffic to web port 3026 is allowed from all directions"
  type              = "ingress"
  from_port         = 3026
  to_port           = 3026
  protocol          = "tcp"
  cidr_blocks       = var.allowed_proxy_ingress_cidr_blocks
  security_group_id = aws_security_group.proxy.id
  count             = var.use_tls_routing ? 0 : 1
}

// Permit inbound to Teleport mysql services
// tfsec:ignore:aws-ec2-no-public-ingress-sgr
resource "aws_security_group_rule" "proxy_ingress_allow_mysql" {
  description       = "Permit inbound to Teleport mysql services"
  type              = "ingress"
  from_port         = 3036
  to_port           = 3036
  protocol          = "tcp"
  cidr_blocks       = var.allowed_proxy_ingress_cidr_blocks
  security_group_id = aws_security_group.proxy.id
  count             = var.enable_mysql_listener && !var.use_tls_routing ? 1 : 0
}

// Permit inbound to Teleport postgres services
// tfsec:ignore:aws-ec2-no-public-ingress-sgr
resource "aws_security_group_rule" "proxy_ingress_allow_postgres" {
  description       = "Permit inbound to Teleport postgres services"
  type              = "ingress"
  from_port         = 5432
  to_port           = 5432
  protocol          = "tcp"
  cidr_blocks       = var.allowed_proxy_ingress_cidr_blocks
  security_group_id = aws_security_group.proxy.id
  count             = var.enable_postgres_listener && !var.use_tls_routing ? 1 : 0
}

// Permit inbound to Teleport mongodb services
// tfsec:ignore:aws-ec2-no-public-ingress-sgr
resource "aws_security_group_rule" "cluster_ingress_mongodb" {
  description       = "Permit inbound to Teleport mongodb services"
  type              = "ingress"
  from_port         = 27017
  to_port           = 27017
  protocol          = "tcp"
  cidr_blocks       = var.allowed_proxy_ingress_cidr_blocks
  security_group_id = aws_security_group.proxy.id
  count             = var.enable_mongodb_listener && !var.use_tls_routing ? 1 : 0
}

// Ingress traffic to web port 3080 is allowed from all directions
// tfsec:ignore:aws-ec2-no-public-ingress-sgr
resource "aws_security_group_rule" "proxy_ingress_allow_web" {
  description       = "Ingress traffic to web port 3080 is allowed from all directions"
  type              = "ingress"
  from_port         = 3080
  to_port           = 3080
  protocol          = "tcp"
  cidr_blocks       = var.allowed_proxy_ingress_cidr_blocks
  security_group_id = aws_security_group.proxy.id
}

// Egress traffic is allowed everywhere
// tfsec:ignore:aws-ec2-no-public-egress-sgr
resource "aws_security_group_rule" "proxy_egress_allow_all_traffic" {
  description       = "Egress traffic is allowed everywhere"
  type              = "egress"
  from_port         = 0
  to_port           = 0
  protocol          = "-1"
  cidr_blocks       = var.allowed_proxy_egress_cidr_blocks
  security_group_id = aws_security_group.proxy.id
}

// Egress traffic is allowed everywhere (ACM)
// tfsec:ignore:aws-ec2-no-public-iegress-sgr
resource "aws_security_group_rule" "proxy_egress_allow_all_traffic_acm" {
  description       = "Egress traffic is allowed everywhere (ACM)"
  type              = "egress"
  from_port         = 0
  to_port           = 0
  protocol          = "-1"
  cidr_blocks       = var.allowed_proxy_egress_cidr_blocks
  security_group_id = aws_security_group.proxy_acm[0].id
  count             = var.use_acm ? 1 : 0
}

// Network load balancer for proxy server
// Expected to be public-facing
// tfsec:ignore:aws-elb-alb-not-public
resource "aws_lb" "proxy" {
  name                             = "${var.cluster_name}-proxy"
  internal                         = false
  subnets                          = aws_subnet.public.*.id
  load_balancer_type               = "network"
  idle_timeout                     = 3600
  enable_cross_zone_load_balancing = true
  count                            = var.use_acm && var.use_tls_routing ? 0 : 1

  tags = {
    TeleportCluster = var.cluster_name
  }
}

// Application load balancer for proxy server TLS listener (using ACM)
resource "aws_lb" "proxy_acm" {
  name                       = "${var.cluster_name}-proxy-acm"
  internal                   = false
  subnets                    = aws_subnet.public.*.id
  load_balancer_type         = "application"
  idle_timeout               = 3600
  drop_invalid_header_fields = true
  security_groups            = [aws_security_group.proxy_acm[0].id]
  count                      = var.use_acm ? 1 : 0
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
  count    = var.use_tls_routing ? 0 : 1
  // required to allow the use of IP pinning
  // this can only be enabled when ACM is not being used
  proxy_protocol_v2 = var.use_acm ? false : true
}

resource "aws_lb_listener" "proxy_proxy" {
  load_balancer_arn = aws_lb.proxy[0].arn
  port              = "3023"
  protocol          = "TCP"
  count             = var.use_tls_routing ? 0 : 1

  default_action {
    target_group_arn = aws_lb_target_group.proxy_proxy[0].arn
    type             = "forward"
  }
}

// Tunnel endpoint/listener on LB - this is only used with ACM (as
// Teleport web/tunnel multiplexing can be used with Let's Encrypt)
resource "aws_lb_target_group" "proxy_tunnel" {
  name     = "${var.cluster_name}-proxy-tunnel"
  port     = 3024
  vpc_id   = aws_vpc.teleport.id
  protocol = "TCP"
  // only create this if TLS routing is disabled
  count = var.use_tls_routing ? 0 : 1
  // required to allow the use of IP pinning
  // this can only be enabled when ACM is not being used
  proxy_protocol_v2 = var.use_acm ? false : true
}

resource "aws_lb_listener" "proxy_tunnel" {
  load_balancer_arn = aws_lb.proxy[0].arn
  port              = "3024"
  protocol          = "TCP"
  // only create this if TLS routing is disabled
  count = var.use_tls_routing ? 0 : 1

  default_action {
    target_group_arn = aws_lb_target_group.proxy_tunnel[0].arn
    type             = "forward"
  }
}

// Proxy is for Kube proxy - jumphost target endpoint.
resource "aws_lb_target_group" "proxy_kube" {
  name     = "${var.cluster_name}-proxy-kube"
  port     = 3026
  vpc_id   = aws_vpc.teleport.id
  protocol = "TCP"
  count    = var.use_tls_routing ? 0 : 1
  // required to allow the use of IP pinning
  // this can only be enabled when ACM is not being used
  proxy_protocol_v2 = var.use_acm ? false : true
}

resource "aws_lb_listener" "proxy_kube" {
  load_balancer_arn = aws_lb.proxy[0].arn
  port              = "3026"
  protocol          = "TCP"
  count             = var.use_tls_routing ? 0 : 1

  default_action {
    target_group_arn = aws_lb_target_group.proxy_kube[0].arn
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
  load_balancer_arn = aws_lb.proxy[0].arn
  port              = "3036"
  protocol          = "TCP"
  // only create this if the mysql listener is enabled and TLS routing is disabled
  count = var.enable_mysql_listener ? !var.use_tls_routing ? 1 : 0 : 0

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
  load_balancer_arn = aws_lb.proxy[0].arn
  port              = "5432"
  protocol          = "TCP"
  // only create this if the postgres listener is enabled and TLS routing is disabled
  count = var.enable_postgres_listener ? !var.use_tls_routing ? 1 : 0 : 0

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
  load_balancer_arn = aws_lb.proxy[0].arn
  port              = "27017"
  protocol          = "TCP"
  // only create this if the mongo listener is enabled and TLS routing is disabled
  count = var.enable_mongodb_listener ? !var.use_tls_routing ? 1 : 0 : 0

  default_action {
    target_group_arn = aws_lb_target_group.proxy_mongodb.arn
    type             = "forward"
  }
}

// This is address used for remote clusters to connect to and the users
// accessing web UI.

// Proxy web target group (using Let's Encrypt)
resource "aws_lb_target_group" "proxy_web" {
  name     = "${var.cluster_name}-proxy-web"
  port     = 3080
  vpc_id   = aws_vpc.teleport.id
  count    = var.use_acm ? 0 : 1
  protocol = "TCP"
  // required to allow the use of IP pinning
  proxy_protocol_v2 = "true"
}

// Proxy web listener (using Let's Encrypt)
resource "aws_lb_listener" "proxy_web" {
  load_balancer_arn = aws_lb.proxy[0].arn
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
