# Running Teleport on AWS

We've created this guide to give customers a high level overview of how to use Teleport
on Amazon Web Services (AWS). This guide provides a high level introduction leading to
a deep dive into how to setup and run Teleport in production.

We have split this guide into:

- [Teleport on AWS FAQ](#teleport-on-aws-faq)
- [Authenticating to EKS Using GitHub Credentials with Teleport Community Edition](#accessing-eks-using-teleport)
- [Setting up Teleport Enterprise on AWS](#running-teleport-enterprise-on-aws)
- [Teleport AWS Tips & Tricks](#teleport-aws-tips-tricks)

-  [AWS HA with Terraform](aws_terraform_guide.md)

### Teleport on AWS FAQ

**Why would you want to use Teleport with AWS?**

At some point you'll want to log into the system using SSH
to help test, debug and troubleshoot a problem box. For EC2, AWS recommends creating
['Key Pairs'](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-key-pairs.html)
and has a range of [other tips for securing EC2 instances](https://aws.amazon.com/articles/tips-for-securing-your-ec2-instance/).

This approach has a number of limitations:

1. As your organization grows, keeping track of end users' public/private keys becomes
  an administrative nightmare.
2. Using SSH public/private keys has a number of limitations. Read why [SSH Certificates are better](https://gravitational.com/blog/ssh-key-management/).
3. Once a machine has been bootstrapped with SSH Keys, there isn't an easy way to
  add new keys and delegate access.

**Which Services can I use Teleport with?**

You can use Teleport for all the services that you would SSH into. This guide is focused
on EC2. We have a short blog post on using Teleport with [EKS](https://gravitational.com/blog/teleport-aws-eks/). We plan to expand the guide based on feedback but will plan to add instructions
for the below.

- RDS
- Detailed EKS
- Lightsail
- Fargate
- AWS ECS

## Teleport Introduction

This guide will cover how to setup, configure and run Teleport on [AWS](https://aws.amazon.com/).

#### AWS Services required to run Teleport in HA

- [EC2 / Autoscale](#ec2-autoscale)
- [DynamoDB](#dynamodb)
- [S3](#s3)
- [Route53](#route53)
- [NLB](#nlb-network-load-balancer)
- [IAM](#iam)
- [ACM](#acm)
- [SSM](#aws-systems-manager-parameter-store)

We recommend setting up Teleport in high availability mode (HA). In HA mode DynamoDB
stores the state of the system and S3 will store audit logs.

![AWS Intro Image](img/aws/aws-intro.png)

### EC2 / Autoscale
To run Teleport in a HA configuration we recommend using m4.large instances. It's best practice to separate the proxy and authentication server, using autoscaling groups for both machines. We have pre-built AMIs for both Teleport OSS and Enterprise editions. Instructions on using these [AMIs are below](#single-oss-teleport-amis-manual-gui-setup).

### DynamoDB
DynamoDB is a key-value and document database that delivers single-digit millisecond
performance at any scale. For large clusters you can provision usage but for smaller
deployments you can leverage DynamoDB's autoscaling.

Teleport 4.0 leverages [DynamoDB's streaming feature](
https://github.com/gravitational/teleport/issues/2430). When turning this on, you'll need
to specify `New Image` from the streaming options. DynamoDB back-end supports two
types of Teleport data:

* Cluster state

See [DynamoDB Admin Guide for more information](https://gravitational.com/teleport/docs/admin-guide/#using-dynamodb)

![AWS DynamoDB Tables](img/aws/dynamodb-tables.png)
![Setting Streams](img/aws/setting-stream.png)
Setting Stream to `NEW IMAGE`

For maintainability and ease of use, we recommend following our [Terraform example](https://github.com/gravitational/teleport/blob/master/examples/aws/terraform/dynamo.tf)
but below are high level definitions for the tables required to run Teleport.

Cluster State:

| Table name            | teleport-cluster-name |
|-----------------------|-----------------------|
| Primary partition key | HashKey (String)      |
| Primary sort key      | FullPath (String)     |

### S3
Amazon Simple Storage Service (Amazon S3) is an object storage service that offers
industry-leading scalability, data availability, security, and performance. In this
Teleport setup, S3 will provide storage for recorded sessions.

We recommend using Amazon S3 Standard.

!!! tip "Tip"

    S3 provides [Amazon S3 Object Lock](https://docs.aws.amazon.com/AmazonS3/latest/dev/object-lock.html),
    which is useful for customers deploying Teleport in regulated environments.

### Route53
Route53 is a highly available Domain Name System (DNS) provided by AWS. It'll be
needed to setup a URL for the proxy - we recommend using a subdomain.

e.g. `teleport.acmeinc.com`

### NLB: Network Load Balancer
AWS provides many different load balancers. To setup Teleport, we recommend
using a Network Load Balancer.  Network Load Balancers provides TLS for the Teleport
proxy and provides the TCP connections needed for Teleport proxy SSH connections.

### IAM
IAM is the recommended tool for creating service access. This guide will follow the
best practice of principle of least privilege (PoLP).

#### IAM for Amazon S3

In order to grant an IAM user in your AWS account access to one of your buckets, `example.s3.bucket` you will need to grant the following permissions: `s3:ListBucket`, `s3:ListBucketVersions`, `s3:PutObject`, `s3:GetObject`, `s3:GetObjectVersion`

An example policy is shown below:

```json
{
   "Version": "2012-10-17",
   "Statement": [
     {
       "Effect": "Allow",
       "Action": [
         "s3:ListBucket",
         "s3:ListBucketVersions"
        ],
       "Resource": ["arn:aws:s3:::example.s3.bucket"]
     },
     {
       "Effect": "Allow",
       "Action": [
         "s3:PutObject",
         "s3:GetObject",
         "s3:GetObjectVersion"
       ],
       "Resource": ["arn:aws:s3:::example.s3.bucket/*"]
     }
   ]
 }
```

!!! note "Note"

    `example.s3.bucket` will need to be replaced with your bucket name.

#### IAM for DynamoDB

In order to grant an IAM user access to DynamoDB make sure that the IAM role assigned to Teleport is configured with proper permissions.

An example policy is shown below:

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "AllAPIActionsOnTeleportAuth",
            "Effect": "Allow",
            "Action": "dynamodb:*",
            "Resource": "arn:aws:dynamodb:eu-west-1:123456789012:table/prod.teleport.auth"
        },
        {
            "Sid": "AllAPIActionsOnTeleportStreams",
            "Effect": "Allow",
            "Action": "dynamodb:*",
            "Resource": "arn:aws:dynamodb:eu-west-1:123456789012:table/prod.teleport.auth/stream/*"
        }
    ]
}
```

!!! note "Note"

    `eu-west-1:123456789012:table/prod.teleport.auth` will need to be replaced with your DynamoDB instance.

### ACM

With AWS Certificate Manager, you can quickly request SSL/TLS certificates.

- TLS Cert: Used to provide SSL for the proxy.
- SSH Certs (not in ACM): Created and self signed by the `authentication server` and are used to
  delegate access to Teleport nodes.

### AWS Systems Manager Parameter Store
To add new nodes to a Teleport Cluster, we recommend using a [strong static token](https://gravitational.com/teleport/docs/admin-guide/#example-configuration). SSM can be also used to store the
enterprise licence.

## Setting up a HA Teleport Cluster
Teleport's config based setup offers a wide range of customization for customers.
This guide offers a range of setup options for AWS. If you have a very large account,
multiple accounts, or over 10k users we would recommend getting in touch. We are
more than happy to help you architect, setup and deploy Teleport into your environment.

We have these options for you.

- [Using AWS Marketplace (Manual Setup)](#single-oss-teleport-amis-manual-gui-setup)
- [Deploying with CloudFormation](#deploying-with-cloudformation)
- [Deploying with Terraform HA + Monitoring](#deploying-with-terraform)

### Single OSS Teleport AMIs (Manual / GUI Setup)
This guide provides instructions on deploying Teleport using AMIs, the below instructions
are designed for using the AMI and GUI. It doesn't setup Teleport in HA, so we recommend
this as a starting point, but then look at the more advanced sections.

### Prerequisites

- Obtain a SSL/TLS Certificate using ACM.

Prerequisites setup.

1. Generate and issue a certificate in [ACM](https://console.aws.amazon.com/acm/home?#)
for `teleport.acmeinc.com`, use email or DNS validation as appropriate and make sure
it’s approved successfully.

#### Step 1: Subscribe to Teleport Community Edition
Subscribe to the Teleport Community Edition on the [AWS Marketplace](https://aws.amazon.com/marketplace/pp/B07FYTZB9B).

1. Select 'Continue to Subscribe'
2. Review the Terms and Conditions, and click `Continue to Configuration'
3. Configure this software. Keep options as set, you might want to change region
to be in the same place as the rest of your infrastructure. Click Continue to Launch
4. _Launch this software_ Under Choose Action, select Launch through EC2.


![AWS Marketplace Subscribe](img/aws/aws-marketplace-subscribe.png)
![AWS Launch via EC2](img/aws/launch-through-ec2.png)

5. Launch through EC2. At this point AWS will take you from the marketplace and drop
you into the EC2 panel. [Link: Shortcut to EC2 Wizard](https://console.aws.amazon.com/ec2/v2/home?region=us-east-1#LaunchInstanceWizard:ami=ami-04e79542e3e5fbf02;product=92c3dc07-bdfa-4e88-8c8b-e6187dac50af)


#### Step 2: Build instance
We recommend using an `m4.large`, but a `t2.medium` should be good for POC testing.

![AWS Instance Size ](img/aws/aws-pick-instance-size.png)


4. Make sure to write appropriate values to `/etc/teleport.d/conf` via user-data
    (using something like this):

```json
#!/bin/bash
cat >/etc/teleport.d/conf <<EOF
USE_ACM="true"
TELEPORT_DOMAIN_NAME="teleport.acmeinc.com"
TELEPORT_EXTERNAL_HOSTNAME="teleport.acmeinc.com"
TELEPORT_EXTERNAL_PORT="443"
TELEPORT_AUTH_SERVER_LB="teleport-nlb.acmeinc.com"
EOF
```

Screenshot of where to put it in via AWS console.

![Config Instance Details](img/aws/adding-user-data.png)

!!! note "Note"

    `TELEPORT_DOMAIN_NAME` and `TELEPORT_EXTERNAL_HOSTNAME` are more or less the
    same thing but we keep them separate just in case you want to use a load balancer
    on a different hostname.

The CA certificates for the server will be generated to have `TELEPORT_EXTERNAL_HOSTNAME` as a CN,
assuming it's set when the server starts.

#### Step 3: Create the Load Balancers
2.  When using ACM you must use an [application load balancer](https://console.aws.amazon.com/ec2/v2/home?region=us-east-1#LoadBalancers:sort=loadBalancerName) (ALB) as this will terminate SSL.
    1. Add the ACM certificate that you approved for `teleport.acmeinc.com`
    - Add a listener on the ALB for HTTPS on `443/tcp`
    - Target group will point to your instance - point to HTTP on `3080/tcp`
    - Create a http health check, point to `/webapi/ping`.
    - Create a DNS record for `teleport.acmeinc.com`
    - Point this to the public A record of the ALB as provided by Amazon.

![Summary for AWS Load Balancer](img/aws/loadbalancer-review.png)

3. You also need to set up a network load balancer (NLB) for the auth traffic:
    1. Set up a listener on `3025/tcp`
    - Target group will point to your instance - point to `3025/tcp`
    - Create a DNS record for `teleport-nlb.acmeinc.com`
    - Point this to the public A record of the NLB as provided by Amazon
    - Make sure that your DNS record is also reflected in `TELEPORT_AUTH_SERVER_LB` in the user data
    - Launch the instance (you can also use an already-running instance if you
    follow the instructions at the bottom of this section)

#### Step 4: Create Teleport user
1. We are going to use `tctl` command to create a user for Teleport. The first step
is to SSH into the newly created OSS Teleport box.

`ssh -i id_rsa ec2-user@52.87.213.96`

^ Replace with IP given available from the EC2 instance list.

```bash
➜  ~ ssh -i id_rsa ec2-user@52.87.213.96
Warning: Identity file id_rsa not accessible: No such file or directory.
The authenticity of host '52.87.213.96 (52.87.213.96)' can't be established.
ECDSA key fingerprint is SHA256:YnTAP29shPpaAbLasfwazkIx7qFsKVWP3Pw40ehiHKg.
Are you sure you want to continue connecting (yes/no/[fingerprint])? yes
Warning: Permanently added '52.87.213.96' (ECDSA) to the list of known hosts.
Enter passphrase for key '/Users/benarent/.ssh/id_rsa':
Last login: Tue Jun 18 00:07:25 2019 from 13.88.188.155

       __|  __|_  )
       _|  (     /   Amazon Linux 2 AMI
      ___|\___|___|

https://aws.amazon.com/amazon-linux-2/
No packages needed for security; 7 packages available
Run "sudo yum update" to apply all updates.
[ec2-user@ip-172-30-0-111 ~]$
```

2. Apply Updates `sudo yum update`
3. Create a new admin user `sudo tctl users add teleport-admin ec2-user`
```xml
[ec2-user@ip-172-30-0-111 ~]$ sudo tctl users add teleport-admin ec2-user
Signup token has been created and is valid for 1 hours. Share this URL with the user:
https://teleport.acmeinc.com:443/web/newuser/cea9871a42e780dff86528fa1b53f382

NOTE: Make sure teleport.acmeinc.com:443 points at a Teleport proxy which users can access.
```
![Summary for AWS Load Balancer](img/aws/teleport-admin.png)

Step 5: Finish
You've now successfully setup a simple Teleport AMI, that uses local storage and
has itself as a node. Next we'll look at using HA services to create a more scalable
Teleport install.

![Summary for AWS Load Balancer](img/aws/teleport-setup.png)

#### Reconfiguring/using a pre-existing instance

To reconfigure any of this, or to do it on a running instance:

1. Make the appropriate changes to `/etc/teleport.d/conf`
    1. `rm -f /etc/teleport.yaml`
    * `systemctl restart teleport-generate-config.service`
    * `systemctl restart teleport-acm.service`

If you have changed the external hostname, you may need to delete `/var/lib/teleport` and start again.

## Deploying with CloudFormation
We are currently working on an updated CloudFormation guide but you can start with our
[AWS Marketplace example](https://github.com/gravitational/teleport/tree/master/assets/marketplace/cloudformation#teleport-aws-quickstart-guide-cloudformation). It requires a VPC, but
we expect customers to deploy within an already existing VPC.

## Deploying with Terraform
To deploy Teleport in AWS using Terraform look at our [AWS Guide](https://github.com/gravitational/teleport/tree/master/examples/aws/terraform#terraform-based-provisioning-example-amazon-single-ami).


### Installing Teleport to EC2 Server
Customers run many workloads within EC2 and depending on how you work there are many
ways to integrate Teleport onto your servers. We recommend looking at our [Admin manual](https://gravitational.com/teleport/docs/admin-guide/#installing).

In short, to add new nodes / EC2 servers that you can "SSH into" you'll need to

1. [Install the Teleport Binary on the Server](https://gravitational.com/teleport/docs/admin-guide/#installing)
- [Run Teleport, we recommend using SystemD](https://gravitational.com/teleport/docs/admin-guide/#systemd-unit-file)
- [Set the correct settings in /etc/teleport.yaml](https://gravitational.com/teleport/docs/admin-guide/#configuration-file)
- [Add EC2 nodes to the Teleport cluster](https://gravitational.com/teleport/docs/admin-guide/#adding-nodes-to-the-cluster)

## Upgrading

To upgrade to a newer version of Teleport:

- Back up `/etc/teleport.yaml`, `/etc/teleport.d/` and the contents of `/var/lib/teleport`
- Launch a new instance with the correct AMI for a newer version of Teleport
- Copy `/etc/teleport.yaml`, `/etc/teleport.d/` and `/var/lib/teleport` to the new instance, overwriting anything that already exists
- Copy  and its contents should also be backed up and copied over to the new instance.
- Either restart the instance, or log in via SSH and run `sudo systemctl restart teleport.service`

## Accessing EKS using Teleport

This guide is based on an orignial post from AWS Open Source Blog - [Authenticating to EKS Using GitHub Credentials with Teleport](https://aws.amazon.com/blogs/opensource/authenticating-eks-github-credentials-teleport/) but will be updated with latest Teleport version & best practices.

### Prerequisites
You’ll need a functioning EKS cluster, we recommend version 1.16.  If you’re unfamiliar
with creating an EKS cluster, see [eksctl.io](https://eksctl.io). Make sure you have eksctl version 0.22.0.

- EKS Version: The below guide has been tested with [Kubernetes 1.16](https://docs.aws.amazon.com/eks/latest/userguide/kubernetes-versions.html)
- [jq](https://stedolan.github.io/jq/) installed on you  local machine.
- An AWS Account with Root Access

### Create cluster role and role binding
The first step is to create a cluster role and role binding that will allow the Teleport
EC2 instance to impersonate other users, groups, and service accounts.

The below command is ran on a machine with `kubectl` access to the cluster.

```bash
$ cat << 'EOF' | kubectl apply -f -
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: teleport-impersonation
rules:
- apiGroups:
  - ""
  resources:
  - users
  - groups
  - serviceaccounts
  verbs:
  - impersonate
- apiGroups:
  - "authorization.k8s.io"
  resources:
  - selfsubjectaccessreviews
  verbs:
  - create
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: teleport-crb
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: teleport-impersonation
subjects:
- kind: Group
  name: teleport-group
- kind: User
  name: system:anonymous
EOF
```
If successful the terminal should output.
```
# clusterrole.rbac.authorization.k8s.io/teleport-impersonation created
# clusterrolebinding.rbac.authorization.k8s.io/teleport-crb created
```

### Create IAM trust policy document
This is the trust policy that allows the Teleport EC2 instance to assume a role.

```bash
$ cat > teleport_assume_role.json << 'EOF'
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": "ec2.amazonaws.com"
      },
      "Action": "sts:AssumeRole"
    }
  ]
}
EOF
```
This will create a `.json` file locally, the below command should then be ran to create the role.

```bash
$ ROLE_ARN=$(aws iam create-role --role-name teleport-roles --assume-role-policy-document file://teleport_assume_role.json | jq -r '.Role.Arn')
```

### Create IAM policy granting list-clusters and describe-cluster permissions (optional)

This policy is necessary to create a `kubeconfig` file using the `aws eks update-kubeconfig`
command. If you have another mechanism to create a kubeconfig file on the instance that runs
Teleport, this step is not required.

```bash
cat > teleport_eks_desc_and_list.json << 'EOF'
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "VisualEditor0",
            "Effect": "Allow",
            "Action": [
                "eks:DescribeCluster",
                "eks:ListClusters"
            ],
            "Resource": "*"
        }
    ]
}
EOF
POLICY_ARN=$(aws iam create-policy --policy-name teleport-policy --policy-document file://teleport_eks_desc_and_list.json | jq -r '.Policy.Arn')
aws iam attach-role-policy --role-name teleport-role --policy-arn $POLICY_ARN
```

### Update aws-auth configmap

This maps the IAM role **teleport-role** to the Kubernetes group **teleport-group**.

!!! note

    If you used eksctl to create your cluster, you may need to add the mapUsers section to the aws-auth ConfigMap before executing these commands.

    ```
    eksctl create iamidentitymapping --cluster [cluster-name] --arn arn:aws:iam::[account-id]:role/teleport-role --group teleport-group --username teleport
    ```

```bash
ROLE="    - userarn: arn:aws:iam::[account-id]:role/teleport-role\n      username: teleport\n      groups:\n        - teleport-group"
kubectl get -n kube-system configmap/aws-auth -o yaml | awk "/mapRoles: \|/{print;print \"$ROLE\";next}1" > /tmp/aws-auth-patch.yml
kubectl patch configmap/aws-auth -n kube-system --patch "$(cat /tmp/aws-auth-patch.yml)"
```

To check, run  `kubectl describe configmap -n kube-system aws-auth`


### Installing Teleport
#### Create EC2 instance
Create an EC2 instance using the [Teleport Community AMI](https://aws.amazon.com/marketplace/pp/Gravitational-Teleport-Community-Edition/B07FYTZB9B) on a public subnet in your VPC. Modify the security group associated with that instance to allow port 22 inbound, so you can SSH to the instancea fter it’s running. You will need security group rules to allow access to ports `3080`  and `3022-3026` so that users can access theTeleport server from the Internet.

This  also allows GitHub to post a response back to the Teleport server. You’ll also
need to open port 80 to allow Let’s Encrypt to complete HTTP validation when issuing
SSL certificates.

| Type   | Protocol | Port Range | Source    |
|--------|----------|------------|-----------|
| Custom | TCP      | 3022-3026  | 0.0.0.0/0 |
| Custom | TCP      | 3080       | 0.0.0.0/0 |
| HTTP   | TCP      | 80         | 0.0.0.0/0 |
| SSH    | TCP      | 22         | your IP   |

If you don’t modify the EKS control plane security group to allow port 443 inbound
from the Teleport security group, your Teleport instance will not be able to communicate
with the Kubernetes API.

Assign role to instance:

```bash
aws iam create-instance-profile --instance-profile-name teleport-role
aws iam add-role-to-instance-profile --instance-profile-name teleport-role --role-name teleport-role
aws ec2 associate-iam-instance-profile --iam-instance-profile Name=teleport-role --instance-id [instance_id]
# instance_id should be replaced with the instance id of the instance where you intend to install Teleport.
```

#### Install Teleport.
1. [Download and Install Teleport](installation.md/#install-pre-built-binaries)
2. [Setup systemd](production.md#running-teleport-in-production)
3. Setup Teleport config file. `sudo cp teleport.yaml /etc/teleport.yaml`. An example is below.

```
{!examples/aws/eks/teleport.yaml!}
```

#### 2. Download Kubectl on Teleport Server
Kubectl is required to obtain the correct Kube config, so Teleport can access the EKS Cluster.

Instruction are below, or can be found on [AWS Docs](https://docs.aws.amazon.com/eks/latest/userguide/install-kubectl.html).

```bash
# Download the Amazon EKS-vended kubectl binary for your cluster's Kubernetes version from Amazon S3:
curl -o kubectl https://amazon-eks.s3.us-west-2.amazonaws.com/1.16.8/2020-04-16/bin/linux/amd64/kubectl
# (Optional) Verify the downloaded binary with the SHA-256 sum for your binary.
curl -o kubectl.sha256 https://amazon-eks.s3.us-west-2.amazonaws.com/1.16.8/2020-04-16/bin/linux/amd64/kubectl.sha256
# Check the SHA-256 sum for your downloaded binary.
openssl sha1 -sha256 kubectl
# Apply execute permissions to the binary.
chmod +x ./kubectl
# Copy the binary to a folder in your PATH. If you have already installed a version of kubectl, then we recommend creating a $HOME/bin/kubectl and ensuring that $HOME/bin comes first in your $PATH.
sudo mv ./kubectl /usr/local/bin
# After you install kubectl, you can verify its version with the following command:
kubectl version --short --client
```

Create kubeconfig:
```bash
aws eks update-kubeconfig --name [teleport-cluster] --region us-west-2
# Added new context arn:aws:eks:us-west-2:480176057099:cluster/teleport-eks-cluster to /home/ec2-user/.kube/config
```

Create SSL certificate for HTTPs. It is absolutely crucial to properly configure TLS
for HTTPS when you use Teleport Proxy in production. For simplicity, we are using Let’s
Encrypt to issue certificates and simple DNS resolution.

However, using an Elastic IP and a Route53 domain name would be appropriate for production use cases.

Install certbot:

```bash
# Install the EPEL release package for RHEL 7 and enable the EPEL repository.
sudo yum install -y https://dl.fedoraproject.org/pub/epel/epel-release-latest-7.noarch.rpm
sudo yum install -y certbot python-certbot-nginx
```

Run Certbot
```bash
export TELEPORT_PUBLIC_DNS_NAME=[teleport-proxy-url]
echo $TELEPORT_PUBLIC_DNS_NAME
export EMAIL=[email-for-letsencrypt]

sudo certbot certonly --standalone \
             --preferred-challenges http \
                      -d $TELEPORT_PUBLIC_DNS_NAME \
                      -n \
                      --agree-tos \
                      --email=$EMAIL
```

#### 4. Setup Github Auth

Run this on the Teleport EC2 Host, see [Github Auth](admin-guide.md#github-oauth-20) for more info.

```bash
export TELEPORT_PUBLIC_DNS_NAME="[teleport-proxy-url]"
export GH_CLIENT_ID="[github-client-id]"
export GH_SECRET="[github-oauth-secret]"
export GH_ORG="[github-org]"
export GH_TEAM="[github-team]"

cat > github.yaml << EOF
kind: github
version: v3
metadata:
  # connector name that will be used with 'tsh --auth=github login'
  name: github
spec:
  # client ID of Github OAuth app
  client_id: $GH_CLIENT_ID
  # client secret of Github OAuth app
  client_secret: $GH_SECRET
  # connector display name that will be shown on web UI login screen
  display: Github
  # callback URL that will be called after successful authentication
  redirect_url: https://$TELEPORT_PUBLIC_DNS_NAME:3080/v1/webapi/github/callback
  # mapping of org/team memberships onto allowed logins and roles
  teams_to_logins:
  - kubernetes_groups:
    - system:masters
    logins:
    - github
    - ec2-user
    organization: $GH_ORG
    team: $GH_TEAM
EOF
```
Use `tctl` to create the github auth connector.

```bash
sudo /usr/local/bin/tctl create -f ./github.yaml
```

```
➜  ~ tsh login --proxy=[teleport-proxy-url].practice.io:3080
If browser window does not open automatically, open it by clicking on the link:
 http://127.0.0.1:64467/54e5d06a-c509-4077-bf54-fb27fd1b8d50
> Profile URL:  https://[teleport-proxy-url].practice.io:3080
  Logged in as: benarent
  Cluster:      teleport-eks
  Roles:        admin*
  Logins:       github, ec2-user
  Valid until:  2020-06-30 02:12:54 -0700 PDT [valid for 12h0m0s]
  Extensions:   permit-agent-forwarding, permit-port-forwarding, permit-pty


* RBAC is only available in Teleport Enterprise
  https://gravitational.com/teleport/docs/enterprise
```

On your local machine test using `kubectl get pods --all-namespaces`

```bash
➜  ~ kubectl get pods --all-namespaces
NAMESPACE     NAME                       READY   STATUS    RESTARTS   AGE
kube-system   aws-node-56p6g             1/1     Running   0          5h28m
kube-system   aws-node-7dv5j             1/1     Running   0          5h28m
kube-system   coredns-5c97f79574-69m6k   1/1     Running   0          5h36m
kube-system   coredns-5c97f79574-dq54w   1/1     Running   0          5h36m
kube-system   kube-proxy-7w4z4           1/1     Running   0          5h28m
kube-system   kube-proxy-c5nv2           1/1     Running   0          5h28m
```

## Running Teleport Enterprise on AWS
Most of this guide has been designed for OSS Teleport. Most of this guide also applies to Teleport Enterprise with a few extra notes around adding a license and getting the correct binary. If you would like help setting up Teleport Enterprise on AWS, please mail us at [info@gravitational.com](mailto:info@gravitational.com)

## Teleport AWS Tips & Tricks

### Generating labels from AWS tags
Labels can be a useful addition to the Teleport UI.  Simply add some or all of the
below to Teleport Nodes in `etc/teleport.yaml` to have helpful labels in the Teleport UI.

```yaml
    commands:
      - name: arch
        command: [uname, -p]
        period: 1h0m0s
      - name: kernel
        command: [uname, -r]
        period: 1h0m0s
      - name: uptime
        command: [uptime, -p]
        period: 1h0m0s
      - name: internal
        command: [curl, "http://169.254.169.254/latest/meta-data/local-ipv4"]
        period: 1h0m0s
      - name: external
        command: [curl, "http://169.254.169.254/latest/meta-data/public-ipv4"]
        period: 1h0m0s
```

Create labels based on [EC2 Tags](https://github.com/gravitational/teleport/issues/1346).
