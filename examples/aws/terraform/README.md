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

## Public Teleport AMI IDs

For your convenience, this is a list of public Teleport AMI IDs which are published by Gravitational.

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