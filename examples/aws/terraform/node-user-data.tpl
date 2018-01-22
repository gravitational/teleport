#!/bin/bash
set -x

# Install uuid used for token generation
apt-get install -y uuid

# Setup teleport data dir used for transient storage
mkdir -p /var/lib/teleport/
chown -R root:adm /var/lib/teleport

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

# install python to get access to SSM to fetch proxy join token
curl $${CURL_OPTS} -O https://bootstrap.pypa.io/get-pip.py
python2.7 get-pip.py
pip install awscli

# Setup teleport proxy server config file
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

aws ssm get-parameter --with-decryption --name /teleport/${cluster_name}/tokens/node --region ${region} --query Parameter.Value --output text | xargs echo -n > /var/lib/teleport/token

EOF
chmod 755 /usr/local/bin/teleport-ssm-get-token

cat >/etc/teleport.yaml <<EOF
teleport:
  auth_token: /var/lib/teleport/token
  nodename: $${LOCAL_HOSTNAME}
  advertise_ip: $${LOCAL_IP}
  log:
    output: stderr
    severity: INFO

  data_dir: /var/lib/teleport
  auth_servers:
    - ${auth_server_addr}

auth_service:
  enabled: no

ssh_service:
  enabled: yes
  listen_addr: 0.0.0.0:3022

proxy_service:
  enabled: no
EOF

# Install and start teleport systemd unit
cat >/etc/systemd/system/teleport.service <<EOF
[Unit]
Description=Teleport SSH Service
After=network.target 

[Service]
User=root
Group=adm
Type=simple
Restart=always
RestartSec=5
ExecStartPre=/usr/local/bin/teleport-ssm-get-token
ExecStart=/usr/local/bin/teleport start --config=/etc/teleport.yaml
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF
systemctl enable teleport
systemctl start teleport

