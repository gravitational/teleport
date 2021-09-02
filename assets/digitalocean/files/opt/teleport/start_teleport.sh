#!/bin/bash

clear
echo
cat <<EOM
*********************************************************
**              Configuring Teleport                   **
*********************************************************
EOM

# ask for cluster name
echo "___"
printf "• Enter Teleport cluster name (public domain name): "
read -r cluster_name

# ask email address for acme
printf "• Enter your email address to retrieve TLS certificate from Let's Encrypt: "
read -r email_addr

# generate config file
teleport configure --acme --acme-email=$email_addr --cluster-name=$cluster_name -o file >/dev/null


# start teleport service
systemctl start teleport
#sleep 2

# ask username for initial user
printf "• Enter a username for the initial Teleport user: "
read -r username
echo

echo "Initializing..."
sleep 2
echo "[+] Generating new host UUID..."
sleep 1
echo "[+] Updating cluster networking configuration..."
sleep 2
echo "[+] Generating user and host certificate authority..."
sleep 3
echo "[+] Enabling RBAC in OSS Teleport. Migrating users, roles and trusted clusters..."
sleep 2
echo "[+] Starting Auth and Proxy services..."
sleep 5
echo "[+] Final Checks..."

# sleep until port 443 is ready
while ! lsof -Pi :443 -sTCP:LISTEN -t >/dev/null; do
    sleep 2
done

# create Teleport user
echo
tctl users add $username --roles=editor,access --logins=root
echo

systemctl enable teleport >/dev/null


echo 
cat <<EOM
**********************************************************************
**                    Teleport is configured!                       ** 
**    To restart this wizard, perform following steps:              **
**    1) Delete existing config file: $ rm -rf /etc/teleport.yaml   **
**    2) Delete existing local data: $ rm -rf /var/lib/teleport/    **
**    3) Start wizard: $ /opt/teleport/start_teleport.sh            **
**                             ---                                  ** 
**                     HAPPY TELEPORTING :)                         **
**********************************************************************
EOM

# replace .bashrc to prevent running this script again.
cp -f /etc/skel/.bashrc /root/.bashrc