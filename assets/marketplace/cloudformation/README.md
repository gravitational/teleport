# Teleport AWS Quickstart Guide (CloudFormation)

#### NOTE: This method will no longer be updated - it's provided as an example for anyone wanting to use CloudFormation with AWS  

## Development instructions

**Prerequisites** 

AWS CLI and Packer are required to build and launch a CloudFormation stack.

On MacOS:

```bash
brew install awscli
brew install packer
```

On Linux:

```bash
apt install awscli
```

Follow instructions at: https://www.packer.io/docs/install/index.html

### To build an AMI

**OSS** 

```bash
make oss
```

**Enterprise**

If you have your Teleport Enterprise license in S3, you can provide the URI via the `TELEPORT_LICENSE_URI` parameter:

```bash
TELEPORT_LICENSE_URI=s3://s3.bucket/teleport-enterprise-license.pem make ent
```

You must have your AWS account credentials configured to be able to run `aws s3 cp`.

Alternatively, copy your license file to `files/system/license.pem` and run `make ent` without any additional parameters:

```bash
cp /home/user/teleport-enterprise-license.pem files/system/license.pem
make ent
```

#### Update YAML files with the new AMI image IDs

##### OSS

```bash
make update-ami-ids-oss
```

##### Enterprise

```bash
make update-ami-ids-ent
```


### Launch a dev CloudFormation stack using an existing VPC

When using an existing VPC it must have both DNS support and DNS hostnames enabled.

The deployment needs six VPC subnet IDs provided - two public (for the proxy) and four private (for auth and nodes).
For redundancy, the subnets should be split across availability zones - odd numbers in AZ A and even numbers in AZ B, for example.

Replace the placeholder values in the exports below.

```bash
export STACK=test1
export STACK_PARAMS="\
ParameterKey=VPC,ParameterValue=EXISTING_VPC_ID \
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

#### Usage instructions

After the stack has been provisioned, login to the AWS Console and capture the IP address of a Proxy Server and a Auth
Server, then type the following to add an admin user:

```bash
ssh -i key.pem -o ProxyCommand="ssh -i key.pem -W %h:%p ec2-user@PROXY_SERVER" ec2-user@$AUTH_SERVER
```

##### For OSS

```bash
sudo -u teleport tctl users add bob ec2-user
```

##### For Enterprise

```bash
sudo -u teleport tctl users add bob --roles=admin
```

