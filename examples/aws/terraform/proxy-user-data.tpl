#!/bin/bash
set -x

# Install uuid used for token generation
apt-get install -y uuid

# Create teleport user
useradd -r teleport
adduser teleport adm

# Setup teleport data dir used for transient storage
mkdir -p /var/lib/teleport/
chown -R teleport:adm /var/lib/teleport

# Set some curl options so that temporary failures get retried
# More info: https://ec.haxx.se/usingcurl-timeouts.html
CURL_OPTS="-L --retry 100 --retry-delay 0 --connect-timeout 10 --max-time 300"

# Download and install teleport
pushd /tmp
curl $${CURL_OPTS} -o teleport.tar.gz https://get.gravitational.com/teleport/${teleport_version}/teleport-ent-v${teleport_version}-linux-amd64-bin.tar.gz
tar -xzf /tmp/teleport.tar.gz
cp teleport-ent/tctl teleport-ent/tsh teleport-ent/teleport /usr/local/bin
rm -rf /tmp/teleport.tar.gz /tmp/teleport-ent
popd

# Install python to get access to SSM to fetch proxy join token
curl $${CURL_OPTS} -O https://bootstrap.pypa.io/get-pip.py
python2.7 get-pip.py
pip install awscli
EC2_AVAIL_ZONE=`curl $${CURL_OPTS} -s http://169.254.169.254/latest/meta-data/placement/availability-zone`
EC2_REGION="`echo \"$EC2_AVAIL_ZONE\" | sed -e 's:\([0-9][0-9]*\)[a-z]*\$:\\1:'`"
PROXY_TOKEN="`aws ssm get-parameter --with-decryption --name /teleport/${cluster_name}/tokens/proxy --region $EC2_REGION --query 'Parameter.Value' --output text`"
chown -R teleport:adm /var/lib/teleport/license.pem

# Setup teleport proxy server config file
CLUSTER_NAME="${cluster_name}"
LOCAL_IP=`curl http://169.254.169.254/latest/meta-data/local-ipv4`
LOCAL_HOSTNAME=`curl http://169.254.169.254/latest/meta-data/local-hostname`

# Install a service that fetches SSM token from parameter store
# Note that in this scenario token is written to the file.
# Script does not attempt to fetch token during boot, because the tokens are published after
# Auth servers are started.
cat >/usr/local/bin/teleport-ssm-get-token <<EOF
#!/bin/bash
set -e
set -o pipefail

aws ssm get-parameter --with-decryption --name /teleport/${cluster_name}/tokens/proxy --region ${region} --query Parameter.Value --output text | xargs echo -n > /var/lib/teleport/token

EOF
chmod 755 /usr/local/bin/teleport-ssm-get-token

cat >/etc/teleport.yaml <<EOF
teleport:
  auth_token: /var/lib/teleport/token
  nodename: $${LOCAL_HOSTNAME}
  advertise_ip: $${LOCAL_IP}
  log:
    output: stderr
    severity: DEBUG

  data_dir: /var/lib/teleport
  auth_servers:
    - ${auth_server_addr}

auth_service:
  enabled: no

ssh_service:
  enabled: no

proxy_service:
  enabled: yes
  listen_addr: 0.0.0.0:3023
  tunnel_listen_addr: 0.0.0.0:3080
  web_listen_addr: 0.0.0.0:3080
  public_addr: ${domain_name}:443
  https_cert_file: /var/lib/teleport/fullchain.pem
  https_key_file: /var/lib/teleport/privkey.pem
EOF

# Install and start teleport systemd unit
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
ExecStartPre=/usr/local/bin/teleport-ssm-get-token
ExecStartPre=/usr/local/bin/aws s3 sync s3://${s3_bucket}/live/${domain_name} /var/lib/teleport
ExecStart=/usr/local/bin/teleport start --config=/etc/teleport.yaml
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF
systemctl enable teleport
systemctl start teleport

