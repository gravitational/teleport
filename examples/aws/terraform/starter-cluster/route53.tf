/*
Route53 is used to configure SSL for this cluster. A
Route53 hosted zone must exist in the AWS account for
this automation to work.
*/

// Create A record to instance IP when ACM is disabled
resource "aws_route53_record" "cluster" {
  zone_id = data.aws_route53_zone.cluster.zone_id
  name    = var.route53_domain
  type    = "A"
  ttl     = 300
  records = [aws_instance.cluster.public_ip]
  count   = var.use_acm ? 0 : 1
}

// Create A record to instance IP for wildcard subdomain when ACM is disabled
resource "aws_route53_record" "wildcard-cluster" {
  zone_id = data.aws_route53_zone.cluster.zone_id
  name    = "*.${var.route53_domain}"
  type    = "A"
  ttl     = 300
  records = [aws_instance.cluster.public_ip]
  count   = var.add_wildcard_route53_record && !var.use_acm ? 1 : 0
}

// ACM (ALB)
resource "aws_route53_record" "cluster_acm" {
  zone_id = data.aws_route53_zone.cluster.zone_id
  name    = var.route53_domain
  type    = "A"
  count   = var.use_acm ? 1 : 0

  alias {
    name                   = aws_lb.cluster[0].dns_name
    zone_id                = aws_lb.cluster[0].zone_id
    evaluate_target_health = true
  }
}

// ACM (ALB) Wildcard
resource "aws_route53_record" "proxy_acm_wildcard" {
  zone_id = data.aws_route53_zone.cluster.zone_id
  name    = "*.${var.route53_domain}"
  type    = "A"
  count   = var.add_wildcard_route53_record && var.use_acm ? 1 : 0

  alias {
    name                   = aws_lb.cluster[0].dns_name
    zone_id                = aws_lb.cluster[0].zone_id
    evaluate_target_health = true
  }
}
