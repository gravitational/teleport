# AWS CloudFormation based provisioning example.

**Prerequisites** 

[AWS CLI](https://aws.amazon.com/cli/) is required to build and launch a CloudFormation stack.


## Launch VPC
In this example Teleport requires a VPC to install into.

```bash
export STACK=teleport-test-cf-vpc
export STACK_PARAMS="\
ParameterKey=DomainName,ParameterValue=DOMAIN-REPLACE \
ParameterKey=HostedZoneID,ParameterValue=HOSTZONE-REPLACE \
ParameterKey=DomainAdminEmail,ParameterValue=DOMAINEMAIL-REPLACE \
ParameterKey=KeyName,ParameterValue=SSHKEYNAME-REPLACE"
make create-stack-vpc
```


## Launch Teleport Cluster

TODO
- [ ] Do I still need a root cert if using ACM?
- [ ] How can the AMI `/usr/bin/teleport-generate-config` set systemd to run in --insecure mode. 


```bash
export STACK=teleport-test-cf-build-servers
export STACK_PARAMS="\
ParameterKey=VPC,ParameterValue=EXISTING_VPC_ID \
ParameterKey=ProxySubnetA,ParameterValue=PUBLIC_SUBNET_ID_1 \
ParameterKey=ProxySubnetB,ParameterValue=PUBLIC_SUBNET_ID_2 \
ParameterKey=AuthSubnetA,ParameterValue=PRIVATE_SUBNET_ID_1 \
ParameterKey=AuthSubnetB,ParameterValue=PRIVATE_SUBNET_ID_2 \
ParameterKey=NodeSubnetA,ParameterValue=PRIVATE_SUBNET_ID_3 \
ParameterKey=NodeSubnetB,ParameterValue=PRIVATE_SUBNET_ID_4  \
ParameterKey=KeyName,ParameterValue=benarentpub \
ParameterKey=DomainName,ParameterValue=DOMAIN_NAME \
ParameterKey=DomainNameNlb,ParameterValue=DOMAIN_NLB \
ParameterKey=LoadBalancerCertificateArn,ParameterValue=arn:aws:acm:us-west-2:115871037100:certificate/006ea504-6606-40a2-b9c3-8805ae2fffb6 \
ParameterKey=HostedZoneID,ParameterValue=HOSTZONEID" 
make create-stack 
```

