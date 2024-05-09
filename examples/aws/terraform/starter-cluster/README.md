# Teleport Terraform AWS AMI Simple Example

This is a simple Terraform example to get you started provisioning an all-in-one Teleport cluster (auth, node, proxy) on a single ec2 instance based on Teleport's pre-built AMI.

Do not use this in production! This example should be used for demo, proof-of-concept, or learning purposes only.

## How does this work?

Teleport AMIs are built so you only need to specify environment variables to bring a fully configured instance online. See `data.tpl` or our [documentation](https://goteleport.com/docs/deploy-a-cluster/deployments/aws-terraform/#set-up-variables) to learn more about supported environment variables.

A series of systemd [units](https://github.com/gravitational/teleport/tree/master/assets/aws/files/system) bootstrap the instance, via several bash [scripts](https://github.com/gravitational/teleport/tree/master/assets/aws/files/bin).

While this may not be sufficient for all use cases, it's a great proof-of-concept that you can fork and customize to your liking. Check out our AWS AMI [generation code](https://github.com/gravitational/teleport/tree/master/assets/aws) if you're interested in adapting this to your requirements.

This Terraform example will configure the following AWS resources:

- Teleport all-in-one (auth, node, proxy) single cluster ec2 instance
- DynamoDB tables (cluster state, cluster events, ssl lock)
- S3 bucket (session recording storage)
- Route53 `A` record
- Security Groups and IAM roles

It can optionally also configure the following AWS resources:

- Application Load Balancer
- ACM certificate and validation via Route53

## Instructions

### Accompanying documentation

- [Teleport Single-Instance Deployment on AWS](https://goteleport.com/docs/deploy-a-cluster/deployments/aws-starter-cluster-terraform/)

### Build Requirements

- terraform v1.0+ [install docs](https://learn.hashicorp.com/tutorials/terraform/install-cli)
- awscli v1.14+ [install docs](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html)

### Usage

- `make plan` and verify the plan is building what you expect.
- `make apply` to begin provisioning.
- `make destroy` to delete the provisioned resources.

### Project layout

File           | Description
-------------- | ---------------------------------------------------------------------------------------------
acm.tf         | ACM certificate setup and DNS record.
cluster.tf     | EC2 instance template and provisioning.
cluster_iam.tf | IAM role provisioning. Permits ec2 instance to talk to AWS resources (ssm, s3, dynamodb, etc)
cluster_lb.tf  | Application load balancer setup (when using ACM).
cluster_sg.tf  | Security Group provisioning. Ingress network rules.
data.tf        | Misc variables used for provisioning AWS resources.
data.tpl       | Template for Teleport configuration.
dynamo.tf      | DynamoDB table provisioning. Tables used for Teleport state and events.
outputs.tf     | Terraform outputs, used to get cluster information.
route53.tpl    | Route53 zone creation. Requires a hosted zone to configure SSL.
s3.tf          | S3 bucket provisioning. Bucket used for session recording storage.
ssm.tf         | Teleport license distribution (if using Teleport enterprise).
vars.tf        | Inbound variables for Teleport configuration.

### Steps

Update the included Makefile to define your configuration.

1. Run `make apply`.
2. SSH to your new instance. `ssh ec2-user@<cluster_domain>`.
3. Create a user (this will create a Teleport User and permit login as the local ec2-user).
   - OSS:
   `sudo tctl users add <username> --roles=access,editor --logins=ec2-user`
   - Enterprise:
    `tctl users add --roles=access,editor <username> --logins=ec2-user`
4. Click the registration link provided by the output. Set a password and configure your 2fa token.
5. Success! You've configured a fully functional Teleport cluster.

```bash
# Set up Terraform variables in a separate environment file, or inline here

# Region to run in - we currently have AMIs in the following regions:
# ap-northeast-1, ap-northeast-2, ap-northeast-3, ap-south-1, ap-southeast-1, ap-southeast-2, ca-central-1, eu-central-1
# eu-north-1, eu-west-1, eu-west-2, eu-west-3, sa-east-1, us-east-1, us-east-2, us-west-1, us-west-2
TF_VAR_region ?= "us-west-2"

# Cluster name is a unique cluster name to use, should be unique and not contain spaces or other special characters
TF_VAR_cluster_name ?= "TeleportCluster1"

# AWS SSH key pair name to provision in installed instances, must be a key pair available in the above defined region (AWS Console > EC2 > Key Pairs)
TF_VAR_key_name ?= "example"

# Full absolute path to the license file, on the machine executing Terraform, for Teleport Enterprise.
# This license will be copied into AWS SSM and then pulled down on the auth nodes to enable Enterprise functionality
TF_VAR_license_path ?= "/path/to/license"

# AMI name contains the version of Teleport to install, and whether to use OSS or Enterprise version
# These AMIs are published by Teleport and shared as public whenever a new version of Teleport is released
# To list available AMIs:
# OSS: aws ec2 describe-images --owners 146628656107 --filters 'Name=name,Values=teleport-oss-*'
# Enterprise: aws ec2 describe-images --owners 146628656107 --filters 'Name=name,Values=teleport-ent-*'
# FIPS 140-2 images are also available for Enterprise customers, look for '-fips' on the end of the AMI's name
TF_VAR_ami_name ?= "teleport-ent-15.3.1-arm64"

# Route 53 hosted zone to use, must be a root zone registered in AWS, e.g. example.com
TF_VAR_route53_zone ?= "example.com"

# Subdomain to set up in the zone above, e.g. cluster.example.com
# This will be used for users connecting to Teleport proxy
TF_VAR_route53_domain ?= "cluster.example.com"

# Set to true to add a wildcard subdomain entry to point to the proxy, e.g. *.cluster.example.com
# This is used to enable Teleport Application Access
export TF_VAR_add_wildcard_route53_record="true"

# Enable adding MongoDB listeners in Teleport proxy, load balancer ports, and security groups
# This will be ignored if TF_VAR_use_tls_routing=true
export TF_VAR_enable_mongodb_listener="false"

# Enable adding MySQL listeners in Teleport proxy, load balancer ports, and security groups
# This will be ignored if TF_VAR_use_tls_routing=true
export TF_VAR_enable_mysql_listener="false"

# Enable adding Postgres listeners in Teleport proxy, load balancer ports, and security groups
# This will be ignored if TF_VAR_use_tls_routing=true
export TF_VAR_enable_postgres_listener="false"

# Bucket name to store Teleport session recordings.
export TF_VAR_s3_bucket_name="teleport.example.com"

# AWS instance type to provision for running this Teleport cluster
export TF_VAR_cluster_instance_type="t4g.micro"

# Email to be used for Let's Encrypt certificate registration process.
export TF_VAR_email="support@example.com"

# Set to true to use Let's Encrypt to provision certificates
export TF_VAR_use_letsencrypt="true"

# Set to true to use ACM (Amazon Certificate Manager) to provision certificates
# If you wish to use a pre-existing ACM certificate rather than having Terraform generate one for you, you can import it:
# terraform import aws_acm_certificate.cert <certificate_arn>
# Note that TLS routing is automatically enabled when using ACM with the starter-cluster Terraform, meaning:
# - you must use Teleport and tsh v13+
# - you must use `tsh proxy` commands for Kubernetes/database access
export TF_VAR_use_acm="false"

# Set to true to use TLS routing to multiplex all Teleport traffic over one port
# See https://goteleport.com/docs/architecture/tls-routing for more information
# Setting this will disable ALL separate listener ports.
# This setting is automatically set to "true" when using ACM with the starter-cluster Terraform
# and will be ignored.
export TF_VAR_use_tls_routing="true"

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

## Public Teleport AMI IDs

Please [see the AMIS.md file](../AMIS.md) for a list of public Teleport AMI IDs that you can use.
