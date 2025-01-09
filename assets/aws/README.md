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
| TELEPORT_VERSION    | Teleport version. See [Teleport releases](https://github.com/gravitational/teleport/releases). ex. `4.2.10` |
| DESTINATION_REGIONS | The regions the AMI will be replicated to. ex. `us-east-1,us-east-2`                                        |

5. Run 
```
make oss
```

6. Once complete, your AMI should be available, in the regions you specified, with the name  `teleport-<type>-<version>-<arch>`. (e.g. teleport-oss-4.2.10-arm64)

## Usage instructions

To see how to use your Teleport AMI to run a single-instance Teleport cluster,
read our [Getting Started Guide](https://goteleport.com/docs/get-started).

You can use your Teleport AMI to deploy EC2 instances running any Teleport
service. To read how to join your instance to a Teleport cluster in order to
protect resources in your infrastructure, see our [Joining Services to a
Cluster](https://goteleport.com/docs/enroll-resources/agents/join-services-to-your-cluster/)
guides. 

If you are hosting the Teleport Auth and Proxy Services yourself, [read our
guide](https://goteleport.com/docs/deploy-a-cluster/high-availability/) to
designing a high-availability architecture for your Teleport deployment.
