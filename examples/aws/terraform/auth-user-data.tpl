#!/bin/bash
set -x

# Install uuid used for random token generation
apt-get install -y uuid

# Create teleport user
useradd -r teleport
adduser teleport adm

# Mount EFS for audit logs storage.
# Teleport auth servers store audit logs on EFS shared file system
apt-get install -y nfs-common
mkdir -p /var/lib/teleport/log
echo "${efs_mount_point}:/ /var/lib/teleport/log nfs4 nfsvers=4.1,rsize=1048576,wsize=1048576,hard,timeo=600,retrans=2 0 0" >> /etc/fstab
until mount -a -t nfs4
do
    echo "mount failed, try again after 1 second"
    sleep 1
done
chown -R teleport:adm /var/lib/teleport

# Set some curl options so that temporary failures get retried
# More info: https://ec.haxx.se/usingcurl-timeouts.html
CURL_OPTS="-L --retry 100 --retry-delay 0 --connect-timeout 10 --max-time 300"

# Download and install teleport from official file server
pushd /tmp
curl $${CURL_OPTS} -o teleport.tar.gz https://get.gravitational.com/teleport/${teleport_version}/teleport-ent-v${teleport_version}-linux-amd64-bin.tar.gz
tar -xzf /tmp/teleport.tar.gz
cp teleport-ent/tctl teleport-ent/tsh teleport-ent/teleport /usr/local/bin
rm -rf /tmp/teleport.tar.gz /tmp/teleport-ent
popd

# Install python to get access to SSM to fetch license file
# that is distributed via SSM parameter store in encrypted form.
curl $${CURL_OPTS} -O https://bootstrap.pypa.io/get-pip.py
python2.7 get-pip.py
pip install awscli
EC2_AVAIL_ZONE=`curl $${CURL_OPTS} -s http://169.254.169.254/latest/meta-data/placement/availability-zone`
EC2_REGION="`echo \"$EC2_AVAIL_ZONE\" | sed -e 's:\([0-9][0-9]*\)[a-z]*\$:\\1:'`"
aws ssm get-parameter --with-decryption --name /teleport/${cluster_name}/license --region $EC2_REGION --query 'Parameter.Value' --output text > /var/lib/teleport/license.pem 
chown -R teleport:adm /var/lib/teleport/license.pem

# Setup teleport auth server config file
CLUSTER_NAME="${cluster_name}"
LOCAL_IP=`curl http://169.254.169.254/latest/meta-data/local-ipv4`
LOCAL_HOSTNAME=`curl http://169.254.169.254/latest/meta-data/local-hostname`
DYNAMO_TABLE_NAME="${dynamo_table_name}"

# Teleport Auth server is using DynamoDB as a backend
# On AWS, see dynamodb.tf for details
cat >/etc/teleport.yaml <<EOF
teleport:
  nodename: $${LOCAL_HOSTNAME}
  advertise_ip: $${LOCAL_IP}
  log:
    output: stderr
    severity: DEBUG

  data_dir: /var/lib/teleport
  storage:
    type: dynamodb
    region: $${EC2_REGION}
    table_name: $${DYNAMO_TABLE_NAME}

auth_service:
  enabled: yes
  listen_addr: 0.0.0.0:3025

  authentication:
    type: oidc

  cluster_name: $${CLUSTER_NAME}

ssh_service:
  enabled: no

proxy_service:
  enabled: no
EOF

# Install and start teleport systemd unit.
# Notice that Auth server does not need to run as root
# as it does not handle any interactive sessions,
# only serves HTTPs API server.
cat >/etc/systemd/system/teleport.service <<EOF
[Unit]
Description=Teleport SSH Service
After=network.target 

[Service]
User=teleport
Group=adm
Type=simple
Restart=always
RestartSec=5
ExecStart=/usr/local/bin/teleport start --config=/etc/teleport.yaml
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF
systemctl enable teleport
systemctl start teleport


