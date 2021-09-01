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
sleep 2

# ask username for initial user
printf "• Enter a username for the initial Teleport user: "
read -r username
echo

echo "Starting Teleport..."

# For some reason, Teleport is not using configurations from /etc/teleport.yaml until restart! 

systemctl restart teleport
sleep 7

# create Teleport user
tctl users add $username --roles=editor,access --logins=root
echo


echo 
cat <<EOM
*********************************************************
**     Teleport is configured. Happy Teleporting :)    **
*********************************************************
EOM

# replace .bashrc to prevent running this script again.
cp -f /etc/skel/.bashrc /root/.bashrc