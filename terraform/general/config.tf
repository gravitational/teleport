terraform {
  backend "s3" {
    bucket         = "loadsmart-terraform"
    key            = "teleport/general/terraform.tfstate"
    region         = "us-east-1"
    encrypt        = true
    kms_key_id     = "arn:aws:kms:us-east-1:845156828388:key/a18b304d-d85b-4deb-b5b2-67771cff721a"
    dynamodb_table = "terraform_locks"
    profile        = "loadsmart-main"
  }
}
