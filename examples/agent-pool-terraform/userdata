#!/bin/bash

curl https://goteleport.com/static/install.sh | bash -s ${teleport_version}

echo ${token} > /var/lib/teleport/token
cat<<EOF >/etc/teleport.yaml
version: v3
teleport:
  auth_token: /var/lib/teleport/token
  proxy_server: ${proxy_service_address}
app_service:
  enabled: true
  resources:
  - labels:
      "*": "*"
auth_service:
  enabled: false
db_service:
  enabled: true
  resources:
  - labels:
      "*": "*"
discovery_service:
  enabled: true
kubernetes_service:
  enabled: true
  resources:
  - labels:
      "*": "*"
proxy_service:
  enabled: false
ssh_service:
  labels:
    role: agent-pool
EOF

systemctl restart teleport;
