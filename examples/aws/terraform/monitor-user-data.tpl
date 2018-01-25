#!/bin/bash
set -x

# Install uuid used for token generation, nginx for grafana frontend
apt-get install -y uuid nginx

# Set some curl options so that temporary failures get retried
# More info: https://ec.haxx.se/usingcurl-timeouts.html
CURL_OPTS="-L --retry 100 --retry-delay 0 --connect-timeout 10 --max-time 300"

# Install python to get access to S3
curl $${CURL_OPTS} -O https://bootstrap.pypa.io/get-pip.py
python2.7 get-pip.py
pip install awscli

# Install InfluxDB
curl $CURL_OPTS -o /tmp/influxdb.deb https://dl.influxdata.com/influxdb/releases/influxdb_${influxdb_version}_amd64.deb
dpkg -i /tmp/influxdb.deb
rm -f /tmp/influxdb.deb

# Install grafana
apt-get install -y adduser libfontconfig
curl $CURL_OPTS -o /tmp/grafana.deb https://s3-us-west-2.amazonaws.com/grafana-releases/release/grafana_${grafana_version}_amd64.deb
dpkg -i /tmp/grafana.deb
rm -f /tmp/grafana.deb

# Start services
systemctl enable grafana-server
systemctl start grafana-server

systemctl enable influxdb.service
systemctl restart influxdb.service

# Import dashboard and data source
until $(curl --output /dev/null --silent --head --fail http://localhost:3000); do
    echo 'waiting for grafana to respond'
    sleep 5
done

echo "Grafana is up setting up dashboards and data sources"

GRAFANA_PASS="`aws ssm get-parameter --with-decryption --name /teleport/${cluster_name}/grafana_pass --region ${region} --query 'Parameter.Value' --output text`"

# Change grafana password
curl -X PUT -H "Content-Type: application/json" -d "{
  \"oldPassword\": \"admin\",
  \"newPassword\": \"$${GRAFANA_PASS}\",
  \"confirmNew\": \"$${GRAFANA_PASS}\"
}" http://admin:admin@127.0.0.1:3000/api/user/password

# Set up default input
curl -s -H "Content-Type: application/json" \
    -XPOST http://admin:$${GRAFANA_PASS}@127.0.0.1:3000/api/datasources \
    -d @- <<EOF
{
    "name": "InfluxDB",
    "type": "influxdb",
    "access": "proxy",
    "url": "http://127.0.0.1:8086",
    "database": "telegraf"
}
EOF

# Download and setup teleport dashboard
aws s3 --region=${region} cp s3://${s3_bucket}/health-dashboard.json /tmp/health-dashboard.json
curl -X POST -d @/tmp/health-dashboard.json http://admin:$${GRAFANA_PASS}@127.0.0.1:3000/api/dashboards/db --header 'Content-Type: application/json'

# Setup nginx frontend for grafana
mkdir -p /etc/tls/certs
cat >/lib/systemd/system/nginx.service <<EOF
[Unit]
Description=A high performance web server and a reverse proxy server
Documentation=man:nginx(8)
After=network.target

[Service]
Restart=always
RestartSec=5
Type=forking
PIDFile=/run/nginx.pid
ExecStartPre=/usr/local/bin/aws s3 sync s3://${s3_bucket}/live/${domain_name} /etc/tls/certs
ExecStartPre=/usr/sbin/nginx -t -q -g 'daemon on; master_process on;'
ExecStart=/usr/sbin/nginx -g 'daemon on; master_process on;'
ExecReload=/usr/sbin/nginx -g 'daemon on; master_process on;' -s reload
ExecStop=-/sbin/start-stop-daemon --quiet --stop --retry QUIT/5 --pidfile /run/nginx.pid
TimeoutStopSec=5
KillMode=mixed

[Install]
WantedBy=multi-user.target
EOF

# Update nginx configuration to use TLS and frontend grafana and restart nginx
aws s3 --region=${region} cp s3://${s3_bucket}/grafana-nginx.conf /etc/nginx/nginx.conf
systemctl daemon-reload
systemctl restart nginx --no-block
