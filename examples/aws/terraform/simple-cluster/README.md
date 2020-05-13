## Teleport Terraform AWS AMI Simple Example

This is a simple Terraform example to get you started provisioning an all-in-one Teleport cluster (auth, node, proxy) on a single ec2 instance based on Gravitational's pre-built AMI. 

### How does this work?
Teleport AMIs are built so you only need to specify environment variables to bring a fully configured instance online. See `data.tpl` or our [documentation](https://gravitational.com/teleport/docs/aws_oss_guide/#single-oss-teleport-amis-manual-gui-setup) to learn more about supported environment variables. 

A series of systemd [units](https://github.com/gravitational/teleport/tree/master/assets/marketplace/files/system) bootstrap the instance, via several bash [scripts](https://github.com/gravitational/teleport/tree/master/assets/marketplace/files/bin). 

While this may not be sufficient for all use cases, it's a great proof of concept that you can fork and customize to your liking. Check out our AWS AMI [generation code](https://github.com/gravitational/teleport/tree/master/assets/marketplace) if you're interested in adapting this to your requirements.

This Terraform example will configure the following AWS resources:
- Teleport all-in-one (auth, node, proxy) single cluster ec2 instance
- DynamoDB tables (cluster state, cluster events, ssl lock)
- S3 bucket (session recording storage)
- Route53 A record
- Security Groups and IAM roles

### Instructions

#### Build Requirements
- terraform v0.12+ ([install docs](https://learn.hashicorp.com/terraform/getting-started/install.html))
- awscli v1.14+ ([install docs](https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-install.html))

#### Usage

- `make plan` and verify the plan is building what you expect. 
- `make apply` to begin provisioning. 
- `make destroy` to delete the provisioned resources.

#### Steps
Update the included Makefile to define your configuration. 
1. Run `make apply`. 
1. SSH to your new instance. `ssh ec2-user@<cluster_domain>`. 
1. Run `tctl users add <username> ec2-user` (this will create a Teleport User and permit login as the local ec2-user)
1. Click the registration link provided by the output. Set a password and configure your 2fa token. 
1. Success! You've configured a fully functional Teleport cluster. 

```bash
# Set variables for Terraform

# Region to run in - we currently have AMIs in the following regions:
# ap-south-1,ap-northeast-2,ap-southeast-1,ap-southeast-2,ap-northeast-1,ca-central-1,eu-central-1,eu-west-1,eu-west-2
# sa-east-1,us-east-1,us-east-2,us-west-1,us-west-2
export TF_VAR_region="us-west-2"

# Cluster name is a unique cluster name to use, should be unique and not contain spaces or other special characters
export TF_VAR_cluster_name="teleport.example.com"

# AMI name contains the version of Teleport to install, and whether to use OSS or Enterprise version
# These AMIs are published by Gravitational and shared as public whenever a new version of Teleport is released
# To list available AMIs:
# OSS: aws ec2 describe-images --owners 126027368216 --filters 'Name=name,Values=gravitational-teleport-ami-oss*'
# Enterprise: aws ec2 describe-images --owners 126027368216 --filters 'Name=name,Values=gravitational-teleport-ami-ent*'
# FIPS 140-2 images are also available for Enterprise customers, look for '-fips' on the end of the AMI's name
export TF_VAR_ami_name="gravitational-teleport-ami-ent-4.2.3"

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
export TF_VAR_grafana_pass="CHANGE_THIS_VALUE"

# (optional) Set to true to use ACM (Amazon Certificate Manager) to provision certificates rather than Letsencrypt
# If you wish to use a pre-existing ACM certificate rather than having Terraform generate one for you, you can import it:
# terraform import aws_acm_certificate.cert <certificate_arn>
# export TF_VAR_use_acm="false"

# plan
make plan
```


#### Project layout

|File|Description|
|---|---|
|cluster.tf|EC2 instance template and provisioning.|
|cluster_iam.tf|IAM role provisioning. Permits ec2 instance to talk to AWS resources (ssm, s3, dynamodb, etc)|
|cluster_sg.tf|Security Group provisioning. Ingress network rules.|
|data.tf|Misc variables used for provisioning AWS resources.|
|data.tpl|Template for Teleport configuration.|
|dynamo.tf|DynamoDB table provisioning. Tables used for Teleport state and events.|
|route53.tpl|Route53 zone creation. Requires a hosted zone to configure SSL.|
|s3.tf|S3 bucket provisioning. Bucket used for session recording storage.|
|ssm.tf|Teleport license distribution (if using Teleport enterprise).|
|vars.tf|Inbound variables for Teleport configuration.|
