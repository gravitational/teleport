# load license from file in local directory
data "local_file" "license" {
    filename = "/home/gus/downloads/teleport/license-gus.pem"
}

# alternatvely, load the license from a variable and write the file locally
# resource "local_file" "license" {
#   sensitive_content = var.teleport_license
#   filename          = "/tmp/license.pem"
# }

# create license resource (which the module depends on)
resource "local_file" "license" {
    sensitive_content = data.local_file.license.content
    filename = "${path.module}/license.pem"
}

module "teleport-ha-autoscale-cluster" {
  # source
  source = "./ha-autoscale-cluster"

  # the license file must be created first, because the module needs to load it
  depends_on = [local_file.license]

  # Teleport cluster name to set up
  # This cannot be changed later, so pick something descriptive
  cluster_name = "gus-tfmodulev3"

  # SSH key name to provision instances with
  # This must be a key that already exists in the AWS account
  key_name = "gus"

  # AMI ID to use
  # See https://github.com/gravitational/teleport/blob/master/examples/aws/terraform/AMIS.md
  ami_id = "ami-05bc569710d7e6cda"

  # Account ID which owns the AMIs used to spin up instances
  # You should only need to set this if you're building your own AMIs for testing purposes.
  ami_owner_account_id = "278576220453"

  # Password for Grafana admin user
  # Grafana is hosted on https://<route53_domain>:8443
  grafana_pass = "this-is-the-grafana-password"

  # Whether to use Amazon-issued certificates via ACM or not
  # This must be set to true for any use of ACM whatsoever, regardless of whether Terraform generates/approves the cert
  use_acm = "true"

  # List of AZs to spawn auth/proxy instances in
  # e.g. ["us-east-1a", "us-east-1d"]
  # This must match the region specified in your provider.tf file
  az_list = ["us-east-1c", "us-east-1d"]

  # CIDR to use in the VPC that the module creates
  # This must be at least a /16
  vpc_cidr = "10.10.0.0/16"

  # Zone name which will host DNS records, e.g. example.com
  # This must already be configured in Route 53
  route53_zone = "teleportdemo.net"

  # Domain name to use for Teleport proxies, e.g. proxy.example.com
  # This will be the domain that Teleport users will connect to via web UI or the tsh client
  route53_domain = "gus-tfmodulev3.teleportdemo.net"

  # Optional domain name to use for Teleport proxy NLB alias
  # When using ACM we have one ALB (for port 443 with TLS termination) and one NLB
  # (for all other traffic - 3023/3024/3026 etc)
  # As this NLB is at a different address, we add an alias record in Route 53 so that
  # it can be used by applications which connect to it directly (like kubectl) rather
  # than discovering the NLB's address through the Teleport API (like tsh does)
  route53_domain_acm_nlb_alias = "gus-tfmodulev3-nlb.teleportdemo.net"

  # Email for LetsEncrypt domain registration
  email = "gus@goteleport.com"

  # S3 bucket to create for encrypted LetsEncrypt certificates
  # This is also used for storing the Teleport license which is downloaded to auth servers
  # This cannot be a pre-existing bucket
  s3_bucket_name = "gus-tfmodulev3.teleportdemo.net"

  # Path to Teleport Enterprise license file
  license_path = local_file.license.filename

  # Instance type used for auth autoscaling group
  auth_instance_type = "t3.micro"

  # Instance type used for proxy autoscaling group
  proxy_instance_type = "t3.micro"

  # Instance type used for node autoscaling group
  node_instance_type = "t3.micro"

  # Instance type used for monitor autoscaling group
  monitor_instance_type = "t3.micro"

  # AWS KMS alias used for encryption/decryption, defaults to alias used in SSM
  kms_alias_name = "alias/aws/ssm"

  # DynamoDB autoscaling parameters
  autoscale_write_target = 50
  autoscale_read_target = 50
  autoscale_min_read_capacity = 5
  autoscale_max_read_capacity = 100
  autoscale_min_write_capacity = 5
  autoscale_max_write_capacity = 100

  # Default auth type to use on Teleport cluster
  # Useful when you have SAML or OIDC connectors configured in DynamoDB and want to relaunch instances with a new AMI
  auth_type = "local"
}
