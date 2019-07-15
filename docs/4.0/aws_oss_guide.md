# Running Teleport on AWS

We've created this guide to give customers a high level overview of how to use Teleport
on Amazon Web Services (AWS). This guide starts with a high level introduction and 
with a deeper dive into how to setup Teleport and run Teleport in production. 
We have split this guide into. 

- [Teleport on AWS FAQ](#teleport-on-aws-faq)
- [Setting up Teleport OSS on AWS](#setting-up-teleport-oss-on-aws)
- [Setting up Teleport Enterprise on AWS](#running-teleport-enterprise-on-aws)
- [Teleport AWS Tips & Tricks](#teleport-aws-tips-tricks)

### Teleport on AWS FAQ
**Why would you want to use Teleport with AWS?**
As with all systems, at some point you'll want to log into the system using SSH
to help test, debug and troubleshoot a problem box. For EC2, AWS recommends creating 
['Key Pairs'](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-key-pairs.html) 
and has a range of [other tips for securing EC2 instances](https://aws.amazon.com/articles/tips-for-securing-your-ec2-instance/).

There is a range of limits with this setup. 

1. As your organization grows keeping track of end users public/private keys becomes
   an administrative nightmare. 
2. Using SSH public/private keys has a range of limits. Read why [SSH Certificates are better](https://gravitational.com/blog/ssh-key-management/). 
3. Once a machine has been bootstrapped with SSH Keys, there isn't an easy way to 
  add new keys and delegate access. 

**Which Services can I use Teleport with?**

You can use Teleport for all the services that you can SSH into. This guide is focused
on EC2. We have a short blog post on using Teleport with [EKS](https://gravitational.com/blog/teleport-aws-eks/). We plan to expand the guide based on feedback but will plan to add

- RDS
- Detailed EKS
- Lightsail
- Fargate
- AWS ECS 

## Teleport Introduction

This guide will cover how to setup, configure and run Teleport on [AWS](https://aws.amazon.com/). 
 
#### AWS Services required for high availability Teleport 

- [EC2 / Autoscale](#ec2-autoscale)
- [DynamoDB](#dynamodb)
- [S3](#s3)
- [Route53](#route53) 
- [NLB](#nlb-network-load-balancer)
- [IAM](#iam)
- [ACM](#acm)

We recommend setting up Teleport in high availability mode (HA). In HA mode DynamoDB 
stores the state of the system and S3 will store audit logs. 

![AWS Intro Image](img/aws/aws-intro.png)

### EC2 / Autoscale
To run Teleport in a HA configuration we recommend using m4.large instances and it's best
practice to separate the proxy and authentication server, creating auto-scale groups 
for both machines. We've a range of AMIs that have Teleport already built in it. 
Instructions on using these [AMIs are below](#).

### DynamoDB
DynamoDB is a  key-value and document database that delivers single-digit millisecond 
performance at any scale. For large clusters you can provision usage but for smaller 
deployments you can leverage DynamoDBs auto-scale. 

Teleport 4.0 leverages [DynamoDB streaming feature](
https://github.com/gravitational/teleport/issues/2430). When turning this on, you'll need
to specify `New Image` from the streaming options. DynamoDB back-end supports two 
types of Teleport data:

* Cluster state
* Audit log events


### S3 
Amazon Simple Storage Service (Amazon S3) is an object storage service that offers
industry-leading scalability, data availability, security, and performance. In 
this setup of Teleport S3 will be the store of recorded sessions. 
 
We recommend using Amazon S3 Standard. 

!!! tip "Tip":
    S3 provides [Amazon S3 Object Lock](https://docs.aws.amazon.com/AmazonS3/latest/dev/object-lock.html),
    which is useful for customers deploying Teleport in regulated environments. 

### Route53
Route53 is a highly available Domain Name System (DNS) provided by AWS. It'll be 
needed to setup a URL for the proxy, we recommend using a sub-domain. 

e.g. `teleport.acmeinc.com`

### NLB: Network Load Balancer
AWS provides many different load balancers. To setup Teleport, we recommend 
using a Network Load Balancer.  Network Load Balancers provides TLS for the Teleport 
proxy and provides the TCP connections needed for Teleport proxy SSH connections. 

### IAM
IAM is the recommend tool for creating service access. This guide will follow the 
best practice of principle of least privilege (PoLP). 

### ACM 
With AWS Certificate Manager, you can quickly request SSL/TLS certificates, deploy it on 
ACM-integrated AWS resources. 

- TLS Cert: Used to provide SSL for the proxy.
- SSH Certs: Created and self signed by the `authentication server` and is used to
  delegate access to Teleport nodes. 

## Setting up a HA Teleport Cluster
Teleport config based setup offers a wide range of customization for customers. 
This guide offers a range of setup options for AWS. If you have a very large accounts,
multiple accounts, or over 10k users we would recommend getting in touch. We are
more than happy to help you architect, setup and deploy Teleport into your environment.

We have these options for you. 

- [Using our AWS Marketplace (Manual Setup)](#using-oss-teleport-amis-manual-setup)
- Using our AWS Marketplace (Cloudformation Setup)
- Building your own base image (Manual)
- Deploying with Cloudformation
- Deploying with Terraform 

Some of these providers will provision 

### Using OSS Teleport AMIs (Manual Setup)
This guide provides instructions on deploying Teleport using AMIs                          

### Prerequisites 

- Obtain a SSL/TLS Certificate.
- Create an ALB and NLB Load Balancers.
- Create S3 Bucket.
- Create DynamoDB Instance.

Prerequisites setup.

1. Generate and issue a certificate in [ACM](https://console.aws.amazon.com/acm/home?#) 
for `teleport.acmeinc.com`, use email or  DNS validation as appropriate and make sure 
it’s approved successfully.

2.  With ACM you must use an application load balancer (ALB) as this will terminate SSL. 
    1. Add the ACM certificate that you approved for `teleport.acmeinc.com`
    - Add a listener on the ALB for HTTPS on `443/tcp`
    - Target group will point to your instance - point to HTTP on `3080/tcp`
    - Create a DNS record for `teleport.acmeinc.com`
    - Point this to the public A record of the ALB as provided by Amazon

3. You also need to set up a network load balancer (NLB) for the auth traffic:
    1. Set up a listener on `3025/tcp`
    - Target group will point to your instance - point to `3025/tcp`
    - Create a DNS record for `teleport-nlb.acmeinc.com`
    - Point this to the public A record of the NLB as provided by Amazon
    - Make sure that your DNS record is also reflected in `TELEPORT_PROXY_SERVER_LB` in the user data
    - Launch the instance (you can also use an already-running instance if you 
    follow the instructions at the bottom of this section)

#### Step 1: Subscribe to Teleport Community Edition 
Subscribe to the Teleport Community Edition on the [AWS Marketplace](https://aws.amazon.com/marketplace/pp/B07FYTZB9B). 

![AWS Marketplace Subscribe](img/aws/aws-marketplace-subscribe.png)
![AWS Launch via EC2](img/aws/launch-through-ec2.png)


#### Select Right size image
![AWS Instance Size ](img/aws/aws-pick-instance-size.png)


4. Make sure to write appropriate values to /etc/teleport.d/conf via user-data 
    (using something like this):

```bash 
#!/bin/bash
cat >/etc/teleport.d/conf <<EOF
USE_ACM="true"
TELEPORT_DOMAIN_NAME="teleport.acmeinc.com"
TELEPORT_EXTERNAL_HOSTNAME="teleport.acmeinc.com"
TELEPORT_EXTERNAL_PORT="443"
TELEPORT_PROXY_SERVER_LB="test-nlb.acmeinc.com:3025"
EOF
```

!!! note "Note":
    `TELEPORT_DOMAIN_NAME` and `TELEPORT_EXTERNAL_HOSTNAME` are more or less the 
    same thing but we keep them separate just in case you want to use a load balancer 
    on a different hostname. 

The CA certificates for the server will be generated to have `TELEPORT_EXTERNAL_HOSTNAME` as a CN,
assuming it's set when the server starts

#### Reconfiguring/using a pre-existing instance


- to reconfigure any of this, or to do it on a running instance:
-- make the appropriate changes to /etc/teleport.d/conf
-- rm -f /etc/teleport.yaml
-- systemctl restart teleport-generate-config.service
-- systemctl restart teleport-acm.service

- If you have changed the external hostname, you may need to delete /var/lib/teleport and start again?



### Building your own base image

### Deploying with Cloudformation

### Deploying with Terraform

## Adding your AWS fleet to Teleport. 
Best practices
- Turn off SSH / or setup in Proxy mode.
- How to build 

### Installing Teleport to EC2 Nodes
? Using 



### Using Teleport with EKS
??

### Using Teleport with Kubernetes running on AWS



# Upgrading

To upgrade to a newer version of Teleport:
- Back up /etc/teleport.yaml and the contents of /var/lib/teleport
- Launch a new instance with the correct AMI for a newer version of Teleport
- Copy /etc/teleport.yaml and /var/lib/teleport to the new instance, overwriting anything that already exists
- Either restart the instance, or log in via SSH and run "sudo systemctl restart teleport.service"

# Running Teleport Enterprise on AWS

# Teleport AWS Tips & Tricks

## Production Nodes:


### Generating labels from AWS tags

```bash
#!/bin/bash

LOCAL_IP=`curl --silent --fail --show-error http://169.254.169.254/latest/meta-data/local-ipv4`
sed -ri "s/.*public_addr.*/    public_addr: ${LOCAL_IP}/" /etc/teleport/teleport.yaml

EXTERNAL_IP=`curl --silent --fail --show-error http://169.254.169.254/latest/meta-data/public-ipv4`
if [ $? -eq 0 ]
then
  sed -ri "s/.*nodename.*/    nodename: ${EXTERNAL_IP}/" /etc/teleport/teleport.yaml
else
  sed -ri "s/.*nodename.*/    nodename: private-${LOCAL_IP}/" /etc/teleport/teleport.yaml
  sed -ri "s/.*\[curl, \"http:\/\/169.254.169.254\/latest\/meta-data\/public-ipv4\"\].*/        command: \[echo, \"n\/a\"\]/" /etc/teleport/teleport.yaml
fi
```

and in teleport.yaml I had:

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


TODO https://github.com/gravitational/teleport/issues/1175 
 https://github.com/gravitational/teleport/issues/1346 