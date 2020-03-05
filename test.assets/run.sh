#!/bin/bash

set -eu -o pipefail

PREFIX=rjones
REGION=us-east-1
GO_VERSION=1.13.2
SECURITY_GROUP_NAME=$PREFIX-tmp

rm -fr $PREFIX-*

# Create VPC that EC2 instance will be launched into.
VPC_ID=$(aws ec2 create-vpc \
    --cidr-block 10.0.0.0/16 \
    --region $REGION | jq -r ".Vpc.VpcId")
echo "Created VPC: $VPC_ID."

# Configure the VPC to automatically assign DNS hostnames to instances.
aws ec2 modify-vpc-attribute \
    --vpc-id $VPC_ID \
    --enable-dns-support "{\"Value\":true}" \
    --region $REGION > /dev/null 2>&1
aws ec2 modify-vpc-attribute \
    --vpc-id $VPC_ID \
    --enable-dns-hostnames "{\"Value\":true}" \
    --region $REGION > /dev/null 2>&1

# Create a subnet in the VPC.
SUBNET_ID=$(aws ec2 create-subnet \
    --vpc-id $VPC_ID \
    --cidr-block 10.0.1.0/24 \
    --region $REGION | jq -r ".Subnet.SubnetId")
echo "Created Subnet: $SUBNET_ID."

# Create and attach a internet gateway to VPC.
GATEWAY_ID=$(aws ec2 create-internet-gateway \
    --region $REGION | jq -r ".InternetGateway.InternetGatewayId")
aws ec2 attach-internet-gateway \
    --vpc-id $VPC_ID \
    --internet-gateway-id $GATEWAY_ID \
    --region $REGION > /dev/null 2>&1
echo "Created Internet Gateway: $GATEWAY_ID."

# Create route table that points all subnet traffic to the internet gateway.
TABLE_ID=$(aws ec2 create-route-table \
    --vpc-id $VPC_ID \
    --region $REGION | jq -r ".RouteTable.RouteTableId")
aws ec2 create-route \
    --route-table-id $TABLE_ID \
    --gateway-id $GATEWAY_ID \
    --destination-cidr-block 0.0.0.0/0 \
    --region $REGION > /dev/null 2>&1
aws ec2 associate-route-table \
    --subnet-id $SUBNET_ID \
    --route-table-id $TABLE_ID \
    --region $REGION > /dev/null 2>&1
echo "Created Route Table: $TABLE_ID."

# Create security group tht EC2 instance will be launched into.
SG_ID=$(aws ec2 create-security-group \
    --group-name $SECURITY_GROUP_NAME \
    --vpc-id $VPC_ID \
    --description "Security group to test SSRF." \
    --region $REGION | jq -r ".GroupId")
echo "Created Security Group: $SG_ID."

# Configure security group to allow SSH traffic in to EC2 instance.
aws ec2 authorize-security-group-ingress \
    --group-id $SG_ID \
    --protocol tcp \
    --port 22 \
    --cidr 0.0.0.0/0 \
    --region $REGION

# Create a temporary keypairs for this test run.
ssh-keygen -t ed25519 -q -N "" -C "" -f $PREFIX-user-key
ssh-keygen -t ed25519 -q -N "" -C "" -f $PREFIX-host-key
USER_PUBLIC_KEY=$(cat $PREFIX-user-key.pub)
HOST_PUBLIC_KEY=$(cat $PREFIX-host-key.pub)
HOST_PRIVATE_KEY=$(cat $PREFIX-host-key)
echo "Created User Public Key: $USER_PUBLIC_KEY"
echo "Created Host Public Key: $HOST_PUBLIC_KEY"

# Create known_hosts file.
echo "* $HOST_PUBLIC_KEY" > $PREFIX-known_hosts

# Create user-data file that allows access with the above temporary keypair.
cat << EOF > $PREFIX-userdata
#!/bin/bash
mkdir -p /root/.ssh
echo "$USER_PUBLIC_KEY" >> /root/.ssh/authorized_keys
chmod 600 /root/.ssh/authorized_keys

echo "$HOST_PUBLIC_KEY" > /etc/ssh/ssh_host_ed25519_key.pub
echo "$HOST_PRIVATE_KEY" > /etc/ssh/ssh_host_ed25519_key
echo "HostKey /etc/ssh/ssh_host_ed25519_key" >> /etc/ssh/sshd_config
systemctl restart ssh
EOF
    #--metadata-options "HttpEndpoint=disabled" \
    #--key-name ops \

