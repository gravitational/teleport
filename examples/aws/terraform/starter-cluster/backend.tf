terraform {
  backend "s3" {
    bucket = "gus-terraform-backend.teleportdemo.com"
    key    = "gus-starter.teleportdemo.com.tfstate"
    region = "us-east-1"
  }
}
