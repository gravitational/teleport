// S3 bucket is used to distribute letsencrypt certificates
resource "aws_s3_bucket" "certs" {
  bucket = "${var.s3_bucket_name}"
  acl = "private"
  force_destroy = true
  server_side_encryption_configuration {
    rule {
      apply_server_side_encryption_by_default {
        sse_algorithm     = "AES256"
      }
    }
  }
}

resource "aws_s3_bucket_object" "grafana_teleport_dashboard" {
  bucket = "${aws_s3_bucket.certs.bucket}"
  key    = "health-dashboard.json"
  source = "../../../assets/monitoring/health-dashboard.json"
}

resource "aws_s3_bucket_object" "grafana_teleport_nginx" {
  bucket = "${aws_s3_bucket.certs.bucket}"
  key    = "grafana-nginx.conf"
  source = "./grafana-nginx.conf"
}
