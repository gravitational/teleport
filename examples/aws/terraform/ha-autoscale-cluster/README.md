## Terraform based provisioning example (Amazon single AMI)

Terraform specifies example provisioning script for Teleport auth, proxy and nodes in High Availability (HA) mode.

Use these examples as possible deployment patterns suggested by Teleport developers.

The scripts set up Let's Encrypt certificates using DNS-01 challenge. This means that users have to control the DNS
zone via Route 53. ACM can optionally be used too, but Route 53 integration is still required.

Teleport join tokens are distributed using SSM parameter store, and certificates are distributed using encrypted S3
bucket.

There are a couple of tricks using DynamoDB locking to make sure there is only one auth server node rotating join token
at a time, but those could be easily replaced and are not critical for performance.

Important bits are that auth servers and proxies are not running as root and are secured exposing absolute minimum of
the ports to the other parts.

## Prerequisites

We recommend familiarizing yourself with the following resources prior to reviewing our Terraform examples:

- [Teleport Architecture](https://goteleport.com/docs/architecture/overview/)
- [Admin Guide](https://goteleport.com/docs/management/admin/)
- [Running Teleport Enterprise in High Availability mode on AWS](https://goteleport.com/docs/deploy-a-cluster/deployments/aws-ha-autoscale-cluster-terraform/)

In order to spin up AWS resources using these Terraform examples, you need the following software:

- terraform v1.0+ [install docs](https://learn.hashicorp.com/tutorials/terraform/install-cli)
- awscli v1.14+ [install docs](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html)

```bash
# Set variables for Terraform

# Region to run in - we currently have AMIs in the following regions:
# ap-northeast-1, ap-northeast-2, ap-northeast-3, ap-south-1, ap-southeast-1, ap-southeast-2, ca-central-1, eu-central-1
# eu-north-1, eu-west-1, eu-west-2, eu-west-3, sa-east-1, us-east-1, us-east-2, us-west-1, us-west-2
export TF_VAR_region="us-west-2"

# Cluster name is a unique cluster name to use, should be unique and not contain spaces or other special characters
export TF_VAR_cluster_name="teleport.example.com"

# AMI name contains the version of Teleport to install, and whether to use OSS or Enterprise version
# These AMIs are published by Teleport (Gravitational) and shared as public whenever a new version of Teleport is released
# To list available AMIs:
# OSS: aws ec2 describe-images --owners 146628656107 --filters 'Name=name,Values=teleport-oss-*'
# Enterprise: aws ec2 describe-images --owners 146628656107 --filters 'Name=name,Values=teleport-ent-*'
# FIPS 140-2 images are also available for Enterprise customers, look for '-fips' on the end of the AMI's name
export TF_VAR_ami_name="teleport-ent-15.2.0-arm64"

# AWS SSH key name to provision in installed instances, should be available in the region
export TF_VAR_key_name="example"

# (optional) Set to true to use ACM (Amazon Certificate Manager) to provision certificates rather than Let's Encrypt
# If you wish to use a pre-existing ACM certificate rather than having Terraform generate one for you, you can import it:
# Terraform import aws_acm_certificate.cert <certificate_arn>
export TF_VAR_use_acm="false"

# (optional) Set to true to use TLS routing to multiplex all Teleport traffic over one port
# See https://goteleport.com/docs/architecture/tls-routing for more information
# Setting this will disable ALL separate listener ports. If you also use ACM, then:
# - you must use Teleport and tsh v13+
# - you must use `tsh proxy` commands for Kubernetes/database access
export TF_VAR_use_tls_routing="false"

# Full absolute path to the license file for Teleport Enterprise.
# This license will be copied into SSM and then pulled down on the auth nodes to enable Enterprise functionality
export TF_VAR_license_path="/path/to/license"

# Route 53 zone to use, should be the zone registered in AWS, e.g. example.com
export TF_VAR_route53_zone="example.com"

# Subdomain to set up in the zone above, e.g. cluster.example.com
# This will be used for internet access for users connecting to teleport proxy
export TF_VAR_route53_domain="cluster.example.com"

# Set to true to add a wildcard subdomain entry to point to the proxy, e.g. *.cluster.example.com
# This is used to enable Teleport Application Access
export TF_VAR_add_wildcard_route53_record="true"

# Enable adding MongoDB listeners in Teleport proxy, load balancer ports, and security groups
# This will be ignored if TF_VAR_use_tls_routing=true
export TF_VAR_enable_mongodb_listener="true"

# Enable adding MySQL listeners in Teleport proxy, load balancer ports, and security groups
# This will be ignored if TF_VAR_use_tls_routing=true
export TF_VAR_enable_mysql_listener="true"

# Enable adding Postgres listeners in Teleport proxy, load balancer ports, and security groups
# This will be ignored if TF_VAR_use_tls_routing=true
export TF_VAR_enable_postgres_listener="true"

# (optional) If using ACM, set an additional DNS alias which will be added pointing to the NLB. This can
# be used with clients like kubectl which should target a DNS record. This will also add the DNS name to the
# Teleport Kubernetes config to prevent certificate SNI issues. You can use this DNS name with commands like:
# `tctl auth sign --user=foo --format=kubernetes --out=kubeconfig --proxy=https://cluster-nlb.example.com:3026`
# This setting only takes effect when using ACM, it will be ignored otherwise.
# This setting only takes effect when TLS routing is _not_ enabled, it will be ignored otherwise.
#export TF_VAR_route53_domain_acm_nlb_alias="cluster-nlb.example.com"

# Bucket name to store encrypted Let's Encrypt certificates.
export TF_VAR_s3_bucket_name="teleport.example.com"

# Email of your support org, used for Let's Encrypt cert registration process.
export TF_VAR_email="support@example.com"

# This value can be used to change the default authentication type used for the Teleport cluster.
# See https://goteleport.com/docs/reference/authentication for more information.
# This is useful for persisting a different default authentication type across AMI upgrades when you have a SAML, OIDC
# or GitHub connector configured in DynamoDB. The default is "local".
# Teleport Community Edition supports "local" or "github"
# Teleport Enterprise Edition supports "local", "github", "oidc", or "saml"
# Teleport Enterprise FIPS deployments have local authentication disabled, so should use "github", "oidc", or "saml"
export TF_VAR_teleport_auth_type="local"

# plan
make plan
```

You can see the full list of variables supported in [`vars.tf`](vars.tf).

## Public Teleport AMI IDs

Please [see the AMIS.md file](../AMIS.md) for a list of public Teleport AMI IDs that you can use.
