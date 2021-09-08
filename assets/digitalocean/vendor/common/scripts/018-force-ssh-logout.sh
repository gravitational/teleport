#!/bin/sh

cat >> /etc/ssh/sshd_config <<EOM
Match User root
        ForceCommand echo "Please wait while we get your droplet ready..."
EOM

# If your build uses this script add the following code snippet
# to your cloud-init scripts
# /var/lib/cloud/scripts/per-instance/onboot/001_onboot
# # Remove the ssh force logout command
# sed -e '/Match User root/d' \
#     -e '/.*ForceCommand.*droplet.*/d' \
#     -i /etc/ssh/sshd_config
#
# systemctl restart ssh
