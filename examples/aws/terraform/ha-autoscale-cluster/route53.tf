// Example route53 zone for automation purposes
// used to provision public DNS name for proxy
data "aws_route53_zone" "proxy" {
  name = var.route53_zone
}

// Route53 record connects proxy network load balancer
// Let's Encrypt
resource "aws_route53_record" "proxy" {
  zone_id = data.aws_route53_zone.proxy.zone_id
  name    = var.route53_domain
  type    = "A"
  count   = var.use_acm ? 0 : 1

  alias {
    name                   = aws_lb.proxy[0].dns_name
    zone_id                = aws_lb.proxy[0].zone_id
    evaluate_target_health = true
  }
}

// Route53 record connects proxy network load balancer with wildcard
// Let's Encrypt
resource "aws_route53_record" "proxy_wildcard" {
  zone_id = data.aws_route53_zone.proxy.zone_id
  name    = "*.${var.route53_domain}"
  type    = "A"
  count   = var.add_wildcard_route53_record && !var.use_acm ? 1 : 0

  alias {
    name                   = aws_lb.proxy[0].dns_name
    zone_id                = aws_lb.proxy[0].zone_id
    evaluate_target_health = true
  }
}

// ACM (ALB)
resource "aws_route53_record" "proxy_acm" {
  zone_id = data.aws_route53_zone.proxy.zone_id
  name    = var.route53_domain
  type    = "A"
  count   = var.use_acm ? 1 : 0

  alias {
    name                   = aws_lb.proxy_acm[0].dns_name
    zone_id                = aws_lb.proxy_acm[0].zone_id
    evaluate_target_health = true
  }
}

// ACM (ALB) Wildcard
resource "aws_route53_record" "proxy_acm_wildcard" {
  zone_id = data.aws_route53_zone.proxy.zone_id
  name    = "*.${var.route53_domain}"
  type    = "A"
  count   = var.add_wildcard_route53_record && var.use_acm ? 1 : 0

  alias {
    name                   = aws_lb.proxy_acm[0].dns_name
    zone_id                = aws_lb.proxy_acm[0].zone_id
    evaluate_target_health = true
  }
}

// ACM (NLB)
resource "aws_route53_record" "proxy_acm_nlb_alias" {
  zone_id = data.aws_route53_zone.proxy.zone_id
  name    = var.route53_domain_acm_nlb_alias
  type    = "A"
  count   = var.use_acm ? var.route53_domain_acm_nlb_alias != "" ? 1 : 0 : 0

  alias {
    name                   = aws_lb.proxy[0].dns_name
    zone_id                = aws_lb.proxy[0].zone_id
    evaluate_target_health = true
  }
}
