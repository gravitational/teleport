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

**Launch a dev cloudformation stack which creates its own VPC**

Fill out the values in the second line for `STACK_PARAMS` then run `make create-stack`.

```
export STACK=test1
export STACK_PARAMS="ParameterKey=KeyName,ParameterValue=KeyName ParameterKey=DomainName,ParameterValue=teleport.example.com ParameterKey=DomainAdminEmail,ParameterValue=admin@example.com ParameterKey=HostedZoneID,ParameterValue=AWSZONEID"
make create-stack
```

**Launch a dev cloudformation stack using an existing VPC**

When using an existing VPC it must have both DNS support and DNS hostnames enabled.

The deployment needs six CIDR subnets provided which are inside the VPC's CIDR range. Each subnet needs at least two addresses available.

```
export STACK=test1
export STACK_PARAMS="\
ParameterKey=VPC,ParameterValue=EXISTINGVPCID \
ParameterKey=CIDRVPC,ParameterValue=10.0.0.0/16 \
ParameterKey=CIDRProxyA,ParameterValue=10.0.252.0/25 \
ParameterKey=CIDRProxyB,ParameterValue=10.0.252.128/25 \
ParameterKey=CIDRAuthA,ParameterValue=10.0.253.0/25 \
ParameterKey=CIDRAuthB,ParameterValue=10.0.253.128/25 \
ParameterKey=CIDRNodeA,ParameterValue=10.0.254.0/25 \
ParameterKey=CIDRNodeB,ParameterValue=10.0.254.128/25 \
ParameterKey=KeyName,ParameterValue=KeyName \
ParameterKey=DomainName,ParameterValue=teleport.example.com \
ParameterKey=DomainAdminEmail,ParameterValue=admin@example.com \
ParameterKey=HostedZoneID,ParameterValue=AWSZONEID"
make create-stack-existing-vpc
```

## Usage instructions

After stack has been provisioned, login to the AWS Console and capture the IP address of a Proxy Server and a Auth Server then type the following to add a admin user:

```
ssh -i key.pem -o ProxyCommand="ssh -i key.pem -W %h:%p ec2-user@PROXY_SERVER" ec2-user@$AUTH_SERVER
```

# For OSS

```
sudo -u teleport tctl users add bob bob
```

# For Enterprise

```
sudo -u teleport tctl users add bob --roles=admin
```

