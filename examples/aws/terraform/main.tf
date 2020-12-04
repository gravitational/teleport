# load license from file in local directory
data "local_file" "license" {
    filename = "teleport-license.pem"
}

# create license resource (which the module depends on)
resource "local_file" "license" {
    content = data.local_file.license.content
    filename = "${path.module}/license.pem"
}

# optionally load license from variable
# resource "local_file" "license" {
#   sensitive_content = data.local_file.license.content
#   filename          = "/tmp/license.pem"
# }

module "teleport-ha-autoscale-cluster" {
  # remote source
  source = "./ha-autoscale-cluster"

  # the license file must be created first, because the module needs to load it
  depends_on = [local_file.license]

  # Required

  # DNS and letsencrypt integration variables
  # Zone name to host DNS record, e.g. example.com
  # Required, no default on module
  route53_zone = "gravitational.io"

  # Subdomain to set up in the zone above, e.g. cluster.example.com
  # This will be used for internet access for users connecting to teleport proxy
  # Required, no default on module
  route53_domain = "gus-tfmodule.gravitational.io"

  # Email for letsencrypt domain registration
  # Required, no default on module
  email = "gus@goteleport.com"

  # Region is AWS region, the region should support EFS
  # Required, no default on module
  region = "us-west-2"

  # path to teleport enterprise/pro license file
  license_path = local_file.license.filename

  # AMI name to use
  # Required, no default on module
  ami_name = "gravitational-teleport-ami-ent-5.0.0"

  # Password for grafana admin user
  # Required, no default on module
  grafana_pass = "this-is-the-grafana-password"

  # Whether to use Amazon-issued certificates via ACM or not
  # This must be set to true for any use of ACM whatsoever, regardless of whether Terraform generates/approves the cert
  # Required, no default on module
  use_acm = "false"

  # Defaults

  # Script creates a separate VPC with demo deployment
  vpc_cidr = "172.31.0.0/16"

  # Teleport cluster name to set up
  cluster_name = "gus-tfmodule"

  # Teleport UID is a UID for teleport user provisioned on the hosts
  teleport_uid = "1007"

  # Instance types used for authentication servers auto scale groups
  auth_instance_type = "t3.micro"

  # Instance types used for proxy auto scale groups
  proxy_instance_type = "t3.micro"

  # Instance types used for teleport nodes auto scale groups
  node_instance_type = "t3.micro"

  # Instance types used for monitor auto scale groups
  monitor_instance_type = "t3.micro"

  # SSH key name to provision instances with
  key_name = "gus"

  # S3 Bucket to create for encrypted letsencrypt certificates
  s3_bucket_name = "gus-tfmodule.gravitational.io"

  # AWS KMS alias used for encryption/decryption
  # default is alias used in SSM
  kms_alias_name = "alias/aws/ssm"

  # DynamoDB autoscale parameters
  autoscale_write_target = 50
  autoscale_read_target = 50
  autoscale_min_read_capacity = 10
  autoscale_max_read_capacity = 100
  autoscale_min_write_capacity = 10
  autoscale_max_write_capacity = 100
}
