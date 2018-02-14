# AWS provisioning examples

## Terraform provisioning example

Terraform specifies example provisioning script
for Teleport auth, proxy and nodes in HA mode.

Use these examples as possible deployment patterns suggested
by Teleport developers.

Scripts set up letsencrypt certificates using DNS-01 challenge.
This means users have to control DNS zone via route53.

Teleport join tokens are distributed using SSM parameter store,
and certificates are distributed using encrypted S3 bucket.

There are a couple of tricks using DynamoDB locking to make sure
there is only one auth server node rotating join token at a time,
but those could be easilly replaced and are not critical for performance.

Important bits are that auth servers and proxes are not running as root
and are secured exposing absolute minimum of the ports to the other parts.

```bash
# Set variables for Terraform

# This region should support EFS
export TF_VAR_region="us-west-2"

# Cluster name is a unique cluster name to use, should be unique
# and do not contain dots, spaces, and other special characters
export TF_VAR_cluster_name=cluster

# Teleport version to install, e.g. 2.4.0
export TF_VAR_teleport_version="2.5.0-alpha.5"

# AWS SSH key name to provision in installed instances, should be available in the region
export TF_VAR_key_name="example"

# Full absolute path to the license file for Teleport enterprise or pro.
# To get a demo license, subscribe here (it is free):
# https://dashboard.gravitational.com/web/signup?plan=teleport-pro
# You can modify the script to use OSS version to download from github instead.
export TF_VAR_license_path="/path/to/license"

# AMI name to use, could be public or private. Make sure to accept TOS ageement for this AMI
# before launching this terraform script or it will not work.
export TF_VAR_ami_name="debian-stretch-hvm-x86_64-gp2-2018-01-06-16218-572488bb-fc09-4638-8628-e1e1d26436f4-ami-628ad918.4"

# Route 53 zone to use, should be the zone registered in AWS,
# e.g. example.com
export TF_VAR_route53_zone="example.com"

# Subdomain to set up in the zone above, e.g. cluster.example.com
# this will be used for internet access for users connecting to teleport proxy
export TF_VAR_route53_domain="cluster.example.com"

# Bucket name to store encrypted letsencrypt certificates.
export TF_VAR_s3_bucket_name="teleport.example.com"

# Email of your support org, uset for letsencrypt cert registration process.
export TF_VAR_email="support@example.com"

# Setup grafana password for "admin" user. Grafana will
# get served on https://teleport.example.com:8443 after install
export TF_VAR_grafana_pass="setup some password here"

# plan
make plan
```
