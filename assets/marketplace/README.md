# Teleport AWS Quickstart Guide

AWS Quickstart for Teleport

## Development instructions

**Prerequisites** 

AWS CLI and Packer are required to build and launch a CloudFormation stack.

On macOS:

```
brew install awscli
brew install packer
```

On Linux:

```
apt install awscli
Follow instructions at: https://www.packer.io/docs/install/index.html
```

**To build an AMI**

```
make oss
```

**Update YAML files with the new AMI image IDs**

```
make update-ami-ids-oss
```

**Launch a dev cloudformation stack using an existing VPC**

When using an existing VPC it must have both DNS support and DNS hostnames enabled.

The deployment needs six VPC subnet IDs provided - two public (for the proxy) and four private (for auth and nodes).
For redundancy, the subnets should be split across availability zones - odd numbers in AZ A and even numbers in AZ B, for example.

Replace the placeholder values in the exports below.

```
export STACK=test1
export STACK_PARAMS="\
ParameterKey=VPCID,ParameterValue=EXISTING_VPC_ID \
ParameterKey=ProxySubnetA,ParameterValue=PUBLIC_SUBNET_ID_1 \
ParameterKey=ProxySubnetB,ParameterValue=PUBLIC_SUBNET_ID_2 \
ParameterKey=AuthSubnetA,ParameterValue=PRIVATE_SUBNET_ID_1 \
ParameterKey=AuthSubnetB,ParameterValue=PRIVATE_SUBNET_ID_2 \
ParameterKey=NodeSubnetA,ParameterValue=PRIVATE_SUBNET_ID_3 \
ParameterKey=NodeSubnetB,ParameterValue=PRIVATE_SUBNET_ID_4 \
ParameterKey=KeyName,ParameterValue=KeyName \
ParameterKey=DomainName,ParameterValue=teleport.example.com \
ParameterKey=DomainAdminEmail,ParameterValue=admin@example.com \
ParameterKey=HostedZoneID,ParameterValue=AWS_ZONE_ID"
make create-stack
```

## Usage instructions

After stack has been provisioned, login to the AWS Console and capture the IP address of a Proxy Server and a Auth Server then type the following to add a admin user:

```
ssh -i key.pem -o ProxyCommand="ssh -i key.pem -W %h:%p ec2-user@PROXY_SERVER" ec2-user@$AUTH_SERVER

# For OSS

sudo -u teleport tctl users add bob bob

# For Enterprise

sudo -u teleport tctl users add bob --roles=admin
```

