#!/bin/bash
if [[ "${DEBUG:-false}" == "true" ]]; then
    set -x
fi

# Update packages
yum -y update

# Install uuid used for random token generation, nginx for grafana frontend
yum install -y uuid libffi-devel gcc openssl-devel adduser libfontconfig

# Install nginx
amazon-linux-extras install nginx1

# Set some curl options so that temporary failures get retried
# More info: https://ec.haxx.se/usingcurl-timeouts.html
CURL_OPTS="-L --retry 100 --retry-delay 0 --connect-timeout 10 --max-time 300"

# Install telegraf to collect stats from influx
curl ${CURL_OPTS} -o /tmp/telegraf.rpm "https://dl.influxdata.com/telegraf/releases/telegraf-${TELEGRAF_VERSION}-1.x86_64.rpm"
yum install -y /tmp/telegraf.rpm
rm -f /tmp/telegraf.rpm

# Install grafana
curl ${CURL_OPTS} -o /tmp/grafana.rpm "https://dl.grafana.com/oss/release/grafana-${GRAFANA_VERSION}-1.x86_64.rpm"
yum install -y /tmp/grafana.rpm
rm -f /tmp/grafana.rpm

# Install InfluxDB
curl $CURL_OPTS -o /tmp/influxdb.rpm "https://dl.influxdata.com/influxdb/releases/influxdb-${INFLUXDB_VERSION}.x86_64.rpm"
yum install -y /tmp/influxdb.rpm
rm -f /tmp/influxdb.rpm

# Install certbot to rotate certificates
# Certbot is a tool to request letsencrypt certificates,
# remove it if you don't need letsencrypt.
yum -y install python3 python3-pip
# pip needs to be upgraded to work around issues with the 'cryptography' package
pip3 install --upgrade pip
# add new pip3 install location to PATH temporarily
export PATH=/usr/local/bin:$PATH
pip3 install -I awscli requests
pip3 install certbot certbot-dns-route53

# Create teleport user. It is helpful to share the same UID
# to have the same permissions on shared NFS volumes across auth servers and for consistency.
useradd -r teleport -u ${TELEPORT_UID} -d /var/lib/teleport
# Add teleport to adm group to read and write logs
usermod -a -G adm teleport

# Setup teleport run dir for pid files
mkdir -p /run/teleport/ /var/lib/teleport /etc/teleport.d
chmod 0700 /var/lib/teleport
chown -R teleport:adm /run/teleport /var/lib/teleport /etc/teleport.d/

# Download and install teleport binaries
pushd /tmp || exit
# Install the FIPS version of Teleport if /tmp/teleport-fips is present
if [ -f /tmp/teleport-fips ]; then
    TARBALL_FILENAME="/tmp/files/teleport-ent-v${TELEPORT_VERSION}-linux-amd64-fips-bin.tar.gz"
    # Use a Teleport artifact uploaded from the build machine, if present
    if [ -f ${TARBALL_FILENAME} ]; then
        echo "Found locally uploaded Enterprise FIPS tarball ${TARBALL_FILENAME}, moving to /tmp/teleport.tar.gz"
        mv ${TARBALL_FILENAME} /tmp/teleport.tar.gz
    else
        echo "Installing Enterprise Teleport version ${TELEPORT_VERSION} with FIPS support"
        curl ${CURL_OPTS} -o teleport.tar.gz https://cdn.teleport.dev/teleport-ent-v${TELEPORT_VERSION}-linux-amd64-fips-bin.tar.gz
    fi
    tar -xzf teleport.tar.gz
    cp teleport-ent/tctl teleport-ent/tsh teleport-ent/teleport teleport-ent/tbot /usr/local/bin
    rm -rf /tmp/teleport.tar.gz /tmp/teleport-ent
    # add --fips to 'teleport start' commands in FIPS mode
    sed -i -E "s_ExecStart=/usr/local/bin/teleport start(.*)_ExecStart=/usr/local/bin/teleport start --fips\1_g" /etc/systemd/system/teleport*.service
else
    if [[ "${TELEPORT_TYPE}" == "oss" ]]; then
        TARBALL_FILENAME="/tmp/files/teleport-v${TELEPORT_VERSION}-linux-amd64-bin.tar.gz"
        # Use a Teleport artifact uploaded from the build machine, if present
        if [ -f ${TARBALL_FILENAME} ]; then
            echo "Found locally uploaded OSS tarball ${TARBALL_FILENAME}, moving to /tmp/teleport.tar.gz"
            mv ${TARBALL_FILENAME} /tmp/teleport.tar.gz
        else
            echo "Installing OSS Teleport version ${TELEPORT_VERSION}"
            curl ${CURL_OPTS} -o teleport.tar.gz https://cdn.teleport.dev/teleport-v${TELEPORT_VERSION}-linux-amd64-bin.tar.gz
        fi
        tar -xzf teleport.tar.gz
        cp teleport/tctl teleport/tsh teleport/teleport teleport/tbot /usr/local/bin
        rm -rf /tmp/teleport.tar.gz /tmp/teleport
    else
        TARBALL_FILENAME="/tmp/files/teleport-ent-v${TELEPORT_VERSION}-linux-amd64-bin.tar.gz"
        # Use a Teleport artifact uploaded from the build machine, if present
        if [ -f ${TARBALL_FILENAME} ]; then
             echo "Found locally uploaded Enterprise tarball ${TARBALL_FILENAME}, moving to /tmp/teleport.tar.gz"
            mv ${TARBALL_FILENAME} /tmp/teleport.tar.gz
        else
            echo "Installing Enterprise Teleport version ${TELEPORT_VERSION}"
            curl ${CURL_OPTS} -o teleport.tar.gz https://cdn.teleport.dev/teleport-ent-v${TELEPORT_VERSION}-linux-amd64-bin.tar.gz
        fi
        tar -xzf teleport.tar.gz
        cp teleport-ent/tctl teleport-ent/tsh teleport-ent/teleport teleport-ent/tbot /usr/local/bin
        rm -rf /tmp/teleport.tar.gz /tmp/teleport-ent
    fi
fi
popd || exit

# Add /usr/local/bin to path used by sudo (so 'sudo tctl users add' will work as per the docs)
echo "Defaults    secure_path = /sbin:/bin:/usr/sbin:/usr/bin:/usr/local/bin" > /etc/sudoers.d/secure_path

# Clean up the authorized keys not used
rm -f /root/.ssh/authorized_keys
rm -f /home/ec2-user/.ssh/authorized_keys

# Clean up copied temp files
rm -rf /tmp/files

# Clean up all packages
yum -y clean all

# Enable Teleport services to start on boot
systemctl enable teleport-generate-config.service
systemctl enable teleport.service
