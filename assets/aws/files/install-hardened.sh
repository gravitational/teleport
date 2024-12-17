#!/bin/bash

# Update packages
dnf -y update

# Install
#  - uuid used for random token generation
#  - python for certbot
dnf install -y uuid python3

# Install certbot
python3 -m venv /opt/certbot
/opt/certbot/bin/pip install --upgrade pip
/opt/certbot/bin/pip install certbot certbot-dns-route53
ln -s /opt/certbot/bin/certbot /usr/local/bin/certbot

# Create teleport user. It is helpful to share the same UID
# to have the same permissions on shared NFS volumes across auth servers and for consistency.
useradd -r teleport -u "${TELEPORT_UID}" -d /var/lib/teleport
# Add teleport to adm group to read and write logs
usermod -a -G adm teleport

# Setup teleport run dir for pid files
install -d -m 0700 -o teleport -g adm /var/lib/teleport
install -d -m 0755 -o teleport -g adm /run/teleport /etc/teleport.d
# Setup teleport/system directory
install -d -m 0755 -o teleport -g adm /opt/teleport/system/bin
install -d -m 0755 -o teleport -g adm /opt/teleport/system/lib/systemd/system

# Extract tarball to /tmp/teleport to get the binaries out
mkdir /tmp/teleport
tar -C /tmp/teleport -x -z -f /tmp/teleport.tar.gz --strip-components=1
install -m 755 /tmp/teleport/{tctl,tsh,teleport,tbot,fdpass-teleport,teleport-update} /opt/teleport/system/bin
install -m 755 /tmp/teleport/examples/systemd/teleport.service /opt/teleport/system/lib/systemd/system
/opt/teleport/system/bin/teleport-update link-package
rm -rf /tmp/teleport /tmp/teleport.tar.gz

if [[ "${TELEPORT_FIPS}" == 1 ]]; then
    # add --fips to 'teleport start' commands in FIPS mode
    sed -i -E 's_^(ExecStart=/usr/local/bin/teleport start)_\1 --fips_' /etc/systemd/system/teleport*.service
    # https://docs.aws.amazon.com/linux/al2023/ug/fips-mode.html
    dnf install -y crypto-policies crypto-policies-scripts
    fips-mode-setup --enable
fi

# Add /usr/local/bin to path used by sudo (so 'sudo tctl users add' will work as per the docs)
echo "Defaults    secure_path = /sbin:/bin:/usr/sbin:/usr/bin:/usr/local/bin" > /etc/sudoers.d/secure_path

# Clean up the authorized keys not used
rm -f /root/.ssh/authorized_keys
rm -f /home/ec2-user/.ssh/authorized_keys

# Clean up copied temp files
rm -rf /tmp/files

# Clean up all packages
dnf -y clean all
rm -rf /var/cache/dnf /var/cache/yum

# Enable Teleport services to start on boot
systemctl enable teleport-generate-config.service
systemctl enable teleport.service
