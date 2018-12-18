// Example route53 zone for automation purposes
// used to provision public DNS name for proxy
data "aws_route53_zone" "proxy" {
  name = "${var.route53_zone}"
}

// Route53 record connects proxy network load balancer
// letsencrypt
resource "aws_route53_record" "proxy" {
  zone_id = "${data.aws_route53_zone.proxy.zone_id}"
  name    = "${var.route53_domain}"
  type    = "A"
  count   =  "${var.use_acm == "" ? 1 : 0}"

  alias {
    name                   = "${aws_lb.proxy.dns_name}"
    zone_id                = "${aws_lb.proxy.zone_id}"
    evaluate_target_health = true
  }
}

// ACM
resource "aws_route53_record" "proxy_acm" {
  zone_id = "${data.aws_route53_zone.proxy.zone_id}"
  name    = "${var.route53_domain}"
  type    = "A"
  count   =  "${var.use_acm ? 1 : 0}"

  alias {
    name                   = "${aws_lb.proxy_acm.dns_name}"
    zone_id                = "${aws_lb.proxy_acm.zone_id}"
    evaluate_target_health = true
  }
}