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

// Whether to add a while a wildcard entry *.proxy.example.com for application access
variable "add_wildcard_route53_record" {
  type = string
}

// whether to enable the mongodb listener
// adds security group setting, maps load balancer to port and adds to teleport config
variable "enable_mongodb_listener" {
  type    = bool
  default = false
}

// whether to enable the mysql listener
// adds security group setting, maps load balancer to port and adds to teleport config
variable "enable_mysql_listener" {
  type    = bool
  default = false
}

// whether to enable the postgres listener
// adds security group setting, maps load balancer to port and adds to teleport config
variable "enable_postgres_listener" {
  type    = bool
  default = false
}

// S3 Bucket to create for encrypted letsencrypt certificates
variable "s3_bucket_name" {
  type = string
}

// Email for LetsEncrypt domain registration
variable "email" {
  type = string
}


// SSH key name to provision instances with
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

// CIDR blocks allowed to connect to the SSH port
variable "allowed_ssh_ingress_cidr_blocks" {
  type = list
  default = ["0.0.0.0/0"]
}

// CIDR blocks allowed for ingress for all Teleport ports 
variable "allowed_ingress_cidr_blocks" {
  type = list
  default = ["0.0.0.0/0"]
}

// CIDR blocks allowed for egress from Teleport
variable "allowed_egress_cidr_blocks" {
  type = list
  default = ["0.0.0.0/0"]
}

variable "kms_alias_name" {
  default = "alias/aws/ssm"
}

// Instance type for cluster
variable "cluster_instance_type" {
  type    = string
  default = "t3.nano"
}
