#!/bin/bash
set -x

# Update packages
yum -y update

# Install uuid used for random token generation
yum install -y uuid libffi-devel gcc openssl-devel

# Set some curl options so that temporary failures get retried
# More info: https://ec.haxx.se/usingcurl-timeouts.html
CURL_OPTS="-L --retry 100 --retry-delay 0 --connect-timeout 10 --max-time 300"

# Install certbot to rotate certificates
# Certbot is a tool to request letsencrypt certificates,
# remove it if you don't need letsencrypt.
curl ${CURL_OPTS} -O https://bootstrap.pypa.io/get-pip.py
python2.7 get-pip.py
pip install -I awscli requests[security]==2.18.4
pip install certbot==0.21.0 certbot-dns-route53==0.21.0

# Create teleport user. It is helpful to share the same UID
# to have the same permissions on shared NFS volumes across auth servers and for consistency.
useradd -r teleport -u ${TELEPORT_UID}
# Add teleport to adm group to read and write logs
usermod -a -G adm teleport

# Setup teleport run dir for pid files
mkdir -p /var/run/teleport/ /var/lib/teleport /etc/teleport.d
chown -R teleport:adm /var/run/teleport /var/lib/teleport /etc/teleport.d/

# Download and install teleport binaries
pushd /tmp
if [ "${TELEPORT_TYPE}" = "oss" ]; then
    echo Installing OSS Teleport
    curl ${CURL_OPTS} -o teleport.tar.gz https://s3.amazonaws.com/clientbuilds.gravitational.io/teleport/${TELEPORT_VERSION}/teleport-v${TELEPORT_VERSION}-linux-amd64-bin.tar.gz
    tar -xzf teleport.tar.gz
    cp teleport/tctl teleport/tsh teleport/teleport /usr/bin
    rm -rf /tmp/teleport.tar.gz /tmp/teleport
else
    echo Installing Enterprise Teleport
    curl ${CURL_OPTS} -o teleport.tar.gz https://get.gravitational.com/teleport/${TELEPORT_VERSION}/teleport-ent-v${TELEPORT_VERSION}-linux-amd64-bin.tar.gz
    tar -xzf teleport.tar.gz
    cp teleport-ent/tctl teleport-ent/tsh teleport-ent/teleport /usr/bin
    rm -rf /tmp/teleport.tar.gz /tmp/teleport-ent
    # Move enforcing license to the location expected by teleport
    mv /etc/systemd/system/license.pem /var/lib/teleport/license.pem
    chown teleport:adm /var/lib/teleport/license.pem
fi
popd

# Clean up the authorized keys not used
rm -f /root/.ssh/authorized_keys
rm -f /home/ec2-user/.ssh/authorized_keys

# Clean up copied temp files
rm -rf /tmp/files

yum -y clean all
