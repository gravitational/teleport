#!/bin/bash
set -x

# Install uuid used for token generation
apt-get install -y uuid

# Set some curl options so that temporary failures get retried
# More info: https://ec.haxx.se/usingcurl-timeouts.html
CURL_OPTS="-L --retry 100 --retry-delay 0 --connect-timeout 10 --max-time 300"

# Install telegraf to collect stats from influx
curl $CURL_OPTS -o /tmp/telegraf.deb https://dl.influxdata.com/telegraf/releases/telegraf_${telegraf_version}_amd64.deb
dpkg -i /tmp/telegraf.deb
rm -f /tmp/telegraf.deb

# Setup teleport data dir used for transient storage
mkdir -p /var/lib/teleport/
chown -R root:adm /var/lib/teleport

# Setup teleport run dir for pid files
mkdir -p /var/run/teleport/
chown -R root:adm /var/run/teleport

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

# Fetch token published by Auth server to SSM parameter store to join the cluster
aws ssm get-parameter --with-decryption --name /teleport/${cluster_name}/tokens/node --region ${region} --query Parameter.Value --output text > /var/lib/teleport/token

# Fetch Auth server CA certificate to validate the identity of the auth server
aws ssm get-parameter --name /teleport/${cluster_name}/ca --region=${region} --query=Parameter.Value --output text > /var/lib/teleport/ca.cert
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
ExecStart=/usr/local/bin/teleport start --config=/etc/teleport.yaml --diag-addr=127.0.0.1:3434 --pid-file=/var/run/teleport/teleport.pid
ExecReload=/bin/kill -HUP \$$MAINPID
PIDFile=/var/run/teleport/teleport.pid
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF
systemctl enable teleport
systemctl start teleport


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
