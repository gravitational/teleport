## Terraform based provisioning example (Amazon single AMI)

Terraform specifies example provisioning script for Teleport auth, proxy and nodes in HA mode.

Use these examples as possible deployment patterns suggested by Teleport developers.

The scripts set up Letsencrypt certificates using DNS-01 challenge. This means that users have to control the DNS zone
via Route 53. ACM can optionally be used too, but Route 53 integration is still required. 

Teleport join tokens are distributed using SSM parameter store, and certificates are distributed using encrypted S3
bucket.

There are a couple of tricks using DynamoDB locking to make sure there is only one auth server node rotating join token
at a time, but those could be easilly replaced and are not critical for performance.

Important bits are that auth servers and proxies are not running as root and are secured exposing absolute minimum of
the ports to the other parts.

```bash
# Set variables for Terraform

# This region should support EFS
export TF_VAR_region="us-west-2"

# Cluster name is a unique cluster name to use, should be unique and not contain spaces or other special characters
export TF_VAR_cluster_name="teleport.example.com"

# AMI name contains the version of Teleport to install, and whether to use OSS or Enterprise version
# These AMIs are published by Gravitational and shared as public whenever a new version of Teleport is released
# To list available AMIs:
# OSS: aws ec2 describe-images --owners 126027368216 --filters 'Name=name,Values=gravitational-teleport-ami-oss*'
# Enterprise: aws ec2 describe-images --owners 126027368216 --filters 'Name=name,Values=gravitational-teleport-ami-ent*'
export TF_VAR_ami_name="gravitational-teleport-ami-ent-3.1.7"

# AWS SSH key name to provision in installed instances, should be available in the region
export TF_VAR_key_name="example"

# Full absolute path to the license file for Teleport Enterprise or Pro.
# This license will be copied into SSM and then pulled down on the auth nodes to enable Enterprise/Pro functionality
export TF_VAR_license_path="/path/to/license"

# Route 53 zone to use, should be the zone registered in AWS, e.g. example.com
export TF_VAR_route53_zone="example.com"

# Subdomain to set up in the zone above, e.g. cluster.example.com
# This will be used for internet access for users connecting to teleport proxy
export TF_VAR_route53_domain="cluster.example.com"

# Bucket name to store encrypted letsencrypt certificates.
export TF_VAR_s3_bucket_name="teleport.example.com"

# Email of your support org, used for Letsencrypt cert registration process.
export TF_VAR_email="support@example.com"

# Setup grafana password for "admin" user. Grafana will be served on https://cluster.example.com:8443 after install
export TF_VAR_grafana_pass="setup some password here"

# (optional) Set to true to use ACM (Amazon Certificate Manager) to provision certificates rather than Letsencrypt
# If you wish to use a pre-existing ACM certificate rather than having Terraform generate one for you, you can import it:
# terraform import aws_acm_certificate.cert <certificate_arn>
# export TF_VAR_use_acm="false"

# plan
make plan
```