# Install a service that rotates teleport join tokens.
# Teleport join tokens are temporary authentication tokens
# letting nodes and proxies to join to the cluster. Notice that timer
# unit rotates the current token every hour, but tokens are valid for 2 hours
# to handle timing cases.
cat >/usr/local/bin/teleport-ssm-publish-tokens <<EOF
#!/bin/bash
set -e
set -o pipefail

PROXY_TOKEN=\$$(uuid)
tctl nodes add --roles=proxy --ttl=2h --token=\$${PROXY_TOKEN}
aws ssm put-parameter --name /teleport/$${CLUSTER_NAME}/tokens/proxy --region $${EC2_REGION} --type="SecureString" --value="\$${PROXY_TOKEN}" --overwrite

NODE_TOKEN=\$$(uuid)
tctl nodes add --roles=node --ttl=2h --token=\$${NODE_TOKEN}
aws ssm put-parameter --name /teleport/$${CLUSTER_NAME}/tokens/node --region $${EC2_REGION} --type="SecureString" --value="\$${NODE_TOKEN}" --overwrite
EOF
chmod 755 /usr/local/bin/teleport-ssm-publish-tokens

# Install and start rotate systemd unit that publishes tokens to SSM
cat >/etc/systemd/system/teleport-ssm-publish-tokens.service <<EOF
[Unit]
Description=Service rotating teleport tokens

[Service]
Type=oneshot
ExecStart=/usr/local/bin/teleport-ssm-publish-tokens
EOF

# Install and start rotate systemd timer unit.
cat >/etc/systemd/system/teleport-ssm-publish-tokens.timer <<EOF
[Unit]
Description=Timer rotating teleport tokens in SSM

[Timer]
OnBootSec=5min
OnCalendar=hourly
Persistent=true
EOF
systemctl enable teleport-ssm-publish-tokens.service teleport-ssm-publish-tokens.timer
systemctl start teleport-ssm-publish-tokens.timer


# Install certbot to rotate certificates
# Certbot is a tool to request letsencrypt certificates,
# remove it if you don't need letsencrypt.
pip install certbot==0.21.0 certbot-dns-route53==0.21.0

# This script uses DNS-01 challenge, which means that you
# have to control route53 zone as it modifes zone records
# to prove to letsencrypt that you own the domain.
cat >/usr/local/bin/teleport-get-cert <<EOF
#!/bin/bash
set -e
set -x

PATH_TO_CHECK="s3://${s3_bucket}/live/${domain_name}/fullchain.pem"
S3_BUCKET="${s3_bucket}"
DOMAIN="${domain_name}"
EMAIL="${email}"

has_fullchain=\$$(aws s3 ls \$${PATH_TO_CHECK} | wc -l)
if [ \$$has_fullchain -gt 0 ]
then
  echo "\$${PATH_TO_CHECK} already exists"
  exit 0
fi

echo "No certs/keys found in \$${S3_BUCKET}. Going to request certificate for \$${DOMAIN}."
certbot certonly -n --agree-tos --email \$${EMAIL} --dns-route53 -d \$${DOMAIN}
echo "Got certificate for \$${DOMAIN}. Syncing to S3."

aws s3 sync /etc/letsencrypt/ s3://\$${S3_BUCKET} --sse=AES256
EOF
chmod 755 /usr/local/bin/teleport-get-cert


# Install and start rotate systemd unit to get certs.
cat >/etc/systemd/system/teleport-get-cert.service <<EOF
[Unit]
Description=Service getting teleport certificates

[Service]
Type=oneshot
ExecStart=/usr/local/bin/teleport-get-cert
EOF


# Install and start rotate systemd timer (in case of failure), just run it every hour
# (the script will be extended to update certificates as well)
cat >/etc/systemd/system/teleport-get-cert.timer <<EOF
[Unit]
Description=Timer getting letsencrypt certificates

[Timer]
OnBootSec=5min
OnCalendar=hourly
Persistent=true
EOF
systemctl enable teleport-get-cert.service teleport-get-cert.timer
systemctl start teleport-get-cert.timer
