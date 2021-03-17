# Base AMI is found by ID - this is to prevent issues where launch configurations
# are changed and ASGs recreated because the AMI ID gets updated externally
data "aws_ami" "base" {
  most_recent = true
  owners      = [var.ami_owner_account_id]

  filter {
    name   = "image-id"
    values = [var.ami_id]
  }
}

# Used in role ARNs
data "aws_caller_identity" "current" {}

# Used to access provider region
data "aws_region" "current" {}

# By default, SSM picks the alias for the encryption key
data "aws_kms_alias" "ssm" {
  name = var.kms_alias_name
}

# Pick up the license path and make it accessible as a file
data "local_file" "license" {
  filename = var.license_path
}
