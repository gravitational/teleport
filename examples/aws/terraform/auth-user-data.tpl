#!/bin/bash
set -x

# Install uuid used for random token generation
apt-get install -y uuid

# Set some curl options so that temporary failures get retried
# More info: https://ec.haxx.se/usingcurl-timeouts.html
CURL_OPTS="-L --retry 100 --retry-delay 0 --connect-timeout 10 --max-time 300"

# Install telegraf to collect stats from influx
curl $CURL_OPTS -o /tmp/telegraf.deb https://dl.influxdata.com/telegraf/releases/telegraf_${telegraf_version}_amd64.deb
dpkg -i /tmp/telegraf.deb
rm -f /tmp/telegraf.deb

# Create teleport user. It is helpful to share the same UID
# to have the same permissions on shared NFS volumes across auth servers and for consistency.
useradd -r teleport -u ${teleport_uid}
adduser teleport adm

# Setup teleport run dir for pid files
mkdir -p /var/run/teleport/
chown -R teleport:adm /var/run/teleport

# Setup teleport auth server config file
LOCAL_IP=`curl http://169.254.169.254/latest/meta-data/local-ipv4`
LOCAL_HOSTNAME=`curl http://169.254.169.254/latest/meta-data/local-hostname`

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

# Set host UUID so auth server picks it up, as each auth server's
# logs are stored in individual folder /var/lib/teleport/log/<host_uuid>/
# and it will be easy to log forwarders to locate them on every auth server
# note though, that host_uuid MUST be unique, otherwise all sorts of unintended
# things will happen.
echo $${LOCAL_HOSTNAME} > /var/lib/teleport/host_uuid
chown -R teleport:adm /var/lib/teleport

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
aws ssm get-parameter --with-decryption --name /teleport/${cluster_name}/license --region ${region} --query 'Parameter.Value' --output text > /var/lib/teleport/license.pem 
chown -R teleport:adm /var/lib/teleport/license.pem

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
    region: ${region}
    table_name: ${dynamo_table_name}

auth_service:
  enabled: yes
  listen_addr: 0.0.0.0:3025

  authentication:
    second_factor: off
    type: oidc

  cluster_name: ${cluster_name}

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
ExecStart=/usr/local/bin/teleport start --config=/etc/teleport.yaml --diag-addr=127.0.0.1:3434 --pid-file=/var/run/teleport/teleport.pid
ExecReload=/bin/kill -HUP \$$MAINPID
PIDFile=/var/run/teleport/teleport.pid
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF
systemctl enable teleport
systemctl start teleport


# Script that makes sure that only one auth server processes
# requests at a time, by using dynamodb-backed locking.
# The lock is implemented as item in DynamoDB table:  {"Lock": "lock1", "Expires": "time", "Process": "server1"}
# The auth server node either renews the lease if lock "Process" holds the server id as owner of the lock
# or grabs the lock in case if expires column indicates that the lease has not been renewed after timeout.
# This pattern can be implemented in many different ways, e.g. using ASG group of 1 as a separate process
# or in Kubernetes as a deployment of scale 1.
cat >/usr/local/bin/teleport-lock <<EOF
#!/bin/bash
set -x

LOCK="/teleport/${cluster_name}"
NOW=\$$(date +%s)
TTL=\$$((\$$NOW+3660))
PROCESS="$${LOCAL_HOSTNAME}"
echo locking \$$PROCESS for \$$TTL

# Either renew the lease if agent still holds it, or grab the lease if it's expired
aws dynamodb put-item \
    --region ${region} \
    --table-name ${locks_table_name}\
    --item  "{\"Lock\": {\"S\": \"/auth/servers\"}, \"Expires\": {\"S\": \"\$$TTL\"}, \"Process\": {\"S\": \"\$$PROCESS\"}}" \
    --condition-expression="(attribute_not_exists(Expires) OR Expires <= :timestamp) OR Process = :process"\
    --expression-attribute-values "{\":timestamp\":{\"S\":\"\$$NOW\"}, \":process\":{\"S\":\"\$$PROCESS\"}}"

if [ \$$? -eq 0 ]; then
    echo "Renewed or locked the lease for \$$PROCESS until $(date -d @\$$TTL)"
else
    echo "Could get renew lease, locked by other process"
    exit 255
fi
EOF
chmod 755 /usr/local/bin/teleport-lock

# Install a service that rotates teleport join tokens.
# Teleport join tokens are temporary authentication tokens
# letting nodes and proxies to join to the cluster. Notice that timer
# unit rotates the current token every hour, but tokens are valid for 2 hours
# to handle timing cases.
cat >/usr/local/bin/teleport-ssm-publish-tokens <<EOF
#!/bin/bash
set -e
set -o pipefail

# Proxy token authenticates proxies joining the cluster
PROXY_TOKEN=\$$(uuid)
tctl nodes add --roles=proxy --ttl=4h --token=\$${PROXY_TOKEN}
aws ssm put-parameter --name /teleport/${cluster_name}/tokens/proxy --region ${region} --type="SecureString" --value="\$${PROXY_TOKEN}" --overwrite

# Node token authenticates nodes joining the cluster
NODE_TOKEN=\$$(uuid)
tctl nodes add --roles=node --ttl=4h --token=\$${NODE_TOKEN}
aws ssm put-parameter --name /teleport/${cluster_name}/tokens/node --region ${region} --type="SecureString" --value="\$${NODE_TOKEN}" --overwrite

