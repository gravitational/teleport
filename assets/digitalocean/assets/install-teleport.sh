#!/bin/bash

cat <<EOM
*********************************************************
**             Installating Teleport                   **
*********************************************************
EOM

# download teleport binaries
echo "Downloading Teleport binaries..."
curl -O https://get.gravitational.com/teleport-v7.1.0-linux-amd64-bin.tar.gz

# extract and install
tar -xzf teleport-v7.1.0-linux-amd64-bin.tar.gz
teleport/install

# install bcc
apt-get -y install bpfcc-tools linux-headers-$(uname -r)
echo 
echo Done.
cat <<EOM
*********************************************************
**              Configuring Teleport                   **
*********************************************************
EOM

# ask for cluster name
echo 
echo "___"
printf "• Enter Teleport cluster name:"
read -r cluster_name

# ask email address for acme
printf "• Enter your email address to retrieve TLS certificate from letsencrypt:"
read -r email_addr

# generate config file
teleport configure --acme --acme-email=$email_addr --cluster-name=$cluster_name -o file

# enable and start Teleport service
cat > /usr/lib/systemd/system/teleport.service <<EOF
[Unit]
Description=Teleport 7.1
After=network.target

[Service]
Type=simple
Restart=on-failure
EnvironmentFile=-/etc/default/teleport
ExecStart=/usr/local/bin/teleport start --pid-file=/run/teleport.pid
ExecReload=/bin/kill -HUP $MAINPID
PIDFile=/run/teleport.pid
LimitNOFILE=8192

[Install]
WantedBy=multi-user.target
EOF

systemctl enable teleport
systemctl start teleport


cat <<EOM
*********************************************************
**              Creating Teleport user                 **
*********************************************************
EOM

# ask email address for acme
printf "• Enter a username for initial Teleport user:"
read -r username
echo

# create Teleport user
tctl users add $username --roles=editor,access --logins=root
echo


echo 
echo Done.

cat <<EOM
***************************************************************
**        Teleport is configured. Happy Teleporting :)       **
***************************************************************
EOM