# Create an EC2 instance.
INSTANCE_ID=$(aws ec2 run-instances \
    --image-id ami-07ebfd5b3428b6f4d \
    --count 1 \
    --instance-type t2.micro \
    --key-name ops \
    --security-group-ids $SG_ID \
    --subnet-id $SUBNET_ID \
    --associate-public-ip-address \
    --user-data file://$PWD/$PREFIX-userdata \
    --region $REGION | jq -r ".Instances[0].InstanceId")
echo "Created Instance: $INSTANCE_ID."

# Wait for the instance to start.
RUNNING=""
until [ "$RUNNING" = "running" ]
do
    RUNNING=$(aws ec2 describe-instance-status \
        --instance-ids $INSTANCE_ID \
        --region $REGION | jq -r ".InstanceStatuses[0].InstanceState.Name")
    echo "Waiting for instance to start..."
    sleep 5
done

# Fetch public IP of EC2 instance.
PUBLIC_IP=$(aws ec2 describe-instances \
    --instance-ids $INSTANCE_ID \
    --region $REGION | jq -r .Reservations[0].Instances[0].PublicIpAddress)
echo "Instance IP Address $PUBLIC_IP."

# Create BPF testing script.
cat << EOF > $PREFIX-test-bpf.sh
#!/bin/bash

set -eu -o pipefail

# Check that TELEPORT_TAG is passed in.
if [ \$# -eq 0 ]; then
   echo "Usage: \$(basename \$0) TELEPORT_TAG"
   exit 1
fi
TELEPORT_TAG=\$1
echo "TELEPORT_TAG: \${TELEPORT_TAG}"

# Download and install development tooling, kernel headers, and bcc-tools.
if [ -f /etc/redhat-release ];
then
   yum -y update
   yum -y groupinstall "Development Tools"
   yum -y install kernel-headers bcc-tools
else
   apt update
   apt install -y build-essential linux-headers-$(uname -r) bpfcc-tools
fi

# Download and install Go.
curl -LO https://dl.google.com/go/go${GO_VERSION}.linux-amd64.tar.gz
tar -C /usr/local -xzf go${GO_VERSION}.linux-amd64.tar.gz

# Update environment variables.
export PATH=/usr/local/go/bin:\$PATH

# Create directory structure needed to test teleport
ORG_DIR="/root/go/src/github.com/gravitational"
mkdir -p \${ORG_DIR}

# Clone Teleport.
cd \${ORG_DIR}
git clone https://github.com/gravitational/teleport.git
cd \${ORG_DIR}/teleport
git fetch origin master:refs/remotes/origin/master +refs/pull/*:refs/remotes/origin/pr/*
git checkout \${TELEPORT_TAG}

# Run the BPF tests specifically.
cd lib/bpf && go test -v -tags="bpf" ./...
make test-bpf
make integration-bpf
EOF
chmod +x $PREFIX-test-bpf.sh

echo ""
echo "To connect to the host, type \"ssh -o IdentityFile=$PREFIX-user-key -o UserKnownHostsFile=$PREFIX-known_hosts root@$PUBLIC_IP\""

echo ""
echo "To cleanup, run the following commands:"
echo "until aws ec2 terminate-instances --instance-ids $INSTANCE_ID --region $REGION; do sleep 5; done"
echo "until aws ec2 delete-security-group --group-id $SG_ID --region $REGION; do sleep 5; done"
echo "until aws ec2 delete-subnet --subnet-id $SUBNET_ID --region $REGION; do sleep 5; done"
echo "until aws ec2 delete-route-table --route-table-id $TABLE_ID --region $REGION; do sleep 5; done"
echo "until aws ec2 detach-internet-gateway --internet-gateway-id $GATEWAY_ID --vpc-id $VPC_ID --region $REGION; do sleep 5; done"
echo "until aws ec2 delete-internet-gateway --internet-gateway-id $GATEWAY_ID --region $REGION; do sleep 5; done"
echo "until aws ec2 delete-vpc --vpc-id $VPC_ID --region $REGION; do sleep 5; done"
echo ""

# Copy over test script.
scp -o "IdentityFile=$PREFIX-user-key" \
    -o "UserKnownHostsFile=$PREFIX-known_hosts" \
    $PREFIX-test-bpf.sh root@$PUBLIC_IP:run-test-bpf.sh

# Run test script.
ssh -o "IdentityFile=$PREFIX-user-key" \
    -o "UserKnownHostsFile=$PREFIX-known_hosts" \
    root@$PUBLIC_IP /root/run-test-bpf.sh