# Export CA certificate to SSM parameter store
# so nodes and proxies can check the identity of the auth server they are connecting to
CERT=\$$(tctl auth export --type=tls)
aws ssm put-parameter --name /teleport/${cluster_name}/ca --region ${region} --type="String" --value="\$${CERT}" --overwrite

EOF
chmod 755 /usr/local/bin/teleport-ssm-publish-tokens

# Install and start rotate systemd unit that publishes tokens to SSM
cat >/etc/systemd/system/teleport-ssm-publish-tokens.service <<EOF
[Unit]
Description=Service rotating teleport tokens

[Service]
Type=oneshot
ExecStartPre=/usr/local/bin/teleport-lock
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
ExecStartPre=/usr/local/bin/teleport-lock
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

# Install teleport telegraf configuration
# Telegraf will collect prometheus metrics and send to influxdb collector
cat >/etc/telegraf/telegraf.conf <<EOF
# Configuration for telegraf agent
[agent]
  ## Default data collection interval for all inputs
  interval = "10s"
  ## Rounds collection interval to 'interval'
  ## ie, if interval="10s" then always collect on :00, :10, :20, etc.
  round_interval = true

  ## Telegraf will send metrics to outputs in batches of at
  ## most metric_batch_size metrics.
  metric_batch_size = 1000
  ## For failed writes, telegraf will cache metric_buffer_limit metrics for each
  ## output, and will flush this buffer on a successful write. Oldest metrics
  ## are dropped first when this buffer fills.
  metric_buffer_limit = 10000

  ## Collection jitter is used to jitter the collection by a random amount.
  ## Each plugin will sleep for a random time within jitter before collecting.
  ## This can be used to avoid many plugins querying things like sysfs at the
  ## same time, which can have a measurable effect on the system.
  collection_jitter = "0s"

  ## Default flushing interval for all outputs. You shouldn't set this below
  ## interval. Maximum flush_interval will be flush_interval + flush_jitter
  flush_interval = "10s"
  ## Jitter the flush interval by a random amount. This is primarily to avoid
  ## large write spikes for users running a large number of telegraf instances.
  ## ie, a jitter of 5s and interval 10s means flushes will happen every 10-15s
  flush_jitter = "0s"

  ## By default, precision will be set to the same timestamp order as the
  ## collection interval, with the maximum being 1s.
  ## Precision will NOT be used for service inputs, such as logparser and statsd.
  precision = ""
  ## Run telegraf in debug mode
  debug = false
  ## Run telegraf in quiet mode
  quiet = false
  ## Override default hostname, if empty use os.Hostname()
  hostname = ""
  ## If set to true, do no set the "host" tag in the telegraf agent.
  omit_hostname = false


###############################################################################
#                            INPUT PLUGINS                                    #
###############################################################################

[[inputs.procstat]]
  exe = "teleport"
  prefix = "teleport"
  
[[inputs.prometheus]]
  # An array of urls to scrape metrics from.
  urls = ["http://127.0.0.1:3434/metrics"]
  # Add a metric name prefix
  name_prefix = "teleport_"
  # Add tags to be able to make beautiful dashboards
  [inputs.prometheus.tags]
    teleservice = "teleport"

# Read metrics about cpu usage
[[inputs.cpu]]
  ## Whether to report per-cpu stats or not
  percpu = true
  ## Whether to report total system cpu stats or not
  totalcpu = true
  ## If true, collect raw CPU time metrics.
  collect_cpu_time = false
  ## If true, compute and report the sum of all non-idle CPU states.
  report_active = false

# Read metrics about disk usage by mount point
[[inputs.disk]]
  ## By default, telegraf gather stats for all mountpoints.
  ## Setting mountpoints will restrict the stats to the specified mountpoints.
  # mount_points = ["/"]

  ## Ignore some mountpoints by filesystem type. For example (dev)tmpfs (usually
  ## present on /run, /var/run, /dev/shm or /dev).
  ignore_fs = ["tmpfs", "devtmpfs", "devfs"]

# Read metrics about disk IO by device
[[inputs.diskio]]

# Get kernel statistics from /proc/stat
[[inputs.kernel]]
  # no configuration

# Read metrics about memory usage
[[inputs.mem]]
  # no configuration

# Get the number of processes and group them by status
[[inputs.processes]]
  # no configuration

# Read metrics about swap memory usage
[[inputs.swap]]
  # no configuration

# Read metrics about system load & uptime
[[inputs.system]]
  # no configuration

###############################################################################
#                            OUTPUT PLUGINS                                   #
###############################################################################

# Configuration for influxdb server to send metrics to
[[outputs.influxdb]]
  ## The full HTTP or UDP endpoint URL for your InfluxDB instance.
  ## Multiple urls can be specified as part of the same cluster,
  ## this means that only ONE of the urls will be written to each interval.
  urls = ["${influxdb_addr}"] # required
  ## The target database for metrics (telegraf will create it if not exists).
  database = "telegraf" # required

  ## Retention policy to write to. Empty string writes to the default rp.
  retention_policy = ""
  ## Write consistency (clusters only), can be: "any", "one", "quorum", "all"
  write_consistency = "any"

  ## Write timeout (for the InfluxDB client), formatted as a string.
  ## If not provided, will default to 5s. 0s means no timeout (not recommended).
  timeout = "5s"
EOF

systemctl enable telegraf.service
systemctl restart telegraf.service
