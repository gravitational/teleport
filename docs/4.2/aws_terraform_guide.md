# Running Teleport Enterprise in a high-availiability configuration on AWS

This guide is designed to accompany our reference Terraform code (https://github.com/gravitational/teleport/tree/master/examples/aws/terraform) and describe how to use and administrate the resulting Teleport deployment.

## Prerequisites

Our code requires Terraform 0.12+. You can [download Terraform here](https://www.terraform.io/downloads.html). We will assume that you have `terraform` installed and available on your path.

```bash
$ terraform version
Terraform v0.12.20
```

You will also require the `aws` command line tool. This is available in Ubuntu/Debian/Fedora/CentOS
and MacOS Homebrew as the `awscli` package. 

When possible, installing via a package is always preferable. If you can't find a package available for
your distribution, you can install `python3` and `pip`, then use these to install `awscli` using `pip3 install awscli`

We will assume that you have configured your AWS cli access with credentials available at `~/.aws/credentials`:

```bash
$ cat ~/.aws/credentials
[default]
aws_access_key_id = AKIA....
aws_secret_access_key = 8ZRxy....
```

You should also have a default region set under `~/.aws/config`:

```bash
$ cat ~/.aws/config
[default]
region = us-east-1
```

As a result, you should be able to run a command like `aws ec2 describe-images --owners 126027368216 --filters 'Name=name,Values=gravitational-teleport-ami-ent*`
to list available EC2 images matching a pattern. If you get an "access denied" or similar message, you will need
to grant additional permissions to the AWS IAM user that your `aws_access_key_id` and `aws_secret_access_key` refers to.

# TODO(gus): detailed explanation of IAM permissions needed

## AWS Services required to run Teleport in HA

- [EC2 / Autoscale](#ec2-autoscale)
- [DynamoDB](#dynamodb)
- [S3](#s3://)
- [Route53](#route53)
- [NLB](#nlb-network-load-balancer)
- [IAM](#iam)
- [ACM](#acm)
- [SSM](#aws-systems-manager-parameter-store)


## Get the Terraform code

Firstly, you'll need to clone the Teleport repo to get the Terraform code available on your system.

```bash
$ git clone https://github.com/gravitational/teleport
Cloning into 'teleport'...
remote: Enumerating objects: 106, done.
remote: Counting objects: 100% (106/106), done.
remote: Compressing objects: 100% (95/95), done.
remote: Total 61144 (delta 33), reused 35 (delta 11), pack-reused 61038
Receiving objects: 100% (61144/61144), 85.17 MiB | 4.66 MiB/s, done.
Resolving deltas: 100% (39141/39141), done.
```

Once this is done, you can change into the directory where the Terraform code is checked out and run `terraform init`:

```bash
$ cd teleport/examples/aws/terraform
$ terraform init

Initializing the backend...

Initializing provider plugins...
- Checking for available provider plugins...
- Downloading plugin for provider "template" (hashicorp/template) 2.1.2...
- Downloading plugin for provider "aws" (hashicorp/aws) 2.51.0...
- Downloading plugin for provider "random" (hashicorp/random) 2.2.1...

Terraform has been successfully initialized!

You may now begin working with Terraform. Try running "terraform plan" to see
any changes that are required for your infrastructure. All Terraform commands
should now work.

If you ever set or change modules or backend configuration for Terraform,
rerun this command to reinitialize your working directory. If you forget, other
commands will detect it and remind you to do so if necessary.
```

This will download the appropriate Terraform plugins needed to spin up Teleport using our
reference code.

## Set up variables

Terraform modules use variables to pass in input. You can do this on the command line to `terraform plan`,
by editing the `vars.tf` file or the way we do it - by setting environment variables.

Any environment variable starting with `TF_VAR_` is automatically stripped down, so `TF_VAR_test_variable` becomes
`test_variable` to Terraform.

We maintain an up-to-date list of the variables and what they do in the README.md file under [the
`examples/aws/terraform` section of the Teleport repo](https://github.com/gravitational/teleport/blob/master/examples/aws/terraform/README.md)
but we'll run through an example list here.

Things you will need to decide on:

### region

!!! note "How to set"
    `export TF_VAR_region="us-west-2"`

The AWS region to run in, pick from the supported list in the README


### cluster_name

!!! note "How to set"
    `export TF_VAR_cluster_name="teleport.example.com"`

The cluster name is the Teleport cluster name to use. This should be unique, and not contain spaces or other special characters.
This will appear in the web UI for your cluster and cannot be changed after creation without rebuilding your
cluster from scratch, so choose carefully. A good example might be `teleport.domain.com` where `domain.com`
is your company domain.


### ami_name

!!! note "How to set"
    `export TF_VAR_ami_name="gravitational-teleport-ami-ent-4.2.3"`

Gravitational automatically builds and publishes OSS, Enterprise and Enterprise FIPS 140-2 AMIs when we
release a new version of Teleport. The AMIs follow the format: `gravitational-teleport-ami-<type>-<version>`
where `<type>` is either `oss` or `ent` (Enterprise) and `version` is the version of Teleport e.g. `4.2.3`.
FIPS 140-2 compatible AMIs (which deploy Teleport in FIPS 140-2 mode by default) have the `-fips` suffix.

The AWS account ID which publishes these AMIs is `126027368216`. You can list the available AMIs with
the example `awscli` commands below. The output is in JSON format by default.

!!! tip "List OSS AMIs"
    `aws ec2 describe-images --owners 126027368216 --filters 'Name=name,Values=gravitational-teleport-ami-oss*'`

!!! tip "List Enterprise AMIs"
    `aws ec2 describe-images --owners 126027368216 --filters 'Name=name,Values=gravitational-teleport-ami-ent*'`

!!! tip "List Enterprise FIPS 140-2 AMIs"
    `aws ec2 describe-images --owners 126027368216 --filters 'Name=name,Values=gravitational-teleport-ami-ent*-fips'`


### key_name

!!! note "How to set"
     `export TF_VAR_key_name="exampleuser"`

The AWS keypair name to use when deploying EC2 instances. This must exist in the same region as you
specify in the `region` variable, and you will need a copy of this keypair to connect to the deployed EC2 instances.


### license_path

!!! note "How to set"
    `export TF_VAR_license_path="/home/user/teleport-license.pem"`

The full local path to your Teleport license file, which customers can download from [the Gravitational dashboard](https://dashboard.gravitational.com/)
This license will be copied into AWS SSM and automatically added to Teleport auth nodes in order to enable
Teleport Enterprise/Pro functionality.


### route53_zone

!!! note "How to set"
    `export TF_VAR_route53_zone="example.com"`

Our Terraform setup requires you to have your domain provisioned in AWS Route 53 - it will automatically add
DNS records for `route53_domain` as set up below. You can list these with `aws route53 list-hosted-zones` - 
you should should use the appropriate "Name" field here.


### route53_domain

!!! note "How to set"
    `export TF_VAR_route53_domain="teleport.example.com"`

A subdomain to set up as a CNAME to the Teleport load balancer for web access. We generally advise that this should be
the same as the `cluster_name` picked above.


### s3_bucket_name

!!! note "How to set"
    `export TF_VAR_s3_bucket_name="teleport.example.com"`

The Terraform example also provisions an S3 bucket to hold certificates provisioned by LetsEncrypt and distribute these
to EC2 instances. This can be any S3-compatible name, and will be generated in the same region as set above.


### email

!!! note "How to set"
    `export TF_VAR_email="support@example.com"`

LetsEncrypt requires an email address for every certificate registered which can be used to send notifications and
useful information. We recommend a generic ops/support email address which the team deploying Teleport has access to.


### grafana_pass

!!! note "How to set"
    `export TF_VAR_grafana_pass="CHANGE_THIS_VALUE"`

We deploy Grafana along with every Terraform deployment and automatically make stats on cluster usage available.
This variable sets up the password for the Grafana `admin` user. The Grafana web UI is served on the same subdomain
as specified above in `route53_domain` on port 8443. 

With the variables set in this example, it would be available on https://cluster.example.com:8443

If you do not change this from the default (`CHANGE_THIS_VALUE`), then it will be set to a random value and written to AWS
SSM as `grafana_pass` - you will need to query this value from SSM.


### use_acm

!!! note "How to set"
    `export TF_VAR_use_acm="false"`

If set to the string `"false"`, Terraform will use [LetsEncrypt](https://letsencrypt.org/) to provision the public-facing
web UI certificate for the Teleport cluster (`route53_subdomain` - so https://teleport.example.com in this example). This uses an [AWS network load balancer](https://docs.aws.amazon.com/elasticloadbalancing/latest/network/introduction.html) to load-balance connections to the Teleport cluster's web UI, and its SSL termination is handled by Teleport itself.

If set to the string `"true"`, Terraform will use [AWS ACM](https://aws.amazon.com/certificate-manager/) to
provision the public-facing web UI certificate for the cluster. This uses an [AWS application load balancer](https://docs.aws.amazon.com/elasticloadbalancing/latest/application/introduction.html) to load-balance connections to the Teleport cluster's web UI, and its SSL termination is handled by the load balancer.

If you wish to use a pre-existing ACM certificate rather than having Terraform generate one for you, you can import it
with this command: `terraform import aws_acm_certificate.cert <certificate_arn>`

## Reference deployment defaults

### Instances

Our reference deployment will provision the following instances for your cluster by default:

- 2 x m4.large Teleport auth instances in an ASG, behind an internal network load balancer, configured using DynamoDB
for shared storage ([the desired size of the ASG is configured here](https://github.com/gravitational/teleport/blob/master/examples/aws/terraform/auth_asg.tf#L11))
- 2 x m4.large Teleport proxy instances in an ASG, behind a public-facing load balancer (NLB for LetsEncrypt, ALB for ACM) ([the desired size of the ASG is configured here](https://github.com/gravitational/teleport/blob/master/examples/aws/terraform/proxy_asg.tf#L12))
- 1 x m4.large Teleport node instance in an ASG ([the desired size of the ASG is configured here](https://github.com/gravitational/teleport/blob/master/examples/aws/terraform/node_asg.tf#L10))
- 1 x m4.large monitoring server in an ASG which hosts the Grafana instance and receives monitoring data from each part of the cluster ([the desired size of the ASG is configured here](https://github.com/gravitational/teleport/blob/master/examples/aws/terraform/monitor_asg.tf#L12))
- 1 x t2.medium bastion server which is the only permitted source for inbound SSH traffic to the instances

[The instance types used for each ASG can be configured here](https://github.com/gravitational/teleport/blob/master/examples/aws/terraform/vars.tf#L23-L44)


### Cluster state database storage

The reference Terraform deployment sets Teleport up to store its cluster state database in DynamoDB. The name of the
table for cluster state will be the same as the cluster name configured in the [`cluster_name`](#cluster_name) variable above.

In our example, the DynamoDB table would be called `teleport.example.com`.

More information about how Teleport works with DynamoDB can be found in our [DynamoDB Admin Guide](https://gravitational.com/teleport/docs/admin-guide/#using-dynamodb)


### Audit event storage

The reference Terraform deployment sets Teleport up to store cluster audit logs in DynamoDB. The name of the table for
audit event storage will be the same as the cluster name configured in the [`cluster_name`](#cluster_name) variable above
with `-events` appended to the end.

In our example, the DynamoDB table would be called `teleport.example.com-events`.

More information about how Teleport works with DynamoDB can be found in our [DynamoDB Admin Guide](https://gravitational.com/teleport/docs/admin-guide/#using-dynamodb)


### Recorded session storage

The reference Terraform deployment sets Teleport up to store recorded session logs in the same S3 bucket configured in
the [`s3_bucket_name`](#s3_bucket_name) variable, under the `records` directory. In our example this would be `s3://teleport.example.com/records`

!!! tip "Tip"

    S3 provides [Amazon S3 Object Lock](https://docs.aws.amazon.com/AmazonS3/latest/dev/object-lock.html),
    which is useful for customers deploying Teleport in regulated environments.

### Cluster domain

The reference Terraform deployment sets the Teleport cluster up to be available on a domain defined in Route53, referenced
by the `route53_domain`[#route53_domain] variable. In our example this would be `teleport.example.com`

Teleport's web interface will be available on port 443 - https://teleport.example.com

Teleport's proxy SSH interface will be available on port 3023 (this is the default port used when connecting with the
`tsh` client)


## Technical details

### Ports exposed by the Teleport deployment

443 - HTTPS access to the Teleport web UI
3023 - Used for SSH access to the Teleport proxy server
3024 - Used for inbound tunnel connections from Teleport [trusted clusters](trustedclusters.md)
3025 - Used for nodes to connect to the Teleport cluster. This is exposed via an internal load balancer inside the VPC
where Teleport is deployed


## Accessing the cluster after Terraform setup

Once the Terraform setup is finished, your Teleport cluster's web UI should be available on https://<route53_domain>

### Adding an admin user to the Teleport cluster

To add users to the Teleport cluster, you will need to connect to a Teleport auth server via SSH and run the `tctl` command.

1. Use the AWS cli to get the IP of the bastion server:

```bash
$ export BASTION_IP=$(aws ec2 describe-instances --filters "Name=tag:TeleportCluster,Values=${TF_VAR_cluster_name},Name=tag:TeleportRole,Values=bastion" --query "Reservations[*].Instances[*].PublicIpAddress" --output text)
$ echo ${BASTION_IP}
1.2.3.4
```

2. Use the AWS cli to get the IP of an auth server:

```bash
$ export AUTH_IP=$(aws ec2 describe-instances --filters "Name=tag:TeleportCluster,Values=${TF_VAR_cluster_name},Name=tag:TeleportRole,Values=auth" --query "Reservations[0].Instances[*].PrivateIpAddress" --output text)
$ echo ${AUTH_IP}
172.31.0.196
```

3. Use both these values to SSH into the auth server (make sure that the AWS keypair that you specified in the
`key_name`[#key_name] variable is available in the current directory):

```bash
$ ssh -i ${TF_VAR_key_name}.pem -J ec2-user@${BASTION_IP} ec2-user@${AUTH_IP}
The authenticity of host '1.2.3.4 (1.2.3.4)' can't be established.
ECDSA key fingerprint is SHA256:vFPnCFliRsRQ1Dk+muIv2B1Owm96hXiihlOUsj5H3bg.
Are you sure you want to continue connecting (yes/no/[fingerprint])? yes
Warning: Permanently added '1.2.3.4' (ECDSA) to the list of known hosts.
The authenticity of host '172.31.0.196 (<no hostip for proxy command>)' can't be established.
ECDSA key fingerprint is SHA256:vFPnCFliRsRQ1Dk+muIv2B1Owm96hXiihlOUsj5H3bg.
Are you sure you want to continue connecting (yes/no/[fingerprint])? yes
Warning: Permanently added '172.31.0.196' (ECDSA) to the list of known hosts.
Last login: Tue Mar  3 18:57:12 2020 from 1.2.3.5

       __|  __|_  )
       _|  (     /   Amazon Linux 2 AMI
      ___|\___|___|

https://aws.amazon.com/amazon-linux-2/
1 package(s) needed for security, out of 6 available
Run "sudo yum update" to apply all updates.
[ec2-user@ip-172-31-0-196 ~]$ 
```

4. Use the `tctl` command to create an admin user for Teleport:

```bash
[ec2-user@ip-172-31-0-196 ~]$ sudo tctl users add teleport-admin --roles=admin
Signup token has been created and is valid for 1 hours. Share this URL with the user:
https://teleport.example.com:443/web/newuser/6489ae886babf4232826076279bcb2fb

NOTE: Make sure teleport.example.com:443 points at a Teleport proxy which users can access.
When the user 'teleport-admin' activates their account, they will be assigned roles [admin]
```

5. Click the link to launch the Teleport web UI and finish setting up your user. You will need to scan the QR
code with an TOTP-compatible app like Google Authenticator or Authy. You will also set a password for the
`teleportadmin` user on this page.

Once this user is successfully configured, you should be logged into the Teleport web UI.


### Teleport service names


TODO(gus)


### Adding EC2 instances to your Teleport cluster
Customers run many workloads within EC2 and depending on how you work there are many
ways to integrate Teleport onto your servers. We recommend looking at our [Admin manual](https://gravitational.com/teleport/docs/admin-guide/#installing).

In short, to add new nodes / EC2 servers that you can "SSH into" you'll need to
- [Install the Teleport Binary on the Server](admin-guide.md#installing)
- [Run Teleport - we recommend using systemd](admin-guide.md#systemd-unit-file)
- [Set the correct settings in /etc/teleport.yaml](admin-guide.md#configuration-file)
- [Add EC2 nodes to the Teleport cluster](admin-guide.md#adding-nodes-to-the-cluster)

The hostname to use with `auth_servers` in your Teleport config file can be found with this command: