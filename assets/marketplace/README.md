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

Once the above command is complete, it will output something like the following:

```
--> Teleport AWS Linux: AMIs were created:
eu-west-1: ami-00000000
us-east-1: ami-11111111
us-east-2: ami-22222222
us-west-2: ami-33333333
```

Update `oss.yaml` these AMI IDs `Mappings -> AWSRegionArch2AMI`.

**Launch a dev CloudFormation stack**

Fill out the values in the second line for `STACK_PARAMS` then run `make create-stack`.

```
export STACK=test1
export STACK_PARAMS="ParameterKey=KeyName,ParameterValue=KeyName ParameterKey=DomainName,ParameterValue=teleport.example.com ParameterKey=DomainAdminEmail,ParameterValue=admin@example.com ParameterKey=HostedZoneID,ParameterValue=AWSZONEID"
make create-stack
```

## Usage instructions

After stack has been provisioned, login to the AWS Console and capture the IP address of a Proxy Server and a Auth Server then type the following to add a admin user: 

```
ssh -i key.pem -o ProxyCommand="ssh -i key.pem -W %h:%p ec2-user@PROXY_SERVER" ec2-user@$AUTH_SERVER

# For OSS.
sudo -u teleport tctl users add bob bob

# For Enterprise.
sudo -u teleport tctl users add bob --roles=admin
```
