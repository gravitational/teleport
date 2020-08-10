# Teleport AWS Quickstart Guide (Single AMI)

AWS Quickstart for Teleport using a single AMI.

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

**To build the OSS AMI**

```
make oss
```

## Usage instructions

After stack has been provisioned, login to the AWS Console and get the IP address of the server.

```
ssh -i key.pem ec2-user@$SERVER_IP

# For OSS

sudo -u teleport tctl users add bob bob

# For Enterprise

sudo -u teleport tctl users add bob --roles=admin
```

