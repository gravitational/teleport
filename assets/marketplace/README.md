# Teleport AWS AMI creation.

Instructions for building Teleport AWS AMIs. 

## Development instructions

**Prerequisites**

AWS CLI and Packer are required to build Teleport AMIs.

Minimum versions:  
awscli == 1.14  
packer == v1.4.0 

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

1. Determine which AWS account you wish to build the AMI within. 
2. Ensure your local awscli profile is configured for this account, and you have adequate IAM roles to build AMIs (ec2, s3, vpc). 
3. Decide which region you'd like to build and distribute AMIs in. We'll use these in the next step.
4. Set the following Makefile values:

| Param               | Description                                                                                                 |
|---------------------|-------------------------------------------------------------------------------------------------------------|
| BUILD_VPC_ID        | With the region you selected in step 3, create or use an existing VPC. ex. `vpc-xxxxxxxx`.                  |
| BUILD_SUBNET_ID     | Within the VPC above, select a subnet. ex. `subnet-xxxxxxxx`                                                |
| AWS_REGION          | Region you selected in step 3. ex. `us-east-1`                                                              |
| TELEPORT_VERSION    | Teleport version. See [Teleport releases](https://github.com/gravitational/teleport/releases). ex. `4.2.10` |
| INSTANCE_TYPE       | The instance type used for the build. ex. `t2.micro`                                                        |
| DESTINATION_REGIONS | The regions the AMI will be replicated to. ex. `us-east-1,us-east-2`                                        |
| S3_BUCKET_ID        | The S3 bucket used for AMI distribution.                                                                    |

5. Run 
```
make oss
```

6. Once complete, your AMI should be available, in the regions you specified, with the name  `teleport-debug-ami-<type>-<version>`. (e.g. teleport-debug-ami-oss-4.2.10)

## Usage instructions

To use the AMI, see the [AMI guide](https://gravitational.com/teleport/docs/aws_oss_guide/#single-oss-teleport-amis-manual-gui-setup) in the docs. 
