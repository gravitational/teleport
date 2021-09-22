terraform {
  backend "s3" {
    bucket = "gus-terraform-backend"
    key    = "old-letsencrypt.tfstate"
    region = "us-east-1"
  }
}
