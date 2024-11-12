// Region is AWS region, the region should support EFS
variable "region" {
  type = string
}

// Script creates a separate VPC with demo deployment
variable "vpc_cidr" {
  type    = string
  default = "172.31.0.0/16"
}

// Teleport cluster name to set up
variable "cluster_name" {
  type = string
}

// Teleport UID is a UID for teleport user provisioned on the hosts
variable "teleport_uid" {
  type    = string
  default = "1007"
}

// Instance types used for authentication servers auto scale groups
variable "auth_instance_type" {
  type    = string
  default = "m7g.large"
}

// Instance types used for proxy auto scale groups
variable "proxy_instance_type" {
  type    = string
  default = "m7g.large"
}

// Instance types used for teleport nodes auto scale groups
variable "node_instance_type" {
  type    = string
  default = "t4g.medium"
}

// Instance type used for bastion server
variable "bastion_instance_type" {
  type    = string
  default = "t4g.medium"
}

// SSH key name to provision instances withx
variable "key_name" {
  type = string
}

// DNS and Let's Encrypt integration variables
// Zone name to host DNS record, e.g. example.com
variable "route53_zone" {
  type = string
}

// Domain name to use for Teleport proxies,
// e.g. proxy.example.com
variable "route53_domain" {
  type = string
}

// Whether to set wildcard Route53
// for use in Application Access
variable "add_wildcard_route53_record" {
  type    = bool
  default = false
}

// whether to enable the mongodb listener
// adds security group setting, maps load balancer to port, and adds to teleport config
// this setting will be ignored if use_tls_routing=true
variable "enable_mongodb_listener" {
  type    = bool
  default = false
}

// whether to enable the mysql listener
// adds security group setting, maps load balancer to port, and adds to teleport config
// this setting will be ignored if use_tls_routing=true
variable "enable_mysql_listener" {
  type    = bool
  default = false
}

// whether to enable the postgres listener
// adds security group setting, maps load balancer to port, and adds to teleport config
// this setting will be ignored if use_tls_routing=true
variable "enable_postgres_listener" {
  type    = bool
  default = false
}

// Email for Let's Encrypt domain registration
variable "email" {
  type = string
}

// S3 Bucket to create for encrypted Let's Encrypt certificates
variable "s3_bucket_name" {
  type = string
}

// AWS KMS alias used for encryption/decryption
// default is alias used in SSM
variable "kms_alias_name" {
  type    = string
  default = "alias/aws/ssm"
}

// path to teleport enterprise/pro license file
variable "license_path" {
  type    = string
  default = ""
}

// AMI name to use
variable "ami_name" {
  type = string
}

// DynamoDB autoscale parameters
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

// Whether to use Amazon-issued certificates via ACM or not
// This must be set to true for any use of ACM whatsoever, regardless of whether Terraform generates/approves the cert
variable "use_acm" {
  type    = bool
  default = false
}

// Whether to enable TLS routing in the cluster
// See https://goteleport.com/docs/architecture/tls-routing for more information
// Setting this will disable ALL separate listener ports. If you also use ACM, then:
// - you must use Teleport and tsh v13+
// - you must use `tsh proxy` commands for Kubernetes/database access
variable "use_tls_routing" {
  type    = bool
  default = false
}

// CIDR blocks allowed to connect to the bastion SSH port
variable "allowed_bastion_ssh_ingress_cidr_blocks" {
  type    = list(any)
  default = ["0.0.0.0/0"]
}

// CIDR blocks allowed for egress from bastion
variable "allowed_bastion_ssh_egress_cidr_blocks" {
  type    = list(any)
  default = ["0.0.0.0/0"]
}

// CIDR blocks allowed for ingress for Teleport Proxy ports
variable "allowed_proxy_ingress_cidr_blocks" {
  type    = list(any)
  default = ["0.0.0.0/0"]
}

// CIDR blocks allowed for egress from Teleport Proxies
variable "allowed_proxy_egress_cidr_blocks" {
  type    = list(any)
  default = ["0.0.0.0/0"]
}

// CIDR blocks allowed for egress from Teleport Auth servers
variable "allowed_auth_egress_cidr_blocks" {
  type    = list(any)
  default = ["0.0.0.0/0"]
}

// CIDR blocks allowed for egress from Teleport Node
variable "allowed_node_egress_cidr_blocks" {
  type    = list(any)
  default = ["0.0.0.0/0"]
}

// Internet gateway destination CIDR Block
variable "internet_gateway_dest_cidr_block" {
  type    = string
  default = "0.0.0.0/0"
}

// Route allowed for Auth Servers Destination CIDR Block
variable "auth_aws_route_dest_cidr_block" {
  type    = string
  default = "0.0.0.0/0"
}

// Route allowed for Node Servers Destination CIDR Block
variable "node_aws_route_dest_cidr_block" {
  type    = string
  default = "0.0.0.0/0"
}

// Optional domain name to use for Teleport proxy NLB alias
// Only applied when using ACM, it will do nothing when ACM is disabled
// Only applied when _not_ using TLS routing, it will do nothing when TLS routing is enabled
// When using ACM we have one ALB (for port 443 with TLS termination) and one NLB
// (for all other traffic - 3023/3024/3026 etc)
// As this NLB is at a different address, we add an alias record in Route 53 so that
// it can be used by applications which connect to it directly (like kubectl) rather
// than discovering the NLB's address through the Teleport API (like tsh does)
variable "route53_domain_acm_nlb_alias" {
  type    = string
  default = ""
}

// (optional) Change the default authentication type used for the Teleport cluster.
// See https://goteleport.com/docs/reference/authentication for more information.
// This is useful for persisting a different default authentication type across AMI upgrades when you have a SAML, OIDC
// or GitHub connector configured in DynamoDB. The default if not set is "local".
// Teleport Community Edition supports "local" or "github"
// Teleport Enterprise Edition supports "local", "github", "oidc", or "saml"
// Teleport Enterprise FIPS deployments have local authentication disabled, so should use "github", "oidc", or "saml"
variable "teleport_auth_type" {
  type    = string
  default = "local"
}

// (optional) Change the default tags applied to all resources.
variable "default_tags" {
  type    = map(string)
  default = {}
}

// Whether to trigger instance refresh rollout for Teleport Auth servers when
// servers when the launch template or configuration changes.
// Enable this with caution - upgrading Teleport version will trigger an
// instance refresh and auth servers must be scaled down to only one instance
// before upgrading your Teleport cluster.
variable "enable_auth_asg_instance_refresh" {
  type    = bool
  default = false
}

// Whether to trigger instance refresh rollout for Teleport Proxy servers when
// servers when the launch template or configuration changes.
variable "enable_proxy_asg_instance_refresh" {
  type    = bool
  default = false
}

// Whether to trigger instance refresh rollout for Teleport Node servers when
// servers when the launch template or configuration changes.
variable "enable_node_asg_instance_refresh" {
  type    = bool
  default = false
}
