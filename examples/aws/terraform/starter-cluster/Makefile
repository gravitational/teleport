# Set up terraform variables in a separate environment file, or inline here

# This region should support EFS
TF_VAR_region ?=

# Cluster name is a unique cluster name to use, should be unique and not contain spaces or other special characters
TF_VAR_cluster_name ?=

# AWS SSH key name to provision in installed instances, should be available in the region
TF_VAR_key_name ?=

# Full absolute path to the license file for Teleport Enterprise.
# This license will be copied into SSM and then pulled down on the auth nodes to enable Enterprise functionality
TF_VAR_license_path ?=

# AMI name contains the version of Teleport to install, and whether to use OSS or Enterprise version
# These AMIs are published by Teleport (Gravitational) and shared as public whenever a new version of Teleport is released
# To list available AMIs:
# OSS: aws ec2 describe-images --owners 146628656107 --filters 'Name=name,Values=teleport-oss-*'
# Enterprise: aws ec2 describe-images --owners 146628656107 --filters 'Name=name,Values=teleport-ent-*'
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
# This will be ignored if TF_VAR_use_tls_routing=true
TF_VAR_enable_mongodb_listener ?= false

# Enable adding MySQL listeners in Teleport proxy, load balancer ports and security groups
# This will be ignored if TF_VAR_use_tls_routing=true
TF_VAR_enable_mysql_listener ?= false

# Enable adding Postgres listeners in Teleport proxy, load balancer ports and security groups
# This will be ignored if TF_VAR_use_tls_routing=true
TF_VAR_enable_postgres_listener ?= false

# Bucket name to store Teleport session recordings.
TF_VAR_s3_bucket_name ?=

# AWS instance type to provision for running this Teleport cluster
TF_VAR_cluster_instance_type = t3.micro

# Email of your support org, used for Let's Encrypt cert registration process.
TF_VAR_email ?=

# Set to true to use Let's Encrypt to provision certificates
# Note: Let's Encrypt will be automatically disabled if using ACM
TF_VAR_use_letsencrypt ?= true

# Set to true to use ACM (Amazon Certificate Manager) to provision certificates
# If you wish to use a pre-existing ACM certificate rather than having Terraform generate one for you, you can import it:
# terraform import aws_acm_certificate.cert <certificate_arn>
# Note that TLS routing is automatically enabled when using ACM with the starter-cluster Terraform, meaning:
# - you must use Teleport and tsh v13+
# - you must use `tsh proxy` commands for Kubernetes/database access
TF_VAR_use_acm ?= false

# Set to true to use TLS routing to multiplex all Teleport traffic over one port
# See https://goteleport.com/docs/architecture/tls-routing for more information
# Setting this will disable ALL separate listener ports.
# This setting is automatically set to "true" when using ACM with the starter-cluster Terraform
# and will be ignored.
TF_VAR_use_tls_routing ?= true

# (optional) Change the default authentication type used for the Teleport cluster.
# See https://goteleport.com/docs/reference/authentication for more information.
# This is useful for persisting a different default authentication type across AMI upgrades when you have a SAML, OIDC
# or GitHub connector configured in DynamoDB. The default if not set is "local".
# Teleport Community Edition supports "local" or "github"
# Teleport Enterprise Edition supports "local", "github", "oidc", or "saml"
# Teleport Enterprise FIPS deployments have local authentication disabled, so should use "github", "oidc", or "saml"
TF_VAR_teleport_auth_type ?= "local"

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

# Destroy destroys the infrastructure, it doesn't ask for confirmation so be sure you actually want to
.PHONY: destroy-yes-i-want-to-do-this
destroy-yes-i-want-to-do-this:
	terraform init
	terraform destroy -auto-approve
