/*
Route53 is used to configure SSL for this cluster. A
Route53 hosted zone must exist in the AWS account for
this automation to work.
*/

// Create A record to instance IP
resource "aws_route53_record" "cluster" {
  zone_id = data.aws_route53_zone.cluster.zone_id
  name    = var.route53_domain
  type    = "A"
  ttl     = "300"
  records = [aws_instance.cluster.public_ip]
}

// Create A record to instance IP for wildcard subdomain
resource "aws_route53_record" "wildcard-cluster" {
  zone_id = data.aws_route53_zone.cluster.zone_id
  name    = "*.${var.route53_domain}"
  type    = "A"
  ttl     = "300"
  records = [aws_instance.cluster.public_ip]
  count  = var.add_wildcard_route53_record ? 1 : 0
}
