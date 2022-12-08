#!/usr/bin/bash

# Set variables for Terraform

# Region to run in - we currently have AMIs in the following regions:
# ap-south-1,ap-northeast-2,ap-southeast-1,ap-southeast-2,ap-northeast-1,ca-central-1,eu-central-1,eu-west-1,eu-west-2
# sa-east-1,us-east-1,us-east-2,us-west-1,us-west-2
export TF_VAR_region="us-east-1"

# Cluster name is a unique cluster name to use, should be unique and not contain spaces or other special characters
export TF_VAR_cluster_name="hugo-tf"

# AMI name contains the version of Teleport to install, and whether to use OSS or Enterprise version
# These AMIs are published by Gravitational and shared as public whenever a new version of Teleport is released
# To list available AMIs:
# OSS: aws ec2 describe-images --owners 126027368216 --filters 'Name=name,Values=gravitational-teleport-ami-oss*'
# Enterprise: aws ec2 describe-images --owners 126027368216 --filters 'Name=name,Values=gravitational-teleport-ami-ent*'
# FIPS 140-2 images are also available for Enterprise customers, look for '-fips' on the end of the AMI's name
# export TF_VAR_ami_name="gravitational-teleport-ami-oss-10.0.2"
export TF_VAR_ami_name="teleport-debug-ami-oss-10.1.2"

# AWS SSH key name to provision in installed instances, should be available in the region
export TF_VAR_key_name="hugo-ed25519-us-east-1"

# (optional) Set to true to use ACM (Amazon Certificate Manager) to provision certificates rather than LetsEncrypt
# If you wish to use a pre-existing ACM certificate rather than having Terraform generate one for you, you can import it:
# Terraform import aws_acm_certificate.cert <certificate_arn>
export TF_VAR_use_acm="false"

# Full absolute path to the license file for Teleport Enterprise or Pro.
# This license will be copied into SSM and then pulled down on the auth nodes to enable Enterprise/Pro functionality
export TF_VAR_license_path="/tmp/tf/coucou"

# Route 53 zone to use, should be the zone registered in AWS, e.g. example.com
export TF_VAR_route53_zone="teleportdemo.net"

# Subdomain to set up in the zone above, e.g. cluster.example.com
# This will be used for internet access for users connecting to teleport proxy
export TF_VAR_route53_domain="hugo-tf.teleportdemo.net"

# Set to true to add a wildcard subdomain entry to point to the proxy, e.g. *.cluster.example.com
# This is used to enable Teleport Application Access
export TF_VAR_add_wildcard_route53_record="true"

# Enable adding MongoDB listeners in Teleport proxy, load balancer ports and security groups
export TF_VAR_enable_mongodb_listener="true"

# Enable adding MySQL listeners in Teleport proxy, load balancer ports and security groups
export TF_VAR_enable_mysql_listener="true"

# Enable adding Postgres listeners in Teleport proxy, load balancer ports and security groups
export TF_VAR_enable_postgres_listener="true"

# (optional) If using ACM, set an additional DNS alias which will be added pointing to the NLB. This can
# be used with clients like kubectl which should target a DNS record. This will also add the DNS name to the
# Teleport Kubernetes config to prevent certificate SNI issues. You can use this DNS name with commands like:
# `tctl auth sign --user=foo --format=kubernetes --out=kubeconfig --proxy=https://cluster-nlb.example.com:3026`
# This setting only takes effect when using ACM, it will be ignored otherwise.
#export TF_VAR_route53_domain_acm_nlb_alias="cluster-nlb.example.com"

# Bucket name to store encrypted letsencrypt certificates.
export TF_VAR_s3_bucket_name="hugo-tf.teleportdemo.net"

# Email of your support org, used for Letsencrypt cert registration process.
export TF_VAR_email="hugo.hervieux@goteleport.com"

# Setup grafana password for "admin" user. Grafana will be served on https://cluster.example.com:8443 after install
export TF_VAR_grafana_pass="eezei6aitoovooyieLu6"
