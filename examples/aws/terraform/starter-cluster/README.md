# Teleport Terraform AWS AMI Simple Example

This is a simple Terraform example to get you started provisioning an all-in-one Teleport cluster (auth, node, proxy) on a single ec2 instance based on Gravitational's pre-built AMI.

Do not use this in production! This example should be used for demo, proof-of-concept, or learning purposes only.

## How does this work?

Teleport AMIs are built so you only need to specify environment variables to bring a fully configured instance online. See `data.tpl` or our [documentation](https://gravitational.com/teleport/docs/aws_oss_guide/#single-oss-teleport-amis-manual-gui-setup) to learn more about supported environment variables.

A series of systemd [units](https://github.com/gravitational/teleport/tree/master/assets/marketplace/files/system) bootstrap the instance, via several bash [scripts](https://github.com/gravitational/teleport/tree/master/assets/marketplace/files/bin).

While this may not be sufficient for all use cases, it's a great proof-of-concept that you can fork and customize to your liking. Check out our AWS AMI [generation code](https://github.com/gravitational/teleport/tree/master/assets/marketplace) if you're interested in adapting this to your requirements.

This Terraform example will configure the following AWS resources:

- Teleport all-in-one (auth, node, proxy) single cluster ec2 instance
- DynamoDB tables (cluster state, cluster events, ssl lock)
- S3 bucket (session recording storage)
- Route53 `A` record
- Security Groups and IAM roles

## Instructions

### Build Requirements

- terraform v0.12+ [install docs](https://learn.hashicorp.com/terraform/getting-started/install.html)
- awscli v1.14+ [install docs](https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-install.html)

### Usage

- `make plan` and verify the plan is building what you expect.
- `make apply` to begin provisioning.
- `make destroy` to delete the provisioned resources.

### Project layout

File           | Description
-------------- | ---------------------------------------------------------------------------------------------
cluster.tf     | EC2 instance template and provisioning.
cluster_iam.tf | IAM role provisioning. Permits ec2 instance to talk to AWS resources (ssm, s3, dynamodb, etc)
cluster_sg.tf  | Security Group provisioning. Ingress network rules.
data.tf        | Misc variables used for provisioning AWS resources.
data.tpl       | Template for Teleport configuration.
dynamo.tf      | DynamoDB table provisioning. Tables used for Teleport state and events.
route53.tpl    | Route53 zone creation. Requires a hosted zone to configure SSL.
s3.tf          | S3 bucket provisioning. Bucket used for session recording storage.
ssm.tf         | Teleport license distribution (if using Teleport enterprise).
vars.tf        | Inbound variables for Teleport configuration.

### Steps

Update the included Makefile to define your configuration.

1. Run `make apply`.
2. SSH to your new instance. `ssh ec2-user@<cluster_domain>`.
3. Run `tctl users add <username> ec2-user` (this will create a Teleport User and permit login as the local ec2-user)
4. Click the registration link provided by the output. Set a password and configure your 2fa token.
5. Success! You've configured a fully functional Teleport cluster.

```bash
# Set up Terraform variables in a separate environment file, or inline here

# Region to run in - we currently have AMIs in the following regions:
# ap-south-1, ap-northeast-2, ap-southeast-1, ap-southeast-2, ap-northeast-1, ca-central-1, eu-central-1, eu-west-1, eu-west-2
# sa-east-1, us-east-1, us-east-2, us-west-1, us-west-2
TF_VAR_region ?="us-east-1"

# Cluster name is a unique cluster name to use, should be unique and not contain spaces or other special characters
TF_VAR_cluster_name ?="TeleportCluster1"

# AWS SSH key pair name to provision in installed instances, must be a key pair available in the above defined region (AWS Console > EC2 > Key Pairs)
TF_VAR_key_name ?="example"

# Full absolute path to the license file, on the machine executing Terraform, for Teleport Enterprise.
# This license will be copied into AWS SSM and then pulled down on the auth nodes to enable Enterprise functionality
TF_VAR_license_path ?="/path/to/license"

# AMI name contains the version of Teleport to install, and whether to use OSS or Enterprise version
# These AMIs are published by Gravitational and shared as public whenever a new version of Teleport is released
# To list available AMIs:
# OSS: aws ec2 describe-images --owners 126027368216 --filters 'Name=name,Values=gravitational-teleport-ami-oss*'
# Enterprise: aws ec2 describe-images --owners 126027368216 --filters 'Name=name,Values=gravitational-teleport-ami-ent*'
# FIPS 140-2 images are also available for Enterprise customers, look for '-fips' on the end of the AMI's name
TF_VAR_ami_name ?="gravitational-teleport-ami-ent-4.2.9"

# Route 53 hosted zone to use, must be a root zone registered in AWS, e.g. example.com
TF_VAR_route53_zone ?="example.com"

# Subdomain to set up in the zone above, e.g. cluster.example.com
# This will be used for users connecting to Teleport proxy
TF_VAR_route53_domain ?="cluster.example.com"

# Bucket name to store encrypted LetsEncrypt certificates.
TF_VAR_s3_bucket_name ?="teleport.example.com"

# Email to be used for LetsEncrypt certificate registration process.
TF_VAR_email ?="support@example.com"

# Set to true to use LetsEncrypt to provision certificates
TF_VAR_use_letsencrypt ?=true

# Set to true to use ACM (Amazon Certificate Manager) to provision certificates
# If you wish to use a pre-existing ACM certificate rather than having Terraform generate one for you, you can import it:
# terraform import aws_acm_certificate.cert <certificate_arn>
TF_VAR_use_acm ?=false

# plan
make plan
```

## Public Teleport AMI IDs

For your convenience, this is a list of public Teleport AMI IDs which are published by Gravitational. This list
is updated when new AMI versions are released.

### OSS

```
# ap-south-1 v4.3.5 OSS: ami-0d277d983018002ec
# ap-northeast-2 v4.3.5 OSS: ami-072f84faa9242a47e
# ap-southeast-1 v4.3.5 OSS: ami-02a7715ddf767c966
# ap-southeast-2 v4.3.5 OSS: ami-0dbdbc8c568567d80
# ap-northeast-1 v4.3.5 OSS: ami-047135319113ca54d
# ca-central-1 v4.3.5 OSS: ami-09e9fa154f5d7e676
# eu-central-1 v4.3.5 OSS: ami-06490e9cd8d95ba09
# eu-west-1 v4.3.5 OSS: ami-0453fc6afc07a4b34
# eu-west-2 v4.3.5 OSS: ami-0c93b69dc46ce70a2
# sa-east-1 v4.3.5 OSS: ami-0e77d1cbf2b80db47
# us-east-1 v4.3.5 OSS: ami-0a12d80becdd5d1a1
# us-east-2 v4.3.5 OSS: ami-02b4742f89960ce18
# us-west-1 v4.3.5 OSS: ami-0598f55e8dc41d652
# us-west-2 v4.3.5 OSS: ami-0d63a03d1519101b5
```

### Enterprise

```
# ap-south-1 v4.3.5 Enterprise: ami-09d50faa4da796ada
# ap-northeast-2 v4.3.5 Enterprise: ami-091d5e7bdfe387cb7
# ap-southeast-1 v4.3.5 Enterprise: ami-025f42e94bdeda91f
# ap-southeast-2 v4.3.5 Enterprise: ami-00ed65891b4770941
# ap-northeast-1 v4.3.5 Enterprise: ami-0a9bd0ec2aaa77ce9
# ca-central-1 v4.3.5 Enterprise: ami-0a4830e7882740ca6
# eu-central-1 v4.3.5 Enterprise: ami-0e77128f1392b2250
# eu-west-1 v4.3.5 Enterprise: ami-07e76616b360885fb
# eu-west-2 v4.3.5 Enterprise: ami-036bb80a85cc6acf7
# sa-east-1 v4.3.5 Enterprise: ami-0c68b2b86b2cc4898
# us-east-1 v4.3.5 Enterprise: ami-0e08f81f767c62ddd
# us-east-2 v4.3.5 Enterprise: ami-07925f0b4f361ab01
# us-west-1 v4.3.5 Enterprise: ami-0627bae15bd3b7fa1
# us-west-2 v4.3.5 Enterprise: ami-0212214f21f3d7a06
```

### Enterprise FIPS

```
# ap-south-1 v4.3.5 Enterprise FIPS: ami-0c2c57c0dc5ae7c46
# ap-northeast-2 v4.3.5 Enterprise FIPS: ami-0e5c3dc6104bf9dd9
# ap-southeast-1 v4.3.5 Enterprise FIPS: ami-0ae54863080227132
# ap-southeast-2 v4.3.5 Enterprise FIPS: ami-0fe72e5c60f6a63ae
# ap-northeast-1 v4.3.5 Enterprise FIPS: ami-0ec9f8014bb42e674
# ca-central-1 v4.3.5 Enterprise FIPS: ami-0c3ecff82cf3f5a97
# eu-central-1 v4.3.5 Enterprise FIPS: ami-05ef481c07e8cc7da
# eu-west-1 v4.3.5 Enterprise FIPS: ami-0b1bb1d0930b2cedb
# eu-west-2 v4.3.5 Enterprise FIPS: ami-0599548c2e586e9b2
# sa-east-1 v4.3.5 Enterprise FIPS: ami-022d9de7679681cd8
# us-east-1 v4.3.5 Enterprise FIPS: ami-0192074892ea1bca1
# us-east-2 v4.3.5 Enterprise FIPS: ami-094df13554a9ed48c
# us-west-1 v4.3.5 Enterprise FIPS: ami-00f70d17feb41e977
# us-west-2 v4.3.5 Enterprise FIPS: ami-010c200a8731ede9b
```
