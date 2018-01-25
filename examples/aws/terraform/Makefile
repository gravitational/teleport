# Set up terraform variables in a separate environment file, or inline here
# This region should support EFS
TF_VAR_region ?= us-west-2
# Cluster name is a unique cluster name to use, better to use FQDN, e.g. cluster.example.com
TF_VAR_cluster_name ?=
# Teleport version to install, e.g. 2.4.0
TF_VAR_teleport_version ?=
# AWS SSH key name to provision in installed instances, should be available in the region
TF_VAR_key_name ?=
# Full absolute path to the license file for Teleport enterprise or pro
TF_VAR_license_path ?=
# AMI name to use, could be public or private
TF_VAR_ami_name ?=
# Route 53 zone to use, should be the zone registered in AWS,
# e.g. example.com
TF_VAR_route53_zone ?=
# Subdomain to set up in the zone above, e.g. cluster.example.com
# this will be used for internet access for users connecting to teleport proxy
TF_VAR_route53_domain ?=
# Bucket name to store encrypted letsencrypt certificates.
TF_VAR_s3_bucket_name ?=
# Email of your support org, uset for letsencrypt cert registration process.
TF_VAR_email ?=
# Setup grafana password for "admin" user. Grafana will
# get served on https://cluster.example.com:8443 after install
TF_VAR_grafana_pass ?=
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


# Destroy destroys the infrastructure
.PHONY: destroy
destroy:
	terraform init
	terraform destroy
