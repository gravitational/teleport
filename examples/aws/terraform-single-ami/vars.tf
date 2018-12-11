// Region is AWS region, the region should support EFS
variable "region" {
  type = "string"
}

// Script creates a separate VPC with demo deployment
variable "vpc_cidr" {
  type = "string"
  default = "172.31.0.0/16"
}

// Teleport cluster name to set up
variable "cluster_name" {
  type = "string"
}

// Teleport version to install
variable "teleport_version" {
  type = "string"
}

// Teleport UID is a UID for teleport user provisioned on the hosts
variable "teleport_uid" {
  type = "string"
  default = "1007"
}

// Instance types used for authentication servers auto scale groups
variable "auth_instance_type" {
  type = "string"
  default = "m4.large"
}

// Instance types used for proxy auto scale groups
variable "proxy_instance_type" {
  type = "string"
  default = "m4.large"
}

// Instance types used for teleport nodes auto scale groups
variable "node_instance_type" {
  type = "string"
  default = "t2.medium"
}

// SSH key name to provision instances withx
variable "key_name" {
  type = "string"
}

// DNS and letsencrypt integration variables
// Zone name to host DNS record, e.g. example.com
variable "route53_zone" {
  type = "string"
}

// Domain name to use for Teleport proxies,
// e.g. proxy.example.com
variable "route53_domain" {
  type = "string"
}

// Email for letsencrypt domain registration
variable "email" {
  type = "string"
}

// S3 Bucket to create for encrypted letsencrypt certificates
variable "s3_bucket_name" {
  type = "string"
}

// AWS KMS alias used for encryption/decryption
// default is alias used in SSM
variable "kms_alias_name" {
  default = "alias/aws/ssm"
}

// path to teleport enterprise/pro license file
variable license_path {
  type = "string"
}

// AMI name to use
variable ami_name {
 type = "string"
}

// DynamoDB autoscale parameters
variable "autoscale_write_target" {
  type = "string"
  default = 50
}

variable "autoscale_read_target" {
  type = "string"
  default = 50
}

variable "autoscale_min_read_capacity" {
  type = "string"
  default = 5
}

variable "autoscale_max_read_capacity" {
  type = "string"
  default = 100
}

variable "autoscale_min_write_capacity" {
  type = "string"
  default = 5
}

variable "autoscale_max_write_capacity" {
  type = "string"
  default = 100
}

// InfluxDB and Telegraf versions
variable "influxdb_version" {
   type = "string"
   default = "1.4.2"
}

variable "telegraf_version" {
   type = "string"
   default = "1.5.1-1"
}

variable "grafana_version" {
   type = "string"
   default = "4.6.3"
}

// Instance types used for proxy auto scale groups
variable "monitor_instance_type" {
  type = "string"
  default = "m4.large"
}

// Password for grafana admin user
variable "grafana_pass" {
  type = "string"
}
