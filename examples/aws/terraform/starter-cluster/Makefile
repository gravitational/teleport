# Set up terraform variables in a separate environment file, or inline here

# This region should support EFS
TF_VAR_region ?=

# Cluster name is a unique cluster name to use, should be unique and not contain spaces or other special characters
TF_VAR_cluster_name ?=

# AWS SSH key name to provision in installed instances, should be available in the region
TF_VAR_key_name ?=

# Full absolute path to the license file for Teleport Enterprise or Pro.
# This license will be copied into SSM and then pulled down on the auth nodes to enable Enterprise/Pro functionality
TF_VAR_license_path ?=

# AMI name contains the version of Teleport to install, and whether to use OSS or Enterprise version
# These AMIs are published by Gravitational and shared as public whenever a new version of Teleport is released
# To list available AMIs:
# OSS: aws ec2 describe-images --filters 'Name=name,Values=gravitational-teleport-ami-oss*'
# Enterprise: aws ec2 describe-images --filters 'Name=name,Values=gravitational-teleport-ami-ent*'
TF_VAR_ami_name ?=

# Route 53 zone to use, should be the zone registered in AWS, e.g. example.com
TF_VAR_route53_zone ?=

# Subdomain to set up in the zone above, e.g. cluster.example.com
# This will be used for internet access for users connecting to teleport proxy
TF_VAR_route53_domain ?=

# Set to true to add a wildcard subdomain entry to point to the proxy, e.g. *.cluster.example.com
# This is used to enable Teleport Application Access
TF_VAR_add_wildcard_route53_record ?= true

# Enable adding MongoDB listeners in Teleport proxy, load balancer ports and security groups
TF_VAR_enable_mongodb_listener ?= true

# Enable adding MySQL listeners in Teleport proxy, load balancer ports and security groups
TF_VAR_enable_mysql_listener ?= true

# Enable adding Postgres listeners in Teleport proxy, load balancer ports and security groups
TF_VAR_enable_postgres_listener ?= true

# Bucket name to store encrypted letsencrypt certificates.
TF_VAR_s3_bucket_name ?=

# Email of your support org, used for Letsencrypt cert registration process.
TF_VAR_email ?=

# Set to true to use LetsEncrypt to provision certificates
TF_VAR_use_letsencrypt ?=true

# Set to true to use ACM (Amazon Certificate Manager) to provision certificates
# If you wish to use a pre-existing ACM certificate rather than having Terraform generate one for you, you can import it:
# terraform import aws_acm_certificate.cert <certificate_arn>
TF_VAR_use_acm ?=false

export

# Plan launches terraform plan
.PHONY: plan
plan:
	terraform init
	terraform plan

# Apply launches terraform apply
.PHONY: apply
apply:
	terraform init
	terraform apply

# Destroy deletes the provisioned resources
.PHONY: destroy
destroy:
	terraform init
	terraform destroy
