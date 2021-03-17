# Teleport cluster name to set up
# This cannot be changed later, so pick something descriptive
variable "cluster_name" {
  type = string
}

# SSH key name to provision instances with
# This must be a key that already exists in the AWS account
variable "key_name" {
  type = string
}

# AMI ID to use
# See https://github.com/gravitational/teleport/blob/master/examples/aws/terraform/AMIS.md
variable "ami_id" {
  type = string
}

# Password for Grafana admin user
variable "grafana_pass" {
  type = string
}

# Whether to use Amazon-issued certificates via ACM or not
# This must be set to true for any use of ACM whatsoever, regardless of whether Terraform generates/approves the cert
variable "use_acm" {
  type = string
}

# List of AZs to spawn auth/proxy instances in
# e.g. ["us-east-1a", "us-east-1d"]
# This must match the region specified in your provider.tf file
variable "az_list" {
  type = set(string)
}

# CIDR to use in the VPC that the module creates
# This must be at least a /16
variable "vpc_cidr" {
  type    = string
  default = "10.10.0.0/16"
}

# DNS and LetsEncrypt integration variables

# Zone name which will host DNS records, e.g. example.com
# This must already be configured in Route 53
variable "route53_zone" {
  type = string
}

# Domain name to use for Teleport proxies, e.g. proxy.example.com
# This will be the domain that Teleport users will connect to via web UI or the tsh client
variable "route53_domain" {
  type = string
}

# Optional domain name to use for Teleport proxy NLB alias
# When using ACM we have one ALB (for port 443 with TLS termination) and one NLB
# (for all other traffic - 3023/3024/3026 etc)
# As this NLB is at a different address, we add an alias record in Route 53 so that
# it can be used by applications which connect to it directly (like kubectl) rather
# than discovering the NLB's address through the Teleport API (like tsh does)
variable "route53_domain_acm_nlb_alias" {
  type = string
  default = ""
}

# Email for LetsEncrypt domain registration
variable "email" {
  type = string
}

# S3 bucket to create for encrypted LetsEncrypt certificates
# This is also used for storing the Teleport license which is downloaded to auth servers
variable "s3_bucket_name" {
  type = string
}

# Path to Teleport Enterprise license file
variable "license_path" {
  type    = string
  default = ""
}

# Instance type used for auth autoscaling group
variable "auth_instance_type" {
  type    = string
  default = "m4.large"
}

# Instance type used for proxy autoscaling group
variable "proxy_instance_type" {
  type    = string
  default = "m4.large"
}

# Instance type used for node autoscaling group
variable "node_instance_type" {
  type    = string
  default = "t2.medium"
}

# Instance type used for monitor autoscaling group
variable "monitor_instance_type" {
  type    = string
  default = "m4.large"
}

# AWS KMS alias used for encryption/decryption, defaults to alias used in SSM
variable "kms_alias_name" {
  default = "alias/aws/ssm"
}

# DynamoDB autoscaling parameters
variable "autoscale_write_target" {
  type    = string
  default = 50
}

variable "autoscale_read_target" {
  type    = string
  default = 50
}

variable "autoscale_min_read_capacity" {
  type    = string
  default = 5
}

variable "autoscale_max_read_capacity" {
  type    = string
  default = 100
}

variable "autoscale_min_write_capacity" {
  type    = string
  default = 5
}

variable "autoscale_max_write_capacity" {
  type    = string
  default = 100
}

# Default auth type to configure for this Teleport cluster
# Affects the default connector chosen when logging in using `tsh login`
# Can be `local`, `oidc`, `saml` or `github`
# Default is `local`
variable "auth_type" {
  type    = string
  default = "local"
}

# Account ID which owns the AMIs used to spin up instances
# You should only need to change this if you're building your own AMIs for testing purposes.
variable "ami_owner_account_id" {
  type    = string
  default = "126027368216"
}
