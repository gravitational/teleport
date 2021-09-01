#!/bin/bash
#
# Configured as part of the DigitalOcean 1-Click Image build process

myip=$(hostname -I | awk '{print$1}')
cat <<EOF
********************************************************************************

Welcome to the Teleport 1-Click Application

On your first login you will be prompted to configure your Teleport installation.


Learn more about using this applicaition here:
https://goteleport.com/docs/getting-started/linux-server/

Need help with your setup? Ping us in our Slack channel:
https://goteleport.com/slack

********************************************************************************
To delete this message of the day: rm -rf $(readlink -f ${0})
EOF
