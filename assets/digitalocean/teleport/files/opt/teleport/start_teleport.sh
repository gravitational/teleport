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
printf "• Enter Teleport cluster name (FQDN): "
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

# enable teleport service
systemctl enable teleport &>/dev/null

# create Teleport user

SIGNUP_LINK=$(tctl users add $username --roles=editor,access --logins=root | grep "https://")

echo
echo "***************************************************************************"
echo "**                             ---                                      "
echo "**    Teleport is configured and user $username has been created    "
echo "**    but requires a password. Open the URL link below to complete the   "
echo "**    setup. The link is valid for 1h:"
echo "**"
echo "**    $SIGNUP_LINK  "
echo "**"
echo "**                             ---                                       "
echo "**                     HAPPY TELEPORTING :)                              "
echo "***************************************************************************"



echo 
cat <<EOM
Is it OK if we collect some info about your install?
Please run this command to send in a survey.
(optional - replace email to join our newsletter and get a swag package.)

curl -X POST https://usage.teleport.dev -F OS=linux -F use-case="access my ..." -F email="alice@example.com"

Otherwise, ignore!
EOM
echo

# replace .bashrc to prevent running this script again.
cp -f /etc/skel/.bashrc /root/.bashrc