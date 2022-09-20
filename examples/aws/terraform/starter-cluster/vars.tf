// Region is AWS region, the region should support EFS
variable "region" {
  type = string
}

// Teleport cluster name to set up
variable "cluster_name" {
  type = string
}

// Path to Teleport Enterprise license file
variable "license_path" {
  type    = string
  default = ""
}

// AMI name to use
variable "ami_name" {
  type = string
}

// DNS and letsencrypt integration variables
// Zone name to host DNS record, e.g. example.com
variable "route53_zone" {
  type = string
}

// Domain name to use for Teleport proxy,
// e.g. proxy.example.com
variable "route53_domain" {
  type = string
}

// S3 Bucket to create for encrypted letsencrypt certificates
variable "s3_bucket_name" {
  type = string
}

// Email for LetsEncrypt domain registration
variable "email" {
  type = string
}


// SSH key name to provision instances withx
variable "key_name" {
  type = string
}

// Whether to use Amazon-issued certificates via ACM or not
// This must be set to true for any use of ACM whatsoever, regardless of whether Terraform generates/approves the cert
variable "use_letsencrypt" {
  type = string
}

// Whether to use Amazon-issued certificates via ACM or not
// This must be set to true for any use of ACM whatsoever, regardless of whether Terraform generates/approves the cert
variable "use_acm" {
  type = string
}

variable "kms_alias_name" {
  default = "alias/aws/ssm"
}

// Instance type for cluster
variable "cluster_instance_type" {
  type    = string
  default = "t3.nano"
}
